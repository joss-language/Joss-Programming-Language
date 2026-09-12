package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
)

const (
	sqliteDbFile  = "database.sqlite"
	sqliteShmFile = "database.sqlite-shm"
	sqliteWalFile = "database.sqlite-wal"
)

var supportedTargets = map[string][]string{
	"windows": {"amd64", "arm64", "386"},
	"linux":   {"amd64", "arm64", "arm", "386", "riscv64"},
	"darwin":  {"amd64", "arm64"},
}

// buildNative orchestrates self-contained native binary compilation.
func buildNative(targetOS, targetArch string, enableGUI bool) {
	tOS, tArch, valid := validateBuildTarget(targetOS, targetArch)
	if !valid {
		os.Exit(1)
	}

	modeStr := "Console CLI"
	if enableGUI {
		modeStr = "Desktop GUI (Sin consola CMD)"
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("🔨 JOSS NATIVE COMPILER (Cross-Platform Native Binary)\n")
	fmt.Printf(" Destino     : %s/%s\n", tOS, tArch)
	fmt.Printf(" Modo        : %s\n", modeStr)
	fmt.Printf(" CGO Enabled : 0 (Enlazado Estático Puro)\n")
	fmt.Printf("=======================================================\n\n")

	if _, err := exec.LookPath("go"); err != nil {
		fmt.Println(i18n.Tr("nativeBuildMissingGo"))
		os.Exit(1)
	}

	buildDir := "build"
	os.RemoveAll(buildDir)
	if err := os.MkdirAll(filepath.Join(buildDir, "Storage"), 0755); err != nil {
		fmt.Println(i18n.Tr("nativeBuildDirError", i18n.M{"error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("nativeBuildPackagingAssets"))
	encryptedAssets, buildKey, err := collectAndEncryptAssets(enableGUI)
	if err != nil {
		fmt.Println(i18n.Tr("nativeBuildAssetsError", i18n.M{"error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("nativeBuildCompilingRunner"))
	runnerBytes, err := compileRunnerBinary(tOS, tArch, enableGUI)
	if err != nil {
		fmt.Println(i18n.Tr("nativeBuildRunnerError", i18n.M{"error": err.Error()}))
		os.Exit(1)
	}

	outPath, err := assembleFinalExecutable(buildDir, tOS, runnerBytes, encryptedAssets, buildKey)
	if err != nil {
		fmt.Println(i18n.Tr("nativeBuildAssembleError", i18n.M{"error": err.Error()}))
		os.Exit(1)
	}

	copyDatabaseFiles(buildDir)

	stat, _ := os.Stat(outPath)
	sizeMB := float64(stat.Size()) / (1024 * 1024)

	fmt.Printf("\n%s\n", i18n.Tr("nativeBuildSuccessTitle"))
	fmt.Printf(" %s : %s\n", i18n.Tr("nativeBuildOutputFile"), outPath)
	fmt.Printf(" %s   : %.2f MB\n", i18n.Tr("nativeBuildBinarySize"), sizeMB)
	fmt.Printf(" %s           : %s/%s\n", i18n.Tr("nativeBuildTarget"), tOS, tArch)
	fmt.Printf(" %s              : %s\n", i18n.Tr("nativeBuildMode"), modeStr)
	fmt.Printf(" %s\n\n", i18n.Tr("nativeBuildInstructions", map[string]interface{}{"path": outPath, "os": strings.ToUpper(tOS)}))
}

func validateBuildTarget(targetOS, targetArch string) (string, string, bool) {
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	tOS := strings.ToLower(targetOS)
	tArch := strings.ToLower(targetArch)

	archs, osValid := supportedTargets[tOS]
	if !osValid {
		fmt.Println(i18n.Tr("nativeBuildUnsupportedOS", map[string]interface{}{"os": targetOS}))
		return tOS, tArch, false
	}

	for _, a := range archs {
		if a == tArch {
			return tOS, tArch, true
		}
	}

	fmt.Println(i18n.Tr("nativeBuildUnsupportedArch", map[string]interface{}{"arch": targetArch, "os": targetOS, "options": strings.Join(archs, ", ")}))
	return tOS, tArch, false
}

func collectAndEncryptAssets(enableGUI bool) ([]byte, []byte, error) {
	buildKey := make([]byte, 32)
	if _, err := rand.Read(buildKey); err != nil {
		return nil, nil, err
	}

	files := make(map[string][]byte)
	ignoredDirs := map[string]bool{
		".git": true, ".vscode": true, ".idea": true, "build": true, "vendor": true,
		"node_modules": true, ".gemini": true, ".codex": true, ".agents": true, ".github": true,
	}

	compiledCount := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		return processSingleWalkFile(path, info, err, files, ignoredDirs, &compiledCount)
	})

	if err != nil {
		return nil, nil, err
	}

	fmt.Printf("⚡ %s\n", i18n.Tr("nativeBuildPrecompiledFiles", i18n.M{"count": compiledCount}))
	encryptProjectEnvironment(files, enableGUI)

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(files); err != nil {
		return nil, nil, err
	}

	encryptedAssets, err := crypto.EncryptAES(buf.Bytes(), buildKey)
	return encryptedAssets, buildKey, err
}

func processSingleWalkFile(path string, info os.FileInfo, err error, files map[string][]byte, ignoredDirs map[string]bool, compiledCount *int) error {
	if err != nil || path == "." {
		return nil
	}
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) > 0 && ignoredDirs[parts[0]] {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if info.IsDir() {
		return nil
	}

	name := strings.ToLower(info.Name())
	if shouldSkipFileByName(name) || info.Size() > 5<<20 {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	relPath := filepath.ToSlash(path)
	if strings.HasSuffix(relPath, ".joss") {
		if bcData, ok := tryCompileJossBytecode(data); ok {
			data = bcData
			*compiledCount++
		}
	}

	files[relPath] = data
	return nil
}

func shouldSkipFileByName(name string) bool {
	return strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".log") ||
		strings.HasSuffix(name, ".enc") || strings.HasSuffix(name, ".dll") ||
		strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dylib") ||
		name == "runner" || name == "joss"
}

func tryCompileJossBytecode(data []byte) ([]byte, bool) {
	l := parser.NewLexer(string(data))
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	if len(p.Errors()) == 0 && prog != nil {
		if bc, bcErr := bytecode.Encode(prog); bcErr == nil {
			return bc, true
		}
	}
	return nil, false
}

func encryptProjectEnvironment(files map[string][]byte, enableGUI bool) {
	envPath := GetEnvFile()
	data, _ := os.ReadFile(envPath)

	if enableGUI {
		data = append(data, []byte("\nJOSS_GUI=\"true\"")...)
	} else {
		data = append(data, []byte("\nJOSS_GUI=\"false\"")...)
	}

	if _, err := os.Stat(sqliteDbFile); err == nil {
		override := "\nDB_PATH=\"Storage/" + sqliteDbFile + "\""
		data = append(data, []byte(override)...)
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	masterSecret := []byte("JOSSECURITY_MASTER_SECRET_2025")
	key := crypto.DeriveKey(masterSecret, salt)
	encrypted, err := crypto.EncryptAES(data, key)
	if err == nil {
		files["env.enc"] = append(salt, encrypted...)
	}
}

func compileRunnerBinary(targetOS, targetArch string, enableGUI bool) ([]byte, error) {
	tempRunnerDir, err := os.MkdirTemp("", "joss-build-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempRunnerDir)

	tempRunnerBin := filepath.Join(tempRunnerDir, "runner")
	if targetOS == "windows" {
		tempRunnerBin += ".exe"
	}

	runnerPkg := "github.com/jossecurity/joss/cmd/runner"
	if _, err := os.Stat("cmd/runner"); err == nil {
		runnerPkg = "./cmd/runner"
	}

	ldflags := "-s -w"
	if targetOS == "windows" && enableGUI {
		ldflags += " -H=windowsgui"
	}
	cmd := exec.Command("go", "build", "-ldflags="+ldflags, "-o", tempRunnerBin, runnerPkg)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+targetOS, "GOARCH="+targetArch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, string(out))
	}

	if upxPath, err := exec.LookPath("upx"); err == nil {
		upxCmd := exec.Command(upxPath, "--best", "--lzma", tempRunnerBin)
		_ = upxCmd.Run()
	}

	return os.ReadFile(tempRunnerBin)
}

func compressFinalExecutableWithUPX(outPath string) {
	if upxPath, err := exec.LookPath("upx"); err == nil {
		upxCmd := exec.Command(upxPath, "--best", "--lzma", outPath)
		_ = upxCmd.Run()
	}
}

func assembleFinalExecutable(buildDir, targetOS string, runnerBytes, encryptedAssets, buildKey []byte) (string, error) {
	projectName := filepath.Base(getWorkingDir())
	if projectName == "." || projectName == "/" || projectName == "" {
		projectName = "app"
	}

	exeName := projectName
	if targetOS == "windows" {
		exeName += ".exe"
	}

	outPath := filepath.Join(buildDir, exeName)
	outFile, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	if _, err := outFile.Write(runnerBytes); err != nil {
		return "", err
	}
	if _, err := outFile.Write(encryptedAssets); err != nil {
		return "", err
	}
	if _, err := outFile.Write(buildKey); err != nil {
		return "", err
	}

	lenBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenBuf, uint64(len(encryptedAssets)))
	if _, err := outFile.Write(lenBuf); err != nil {
		return "", err
	}

	magic := []byte("JOSS_RUNNER_DATA")
	if _, err := outFile.Write(magic); err != nil {
		return "", err
	}

	if targetOS != "windows" {
		os.Chmod(outPath, 0755)
	}

	compressFinalExecutableWithUPX(outPath)

	return outPath, nil
}

func copyDatabaseFiles(buildDir string) {
	if _, err := os.Stat(sqliteDbFile); err == nil {
		copyFile(sqliteDbFile, filepath.Join(buildDir, "Storage", sqliteDbFile))
		if _, err := os.Stat(sqliteShmFile); err == nil {
			copyFile(sqliteShmFile, filepath.Join(buildDir, "Storage", sqliteShmFile))
		}
		if _, err := os.Stat(sqliteWalFile); err == nil {
			copyFile(sqliteWalFile, filepath.Join(buildDir, "Storage", sqliteWalFile))
		}
		fmt.Printf("🗄️  %s\n", i18n.Tr("nativeBuildDbCopied"))
	}
}

func getWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "app"
	}
	return dir
}

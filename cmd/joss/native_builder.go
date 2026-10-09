package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/buildmanifest"
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
	"android": {"arm64", "arm", "amd64", "386"},
}

// buildNative orchestrates self-contained native binary compilation.
func buildNative(targetOS, targetArch string, enableGUI bool) {
	_, _ = buildNativeWithOutput(targetOS, targetArch, enableGUI, "")
}

func buildNativeWithOutput(targetOS, targetArch string, enableGUI bool, outExe string) (string, error) {
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
	encryptedAssets, buildKey, detectedCaps, err := collectAndEncryptAssets(enableGUI)
	if err != nil {
		fmt.Println(i18n.Tr("nativeBuildAssetsError", i18n.M{"error": err.Error()}))
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("nativeBuildCompilingRunner"))
	runnerBytes, err := compileRunnerBinary(tOS, tArch, enableGUI, detectedCaps)
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

	if outExe != "" && outExe != outPath {
		if data, err := os.ReadFile(outPath); err == nil {
			_ = os.WriteFile(outExe, data, 0755)
		}
	}

	stat, _ := os.Stat(outPath)
	sizeMB := float64(stat.Size()) / (1024 * 1024)

	fmt.Printf("\n%s\n", i18n.Tr("nativeBuildSuccessTitle"))
	fmt.Printf(" %s : %s\n", i18n.Tr("nativeBuildOutputFile"), outPath)
	if outExe != "" && outExe != outPath {
		fmt.Printf(" %s (destino) : %s\n", i18n.Tr("nativeBuildOutputFile"), outExe)
	}
	fmt.Printf(" %s   : %.2f MB\n", i18n.Tr("nativeBuildBinarySize"), sizeMB)
	fmt.Printf(" %s           : %s/%s\n", i18n.Tr("nativeBuildTarget"), tOS, tArch)
	fmt.Printf(" %s              : %s\n", i18n.Tr("nativeBuildMode"), modeStr)
	fmt.Printf(" %s\n\n", i18n.Tr("nativeBuildInstructions", map[string]interface{}{"path": outPath, "os": strings.ToUpper(tOS)}))
	return outPath, nil
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

func collectAndEncryptAssets(enableGUI bool) ([]byte, []byte, map[string]bool, error) {
	buildKey := make([]byte, 32)
	if _, err := rand.Read(buildKey); err != nil {
		return nil, nil, nil, err
	}

	files := make(map[string][]byte)
	ignoredDirs := map[string]bool{
		".git": true, ".vscode": true, ".idea": true, "build": true, "vendor": true,
		"node_modules": true, ".gemini": true, ".codex": true, ".agents": true, ".github": true,
	}

	// 1. Run reachability analysis starting from main.joss
	bCfg := LoadProjectBuildConfig()
	reachGraph, units, reachErr := AnalyzeProjectReachability("main.joss", bCfg.KeepClasses, bCfg.KeepSymbols)
	if reachErr != nil {
		fmt.Printf("⚠️  [Build Warning] Reachability analysis fallback: %v\n", reachErr)
	}

	compiledCount := 0
	var allDiscoveredFiles []string

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
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
		allDiscoveredFiles = append(allDiscoveredFiles, filepath.ToSlash(path))
		return processSingleWalkFile(path, info, err, files, ignoredDirs, &compiledCount)
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// 2. Prune dead Joss files and methods from the VFS map if reachability was computed
	if reachGraph != nil {
		prunedUnits := FilterProjectFiles(allDiscoveredFiles, reachGraph, units, bCfg.PruneMethods)
		for filePath, fileBytes := range files {
			if strings.HasSuffix(filePath, ".joss") {
				if bCfg.PruneFiles && !reachGraph.IsFileReachable(filePath) {
					// Drop dead file entirely from packaged VFS!
					delete(files, filePath)
					continue
				}
				// If pruned AST is available, re-encode to optimized bytecode
				if prunedUnit, exists := prunedUnits[filePath]; exists && prunedUnit.Program != nil {
					if bc, bcErr := bytecode.Encode(prunedUnit.Program); bcErr == nil {
						files[filePath] = bc
					}
				}
			}
			_ = fileBytes
		}
	}

	fmt.Printf("⚡ %s\n", i18n.Tr("nativeBuildPrecompiledFiles", i18n.M{"count": compiledCount}))
	encryptProjectEnvironment(files, enableGUI)

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(files); err != nil {
		return nil, nil, nil, err
	}

	encryptedAssets, err := crypto.EncryptAES(buf.Bytes(), buildKey)
	if err == nil && reachGraph != nil {
		manifest := buildmanifest.GenerateManifest(
			"main.joss", bCfg.Mode, runtime.GOOS, runtime.GOARCH, bCfg.Profile,
			filepath.Join("build", "app"), allDiscoveredFiles, reachGraph, int64(len(encryptedAssets)),
		)
		_ = manifest.SaveToFile(filepath.Join(".joss", "cache", "build-manifest.json"))
	}

	capsMap := make(map[string]bool)
	if reachGraph != nil {
		for cap, active := range reachGraph.RuntimeCapabilities {
			capsMap[string(cap)] = active
		}
	}

	return encryptedAssets, buildKey, capsMap, err
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
		name == "runner" || name == "joss" ||
		name == ".env" || name == "env.joss" || name == ".env.joss"
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

func compileRunnerBinary(targetOS, targetArch string, enableGUI bool, detectedCaps map[string]bool) ([]byte, error) {
	tempRunnerDir, err := os.MkdirTemp("", "joss-build-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempRunnerDir)

	tempRunnerBin := filepath.Join(tempRunnerDir, "runner")
	if targetOS == "windows" {
		tempRunnerBin += ".exe"
	}

	repoRoot := ""
	for check := "."; ; check = filepath.Join("..", check) {
		absCheck, err := filepath.Abs(check)
		if err != nil {
			break
		}
		if _, err := os.Stat(filepath.Join(absCheck, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(absCheck, "cmd", "runner")); err == nil {
				repoRoot = absCheck
				break
			}
		}
		parent := filepath.Dir(absCheck)
		if parent == absCheck {
			break
		}
	}

	runnerPkg := "github.com/jossecurity/joss/cmd/runner"
	if repoRoot != "" {
		runnerPkg = "./cmd/runner"
	} else if _, err := os.Stat("cmd/runner"); err == nil {
		runnerPkg = "./cmd/runner"
	}

	ldflags := "-s -w"
	if targetOS == "windows" && enableGUI {
		ldflags += " -H=windowsgui"
	}

	var tags []string
	if detectedCaps != nil && !detectedCaps["server"] && !detectedCaps["http"] && !enableGUI {
		tags = append(tags, "cli")
	}

	args := []string{"build", "-ldflags=" + ldflags}
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}
	args = append(args, "-o", tempRunnerBin, runnerPkg)

	cmd := exec.Command("go", args...)
	if repoRoot != "" {
		cmd.Dir = repoRoot
	}
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+targetOS, "GOARCH="+targetArch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, string(out))
	}

	if upxPath := resolveOrDownloadUPX(); upxPath != "" {
		fmt.Printf("⚡ Optimizando runtime nativo con UPX...\n")
		upxCmd := exec.Command(upxPath, "--best", "--lzma", tempRunnerBin)
		_ = upxCmd.Run()
	}

	return os.ReadFile(tempRunnerBin)
}

func compressFinalExecutableWithUPX(outPath string) {
	if upxPath := resolveOrDownloadUPX(); upxPath != "" {
		upxCmd := exec.Command(upxPath, "--best", "--lzma", outPath)
		_ = upxCmd.Run()
	}
}

// resolveOrDownloadUPX ensures a functional UPX executable is available.
// It checks PATH, Joss tools directory, and automatically downloads a standalone binary if missing.
func resolveOrDownloadUPX() string {
	// 1. Check system PATH first
	if p, err := exec.LookPath("upx"); err == nil {
		return p
	}

	// 2. Check Joss local tools directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}

	toolsDir := filepath.Join(homeDir, ".joss", "tools")
	exeName := "upx"
	if runtime.GOOS == "windows" {
		exeName = "upx.exe"
	}
	cachedUPX := filepath.Join(toolsDir, exeName)
	if fi, err := os.Stat(cachedUPX); err == nil && !fi.IsDir() {
		return cachedUPX
	}

	// Also check next to running joss binary
	if selfExe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(selfExe), exeName)
		if fi, err := os.Stat(sibling); err == nil && !fi.IsDir() {
			return sibling
		}
	}

	// 3. Attempt automated download for host platform
	_ = os.MkdirAll(toolsDir, 0755)
	if downloaded := downloadUPXForHost(cachedUPX); downloaded {
		return cachedUPX
	}

	return ""
}

func downloadUPXForHost(destPath string) bool {
	archiveURL, innerFile := getUPXDownloadURL()
	if archiveURL == "" {
		return false
	}

	fmt.Printf("⚡ Descargando optimizador binario nativo (UPX) para %s/%s...\n", runtime.GOOS, runtime.GOARCH)

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Get(archiveURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return false
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	if strings.HasSuffix(archiveURL, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
		if err != nil {
			return false
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == innerFile {
				rc, err := f.Open()
				if err != nil {
					return false
				}
				defer rc.Close()
				out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return false
				}
				defer out.Close()
				_, err = io.Copy(out, rc)
				return err == nil
			}
		}
	} else if strings.HasSuffix(archiveURL, ".tar.xz") || strings.HasSuffix(archiveURL, ".tar.gz") {
		// For tar.gz or simple archives, extract the upx binary
		gzr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err == nil {
			defer gzr.Close()
			tr := tar.NewReader(gzr)
			for {
				hdr, err := tr.Next()
				if err != nil {
					break
				}
				if filepath.Base(hdr.Name) == innerFile {
					out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
					if err != nil {
						return false
					}
					defer out.Close()
					_, err = io.Copy(out, tr)
					return err == nil
				}
			}
		}
	}

	return false
}

func getUPXDownloadURL() (string, string) {
	// Standard release: UPX 5.2.1
	const version = "5.2.1"
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("https://github.com/upx/upx/releases/download/v%s/upx-%s-win64.zip", version, version), "upx.exe"
		case "386":
			return fmt.Sprintf("https://github.com/upx/upx/releases/download/v%s/upx-%s-win32.zip", version, version), "upx.exe"
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("https://github.com/upx/upx/releases/download/v%s/upx-%s-amd64_linux.tar.xz", version, version), "upx"
		case "arm64":
			return fmt.Sprintf("https://github.com/upx/upx/releases/download/v%s/upx-%s-arm64_linux.tar.xz", version, version), "upx"
		case "arm":
			return fmt.Sprintf("https://github.com/upx/upx/releases/download/v%s/upx-%s-arm_linux.tar.xz", version, version), "upx"
		}
	}
	return "", ""
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

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
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/buildmanifest"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
	"github.com/jossecurity/joss/pkg/server"
)

const (
	sqliteDbFile  = "database.sqlite"
	sqliteShmFile = "database.sqlite-shm"
	sqliteWalFile = "database.sqlite-wal"
)

func collectProjectFiles(enableGUI bool) (map[string][]byte, map[string]bool, error) {
	files := make(map[string][]byte)
	ignoredDirs := map[string]bool{
		".git": true, ".vscode": true, ".idea": true, "build": true, "dist": true, "vendor": true,
		"node_modules": true, ".gemini": true, ".codex": true, ".agents": true, ".github": true,
		"bruno": true, ".joss": true, "tests": true, "test": true,
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
		return nil, nil, err
	}

	// 2. Prune dead Joss files and methods from the VFS map if reachability was computed
	if reachGraph != nil {
		prunedUnits := FilterProjectFiles(allDiscoveredFiles, reachGraph, units, bCfg.PruneMethods)
		for filePath := range files {
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
			} else if strings.HasSuffix(filePath, ".jp") {
				// Tree-shake JP plugin archive based on reachGraph.LivePluginSymbols
				baseName := strings.TrimSuffix(filepath.Base(filePath), ".jp")
				if usedSyms, hasUsed := reachGraph.LivePluginSymbols[baseName]; hasUsed && len(usedSyms) > 0 {
					if prunedJP, didPrune, pruneErr := pluginpkg.PruneArchive(files[filePath], usedSyms); pruneErr == nil && didPrune {
						files[filePath] = prunedJP
					}
				}
			}
		}
	}

	fmt.Printf("⚡ %s\n", i18n.Tr("nativeBuildPrecompiledFiles", i18n.M{"count": compiledCount}))
	encryptProjectEnvironment(files, enableGUI)

	if reachGraph != nil {
		manifest := buildmanifest.GenerateManifest(
			"main.joss", bCfg.Mode, runtime.GOOS, runtime.GOARCH, bCfg.Profile,
			filepath.Join("build", "app"), allDiscoveredFiles, reachGraph, 0,
		)
		_ = manifest.SaveToFile(filepath.Join(".joss", "cache", "build-manifest.json"))
	}

	capsMap := make(map[string]bool)
	if reachGraph != nil {
		for cap, active := range reachGraph.RuntimeCapabilities {
			capsMap[string(cap)] = active
		}
	}
	if _, err := os.Stat("routes.joss"); err == nil {
		capsMap["server"] = true
		capsMap["http"] = true
	}
	if _, err := os.Stat("api.joss"); err == nil {
		capsMap["server"] = true
		capsMap["http"] = true
	}

	envDetect := core.DetectAndLoadEnv(nil)
	envMap := envDetect.EnvMap
	dbDriver := strings.ToLower(strings.TrimSpace(envMap["DB"]))
	switch dbDriver {
	case "mysql":
		capsMap["mysql"] = true
		capsMap["sqlite"] = false
		capsMap["postgres"] = false
		capsMap["mssql"] = false
		capsMap["database"] = true
	case "sqlite", "sqlite3":
		capsMap["sqlite"] = true
		capsMap["mysql"] = false
		capsMap["postgres"] = false
		capsMap["mssql"] = false
		capsMap["database"] = true
	case "postgres", "postgresql", "pgx":
		capsMap["postgres"] = true
		capsMap["sqlite"] = false
		capsMap["mysql"] = false
		capsMap["mssql"] = false
		capsMap["database"] = true
	case "sqlserver", "mssql", "mssqlserver":
		capsMap["mssql"] = true
		capsMap["sqlite"] = false
		capsMap["mysql"] = false
		capsMap["postgres"] = false
		capsMap["database"] = true
	}
	if strings.EqualFold(strings.TrimSpace(envMap["STORAGE"]), "OCI") {
		capsMap["oci"] = true
	}
	if strings.EqualFold(strings.TrimSpace(envMap["SESSION_DRIVER"]), "redis") || strings.TrimSpace(envMap["REDIS_HOST"]) != "" || strings.TrimSpace(envMap["REDIS_URL"]) != "" {
		capsMap["redis"] = true
	}
	if capsMap["database"] && !capsMap["mysql"] && !capsMap["sqlite"] && !capsMap["postgres"] && !capsMap["mssql"] {
		if _, err := os.Stat(sqliteDbFile); err == nil {
			capsMap["sqlite"] = true
		} else {
			capsMap["mysql"] = true
		}
	}

	return files, capsMap, nil
}

func collectAndEncryptAssets(enableGUI bool) ([]byte, []byte, map[string]bool, error) {
	files, capsMap, err := collectProjectFiles(enableGUI)
	if err != nil {
		return nil, nil, nil, err
	}
	buildKey := make([]byte, 32)
	if _, err := rand.Read(buildKey); err != nil {
		return nil, nil, nil, err
	}
	var compBuf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&compBuf, gzip.BestCompression)
	if err != nil {
		return nil, nil, nil, err
	}
	enc := gob.NewEncoder(gw)
	if err := enc.Encode(files); err != nil {
		return nil, nil, nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, nil, nil, err
	}
	encryptedAssets, err := crypto.EncryptAES(compBuf.Bytes(), buildKey)
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
	} else if strings.HasSuffix(name, ".png") && len(data) > 150*1024 {
		if optPNG, ok := optimizeLargePNG(data, 720); ok && len(optPNG) < len(data) {
			data = optPNG
		}
	}

	files[relPath] = data
	return nil
}

func shouldSkipFileByName(name string) bool {
	return strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".log") ||
		strings.HasSuffix(name, ".enc") || strings.HasSuffix(name, ".dll") ||
		strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dylib") ||
		strings.HasSuffix(name, ".scss") || strings.HasSuffix(name, ".map") ||
		strings.HasSuffix(name, ".sqlite") || strings.HasSuffix(name, ".sqlite-shm") || strings.HasSuffix(name, ".sqlite-wal") ||
		name == "runner" || name == "joss" ||
		name == ".env" || name == "env.joss" || name == ".env.joss" ||
		name == "package-lock.json" || name == "dockerfile" || name == ".dockerignore" ||
		name == ".gitignore" || name == "agents.md" || name == "entrypoint.sh" ||
		name == "translation_history.json"
}

func optimizeLargePNG(data []byte, maxDim int) ([]byte, bool) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxDim && h <= maxDim {
		var buf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, img); err == nil {
			return buf.Bytes(), true
		}
		return nil, false
	}
	newW, newH := w, h
	if w > h {
		newW = maxDim
		newH = (h * maxDim) / w
	} else {
		newH = maxDim
		newW = (w * maxDim) / h
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + (y*h)/newH
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + (x*w)/newW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, dst); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
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
	contentStr := string(data)

	if !strings.Contains(contentStr, "JWT_SECRET=") || strings.Contains(contentStr, "joss_default_secret") {
		keyBytes := make([]byte, 32)
		_, _ = rand.Read(keyBytes)
		data = append(data, []byte(fmt.Sprintf("\nJWT_SECRET=\"%x\"", keyBytes))...)
	}
	if !strings.Contains(contentStr, "APP_KEY=") || strings.Contains(contentStr, "joss_default_secret") {
		keyBytes := make([]byte, 32)
		_, _ = rand.Read(keyBytes)
		data = append(data, []byte(fmt.Sprintf("\nAPP_KEY=\"%x\"", keyBytes))...)
	}

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

func computePruningTags(detectedCaps map[string]bool) []string {
	var tags []string
	if detectedCaps != nil {
		if !detectedCaps["database"] && !detectedCaps["sqlite"] && !detectedCaps["mysql"] && !detectedCaps["postgres"] && !detectedCaps["mssql"] {
			tags = append(tags, "nodb")
		} else {
			if !detectedCaps["mysql"] {
				tags = append(tags, "nomysql")
			}
			if !detectedCaps["sqlite"] {
				tags = append(tags, "nosqlite")
			}
			if !detectedCaps["postgres"] {
				tags = append(tags, "nopostgres")
			}
			if !detectedCaps["mssql"] {
				tags = append(tags, "nomssql")
			}
		}
		if !detectedCaps["oci"] {
			tags = append(tags, "nooci")
		}
		if !detectedCaps["redis"] {
			tags = append(tags, "noredis")
		}
		if !detectedCaps["gui"] {
			tags = append(tags, "nogui")
		}
	}
	return tags
}

func findRepoRoot() string {
	for check := "."; ; check = filepath.Join("..", check) {
		absCheck, absErr := filepath.Abs(check)
		if absErr != nil {
			break
		}
		if _, statErr := os.Stat(filepath.Join(absCheck, "pkg", "backend", "native", "abi", "host_bridge.go")); statErr == nil {
			return absCheck
		}
		parent := filepath.Dir(absCheck)
		if parent == absCheck {
			break
		}
	}
	return ""
}

func compileRunnerBinary(outExe string, target native.Target, detectedCaps map[string]bool) error {
	absOutExe, err := filepath.Abs(outExe)
	if err != nil {
		absOutExe = outExe
	}

	tags := computePruningTags(detectedCaps)
	sort.Strings(tags)
	tagKey := strings.Join(tags, "_")
	if tagKey == "" {
		tagKey = "default"
	}

	userHome, _ := os.UserHomeDir()
	cacheDir := filepath.Join(userHome, ".joss", "cache", "host_runner")
	cachedRunner := filepath.Join(cacheDir, fmt.Sprintf("runner_%s_%s_%s", target.OS, target.Arch, tagKey))

	// 1. Check user cache
	if fi, statErr := os.Stat(cachedRunner); statErr == nil && fi.Size() > 0 {
		if copyErr := copyFile(cachedRunner, absOutExe); copyErr == nil {
			_ = os.Chmod(absOutExe, 0755)
			fmt.Printf("⚡ Host runner reutilizado desde caché nativa (%s/%s, tags: %s)\n", target.OS, target.Arch, tagKey)
			return nil
		}
	}

	// 2. Compile host runner using Go toolchain if available
	repoRoot := findRepoRoot()
	hasGo := false
	if _, lookErr := exec.LookPath("go"); lookErr == nil {
		hasGo = true
	}

	if repoRoot != "" && hasGo {
		fmt.Printf("⚡ Compilando host runner autónomo para %s/%s (%s)...\n", target.OS, target.Arch, tagKey)
		args := []string{"build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w"}
		if len(tags) > 0 {
			args = append(args, "-tags="+strings.Join(tags, ","))
		}
		args = append(args, "-o", absOutExe, "./pkg/backend/native/abi/")

		cmd := exec.Command("go", args...)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"GOOS="+target.OS,
			"GOARCH="+target.Arch,
		)

		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("error compilando host runner: %v\n%s", err, string(out))
		}

		_ = os.Chmod(absOutExe, 0755)

		// 3. Compress with UPX if available
		if upx := resolveOrDownloadUPX(); upx != "" {
			fmt.Printf("⚡ Optimizando host runner con UPX...\n")
			upxCmd := exec.Command(upx, "-9", absOutExe)
			_ = upxCmd.Run()
		}

		// 4. Cache the resulting runner for subsequent instant builds
		_ = os.MkdirAll(cacheDir, 0755)
		_ = copyFile(absOutExe, cachedRunner)

		return nil
	}

	// 3. Si no hay toolchain de Go ni código fuente, pero estamos en la misma plataforma (ej. contenedor Docker o instalación binaria)
	if target.OS == runtime.GOOS && target.Arch == runtime.GOARCH {
		selfExe, selfErr := os.Executable()
		if selfErr == nil && selfExe != "" {
			if copyErr := copyFile(selfExe, absOutExe); copyErr == nil {
				_ = os.Chmod(absOutExe, 0755)
				fmt.Printf("⚡ Host runner autónomo reutilizado desde la instalación local de Joss (%s/%s)\n", target.OS, target.Arch)
				if upx := resolveOrDownloadUPX(); upx != "" {
					upxCmd := exec.Command(upx, "-9", absOutExe)
					_ = upxCmd.Run()
				}
				// Cachear para siguientes ejecuciones
				_ = os.MkdirAll(cacheDir, 0755)
				_ = copyFile(absOutExe, cachedRunner)
				return nil
			}
		}
		return fmt.Errorf("no se pudo copiar el ejecutable actual de Joss como runner: %v", selfErr)
	}

	return fmt.Errorf("se requiere 'go' en el PATH y el repositorio de Joss para compilar un host runner cruzado para %s/%s", target.OS, target.Arch)
}

func appendSealedAssetsToExecutable(outExe string, enableGUI bool, target native.Target) (int64, error) {
	if _, err := os.Stat("assets/css"); err == nil {
		server.CompileStyles()
	}

	fmt.Println(i18n.Tr("nativeBuildPackagingAssets"))
	encryptedAssets, buildKey, detectedCaps, err := collectAndEncryptAssets(enableGUI)
	if err != nil {
		return 0, err
	}

	outDir := filepath.Dir(outExe)
	if outDir != "" && outDir != "." {
		_ = os.MkdirAll(filepath.Join(outDir, "Storage"), 0755)
		copyDatabaseFiles(outDir)
	}

	if target.OS == "windows" {
		if outDir != "" && outDir != "." {
			optimizeHostBridgeDLL(outDir, detectedCaps)
		}
	} else {
		if err := compileRunnerBinary(outExe, target, detectedCaps); err != nil {
			return 0, err
		}
	}

	outFile, err := os.OpenFile(outExe, os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	if _, err := outFile.Write(encryptedAssets); err != nil {
		return 0, err
	}
	if _, err := outFile.Write(buildKey); err != nil {
		return 0, err
	}

	lenBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenBuf, uint64(len(encryptedAssets)))
	if _, err := outFile.Write(lenBuf); err != nil {
		return 0, err
	}

	magic := []byte("JOSS_RUNNER_DATA")
	if _, err := outFile.Write(magic); err != nil {
		return 0, err
	}

	_ = outFile.Close()
	if stat, err := os.Stat(outExe); err == nil {
		return stat.Size(), nil
	}
	return 0, nil
}

func optimizeHostBridgeDLL(outDir string, detectedCaps map[string]bool) {
	if runtime.GOOS != "windows" {
		return
	}
	targetDll := filepath.Join(outDir, "joss_host.dll")
	absTargetDll, err := filepath.Abs(targetDll)
	if err != nil {
		absTargetDll = targetDll
	}

	tags := computePruningTags(detectedCaps)
	repoRoot := findRepoRoot()

	// 1. Fast path: Check user cache or repo prebuilt DLL to avoid 20-30s recompilation + UPX
	sort.Strings(tags)
	tagKey := strings.Join(tags, "_")
	if tagKey == "" {
		tagKey = "default"
	}

	userHome, _ := os.UserHomeDir()
	cacheDir := filepath.Join(userHome, ".joss", "cache", "host_bridge")
	cachedDLL := filepath.Join(cacheDir, "joss_host_"+tagKey+".dll")

	if fi, statErr := os.Stat(cachedDLL); statErr == nil && fi.Size() > 0 {
		// Auto-heal: If cached DLL is larger than 12MB, it was stored without UPX compression
		if fi.Size() > 12*1024*1024 {
			if upxPath := resolveOrDownloadUPX(); upxPath != "" {
				fmt.Printf("⚡ Optimizando host bridge en caché con UPX...\n")
				upxCmd := exec.Command(upxPath, "-9", cachedDLL)
				_ = upxCmd.Run()
			}
		}
		if newFi, statErr2 := os.Stat(cachedDLL); statErr2 == nil && newFi.Size() < 12*1024*1024 {
			if copyErr := copyFile(cachedDLL, absTargetDll); copyErr == nil {
				fmt.Printf("⚡ Host bridge reutilizado desde caché nativa (%s)\n", strings.Join(tags, ", "))
				return
			}
		}
	}

	// Fallback to prebuilt template in repo ONLY if tags match default (no pruned features requested)
	if repoRoot != "" && tagKey == "default" {
		repoPrebuilt := filepath.Join(repoRoot, "pkg", "backend", "native", "abi", "build", "joss_host.dll")
		if fi, statErr := os.Stat(repoPrebuilt); statErr == nil && fi.Size() > 0 {
			if copyErr := copyFile(repoPrebuilt, absTargetDll); copyErr == nil {
				if fi.Size() > 12*1024*1024 {
					if upxPath := resolveOrDownloadUPX(); upxPath != "" {
						fmt.Printf("⚡ Optimizando host bridge con UPX...\n")
						upxCmd := exec.Command(upxPath, "-9", absTargetDll)
						_ = upxCmd.Run()
					}
				}
				fmt.Printf("⚡ Host bridge desplegado desde runtime nativo preconstruido (%s)\n", strings.Join(tags, ", "))
				// Cache it for future instant builds
				_ = os.MkdirAll(cacheDir, 0755)
				_ = copyFile(absTargetDll, cachedDLL)
				return
			}
		}
	}

	// 2. Slow path: Recompile with go build and UPX only when strictly necessary
	if repoRoot != "" {
		if _, lookErr := exec.LookPath("go"); lookErr == nil {
			args := []string{"build", "-buildmode=c-shared", "-trimpath", "-buildvcs=false", "-ldflags=-s -w"}
			if len(tags) > 0 {
				args = append(args, "-tags="+strings.Join(tags, ","))
			}
			args = append(args, "-o", absTargetDll, "./pkg/backend/native/abi/")
			cmd := exec.Command("go", args...)
			cmd.Dir = repoRoot
			if err := cmd.Run(); err == nil {
				_ = os.Remove(strings.TrimSuffix(absTargetDll, ".dll") + ".h")
				fmt.Printf("⚡ Host bridge compilado con poda selectiva de dependencias (%s)\n", strings.Join(tags, ", "))
			}
		}
	}

	if upxPath := resolveOrDownloadUPX(); upxPath != "" {
		fmt.Printf("⚡ Optimizando host bridge con UPX...\n")
		upxCmd := exec.Command(upxPath, "-9", absTargetDll)
		_ = upxCmd.Run()
	}

	// Save to cache for subsequent builds
	_ = os.MkdirAll(cacheDir, 0755)
	_ = copyFile(absTargetDll, cachedDLL)
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

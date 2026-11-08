package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jossecurity/joss/pkg/buildmanifest"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
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
		if !detectedCaps["redis"] {
			tags = append(tags, "noredis")
		}
		if !detectedCaps["gui"] {
			tags = append(tags, "nogui")
		}
		if !detectedCaps["embedlocales"] {
			tags = append(tags, "noembedlocales")
		}
	}
	return tags
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

package server

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
)

var (
	hotReloadUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	hotReloadClients   = make(map[*websocket.Conn]bool)
	hotReloadClientsMu sync.Mutex
)

func hotReloadHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := hotReloadUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	hotReloadClientsMu.Lock()
	hotReloadClients[conn] = true
	hotReloadClientsMu.Unlock()

	// Keep connection alive and handle close
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			hotReloadClientsMu.Lock()
			delete(hotReloadClients, conn)
			hotReloadClientsMu.Unlock()
			conn.Close()
			break
		}
	}
}

func notifyClientsTyped(msgType string, filePath string) {
	hotReloadClientsMu.Lock()
	defer hotReloadClientsMu.Unlock()

	payload := fmt.Sprintf(`{"type":"%s","file":%q}`, msgType, filePath)
	for conn := range hotReloadClients {
		err := conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			conn.Close()
			delete(hotReloadClients, conn)
		}
	}
}

func notifyClients() {
	notifyClientsTyped("soft_reload", "")
}

func shouldSkipDir(name string) bool {
	switch name {
	case "node_modules", ".git", "vendor", "build", "dist", ".system_generated", ".cache", ".turbo", ".next", ".idea", ".vscode", "tmp", "temp", "scratch":
		return true
	default:
		return false
	}
}

func watchChanges() {
	// If using VFS (Production), disable hot reload
	if GlobalFileSystem != nil {
		return
	}

	// Store file hashes: path -> hash
	fileHashes := make(map[string]string)
	var lastNodeModulesHash string
	if nmInfo, err := os.Stat("node_modules"); err == nil {
		lastNodeModulesHash = nmInfo.ModTime().String()
	}

	// Helper to calculate hash
	getHash := func(path string) string {
		content, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		// Simple MD5 hash
		hash := md5.Sum(content)
		return hex.EncodeToString(hash[:])
	}

	// Initial scan (skipping node_modules and other ignored dirs)
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".joss" || ext == ".html" || ext == ".css" || ext == ".js" || ext == ".scss" || ext == ".arb" || filepath.Base(path) == "package.json" || filepath.Base(path) == ".env" || filepath.Base(path) == "env.joss" {
			fileHashes[path] = getHash(path)
		}
		return nil
	})

	for {
		time.Sleep(500 * time.Millisecond)

		// Check node_modules separately (lightweight)
		if nmInfo, err := os.Stat("node_modules"); err == nil {
			currentNMHash := nmInfo.ModTime().String()
			if lastNodeModulesHash != "" && currentNMHash != lastNodeModulesHash {
				fmt.Println("[HotReload] 'node_modules' changed. Rescanning assets...")
				core.GetAssetManager().ScanNodeModules()
				notifyClientsTyped("css", "node_modules")
			}
			lastNodeModulesHash = currentNMHash
		} else if lastNodeModulesHash != "" {
			// It existed, now it doesn't (Deleted)
			fmt.Println("[HotReload] 'node_modules' deleted. Clearing assets...")
			core.GetAssetManager().ScanNodeModules() // Will clear map
			notifyClientsTyped("css", "node_modules")
			lastNodeModulesHash = ""
		}

		var changedPaths []string
		walkedFiles := make(map[string]bool)
		err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			if info.IsDir() {
				if shouldSkipDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			ext := filepath.Ext(path)
			if ext == ".joss" || ext == ".html" || ext == ".css" || ext == ".js" || ext == ".scss" || ext == ".arb" || filepath.Base(path) == "package.json" {
				walkedFiles[path] = true
				currentHash := getHash(path)
				if lastHash, ok := fileHashes[path]; ok {
					if currentHash != lastHash {
						fileHashes[path] = currentHash
						changedPaths = append(changedPaths, path)
						fmt.Printf("[HotReload] Cambio detectado en: %s\n", path)
					}
				} else {
					// New file
					fileHashes[path] = currentHash
					changedPaths = append(changedPaths, path)
					fmt.Printf("[HotReload] Nuevo archivo detectado: %s\n", path)
				}
			} else if filepath.Base(path) == ".env" || filepath.Base(path) == "env.joss" {
				walkedFiles[path] = true
				currentHash := getHash(path)
				if lastHash, ok := fileHashes[path]; ok {
					if currentHash != lastHash {
						fileHashes[path] = currentHash
						changedPaths = append(changedPaths, path)
						fmt.Printf("[HotReload] Cambio detectado en entorno: %s\n", path)
					}
				} else {
					fileHashes[path] = currentHash
					changedPaths = append(changedPaths, path)
					fmt.Printf("[HotReload] Nuevo archivo entorno detectado: %s\n", path)
				}
			}
			return nil
		})
		_ = err // Walk errors are handled per-path above

		// Detect deleted files: anything in fileHashes not visited this walk was deleted
		for path := range fileHashes {
			if !walkedFiles[path] {
				delete(fileHashes, path)
				fmt.Printf("[HotReload] Archivo eliminado detectado: %s — recargando...\n", path)
				changedPaths = append(changedPaths, path)
			}
		}

		if len(changedPaths) > 0 {
			// Debounce
			time.Sleep(100 * time.Millisecond)
			for _, p := range changedPaths {
				reloadApp(p)
			}
		}
	}
}

func reloadApp(changedFile string) {
	mutex.Lock()
	defer mutex.Unlock()

	if changedFile == "" {
		fmt.Println("Recargando aplicación completa...")
	} else {
		fmt.Printf("Recargando parcial: %s\n", changedFile)
	}

	// 1. Styles
	if changedFile == "" || strings.HasSuffix(changedFile, ".scss") || strings.HasSuffix(changedFile, ".css") {
		compileStyles()
		if strings.HasSuffix(changedFile, ".scss") || strings.HasSuffix(changedFile, ".css") {
			notifyClientsTyped("css", changedFile)
			return
		}
	}

	// 1.5 Package.json (Node Modules)
	if strings.HasSuffix(changedFile, "package.json") {
		fmt.Println("[HotReload] Detectado cambio en dependencias (package.json). Escaneando assets...")
		am := core.GetAssetManager()
		am.ScanNodeModules()
		notifyClientsTyped("css", changedFile)
		return
	}

	// 2. Views (HTML)
	if strings.HasSuffix(changedFile, ".html") {
		// Views are read from disk, so just notify soft reload
		notifyClientsTyped("soft_reload", changedFile)
		return
	}

	// 2.5 I18n (.arb)
	if strings.HasSuffix(changedFile, ".arb") {
		fmt.Println("[HotReload] Translations changed. Reloading I18n...")
		i18n.GlobalManager.Load(GlobalFileSystem)
		notifyClientsTyped("soft_reload", changedFile)
		return
	}

	if changedFile == "" || strings.HasSuffix(changedFile, ".joss") {
		if reloadPreparedJossRuntime() {
			notifyClients()
		}
		return
	}

	// 3. Runtime Logic
	if currentRuntime == nil {
		currentRuntime = core.NewRuntime()
		currentRuntime.LoadEnv(GlobalFileSystem)

		// Init Redis if configured
		if currentRuntime.Env["SESSION_DRIVER"] == "redis" {
			if err := initializeRedisSessions(currentRuntime.Env); err != nil {
				fmt.Printf("[Security] %v\n", err)
			} else {
				fmt.Println("[Security] Redis conectado para sesiones")
			}
		}
	}

	// Helper to load a single file
	loadFile := func(path string) {
		fmt.Printf("[HotReload] Loading file: %s\n", path)
		content, err := vfsReadFile(path)
		if err != nil {
			fmt.Printf("[HotReload] Error leyendo %s: %v\n", path, err)
			return
		}
		l := parser.NewLexer(string(content))
		p := parser.NewParser(l)
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			fmt.Printf("[HotReload] Errores de parseo en %s — no se ejecutará código roto:\n", path)
			for _, msg := range p.Errors() {
				fmt.Printf("\t%s\n", msg)
			}
			return // DO NOT execute a broken AST
		}
		currentRuntime.Execute(program)
	}

	if changedFile != "" && (strings.HasSuffix(changedFile, "env.joss") || strings.HasSuffix(changedFile, ".env")) {
		fmt.Println("Recargando entorno...")
		if currentRuntime != nil {
			currentRuntime.LoadEnv(GlobalFileSystem)
			// Re-init Redis if configuration changed
			if currentRuntime.Env["SESSION_DRIVER"] == "redis" {
				if err := initializeRedisSessions(currentRuntime.Env); err != nil {
					fmt.Printf("[Security] %v\n", err)
				}
			}
		}
		notifyClients()
		return
	}

	if changedFile != "" && strings.HasSuffix(changedFile, ".joss") {
		if strings.HasSuffix(changedFile, "routes.joss") {
			// Reload Routes
			fmt.Println("[DEBUG] Reloading routes...")
			// Clear existing routes? Router overwrites, so it's okay.
			// Ideally we should clear, but Runtime doesn't expose a ClearRoutes method.
			// We can manually clear if we want, but overwriting is fine for now.
			if currentRuntime.Routes != nil {
				// Optional: clear routes to remove deleted ones
				currentRuntime.Routes = make(map[string]map[string]interface{})
			}
			currentRuntime.CurrentSource = "routes"
			loadFile(changedFile)
		} else {
			// Reload Controller/Model
			loadFile(changedFile)
		}
		notifyClients()
		return
	}

	// Full Reload (Initial or unknown change)
	if changedFile == "" {
		// Reset Runtime
		if currentRuntime != nil {
			currentRuntime.Free()
		}
		currentRuntime = core.NewRuntime()
		currentRuntime.LoadEnv(GlobalFileSystem)

		// Init Redis if configured
		if currentRuntime.Env["SESSION_DRIVER"] == "redis" {
			if err := initializeRedisSessions(currentRuntime.Env); err != nil {
				fmt.Printf("[Security] %v\n", err)
			} else {
				fmt.Println("[Security] Redis conectado para sesiones")
			}
		}

		// Load App Files
		walkFn := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".joss") {
				loadFile(path)
			}
			return nil
		}

		var err error
		if GlobalFileSystem != nil {
			err = vfsWalk(GlobalFileSystem, "app", walkFn)
		} else {
			err = filepath.Walk("app", walkFn)
		}

		if err != nil {
			fmt.Printf("[DEBUG] Error walking app directory: %v\n", err)
		}

		// Load Routes (routes.joss)
		routesPath := "routes.joss"
		if existsFile(routesPath) {
			currentRuntime.CurrentSource = "routes"
			loadFile(routesPath)
		} else {
			fmt.Println("[DEBUG] routes.joss not found")
		}

		// Load API Routes (api.joss)
		apiRoutesPath := "api.joss"
		if existsFile(apiRoutesPath) {
			currentRuntime.CurrentSource = "api"
			loadFile(apiRoutesPath)
		}

		// Load Cron Configuration (config/cron.joss)
		cronPath := filepath.Join("config", "cron.joss")
		if existsFile(cronPath) {
			currentRuntime.CurrentSource = "cron"
			loadFile(cronPath)
		}

		notifyClients()
	}
}

func reloadPreparedJossRuntime() bool {
	report := prepareHotReloadProject()
	if report.HasIssues() {
		report.PrintReport()
	}
	if report.HasErrors() || report.Prepared == nil {
		fmt.Println("[HotReload] Recarga rechazada; el runtime anterior permanece activo.")
		return false
	}

	candidate := core.NewRuntime()
	candidate.LoadEnv(GlobalFileSystem)
	succeeded := false
	defer func() {
		if !succeeded {
			candidate.Free()
		}
	}()
	if candidate.Env["SESSION_DRIVER"] == "redis" {
		if err := initializeRedisSessions(candidate.Env); err != nil {
			fmt.Printf("[Security] %v\n", err)
			return false
		}
	}
	for _, unit := range report.Prepared.Units {
		candidate.CurrentSource = unit.Path
		completed := func() (ok bool) {
			defer func() {
				if recovered := recover(); recovered != nil {
					fmt.Printf("[HotReload] Error ejecutando %s: %v\n", unit.Path, recovered)
				}
			}()
			candidate.Execute(unit.Program)
			return true
		}()
		if !completed {
			fmt.Println("[HotReload] Recarga abortada; el runtime anterior permanece activo.")
			return false
		}
	}

	previous := currentRuntime
	currentRuntime = candidate
	succeeded = true
	if previous != nil {
		previous.Free()
	}
	return true
}

func prepareHotReloadProject() *core.AnalysisReport {
	paths := make([]string, 0)
	seen := make(map[string]bool)
	add := func(sourcePath string) {
		normalized := filepath.ToSlash(sourcePath)
		if !seen[normalized] {
			seen[normalized] = true
			paths = append(paths, normalized)
		}
	}
	walkFn := func(sourcePath string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && parser.IsJossSourceFile(sourcePath) {
			add(sourcePath)
		}
		return nil
	}
	if GlobalFileSystem != nil {
		_ = vfsWalk(GlobalFileSystem, "app", walkFn)
	} else {
		_ = filepath.Walk("app", walkFn)
	}
	for _, sourcePath := range []string{"routes.joss", "api.joss", "config/cron.joss"} {
		if existsFile(sourcePath) {
			add(sourcePath)
		}
	}
	sort.Strings(paths)

	units := make([]semanticanalyzer.SourceUnit, 0, len(paths))
	parseIssues := make([]diagnostics.Diagnostic, 0)
	for _, sourcePath := range paths {
		content, err := vfsReadFile(sourcePath)
		if err != nil {
			parseIssues = append(parseIssues, diagnostics.Diagnostic{Code: "JOSS-IO-001", Severity: diagnostics.SeverityError, File: sourcePath, Message: err.Error()})
			continue
		}
		p := parser.NewParser(parser.NewLexer(string(content)))
		program := p.ParseProgram()
		if items := p.Diagnostics(); len(items) > 0 {
			for _, item := range items {
				item.File = sourcePath
				parseIssues = append(parseIssues, item)
			}
			continue
		}
		units = append(units, semanticanalyzer.SourceUnit{Path: sourcePath, Program: program})
	}
	if len(parseIssues) > 0 {
		return core.AnalysisReportFromDiagnostics(parseIssues)
	}
	return core.AnalyzeSourceUnits(units)
}

func existsFile(path string) bool {
	if GlobalFileSystem != nil {
		f, err := GlobalFileSystem.Open(path)
		if err == nil {
			f.Close()
			return true
		}
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// Helpers for VFS support

func vfsReadFile(pathStr string) ([]byte, error) {
	if GlobalFileSystem != nil {
		// Ensure path is slash-separated
		pathStr = filepath.ToSlash(pathStr)
		f, err := GlobalFileSystem.Open(pathStr)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		// Get size
		stat, err := f.Stat()
		if err != nil {
			return nil, err
		}

		data := make([]byte, stat.Size())
		_, err = f.Read(data)
		return data, err
	}
	return os.ReadFile(pathStr)
}

func vfsWalk(fs http.FileSystem, root string, walkFn filepath.WalkFunc) error {
	f, err := fs.Open(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return walkFn(root, nil, err)
	}

	if err := walkFn(root, info, nil); err != nil {
		return err
	}

	if !info.IsDir() {
		return nil
	}

	// It's a directory, read children
	// http.File doesn't strictly support Readdir unless it's an os.File or similar.
	// But our MemFS implements Readdir.
	// We need to type assert or assume it supports Readdir if it's a directory.
	// Standard http.File interface includes Readdir.

	// However, http.File.Readdir returns []os.FileInfo
	dirs, err := f.Readdir(-1)
	if err != nil {
		return walkFn(root, info, err)
	}

	for _, d := range dirs {
		// Use path.Join for VFS paths (always forward slashes)
		childPath := path.Join(root, d.Name())
		if err := vfsWalk(fs, childPath, walkFn); err != nil {
			return err
		}
	}
	return nil
}

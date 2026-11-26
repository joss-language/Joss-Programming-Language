package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/mattn/go-isatty"
)

func isInteractiveTerminal() bool {
	if os.Getenv("CI") != "" || os.Getenv("JOSS_NO_WATCH") == "1" || os.Getenv("JOSS_WATCH_CHILD") == "1" || os.Getenv("JOSS_SERVER_MODE") == "1" {
		return false
	}
	inTTY := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	outTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	return inTTY && outTTY
}

// hotReloadHub broadcasts SSE hot-reload events ("css" or "morph") to connected
// browsers while the supervised native server restarts or compiles styles.
type hotReloadHub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
	url     string
	srv     *http.Server
}

func startHotReloadHub() (*hotReloadHub, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	hub := &hotReloadHub{
		clients: make(map[chan string]struct{}),
		url:     fmt.Sprintf("http://%s/__hot_reload_events", ln.Addr().String()),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/__hot_reload_events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		ch := make(chan string, 8)
		hub.mu.Lock()
		hub.clients[ch] = struct{}{}
		hub.mu.Unlock()
		defer func() {
			hub.mu.Lock()
			delete(hub.clients, ch)
			hub.mu.Unlock()
		}()
		_, _ = fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()
		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-ch:
				_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			}
		}
	})
	hub.srv = &http.Server{Handler: mux}
	go func() { _ = hub.srv.Serve(ln) }()
	return hub, nil
}

func (h *hotReloadHub) Broadcast(eventType string) {
	if h == nil {
		return
	}
	payload := fmt.Sprintf(`{"type":%q}`, eventType)
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- payload:
		default:
		}
	}
}

func (h *hotReloadHub) Close() {
	if h == nil || h.srv == nil {
		return
	}
	_ = h.srv.Close()
}

func resolveProjectPort() string {
	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		return p
	}
	envRes := core.DetectAndLoadEnv(nil)
	if p := strings.TrimSpace(envRes.EnvMap["PORT"]); p != "" {
		return p
	}
	if p := strings.TrimSpace(envRes.EnvMap["JOSS_PORT"]); p != "" {
		return p
	}
	return "8000"
}

func waitForTCPPort(port string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort("127.0.0.1", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

var ignoredWatchDirs = map[string]bool{
	".git":         true,
	".vscode":      true,
	".idea":        true,
	".gemini":      true,
	".codex":       true,
	".agents":      true,
	".github":      true,
	".joss":        true,
	"build":        true,
	"dist":         true,
	"vendor":       true,
	"node_modules": true,
	"bruno":        true,
	"tests":        true,
	"test":         true,
	"Storage":      true,
	"storage":      true,
	"public":       true,
}

func isWatchableFile(path string) bool {
	base := filepath.Base(path)
	if base == "env.joss" || base == ".env" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".joss" || ext == ".html" || ext == ".css" || ext == ".scss" || ext == ".json"
}

func takeWatchSnapshot(root string) map[string]time.Time {
	snapshot := make(map[string]time.Time)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == "." {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if ignoredWatchDirs[base] {
				return filepath.SkipDir
			}
			return nil
		}
		if isWatchableFile(path) {
			snapshot[path] = info.ModTime()
		}
		return nil
	})
	return snapshot
}

func detectWatchChange(root string, prevSnapshot map[string]time.Time) (string, map[string]time.Time, bool) {
	current := takeWatchSnapshot(root)
	// Check for modified or new files
	for path, modTime := range current {
		prevTime, exists := prevSnapshot[path]
		if !exists {
			return path, current, true
		}
		if modTime.After(prevTime) {
			return path, current, true
		}
	}
	// Check for deleted files
	for path := range prevSnapshot {
		if _, exists := current[path]; !exists {
			return path, current, true
		}
	}
	return "", prevSnapshot, false
}

type managedServerChild struct {
	cmd  *exec.Cmd
	done chan error
}

func startManagedServerChild(cmd *exec.Cmd) (*managedServerChild, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	child := &managedServerChild{cmd: cmd, done: make(chan error, 1)}
	go func() { child.done <- cmd.Wait() }()
	return child, nil
}

func stopManagedServerChild(child *managedServerChild) {
	if child == nil {
		return
	}
	killChildProcess(child.cmd)
	<-child.done
}

func killChildProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		_ = killCmd.Run()
	}
	_ = cmd.Process.Kill()
}

// handleServerStart supervises the selected runtime and keeps its mode across
// child restarts during hot reload.
func handleServerStart(args []string) {
	if _, err := os.Stat("main.joss"); os.IsNotExist(err) {
		fmt.Println(i18n.Tr("cliErrNoMain"))
		fmt.Println("  main.joss")
		fmt.Println(i18n.Tr("cliErrRequireMain"))
		os.Exit(1)
	}

	mode, args := takeExecutionMode(args)
	noWatch := false
	gui := false
	var extraArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--no-watch" || arg == "--nowatch" {
			noWatch = true
		} else if arg == "--gui" {
			gui = true
		} else if strings.HasPrefix(arg, "--port=") {
			portVal := strings.TrimPrefix(arg, "--port=")
			_ = os.Setenv("PORT", portVal)
		} else if (arg == "-p" || arg == "--port") && i+1 < len(args) {
			_ = os.Setenv("PORT", args[i+1])
			i++
		} else {
			extraArgs = append(extraArgs, arg)
		}
	}

	runServerSupervisor("main.joss", "", gui, extraArgs, noWatch, mode)
}

func runServerSupervisor(entryFile, outExe string, enableGUI bool, extraArgs []string, noWatch bool, mode executionMode) {
	selfExe, err := os.Executable()
	if err != nil || selfExe == "" {
		selfExe = "joss"
	}

	if noWatch {
		// Run standalone server directly without file watching
		args := serverChildArgs(entryFile, extraArgs, mode)
		cmd := exec.Command(selfExe, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = serverChildEnv(enableGUI, mode)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigChan)
		child, err := startManagedServerChild(cmd)
		if err != nil {
			fmt.Printf("❌ Error iniciando servidor: %v\n", err)
			return
		}
		select {
		case err := <-child.done:
			if err == nil {
				return
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		case <-cliShutdownRequested:
			stopManagedServerChild(child)
		case sig := <-sigChan:
			fmt.Printf("\n\n[Joss Server] Deteniendo servidor y procesos asociados (%s)...\n", sig)
			stopManagedServerChild(child)
		}
		return
	}

	hrHub, hrErr := startHotReloadHub()
	if hrErr == nil {
		defer hrHub.Close()
	}

	var currentChild *managedServerChild

	stopCurrent := func() {
		if currentChild != nil {
			child := currentChild
			currentChild = nil
			stopManagedServerChild(child)
		}
	}

	startChild := func() error {
		args := serverChildArgs(entryFile, extraArgs, mode)
		cmd := exec.Command(selfExe, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		env := serverChildEnv(enableGUI, mode)
		if hrHub != nil {
			env = append(env, "JOSS_HOT_RELOAD_URL="+hrHub.url)
		}
		cmd.Env = env
		child, err := startManagedServerChild(cmd)
		if err != nil {
			return err
		}
		currentChild = child
		return nil
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	defer stopCurrent()

	fmt.Printf("⚡ [Joss Server] Iniciando servidor en modo %s...\n", executionModeName(mode))
	if err := startChild(); err != nil {
		fmt.Printf("❌ Error iniciando servidor: %v\n", err)
		return
	}

	fmt.Println("\n========================================================")
	fmt.Printf("⚡ Joss Server Supervisor (Hot-Reload DOM/CSS %s)\n", strings.ToUpper(executionModeName(mode)))
	fmt.Println("📂 Vigilando cambios en .joss, vistas (.html), estilos (.scss/.css) y entorno")
	fmt.Println("✨ Actualización quirúrgica automática sin F5")
	fmt.Println("💡 Presiona Ctrl+C para detener")
	fmt.Println("========================================================")
	fmt.Println()

	appPort := resolveProjectPort()
	snapshot := takeWatchSnapshot(".")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		var childDone <-chan error
		if currentChild != nil {
			childDone = currentChild.done
		}
		select {
		case <-cliShutdownRequested:
			stopCurrent()
			return
		case sig := <-sigChan:
			fmt.Printf("\n\n[Joss Server] Deteniendo servidor y procesos asociados (%s)...\n", sig)
			stopCurrent()
			return
		case err := <-childDone:
			currentChild = nil
			if err != nil {
				fmt.Printf("\n[Joss Server] El proceso del servidor terminó con error: %v\n", err)
			} else {
				fmt.Println("\n[Joss Server] El proceso del servidor terminó.")
			}
			return
		case <-ticker.C:
			changedFile, newSnapshot, changed := detectWatchChange(".", snapshot)
			if !changed {
				continue
			}
			snapshot = newSnapshot

			// Small debounce to let editor write operations flush
			time.Sleep(80 * time.Millisecond)
			snapshot = takeWatchSnapshot(".")

			relPath, relErr := filepath.Rel(".", changedFile)
			if relErr != nil {
				relPath = changedFile
			}
			lowerPath := strings.ToLower(changedFile)

			// Fast-path: SCSS / CSS updates compile in milliseconds and hot-swap
			// stylesheets in the browser without restarting or recompiling the binary.
			if strings.HasSuffix(lowerPath, ".scss") || strings.HasSuffix(lowerPath, ".css") {
				fmt.Printf("\n🎨 [Hot-Reload CSS] Cambio detectado en: %s\n", relPath)
				if strings.HasSuffix(lowerPath, ".scss") {
					core.CompileStyles()
				}
				hrHub.Broadcast("css")
				fmt.Println("✨ [Hot-Reload CSS] Estilos actualizados en vivo sin reiniciar el servidor.")
				continue
			}

			fmt.Printf("\n🔄 [Hot-Reload] Cambio detectado en: %s\n", relPath)
			stopCurrent()

			if err := startChild(); err != nil {
				fmt.Printf("❌ [Hot-Reload] Error reiniciando servidor JIT: %v\n", err)
				return
			}
			if waitForTCPPort(appPort, 30*time.Second) {
				hrHub.Broadcast("morph")
				fmt.Println("✨ [Hot-Reload] Vista/lógica aplicada en vivo (DOM morphing sin F5).")
			} else {
				fmt.Println("⚠️  [Hot-Reload] El servidor tardó en responder; recarga cuando esté listo.")
			}
			fmt.Println()
		}
	}
}

// runScriptWithWatch runs a console/pure Joss script in live-watch mode,
// automatically re-executing and updating terminal output in-place on file save.
func runScriptWithWatch(filename string, mode executionMode) {
	selfExe, err := os.Executable()
	if err != nil || selfExe == "" {
		selfExe = "joss"
	}

	watchRoot := filepath.Dir(filename)
	if watchRoot == "" {
		watchRoot = "."
	}

	runOnce := func(clearScreen bool) {
		if clearScreen {
			fmt.Print("\x1b[2J\x1b[H")
		}
		args := []string{"run", filename, "--native"}
		cmd := exec.Command(selfExe, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = append(os.Environ(), "JOSS_WATCH_CHILD=1")
		_ = cmd.Run()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	runOnce(false)
	snapshot := takeWatchSnapshot(watchRoot)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n[Joss Watch] Detenido.")
			return
		case <-ticker.C:
			_, _, changed := detectWatchChange(watchRoot, snapshot)
			if !changed {
				continue
			}
			time.Sleep(60 * time.Millisecond)
			snapshot = takeWatchSnapshot(watchRoot)
			runOnce(true)
		}
	}
}

func serverChildArgs(entryFile string, extraArgs []string, mode executionMode) []string {
	args := []string{"run", entryFile}
	if mode == executionModeNative {
		args = append(args, "--native")
	}
	return append(args, extraArgs...)
}

func serverChildEnv(gui bool, mode executionMode) []string {
	env := serverModeEnvironment(os.Environ(), mode)
	if gui {
		env = append(env, "JOSS_GUI=true")
	}
	return env
}

func serverModeEnvironment(base []string, mode executionMode) []string {
	env := make([]string, 0, len(base)+2)
	for _, item := range base {
		if strings.HasPrefix(strings.ToUpper(item), "JOSS_HTTP_BACKEND=") {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "JOSS_SERVER_MODE=1", "JOSS_HTTP_BACKEND="+serverBackendForMode(mode), "DISABLE_WATCHDOG=true")
	return env
}

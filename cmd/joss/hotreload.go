package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
)

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
		cmd.Env = serverChildEnv(enableGUI, mode)
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
	fmt.Printf("⚡ Joss Server Supervisor (Hot-Reload %s)\n", strings.ToUpper(executionModeName(mode)))
	fmt.Println("📂 Vigilando cambios en .joss, vistas (.html) y entorno")
	fmt.Println("💡 Presiona Ctrl+C para detener")
	fmt.Println("========================================================")
	fmt.Println()

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
			fmt.Printf("\n🔄 [Hot-Reload] Cambio detectado en: %s\n", relPath)

			if strings.HasSuffix(changedFile, ".scss") {
				core.CompileStyles()
			}

			stopCurrent()

			if err := startChild(); err != nil {
				fmt.Printf("❌ [Hot-Reload] Error reiniciando servidor JIT: %v\n", err)
				return
			}
			fmt.Println("✨ [Hot-Reload] Servidor JIT reiniciado con éxito.")
			fmt.Println()
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

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
	"sync"
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

func killChildProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		_ = killCmd.Run()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	} else {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// handleNativeServerStart handles 'joss server start' by compiling and executing the native binary
// produced by joss build, with Hot-Reload supervisor by default.
func handleNativeServerStart(args []string) {
	if _, err := os.Stat("main.joss"); os.IsNotExist(err) {
		fmt.Println(i18n.Tr("cliErrNoMain"))
		fmt.Println("  main.joss")
		fmt.Println(i18n.Tr("cliErrRequireMain"))
		os.Exit(1)
	}

	noWatch := false
	var extraArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--no-watch" || arg == "--nowatch" {
			noWatch = true
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

	runNativeSupervisor("main.joss", "", false, extraArgs, noWatch)
}

func runNativeSupervisor(entryFile, outExe string, enableGUI bool, extraArgs []string, noWatch bool) {
	selfExe, err := os.Executable()
	if err != nil || selfExe == "" {
		selfExe = "joss"
	}

	if noWatch {
		// Run standalone native JIT directly without file watching
		args := append([]string{"run", entryFile}, extraArgs...)
		cmd := exec.Command(selfExe, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		return
	}

	var procMu sync.Mutex
	var currentCmd *exec.Cmd

	stopCurrent := func() {
		procMu.Lock()
		defer procMu.Unlock()
		if currentCmd != nil {
			killChildProcess(currentCmd)
			currentCmd = nil
		}
	}

	startChild := func() error {
		procMu.Lock()
		defer procMu.Unlock()
		args := append([]string{"run", entryFile}, extraArgs...)
		cmd := exec.Command(selfExe, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()
		if err := cmd.Start(); err != nil {
			return err
		}
		currentCmd = cmd
		go func(c *exec.Cmd) {
			_ = c.Wait()
		}(cmd)
		return nil
	}

	// Capture interrupt signals for graceful cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n[Joss Native Server] Deteniendo servidor y procesos asociados...")
		stopCurrent()
		os.Exit(0)
	}()

	fmt.Println("⚡ [Joss Native Server] Iniciando servidor JIT...")
	if err := startChild(); err != nil {
		fmt.Printf("❌ Error iniciando servidor: %v\n", err)
	}

	fmt.Println("\n========================================================")
	fmt.Println("⚡ Joss Native Server Supervisor (Hot-Reload JIT ACTIVO)")
	fmt.Println("📂 Vigilando cambios en .joss, vistas (.html) y entorno")
	fmt.Println("💡 Presiona Ctrl+C para detener")
	fmt.Println("========================================================")
	fmt.Println()

	snapshot := takeWatchSnapshot(".")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
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
		} else {
			fmt.Println("✨ [Hot-Reload] Servidor JIT reiniciado con éxito.")
			fmt.Println()
		}
	}
}

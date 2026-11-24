package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
)

func shouldDisableJIT() bool {
	v := strings.TrimSpace(os.Getenv("JOSS_NO_JIT"))
	return v == "1" || strings.EqualFold(v, "true")
}

func computeNativeJITCacheKey(filename string, prepared *semanticanalyzer.PreparedProgram, enableGUI bool) string {
	h := sha256.New()

	// Invalidate cache automatically whenever the joss CLI binary itself changes.
	if selfExe, err := os.Executable(); err == nil && selfExe != "" {
		if info, statErr := os.Stat(selfExe); statErr == nil {
			h.Write([]byte("cli:" + selfExe + ":" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "\n"))
		}
	}

	absFile, err := filepath.Abs(filename)
	if err != nil {
		absFile = filename
	}
	h.Write([]byte("entry:" + absFile + ":gui=" + strconv.FormatBool(enableGUI) + "\n"))

	if prepared != nil {
		units := append([]semanticanalyzer.SourceUnit(nil), prepared.Units...)
		sort.Slice(units, func(i, j int) bool { return units[i].Path < units[j].Path })
		for _, u := range units {
			h.Write([]byte("unit:" + u.Path + "\n"))
			if strings.HasPrefix(u.Path, "<") {
				continue
			}
			if data, readErr := os.ReadFile(u.Path); readErr == nil {
				h.Write(data)
			}
			h.Write([]byte("\n"))
		}
	}

	// Include views, compiled CSS, plugins, and manifest metadata in cache key.
	for _, dir := range []string{filepath.Join("app", "views"), filepath.Join("public", "css"), "plugins"} {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info == nil || info.IsDir() {
				return nil
			}
			if strings.HasPrefix(filepath.ToSlash(path), "public/css/") {
				h.Write([]byte("css:" + filepath.ToSlash(path) + "\n"))
				if data, readErr := os.ReadFile(path); readErr == nil {
					h.Write(data)
				}
				return nil
			}
			h.Write([]byte("asset:" + filepath.ToSlash(path) + ":" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "\n"))
			return nil
		})
	}
	for _, extra := range []string{"joss.yaml", "joss.lock", "routes.joss", "api.joss"} {
		if info, statErr := os.Stat(extra); statErr == nil && !info.IsDir() {
			h.Write([]byte("file:" + extra + ":" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "\n"))
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

func nativeJITCachePath(cacheKey string) (string, error) {
	cacheDir := filepath.Join(os.TempDir(), "joss-jit-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	name := "joss-jit-" + cacheKey[:16]
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(cacheDir, name), nil
}

// executeViaNativeLLVMJIT provides Tier-2 native JIT execution for programs
// whose IR requires the C11 runtime (HTTP server, GranDB, TLS/crypto, views,
// plugins) beyond the Tier-1 in-memory machine-code emitter.
func executeViaNativeLLVMJIT(irProg *ir.Program, prepared *semanticanalyzer.PreparedProgram, filename string, mode executionMode) (bool, int, error) {
	target := native.HostTarget()
	if !ensureNativeToolchainReady() {
		return false, 0, fmt.Errorf("JOSS-NATIVE-001: native LLVM toolchain unavailable")
	}
	if err := native.ValidateAOTCapabilitiesForBackend(irProg, target, native.BackendLLVM); err != nil {
		return false, 0, err
	}

	enableGUI := strings.EqualFold(strings.TrimSpace(os.Getenv("JOSS_GUI")), "true")
	cacheKey := computeNativeJITCacheKey(filename, prepared, enableGUI)
	cachePath, err := nativeJITCachePath(cacheKey)
	if err != nil {
		return false, 0, err
	}

	cacheHit := false
	if info, statErr := os.Stat(cachePath); statErr == nil && !info.IsDir() && info.Size() > 0 {
		cacheHit = true
	}

	if !cacheHit {
		if mode == executionModeAuto {
			fmt.Println("[Auto] Capacidades nativas disponibles; se seleccionó JIT nativo (LLVM).")
		}
		var seedTables map[string][]map[string]string
		envDetect := core.DetectAndLoadEnv(nil)
		if len(envDetect.EnvMap) > 0 {
			dbDriver := strings.ToLower(strings.TrimSpace(envDetect.EnvMap["DB"]))
			if dbDriver == "" {
				dbDriver = strings.ToLower(strings.TrimSpace(envDetect.EnvMap["DB_CONNECTION"]))
			}
			if dbDriver != "" {
				seedTables = extractGranDBSeedTables(dbDriver, envDetect.EnvMap, false)
			}
		}

		tmpPath := fmt.Sprintf("%s.tmp-%d", cachePath, os.Getpid())
		if runtime.GOOS == "windows" {
			tmpPath = strings.TrimSuffix(cachePath, ".exe") + fmt.Sprintf(".tmp-%d.exe", os.Getpid())
		}
		if _, buildErr := native.BuildProgram(irProg, native.BuildOptions{
			Target:     target,
			Backend:    native.BackendLLVM,
			OutputPath: tmpPath,
			GUI:        enableGUI,
			SeedTables: seedTables,
		}); buildErr != nil {
			_ = os.Remove(tmpPath)
			return false, 0, buildErr
		}
		if renameErr := os.Rename(tmpPath, cachePath); renameErr != nil {
			// Another process may have populated the same cache entry concurrently.
			if info, statErr := os.Stat(cachePath); statErr == nil && !info.IsDir() && info.Size() > 0 {
				_ = os.Remove(tmpPath)
			} else {
				cachePath = tmpPath
				defer os.Remove(tmpPath)
			}
		}
	} else if mode == executionModeAuto {
		fmt.Println("[Auto] Capacidades nativas disponibles; se seleccionó JIT nativo (LLVM cache).")
	}

	cmd := exec.Command(cachePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	if startErr := cmd.Start(); startErr != nil {
		return true, 1, startErr
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	select {
	case waitErr := <-done:
		if waitErr == nil {
			return true, 0, nil
		}
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return true, exitErr.ExitCode(), nil
		}
		return true, 1, waitErr
	case <-sigChan:
		killChildProcess(cmd)
		<-done
		return true, 0, nil
	}
}

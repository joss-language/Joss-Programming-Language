package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/parser"
)

// executeInlineSourceNative compiles and executes an inline or synthesized Joss
// program strictly through the Joss Native pipeline (IR -> Tier-1/Tier-2 LLVM JIT),
// without invoking the legacy Go AST interpreter.
func executeInlineSourceNative(virtualName, source string, capture bool) (string, string, int, error) {
	l := parser.NewLexer(source)
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", "", 1, fmt.Errorf("error de sintaxis en %s: %s", virtualName, strings.Join(p.Errors(), "; "))
	}

	units := []semanticanalyzer.SourceUnit{{
		Path:    virtualName,
		Program: prog,
	}}
	// Include app/ project units when running migrations/seeders inside a project.
	if _, err := os.Stat("app"); err == nil && (strings.Contains(virtualName, "migration") || strings.Contains(virtualName, "seeder")) {
		if projUnits, _ := semanticanalyzer.LoadProject(virtualName, "app"); len(projUnits) > 0 {
			for _, u := range projUnits {
				if u.Path != virtualName {
					units = append(units, u)
				}
			}
		}
	}

	report := core.AnalyzeSourceUnits(units)
	if report.HasErrors() {
		var msgs []string
		for _, d := range report.Diagnostics {
			if d.Severity == "error" {
				msgs = append(msgs, d.String())
			}
		}
		return "", "", 1, fmt.Errorf("error semántico en %s:\n%s", virtualName, strings.Join(msgs, "\n"))
	}

	irProg, irErr := lowerPreparedProject(report.Prepared, virtualName)
	if irErr != nil {
		return "", "", 1, fmt.Errorf("error lowering IR (%s): %w", virtualName, irErr)
	}

	// Fast path: Tier-1 in-memory JIT when output streaming is requested.
	if !capture {
		emitter := native.SelectCodeEmitter(native.HostTarget(), false)
		if jitExe, jitErr := emitter.CompileInMemory(irProg); jitErr == nil {
			defer jitExe.Free()
			code, execErr := jitExe.Execute()
			return "", "", code, execErr
		}
	}

	// Tier-2 Native LLVM JIT (full C11 runtime: GranDB, Schema, Redis, HTTP, Console, etc.)
	target := native.HostTarget()
	if !ensureNativeToolchainReady() {
		return "", "", 1, fmt.Errorf("JOSS-NATIVE-001: toolchain nativo LLVM no disponible")
	}
	if err := native.ValidateAOTCapabilitiesForBackend(irProg, target, native.BackendLLVM); err != nil {
		return "", "", 1, err
	}

	tmpDir, err := os.MkdirTemp("", "joss-native-exec-*")
	if err != nil {
		return "", "", 1, err
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "joss_exec")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	if _, buildErr := native.BuildProgram(irProg, native.BuildOptions{
		Target:     target,
		Backend:    native.BackendLLVM,
		OutputPath: binPath,
	}); buildErr != nil {
		return "", "", 1, buildErr
	}

	cmd := exec.Command(binPath)
	cmd.Env = os.Environ()
	var stdoutBuf, stderrBuf bytes.Buffer
	if capture {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
	}

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
			runErr = nil
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, runErr
}

// executeEvalNative runs `joss eval` through the Joss Native pipeline.
func executeEvalNative(source string, asJSON bool) {
	stdout, stderr, exitCode, err := executeInlineSourceNative("<eval>", source, asJSON)
	if asJSON {
		payload := map[string]interface{}{
			"success": err == nil && exitCode == 0,
			"stdout":  stdout,
			"stderr":  stderr,
		}
		if err != nil {
			payload["error"] = err.Error()
		}
		enc, _ := json.Marshal(payload)
		fmt.Println(string(enc))
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

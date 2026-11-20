package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionModeDefaultsToAutoAndNativeIsExplicit(t *testing.T) {
	mode, args := takeExecutionMode([]string{"main.joss"})
	if mode != executionModeAuto || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("default mode = %q, args = %v; want auto mode and untouched args", executionModeName(mode), args)
	}

	mode, args = takeExecutionMode([]string{"main.joss", "--native"})
	if mode != executionModeNative || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("native mode = %q, args = %v; want native mode and selector removed", executionModeName(mode), args)
	}
}

func TestServerChildPreservesNativeModeThroughReload(t *testing.T) {
	got := serverChildArgs("main.joss", []string{"--port=9000"}, executionModeNative)
	want := []string{"run", "main.joss", "--native", "--port=9000"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("native child args = %v, want %v", got, want)
	}
	got = serverChildArgs("main.joss", nil, executionModePrevious)
	if strings.Join(got, " ") != "run main.joss" {
		t.Fatalf("previous child args = %v, want no native selector", got)
	}
	got = serverChildArgs("main.joss", nil, executionModeAuto)
	if strings.Join(got, " ") != "run main.joss" {
		t.Fatalf("auto child args = %v, want auto selection via default", got)
	}
}

func TestServerChildSelectsConcurrentGoHTTPByDefault(t *testing.T) {
	if got := serverBackendForMode(executionModeAuto); got != "go" {
		t.Fatalf("auto HTTP backend = %q, want go", got)
	}
	if got := serverBackendForMode(executionModePrevious); got != "go" {
		t.Fatalf("previous HTTP backend = %q, want go", got)
	}
	if got := serverBackendForMode(executionModeNative); got != "native" {
		t.Fatalf("native HTTP backend = %q, want native", got)
	}
	if !shouldUseGoHTTPRuntime(executionModeAuto, "go") || !shouldUseGoHTTPRuntime(executionModePrevious, "go") {
		t.Fatal("automatic and previous server modes should use the Go HTTP runtime")
	}
	if shouldUseGoHTTPRuntime(executionModeNative, "go") || shouldUseGoHTTPRuntime(executionModeAuto, "native") {
		t.Fatal("native mode or an explicitly native server backend must not select Go HTTP")
	}
	t.Setenv("JOSS_HTTP_BACKEND", "inherited-invalid")
	for mode, want := range map[executionMode]string{
		executionModeAuto:     "go",
		executionModePrevious: "go",
		executionModeNative:   "native",
	} {
		var selected string
		for _, entry := range serverChildEnv(false, mode) {
			if strings.HasPrefix(strings.ToUpper(entry), "JOSS_HTTP_BACKEND=") {
				selected = strings.TrimPrefix(entry, "JOSS_HTTP_BACKEND=")
			}
		}
		if selected != want {
			t.Errorf("child HTTP backend for %s = %q, want %q", executionModeName(mode), selected, want)
		}
	}
}

func TestProjectHTTPServerDetectionUsesPreparedCallFacts(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	source := `public func main(): int {
    Server::start()
    return 0
}`
	if err := os.WriteFile("main.joss", []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if !projectUsesHTTPServer("main.joss") {
		t.Fatal("project with resolved Server::start should select the Go HTTP server in auto mode")
	}
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}
	if projectUsesHTTPServer("main.joss") {
		t.Fatal("the Standard Library's own server definitions must not mark a CLI project as an HTTP server")
	}
}

func TestAutomaticHTTPBuildUsesGoRunner(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    Server::start()
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}
	runnerDir := filepath.Join(t.TempDir(), "runners")
	if err := os.MkdirAll(filepath.Join(runnerDir, "windows-amd64"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runnerDir, "windows-amd64", "joss-runner.exe"), []byte("test-runner"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOSS_RUNNER_DIR", runnerDir)

	output, err := buildProgramForMode("main.joss", "", "windows-amd64", false, false, false, false, "native", executionModeAuto)
	if err != nil {
		t.Fatalf("auto HTTP build should select the Go runner: %v", err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contents, []byte(legacyRunnerTrailer)) {
		t.Fatal("auto HTTP build did not package the Go runner")
	}
}

func TestNativeCapabilityGapIsTheOnlyAutomaticFallback(t *testing.T) {
	if !isNativeCapabilityGap(errors.New("JOSS-NATIVE-001: unsupported call")) {
		t.Fatal("native capability diagnostic should allow automatic compatibility selection")
	}
	if !isNativeCapabilityGap(errors.New("capacidades AOT no disponibles: Socket::listen")) {
		t.Fatal("AOT capability validation error should allow automatic compatibility selection")
	}
	for _, err := range []error{
		nil,
		errors.New("syntax error"),
		errors.New("IR verifier internal failure"),
	} {
		if isNativeCapabilityGap(err) {
			t.Errorf("%v must not trigger compatibility selection", err)
		}
	}
}

func TestBuildRejectsMissingAOTCapabilityBeforePreparingProjectAssets(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)

	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    Cache::get("probe")
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("assets", "css"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("assets", "css", "probe.scss"), []byte("body { color: red; }"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := buildNativeProgram("main.joss", "", "windows-amd64", false, false, false, false, "native")
	if err == nil || !strings.Contains(err.Error(), "Cache::get") {
		t.Fatalf("build should reject the missing AOT service capability, got %v", err)
	}
	if _, err := os.Stat(filepath.Join("public", "css", "probe.css")); !os.IsNotExist(err) {
		t.Fatalf("AOT preflight should reject before compiling project styles; stat error = %v", err)
	}
}

func TestAutomaticBuildUsesCompatibilityRunnerOnlyForNativeCapabilityGap(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("custom.joss", []byte(`public func main(): int {
    Cache::get("probe")
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}

	runnerDir := filepath.Join(t.TempDir(), "runners")
	targetRunnerDir := filepath.Join(runnerDir, "windows-amd64")
	if err := os.MkdirAll(targetRunnerDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetRunnerDir, "joss-runner.exe"), []byte("test-runner"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOSS_RUNNER_DIR", runnerDir)

	output, err := buildProgramForMode("custom.joss", "", "windows-amd64", false, false, false, false, "native", executionModeAuto)
	if err != nil {
		t.Fatalf("auto build should select the compatibility runner for a known native gap: %v", err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contents, []byte(legacyRunnerTrailer)) {
		t.Fatalf("auto build output lacks the sealed compatibility payload marker")
	}
}

func TestAutomaticBuildUsesCompactAOTWhenNoHostCapabilityIsNeeded(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    return 0
}`), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := buildProgramForMode("main.joss", "", "windows-amd64", true, false, false, false, "native", executionModeAuto)
	if err != nil {
		t.Fatalf("auto build should select AOT for a pure program: %v", err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) < 2 || string(contents[:2]) != "MZ" {
		t.Fatalf("pure auto build is not a PE executable: prefix=%q", contents[:min(2, len(contents))])
	}
	if bytes.Contains(contents, []byte(legacyRunnerTrailer)) {
		t.Fatal("pure auto build unexpectedly embeds a Go compatibility runner")
	}
	if len(contents) > 80*1024 {
		t.Fatalf("pure auto build size = %d bytes, exceeds the 80 KiB compact profile goal", len(contents))
	}
}

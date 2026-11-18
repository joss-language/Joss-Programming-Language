package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionModeDefaultsToPreviousAndNativeIsExplicit(t *testing.T) {
	mode, args := takeExecutionMode([]string{"main.joss"})
	if mode != executionModePrevious || len(args) != 1 || args[0] != "main.joss" {
		t.Fatalf("default mode = %q, args = %v; want previous mode and untouched args", executionModeName(mode), args)
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
}

func TestBuildRejectsMissingAOTCapabilityBeforePreparingProjectAssets(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)

	if err := os.WriteFile("main.joss", []byte(`public func main(): int {
    JSON::stringify("probe")
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
	if err == nil || !strings.Contains(err.Error(), "JSON::stringify") {
		t.Fatalf("build should reject the missing AOT service capability, got %v", err)
	}
	if _, err := os.Stat(filepath.Join("public", "css", "probe.css")); !os.IsNotExist(err) {
		t.Fatalf("AOT preflight should reject before compiling project styles; stat error = %v", err)
	}
}

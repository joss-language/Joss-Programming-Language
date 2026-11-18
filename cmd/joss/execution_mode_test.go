package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerChildUsesJITUnlessInterpreterOracleIsExplicit(t *testing.T) {
	t.Setenv("JOSS_SERVER_MODE", "1")
	t.Setenv("JOSS_NO_JIT", "")
	if !shouldAttemptJIT() {
		t.Fatal("server child must attempt JIT through the shared IR pipeline")
	}

	t.Setenv("JOSS_NO_JIT", "1")
	if shouldAttemptJIT() {
		t.Fatal("explicit interpreter-oracle mode must skip JIT")
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

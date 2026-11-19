package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceExecutable(t *testing.T) {
	tempDir := t.TempDir()

	currentExe := filepath.Join(tempDir, "joss_current_binary")
	newExe := filepath.Join(tempDir, "joss_new_binary")

	if err := os.WriteFile(currentExe, []byte("v1.0.0-binary-content"), 0755); err != nil {
		t.Fatalf("failed to create currentExe: %v", err)
	}

	if err := os.WriteFile(newExe, []byte("v2.0.0-binary-content"), 0755); err != nil {
		t.Fatalf("failed to create newExe: %v", err)
	}

	if err := replaceExecutable(currentExe, newExe); err != nil {
		t.Fatalf("replaceExecutable failed: %v", err)
	}

	updatedContent, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("failed to read updated currentExe: %v", err)
	}

	if string(updatedContent) != "v2.0.0-binary-content" {
		t.Fatalf("expected 'v2.0.0-binary-content', got '%s'", string(updatedContent))
	}
}

func TestVersionParsingAndComparison(t *testing.T) {
	if !isVersionNewer("3.6.8", "3.6.7") {
		t.Fatalf("expected 3.6.8 to be newer than 3.6.7")
	}
	if !isVersionNewer("3.6.7.4", "3.6.7.3") {
		t.Fatalf("expected 3.6.7.4 to be newer than 3.6.7.3")
	}
	if isVersionNewer("3.6.7", "3.6.7") {
		t.Fatalf("expected 3.6.7 not to be newer than 3.6.7")
	}
	if isVersionNewer("3.6.6", "3.6.7") {
		t.Fatalf("expected 3.6.6 not to be newer than 3.6.7")
	}
}

func writeUpdaterTestZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
}

func TestExtractAndSyncUpdatePackageWindowsARM64AndCleanLeftovers(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "jossecurity-windows.zip")
	writeUpdaterTestZip(t, zipPath, map[string]string{
		"joss.exe":                                "amd64-binary",
		"joss-windows-arm64.exe":                  "arm64-binary",
		"remote-install.ps1":                      "# installer script",
		"runners/windows-arm64/joss-runner.exe":   "new-runner",
		"toolchain/windows-amd64/zig.exe":         "amd64-zig",
		"toolchain/windows-arm64/zig.exe":         "arm64-zig",
		"toolchain/windows-arm64/lib/std/std.zig": "std-lib",
	})

	stageDir := filepath.Join(tempDir, "stage")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		t.Fatal(err)
	}

	pkg, err := extractUpdatePackage(zipPath, stageDir, "windows", "arm64")
	if err != nil {
		t.Fatalf("extractUpdatePackage failed: %v", err)
	}
	gotBin, err := os.ReadFile(pkg.BinaryPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(gotBin) != "arm64-binary" {
		t.Fatalf("expected arm64-binary, got %q", string(gotBin))
	}

	installDir := filepath.Join(tempDir, "install")
	// Pre-populate stale files that must be cleaned up
	for _, stale := range []string{
		filepath.Join(installDir, "sdk", "legacy.txt"),
		filepath.Join(installDir, "runners", "stale-runner.exe"),
		filepath.Join(installDir, "toolchain", "windows-amd64", "old-zig.exe"),
	} {
		if err := os.MkdirAll(filepath.Dir(stale), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stale, []byte("stale"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := syncUpdateCompanionDirs(installDir, pkg.RunnersDir, pkg.ToolchainDir, "windows", "arm64"); err != nil {
		t.Fatalf("syncUpdateCompanionDirs failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(installDir, "sdk")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy sdk directory to be removed")
	}
	if _, err := os.Stat(filepath.Join(installDir, "runners", "stale-runner.exe")); !os.IsNotExist(err) {
		t.Fatalf("expected stale runner file to be removed")
	}
	if _, err := os.Stat(filepath.Join(installDir, "toolchain", "windows-amd64")); !os.IsNotExist(err) {
		t.Fatalf("expected non-host toolchain architecture to be removed")
	}
	if data, err := os.ReadFile(filepath.Join(installDir, "runners", "windows-arm64", "joss-runner.exe")); err != nil || string(data) != "new-runner" {
		t.Fatalf("expected updated runner, got err=%v data=%q", err, string(data))
	}
	if data, err := os.ReadFile(filepath.Join(installDir, "toolchain", "windows-arm64", "lib", "std", "std.zig")); err != nil || string(data) != "std-lib" {
		t.Fatalf("expected nested bundled toolchain file, got err=%v data=%q", err, string(data))
	}
}

package buildmanifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
)

func TestGenerateAndSaveBuildManifest(t *testing.T) {
	graph := analyzer.NewReachabilityGraph("main.joss")
	graph.LiveFiles["main.joss"] = true
	graph.LiveFiles["app/services/Math.joss"] = true
	graph.LiveClasses["Math"] = true
	graph.RuntimeCapabilities[analyzer.CapIO] = true

	allFiles := []string{
		"main.joss",
		"app/services/Math.joss",
		"app/services/Unused.joss",
	}

	manifest := GenerateManifest(
		"main.joss", "release", "windows", "amd64", "cli", "build/app.exe",
		allFiles, graph, 1024,
	)

	if manifest.Reachability.ScannedFiles != 3 {
		t.Errorf("expected 3 scanned files, got %d", manifest.Reachability.ScannedFiles)
	}
	if manifest.Reachability.IncludedFiles != 2 {
		t.Errorf("expected 2 included files, got %d", manifest.Reachability.IncludedFiles)
	}
	if manifest.Reachability.PrunedFiles != 1 {
		t.Errorf("expected 1 pruned file, got %d", manifest.Reachability.PrunedFiles)
	}
	if !manifest.RuntimeCapabilities[string(analyzer.CapIO)] {
		t.Errorf("expected IO capability to be active")
	}

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "build-manifest.json")
	if err := manifest.SaveToFile(outPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatalf("saved manifest file does not exist")
	}
}

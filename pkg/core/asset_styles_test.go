package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileStylesUsesEphemeralAssetsCacheAndCleanup(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	if err := os.MkdirAll(filepath.Join("assets", "css"), 0755); err != nil {
		t.Fatal(err)
	}
	scss := "$primary: #0ea5e9;\nbody { color: $primary; }\n"
	if err := os.WriteFile(filepath.Join("assets", "css", "app.scss"), []byte(scss), 0644); err != nil {
		t.Fatal(err)
	}

	CompileStyles()

	cachedCSS := filepath.Join(StylesCacheDir, "app.css")
	data, err := os.ReadFile(cachedCSS)
	if err != nil {
		t.Fatalf("expected compiled CSS in %s: %v", cachedCSS, err)
	}
	if !strings.Contains(string(data), "#0ea5e9") {
		t.Fatalf("expected SCSS variable substitution, got: %s", string(data))
	}
	if _, err := os.Stat(filepath.Join("public", "css", "app.css")); !os.IsNotExist(err) {
		t.Fatalf("CompileStyles must not write into public/css")
	}

	CleanupAssetCache()
	if _, err := os.Stat(AssetCacheRoot); !os.IsNotExist(err) {
		t.Fatalf("CleanupAssetCache must remove %s", AssetCacheRoot)
	}
}

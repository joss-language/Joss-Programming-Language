package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceSCSSVariablesKeepsPrefixNamesDistinct(t *testing.T) {
	variables := map[string]string{
		"$primary":      "#00f2ff",
		"$primary-dark": "#00bceb",
		"$text":         "#f1f5f9",
		"$text-dim":     "#64748b",
	}
	source := ".button { color: $text-dim; background: $primary-dark; border-color: $missing; }"
	want := ".button { color: #64748b; background: #00bceb; border-color: $missing; }"

	if got := replaceSCSSVariables(source, variables); got != want {
		t.Fatalf("replaceSCSSVariables() = %q, want %q", got, want)
	}
}

func TestServePublicFile(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	_ = os.Chdir(tempDir)

	publicDir := filepath.Join(tempDir, "public")
	_ = os.MkdirAll(publicDir, 0755)
	adsFile := filepath.Join(publicDir, "ads.txt")
	_ = os.WriteFile(adsFile, []byte("google.com, pub-1234567890123456, DIRECT, f08c47fec0942fa0"), 0644)

	// 1. Test GET /ads.txt
	req, _ := http.NewRequest("GET", "/ads.txt", nil)
	rr := httptest.NewRecorder()
	handled := servePublicFile(rr, req)
	if !handled {
		t.Fatalf("expected servePublicFile to handle /ads.txt")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "pub-1234567890123456") {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}

	// 2. Test nonexistent file returns false
	req404, _ := http.NewRequest("GET", "/nonexistent.txt", nil)
	rr404 := httptest.NewRecorder()
	handled404 := servePublicFile(rr404, req404)
	if handled404 {
		t.Fatalf("expected servePublicFile to return false for nonexistent file")
	}
}

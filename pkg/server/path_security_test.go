package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSafeSubpathBlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	pubDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(pubDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "ok.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	if resolved, ok := resolveSafeSubpath(pubDir, "ok.txt"); !ok || filepath.Base(resolved) != "ok.txt" {
		t.Fatalf("expected valid path ok.txt to resolve, got ok=%v path=%q", ok, resolved)
	}

	for _, bad := range []string{
		"../secret.txt",
		"..%2fsecret.txt",
		"%2e%2e/secret.txt",
		"..\\secret.txt",
		"a/../../secret.txt",
		"ok.txt\x00.png",
		"",
		".",
	} {
		if resolved, ok := resolveSafeSubpath(pubDir, bad); ok {
			t.Fatalf("expected traversal path %q to be rejected, but resolved to %q", bad, resolved)
		}
	}
}

func TestHandleVendorAssetsBlocksEncodedTraversal(t *testing.T) {
	for _, path := range []string{
		"/assets/vendor/../go.mod",
		"/assets/vendor/%2e%2e/go.mod",
		"/assets/vendor/..\\go.mod",
	} {
		req := httptest.NewRequest("GET", path, nil)
		rr := httptest.NewRecorder()
		handleVendorAssets(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for %q, got %d", path, rr.Code)
		}
	}
}

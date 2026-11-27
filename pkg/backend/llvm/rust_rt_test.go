package llvm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRustRuntime_ZeroInstallAutoDownloadAndCache(t *testing.T) {
	fakeLib := []byte("!<arch>\nfake-joss-rust-staticlib-payload")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/libjoss_rt_rs_linux_amd64.a" {
			http.NotFound(w, r)
			return
		}
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeLib)
	}))
	defer srv.Close()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("JOSS_DISABLE_RUST_RT", "")
	t.Setenv("JOSS_NO_AUTO_TOOLCHAIN", "")
	t.Setenv("JOSS_RUST_RT_BASE_URL", srv.URL)

	emptyRtDir := filepath.Join(t.TempDir(), "native")
	if err := os.MkdirAll(emptyRtDir, 0755); err != nil {
		t.Fatal(err)
	}

	// First call downloads and caches
	p1 := EnsureRustRuntime(emptyRtDir, "linux", "amd64")
	if p1 == "" {
		t.Fatalf("expected EnsureRustRuntime to auto-download libjoss_rt_rs_linux_amd64.a")
	}
	got, err := os.ReadFile(p1)
	if err != nil || string(got) != string(fakeLib) {
		t.Fatalf("unexpected downloaded payload: err=%v len=%d", err, len(got))
	}
	if hits != 1 {
		t.Fatalf("expected 1 HTTP hit, got %d", hits)
	}

	// Second call must hit cache (0 additional HTTP requests)
	p2 := EnsureRustRuntime(emptyRtDir, "linux", "amd64")
	if p2 != p1 {
		t.Fatalf("expected cached path %s, got %s", p1, p2)
	}
	if hits != 1 {
		t.Fatalf("expected cache hit with still 1 HTTP request, got %d", hits)
	}
}

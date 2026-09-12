package main

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/pluginpkg"
)

func TestVerifyPublishArtifactRequiresValidSignedJP(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	data, err := pluginpkg.BuildSigned(
		pluginpkg.Metadata{Name: "vendor/demo", Version: "1.2.3", Bytecode: "main.jbc"},
		map[string][]byte{"main.jbc": []byte("compiled")},
		key,
	)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write(data)
	}))
	defer server.Close()
	digest := sha256.Sum256(data)
	keyID, err := verifyPublishArtifact(server.URL, hex.EncodeToString(digest[:]), "vendor/demo", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if keyID == "" {
		t.Fatal("verified artifact did not return key id")
	}
}

func writeTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func withTempCWD(t *testing.T) string {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	return dir
}

func TestExtractZipSecurelyCharacterization(t *testing.T) {
	dir := withTempCWD(t)
	archivePath := writeTestZip(t, map[string]string{
		"demo-main/joss.yaml":      "name: demo\n",
		"demo-main/src/main.joss":  "echo('ok')\n",
		"demo-main/docs/readme.md": "ignored\n",
	})
	if err := extractZipSecurely(archivePath, "demo", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins", "demo", "1.0.0", "joss.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins", "demo", "1.0.0", "src", "main.joss")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins", "demo", "1.0.0", "docs", "readme.md")); !os.IsNotExist(err) {
		t.Fatalf("non-runtime file was extracted: %v", err)
	}

	traversal := writeTestZip(t, map[string]string{"demo-main/src/../../escape.joss": "bad"})
	if err := extractZipSecurely(traversal, "demo", "2.0.0"); err == nil || !strings.Contains(err.Error(), "Traversal") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.joss")); !os.IsNotExist(err) {
		t.Fatal("path traversal escaped extraction root")
	}
}

func TestExtractZipSecurelyRejectsInvalidArchive(t *testing.T) {
	withTempCWD(t)
	path := filepath.Join(t.TempDir(), "invalid.zip")
	if err := os.WriteFile(path, []byte("not a zip"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractZipSecurely(path, "demo", "1.0.0"); err == nil {
		t.Fatal("invalid archive unexpectedly accepted")
	}
}

func TestManifestAndLockfileCharacterization(t *testing.T) {
	dir := withTempCWD(t)
	if got := parseManifestDependencies("dependencies:\n  a: ^1.2.3\n  b: \"~2.0\"\n"); got["a"] != "^1.2.3" || got["b"] != "~2.0" {
		t.Fatalf("unexpected dependencies: %#v", got)
	}
	if got := cleanPackageConstraint(" ^~1.2.3 "); got != "1.2.3" {
		t.Fatalf("constraint cleanup = %q", got)
	}
	if err := os.WriteFile("joss.yaml", []byte("name: app\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("plugins", "a", "1.0.0"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("plugins", "a", "1.0.0", "joss.yaml"), []byte("dependencies:\n  b: 2.0.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	generateLockFile(map[string]string{"b": "2.0.0", "a": "^1.0.0"})
	first, err := os.ReadFile("joss.lock")
	if err != nil {
		t.Fatal(err)
	}
	generateLockFile(map[string]string{"a": "^1.0.0", "b": "2.0.0"})
	second, err := os.ReadFile("joss.lock")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("lockfile is not deterministic")
	}
	var lock LockFile
	if err := json.Unmarshal(first, &lock); err != nil {
		t.Fatal(err)
	}
	if lock.Packages["a"].Version != "1.0.0" || lock.Packages["a"].Dependencies["b"] != "2.0.0" {
		t.Fatalf("unexpected lock package: %#v", lock.Packages["a"])
	}
	if lock.ManifestHash == "" || !strings.Contains(lock.Packages["a"].Resolved, "/api/v1/pub/packages/a/versions/1.0.0") {
		t.Fatalf("incomplete lock metadata in %s", dir)
	}
}

func TestResolvePackageDownloadUsesRegistryResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/versions/1.2.3") {
			_, _ = w.Write([]byte(`{"download_url":"https://cdn.test/demo.jp","checksum":"abc"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	t.Setenv("PUB_REGISTRY_URL", server.URL)
	url, checksum, err := resolvePackageDownload("vendor/demo", "1.2.3")
	if err != nil || url != "https://cdn.test/demo.jp" || checksum != "abc" {
		t.Fatalf("registry resolution = %q, %q, %v", url, checksum, err)
	}
	missing, _, err := resolvePackageDownload("vendor/missing", "1.0.0")
	if err == nil || missing != "" || !strings.Contains(err.Error(), "no existe") {
		t.Fatalf("missing package result = %q, %v", missing, err)
	}
}

func TestResolvePackageDownloadRegistryFailures(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			t.Setenv("PUB_REGISTRY_URL", server.URL)
			if _, _, err := resolvePackageDownload("vendor/demo", "1.0.0"); err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) {
				t.Fatalf("status %d error = %v", status, err)
			}
		})
	}
	t.Run("invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/versions/") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"versions":[`))
		}))
		defer server.Close()
		t.Setenv("PUB_REGISTRY_URL", server.URL)
		if _, _, err := resolvePackageDownload("vendor/demo", "1.0.0"); err == nil || !strings.Contains(err.Error(), "invalida") {
			t.Fatalf("invalid JSON error = %v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		previous := pubHTTPClient
		pubHTTPClient = &http.Client{Timeout: 10 * time.Millisecond}
		t.Cleanup(func() { pubHTTPClient = previous })
		t.Setenv("PUB_REGISTRY_URL", server.URL)
		if _, _, err := resolvePackageDownload("vendor/demo", "1.0.0"); err == nil {
			t.Fatal("registry timeout unexpectedly succeeded")
		}
	})
}

func TestPackageCacheHitStaleAndNoPartialFile(t *testing.T) {
	withTempCWD(t)
	cache := t.TempDir()
	t.Setenv("JOSS_PUB_CACHE_DIR", cache)
	archivePath := writeTestZip(t, map[string]string{"demo-main/joss.yaml": "name: demo\n", "demo-main/src/main.joss": "ok\n"})
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archiveData)
	checksum := hex.EncodeToString(digest[:])
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); _, _ = w.Write(archiveData) }))
	defer server.Close()
	url := server.URL + "/demo.zip"
	if err := downloadAndExtract("demo", "1.0.0", url, checksum); err != nil {
		t.Fatal(err)
	}
	if err := downloadAndExtract("demo", "1.0.0", url, checksum); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("cache hit performed %d downloads", requests.Load())
	}
	cachePath := filepath.Join(cache, "demo-1.0.0.zip")
	if err := os.WriteFile(cachePath, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := downloadAndExtract("demo", "1.0.0", url, checksum); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatalf("stale cache performed %d downloads", requests.Load())
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer failing.Close()
	if err := downloadAndExtract("broken", "1.0.0", failing.URL+"/broken.zip", ""); err == nil {
		t.Fatal("failed download unexpectedly succeeded")
	}
	if matches, _ := filepath.Glob(filepath.Join(cache, "broken-*.tmp")); len(matches) != 0 {
		t.Fatalf("partial cache files remain: %v", matches)
	}
}

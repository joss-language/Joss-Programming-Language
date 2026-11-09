package native

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeBlogArticlesLoadedFromMySQL(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	josSecurityDir := filepath.Join(repoRoot, "ejemplos", "Joss-Red-JosSecurity")
	jossExe := filepath.Join(repoRoot, "joss.exe")

	if _, err := os.Stat(jossExe); os.IsNotExist(err) {
		t.Fatalf("joss.exe not found at %s", jossExe)
	}

	if err := os.Chdir(josSecurityDir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	port := "8089"
	cmd := exec.Command(jossExe, "run", "main.joss")
	cmd.Dir = josSecurityDir
	cmd.Env = append(os.Environ(), "PORT="+port)

	var srvBuf bytes.Buffer
	cmd.Stdout = &srvBuf
	cmd.Stderr = &srvBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start joss run main.joss: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	client := &http.Client{Timeout: 3 * time.Second}
	var resp *http.Response
	var reqErr error

	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, reqErr = client.Get("http://127.0.0.1:" + port + "/blog")
		if reqErr == nil {
			break
		}
	}

	if reqErr != nil {
		t.Fatalf("Server failed to respond on http://127.0.0.1:%s/blog: %v\nOutput:\n%s", port, reqErr, srvBuf.String())
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading response body: %v", err)
	}
	html := string(bodyBytes)
	t.Logf("GET /blog returned %d bytes, status %d", len(html), resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	// Verify that real MySQL articles are rendered in the HTML!
	foundArticle := false
	for _, articleSnippet := range []string{
		"Importancia del Diseño UI",
		"Java y JavaScript",
		"Modelo en Espiral",
		"1&1 IONOS",
		"Bases de Datos",
		"Oracle Cloud Always Free",
		"GitHub Copilot",
		"La Iniciativa Joss",
	} {
		if strings.Contains(html, articleSnippet) {
			t.Logf("✓ Verified article present in rendered HTML: %q", articleSnippet)
			foundArticle = true
		}
	}

	if !foundArticle {
		t.Fatalf("No blog articles found in /blog HTML!\nPreview:\n%.1000s", html)
	}

	// Verify that OCI Storage images are served online via GET /admin/cms/media-view/...
	imgResp, err := http.Get("http://127.0.0.1:8089/admin/cms/media-view/cms-media/1788676862_cover.png")
	if err != nil {
		t.Fatalf("failed requesting OCI image: %v", err)
	}
	defer imgResp.Body.Close()
	imgBytes, _ := io.ReadAll(imgResp.Body)
	t.Logf("GET /admin/cms/media-view/cms-media/1788676862_cover.png returned %d bytes, status %d", len(imgBytes), imgResp.StatusCode)
	if imgResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for OCI media image, got %d. Body: %s", imgResp.StatusCode, string(imgBytes))
	}
	if len(imgBytes) < 4 || (imgBytes[0] != 0x89 && imgBytes[0] != 0xFF) {
		t.Fatalf("expected image binary header (PNG or JPEG), got %v", imgBytes[:min(len(imgBytes), 16)])
	}
	t.Logf("✓ Verified OCI Storage media served online: %d bytes (starts with %x %x)", len(imgBytes), imgBytes[0], imgBytes[1])
}

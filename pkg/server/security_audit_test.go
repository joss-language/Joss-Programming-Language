package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// PoC JOSS-SEC-VULN-003: Archivos temporales de subida multipart deben sanitizar el nombre
// y limpiarse automáticamente al terminar MainHandler.
func TestSecurityAudit_VULN003_MultipartTempFileCleanupAndSanitization(t *testing.T) {
	rt := installHandlerRuntime(t, nil, `
public class UploadController {
    public func upload(): string {
        return "ok";
    }
}
`)
	rt.Routes[http.MethodPost] = map[string]interface{}{
		"/api/upload": map[string]interface{}{"handler": "UploadController@upload", "middleware": []string{}},
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("avatar", "../../evil_upload_test.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("hello upload"))
	_ = mw.Close()

	// 1. Test decodeRequestData directly to inspect temp file path
	req1 := httptest.NewRequest(http.MethodPost, "http://example.test/api/upload", bytes.NewReader(body.Bytes()))
	req1.Header.Set("Content-Type", mw.FormDataContentType())
	data := decodeRequestData(req1, "example.test", "http")
	filesMap, ok := data["_files"].(map[string]interface{})
	if !ok {
		t.Fatal("expected _files map")
	}
	avatarMap, ok := filesMap["avatar"].(map[string]interface{})
	if !ok {
		t.Fatal("expected avatar entry")
	}
	tempPath, _ := avatarMap["path"].(string)
	if tempPath == "" {
		t.Fatalf("VULN-003: temp file creation failed when filename contained path traversal '../..': path is empty")
	}
	_ = os.Remove(tempPath)

	// 2. Test full MainHandler lifecycle cleans up temporary upload files
	req2 := httptest.NewRequest(http.MethodPost, "http://example.test/api/upload", bytes.NewReader(body.Bytes()))
	req2.Header.Set("Content-Type", mw.FormDataContentType())
	rec2 := httptest.NewRecorder()

	beforeEntries, _ := os.ReadDir(os.TempDir())
	beforeSet := make(map[string]bool, len(beforeEntries))
	for _, e := range beforeEntries {
		beforeSet[e.Name()] = true
	}

	MainHandler(rec2, req2)

	afterEntries, _ := os.ReadDir(os.TempDir())
	for _, e := range afterEntries {
		if !beforeSet[e.Name()] && strings.HasPrefix(e.Name(), "joss_upload_") {
			_ = os.Remove(os.TempDir() + string(os.PathSeparator) + e.Name())
			t.Fatalf("VULN-003: temporary upload file %s leaked on disk after MainHandler finished", e.Name())
		}
	}
}

// PoC JOSS-SEC-VULN-005 (B): Session Fixation / Cookie Session ID arbitrario
func TestSecurityAudit_VULN005_MalformedSessionIDRejected(t *testing.T) {
	installHandlerRuntime(t, map[string]string{"SESSION_DRIVER": "memory"}, "")

	req := httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil)
	maliciousID := "../../etc/passwd_evil"
	req.AddCookie(&http.Cookie{Name: "joss_session", Value: maliciousID})
	rec := httptest.NewRecorder()

	MainHandler(rec, req)

	sessionMu.Lock()
	_, exists := sessionStore[maliciousID]
	sessionMu.Unlock()

	if exists {
		t.Fatalf("VULN-005: Server accepted arbitrary non-hex session ID %q into sessionStore", maliciousID)
	}
}

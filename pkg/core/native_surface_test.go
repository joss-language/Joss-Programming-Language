package core

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestDocumentedNativeSurfaceIsRegistered(t *testing.T) {
	r := NewRuntime()

	want := map[string][]string{
		"Router":      {"group", "middleware", "registerMiddleware", "ws"},
		"Request":     {"input", "post", "all", "except", "file", "cookie", "root", "header"},
		"Response":    {"json", "error", "redirect", "back", "raw", "stream"},
		"Schema":      {"create", "table", "rename", "drop", "dropIfExists", "hasTable", "hasColumn", "hasForeign"},
		"System":      {"env", "Run", "load_driver", "driver_call", "log", "sleep", "now"},
		"Blueprint":   {"id", "string", "timestamps", "nullable", "unique", "default", "dropColumn", "renameColumn", "index", "uniqueIndex", "dropIndex", "foreign", "references", "on", "onDelete", "onUpdate"},
		"MFA":         {"generateTOTP", "verifyTOTP", "generateRecoveryCodes", "verifyRecoveryCode"},
		"TwoFactor":   {"verify", "required"},
		"UserStorage": {"put", "get", "getToFile", "delete"},
	}

	for className, methods := range want {
		class, ok := r.Classes[className]
		if !ok {
			t.Fatalf("native class %s is not registered", className)
		}
		registered := make(map[string]bool)
		for _, statement := range class.Body.Statements {
			if method, ok := statement.(*parser.MethodStatement); ok {
				registered[method.Name.Value] = true
			}
		}
		for _, method := range methods {
			if !registered[method] {
				t.Errorf("%s::%s is documented but not registered", className, method)
			}
		}
	}
}

func TestResponseStatusAndRequestDefaults(t *testing.T) {
	r := NewRuntime()

	errorResponse := r.executeResponseMethod(nil, "error", []interface{}{"bad input", 422}).(*Instance)
	if got := errorResponse.Fields["status_code"]; got != 422 {
		t.Fatalf("Response::error status = %v, want 422", got)
	}
	errorData := errorResponse.Fields["data"].(map[string]interface{})
	if got := errorData["error"]; got != "bad input" {
		t.Fatalf("Response::error data = %v", got)
	}

	redirect := r.executeResponseMethod(nil, "redirect", []interface{}{"/next", 301}).(*Instance)
	if got := redirect.Fields["status_code"]; got != 301 {
		t.Fatalf("Response::redirect status = %v, want 301", got)
	}

	r.Variables["$__request"] = &Instance{Fields: map[string]interface{}{
		"_cookies": map[string]interface{}{},
	}}
	if got := r.executeRequestMethod(nil, "input", []interface{}{"missing", "fallback"}); got != "fallback" {
		t.Fatalf("Request::input default = %v", got)
	}
	if got := r.executeRequestMethod(nil, "cookie", []interface{}{"missing", "fallback"}); got != "fallback" {
		t.Fatalf("Request::cookie default = %v", got)
	}
}

func TestRedisConnectCanInitializeTheNativeClient(t *testing.T) {
	previous := GlobalRedis
	GlobalRedis = nil
	t.Cleanup(func() {
		if GlobalRedis != nil {
			_ = GlobalRedis.Close()
		}
		GlobalRedis = previous
	})

	r := NewRuntime()
	if got := r.executeRedisMethod(nil, "connect", []interface{}{"127.0.0.1:6379"}); got != true {
		t.Fatalf("Redis::connect returned %v", got)
	}
	if GlobalRedis == nil {
		t.Fatal("Redis::connect did not initialize the client")
	}
}

func TestZipExtractRejectsSiblingPrefixTraversal(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "unsafe.zip")
	destination := filepath.Join(root, "target")

	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	entry, err := writer.Create("../target-escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("unsafe")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	if got := r.executeZipMethod(nil, "extract", []interface{}{archivePath, destination}); got != false {
		t.Fatalf("Zip::extract traversal returned %v, want false", got)
	}
	if _, err := os.Stat(filepath.Join(root, "target-escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("Zip::extract wrote outside destination: %v", err)
	}
}

func TestUserStoragePathCannotEscapeItsRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := safeUserStoragePath(root, "user", "../escape.txt"); err == nil {
		t.Fatal("safeUserStoragePath accepted parent traversal")
	}
	if _, err := safeUserStoragePath(root, "../other-user", "file.txt"); err == nil {
		t.Fatal("safeUserStoragePath accepted a traversal token")
	}
	got, err := safeUserStoragePath(root, "user", "photos/avatar.jpg")
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(root, got)
	if err != nil || relative != filepath.Join("user", "photos", "avatar.jpg") {
		t.Fatalf("safe path = %q, relative = %q, err = %v", got, relative, err)
	}
}

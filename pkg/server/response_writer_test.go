package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/core"
)

func TestWriteExecutionResultMapping(t *testing.T) {
	runtime := core.NewRuntime()
	defer runtime.Free()
	request := httptest.NewRequest(http.MethodGet, "http://example.test/source", nil)
	tests := []struct {
		name        string
		result      interface{}
		wantHandled bool
		wantStatus  int
		wantType    string
		wantBody    string
		wantHeader  string
	}{
		{name: "string", result: "hello", wantHandled: true, wantStatus: 200, wantType: "text/html", wantBody: "hello"},
		{name: "json map", result: map[string]interface{}{"_type": "JSON", "status_code": int64(201), "data": map[string]interface{}{"ok": true}}, wantHandled: true, wantStatus: 201, wantType: "application/json", wantBody: `"ok":true`},
		{name: "raw bytes and headers", result: map[string]interface{}{"_type": "RAW", "status_code": 202, "content_type": "application/octet-stream", "headers": map[string]interface{}{"X-Joss": "yes"}, "data": []byte("raw")}, wantHandled: true, wantStatus: 202, wantType: "application/octet-stream", wantBody: "raw", wantHeader: "yes"},
		{name: "instance json status", result: &core.Instance{Fields: map[string]interface{}{"_type": "JSON", "status_code": int64(207), "data": true}}, wantHandled: true, wantStatus: 207, wantType: "application/json", wantBody: "true"},
		{name: "unsupported number", result: int64(42), wantHandled: false, wantStatus: 200},
		{name: "unsupported null", result: nil, wantHandled: false, wantStatus: 200},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handled := writeExecutionResult(response, request, runtime, test.result, responseSession{})
			if handled != test.wantHandled {
				t.Fatalf("handled = %v, want %v", handled, test.wantHandled)
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantType != "" && !strings.Contains(response.Header().Get("Content-Type"), test.wantType) {
				t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
			}
			if test.wantBody != "" && !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("body = %q", response.Body.String())
			}
			if test.wantHeader != "" && response.Header().Get("X-Joss") != test.wantHeader {
				t.Fatalf("X-Joss = %q", response.Header().Get("X-Joss"))
			}
		})
	}
}

func TestWriteExecutionResultRedirectCookiesAndFile(t *testing.T) {
	runtime := core.NewRuntime()
	defer runtime.Free()
	request := httptest.NewRequest(http.MethodGet, "https://example.test/source", nil)
	redirect := &core.Instance{Fields: map[string]interface{}{
		"_type": "REDIRECT", "url": "/next", "status_code": int64(307),
		"headers": map[string]interface{}{"X-Result": "redirect"},
		"cookies": map[string]interface{}{"token": "abc"},
	}}
	response := httptest.NewRecorder()
	if !writeExecutionResult(response, request, runtime, redirect, responseSession{}) {
		t.Fatal("redirect not handled")
	}
	if response.Code != 307 || response.Header().Get("Location") != "/next" || response.Header().Get("X-Result") != "redirect" {
		t.Fatalf("redirect = %d %#v", response.Code, response.Header())
	}
	if cookies := response.Result().Cookies(); len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %#v", cookies)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte("download"), 0600); err != nil {
		t.Fatal(err)
	}
	fileResponse := httptest.NewRecorder()
	file := &core.Instance{Fields: map[string]interface{}{"_type": "FILE", "data": path, "download_name": "report.txt"}}
	if !writeExecutionResult(fileResponse, request, runtime, file, responseSession{}) {
		t.Fatal("file not handled")
	}
	if fileResponse.Code != 200 || fileResponse.Body.String() != "download" || !strings.Contains(fileResponse.Header().Get("Content-Disposition"), "report.txt") {
		t.Fatalf("file response = %d %#v %q", fileResponse.Code, fileResponse.Header(), fileResponse.Body.String())
	}
}

func TestJSONResponseIsValidJSON(t *testing.T) {
	runtime := core.NewRuntime()
	defer runtime.Free()
	response := httptest.NewRecorder()
	writeExecutionResult(response, httptest.NewRequest(http.MethodGet, "http://example.test", nil), runtime, map[string]interface{}{"_type": "JSON", "data": []interface{}{int64(1), true}}, responseSession{})
	var decoded interface{}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
}

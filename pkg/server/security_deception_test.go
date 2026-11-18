package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/core"
)

func TestSecurityDeceptionConverts403To404(t *testing.T) {
	runtime := core.NewRuntime()
	defer runtime.Free()
	runtime.Env["SECURITY_DECEPTION"] = "true"

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.test/admin/secret", nil)

	fields := map[string]interface{}{
		"_type":       "JSON",
		"status_code": 403,
		"data":        map[string]interface{}{"error": "Forbidden"},
	}

	handled := writeResponseFields(response, request, runtime, fields, responseSession{}, true)
	if !handled {
		t.Fatal("expected response to be handled")
	}

	// Should deceive the attacker: status 404 instead of 403
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 under SECURITY_DECEPTION, got %d", response.Code)
	}

	body := response.Body.String()
	if !strings.Contains(body, "404") || !strings.Contains(body, "Página no encontrada") {
		t.Fatalf("expected styled 404 error page, got: %s", body)
	}
}

func TestSecurityDeceptionDisabledKeepsOriginalStatus(t *testing.T) {
	runtime := core.NewRuntime()
	defer runtime.Free()
	runtime.Env["SECURITY_DECEPTION"] = "false"

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.test/admin/secret", nil)

	fields := map[string]interface{}{
		"_type":       "JSON",
		"status_code": 403,
		"data":        map[string]interface{}{"error": "Forbidden"},
	}

	handled := writeResponseFields(response, request, runtime, fields, responseSession{}, true)
	if !handled {
		t.Fatal("expected response to be handled")
	}

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 when deception disabled, got %d", response.Code)
	}
}

func TestRenderErrorPageJSONWhenClientRequestsJSON(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/unknown", nil)
	request.Header.Set("Accept", "application/json")

	renderErrorPage(response, request, nil, http.StatusNotFound, "Not Found", "Resource not found")

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
	if !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected application/json Content-Type, got %s", response.Header().Get("Content-Type"))
	}
	if !strings.Contains(response.Body.String(), `"status":404`) {
		t.Fatalf("expected JSON body with status:404, got %s", response.Body.String())
	}
}

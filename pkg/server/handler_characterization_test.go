package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/parser"
)

func installHandlerRuntime(t *testing.T, env map[string]string, source string) *core.Runtime {
	t.Helper()
	runtime := core.NewRuntime()
	for key, value := range env {
		runtime.Env[key] = value
	}
	if source != "" {
		p := parser.NewParser(parser.NewLexer(source))
		program := p.ParseProgram()
		if errors := p.Errors(); len(errors) != 0 {
			runtime.Free()
			t.Fatalf("parse errors: %v", errors)
		}
		for _, statement := range program.Statements {
			if class, ok := statement.(*parser.ClassStatement); ok && class.Name != nil {
				runtime.Classes[class.Name.Value] = class
			}
		}
	}
	mutex.Lock()
	previous := currentRuntime
	currentRuntime = runtime
	mutex.Unlock()
	sessionMu.Lock()
	previousSessions := sessionStore
	sessionStore = make(map[string]map[string]interface{})
	sessionMu.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		currentRuntime = previous
		mutex.Unlock()
		sessionMu.Lock()
		sessionStore = previousSessions
		sessionMu.Unlock()
		runtime.Free()
	})
	return runtime
}

func TestMainHandlerWebSocketUpgradeAndCleanup(t *testing.T) {
	installHandlerRuntime(t, nil, "")
	server := httptest.NewServer(http.HandlerFunc(MainHandler))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/unregistered"
	connection, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("websocket upgrade failed: %v", err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := connection.ReadMessage(); err == nil {
		t.Fatal("unregistered websocket route remained open")
	}

	response, err := http.Get(server.URL + "/after-websocket")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusInternalServerError {
		t.Fatal("runtime remained contaminated after websocket close")
	}
}

func TestMainHandlerRejectsInvalidWebSocketUpgrade(t *testing.T) {
	installHandlerRuntime(t, nil, "")
	request := httptest.NewRequest(http.MethodGet, "http://example.test/socket", nil)
	request.Header.Set("Upgrade", "websocket")
	response := httptest.NewRecorder()
	MainHandler(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid upgrade status = %d, want 400", response.Code)
	}
}

func TestMainHandlerWithoutRuntimeReturnsServiceUnavailable(t *testing.T) {
	mutex.Lock()
	previous := currentRuntime
	currentRuntime = nil
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		currentRuntime = previous
		mutex.Unlock()
	})
	response := httptest.NewRecorder()
	MainHandler(response, httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}

func TestMainHandlerCORSPolicyCharacterization(t *testing.T) {
	tests := []struct {
		name        string
		policy      string
		origin      string
		wantOrigin  string
		credentials string
	}{
		{name: "explicit allowed", policy: "https://allowed.test, https://other.test", origin: "https://allowed.test", wantOrigin: "https://allowed.test", credentials: "true"},
		{name: "explicit denied", policy: "https://allowed.test", origin: "https://denied.test"},
		{name: "wildcard", policy: "*", origin: "https://any.test", wantOrigin: "*"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installHandlerRuntime(t, map[string]string{"CORS_WEB": test.policy}, "")
			request := httptest.NewRequest(http.MethodOptions, "http://example.test/resource", nil)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			MainHandler(response, request)
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.wantOrigin {
				t.Fatalf("Allow-Origin = %q, want %q", got, test.wantOrigin)
			}
			if got := response.Header().Get("Access-Control-Allow-Credentials"); got != test.credentials {
				t.Fatalf("Allow-Credentials = %q, want %q", got, test.credentials)
			}
			if test.wantOrigin != "" && response.Code != http.StatusOK {
				t.Fatalf("preflight status = %d, want 200", response.Code)
			}
		})
	}
}

func TestMainHandlerSessionAndCSRFCharacterization(t *testing.T) {
	installHandlerRuntime(t, nil, "")
	first := httptest.NewRecorder()
	MainHandler(first, httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil))
	cookies := first.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "joss_session" {
		t.Fatalf("first request did not create joss_session: %#v", cookies)
	}
	sessionID := cookies[0].Value
	sessionMu.Lock()
	token, _ := sessionStore[sessionID]["csrf_token"].(string)
	sessionMu.Unlock()
	if token == "" {
		t.Fatal("session did not receive a CSRF token")
	}

	missing := httptest.NewRequest(http.MethodPost, "http://example.test/form", strings.NewReader("name=joss"))
	missing.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missing.AddCookie(cookies[0])
	missingResponse := httptest.NewRecorder()
	MainHandler(missingResponse, missing)
	if missingResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, want 403", missingResponse.Code)
	}

	valid := httptest.NewRequest(http.MethodPost, "http://example.test/form", nil)
	valid.AddCookie(cookies[0])
	valid.Header.Set("X-CSRF-TOKEN", token)
	validResponse := httptest.NewRecorder()
	MainHandler(validResponse, valid)
	if validResponse.Code == http.StatusForbidden {
		t.Fatal("valid CSRF token was rejected")
	}

	api := httptest.NewRequest(http.MethodPost, "http://example.test/api/missing", nil)
	apiResponse := httptest.NewRecorder()
	MainHandler(apiResponse, api)
	if apiResponse.Code == http.StatusForbidden {
		t.Fatal("API route unexpectedly required CSRF")
	}
}

func TestMainHandlerMapsJossStringResult(t *testing.T) {
	runtime := installHandlerRuntime(t, nil, `
public class TestController {
    public func show(): string { return "hello from joss"; }
}
`)
	runtime.Routes[http.MethodGet] = map[string]interface{}{
		"/hello": map[string]interface{}{"handler": "TestController@show", "middleware": []string{}},
	}
	response := httptest.NewRecorder()
	MainHandler(response, httptest.NewRequest(http.MethodGet, "http://example.test/hello", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "hello from joss") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want HTML", got)
	}
}

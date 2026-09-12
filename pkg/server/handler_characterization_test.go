package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	runtime.Env["SESSION_DRIVER"] = "memory"
	for key, value := range env {
		runtime.Env[key] = value
	}
	if strings.TrimSpace(runtime.Env["SESSION_DRIVER"]) == "" {
		runtime.Env["SESSION_DRIVER"] = "memory"
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
	previousSessionPath := loadedSessionPath
	sessionStore = make(map[string]map[string]interface{})
	loadedSessionPath = ""
	sessionMu.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		currentRuntime = previous
		mutex.Unlock()
		sessionMu.Lock()
		sessionStore = previousSessions
		loadedSessionPath = previousSessionPath
		sessionMu.Unlock()
		runtime.Free()
	})
	return runtime
}

func TestMainHandlerRejectsUnavailableAndCorruptSessionBackends(t *testing.T) {
	t.Run("redis unavailable", func(t *testing.T) {
		previousRedis := core.GlobalRedis
		core.GlobalRedis = nil
		t.Cleanup(func() { core.GlobalRedis = previousRedis })
		installHandlerRuntime(t, map[string]string{"SESSION_DRIVER": "redis"}, "")
		response := httptest.NewRecorder()
		MainHandler(response, httptest.NewRequest(http.MethodGet, "http://example.test/account", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", response.Code)
		}
	})

	t.Run("file corrupt", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sessions.json")
		if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		installHandlerRuntime(t, map[string]string{"SESSION_DRIVER": "file", "SESSION_FILE": path}, "")
		response := httptest.NewRecorder()
		MainHandler(response, httptest.NewRequest(http.MethodGet, "http://example.test/account", nil))
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", response.Code)
		}
	})
}

func TestSessionBackendSnapshotsAreIsolatedAndFailedWritesRollback(t *testing.T) {
	installHandlerRuntime(t, map[string]string{"SESSION_DRIVER": "memory"}, "")
	if err := saveSession(map[string]string{"SESSION_DRIVER": "memory"}, "memory", "a", map[string]interface{}{"user": "Ada"}); err != nil {
		t.Fatal(err)
	}
	first, _, err := loadSession(map[string]string{"SESSION_DRIVER": "memory"}, "a")
	if err != nil {
		t.Fatal(err)
	}
	first["user"] = "Grace"
	second, _, err := loadSession(map[string]string{"SESSION_DRIVER": "memory"}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if second["user"] != "Ada" {
		t.Fatal("request-local session mutation leaked before save")
	}

	path := filepath.Join(t.TempDir(), "sessions.json")
	env := map[string]string{"SESSION_DRIVER": "file", "SESSION_FILE": path}
	sessionMu.Lock()
	loadedSessionPath = path
	sessionStore["rollback"] = map[string]interface{}{"stable": true}
	sessionMu.Unlock()
	if err := saveSession(env, "file", "rollback", map[string]interface{}{"bad": make(chan int)}); err == nil {
		t.Fatal("non-serializable session unexpectedly persisted")
	}
	sessionMu.Lock()
	stable := sessionStore["rollback"]["stable"]
	_, badExists := sessionStore["rollback"]["bad"]
	sessionMu.Unlock()
	if stable != true || badExists {
		t.Fatal("failed persistence changed the in-memory session")
	}
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

func TestSessionClientIsolationAcrossMultipleRequests(t *testing.T) {
	installHandlerRuntime(t, map[string]string{"SESSION_DRIVER": "memory"}, "")

	// Client A request 1
	recA1 := httptest.NewRecorder()
	reqA1 := httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil)
	MainHandler(recA1, reqA1)
	cookieA := recA1.Result().Cookies()
	if len(cookieA) == 0 {
		t.Fatal("expected session cookie for Client A")
	}

	// Client B request 1
	recB1 := httptest.NewRecorder()
	reqB1 := httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil)
	MainHandler(recB1, reqB1)
	cookieB := recB1.Result().Cookies()
	if len(cookieB) == 0 {
		t.Fatal("expected session cookie for Client B")
	}

	if cookieA[0].Value == cookieB[0].Value {
		t.Fatalf("clients A and B received identical session ID: %q", cookieA[0].Value)
	}

	// Client A request 2 using cookieA
	recA2 := httptest.NewRecorder()
	reqA2 := httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil)
	reqA2.AddCookie(cookieA[0])
	MainHandler(recA2, reqA2)

	// Client B request 2 using cookieB
	recB2 := httptest.NewRecorder()
	reqB2 := httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil)
	reqB2.AddCookie(cookieB[0])
	MainHandler(recB2, reqB2)

	sessionMu.Lock()
	dataA := sessionStore[cookieA[0].Value]
	dataB := sessionStore[cookieB[0].Value]
	sessionMu.Unlock()

	if dataA == nil || dataB == nil {
		t.Fatal("sessions not found in store")
	}
	if dataA["csrf_token"] == dataB["csrf_token"] {
		t.Fatal("CSRF tokens must be distinct across clients")
	}
}

func TestRedirectFlashPersistenceAndFailureHandling(t *testing.T) {
	rt := installHandlerRuntime(t, map[string]string{"SESSION_DRIVER": "memory"}, "")
	sessionID := "flash_test_session"
	sessionMu.Lock()
	sessionStore[sessionID] = map[string]interface{}{"csrf_token": "token123"}
	sessionMu.Unlock()

	session := responseSession{id: sessionID, driver: "memory", data: map[string]interface{}{"csrf_token": "token123"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/redir", nil)

	// Successful flash persistence
	instSuccess := &core.Instance{
		Fields: map[string]interface{}{
			"_type": "REDIRECT",
			"url":   "/login",
			"flash": map[string]interface{}{"message": "Welcome back"},
		},
	}
	handled := writeExecutionResult(rec, req, rt, instSuccess, session)
	if !handled || rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Fatalf("redirect failed: handled=%v code=%d", handled, rec.Code)
	}

	sessionMu.Lock()
	storedFlash := sessionStore[sessionID]["message"]
	sessionMu.Unlock()
	if storedFlash != "Welcome back" {
		t.Fatalf("flash not persisted: got %v", storedFlash)
	}

	// Failed flash persistence (non-serializable object with file driver)
	path := filepath.Join(t.TempDir(), "sessions.json")
	rt.Env["SESSION_DRIVER"] = "file"
	rt.Env["SESSION_FILE"] = path
	sessionFile := responseSession{id: sessionID, driver: "file", data: map[string]interface{}{"csrf_token": "token123"}}
	recFail := httptest.NewRecorder()
	instFail := &core.Instance{
		Fields: map[string]interface{}{
			"_type": "REDIRECT",
			"url":   "/dashboard",
			"flash": map[string]interface{}{"channel": make(chan int)},
		},
	}
	handledFail := writeExecutionResult(recFail, req, rt, instFail, sessionFile)
	if !handledFail || recFail.Code != http.StatusInternalServerError {
		t.Fatalf("failed flash must return 500: handled=%v code=%d", handledFail, recFail.Code)
	}
	if recFail.Header().Get("Location") != "" {
		t.Fatal("failed flash persistence must not set redirect Location")
	}
}

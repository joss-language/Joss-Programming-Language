package core

import (
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func installJossHTTPParser(t *testing.T, runtime *Runtime) {
	t.Helper()
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path != "<std/http/request.joss>" || unit.Program == nil {
			continue
		}
		for _, statement := range unit.Program.Statements {
			if class, ok := statement.(*parser.ClassStatement); ok && class.Name != nil && class.Name.Value == "HttpRequestParser" {
				runtime.registerClass(class)
				return
			}
		}
	}
	t.Fatal("canonical std/http/request.joss parser was not embedded")
}

func installJossAuthPolicy(t *testing.T, runtime *Runtime) {
	t.Helper()
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path != "<std/auth/auth.joss>" || unit.Program == nil {
			continue
		}
		for _, statement := range unit.Program.Statements {
			if class, ok := statement.(*parser.ClassStatement); ok && class.Name != nil && class.Name.Value == "Auth" {
				runtime.registerClass(class)
				return
			}
		}
	}
	t.Fatal("canonical std/auth/auth.joss policy was not embedded")
}

func TestJossAuthPolicyUsesCookieHydratedRequestIdentity(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossAuthPolicy(t, runtime)
	runtime.Env["JWT_SECRET"] = "test-auth-cookie-secret"
	token := runtime.generateJWT(42, "ada@example.test", "ada", "client", false)
	runtime.Variables["$__request"] = &Instance{Fields: map[string]interface{}{
		"_cookies": map[string]interface{}{"joss_token": token},
	}}
	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{}}
	runtime.hydrateAuthSessionFromRequest()

	class := runtime.Classes["Auth"]
	methods := map[string]*parser.MethodStatement{}
	for _, statement := range class.Body.Statements {
		if method, ok := statement.(*parser.MethodStatement); ok && method.Name != nil {
			methods[method.Name.Value] = method
		}
	}
	if runtime.CallMethodEvaluated(methods["check"], nil, nil) != true {
		t.Fatal("Joss Auth::check did not recognize the hydrated session identity")
	}
	if got := runtime.CallMethodEvaluated(methods["id"], nil, nil); got != 42 {
		t.Fatalf("Joss Auth::id returned %#v after cookie hydration, want user id 42", got)
	}
	if runtime.CallMethodEvaluated(methods["guest"], nil, nil) != false {
		t.Fatal("Joss Auth::guest disagreed with Auth::check for the same session")
	}
	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{}}
	if got := runtime.CallMethodEvaluated(methods["id"], nil, nil); got != nil {
		t.Fatalf("Joss Auth::id for a guest = %#v, want nil", got)
	}
	if runtime.CallMethodEvaluated(methods["check"], nil, nil) != false || runtime.CallMethodEvaluated(methods["guest"], nil, nil) != true {
		t.Fatal("Joss Auth::check/Auth::guest disagree for a guest session")
	}
}

func TestAuthBcryptPrimitiveVerifiesWithoutHandlingFlow(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()

	hash := runtime.executeAuthMethod(nil, "hash", []interface{}{"correct horse battery staple"})
	encoded, ok := hash.(string)
	if !ok || encoded == "" {
		t.Fatal("Auth::hash did not return a bcrypt hash")
	}
	if got := runtime.executeAuthMethod(nil, "verifyHash", []interface{}{encoded, "correct horse battery staple"}); got != true {
		t.Fatal("Auth::verifyHash rejected the matching password")
	}
	if got := runtime.executeAuthMethod(nil, "verifyHash", []interface{}{encoded, "wrong password"}); got != false {
		t.Fatal("Auth::verifyHash accepted a different password")
	}
}

func TestAuthDatabaseReadyRejectsMissingConnection(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	if got := runtime.executeAuthMethod(nil, "databaseReady", nil); got != false {
		t.Fatalf("Auth::databaseReady without a DB returned %#v, want false", got)
	}
}

func TestParseAuthExpiryUsesLocalDatabaseDatetime(t *testing.T) {
	want := time.Now().Add(time.Hour).Truncate(time.Second)
	localFields := want.Format("2006-01-02 15:04:05")
	for _, raw := range []string{localFields, time.Date(
		want.Year(), want.Month(), want.Day(), want.Hour(), want.Minute(), want.Second(), 0, time.UTC,
	).Format(time.RFC3339)} {
		got, ok := parseAuthExpiry(raw)
		if !ok {
			t.Fatalf("parseAuthExpiry rejected local DATETIME %q", raw)
		}
		if delta := got.Sub(want); delta < -time.Second || delta > time.Second {
			t.Fatalf("parsed local DATETIME %q differs by %s, want same wall-clock instant", raw, delta)
		}
	}
}

func TestJossAuthHasRolePolicyUsesRequestSession(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossAuthPolicy(t, runtime)

	class := runtime.Classes["Auth"]
	methods := map[string]*parser.MethodStatement{}
	for _, statement := range class.Body.Statements {
		if method, ok := statement.(*parser.MethodStatement); ok && method.Name != nil {
			methods[method.Name.Value] = method
		}
	}
	if methods["hasRole"] == nil {
		t.Fatal("Pure Joss Auth::hasRole implementation was not registered")
	}
	call := func(role string) interface{} {
		return runtime.CallMethodEvaluated(methods["hasRole"], nil, []interface{}{role})
	}

	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{"user_role": "client"}}
	if got := call("client"); got != true {
		t.Fatalf("matching session role = %#v, want true", got)
	}
	if got := call("admin"); got != false {
		t.Fatalf("client session granting admin = %#v, want false", got)
	}

	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{"user_role": "admin"}}
	if got := call("client"); got != true {
		t.Fatalf("admin session granting client role = %#v, want true", got)
	}

	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{}}
	if got := call("client"); got != false {
		t.Fatalf("guest session granting client role = %#v, want false", got)
	}
}

func TestJossAuthLogoutClearsRequestSession(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossAuthPolicy(t, runtime)
	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{
		"user_id":       42,
		"user_token":    "secret",
		"user_name":     "Ada",
		"user_email":    "ada@example.test",
		"user_role":     "admin",
		"last_login_at": "2026-11-12 10:00:00",
	}}

	logout := authMethod(t, runtime, "logout")
	if got := runtime.CallMethodEvaluated(logout, nil, nil); got != true {
		t.Fatalf("Joss Auth::logout returned %#v, want true", got)
	}
	session := runtime.Variables["$__session"].(*Instance)
	for _, key := range []string{"user_id", "user_token", "user_name", "user_email", "user_role", "last_login_at"} {
		if _, exists := session.Fields[key]; exists {
			t.Errorf("Joss Auth::logout left session key %q", key)
		}
	}
}

func authMethod(t *testing.T, runtime *Runtime, name string) *parser.MethodStatement {
	t.Helper()
	class := runtime.Classes["Auth"]
	if class == nil || class.Body == nil {
		t.Fatal("Joss Auth class was not registered")
	}
	for _, statement := range class.Body.Statements {
		method, ok := statement.(*parser.MethodStatement)
		if ok && method.Name != nil && method.Name.Value == name {
			return method
		}
	}
	t.Fatalf("Joss Auth::%s was not registered", name)
	return nil
}

func TestJossHTTPParserPreservesAuthFormAndCookies(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossHTTPParser(t, runtime)
	raw := "POST /login?next=%2Fdashboard HTTP/1.1\r\n" +
		"Host: example.test\r\n" +
		"Content-Type: application/x-www-form-urlencoded\r\n" +
		"Content-Length: 40\r\n" +
		"Cookie: joss_token=access-token; theme=dark\r\n" +
		"Referer: http://example.test/login\r\n\r\n" +
		"email=ada%40example.test&password=secret"
	requestData := runtime.parseHTTPWithStdlib(raw)
	if requestData == nil {
		t.Fatal("Joss HTTP parser returned no request map")
	}
	if requestData["email"] != "ada@example.test" || requestData["password"] != "secret" {
		t.Fatalf("form fields were not decoded: %#v", requestData)
	}
	if requestData["_method"] != "POST" || requestData["_path"] != "/login" || requestData["_query"] != "next=%2Fdashboard" {
		t.Fatalf("request metadata was not preserved: %#v", requestData)
	}
	cookies, ok := requestData["_cookies"].(map[string]interface{})
	if !ok || cookies["joss_token"] != "access-token" || cookies["theme"] != "dark" {
		t.Fatalf("cookies were not decoded: %#v", requestData["_cookies"])
	}
	if requestData["_referer"] != "http://example.test/login" {
		t.Fatalf("referer was not preserved: %#v", requestData["_referer"])
	}
}

func TestJossHTTPParserAcceptsJSONPayloads(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossHTTPParser(t, runtime)
	raw := "POST /api/login HTTP/1.1\r\n" +
		"Host: example.test\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 48\r\n\r\n" +
		`{"email":"ada@example.test","password":"secret"}`
	requestData := runtime.parseHTTPWithStdlib(raw)
	if requestData == nil {
		t.Fatal("Joss HTTP parser returned no request map")
	}
	if requestData["email"] != "ada@example.test" || requestData["password"] != "secret" {
		t.Fatalf("JSON payload was not decoded: %#v", requestData)
	}
}

func TestJossHTTPParserKeepsMalformedJSONBodyAsRawInput(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossHTTPParser(t, runtime)
	raw := "POST /api/login HTTP/1.1\r\n" +
		"Content-Type: application/json\r\n\r\n" +
		`{"email":broken}`
	requestData := runtime.parseHTTPWithStdlib(raw)
	if requestData == nil {
		t.Fatal("Joss HTTP parser returned no request map")
	}
	if requestData["_body"] != `{"email":broken}` {
		t.Fatalf("malformed JSON body was not preserved: %#v", requestData["_body"])
	}
	if _, exists := requestData["email"]; exists {
		t.Fatal("malformed JSON populated request fields")
	}
}

func TestNativeHTTPDispatchSelectsVerbAndPassesAuthInput(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossHTTPParser(t, runtime)
	program := parser.NewParser(parser.NewLexer(`
public class AuthDispatchProbe {
    public func show(): mixed { return json({"verb": Request::method()}) }
    public func submit(): mixed { return json({"verb": Request::method(), "email": Request::input("email")}) }
}`)).ParseProgram()
	class, ok := program.Statements[0].(*parser.ClassStatement)
	if !ok {
		t.Fatal("could not parse the Auth dispatch probe class")
	}
	runtime.registerClass(class)
	runtime.Routes["GET"] = map[string]interface{}{
		"/login": map[string]interface{}{"handler": "AuthDispatchProbe@show", "middleware": []string{}},
	}
	runtime.Routes["POST"] = map[string]interface{}{
		"/login": map[string]interface{}{"handler": "AuthDispatchProbe@submit", "middleware": []string{}},
	}

	getRaw := "GET /login HTTP/1.1\r\nHost: example.test\r\n\r\n"
	getValue, _ := runtime.callBuiltinIO("__joss_http_dispatch", []interface{}{"GET", "/login", getRaw})
	getResponse := getValue.(string)
	if !strings.Contains(getResponse, `"verb":"GET"`) {
		t.Fatalf("GET route did not receive the request method: %s", getResponse)
	}

	postRaw := "POST /login HTTP/1.1\r\nHost: example.test\r\n" +
		"Content-Type: application/x-www-form-urlencoded\r\nContent-Length: 24\r\n\r\n" +
		"email=ada%40example.test"
	postValue, _ := runtime.callBuiltinIO("__joss_http_dispatch", []interface{}{"POST", "/login", postRaw})
	postResponse := postValue.(string)
	if !strings.Contains(postResponse, `"verb":"POST"`) || !strings.Contains(postResponse, `"email":"ada@example.test"`) {
		t.Fatalf("POST route did not receive method and form input: %s", postResponse)
	}

	parsedRequest := runtime.parseHTTPWithStdlib("POST /login HTTP/1.1\r\nHost: example.test\r\nContent-Type: application/json\r\n\r\n{\"email\":\"joss@example.test\"}")
	if parsedRequest == nil {
		t.Fatal("Joss HTTP parser returned no JSON request map")
	}
	parsedValue, _ := runtime.callBuiltinIO("__joss_http_dispatch", []interface{}{"POST", "/login", parsedRequest})
	parsedResponse := parsedValue.(string)
	if !strings.Contains(parsedResponse, `"verb":"POST"`) || !strings.Contains(parsedResponse, `"email":"joss@example.test"`) {
		t.Fatalf("Joss-parsed request map was not dispatched: %s", parsedResponse)
	}
}

func TestNativeHTTPDispatchPreservesSessionAcrossRequests(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	installJossHTTPParser(t, runtime)
	program := parser.NewParser(parser.NewLexer(`
public class SessionDispatchProbe {
    public func store(): mixed {
        Session::put("temp_2fa_token", "challenge-token")
        return json({"stored": true})
    }
    public func read(): mixed {
        return json({"challenge": Session::get("temp_2fa_token")})
    }
}`)).ParseProgram()
	class, ok := program.Statements[0].(*parser.ClassStatement)
	if !ok {
		t.Fatal("could not parse the session dispatch probe class")
	}
	runtime.registerClass(class)
	runtime.Routes["POST"] = map[string]interface{}{
		"/2fa/verify": map[string]interface{}{"handler": "SessionDispatchProbe@store", "middleware": []string{}},
	}
	runtime.Routes["GET"] = map[string]interface{}{
		"/2fa/verify": map[string]interface{}{"handler": "SessionDispatchProbe@read", "middleware": []string{}},
	}
	session := map[string]interface{}{}
	post, _ := runtime.callBuiltinIO("__joss_http_dispatch", []interface{}{
		"POST", "/2fa/verify", map[string]interface{}{"_path": "/2fa/verify"}, session,
		"Set-Cookie: joss_session=test-id; Path=/; HttpOnly; SameSite=Lax",
	})
	if !strings.Contains(post.(string), `"stored":true`) {
		t.Fatalf("session POST did not complete: %v", post)
	}
	if !strings.Contains(post.(string), "Set-Cookie: joss_session=test-id") {
		t.Fatalf("HTTP response omitted the session cookie: %v", post)
	}
	if session["temp_2fa_token"] != "challenge-token" {
		t.Fatalf("dispatch did not persist its session mutation: %#v", session)
	}
	get, _ := runtime.callBuiltinIO("__joss_http_dispatch", []interface{}{
		"GET", "/2fa/verify", map[string]interface{}{"_path": "/2fa/verify"}, session,
		"Set-Cookie: joss_session=test-id; Path=/; HttpOnly; SameSite=Lax",
	})
	if !strings.Contains(get.(string), `"challenge":"challenge-token"`) {
		t.Fatalf("next request could not read the prior session value: %v", get)
	}
}

func TestPureJossSessionRestoresCookieBackedChallenge(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	var sessionClass *parser.ClassStatement
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path != "<std/http/session.joss>" || unit.Program == nil {
			continue
		}
		for _, statement := range unit.Program.Statements {
			if class, ok := statement.(*parser.ClassStatement); ok && class.Name != nil && class.Name.Value == "Session" {
				sessionClass = class
				break
			}
		}
	}
	if sessionClass == nil {
		t.Fatal("canonical std/http/session.joss class was not embedded")
	}
	runtime.registerClass(sessionClass)
	methods := map[string]*parser.MethodStatement{}
	for _, statement := range sessionClass.Body.Statements {
		if method, ok := statement.(*parser.MethodStatement); ok && method.Name != nil {
			methods[method.Name.Value] = method
		}
	}
	start := methods["start"]
	save := methods["save"]
	if start == nil || save == nil {
		t.Fatal("Joss Session start/save methods are missing")
	}
	first, ok := runtime.CallMethodEvaluated(start, nil, []interface{}{""}).(*Instance)
	if !ok {
		t.Fatal("Session::start did not return a Session instance")
	}
	sessionID, ok := first.Fields["id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("Session::start returned invalid session id %#v", first.Fields["id"])
	}
	firstData := first.Fields["data"].(map[string]interface{})
	if firstData["csrf_token"] == nil || firstData["_token"] == nil {
		t.Fatalf("new web session did not receive a CSRF token: %#v", firstData)
	}
	firstData["temp_2fa_token"] = "challenge-token"
	runtime.CallMethodEvaluated(save, first, nil)
	second, ok := runtime.CallMethodEvaluated(start, nil, []interface{}{"joss_session=" + sessionID}).(*Instance)
	if !ok {
		t.Fatal("Session::start could not restore the cookie-backed session")
	}
	if got := second.Fields["data"].(map[string]interface{})["temp_2fa_token"]; got != "challenge-token" {
		t.Fatalf("restored session challenge = %#v, want challenge-token", got)
	}
}

func TestPureJossHTTPServerAddsSessionCookieToRedirect(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	response := "HTTP/1.1 303 See Other\r\nLocation: /2fa/verify\r\n\r\n"
	header := "Set-Cookie: joss_session=secure-id; Path=/; HttpOnly; SameSite=Lax"
	raw := appendNativeHTTPHeader(response, header)
	if !strings.Contains(raw, "Location: /2fa/verify\r\n"+header+"\r\n\r\n") {
		t.Fatalf("redirect response did not carry a session cookie header: %q", raw)
	}
	binaryBody := "\x00\xff\xfeimage-bytes"
	binaryResponse := "HTTP/1.1 200 OK\r\nContent-Type: image/png\r\nContent-Length: 14\r\n\r\n" + binaryBody
	binaryRaw := appendNativeHTTPHeader(binaryResponse, header)
	if !strings.HasSuffix(binaryRaw, "\r\n\r\n"+binaryBody) {
		t.Fatalf("adding session cookie corrupted binary response body: %q", binaryRaw)
	}
}

func TestDispatchHydratesAuthFromAccessCookie(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.Env["JWT_SECRET"] = "test-auth-cookie-secret"
	token := runtime.generateJWT(42, "ada@example.test", "ada", "client", false)
	if token == nil || token == false {
		t.Fatal("could not create test access token")
	}
	requestData := map[string]interface{}{
		"_cookies": map[string]interface{}{"joss_token": token},
	}
	runtime.Variables["$__request"] = &Instance{Fields: requestData}
	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{}}
	runtime.hydrateAuthSessionFromRequest()
	if runtime.currentAuthUserID() != 42 {
		t.Fatal("access cookie did not hydrate the request identity")
	}
	if runtime.executeAuthMethod(nil, "check", nil) != true || runtime.executeAuthMethod(nil, "guest", nil) != false {
		t.Fatal("Auth::check/Auth::guest disagree with the cookie-backed request identity")
	}
}

func TestDispatchRejectsInvalidAuthCookie(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.Env["JWT_SECRET"] = "test-auth-cookie-secret"
	runtime.Variables["$__request"] = &Instance{Fields: map[string]interface{}{
		"_cookies": map[string]interface{}{"joss_token": "not-a-jwt"},
	}}
	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{}}
	runtime.hydrateAuthSessionFromRequest()
	if runtime.currentAuthUserID() != 0 || runtime.executeAuthMethod(nil, "guest", nil) != true {
		t.Fatal("invalid access cookie authenticated the request")
	}
}

func TestFormatNativeHTTPResponseKeepsRedirectCookieAndStatus(t *testing.T) {
	raw := formatNativeHTTPResponse(&Instance{Fields: map[string]interface{}{
		"_type": "REDIRECT", "url": "/dashboard", "status_code": 303,
		"cookies": map[string]interface{}{"joss_token": "token-value"},
		"headers": map[string]interface{}{"Cache-Control": "no-store"},
	}})
	for _, expected := range []string{"HTTP/1.1 303 See Other", "Location: /dashboard", "Set-Cookie: joss_token=token-value", "HttpOnly", "Cache-Control: no-store"} {
		if !strings.Contains(raw, expected) {
			t.Errorf("response missing %q: %s", expected, raw)
		}
	}
	clear := formatNativeHTTPResponse(&Instance{Fields: map[string]interface{}{
		"_type": "REDIRECT", "url": "/login", "cookies": map[string]interface{}{"joss_token": ""},
	}})
	if !strings.Contains(clear, "Max-Age=0") {
		t.Errorf("empty auth cookie was not expired: %s", clear)
	}
}

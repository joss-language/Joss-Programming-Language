package native_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
	"golang.org/x/crypto/bcrypt"
)

func buildNativeTestServerExe(t *testing.T, source string, name string) string {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: name + ".joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: name + ".joss"})
	irProg, err := ir.LowerAndVerify(prep, name, reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}
	outExe := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}
	if _, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	}); err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}
	return outExe
}

type mockRESPServer struct {
	ln       net.Listener
	mu       sync.Mutex
	store    map[string]string
	cmds     []string
	password string
}

func startMockRESPServer(t *testing.T, password string) *mockRESPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock redis: %v", err)
	}
	s := &mockRESPServer{
		ln:       ln,
		store:    make(map[string]string),
		password: password,
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handleConn(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *mockRESPServer) Port() int {
	return s.ln.Addr().(*net.TCPAddr).Port
}

func readRESPCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("expected *, got %q", line)
	}
	n, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, n)
	for i := 0; i < n; i++ {
		szLine, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		szLine = strings.TrimRight(szLine, "\r\n")
		if !strings.HasPrefix(szLine, "$") {
			return nil, fmt.Errorf("expected $, got %q", szLine)
		}
		sz, err := strconv.Atoi(szLine[1:])
		if err != nil {
			return nil, err
		}
		buf := make([]byte, sz+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:sz]))
	}
	return args, nil
}

func (s *mockRESPServer) handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		args, err := readRESPCommand(r)
		if err != nil || len(args) == 0 {
			return
		}
		cmd := strings.ToUpper(args[0])
		s.mu.Lock()
		s.cmds = append(s.cmds, strings.Join(args, " "))
		switch cmd {
		case "AUTH":
			if len(args) >= 2 && args[1] == s.password {
				_, _ = conn.Write([]byte("+OK\r\n"))
			} else {
				_, _ = conn.Write([]byte("-ERR invalid password\r\n"))
			}
		case "SELECT":
			_, _ = conn.Write([]byte("+OK\r\n"))
		case "GET":
			val, ok := s.store[args[1]]
			if !ok {
				_, _ = conn.Write([]byte("$-1\r\n"))
			} else {
				_, _ = fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(val), val)
			}
		case "SETEX":
			if len(args) >= 4 {
				s.store[args[1]] = args[3]
			}
			_, _ = conn.Write([]byte("+OK\r\n"))
		case "DEL":
			delete(s.store, args[1])
			_, _ = conn.Write([]byte(":1\r\n"))
		default:
			_, _ = conn.Write([]byte("+OK\r\n"))
		}
		s.mu.Unlock()
	}
}

func extractSessionCookie(resp *http.Response) string {
	for _, sc := range resp.Header.Values("Set-Cookie") {
		if strings.HasPrefix(sc, "joss_session=") {
			parts := strings.SplitN(sc, ";", 2)
			return strings.TrimSpace(parts[0])
		}
	}
	return ""
}

func TestNative_SessionAndAuthParity(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	sessSource := `
public class SessCtrl {
	public func setVal(): string {
		string $v = strval(Request::input("v", "default"))
		Session::put("saved_note", $v)
		return "SET:" . $v
	}

	public func getVal(): string {
		return "GET:" . strval(Session::get("saved_note", ""))
	}

	public func flashRedirect(): mixed {
		return redirect("/flash-target")->with("error", "bad_creds")
	}

	public func flashTarget(): string {
		string $e = strval(Session::get("error", ""))
		return "FLASH:" . $e
	}
}

Router::get("/set", "SessCtrl@setVal")
Router::get("/get", "SessCtrl@getVal")
Router::get("/flash-go", "SessCtrl@flashRedirect")
Router::get("/flash-target", "SessCtrl@flashTarget")

public class Main {
	Init main() {
		Server::start()
	}
}
`
	sessExe := buildNativeTestServerExe(t, sessSource, "sess_parity")
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("session_memory_driver_cookie_and_flash", func(t *testing.T) {
		port := allocFreePort(t)
		cmd, logs := startBuiltServer(t, sessExe, port, []string{
			"SESSION_DRIVER=memory",
		})
		t.Cleanup(func() {
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		base := fmt.Sprintf("http://127.0.0.1:%d", port)

		r1, err := client.Get(base + "/set?v=hello_mem")
		if err != nil {
			t.Fatalf("GET /set: %v\nlogs:\n%s", err, logs.String())
		}
		_ = r1.Body.Close()
		setCookie := strings.Join(r1.Header.Values("Set-Cookie"), " | ")
		if !strings.Contains(setCookie, "joss_session=") || !strings.Contains(setCookie, "HttpOnly") || !strings.Contains(setCookie, "SameSite=Lax") {
			t.Fatalf("expected joss_session Set-Cookie attributes, got %q", setCookie)
		}
		cookiePair := extractSessionCookie(r1)

		req2, _ := http.NewRequest("GET", base+"/get", nil)
		req2.Header.Set("Cookie", cookiePair)
		r2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("GET /get: %v", err)
		}
		b2, _ := io.ReadAll(r2.Body)
		_ = r2.Body.Close()
		if string(b2) != "GET:hello_mem" {
			t.Fatalf("expected GET:hello_mem, got %q", string(b2))
		}

		// Flash redirect -> read once -> gone on next request
		reqF, _ := http.NewRequest("GET", base+"/flash-go", nil)
		reqF.Header.Set("Cookie", cookiePair)
		rf, err := client.Do(reqF)
		if err != nil {
			t.Fatalf("GET /flash-go: %v", err)
		}
		_ = rf.Body.Close()
		if rf.StatusCode != 302 {
			t.Fatalf("expected 302, got %d", rf.StatusCode)
		}

		reqFT1, _ := http.NewRequest("GET", base+"/flash-target", nil)
		reqFT1.Header.Set("Cookie", cookiePair)
		rft1, err := client.Do(reqFT1)
		if err != nil {
			t.Fatalf("GET /flash-target 1: %v", err)
		}
		bft1, _ := io.ReadAll(rft1.Body)
		_ = rft1.Body.Close()
		if string(bft1) != "FLASH:bad_creds" {
			t.Fatalf("expected FLASH:bad_creds on first read, got %q", string(bft1))
		}

		reqFT2, _ := http.NewRequest("GET", base+"/flash-target", nil)
		reqFT2.Header.Set("Cookie", cookiePair)
		rft2, err := client.Do(reqFT2)
		if err != nil {
			t.Fatalf("GET /flash-target 2: %v", err)
		}
		bft2, _ := io.ReadAll(rft2.Body)
		_ = rft2.Body.Close()
		if string(bft2) != "FLASH:" {
			t.Fatalf("expected flash cleared on second read, got %q", string(bft2))
		}
	})

	t.Run("session_file_driver_persists_across_restart", func(t *testing.T) {
		sessFile := filepath.Join(t.TempDir(), "sub", "sessions.json")
		port1 := allocFreePort(t)
		cmd1, logs1 := startBuiltServer(t, sessExe, port1, []string{
			"SESSION_DRIVER=file",
			"SESSION_FILE=" + sessFile,
		})
		base1 := fmt.Sprintf("http://127.0.0.1:%d", port1)
		r1, err := client.Get(base1 + "/set?v=persisted_42")
		if err != nil {
			_ = cmd1.Process.Kill()
			_ = cmd1.Wait()
			t.Fatalf("GET /set: %v\nlogs:\n%s", err, logs1.String())
		}
		_ = r1.Body.Close()
		cookiePair := extractSessionCookie(r1)
		_ = cmd1.Process.Kill()
		_ = cmd1.Wait()

		if _, err := os.Stat(sessFile); err != nil {
			t.Fatalf("expected session file created at %s: %v", sessFile, err)
		}

		port2 := allocFreePort(t)
		cmd2, logs2 := startBuiltServer(t, sessExe, port2, []string{
			"SESSION_DRIVER=file",
			"SESSION_FILE=" + sessFile,
		})
		t.Cleanup(func() {
			if cmd2 != nil && cmd2.Process != nil {
				_ = cmd2.Process.Kill()
				_ = cmd2.Wait()
			}
		})
		base2 := fmt.Sprintf("http://127.0.0.1:%d", port2)
		req2, _ := http.NewRequest("GET", base2+"/get", nil)
		req2.Header.Set("Cookie", cookiePair)
		r2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("GET /get after restart: %v\nlogs:\n%s", err, logs2.String())
		}
		b2, _ := io.ReadAll(r2.Body)
		_ = r2.Body.Close()
		if string(b2) != "GET:persisted_42" {
			t.Fatalf("expected GET:persisted_42 across restart, got %q", string(b2))
		}
	})

	t.Run("session_redis_driver_mock_tcp_resp", func(t *testing.T) {
		mockRedis := startMockRESPServer(t, "secretpass")
		port := allocFreePort(t)
		cmd, logs := startBuiltServer(t, sessExe, port, []string{
			"SESSION_DRIVER=redis",
			"REDIS_HOST=127.0.0.1",
			fmt.Sprintf("REDIS_PORT=%d", mockRedis.Port()),
			"REDIS_PASSWORD=secretpass",
			"REDIS_DB=2",
		})
		t.Cleanup(func() {
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		r1, err := client.Get(base + "/set?v=redis_ok")
		if err != nil {
			t.Fatalf("GET /set redis: %v\nlogs:\n%s", err, logs.String())
		}
		_ = r1.Body.Close()
		if r1.StatusCode != 200 {
			t.Fatalf("expected 200 on redis /set, got %d", r1.StatusCode)
		}
		cookiePair := extractSessionCookie(r1)

		req2, _ := http.NewRequest("GET", base+"/get", nil)
		req2.Header.Set("Cookie", cookiePair)
		r2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("GET /get redis: %v", err)
		}
		b2, _ := io.ReadAll(r2.Body)
		_ = r2.Body.Close()
		if string(b2) != "GET:redis_ok" {
			t.Fatalf("expected GET:redis_ok, got %q", string(b2))
		}

		mockRedis.mu.Lock()
		allCmds := strings.Join(mockRedis.cmds, "\n")
		mockRedis.mu.Unlock()
		if !strings.Contains(allCmds, "AUTH secretpass") || !strings.Contains(allCmds, "SELECT 2") || !strings.Contains(allCmds, "SETEX session:") {
			t.Fatalf("expected AUTH/SELECT/SETEX in mock redis commands, got:\n%s", allCmds)
		}

		// Unreachable redis port must fail explicitly with HTTP 500
		deadPort := allocFreePort(t)
		portBad := allocFreePort(t)
		cmdBad, _ := startBuiltServer(t, sessExe, portBad, []string{
			"SESSION_DRIVER=redis",
			"REDIS_HOST=127.0.0.1",
			fmt.Sprintf("REDIS_PORT=%d", deadPort),
		})
		t.Cleanup(func() {
			if cmdBad != nil && cmdBad.Process != nil {
				_ = cmdBad.Process.Kill()
				_ = cmdBad.Wait()
			}
		})
		rb, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/get", portBad))
		if err != nil {
			t.Fatalf("GET unreachable redis: %v", err)
		}
		_ = rb.Body.Close()
		if rb.StatusCode != 500 {
			t.Fatalf("unreachable redis expected 500, got %d", rb.StatusCode)
		}
	})

	t.Run("session_invalid_driver_fails_explicitly", func(t *testing.T) {
		port := allocFreePort(t)
		cmd, _ := startBuiltServer(t, sessExe, port, []string{
			"SESSION_DRIVER=bogus",
		})
		t.Cleanup(func() {
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		r, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/get", port))
		if err != nil {
			t.Fatalf("GET invalid driver: %v", err)
		}
		_ = r.Body.Close()
		if r.StatusCode != 500 {
			t.Fatalf("invalid SESSION_DRIVER expected 500, got %d", r.StatusCode)
		}
	})

	t.Run("auth_fluent_closures_bcrypt_totp_and_oauth_url", func(t *testing.T) {
		bcryptBytes, err := bcrypt.GenerateFromPassword([]byte("Password123!"), 10)
		if err != nil {
			t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
		}
		authSource := fmt.Sprintf(`
public class Main {
	Init main() {
		string $capturedEmail = "ada@joss.dev"
		string $pbkdf = Auth::hash("Password123!")
		print("PBKDF2_HASH:" . strval(Auth::verifyHash($pbkdf, "Password123!")))

		string $bcryptHash = "%s"
		print("BCRYPT:" . strval(Auth::verifyHash($bcryptHash, "Password123!") && !Auth::verifyHash($bcryptHash, "wrong")))

		mixed $totpInfo = MFA::generateTOTP()
		string $sec = strval($totpInfo["secret"])
		string $code = strval(MFA::generateTOTP($sec))
		print("TOTP_LEN:" . strval(strlen($sec) == 16 && strlen($code) == 6 && MFA::verifyTOTP($sec, $code)))

		// Fluent AuthLoginResult + closure capture
		var $res = new AuthLoginResult()
		$res->success = false
		$res->error = "invalid"
		mixed $failOut = $res->onSuccess(func(mixed $jwt) {
			return "NO"
		})->onFail(func(mixed $err) {
			return "FAIL_FOR:" . $capturedEmail . ":" . strval($err)
		})->response()
		print("FLUENT_FAIL:" . strval($failOut))

		var $okRes = new AuthLoginResult()
		$okRes->success = true
		$okRes->requires_2fa = false
		$okRes->jwt = "tok_123"
		mixed $okOut = $okRes->onSuccess(func(mixed $jwt) {
			return "OK:" . $capturedEmail . ":" . strval($jwt)
		})->response()
		print("FLUENT_OK:" . strval($okOut))

		string $redir = Auth::socialRedirect("github", "http://127.0.0.1/cb", "st1")
		print("OAUTH_URL:" . strval(str_contains($redir, "github.com/login/oauth/authorize") && str_contains($redir, "client_id=gh_cid")))
	}
}
`, string(bcryptBytes))
		authExe := buildNativeTestServerExe(t, authSource, "auth_unit_parity")
		cmd := exec.Command(authExe)
		cmd.Env = append(os.Environ(), "GITHUB_CLIENT_ID=gh_cid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("auth_unit_parity failed: %v\noutput:\n%s", err, string(out))
		}
		got := string(out)
		for _, want := range []string{
			"PBKDF2_HASH:true",
			"BCRYPT:true",
			"TOTP_LEN:true",
			"FLUENT_FAIL:FAIL_FOR:ada@joss.dev:invalid",
			"FLUENT_OK:OK:ada@joss.dev:tok_123",
			"OAUTH_URL:true",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %q in output:\n%s", want, got)
			}
		}
	})

	t.Run("auth_fluent_closures_2fa_and_oauth_sqlite", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SQLite3 C development library not preinstalled on Windows CI runner")
		}
		mockOAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/token" {
				_, _ = io.WriteString(w, `{"access_token":"gh_mock_tok_123"}`)
				return
			}
			if r.URL.Path == "/user" {
				_, _ = io.WriteString(w, `{"id":"998877","login":"ada_gh","email":"ada_oauth@joss.dev","name":"Ada OAuth","avatar_url":"https://example.com/a.png"}`)
				return
			}
			w.WriteHeader(404)
		}))
		defer mockOAuth.Close()

		authSource := `
public class Main {
	Init main() {
		__joss_db_execute("CREATE TABLE IF NOT EXISTS js_roles (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)", [])
		__joss_db_execute("CREATE TABLE IF NOT EXISTS js_users (id INTEGER PRIMARY KEY AUTOINCREMENT, user_token TEXT, username TEXT, first_name TEXT, last_name TEXT, email TEXT, phone TEXT, password TEXT, role_id INTEGER, token_expires_at TEXT, verificado INTEGER, last_login_at TEXT)", [])
		__joss_db_execute("CREATE TABLE IF NOT EXISTS js_user_mfa_methods (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, method_type TEXT, secret TEXT, is_active INTEGER)", [])
		__joss_db_execute("CREATE TABLE IF NOT EXISTS js_user_mfa_challenges (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, code_hash TEXT, used INTEGER)", [])
		__joss_db_execute("CREATE TABLE IF NOT EXISTS js_user_recovery_codes (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, code_hash TEXT, used INTEGER, used_at TEXT)", [])
		__joss_db_execute("CREATE TABLE IF NOT EXISTS js_user_social_accounts (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, provider TEXT, provider_user_id TEXT, avatar TEXT)", [])
		__joss_db_execute("INSERT INTO js_roles (id, name) VALUES (2, 'user')", [])

		// 1. Create user + unverified -> verified
		string $capturedEmail = "ada@joss.dev"
		mixed $tok = Auth::create({"email": $capturedEmail, "password": "Password123!", "username": "ada"})
		print("CREATE:" . strval($tok != false))
		print("STAT1:" . Auth::verificationStatus($capturedEmail))
		bool $verOk = Auth::verify(strval($tok))
		print("VER:" . strval($verOk) . "|STAT2:" . Auth::verificationStatus($capturedEmail))

		// 2. Fluent login with closure capture (fail then success)
		var $badRes = Auth::login($capturedEmail, "wrongpass")
		mixed $badOut = $badRes->onSuccess(func(mixed $jwt) {
			return "SHOULD_NOT_SUCCEED"
		})->onFail(func(mixed $err) {
			return "FAIL_FOR:" . $capturedEmail
		})->response()
		print("BAD:" . strval($badOut))

		var $okRes = Auth::login($capturedEmail, "Password123!")
		mixed $okOut = $okRes->onSuccess(func(mixed $jwt) {
			return "OK:" . $capturedEmail . ":" . strval(strlen(strval($jwt)) > 20)
		})->onFail(func(mixed $err) {
			return "SHOULD_NOT_FAIL"
		})->response()
		print("PBKDF2:" . strval($okOut))

		// 3. Bcrypt $2b$ hash compatibility
		string $bcryptHash = "$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		print("BCRYPT:" . strval(Auth::verifyHash($bcryptHash, "password")))

		// 4. 2FA TOTP flow + single-use challenge replay guard
		mixed $totpInfo = MFA::generateTOTP()
		string $sec = strval($totpInfo["secret"])
		string $code = strval(MFA::generateTOTP($sec))
		mixed $uRow = GranDB::table("users")->select("id")->where("email", $capturedEmail)->first()
		GranDB::table("user_mfa_methods")->insert({"user_id": $uRow["id"], "method_type": "totp", "secret": $sec, "is_active": 1})
		var $chRes = Auth::login($capturedEmail, "Password123!")
		string $chTok = strval($chRes->onChallenge(func(mixed $t) {
			return strval($t)
		})->response())
		mixed $finalJwt = Auth::verify2FAChallenge($chTok, $code)
		mixed $replayJwt = Auth::verify2FAChallenge($chTok, $code)
		print("2FA:" . strval($finalJwt != false && strlen(strval($finalJwt)) > 20) . "|REPLAY:" . strval($replayJwt == false))

		// 5. OAuth socialRedirect + socialCallback
		string $redir = Auth::socialRedirect("github", "http://127.0.0.1/cb", "st1")
		print("OAUTH_URL:" . strval(str_contains($redir, "github.com/login/oauth/authorize") && str_contains($redir, "client_id=gh_cid")))
		var $socRes = Auth::socialCallback("github", "mock_code", "http://127.0.0.1/cb")
		mixed $socJwt = $socRes->onSuccess(func(mixed $jwt) {
			return strval($jwt)
		})->response()
		print("OAUTH_CB:" . strval($socJwt != null && strlen(strval($socJwt)) > 20))
	}
}
`
		authExe := buildNativeTestServerExe(t, authSource, "auth_parity")
		dbPath := filepath.Join(t.TempDir(), "auth.db")
		cmd := exec.Command(authExe)
		cmd.Env = append(os.Environ(),
			"DB=sqlite",
			"DB_DATABASE="+dbPath,
			"DB_PATH="+dbPath,
			"PREFIX=js_",
			"JWT_SECRET=espiral3-super-secret-jwt-key-123456",
			"GITHUB_CLIENT_ID=gh_cid",
			"GITHUB_CLIENT_SECRET=gh_csec",
			"OAUTH_GITHUB_TOKEN_URL="+mockOAuth.URL+"/token",
			"OAUTH_GITHUB_USERINFO_URL="+mockOAuth.URL+"/user",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("auth_parity failed: %v\noutput:\n%s", err, string(out))
		}
		got := string(out)
		for _, want := range []string{
			"CREATE:true",
			"STAT1:unverified",
			"VER:true|STAT2:verified",
			"BAD:FAIL_FOR:ada@joss.dev",
			"PBKDF2:OK:ada@joss.dev:true",
			"BCRYPT:true",
			"2FA:true|REPLAY:true",
			"OAUTH_URL:true",
			"OAUTH_CB:true",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %q in output:\n%s", want, got)
			}
		}
	})
}

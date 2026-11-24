package native_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
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
)

func allocFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func startBuiltServer(t *testing.T, exePath string, port int, extraEnv []string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(exePath)
	cmd.Dir = filepath.Dir(exePath)
	env := append(cmd.Environ(), fmt.Sprintf("PORT=%d", port))
	env = append(env, extraEnv...)
	cmd.Env = env
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return cmd, &logs
		}
		time.Sleep(30 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	t.Fatalf("server failed to bind port %d\nlogs:\n%s", port, logs.String())
	return nil, nil
}

func makeTestHS256JWT(secret string, payloadJSON string) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pl := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	msg := hdr + "." + pl
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return msg + "." + sig
}

func TestNative_HTTPConcurrentMultipartAndMiddlewareParity(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	source := `
public class AppCtrl {
	public func echoId(string $id): string {
		string $q = strval(Request::input("q", ""))
		return "ECHO:" . $id . ":" . $q
	}

	public func upload(): string {
		string $cap = strval(Request::input("caption", ""))
		bool $has = Request::hasFile("avatar")
		mixed $f = Request::file("avatar")
		int $sz = 0
		string $sha = ""
		string $fname = ""
		($f != null) ? {
			$sz = intval($f["size"])
			$fname = strval($f["name"])
			mixed $bytes = file_get_contents(strval($f["path"]))
			$sha = sha256_base64($bytes)
		}
		return "UP:has=" . strval($has) . "|name=" . $fname . "|cap=" . $cap . "|size=" . strval($sz) . "|sha=" . $sha
	}

	public func setUser(): mixed {
		Session::put("user_id", 7)
		Session::put("user_role", "user")
		return Response::raw("USER_SET", 200, "text/plain; charset=utf-8")
	}

	public func setAdmin(): mixed {
		Session::put("user_id", 1)
		Session::put("user_role", "admin")
		return Response::raw("ADMIN_SET", 200, "text/plain; charset=utf-8")
	}

	public func privatePage(): string {
		return "PRIVATE_OK:" . strval(Session::get("user_id", 0))
	}

	public func guestPage(): string {
		return "GUEST_OK"
	}

	public func adminPage(): string {
		return "ADMIN_OK"
	}

	public func apiMe(): mixed {
		return json({"user_id": Session::get("user_id", 0), "role": strval(Session::get("user_role", ""))})
	}

	public func limited(): string {
		return "LIMITED_OK"
	}
}

Router::get("/echo/{id}", "AppCtrl@echoId")
Router::post("/upload", "AppCtrl@upload")
Router::get("/test-login", "AppCtrl@setUser")
Router::get("/test-admin", "AppCtrl@setAdmin")

Router::middleware("auth")
Router::get("/private", "AppCtrl@privatePage")
Router::middleware("admin")
Router::get("/admin-only", "AppCtrl@adminPage")
Router::end()
Router::end()

Router::middleware("guest")
Router::get("/guest-only", "AppCtrl@guestPage")
Router::end()

Router::middleware("auth_api")
Router::get("/api/me", "AppCtrl@apiMe")
Router::end()

Router::group("throttle:2,10", func() {
	Router::get("/limited", "AppCtrl@limited")
})

Router::get("/ads.txt", func() {
	return Response::raw("ads-closure-ok", 200, "text/plain; charset=utf-8")
})

Router::get("/hi/{name}", func(string $name) {
	return "hi:" . $name
})

public class Main {
	Init main() {
		Server::start()
	}
}
`

	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := append([]analyzer.SourceUnit{{Path: "http_conc_mw.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}

	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: "http_conc_mw.joss"})
	irProg, err := ir.LowerAndVerify(prep, "http_conc_mw", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}

	outExe := filepath.Join(t.TempDir(), "http_conc_mw")
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

	const jwtSecret = "super-secret-espiral2-key-123456"
	port1 := allocFreePort(t)
	cmd1, logs1 := startBuiltServer(t, outExe, port1, []string{
		"RATE_LIMIT_REQUESTS=1000",
		"RATE_LIMIT_WINDOW_SECONDS=60",
		"JWT_SECRET=" + jwtSecret,
	})
	t.Cleanup(func() {
		if cmd1 != nil && cmd1.Process != nil {
			_ = cmd1.Process.Kill()
			_ = cmd1.Wait()
		}
	})
	base1 := fmt.Sprintf("http://127.0.0.1:%d", port1)
	clientNoRedirect := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("concurrent_16_requests", func(t *testing.T) {
		const n = 16
		var wg sync.WaitGroup
		errs := make(chan error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				url := fmt.Sprintf("%s/echo/%d?q=val_%d", base1, id, id*7)
				resp, err := clientNoRedirect.Get(url)
				if err != nil {
					errs <- fmt.Errorf("id=%d get: %w", id, err)
					return
				}
				bodyBytes, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != 200 {
					errs <- fmt.Errorf("id=%d status=%d body=%s", id, resp.StatusCode, string(bodyBytes))
					return
				}
				want := fmt.Sprintf("ECHO:%d:val_%d", id, id*7)
				if strings.TrimSpace(string(bodyBytes)) != want {
					errs <- fmt.Errorf("id=%d got %q, want %q", id, string(bodyBytes), want)
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			t.Fatal(e)
		}
	})

	t.Run("binary_multipart_upload_with_nul_bytes", func(t *testing.T) {
		pngBytes := []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
			0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
			0x00, 0x01, 0x02, 0x00, 0x03, 0x04, 0xFF, 0xFE, 0x00, 0xAA, 0x55,
		}
		sum := sha256.Sum256(pngBytes)
		wantSHA := base64.StdEncoding.EncodeToString(sum[:])

		var bodyBuf bytes.Buffer
		mw := multipart.NewWriter(&bodyBuf)
		_ = mw.WriteField("caption", "mi imagen\nsegura")
		fw, err := mw.CreateFormFile("avatar", "test_bin.png")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		_, _ = fw.Write(pngBytes)
		_ = mw.Close()

		req, _ := http.NewRequest("POST", base1+"/upload", &bodyBuf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := clientNoRedirect.Do(req)
		if err != nil {
			t.Fatalf("upload post: %v", err)
		}
		respBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		got := string(respBytes)
		want := fmt.Sprintf("UP:has=true|name=test_bin.png|cap=mi imagen\nsegura|size=%d|sha=%s", len(pngBytes), wantSHA)
		if got != want {
			t.Fatalf("multipart mismatch:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("middlewares_auth_guest_admin_auth_api_throttle", func(t *testing.T) {
		// Unauthenticated -> /private 302 /login, /guest-only 200
		r1, err := clientNoRedirect.Get(base1 + "/private")
		if err != nil {
			t.Fatalf("get /private: %v", err)
		}
		_ = r1.Body.Close()
		if r1.StatusCode != 302 || r1.Header.Get("Location") != "/login" {
			t.Fatalf("/private unauth expected 302 /login, got %d %q", r1.StatusCode, r1.Header.Get("Location"))
		}

		r2, err := clientNoRedirect.Get(base1 + "/guest-only")
		if err != nil {
			t.Fatalf("get /guest-only: %v", err)
		}
		_ = r2.Body.Close()
		if r2.StatusCode != 200 {
			t.Fatalf("/guest-only unauth expected 200, got %d", r2.StatusCode)
		}

		// User session via Cookie joss_session=sid-user-1
		reqLogin, _ := http.NewRequest("GET", base1+"/test-login", nil)
		reqLogin.Header.Set("Cookie", "joss_session=sid-user-1")
		rl, err := clientNoRedirect.Do(reqLogin)
		if err != nil {
			t.Fatalf("get /test-login: %v", err)
		}
		_ = rl.Body.Close()
		if rl.StatusCode != 200 {
			t.Fatalf("/test-login expected 200, got %d", rl.StatusCode)
		}

		reqPriv, _ := http.NewRequest("GET", base1+"/private", nil)
		reqPriv.Header.Set("Cookie", "joss_session=sid-user-1")
		rp, err := clientNoRedirect.Do(reqPriv)
		if err != nil {
			t.Fatalf("get /private authed: %v\nlogs:\n%s", err, logs1.String())
		}
		pb, _ := io.ReadAll(rp.Body)
		_ = rp.Body.Close()
		if rp.StatusCode != 200 || strings.TrimSpace(string(pb)) != "PRIVATE_OK:7" {
			t.Fatalf("/private authed expected 200 PRIVATE_OK:7, got %d %q", rp.StatusCode, string(pb))
		}

		reqGuestAuthed, _ := http.NewRequest("GET", base1+"/guest-only", nil)
		reqGuestAuthed.Header.Set("Cookie", "joss_session=sid-user-1")
		rg, err := clientNoRedirect.Do(reqGuestAuthed)
		if err != nil {
			t.Fatalf("get /guest-only authed: %v\nlogs:\n%s", err, logs1.String())
		}
		_ = rg.Body.Close()
		if rg.StatusCode != 302 || rg.Header.Get("Location") != "/dashboard" {
			t.Fatalf("/guest-only authed expected 302 /dashboard, got %d %q", rg.StatusCode, rg.Header.Get("Location"))
		}

		reqAdminDeny, _ := http.NewRequest("GET", base1+"/admin-only", nil)
		reqAdminDeny.Header.Set("Cookie", "joss_session=sid-user-1")
		rad, err := clientNoRedirect.Do(reqAdminDeny)
		if err != nil {
			t.Fatalf("get /admin-only non-admin: %v\nlogs:\n%s", err, logs1.String())
		}
		_ = rad.Body.Close()
		if rad.StatusCode != 302 || rad.Header.Get("Location") != "/" {
			t.Fatalf("/admin-only non-admin expected 302 /, got %d %q", rad.StatusCode, rad.Header.Get("Location"))
		}

		// Admin session
		reqSetAdmin, _ := http.NewRequest("GET", base1+"/test-admin", nil)
		reqSetAdmin.Header.Set("Cookie", "joss_session=sid-admin-1")
		rsa, err := clientNoRedirect.Do(reqSetAdmin)
		if err != nil {
			t.Fatalf("get /test-admin: %v\nlogs:\n%s", err, logs1.String())
		}
		_ = rsa.Body.Close()

		reqAdminOK, _ := http.NewRequest("GET", base1+"/admin-only", nil)
		reqAdminOK.Header.Set("Cookie", "joss_session=sid-admin-1")
		rao, err := clientNoRedirect.Do(reqAdminOK)
		if err != nil {
			t.Fatalf("get /admin-only admin: %v\nlogs:\n%s", err, logs1.String())
		}
		aob, _ := io.ReadAll(rao.Body)
		_ = rao.Body.Close()
		if rao.StatusCode != 200 || strings.TrimSpace(string(aob)) != "ADMIN_OK" {
			t.Fatalf("/admin-only admin expected 200 ADMIN_OK, got %d %q", rao.StatusCode, string(aob))
		}

		// auth_api: missing, invalid, valid HS256 JWT
		rMissing, err := clientNoRedirect.Get(base1 + "/api/me")
		if err != nil {
			t.Fatalf("get /api/me missing: %v\nlogs:\n%s", err, logs1.String())
		}
		rmb, _ := io.ReadAll(rMissing.Body)
		_ = rMissing.Body.Close()
		if rMissing.StatusCode != 401 || !strings.Contains(string(rmb), "Missing Token") {
			t.Fatalf("/api/me missing expected 401 Missing Token, got %d %s", rMissing.StatusCode, string(rmb))
		}

		reqBadTok, _ := http.NewRequest("GET", base1+"/api/me", nil)
		reqBadTok.Header.Set("Authorization", "Bearer invalid.jwt.token")
		rBad, err := clientNoRedirect.Do(reqBadTok)
		if err != nil {
			t.Fatalf("get /api/me invalid: %v\nlogs:\n%s", err, logs1.String())
		}
		rbb, _ := io.ReadAll(rBad.Body)
		_ = rBad.Body.Close()
		if rBad.StatusCode != 401 || !strings.Contains(string(rbb), "Invalid Token") {
			t.Fatalf("/api/me invalid expected 401 Invalid Token, got %d %s", rBad.StatusCode, string(rbb))
		}

		validTok := makeTestHS256JWT(jwtSecret, fmt.Sprintf(`{"user_id":42,"email":"ada@joss.dev","name":"Ada","role":"admin","exp":%d}`, time.Now().Add(time.Hour).Unix()))
		reqGoodTok, _ := http.NewRequest("GET", base1+"/api/me", nil)
		reqGoodTok.Header.Set("Authorization", "Bearer "+validTok)
		rGood, err := clientNoRedirect.Do(reqGoodTok)
		if err != nil {
			t.Fatalf("get /api/me valid: %v\nlogs:\n%s", err, logs1.String())
		}
		rgb, _ := io.ReadAll(rGood.Body)
		_ = rGood.Body.Close()
		if rGood.StatusCode != 200 || !strings.Contains(string(rgb), `"user_id":42`) || !strings.Contains(string(rgb), `"role":"admin"`) {
			t.Fatalf("/api/me valid expected 200 with claims, got %d %s", rGood.StatusCode, string(rgb))
		}

		// throttle:2,10 route middleware
		for i := 1; i <= 2; i++ {
			reqT, _ := http.NewRequest("GET", base1+"/limited", nil)
			reqT.Header.Set("X-Forwarded-For", "198.51.100.77")
			rt, err := clientNoRedirect.Do(reqT)
			if err != nil {
				t.Fatalf("throttle request %d: %v\nlogs:\n%s", i, err, logs1.String())
			}
			_ = rt.Body.Close()
			if rt.StatusCode != 200 {
				t.Fatalf("throttle request %d expected 200, got %d", i, rt.StatusCode)
			}
		}
		reqT3, _ := http.NewRequest("GET", base1+"/limited", nil)
		reqT3.Header.Set("X-Forwarded-For", "198.51.100.77")
		rt3, err := clientNoRedirect.Do(reqT3)
		if err != nil {
			t.Fatalf("throttle request 3: %v\nlogs:\n%s", err, logs1.String())
		}
		_ = rt3.Body.Close()
		if rt3.StatusCode != 429 || rt3.Header.Get("Retry-After") != "10" {
			t.Fatalf("throttle request 3 expected 429 Retry-After:10, got %d Retry-After=%q", rt3.StatusCode, rt3.Header.Get("Retry-After"))
		}
	})

	t.Run("closure_routes", func(t *testing.T) {
		rAds, err := clientNoRedirect.Get(base1 + "/ads.txt")
		if err != nil {
			t.Fatalf("get /ads.txt: %v", err)
		}
		ab, _ := io.ReadAll(rAds.Body)
		_ = rAds.Body.Close()
		if rAds.StatusCode != 200 || string(ab) != "ads-closure-ok" {
			t.Fatalf("/ads.txt got %d %q, want 200 ads-closure-ok", rAds.StatusCode, string(ab))
		}

		rHi, err := clientNoRedirect.Get(base1 + "/hi/Joss")
		if err != nil {
			t.Fatalf("get /hi/Joss: %v", err)
		}
		hb, _ := io.ReadAll(rHi.Body)
		_ = rHi.Body.Close()
		if rHi.StatusCode != 200 || string(hb) != "hi:Joss" {
			t.Fatalf("/hi/Joss got %d %q, want 200 hi:Joss", rHi.StatusCode, string(hb))
		}
	})

	t.Run("global_rate_limit_429", func(t *testing.T) {
		port2 := allocFreePort(t)
		cmd2, logs2 := startBuiltServer(t, outExe, port2, []string{
			"RATE_LIMIT_REQUESTS=3",
			"RATE_LIMIT_WINDOW_SECONDS=10",
		})
		t.Cleanup(func() {
			if cmd2 != nil && cmd2.Process != nil {
				_ = cmd2.Process.Kill()
				_ = cmd2.Wait()
			}
		})
		base2 := fmt.Sprintf("http://127.0.0.1:%d", port2)

		for i := 1; i <= 3; i++ {
			req, _ := http.NewRequest("GET", base2+"/ads.txt", nil)
			req.Header.Set("X-Forwarded-For", "203.0.113.9")
			resp, err := clientNoRedirect.Do(req)
			if err != nil {
				t.Fatalf("rl req %d: %v\nlogs:\n%s", i, err, logs2.String())
			}
			_ = resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("rl req %d expected 200, got %d", i, resp.StatusCode)
			}
		}
		req4, _ := http.NewRequest("GET", base2+"/ads.txt", nil)
		req4.Header.Set("X-Forwarded-For", "203.0.113.9")
		r4, err := clientNoRedirect.Do(req4)
		if err != nil {
			t.Fatalf("rl req 4: %v\nlogs:\n%s", err, logs2.String())
		}
		_ = r4.Body.Close()
		if r4.StatusCode != 429 || r4.Header.Get("Retry-After") != "10" {
			t.Fatalf("rl req 4 expected 429 Retry-After:10, got %d Retry-After=%q", r4.StatusCode, r4.Header.Get("Retry-After"))
		}

		// Different IP is unaffected
		reqOther, _ := http.NewRequest("GET", base2+"/ads.txt", nil)
		reqOther.Header.Set("X-Forwarded-For", "203.0.113.10")
		ro, err := clientNoRedirect.Do(reqOther)
		if err != nil {
			t.Fatalf("rl different IP: %v\nlogs:\n%s", err, logs2.String())
		}
		_ = ro.Body.Close()
		if ro.StatusCode != 200 {
			t.Fatalf("different IP expected 200, got %d", ro.StatusCode)
		}
	})
}

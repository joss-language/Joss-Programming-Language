package native_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestNative_RSASignAndHttpRequestMap(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	source := `
		string $date = gmdate("D, d M Y H:i:s");
		echo "DATE:" . $date;
		var $res = Http::request("GET", "http://127.0.0.1:1/nonexistent", {
			"headers": {"X-Test": "1"},
			"timeout": 1
		});
		echo "STATUS:" . strval($res["status"]);
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "oci_probe.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: "oci_probe.joss"})
	irProg, err := ir.LowerAndVerify(prep, "oci_probe", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}

	outExe := filepath.Join(t.TempDir(), "oci_probe")
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

	out, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("oci_probe failed: %v\nOutput:\n%s", err, string(out))
	}
	outStr := string(out)
	if !strings.Contains(outStr, "DATE:") || !strings.Contains(outStr, "STATUS:0") {
		t.Fatalf("unexpected output:\n%s", outStr)
	}
}

func TestNative_RSASignPKCS8(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	source := `
		string $pem = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7VJTUt9Us8cKj\nMzEfYyjiWA4R4/M2bS1GB4t7NXp98C369X660nE0Y9G1527A5P15+K5W3r4L+3W5\n-----END PRIVATE KEY-----";
		string $sig = rsa_sign_sha256("date: Sun, 22 Nov 2026 04:00:00 GMT", $pem);
		echo "SIGLEN:" . strval(strlen($sig));
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "rsa_probe.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: "rsa_probe.joss"})
	irProg, err := ir.LowerAndVerify(prep, "rsa_probe", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}

	outExe := filepath.Join(t.TempDir(), "rsa_probe")
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

	out, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("rsa_probe failed: %v\nOutput:\n%s", err, string(out))
	}
	t.Logf("rsa_probe output: %s", string(out))
}

func TestDifferential_CryptoAndHttpPrimitives(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	bcryptBytes, err := bcrypt.GenerateFromPassword([]byte("Sup3rS3cret!"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword failed: %v", err)
	}
	bcryptHash := string(bcryptBytes)

	var gotFormSecret, gotFormResp, gotFormCT string
	var gotPutBody []byte
	var gotPutSHA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/form":
			gotFormCT = r.Header.Get("Content-Type")
			_ = r.ParseForm()
			gotFormSecret = r.Form.Get("secret")
			gotFormResp = r.Form.Get("response")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"host":"local"}`))
		case "/put":
			gotPutSHA = r.Header.Get("x-content-sha256")
			gotPutBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`ok`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	source := fmt.Sprintf(`
		echo "MD5_EMPTY:" . md5("");
		echo "MD5_HELLO:" . md5("hello world");
		echo "SHA_EMPTY:" . sha256_base64("");
		echo "SHA_HELLO:" . sha256_base64("hello world");
		echo "GMDATE:" . gmdate("D, d M Y H:i:s", 1700000000);

		string $bhash = %q;
		echo "BCRYPT_OK:" . strval(password_verify($bhash, "Sup3rS3cret!"));
		echo "BCRYPT_BAD:" . strval(password_verify($bhash, "WrongPass"));

		string $phash = password_hash("MyP4ss!");
		echo "PBKDF2_OK:" . strval(password_verify($phash, "MyP4ss!"));
		echo "PBKDF2_BAD:" . strval(password_verify($phash, "Nope"));

		var $formRes = Http::request("POST", %q, {
			"timeout": 5,
			"form": {"secret": "sec=1&2", "response": "tok+a/b=c"},
			"headers": {"Accept": "application/json"}
		});
		echo "FORM_STATUS:" . strval($formRes["status"] ?? 0);
		echo "FORM_SUCCESS:" . strval($formRes["success"] ?? false);
		var $fjson = JSON::parse(strval($formRes["body"] ?? ""));
		echo "FORM_CF:" . strval($fjson["success"] ?? false);

		string $payload = "oci-payload-123";
		string $sum = sha256_base64($payload);
		var $putRes = Http::request("PUT", %q, {
			"timeout": 5,
			"body": $payload,
			"headers": {
				"Content-Type": "application/octet-stream",
				"x-content-sha256": $sum
			}
		});
		echo "PUT_STATUS:" . strval($putRes["status"] ?? 0);
	`, bcryptHash, srv.URL+"/form", srv.URL+"/put")

	goOut, ok, errInfo := runInterpreterWithUnits(t, source, std.GetStdlibUnits())
	if !ok {
		t.Fatalf("interpreter failed: %s", errInfo)
	}
	if gotFormSecret != "sec=1&2" || gotFormResp != "tok+a/b=c" || !strings.Contains(gotFormCT, "application/x-www-form-urlencoded") {
		t.Fatalf("interpreter form mismatch: ct=%q secret=%q resp=%q", gotFormCT, gotFormSecret, gotFormResp)
	}
	wantSHA := sha256.Sum256([]byte("oci-payload-123"))
	wantSHAB64 := base64.StdEncoding.EncodeToString(wantSHA[:])
	if string(gotPutBody) != "oci-payload-123" || gotPutSHA != wantSHAB64 {
		t.Fatalf("interpreter PUT mismatch: body=%q sha=%q want=%q", string(gotPutBody), gotPutSHA, wantSHAB64)
	}

	// Reset captured server state before running native executable
	gotFormSecret, gotFormResp, gotFormCT = "", "", ""
	gotPutBody, gotPutSHA = nil, ""

	// Compile explicitly via LLVM backend (with stdlib) since this exercises C11 web/crypto primitives
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "diff_s1.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: "diff_s1.joss"})
	irProg, err := ir.LowerAndVerify(prep, "diff_s1", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}
	outExe := filepath.Join(t.TempDir(), "diff_s1")
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
	nativeBytes, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("native binary failed: %v\nOutput:\n%s", err, string(nativeBytes))
	}
	nativeOut := strings.TrimSpace(strings.ReplaceAll(string(nativeBytes), "\r\n", "\n"))
	normGo := strings.TrimSpace(strings.ReplaceAll(goOut, "\r\n", "\n"))
	if nativeOut != normGo {
		t.Fatalf("differential mismatch Go vs Native:\n--- Go ---\n%s\n--- Native ---\n%s", normGo, nativeOut)
	}
	if gotFormSecret != "sec=1&2" || gotFormResp != "tok+a/b=c" || !strings.Contains(gotFormCT, "application/x-www-form-urlencoded") {
		t.Fatalf("native form mismatch: ct=%q secret=%q resp=%q", gotFormCT, gotFormSecret, gotFormResp)
	}
	if string(gotPutBody) != "oci-payload-123" || gotPutSHA != wantSHAB64 {
		t.Fatalf("native PUT mismatch: body=%q sha=%q want=%q", string(gotPutBody), gotPutSHA, wantSHAB64)
	}
	if !strings.Contains(nativeOut, "MD5_EMPTY:d41d8cd98f00b204e9800998ecf8427e") ||
		!strings.Contains(nativeOut, "SHA_EMPTY:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=") ||
		!strings.Contains(nativeOut, "BCRYPT_OK:true") ||
		!strings.Contains(nativeOut, "BCRYPT_BAD:false") ||
		!strings.Contains(nativeOut, "FORM_CF:true") {
		t.Fatalf("expected crypto/http assertions missing in output:\n%s", nativeOut)
	}
}

func TestNative_BinaryBytesRoundTrip(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	rawPNG := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n', 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R', 0x00, 0x01, 0x02, 0x03}, bytes.Repeat([]byte{0x7F, 0x00, 0xAB}, 64)...)
	wantSum := sha256.Sum256(rawPNG)
	wantSHAB64 := base64.StdEncoding.EncodeToString(wantSum[:])
	var uploaded []byte
	var uploadedSHA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(rawPNG)
		case "/upload":
			uploadedSHA = r.Header.Get("x-content-sha256")
			uploaded, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("stored"))
		}
	}))
	defer srv.Close()

	source := fmt.Sprintf(`
		var $getRes = Http::request("GET", %q, {"timeout": 5});
		mixed $bin = $getRes["body"] ?? null;
		string $binStr = strval($bin);
		echo "BINLEN:" . strval(strlen($binStr));
		string $sha = sha256_base64($binStr);
		echo "BINSHA:" . $sha;
		var $putRes = Http::request("PUT", %q, {
			"timeout": 5,
			"body": $binStr,
			"headers": {"Content-Type": "application/octet-stream", "x-content-sha256": $sha}
		});
		echo "PUTSTATUS:" . strval($putRes["status"] ?? 0);
	`, srv.URL+"/download", srv.URL+"/upload")

	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "bin_roundtrip.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: "bin_roundtrip.joss"})
	irProg, err := ir.LowerAndVerify(prep, "bin_roundtrip", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}
	outExe := filepath.Join(t.TempDir(), "bin_roundtrip")
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
	outBytes, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("bin_roundtrip failed: %v\n%s", err, string(outBytes))
	}
	outStr := string(outBytes)
	if !strings.Contains(outStr, fmt.Sprintf("BINLEN:%d", len(rawPNG))) {
		t.Fatalf("expected BINLEN:%d, got:\n%s", len(rawPNG), outStr)
	}
	if !strings.Contains(outStr, "BINSHA:"+wantSHAB64) {
		t.Fatalf("expected BINSHA:%s, got:\n%s", wantSHAB64, outStr)
	}
	if !strings.Contains(outStr, "PUTSTATUS:200") {
		t.Fatalf("expected PUTSTATUS:200, got:\n%s", outStr)
	}
	if uploadedSHA != wantSHAB64 || !bytes.Equal(uploaded, rawPNG) {
		t.Fatalf("binary upload corruption: uploaded len=%d (want %d), sha=%q (want %q)", len(uploaded), len(rawPNG), uploadedSHA, wantSHAB64)
	}
}

func TestWhiteBox_JosSecurity_BlogAndOCI(t *testing.T) {
	if os.Getenv("JOSS_LIVE_PROBE") != "1" {
		t.Skip("set JOSS_LIVE_PROBE=1 to run live JosSecurity probe")
	}
	projDir, _ := filepath.Abs(filepath.Join("..", "..", "ejemplos", "Joss-Red-JosSecurity"))
	exePath := filepath.Join(projDir, "build", "main.exe")
	if verifyExe := os.Getenv("JOSS_VERIFY_EXE"); verifyExe != "" {
		exePath = verifyExe
	}

	runProbe := func(path string) string {
		cmd := exec.Command(exePath)
		cmd.Dir = filepath.Join(projDir, "build")
		var outBuf strings.Builder
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf
		if err := cmd.Start(); err != nil {
			t.Fatalf("start main.exe: %v", err)
		}
		defer func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}()

		var resp string
		var rerr error
		for i := 0; i < 50; i++ {
			conn, derr := net.DialTimeout("tcp", "127.0.0.1:80", 100*time.Millisecond)
			if derr != nil {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			_, _ = conn.Write([]byte("GET " + path + " HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"))
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			buf, err := io.ReadAll(conn)
			_ = conn.Close()
			resp = string(buf)
			rerr = err
			break
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Logf("=== PROBE %s ===\nREAD_ERR: %v\nRESP_LEN: %d\nRESP_HEAD:\n%.180s\nSERVER_LOGS:\n%s",
			path, rerr, len(resp), resp, outBuf.String())
		return resp
	}

	imgResp := runProbe("/admin/cms/media-view/cms-media/1792622644_vulnerabilidad.png")
	if !strings.Contains(imgResp, "HTTP/1.1 200 OK") || !strings.Contains(imgResp, "Content-Type: image/png") || len(imgResp) < 500000 {
		t.Fatalf("expected 200 OK PNG image (>500KB), got len=%d head=%.180s", len(imgResp), imgResp)
	}
	postResp := runProbe("/blog/analisis-cve-2026-109618-cve-2026-109619-php-uaf")
	if !strings.Contains(postResp, "HTTP/1.1 200 OK") || len(postResp) < 10000 {
		t.Fatalf("expected 200 OK blog post HTML (>10KB), got len=%d head=%.180s", len(postResp), postResp)
	}
	proseRe := regexp.MustCompile(`(?s)<article class="blog-prose">\s*([0-9]{11,})\s*</article>`)
	if proseRe.MatchString(postResp) {
		t.Fatalf("blog post prose rendered raw pointer digits instead of HTML")
	}
}

func TestNative_MixedReassignedVarNotBoxedAsPointerInt(t *testing.T) {
	source := `
public func makeMixedRow(): mixed {
    return {"user_id": "1", "title": "Hola Mundo"}
}

public func showMixed(mixed $val): string {
    return strval($val)
}

var $row = makeMixedRow()
var $id = 0
($row) ? {
    $id = $row["user_id"]
} : {
}
echo "UID:" . showMixed($id)

var $pure = 42
echo "|PURE:" . showMixed($pure)

var $content = null
($row) ? {
    $content = $row["title"]
} : {
}
echo "|CONTENT:" . strval($content)
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "reassigned_mixed.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: "reassigned_mixed.joss"})
	irProg, err := ir.LowerAndVerify(prep, "reassigned_mixed", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}
	outExe := filepath.Join(t.TempDir(), "reassigned_mixed")
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
	outBytes, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("exec failed: %v\n%s", err, string(outBytes))
	}
	got := strings.TrimSpace(strings.ReplaceAll(string(outBytes), "\r\n", "\n"))
	want := "UID:1\n|PURE:42\n|CONTENT:Hola Mundo"
	if got != want {
		t.Fatalf("unexpected output: got %q, want %q", got, want)
	}
}

func TestNative_TryCatchMutationNotShadowedByOptimizer(t *testing.T) {
	source := `
var $r = null
try {
    $r = {"status": 200, "ok": true}
} catch ($e) {
}
echo "S=" . ($r["status"] ?? 0) . ",OK=" . ($r["ok"] ?? false) . ",RAWBOOL=" . true
`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	units := append([]analyzer.SourceUnit{{Path: "trycatch_opt.joss", Program: prog}}, std.GetStdlibUnits()...)
	prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(prep, analyzer.ReachabilityOptions{Entrypoint: "trycatch_opt.joss"})
	irProg, err := ir.LowerAndVerify(prep, "trycatch_opt", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}
	outExe := filepath.Join(t.TempDir(), "trycatch_opt")
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
	outBytes, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("exec failed: %v\n%s", err, string(outBytes))
	}
	got := strings.TrimSpace(string(outBytes))
	want := "S=200,OK=true,RAWBOOL=true"
	if got != want {
		t.Fatalf("unexpected output: got %q, want %q", got, want)
	}
}

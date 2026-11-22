package native_test

import (
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func TestWhiteBox_JosSecurity_BlogAndOCI(t *testing.T) {
	projDir, _ := filepath.Abs(filepath.Join("..", "..", "ejemplos", "Joss-Red-JosSecurity"))
	exePath := filepath.Join(projDir, "build", "main.exe")

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
}

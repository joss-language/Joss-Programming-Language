package native_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
)

func httpGetLocal(addr, path string) (string, error) {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_, err = conn.Write([]byte("GET " + path + " HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"))
	if err != nil {
		return "", err
	}
	var resp bytes.Buffer
	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rerr := conn.Read(buf)
		if n > 0 {
			resp.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	return resp.String(), nil
}

func TestNativeWebApp_TutorialWeb_LiveHTTP(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Clean(filepath.Join(origWd, "..", "..", "ejemplos", "tutorial-web"))
	if err := os.Chdir(webDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	units, parseDiags := analyzer.LoadProject("main.joss", "app")
	if len(parseDiags) > 0 {
		t.Fatalf("parse diags: %v", parseDiags)
	}
	report := core.AnalyzeSourceUnits(units)
	if report.HasErrors() {
		t.Fatalf("analysis errors: %v", report.Diagnostics)
	}
	reach := analyzer.BuildReachabilityGraph(report.Prepared, analyzer.ReachabilityOptions{
		Entrypoint: "main.joss",
		RouteFiles: []string{"routes.joss", "api.joss"},
	})
	irProg, err := ir.LowerAndVerify(report.Prepared, "tutorial_web", reach)
	if err != nil {
		t.Fatalf("LowerAndVerify failed: %v", err)
	}

	target := backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if err := backend.ValidateAOTCapabilitiesForBackend(irProg, target, backend.BackendLLVM); err != nil {
		t.Fatalf("ValidateAOTCapabilitiesForBackend(BackendLLVM) failed:\n%v", err)
	}

	outExe := filepath.Join(t.TempDir(), "tutorial_web_native")
	if runtime.GOOS == "windows" {
		outExe += ".exe"
	}
	res, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     target,
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	})
	if err != nil {
		t.Fatalf("BuildProgram(BackendLLVM) failed:\n%v", err)
	}
	t.Logf("Compiled standalone web app binary: %d bytes (zero Go runtime)", res.ExecutableSize)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, outExe)
	cmd.Env = append(os.Environ(), "PORT=18999")
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start native web server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	var homeResp string
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		if resp, rerr := httpGetLocal("127.0.0.1:18999", "/"); rerr == nil && len(resp) > 0 {
			homeResp = resp
			break
		}
	}
	if !strings.Contains(homeResp, "HTTP/1.1 200 OK") || !strings.Contains(homeResp, "Bienvenido a") {
		t.Fatalf("unexpected GET / response:\n%s\nserver logs:\n%s", homeResp, outBuf.String())
	}

	saludoResp, err := httpGetLocal("127.0.0.1:18999", "/saludo/Joss")
	if err != nil || !strings.Contains(saludoResp, "HTTP/1.1 200 OK") || !strings.Contains(saludoResp, "Joss") {
		t.Fatalf("unexpected GET /saludo/Joss response (err=%v):\n%s", err, saludoResp)
	}

	loginResp, err := httpGetLocal("127.0.0.1:18999", "/login")
	if err != nil || !strings.Contains(loginResp, "HTTP/1.1 200 OK") {
		t.Fatalf("unexpected GET /login response (err=%v):\n%s", err, loginResp)
	}
}

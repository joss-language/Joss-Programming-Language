package native_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestNative_ServerExpressiveLogging(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang/zig not available on PATH")
	}

	src := `
HttpServer::logResponse("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n", "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n<h1>Hi</h1>")
HttpServer::logResponse("GET /admin/cms/media-view/cms-media/cookie.png HTTP/1.1\r\nHost: localhost\r\n\r\n", "HTTP/1.1 202 Accepted\r\nContent-Type: image/png\r\n\r\nPNGDATA")
HttpServer::logResponse("GET /admin/cms/media-view/cms-media/missing.jpg HTTP/1.1\r\nHost: localhost\r\n\r\n", "HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\n\r\n{\"error\":\"Imagen no encontrada en la nube (OCI)\"}")
HttpServer::logResponse("POST /api/crash HTTP/1.1\r\nHost: localhost\r\n\r\n", "HTTP/1.1 500 Internal Server Error\r\nContent-Type: application/json\r\n\r\n{\"error\":\"DB connection lost\",\"status\":500}")
HttpServer::logResponse("GET /__joss_health HTTP/1.1\r\nHost: localhost\r\n\r\n", "HTTP/1.1 200 OK\r\n\r\n12345")
`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse failed: %v", errs)
	}
	units := append([]analyzer.SourceUnit{{Path: "main.joss", Program: prog}}, std.GetStdlibUnits()...)
	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("semantic analysis failed: %v", prepared.Diagnostics)
	}
	irProg, err := ir.LowerAndVerify(prepared, "server_logging_test", nil)
	if err != nil {
		t.Fatalf("lower/verify failed: %v", err)
	}

	outExe := filepath.Join(t.TempDir(), "server_logging_test")
	if backend.HostTarget().OS == "windows" {
		outExe += ".exe"
	}
	if _, err := backend.BuildProgram(irProg, backend.BuildOptions{
		Target:     backend.HostTarget(),
		Backend:    backend.BackendLLVM,
		OutputPath: outExe,
	}); err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}

	rawOut, err := exec.Command(outExe).CombinedOutput()
	if err != nil {
		t.Fatalf("executable failed: %v\n%s", err, string(rawOut))
	}
	out := strings.ReplaceAll(string(rawOut), "\r\n", "\n")

	for _, want := range []string{
		"[Server] [status:200] GET /",
		"[Server] [status:202] GET /admin/cms/media-view/cms-media/cookie.png",
		`[Server] [status:404] [{"error":"Imagen no encontrada en la nube (OCI)"}] GET /admin/cms/media-view/cms-media/missing.jpg`,
		"[SERVER ERROR 500] [status:500] POST /api/crash",
		`[{"error":"DB connection lost","status":500}]`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected log line %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/__joss_health") {
		t.Errorf("internal hot-reload health endpoint should not be logged:\n%s", out)
	}
}

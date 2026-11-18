package native_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestNativeSessionContextFlowsThroughCompiledHTTPDispatcher(t *testing.T) {
	sources := []struct {
		path string
		code string
	}{
		{
			path: "main.joss",
			code: `print(__joss_http_dispatch("GET", "/session", {}, {"user_id": 42}, ""));`,
		},
		{
			path: "routes.joss",
			code: `Router::get("/session", "SessionController@show");`,
		},
		{
			path: "SessionController.joss",
			code: `public class SessionController { public func show(): string { Session::put("user_id", 42); echo Session::get("user_id"); echo Session::has("user_id"); Session::forget("user_id"); echo Session::has("user_id"); return "ok"; } }`,
		},
	}
	units := make([]analyzer.SourceUnit, 0, len(sources)+1)
	for _, source := range sources {
		p := parser.NewParser(parser.NewLexer(source.code))
		program := p.ParseProgram()
		if parseErrors := p.Errors(); len(parseErrors) > 0 {
			t.Fatalf("parse %s: %v", source.path, parseErrors)
		}
		units = append(units, analyzer.SourceUnit{Path: source.path, Program: program})
	}
	for _, unit := range std.GetStdlibUnits() {
		if unit.Path == "<std/http/session_host.joss>" {
			units = append(units, unit)
			break
		}
	}
	if len(units) != len(sources)+1 {
		t.Fatal("canonical Joss Session host-service implementation is missing from embedded Standard Library")
	}

	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("semantic analysis failed: %v", prepared.Diagnostics)
	}
	lowerer := ir.NewLowerer(prepared, nil)
	program, err := lowerer.LowerProgram("session_context")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(program); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}
	if program.Functions["__joss_http_dispatch"] == nil || program.Globals["__joss_request_session"] != nil {
		t.Fatal("request session context must be passed through dispatcher arguments, not a process global")
	}
	if controller := program.Functions["SessionController_show"]; controller == nil || controller.Attributes["request_session_context"] != "true" {
		t.Fatal("route controller did not receive the hidden request-local session parameter")
	}
	if sessionPolicy := program.Functions["__joss_host_Session_get"]; sessionPolicy == nil {
		t.Fatal("lowering dropped the Joss-owned Session::get policy")
	} else {
		foundMapGet := false
		for _, block := range sessionPolicy.Blocks {
			for _, instruction := range block.Instructions {
				if _, ok := instruction.(*ir.MapGetInst); ok {
					foundMapGet = true
				}
				if call, ok := instruction.(*ir.CallBuiltinInst); ok && call.BuiltinID == "session" {
					t.Fatal("native session context read remained a backend-specific session builtin instead of generic IR")
				}
			}
		}
		if !foundMapGet {
			t.Fatal("Joss Session policy was not lowered to generic MapGet IR")
		}
	}
	if sessionPut := program.Functions["__joss_host_Session_put"]; sessionPut == nil {
		t.Fatal("lowering dropped the Joss-owned Session::put policy")
	} else {
		foundMapSet := false
		for _, block := range sessionPut.Blocks {
			for _, instruction := range block.Instructions {
				if _, ok := instruction.(*ir.MapSetInst); ok {
					foundMapSet = true
				}
			}
		}
		if !foundMapSet {
			t.Fatal("Joss Session::put policy was not lowered to generic MapSet IR")
		}
	}
	for _, target := range []backend.Target{
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"},
		{OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("native session dispatch compilation failed for %s: %v", target.DashString(), err)
		}
	}

	outPath := filepath.Join(t.TempDir(), "session_context")
	if backend.HostTarget().OS == "windows" {
		outPath += ".exe"
	}
	if _, err := backend.BuildProgram(program, backend.BuildOptions{Target: backend.HostTarget(), OutputPath: outPath}); err != nil {
		t.Fatalf("native build failed: %v", err)
	}
	output, err := exec.Command(outPath).CombinedOutput()
	if err != nil {
		t.Fatalf("native executable failed: %v, output=%q", err, output)
	}
	got := strings.ReplaceAll(strings.TrimSpace(string(output)), "\r\n", "\n")
	if got != "42\ntrue\nfalse\nok" {
		t.Fatalf("native request session output = %q, want %q", got, "42\ntrue\nfalse\nok")
	}
}

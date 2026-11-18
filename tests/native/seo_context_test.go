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

func TestNativeSEOUsesPureJossAndRequestLocalState(t *testing.T) {
	sources := []struct{ path, code string }{
		{path: "main.joss", code: `print(__joss_http_dispatch("GET", "/seo", {}, {}, "")); print(__joss_http_dispatch("GET", "/blank", {}, {}, ""));`},
		{path: "routes.joss", code: `Router::get("/seo", "SEOController@show"); Router::get("/blank", "SEOController@blank");`},
		{path: "SEOController.joss", code: `public class SEOController { public func show(): string { SEO::title("T & <"); SEO::description("D \"quoted\""); SEO::keywords("joss,lang"); SEO::canonical("/canonical"); SEO::og("image", "/cover.png"); SEO::meta("robots", "index"); return SEO::render(); } public func blank(): string { return SEO::render(); } }`},
	}
	units := make([]analyzer.SourceUnit, 0, len(sources)+len(std.GetStdlibUnits()))
	for _, source := range sources {
		p := parser.NewParser(parser.NewLexer(source.code))
		program := p.ParseProgram()
		if parseErrors := p.Errors(); len(parseErrors) > 0 {
			t.Fatalf("parse %s: %v", source.path, parseErrors)
		}
		units = append(units, analyzer.SourceUnit{Path: source.path, Program: program})
	}
	units = append(units, std.GetStdlibUnits()...)
	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("semantic analysis failed: %v", prepared.Diagnostics)
	}
	program, err := ir.NewLowerer(prepared, nil).LowerProgram("seo_context")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	if err := ir.NewVerifier().Verify(program); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}
	if fn := program.Functions["SEOController_show"]; fn == nil || fn.Attributes["request_seo_context"] != "true" {
		t.Fatal("SEO controller must receive request-local context through the common IR call graph")
	}
	for _, function := range program.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instructions {
				if call, ok := instruction.(*ir.CallHostServiceInst); ok && call.ClassName == "SEO" {
					t.Fatalf("SEO::%s remained a host service instead of a Joss call", call.MethodName)
				}
			}
		}
	}
	for _, target := range []backend.Target{
		{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"}, {OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"},
		{OS: "android", Arch: "arm64"},
	} {
		if _, _, err := backend.SelectCodeEmitter(target, false).Compile(program); err != nil {
			t.Errorf("native SEO compile failed for %s: %v", target.DashString(), err)
		}
	}
	outPath := filepath.Join(t.TempDir(), "seo_context")
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
	want := "<title>T &amp; &lt;</title>\n<meta name=\"description\" content=\"D &quot;quoted&quot;\">\n<meta name=\"keywords\" content=\"joss, lang\">\n<link rel=\"canonical\" href=\"/canonical\">\n<meta name=\"robots\" content=\"index\">\n<meta property=\"og:image\" content=\"/cover.png\">\n<meta property=\"og:title\" content=\"T &amp; &lt;\">\n<meta property=\"og:description\" content=\"D &quot;quoted&quot;\">\n<meta name=\"twitter:card\" content=\"summary_large_image\">\n\n<meta name=\"twitter:card\" content=\"summary_large_image\">"
	if got != want {
		t.Fatalf("native SEO output = %q, want %q", got, want)
	}
}

package native_test

import (
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestPureJossNowCompilesForSupportedTargets(t *testing.T) {
	parsed := parser.NewParser(parser.NewLexer(`echo now("Y-m-d H:i:s T");`))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatalf("parse now() fixture: %v", errs)
	}
	units := []analyzer.SourceUnit{{Path: "now_target_test.joss", Program: program}}
	for _, unit := range std.GetStdlibUnits() {
		if strings.HasPrefix(unit.Path, "<std/date/") {
			units = append(units, unit)
		}
	}
	prepared := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
	if prepared.HasErrors() {
		t.Fatalf("analyze now() fixture: %v", prepared.Diagnostics)
	}
	lowered, err := ir.NewLowerer(prepared, nil).LowerProgram("pure_joss_now_targets")
	if err != nil {
		t.Fatalf("lower now() fixture: %v", err)
	}
	if err := (ir.NewVerifier()).Verify(lowered); err != nil {
		t.Fatalf("verify now() IR: %v", err)
	}
	targets := []backend.Target{
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "windows", Arch: "amd64"},
		{OS: "windows", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
	}
	for _, target := range targets {
		t.Run(target.DashString(), func(t *testing.T) {
			binary, _, err := backend.SelectCodeEmitter(target, false).Compile(lowered)
			if err != nil {
				t.Fatalf("compile %s: %v", target, err)
			}
			if len(binary) == 0 {
				t.Fatalf("compile %s emitted an empty binary", target)
			}
		})
	}
}

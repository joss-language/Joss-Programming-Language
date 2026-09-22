package core

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func parseAndRunSource(r *Runtime, source string) (panickedErr interface{}) {
	defer func() {
		if rec := recover(); rec != nil {
			panickedErr = rec
		}
	}()
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	r.Execute(program)
	return nil
}

func TestProcessCapabilityDeniedWhenRestricted(t *testing.T) {
	r := NewRuntime()
	defer r.Free()

	r.RestrictedMode = true
	r.Env["ALLOW_SYSTEM_RUN"] = "true"

	err := parseAndRunSource(r, `$p = new Process("echo", ["hello"])`)
	if err == nil {
		t.Fatal("expected SecurityError when Process executed in restricted mode, got nil")
	}
	jerr, ok := err.(*JossError)
	if !ok || jerr.Type != "SecurityError" {
		t.Fatalf("expected SecurityError, got %#v", err)
	}
}

func TestFSCapabilityDeniedWhenDisabled(t *testing.T) {
	r := NewRuntime()
	defer r.Free()

	r.Capabilities.AllowFS = false

	err := parseAndRunSource(r, `$content = file_get_contents("some_file.txt")`)
	if err == nil {
		t.Fatal("expected SecurityError when file_get_contents executed with AllowFS=false, got nil")
	}
	jerr, ok := err.(*JossError)
	if !ok || jerr.Type != "SecurityError" {
		t.Fatalf("expected SecurityError, got %#v", err)
	}
}

func TestCapabilitiesPropagateToFork(t *testing.T) {
	r := NewRuntime()
	defer r.Free()

	r.Capabilities.AllowProcess = false
	r.RestrictedMode = true

	forked := r.Fork()
	defer forked.Free()

	if forked.Capabilities.AllowProcess != false {
		t.Fatal("expected forked runtime to inherit AllowProcess=false")
	}
	if forked.RestrictedMode != true {
		t.Fatal("expected forked runtime to inherit RestrictedMode=true")
	}
	if forked.CanExecuteProcess() {
		t.Fatal("expected CanExecuteProcess to be false on forked runtime")
	}
}

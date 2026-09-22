package core

import (
	"fmt"
	"path/filepath"
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

func TestNativeStreamsRespectFilesystemCapability(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "denied.txt"))
	for _, source := range []string{
		fmt.Sprintf(`$stream = new FileStream(); $stream->open(%q, "w");`, path),
		fmt.Sprintf(`$reader = new StreamReader(); $reader->open(%q);`, path),
		fmt.Sprintf(`$writer = new StreamWriter(); $writer->open(%q);`, path),
	} {
		r := NewRuntime()
		r.Capabilities.AllowFS = false
		got := parseAndRunSource(r, source)
		r.Free()
		jerr, ok := got.(*JossError)
		if !ok || jerr.Type != "SecurityError" {
			t.Fatalf("source %q bypassed filesystem capability: %#v", source, got)
		}
	}
}

func TestNativeSocketRespectsNetworkCapability(t *testing.T) {
	for _, method := range []string{"listen", "connect"} {
		r := NewRuntime()
		r.Capabilities.AllowNetwork = false
		got := parseAndRunSource(r, fmt.Sprintf(`$socket = new Socket(); $socket->%s("127.0.0.1", "0");`, method))
		r.Free()
		jerr, ok := got.(*JossError)
		if !ok || jerr.Type != "SecurityError" {
			t.Fatalf("Socket::%s bypassed network capability: %#v", method, got)
		}
	}
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

package native_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func compileSourceToNativeExecutable(t *testing.T, source string, exeName string) (string, func()) {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: exeName + ".joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram(exeName)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, exeName+".exe")

	builder := backend.NewStandaloneBuilder()
	if err := builder.BuildNativeExecutable(irProg, outExe); err != nil {
		t.Fatalf("native build failed: %v", err)
	}

	cleanup := func() {
		_ = os.Remove(outExe)
	}

	return outExe, cleanup
}

func runNativeBinary(t *testing.T, exePath string) (string, int) {
	t.Helper()
	cmd := exec.Command(exePath)
	outBytes, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", exePath, err)
		}
	}
	return strings.TrimSpace(string(outBytes)), exitCode
}

func TestEndToEnd_EmptyMain(t *testing.T) {
	src := `
		int $x = 0;
		return $x;
	`
	exe, cleanup := compileSourceToNativeExecutable(t, src, "test_empty")
	defer cleanup()

	_, code := runNativeBinary(t, exe)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestEndToEnd_HelloWorld(t *testing.T) {
	src := `
		echo "Hello World";
	`
	exe, cleanup := compileSourceToNativeExecutable(t, src, "test_hello")
	defer cleanup()

	out, code := runNativeBinary(t, exe)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if out != "Hello World" {
		t.Errorf("expected output 'Hello World', got %q", out)
	}
}

func TestEndToEnd_Arithmetic(t *testing.T) {
	src := `
		int $x = 10;
		int $y = 20;
		int $z = $x + $y;
		echo $z;
	`
	exe, cleanup := compileSourceToNativeExecutable(t, src, "test_arith")
	defer cleanup()

	out, code := runNativeBinary(t, exe)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if out != "30" {
		t.Errorf("expected output '30', got %q", out)
	}
}

func TestEndToEnd_FunctionsAndControlFlow(t *testing.T) {
	src := `
		public func max(int $a, int $b): int {
			int $res = ($a > $b) ? $a : $b;
			return $res;
		}

		int $val = max(50, 100);
		echo $val;
	`
	exe, cleanup := compileSourceToNativeExecutable(t, src, "test_func")
	defer cleanup()

	out, code := runNativeBinary(t, exe)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if out != "100" {
		t.Errorf("expected output '100', got %q", out)
	}
}

func TestEndToEnd_Recursion(t *testing.T) {
	src := `
		public func fib(int $n): int {
			int $res = ($n <= 1) ? $n : (fib($n - 1) + fib($n - 2));
			return $res;
		}

		int $r = fib(7);
		echo $r;
	`
	exe, cleanup := compileSourceToNativeExecutable(t, src, "test_fib")
	defer cleanup()

	out, code := runNativeBinary(t, exe)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if out != "13" {
		t.Errorf("expected output '13', got %q", out)
	}
}

func TestEndToEnd_ExtremeValuesAndTypes(t *testing.T) {
	src := `
		int $zero = 0;
		int $neg = -1;
		int $max = 9223372036854775807;
		echo $zero;
		echo $neg;
		echo $max;
	`
	exe, cleanup := compileSourceToNativeExecutable(t, src, "test_extreme")
	defer cleanup()

	out, code := runNativeBinary(t, exe)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	expected := "0\n-1\n9223372036854775807"
	normOut := strings.ReplaceAll(out, "\r\n", "\n")
	if normOut != expected {
		t.Errorf("expected %q, got %q", expected, normOut)
	}
}

package ir_test

import (
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func compileSourceToIR(t *testing.T, source string) (*ir.Program, string) {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "test.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("semantic errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("test_module")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	return irProg, irProg.Dump()
}

func TestIR_BasicArithmeticAndReturn(t *testing.T) {
	src := `
		int $a = 10;
		int $b = 20;
		return $a + $b;
	`
	_, dump := compileSourceToIR(t, src)

	if !strings.Contains(dump, "alloca i64") {
		t.Errorf("expected alloca i64 in dump, got:\n%s", dump)
	}
	if !strings.Contains(dump, "add") {
		t.Errorf("expected add instruction in dump, got:\n%s", dump)
	}
	if !strings.Contains(dump, "return") {
		t.Errorf("expected return instruction in dump, got:\n%s", dump)
	}
}

func TestIR_ComparisonAndTernary(t *testing.T) {
	src := `
		int $x = 15;
		int $res = ($x > 10) ? 100 : 200;
		return $res;
	`
	_, dump := compileSourceToIR(t, src)

	if !strings.Contains(dump, "cmp_gt") {
		t.Errorf("expected cmp_gt instruction, got:\n%s", dump)
	}
	if !strings.Contains(dump, "branch") {
		t.Errorf("expected branch instruction for ternary, got:\n%s", dump)
	}
	if !strings.Contains(dump, "ternary_true") || !strings.Contains(dump, "ternary_false") {
		t.Errorf("expected ternary blocks, got:\n%s", dump)
	}
}

func TestIR_WhileLoopAndGuard(t *testing.T) {
	src := `
		int $i = 0;
		while ($i < 10) {
			$i = $i + 1;
			guard ($i < 5) else {
				return $i;
			}
		}
		return $i;
	`
	_, dump := compileSourceToIR(t, src)

	if !strings.Contains(dump, "while_cond") {
		t.Errorf("expected while_cond block, got:\n%s", dump)
	}
	if !strings.Contains(dump, "while_body") {
		t.Errorf("expected while_body block, got:\n%s", dump)
	}
	if !strings.Contains(dump, "while_exit") {
		t.Errorf("expected while_exit block, got:\n%s", dump)
	}
	if !strings.Contains(dump, "guard_pass") || !strings.Contains(dump, "guard_fail") {
		t.Errorf("expected guard blocks, got:\n%s", dump)
	}
}

func TestIR_FunctionDefinitionAndCall(t *testing.T) {
	src := `
		public func add(int $x, int $y): int {
			return $x + $y;
		}

		int $val = add(5, 7);
		return $val;
	`
	prog, dump := compileSourceToIR(t, src)

	if prog.Functions["add"] == nil {
		t.Fatalf("expected function @add in IR program")
	}
	if prog.Functions["main"] == nil {
		t.Fatalf("expected function @main in IR program")
	}
	if !strings.Contains(dump, "call @add") {
		t.Errorf("expected call @add instruction, got:\n%s", dump)
	}
}

func TestIR_RuntimeRequirementsTracking(t *testing.T) {
	src := `
		echo "Hola Mundo";
	`
	prog, dump := compileSourceToIR(t, src)

	if !prog.RuntimeRequirements["print"] {
		t.Errorf("expected 'print' in RuntimeRequirements")
	}
	if !strings.Contains(dump, "call_runtime \"print_string\"") {
		t.Errorf("expected call_runtime print_string in dump, got:\n%s", dump)
	}
}

func TestIR_VerifierCatchesInvalidIR(t *testing.T) {
	fn := ir.NewFunction("invalid_fn", ir.TypeI64)
	// No terminator in entry block!
	prog := ir.NewProgram("invalid_prog")
	prog.AddFunction(fn)

	verifier := ir.NewVerifier()
	err := verifier.Verify(prog)
	if err == nil {
		t.Fatalf("expected verifier error for missing terminator, got nil")
	}
	if !strings.Contains(err.Error(), "carece de terminador") {
		t.Errorf("unexpected verifier error message: %v", err)
	}
}

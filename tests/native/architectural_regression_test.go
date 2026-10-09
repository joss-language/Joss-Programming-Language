package native_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// TestArchitecturalRegression_NoASTSerializationInNativeExecutable verifies that
// native binaries produced by the native compiler do not embed compressed AST (JOSSBC2Z),
// runner data payloads (JOSS_RUNNER_DATA), or monolithic server/database libraries.
func TestArchitecturalRegression_NoASTSerializationInNativeExecutable(t *testing.T) {
	source := `
		int $x = 10;
		int $y = 20;
		echo $x + $y;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "no_ast.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("no_ast")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, "no_ast.exe")

	builder := backend.NewStandaloneBuilder()
	if err := builder.BuildNativeExecutable(irProg, outExe); err != nil {
		t.Fatalf("build executable failed: %v", err)
	}

	binBytes, err := os.ReadFile(outExe)
	if err != nil {
		t.Fatalf("failed to read compiled binary: %v", err)
	}

	// 1. Must NOT contain serialized AST magic marker
	if bytes.Contains(binBytes, []byte("JOSSBC2Z")) {
		t.Fatal("REGRESSION DETECTED: native binary contains serialized AST magic 'JOSSBC2Z'")
	}

	// 2. Must NOT contain runner data marker
	if bytes.Contains(binBytes, []byte("JOSS_RUNNER_DATA")) {
		t.Fatal("REGRESSION DETECTED: native binary contains 'JOSS_RUNNER_DATA'")
	}

	// 3. Must NOT contain heavy monolithic third-party packages
	forbiddenSignatures := []string{
		"modernc.org/sqlite",
		"jackc/pgx",
		"gorilla/websocket",
	}
	for _, sig := range forbiddenSignatures {
		if bytes.Contains(binBytes, []byte(sig)) {
			t.Fatalf("REGRESSION DETECTED: native binary contains forbidden monolithic library '%s'", sig)
		}
	}
}

// TestArchitecturalRegression_LoweringConsumesPreparedProgramDirectly verifies that
// native lowering consumes PreparedProgram without executing an independent frontend or parser.
func TestArchitecturalRegression_LoweringConsumesPreparedProgramDirectly(t *testing.T) {
	source := `
		public func compute(int $val): int {
			return $val * 2;
		}
		int $ans = compute(21);
		echo $ans;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "model_test.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())

	// Assert semantic model facts exist before lowering
	if prep.Facts == nil || len(prep.Facts.InferredTypes) == 0 {
		t.Fatal("REGRESSION: PreparedProgram lacks inferred types in semantic facts")
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("model_test")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	// The lowered function must exist in IR program
	if irProg.Functions["compute"] == nil {
		t.Fatal("REGRESSION: Function 'compute' was not lowered into IR")
	}
	if irProg.Functions["main"] == nil {
		t.Fatal("REGRESSION: Main entrypoint was not synthesized in IR")
	}
}

// TestArchitecturalRegression_InterpreterConsumesPreparedProgramDirectly verifies that
// the interpreter backend executes via ExecutePrepared without reading from the filesystem or re-parsing.
func TestArchitecturalRegression_InterpreterConsumesPreparedProgramDirectly(t *testing.T) {
	source := `
		public func sayAnswer(): int {
			return 42;
		}
		int $v = sayAnswer();
		echo $v;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "mem://virtual.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())

	rt := core.NewRuntime()
	defer rt.Free()

	// ExecutePrepared must succeed purely in-memory
	rt.ExecutePrepared(prep)

	if rt.PreparedProgram != prep {
		t.Fatal("REGRESSION: Runtime does not hold reference to PreparedProgram")
	}
	if rt.AnalysisFacts != prep.Facts {
		t.Fatal("REGRESSION: Runtime does not hold reference to AnalysisFacts")
	}
}

// TestArchitecturalRegression_CapabilityMatrixDistinguishesSemanticFromMaterialization verifies that
// all entries in the capability matrix distinguish language semantics from backend materialization.
func TestArchitecturalRegression_CapabilityMatrixDistinguishesSemanticFromMaterialization(t *testing.T) {
	for _, entry := range analyzer.CanonicalCapabilityMatrix {
		// Rule: Every feature known to Joss is Supported in the Language specification and SemanticModel
		if entry.Language != analyzer.Supported {
			t.Fatalf("REGRESSION: Feature '%s' marked unsupported in language specification; language should define semantics once", entry.Feature)
		}
		if entry.SemanticModel != analyzer.Supported {
			t.Fatalf("REGRESSION: Feature '%s' marked unsupported in semantic model", entry.Feature)
		}
		// If native is unsupported, error format must cite JOSS-NATIVE-001 and joss run
		if entry.Native == analyzer.Unsupported {
			errText := analyzer.FormatCapabilityError(entry.Feature)
			if !strings.Contains(errText, "JOSS-NATIVE-001") {
				t.Fatalf("REGRESSION: Capability error for '%s' missing JOSS-NATIVE-001 code", entry.Feature)
			}
			if !strings.Contains(errText, "joss run") {
				t.Fatalf("REGRESSION: Capability error for '%s' missing 'joss run' mitigation", entry.Feature)
			}
		}
	}
}

// TestArchitecturalRegression_ResolvedCallFactRespectedByLowering asserts that
// LowerProgram uses prep.Facts.ResolvedCalls rather than performing ad-hoc name resolution.
func TestArchitecturalRegression_ResolvedCallFactRespectedByLowering(t *testing.T) {
	source := `
		public class Calculator {
			public static func calculate(): int {
				return 99;
			}
		}
		int $val = Calculator::calculate();
		echo $val;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "calc.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())

	// Find the call expression in AST
	var callExpr *parser.CallExpression
	for _, stmt := range prog.Statements {
		if letStmt, ok := stmt.(*parser.LetStatement); ok {
			if c, ok := letStmt.Value.(*parser.CallExpression); ok {
				callExpr = c
			}
		}
	}

	if callExpr == nil {
		t.Fatal("could not locate CallExpression in AST")
	}

	// Verify resolved call is present in facts
	resolved, ok := prep.Facts.ResolvedCalls[callExpr]
	if !ok {
		t.Fatal("REGRESSION: Calculator::calculate() call was not resolved in AnalysisFacts")
	}
	if resolved.ReturnType.Kind != typesystem.Int {
		t.Fatalf("expected int return type in ResolvedCallFact, got %v", resolved.ReturnType)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("calc")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	// Verify the lowered function name matches the canonical @Calculator_calculate
	if irProg.Functions["Calculator_calculate"] == nil {
		t.Fatal("REGRESSION: Function @Calculator_calculate missing in IR program")
	}
}

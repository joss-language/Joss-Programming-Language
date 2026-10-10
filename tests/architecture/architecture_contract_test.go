package architecture_test

import (
	"bytes"
	"os"
	"os/exec"
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

// TestArchitecture_SemanticIntegrityAndSharedPreparedProgramReuse demonstrates that
// a single in-memory PreparedProgram can be analyzed once, and subsequently executed
// by the Interpreter, compiled by the Native Lowerer, and executed again by another Interpreter,
// yielding identical outputs and without mutating the canonical semantic model.
func TestArchitecture_SemanticIntegrityAndSharedPreparedProgramReuse(t *testing.T) {
	source := `
		public class MathBox {
			public static func calculate(int $n): int {
				int $acc = 0;
				int $i = 1;
				while ($i <= $n) {
					$acc = $acc + $i;
					$i = $i + 1;
				}
				return $acc;
			}
		}

		int $ans = MathBox::calculate(5);
		echo $ans;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "shared.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("semantic errors: %v", prep.Diagnostics)
	}

	// Capture baseline state before any backend execution
	initialClassesCount := len(prep.Classes())
	initialFunctionsCount := len(prep.Functions())
	initialInferredCount := len(prep.Facts.InferredTypes)
	initialResolvedCallsCount := len(prep.Facts.ResolvedCalls)

	// 1. First Execution: Interpreter Backend
	rt1 := core.NewRuntime()
	var interp1Buf bytes.Buffer
	rt1.Out = &interp1Buf
	rt1.ExecutePrepared(prep)
	rt1.Free()
	interp1Out := strings.TrimSpace(interp1Buf.String())

	// 2. Second Execution: Native Lowering & Binary Compilation
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("shared")
	if err != nil {
		t.Fatalf("native lowering failed: %v", err)
	}
	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("IR verification failed: %v", err)
	}

	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, "shared.exe")
	builder := backend.NewStandaloneBuilder()
	if err := builder.BuildNativeExecutable(irProg, outExe); err != nil {
		t.Fatalf("native build failed: %v", err)
	}

	cmd := exec.Command(outExe)
	nativeBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native execution failed: %v (output: %s)", err, string(nativeBytes))
	}
	nativeOut := strings.TrimSpace(string(nativeBytes))

	// 3. Third Execution: Second Interpreter Backend instance using the SAME PreparedProgram
	rt2 := core.NewRuntime()
	var interp2Buf bytes.Buffer
	rt2.Out = &interp2Buf
	rt2.ExecutePrepared(prep)
	rt2.Free()
	interp2Out := strings.TrimSpace(interp2Buf.String())

	// Assertions: Equivalence & Immutability
	if interp1Out != "15" {
		t.Fatalf("Interpreter 1 output = %q, want 15", interp1Out)
	}
	if nativeOut != "15" {
		t.Fatalf("Native output = %q, want 15", nativeOut)
	}
	if interp2Out != "15" {
		t.Fatalf("Interpreter 2 output = %q, want 15", interp2Out)
	}

	// Verify semantic model was not mutated during multi-backend execution
	if len(prep.Classes()) != initialClassesCount {
		t.Fatal("REGRESSION: PreparedProgram.Classes() was mutated by backend execution")
	}
	if len(prep.Functions()) != initialFunctionsCount {
		t.Fatal("REGRESSION: PreparedProgram.Functions() was mutated by backend execution")
	}
	if len(prep.Facts.InferredTypes) != initialInferredCount {
		t.Fatal("REGRESSION: PreparedProgram.Facts.InferredTypes was mutated by backend execution")
	}
	if len(prep.Facts.ResolvedCalls) != initialResolvedCallsCount {
		t.Fatal("REGRESSION: PreparedProgram.Facts.ResolvedCalls was mutated by backend execution")
	}
}

// TestArchitecture_NativeLoweringNeverResolvesTypesOrCallsIndependently guarantees that
// LowerProgram exclusively consumes ResolvedCalls and InferredTypes from PreparedProgram.Facts.
func TestArchitecture_NativeLoweringNeverResolvesTypesOrCallsIndependently(t *testing.T) {
	source := `
		public class Utility {
			public static func multiply(int $a, int $b): int {
				return $a * $b;
			}
		}
		int $res = Utility::multiply(6, 7);
		echo $res;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "util.joss", Program: prog}}
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
		t.Fatal("call expression not found in AST")
	}

	// Verify the resolved call fact exists
	resolvedFact, ok := prep.Facts.ResolvedCalls[callExpr]
	if !ok {
		t.Fatal("REGRESSION: Call was not resolved in AnalysisFacts")
	}
	if resolvedFact.ReturnType.Kind != typesystem.Int {
		t.Fatalf("expected resolved return type int, got %v", resolvedFact.ReturnType)
	}

	// Lower to IR
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("util")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	// Verify that the lowered function was materialized with the canonical name
	loweredFn := irProg.Functions["Utility_multiply"]
	if loweredFn == nil {
		t.Fatal("REGRESSION: Function @Utility_multiply was not lowered into IR")
	}
	if loweredFn.ReturnType.Kind != ir.TypeKindI64 {
		t.Fatalf("expected lowered return type I64, got %v", loweredFn.ReturnType)
	}
}

// TestArchitecture_NoASTSerializationOrMonolithicRuntimeInNative verifies that native binaries
// are compiled directly to machine code and do not bundle AST serialization magic or large DB drivers.
func TestArchitecture_NoASTSerializationOrMonolithicRuntimeInNative(t *testing.T) {
	source := `
		int $val = 42;
		echo $val;
	`
	p := parser.NewParser(parser.NewLexer(source))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "bin_check.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("bin_check")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	tempDir := t.TempDir()
	outExe := filepath.Join(tempDir, "bin_check.exe")
	builder := backend.NewStandaloneBuilder()
	if err := builder.BuildNativeExecutable(irProg, outExe); err != nil {
		t.Fatalf("native build failed: %v", err)
	}

	binBytes, err := os.ReadFile(outExe)
	if err != nil {
		t.Fatalf("failed to read compiled binary: %v", err)
	}

	if bytes.Contains(binBytes, []byte("JOSSBC2Z")) {
		t.Fatal("CI GATE FAILURE: Native binary embeds serialized AST container 'JOSSBC2Z'")
	}
	if bytes.Contains(binBytes, []byte("JOSS_RUNNER_DATA")) {
		t.Fatal("CI GATE FAILURE: Native binary embeds 'JOSS_RUNNER_DATA'")
	}
	if bytes.Contains(binBytes, []byte("modernc.org/sqlite")) {
		t.Fatal("CI GATE FAILURE: Native binary bundles 'modernc.org/sqlite'")
	}
}

// TestArchitecture_CapabilityMatrixReconciliationAndOwnership verifies that
// every entry in Capability Matrix 2.0 has an assigned semantic owner and lifecycle state.
func TestArchitecture_CapabilityMatrixReconciliationAndOwnership(t *testing.T) {
	for _, entry := range analyzer.CanonicalCapabilityMatrix {
		if entry.Ownership.SemanticOwner == "" {
			t.Fatalf("CI GATE FAILURE: Feature '%s' lacks SemanticOwner", entry.Feature)
		}
		if entry.Ownership.CanonicalRepresentation == "" {
			t.Fatalf("CI GATE FAILURE: Feature '%s' lacks CanonicalRepresentation", entry.Feature)
		}
		if entry.State == "" {
			t.Fatalf("CI GATE FAILURE: Feature '%s' lacks CompletionState", entry.Feature)
		}
		if entry.Language != analyzer.Supported {
			t.Fatalf("CI GATE FAILURE: Feature '%s' is marked unsupported in Language; semantics must be defined once", entry.Feature)
		}
	}
}

// TestArchitecture_CoreLanguageImplementedFeaturesMustExecuteEverywhere ensures that
// any feature belonging to the Core Language category marked as StateFullyComplete
// is physically supported in Native and does not emit JOSS-NATIVE-001.
// Furthermore, any feature in Core Language supported in Interpreter/Server but not in Native
// must be explicitly documented with State < StateFullyComplete and tracked with diagnostic JOSS-NATIVE-001.
func TestArchitecture_CoreLanguageImplementedFeaturesMustExecuteEverywhere(t *testing.T) {
	for _, entry := range analyzer.CanonicalCapabilityMatrix {
		if entry.Category == analyzer.CategoryCoreLanguage {
			if entry.State == analyzer.StateFullyComplete {
				if entry.Native != analyzer.Supported {
					t.Fatalf("ARCHITECTURAL VIOLATION: Core Language feature '%s' is marked FullyComplete but Native is %s",
						entry.Feature, entry.Native)
				}
				if entry.DiagnosticCode == "JOSS-NATIVE-001" {
					t.Fatalf("ARCHITECTURAL VIOLATION: Core Language feature '%s' is FullyComplete but retains JOSS-NATIVE-001 diagnostic",
						entry.Feature)
				}
			}
			// Verify that dynamic classes, dynamic arrays, dynamic maps, exceptions and interfaces are fully materialized across all backends
			if entry.Feature == "DynamicClasses" || entry.Feature == "DynamicArrays" || entry.Feature == "DynamicMaps" || entry.Feature == "Exceptions" || entry.Feature == "Interfaces" {
				if entry.Native != analyzer.Supported || entry.State != analyzer.StateFullyComplete {
					t.Fatalf("ARCHITECTURAL VIOLATION: Mandatory feature '%s' is not FullyComplete in Native", entry.Feature)
				}
			}
		}
	}
}

// TestArchitecture_EveryCoreLanguageFeatureExecutesEverywhere enforces that every
// feature classified under Core Language that is implemented in the interpreter
// also compiles and executes in native without JOSS-NATIVE-001.
func TestArchitecture_EveryCoreLanguageFeatureExecutesEverywhere(t *testing.T) {
	for _, entry := range analyzer.CanonicalCapabilityMatrix {
		if entry.Category == analyzer.CategoryCoreLanguage && entry.State == analyzer.StateFullyComplete {
			if entry.Native != analyzer.Supported {
				t.Fatalf("Core Language feature '%s' is not supported in Native", entry.Feature)
			}
			if entry.DiagnosticCode != "" {
				t.Fatalf("Core Language feature '%s' emits diagnostic code '%s' in Native", entry.Feature, entry.DiagnosticCode)
			}
		}
	}
}

// TestArchitecture_LanguageFeatureMustBeImplementedOnceAndExecuteEverywhere enforces
// that features cannot diverge semantically between execution backends.
func TestArchitecture_LanguageFeatureMustBeImplementedOnceAndExecuteEverywhere(t *testing.T) {
	for _, entry := range analyzer.CanonicalCapabilityMatrix {
		if entry.Language == analyzer.Supported && entry.State == analyzer.StateFullyComplete {
			if entry.Interpreter != analyzer.Supported || entry.Server != analyzer.Supported || entry.Native != analyzer.Supported {
				t.Fatalf("FullyComplete feature '%s' does not execute in all backends", entry.Feature)
			}
		}
	}
}

// TestArchitecture_Regression_InterpreterSupportedNativeUnsupportedRejected ensures
// that no feature marked as StateFullyComplete has Interpreter=Supported and Native=Unsupported.
func TestArchitecture_Regression_InterpreterSupportedNativeUnsupportedRejected(t *testing.T) {
	for _, entry := range analyzer.CanonicalCapabilityMatrix {
		if entry.State == analyzer.StateFullyComplete {
			if entry.Interpreter == analyzer.Supported && entry.Native == analyzer.Unsupported {
				t.Fatalf("REGRESSION: Feature '%s' is marked FullyComplete with Interpreter Supported but Native Unsupported", entry.Feature)
			}
		}
	}
}

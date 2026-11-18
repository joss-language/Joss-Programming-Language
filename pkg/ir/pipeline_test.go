package ir_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func preparedForPipeline(t *testing.T, source string) *analyzer.PreparedProgram {
	t.Helper()
	parserInstance := parser.NewParser(parser.NewLexer(source))
	program := parserInstance.ParseProgram()
	if parseErrors := parserInstance.Errors(); len(parseErrors) > 0 {
		t.Fatalf("parse errors: %v", parseErrors)
	}
	return analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "pipeline_test.joss", Program: program}}, analyzer.NewEnvironment())
}

func TestLowerAndVerifyUsesOneValidatedIRPipeline(t *testing.T) {
	prepared := preparedForPipeline(t, `
		var $value = 40 + 2;
		print($value);
	`)
	program, err := ir.LowerAndVerify(prepared, "pipeline_test", nil)
	if err != nil {
		t.Fatalf("LowerAndVerify() error = %v", err)
	}
	if program == nil || program.Functions["main"] == nil {
		t.Fatal("LowerAndVerify() must return verified IR with a main function")
	}
}

func TestLowerAndVerifyAppliesTheCanonicalIROptimizations(t *testing.T) {
	prepared := preparedForPipeline(t, `
		var $value = 40 + 2;
		print($value);
	`)
	program, err := ir.LowerAndVerify(prepared, "optimized_pipeline_test", nil)
	if err != nil {
		t.Fatalf("LowerAndVerify() error = %v", err)
	}

	foundFoldedValue := false
	for _, function := range program.Functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instructions {
				if binary, ok := instruction.(*ir.BinaryInst); ok && binary.Op == ir.OpAdd {
					if left, ok := binary.Left.(*ir.ConstInt); ok && left.Val == 40 {
						if right, ok := binary.Right.(*ir.ConstInt); ok && right.Val == 2 {
							t.Fatal("canonical pipeline left constant expression 40 + 2 unoptimized")
						}
					}
				}
				for _, operand := range instruction.Operands() {
					if folded, ok := operand.(*ir.ConstInt); ok && folded.Val == 42 {
						foundFoldedValue = true
					}
				}
			}
		}
	}
	if !foundFoldedValue {
		t.Fatalf("canonical PreparedProgram-to-IR pipeline did not retain a fold of 40 + 2 to 42:\n%s", program.Dump())
	}
}

func TestLowerAndVerifyReportsPipelineStage(t *testing.T) {
	_, err := ir.LowerAndVerify(nil, "invalid", nil)
	var pipelineErr *ir.PipelineError
	if !errors.As(err, &pipelineErr) || pipelineErr.Stage != "input" {
		t.Fatalf("expected input PipelineError, got %T (%v)", err, err)
	}

	prepared := preparedForPipeline(t, `defer cleanup();`)
	_, err = ir.LowerAndVerify(prepared, "unsupported", nil)
	if !errors.As(err, &pipelineErr) || pipelineErr.Stage != "lowering" {
		t.Fatalf("expected lowering PipelineError, got %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "defer") {
		t.Fatalf("pipeline error should preserve the underlying capability failure, got %v", err)
	}
}

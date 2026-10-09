package ir_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestIR_GoldenArithmetic(t *testing.T) {
	srcBytes, err := os.ReadFile("testdata/arithmetic.joss")
	if err != nil {
		t.Fatalf("failed to read testdata/arithmetic.joss: %v", err)
	}

	expectedBytes, err := os.ReadFile("testdata/arithmetic.ir")
	if err != nil {
		t.Fatalf("failed to read testdata/arithmetic.ir: %v", err)
	}

	p := parser.NewParser(parser.NewLexer(string(srcBytes)))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	units := []analyzer.SourceUnit{{Path: "arithmetic.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("analyzer errors: %v", prep.Diagnostics)
	}

	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("arithmetic")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	verifier := ir.NewVerifier()
	if err := verifier.Verify(irProg); err != nil {
		t.Fatalf("verifier failed: %v", err)
	}

	actualDump := strings.TrimSpace(irProg.Dump())
	expectedDump := strings.TrimSpace(string(expectedBytes))

	// Normalizar retornos de carro (\r\n -> \n)
	actualDump = strings.ReplaceAll(actualDump, "\r\n", "\n")
	expectedDump = strings.ReplaceAll(expectedDump, "\r\n", "\n")

	if actualDump != expectedDump {
		t.Errorf("Golden test mismatch!\n=== EXPECTED ===\n%s\n=== ACTUAL ===\n%s\n", expectedDump, actualDump)
	}
}

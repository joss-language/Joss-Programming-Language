package ir_test

import (
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestLowerer_RejectsUnsupportedClassCapability(t *testing.T) {
	src := `
		public class Persona {
			public string $nombre;
		}
	`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	prep := analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "test.joss", Program: prog}}, analyzer.NewEnvironment())
	lowerer := ir.NewLowerer(prep, nil)
	_, err := lowerer.LowerProgram("test")
	if err == nil {
		t.Fatalf("expected LowerProgram to fail on class statement")
	}

	capErr, ok := err.(*ir.UnsupportedCapabilityError)
	if !ok {
		t.Fatalf("expected error to be *ir.UnsupportedCapabilityError, got: %T (%v)", err, err)
	}

	if capErr.Feature == "" {
		t.Errorf("expected feature description in capability error")
	}
}

func TestLowerer_RejectsUnsupportedArrayLiteral(t *testing.T) {
	src := `
		var $lista = [1, 2, 3];
	`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	prep := analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "test.joss", Program: prog}}, analyzer.NewEnvironment())
	lowerer := ir.NewLowerer(prep, nil)
	_, err := lowerer.LowerProgram("test")
	if err == nil {
		t.Fatalf("expected LowerProgram to fail on array literal")
	}

	capErr, ok := err.(*ir.UnsupportedCapabilityError)
	if !ok {
		t.Fatalf("expected error to be *ir.UnsupportedCapabilityError, got: %T (%v)", err, err)
	}

	if capErr.Feature == "" {
		t.Errorf("expected feature description in capability error")
	}
}

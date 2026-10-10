package ir_test

import (
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestLowerer_SupportsClassesAndArrays(t *testing.T) {
	src := `
		public class Persona {
			public string $nombre = "Joss";
		}
		var $p = new Persona();
		var $lista = [1, 2, 3];
	`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	prep := analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "test.joss", Program: prog}}, analyzer.NewEnvironment())
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("test")
	if err != nil {
		t.Fatalf("expected LowerProgram to succeed on classes and arrays, got: %v", err)
	}
	if irProg == nil {
		t.Fatalf("expected non-nil irProg")
	}
}

func TestLowerer_SupportsTryCatchAndInterfaces(t *testing.T) {
	src := `
		public interface Calculable {
			public func calc(int $x): int;
		}

		public class MyCalc implements Calculable {
			public func calc(int $x): int {
				return $x * 2;
			}
		}

		try {
			var $c = new MyCalc();
			int $r = $c->calc(21);
			echo $r;
		} catch ($e) {
			echo $e;
		}
	`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	prep := analyzer.PrepareProgram([]analyzer.SourceUnit{{Path: "test.joss", Program: prog}}, analyzer.NewEnvironment())
	lowerer := ir.NewLowerer(prep, nil)
	irProg, err := lowerer.LowerProgram("test")
	if err != nil {
		t.Fatalf("expected LowerProgram to succeed on try/catch and interfaces, got: %v", err)
	}
	if irProg == nil {
		t.Fatalf("expected non-nil irProg")
	}
}

func TestLowerer_RejectsUnsupportedDeferCapability(t *testing.T) {
	src := `
		public func cleanup(): void {}
		defer cleanup();
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
		t.Fatalf("expected LowerProgram to fail on defer statement")
	}

	capErr, ok := err.(*ir.UnsupportedCapabilityError)
	if !ok {
		t.Fatalf("expected error to be *ir.UnsupportedCapabilityError, got: %T (%v)", err, err)
	}

	if capErr.Feature == "" {
		t.Errorf("expected feature description in capability error")
	}
}

package ir_test

import (
	"testing"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func BenchmarkIR_LoweringAndVerification(b *testing.B) {
	src := `
		public func fib(int $n): int {
			guard ($n > 1) else {
				return $n;
			}
			return fib($n - 1) + fib($n - 2);
		}

		int $res = fib(10);
		return $res;
	`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "bench.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())

	verifier := ir.NewVerifier()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lowerer := ir.NewLowerer(prep, nil)
		irProg, err := lowerer.LowerProgram("bench_module")
		if err != nil {
			b.Fatalf("lowering failed: %v", err)
		}
		if err := verifier.Verify(irProg); err != nil {
			b.Fatalf("verification failed: %v", err)
		}
	}
}

func BenchmarkIR_TextualDump(b *testing.B) {
	src := `
		public func compute(int $x, int $y): int {
			int $z = ($x * $y) + ($x - $y);
			while ($z > 0) {
				$z = $z - 1;
			}
			return $z;
		}

		int $ans = compute(50, 25);
		return $ans;
	`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	units := []analyzer.SourceUnit{{Path: "bench.joss", Program: prog}}
	prep := analyzer.PrepareProgram(units, analyzer.NewEnvironment())
	lowerer := ir.NewLowerer(prep, nil)
	irProg, _ := lowerer.LowerProgram("bench_module")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = irProg.Dump()
	}
}

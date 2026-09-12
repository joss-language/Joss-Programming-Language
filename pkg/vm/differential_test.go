package vm

import (
	"fmt"
	"testing"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/parser"
)

// differentialFeatures is intentionally narrow: it records only semantics
// currently implemented by both the published interpreter and prototype VM.
var differentialFeatures = []string{"integer arithmetic", "integer comparison", "local assignment", "integer prefix"}

func TestInterpreterAndVMSupportedDifferentialFeatures(t *testing.T) {
	if len(differentialFeatures) == 0 {
		t.Fatal("differential feature inventory is empty")
	}
	tests := []struct {
		name, source string
		want         interface{}
	}{
		{name: "integer arithmetic", source: `$result = 1 + 2 * 3`, want: int64(7)},
		{name: "integer comparison", source: `$result = 3 >= 2`, want: true},
		{name: "integer prefix", source: `$result = -4 + 10`, want: int64(6)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := parser.NewParser(parser.NewLexer(test.source))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}
			report := core.AnalyzeProgram(program)
			if report.HasErrors() {
				t.Fatalf("analyzer rejected supported differential case: %#v", report.Diagnostics)
			}

			interpreter := core.NewRuntime()
			defer interpreter.Free()
			interpreter.Execute(program)
			if got := interpreter.Variables["result"]; got != test.want {
				t.Fatalf("interpreter result = %#v, want %#v", got, test.want)
			}

			compiler := NewCompiler()
			chunk, err := compiler.Compile(program)
			if err != nil {
				t.Fatalf("VM compiler: %v", err)
			}
			machine := NewVM()
			if _, err := machine.Run(chunk); err != nil {
				t.Fatalf("VM: %v", err)
			}
			slot, exists := compiler.slots["result"]
			if !exists {
				t.Fatal("VM compiler did not allocate result")
			}
			if got := vmValue(machine.locals[slot]); got != test.want {
				t.Fatalf("VM result = %#v, interpreter = %#v", got, test.want)
			}
		})
	}
}

func vmValue(value Value) interface{} {
	switch value.Kind {
	case ValNull:
		return nil
	case ValBool:
		return value.Boolean
	case ValInt:
		return value.Integer
	case ValFloat:
		return value.Float
	case ValString:
		return value.Str
	default:
		return fmt.Sprintf("<value-kind-%d>", value.Kind)
	}
}

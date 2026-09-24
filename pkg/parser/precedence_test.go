package parser

import "testing"

func TestCanonicalOperatorPrecedence(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"8 * 5 % 3", "((8 * 5) % 3)"},
		{"8 % 5 * 3", "((8 % 5) * 3)"},
		{"8 / 4 * 2", "((8 / 4) * 2)"},
		{"true || false && false", "(true || (false && false))"},
		{"1 + 2 << 3", "((1 + 2) << 3)"},
		{"1 << 2 + 3", "(1 << (2 + 3))"},
		{"1..2 + 3", "(1 .. (2 + 3))"},
		{"1 + 2..3", "((1 + 2) .. 3)"},
		{"1 < 2 == true", "((1 < 2) == true)"},
		{"null ?? false || true", "(null ?? (false || true))"},
		{"false || true |> boolval", "((false || true) |> boolval)"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			parser := NewParser(NewLexer(test.input))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatalf("parser errors: %v", parser.Errors())
			}
			if got := program.String(); got != test.want {
				t.Fatalf("parsed as %q, want %q", got, test.want)
			}
		})
	}
}

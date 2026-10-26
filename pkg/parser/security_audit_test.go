package parser

import (
	"strings"
	"testing"
)

// PoC JOSS-SEC-VULN-007: Profundidad máxima de recursión en Parser ante expresiones patológicas
func TestSecurityAudit_VULN007_DeeplyNestedExpressionDoesNotOverflowStack(t *testing.T) {
	depth := 2000
	input := "var $x = " + strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)
	l := NewLexer(input)
	p := NewParser(l)
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("VULN-007: expected parser error for nesting depth %d exceeding safe limit, got 0 errors", depth)
	}
}

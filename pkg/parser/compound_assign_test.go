package parser

import (
	"testing"
)

func TestCompoundAssignmentOperators(t *testing.T) {
	tests := []struct {
		input       string
		expectedOp  string
		expectedVal string
	}{
		{"$x += 10", "+", "10"},
		{"$total -= $descuento", "-", "descuento"},
		{"$producto *= 2", "*", "2"},
		{"$valor /= 4", "/", "4"},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		p := NewParser(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("parser errors for %q: %v", tt.input, p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			t.Fatalf("expected ExpressionStatement, got %T", program.Statements[0])
		}

		assign, ok := stmt.Expression.(*AssignExpression)
		if !ok {
			t.Fatalf("expected AssignExpression, got %T", stmt.Expression)
		}

		infix, ok := assign.Value.(*InfixExpression)
		if !ok {
			t.Fatalf("expected InfixExpression inside Assign, got %T", assign.Value)
		}

		if infix.Operator != tt.expectedOp {
			t.Errorf("expected operator %q, got %q", tt.expectedOp, infix.Operator)
		}

		if infix.Right.String() != tt.expectedVal {
			t.Errorf("expected right %q, got %q", tt.expectedVal, infix.Right.String())
		}
	}
}

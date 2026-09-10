package parser

import (
	"testing"
)

func TestParseSpreadInArray(t *testing.T) {
	input := `[1, ...$items, 3]`
	l := NewLexer(input)
	p := NewParser(l)
	expr := p.parseExpression(LOWEST)

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	arr, ok := expr.(*ArrayLiteral)
	if !ok {
		t.Fatalf("expected *ArrayLiteral, got %T", expr)
	}

	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr.Elements))
	}

	spread, ok := arr.Elements[1].(*SpreadExpression)
	if !ok {
		t.Fatalf("expected *SpreadExpression for element 1, got %T", arr.Elements[1])
	}

	ident, ok := spread.Expression.(*Identifier)
	if !ok || ident.Value != "items" {
		t.Fatalf("expected items in spread, got %v", spread.Expression)
	}
}

func TestParseNamedArgumentsAndSpreadInCall(t *testing.T) {
	input := `hacerAlgo(1, nombre: "Ada", ...$rest)`
	l := NewLexer(input)
	p := NewParser(l)
	expr := p.parseExpression(LOWEST)

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	call, ok := expr.(*CallExpression)
	if !ok {
		t.Fatalf("expected *CallExpression, got %T", expr)
	}

	if len(call.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(call.Arguments))
	}

	named, ok := call.Arguments[1].(*NamedArgument)
	if !ok {
		t.Fatalf("expected *NamedArgument for argument 1, got %T", call.Arguments[1])
	}
	if named.Name != "nombre" {
		t.Errorf("expected named argument name 'nombre', got '%s'", named.Name)
	}

	spread, ok := call.Arguments[2].(*SpreadExpression)
	if !ok {
		t.Fatalf("expected *SpreadExpression for argument 2, got %T", call.Arguments[2])
	}
	ident, ok := spread.Expression.(*Identifier)
	if !ok || ident.Value != "rest" {
		t.Fatalf("expected rest in spread, got %v", spread.Expression)
	}
}

func TestParseConstructorPropertyPromotion(t *testing.T) {
	input := `
public class Cliente {
    Init(public string $nombre, private int $edad = 30) {
    }
}
`
	l := NewLexer(input)
	p := NewParser(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	cls, ok := prog.Statements[0].(*ClassStatement)
	if !ok {
		t.Fatalf("expected *ClassStatement, got %T", prog.Statements[0])
	}

	if len(cls.Body.Statements) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(cls.Body.Statements))
	}

	initStmt, ok := cls.Body.Statements[0].(*InitStatement)
	if !ok {
		t.Fatalf("expected *InitStatement, got %T", cls.Body.Statements[0])
	}

	if len(initStmt.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(initStmt.Parameters))
	}

	p1 := initStmt.Parameters[0]
	if p1.Visibility.Literal != "public" {
		t.Errorf("expected visibility 'public', got '%s'", p1.Visibility.Literal)
	}
	if p1.Type.Literal != "string" {
		t.Errorf("expected type 'string', got '%s'", p1.Type.Literal)
	}
	if p1.Name.Value != "nombre" {
		t.Errorf("expected name 'nombre', got '%s'", p1.Name.Value)
	}

	p2 := initStmt.Parameters[1]
	if p2.Visibility.Literal != "private" {
		t.Errorf("expected visibility 'private', got '%s'", p2.Visibility.Literal)
	}
	if p2.DefaultValue == nil {
		t.Errorf("expected default value for $edad")
	}
}

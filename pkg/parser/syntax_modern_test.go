package parser

import (
	"testing"
)

func TestParseModernSyntax(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, program *Program)
	}{
		{
			name:  "first-class await expression",
			input: "let $res = await $task;",
			validate: func(t *testing.T, p *Program) {
				if len(p.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(p.Statements))
				}
				stmt, ok := p.Statements[0].(*LetStatement)
				if !ok {
					t.Fatalf("expected LetStatement, got %T", p.Statements[0])
				}
				call, ok := stmt.Value.(*CallExpression)
				if !ok {
					t.Fatalf("expected CallExpression for await, got %T", stmt.Value)
				}
				if call.Function.String() != "await" {
					t.Fatalf("expected function 'await', got %q", call.Function.String())
				}
				if len(call.Arguments) != 1 {
					t.Fatalf("expected 1 argument to await, got %d", len(call.Arguments))
				}
			},
		},
		{
			name:  "generic class declaration",
			input: "public class Box<T> { public T $item; }",
			validate: func(t *testing.T, p *Program) {
				if len(p.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(p.Statements))
				}
				cs, ok := p.Statements[0].(*ClassStatement)
				if !ok {
					t.Fatalf("expected ClassStatement, got %T", p.Statements[0])
				}
				if cs.Name.Value != "Box" {
					t.Fatalf("expected class name 'Box', got %q", cs.Name.Value)
				}
				if len(cs.TypeParameters) != 1 || cs.TypeParameters[0].Value != "T" {
					t.Fatalf("expected type parameter T, got %v", cs.TypeParameters)
				}
			},
		},
		{
			name:  "generic method declaration",
			input: "public func identity<T>(T $x): T { return $x; }",
			validate: func(t *testing.T, p *Program) {
				if len(p.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(p.Statements))
				}
				ms, ok := p.Statements[0].(*MethodStatement)
				if !ok {
					t.Fatalf("expected MethodStatement, got %T", p.Statements[0])
				}
				if len(ms.TypeParameters) != 1 || ms.TypeParameters[0].Value != "T" {
					t.Fatalf("expected type parameter T, got %v", ms.TypeParameters)
				}
			},
		},
		{
			name:  "record declaration",
			input: "public record Point(int $x, int $y);",
			validate: func(t *testing.T, p *Program) {
				if len(p.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(p.Statements))
				}
				cs, ok := p.Statements[0].(*ClassStatement)
				if !ok {
					t.Fatalf("expected ClassStatement for record, got %T", p.Statements[0])
				}
				if !cs.IsRecord {
					t.Fatalf("expected IsRecord to be true")
				}
				if cs.Name.Value != "Point" {
					t.Fatalf("expected record name 'Point', got %q", cs.Name.Value)
				}
				if len(cs.Body.Statements) != 3 {
					t.Fatalf("expected 3 statements (2 fields + Init) in record body, got %d", len(cs.Body.Statements))
				}
				if _, ok := cs.Body.Statements[0].(*LetStatement); !ok {
					t.Fatalf("expected LetStatement for first field, got %T", cs.Body.Statements[0])
				}
				if _, ok := cs.Body.Statements[2].(*MethodStatement); !ok {
					t.Fatalf("expected MethodStatement for Init, got %T", cs.Body.Statements[2])
				}
			},
		},
		{
			name:  "tuple destructuring statement",
			input: "let ($a, $b) = [10, 20];",
			validate: func(t *testing.T, p *Program) {
				if len(p.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(p.Statements))
				}
				ds, ok := p.Statements[0].(*DestructureStatement)
				if !ok {
					t.Fatalf("expected DestructureStatement, got %T", p.Statements[0])
				}
				if len(ds.Names) != 2 || ds.Names[0].Value != "a" || ds.Names[1].Value != "b" {
					t.Fatalf("expected names [a, b], got %v", ds.Names)
				}
			},
		},
		{
			name:  "generic new expression",
			input: "let $box = new Box<int>(42);",
			validate: func(t *testing.T, p *Program) {
				if len(p.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(p.Statements))
				}
				stmt, ok := p.Statements[0].(*LetStatement)
				if !ok {
					t.Fatalf("expected LetStatement, got %T", p.Statements[0])
				}
				ne, ok := stmt.Value.(*NewExpression)
				if !ok {
					t.Fatalf("expected NewExpression, got %T", stmt.Value)
				}
				if len(ne.TypeArguments) != 1 || ne.TypeArguments[0].Literal != "int" {
					t.Fatalf("expected TypeArguments [int], got %v", ne.TypeArguments)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.input)
			p := NewParser(l)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parser had %d errors: %v", len(p.Errors()), p.Errors())
			}
			tt.validate(t, program)
		})
	}
}

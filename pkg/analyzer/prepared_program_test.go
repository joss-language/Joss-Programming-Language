package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

func TestPrepareProgramProducesFactsAndDiagnostics(t *testing.T) {
	source := `
public func fetch(string $url): string {
    return $url
}
`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	prep := PrepareProgram([]SourceUnit{{Path: "service.joss", Program: program}}, NewEnvironment())
	if prep.HasErrors() {
		t.Fatalf("expected no errors in prepared program, got %#v", prep.Diagnostics)
	}

	returnType, exists := prep.Facts.CallableTypes["fetch"]
	if !exists {
		t.Fatal("expected 'fetch' callable to be indexed in analysis facts")
	}
	if returnType.Kind != typesystem.String {
		t.Fatalf("expected string return type, got %s", returnType.String())
	}

	effects, hasEffects := prep.Facts.Effects["fetch"]
	if hasEffects && len(effects) > 0 {
		t.Fatal("did not expect effects for 'fetch'")
	}
}

func TestPrepareProgramIndexesEffects(t *testing.T) {
	source := `
public func read(string $path): string {
    return $path
}
`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()

	prep := PrepareProgram([]SourceUnit{{Path: "io.joss", Program: program}}, NewEnvironment())
	effects, hasEffects := prep.Facts.Effects["read"]
	if !hasEffects || len(effects) == 0 {
		t.Fatal("expected 'read' to be tagged with effects in facts")
	}
}

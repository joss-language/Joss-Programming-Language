package parser_test

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestParseExternLibraryStatement(t *testing.T) {
	src := `
extern "C" library "user32.dll" {
    public func MessageBoxA(int $hWnd, string $text, string $caption, int $type): int;
    public func GetActiveWindow(): int;
}
`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("unexpected parser errors: %v", p.Errors())
	}

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	externStmt, ok := prog.Statements[0].(*parser.ExternLibraryStatement)
	if !ok {
		t.Fatalf("expected *ExternLibraryStatement, got %T", prog.Statements[0])
	}

	if externStmt.ABI != "C" {
		t.Errorf("expected ABI 'C', got %q", externStmt.ABI)
	}

	if externStmt.LibraryPath != "user32.dll" {
		t.Errorf("expected LibraryPath 'user32.dll', got %q", externStmt.LibraryPath)
	}

	if len(externStmt.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(externStmt.Functions))
	}

	fn0 := externStmt.Functions[0]
	if fn0.Name.Value != "MessageBoxA" {
		t.Errorf("expected fn 0 Name 'MessageBoxA', got %q", fn0.Name.Value)
	}
	if fn0.ReturnType.Literal != "int" {
		t.Errorf("expected fn 0 ReturnType 'int', got %q", fn0.ReturnType.Literal)
	}
	if len(fn0.Parameters) != 4 {
		t.Errorf("expected 4 parameters for MessageBoxA, got %d", len(fn0.Parameters))
	}
}

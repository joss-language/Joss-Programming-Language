package analyzer

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestCollectSymbolOccurrencesAndRename(t *testing.T) {
	src := `
public func process(int $count): int {
    int $total = $count * 2
    return $total
}
`
	p := parser.NewParser(parser.NewLexer(src))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	unit := SourceUnit{Path: "math.joss", Program: program}
	occurrences := CollectSymbolOccurrences([]SourceUnit{unit})

	// Check count occurrences
	countOccs := 0
	for _, occ := range occurrences {
		if occ.Name == "count" {
			countOccs++
		}
	}
	if countOccs != 2 {
		t.Fatalf("expected 2 occurrences of 'count' (parameter + reference), got %d", countOccs)
	}

	// Check rename of 'total' at line 3, column 10
	edits, err := PrepareRename([]SourceUnit{unit}, "math.joss", 3, 10, "subtotal")
	if err != nil {
		t.Fatalf("PrepareRename failed: %v", err)
	}
	fileEdits, ok := edits["math.joss"]
	if !ok || len(fileEdits) != 2 {
		t.Fatalf("expected 2 edits for 'total' in math.joss, got %d", len(fileEdits))
	}
	for _, edit := range fileEdits {
		if edit.NewText != "subtotal" {
			t.Fatalf("expected replacement text 'subtotal', got %s", edit.NewText)
		}
	}
}

package formatter

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestScannerConsumesCanonicalParserSymbols(t *testing.T) {
	for _, definition := range parser.SymbolDefinitions() {
		t.Run(string(definition.Token), func(t *testing.T) {
			token := NewScanner(definition.Literal).NextToken()
			wantKind := TokOperator
			if definition.Kind == parser.SymbolDelimiter {
				wantKind = TokDelimiter
			}
			if token.Text != definition.Literal || token.Kind != wantKind {
				t.Fatalf("scanned %q as (%q, %v), want (%q, %v)", definition.Literal, token.Text, token.Kind, definition.Literal, wantKind)
			}
		})
	}
}

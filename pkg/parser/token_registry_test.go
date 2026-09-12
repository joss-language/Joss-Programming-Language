package parser

import "testing"

func TestSymbolRegistryIsUniqueAndLongestFirst(t *testing.T) {
	definitions := SymbolDefinitions()
	seenLiterals := make(map[string]bool, len(definitions))
	seenTokens := make(map[TokenType]bool, len(definitions))
	previousLength := int(^uint(0) >> 1)

	for _, definition := range definitions {
		if definition.Literal == "" {
			t.Fatal("symbol registry contains an empty literal")
		}
		if seenLiterals[definition.Literal] {
			t.Fatalf("symbol literal %q is registered more than once", definition.Literal)
		}
		if seenTokens[definition.Token] {
			t.Fatalf("symbol token %q is registered more than once", definition.Token)
		}
		if len(definition.Literal) > previousLength {
			t.Fatalf("symbol registry is not longest-first at %q", definition.Literal)
		}
		seenLiterals[definition.Literal] = true
		seenTokens[definition.Token] = true
		previousLength = len(definition.Literal)
	}
}

func TestEveryRegisteredSymbolIsLexedFromCanonicalDefinition(t *testing.T) {
	for _, definition := range SymbolDefinitions() {
		t.Run(string(definition.Token), func(t *testing.T) {
			lexer := NewLexer(definition.Literal)
			token := lexer.NextToken()
			if token.Type != definition.Token || token.Literal != definition.Literal {
				t.Fatalf("lexed %q as (%q, %q), want (%q, %q)", definition.Literal, token.Type, token.Literal, definition.Token, definition.Literal)
			}
			if eof := lexer.NextToken(); eof.Type != EOF {
				t.Fatalf("symbol %q left trailing token %q", definition.Literal, eof.Type)
			}
		})
	}
}

func TestLookupSymbolReturnsRegistryMetadata(t *testing.T) {
	definition, ok := LookupSymbol(NULL_SAFE_ARROW)
	if !ok || definition.Token != NULL_SAFE_ARROW || definition.Kind != SymbolOperator {
		t.Fatalf("LookupSymbol(%q) = %#v, %v", NULL_SAFE_ARROW, definition, ok)
	}
	if _, ok := LookupSymbol("not-a-symbol"); ok {
		t.Fatal("LookupSymbol accepted an unknown literal")
	}
}

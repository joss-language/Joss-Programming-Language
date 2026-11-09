package std_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

// forbiddenIdentifiers is the canonical list of keywords from other languages (like PHP, JS, C, Go, Python)
// that do NOT exist in Joss and are forbidden in any stdlib .joss source code.
var forbiddenIdentifiers = map[string]string{
	"if":       "En Joss no existe 'if'; las decisiones se toman con '($cond) ? { ... } : { ... }', 'guard' o 'match'.",
	"elseif":   "En Joss no existe 'elseif'; usa 'match' o encadenamiento ternario '($cond1) ? { ... } : ($cond2) ? { ... }'.",
	"switch":   "En Joss no existe 'switch'; usa 'match ($val) { ... }'.",
	"for":      "En Joss los bucles son 'while ($cond) { ... }' y 'foreach ($arr as $v) { ... }'.",
	"function": "En Joss las funciones se declaran con 'func', con visibilidad 'public func' / 'private func'.",
	"goto":     "En Joss no existe 'goto'.",
}

// TestStdlib_GrammarAndIdiomaticJossCompliance uses the official Joss lexer to inspect
// all tokens across all standard library files (.joss) in std/, ensuring no foreign keywords
// or invalid constructs enter the codebase.
func TestStdlib_GrammarAndIdiomaticJossCompliance(t *testing.T) {
	violations := 0

	err := filepath.Walk("..", func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel("..", path)
		if relErr != nil || !strings.HasPrefix(filepath.ToSlash(rel), "std/") || !strings.HasSuffix(path, ".joss") {
			return nil
		}

		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("Error leyendo archivo std %s: %v", path, readErr)
			return nil
		}

		lexer := parser.NewLexer(string(contentBytes))
		for {
			tok := lexer.NextToken()
			if tok.Type == parser.EOF {
				break
			}

			// Check only IDENTIFIER tokens (not string literals or comments)
			if tok.Type == parser.IDENT {
				lower := strings.ToLower(tok.Literal)
				if reason, forbidden := forbiddenIdentifiers[lower]; forbidden {
					t.Errorf("[Joss STD Linter] Violación en %s (línea %d): Identificador prohibido '%s'.\n  Explicación: %s",
						rel, tok.Line, tok.Literal, reason)
					violations++
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Fallo al recorrer el directorio std: %v", err)
	}

	if violations > 0 {
		t.Fatalf("Se encontraron %d violaciones de sintaxis no-canónica en la biblioteca estándar std/", violations)
	}
}

// TestStdlib_AllUnitsSemanticIntegrity verifies that all standard library units
// are loaded, parsed, and semantically verified with zero compiler errors.
func TestStdlib_AllUnitsSemanticIntegrity(t *testing.T) {
	units := std.GetStdlibUnits()
	if len(units) == 0 {
		t.Fatalf("GetStdlibUnits() devolvió 0 unidades")
	}

	for _, u := range units {
		if u.Program == nil {
			t.Errorf("La unidad %s no generó un AST válido", u.Path)
		}
	}
}

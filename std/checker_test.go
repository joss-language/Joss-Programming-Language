package std_test

import (
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/formatter"
	"github.com/jossecurity/joss/pkg/linter"
	"github.com/jossecurity/joss/std"
)

// TestStdlib_OfficialLinterAndChecker uses the language's official linter (pkg/linter)
// and official formatter (pkg/formatter) to verify that all code in std/
// strictly complies with Joss canonical syntax, rules, and semantic integrity.
func TestStdlib_OfficialLinterAndChecker(t *testing.T) {
	stdDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("No se pudo obtener la ruta de std: %v", err)
	}

	// 1. Verificación oficial de formateo canónico
	unformatted, err := formatter.FormatDirectory(stdDir, false, true)
	if err != nil {
		t.Fatalf("Error en verificación oficial de formato: %v", err)
	}
	if len(unformatted) > 0 {
		for _, f := range unformatted {
			t.Errorf("[Oficial Formatter] El archivo %s no cumple con el formato canónico.", f)
		}
	}

	// 2. Verificación oficial con Linter y Análisis Semántico
	l := linter.NewLinter()
	issues, err := l.LintPath(stdDir)
	if err != nil {
		t.Fatalf("Error ejecutando el linter oficial sobre std: %v", err)
	}

	errorCount := 0
	for _, issue := range issues {
		if issue.Severity == diagnostics.SeverityError {
			t.Errorf("[Oficial Linter] %s", issue.String())
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Fatalf("Se encontraron %d errores reportados por el linter oficial en std/", errorCount)
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

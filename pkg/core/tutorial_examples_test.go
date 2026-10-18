package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestTutorialExampleProjects(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("tutorial-curso", func(t *testing.T) {
		projectDir := filepath.Join(root, "ejemplos", "tutorial-curso")
		entryPath := filepath.Join(projectDir, "main.joss")
		if _, err := os.Stat(entryPath); err != nil {
			t.Skipf("tutorial-curso project not available: %v", err)
		}

		t.Chdir(projectDir)
		units, parseDiagnostics := semanticanalyzer.LoadProject("main.joss", "app")
		if len(parseDiagnostics) > 0 {
			t.Fatalf("parse issues in tutorial-curso: %v", parseDiagnostics)
		}

		report := AnalyzeSourceUnits(units)
		if report.HasErrors() {
			t.Fatalf("semantic errors in tutorial-curso: %v", report.Diagnostics)
		}

		r := NewRuntime()
		defer r.Free()
		output := captureDocumentationOutput(t, func() {
			r.ExecutePrepared(report.Prepared)
		})

		expectedLines := []string{
			"=== 1. FUNDAMENTOS Y DECIMAL EXACTO ===",
			"Usuario: Ada | Subtotal: 59.97",
			"=== 2. CONTROL DE FLUJO (TERNARIO, MATCH, CICLOS) ===",
			"Categoria: Mayorista | Tipo: Pack pequeño",
			"Suma 1..4: 10",
			"=== 3. COLECCIONES Y UNICODE ===",
			"Items: 4 | Modo: estricto | Chars: 7",
			"=== 4. FUNCIONES, REF Y PIPELINE ===",
			"Visitas: 15 | Pipe: 24 | Total c/IVA: 116",
			"=== 5. EXCEPCIONES TRY / CATCH ===",
			"Division valida: 5",
			"Excepcion capturada: Division por cero no permitida",
			"=== 6. CLASES, INTERFACES, RECORD Y GENERICOS ===",
			"Teclado Mecanico ($85.5) | Coord: 12,34 | Caja: Contenido Seguro",
			"=== 7. CONCURRENCIA ASYNC/AWAIT Y CHANNEL ===",
			"Async 9^2: 81 | Canal: Mensaje por canal OK",
		}
		for _, wantLine := range expectedLines {
			if !strings.Contains(output, wantLine) {
				t.Errorf("tutorial-curso output missing %q; got:\n%s", wantLine, output)
			}
		}
	})

	t.Run("tutorial-consola", func(t *testing.T) {
		projectDir := filepath.Join(root, "ejemplos", "tutorial-consola")
		entryPath := filepath.Join(projectDir, "main.joss")
		if _, err := os.Stat(entryPath); err != nil {
			t.Skipf("tutorial-consola project not available: %v", err)
		}

		t.Chdir(projectDir)
		defer os.Remove(filepath.Join(projectDir, "compras.json"))

		units, parseDiagnostics := semanticanalyzer.LoadProject("main.joss", "app")
		if len(parseDiagnostics) > 0 {
			t.Fatalf("parse issues in tutorial-consola: %v", parseDiagnostics)
		}

		report := AnalyzeSourceUnits(units)
		if report.HasErrors() {
			t.Fatalf("semantic errors in tutorial-consola: %v", report.Diagnostics)
		}

		r := NewRuntime()
		defer r.Free()
		output := captureDocumentationOutput(t, func() {
			r.ExecutePrepared(report.Prepared)
		})

		expectedLines := []string{
			"Articulos: 3",
			"Articulos > 10.00: 2",
			"Total: 77.5",
			"Archivo guardado",
		}
		for _, wantLine := range expectedLines {
			if !strings.Contains(output, wantLine) {
				t.Errorf("tutorial-consola output missing %q; got:\n%s", wantLine, output)
			}
		}

		if _, err := os.Stat(filepath.Join(projectDir, "compras.json")); err != nil {
			t.Errorf("expected compras.json to be created on disk: %v", err)
		}
	})

	t.Run("tutorial-web", func(t *testing.T) {
		projectDir := filepath.Join(root, "ejemplos", "tutorial-web")
		entryPath := filepath.Join(projectDir, "main.joss")
		if _, err := os.Stat(entryPath); err != nil {
			t.Skipf("tutorial-web project not available: %v", err)
		}

		t.Chdir(projectDir)
		units, parseDiagnostics := semanticanalyzer.LoadProject("main.joss", "app")
		if len(parseDiagnostics) > 0 {
			t.Fatalf("parse issues in tutorial-web: %v", parseDiagnostics)
		}

		report := AnalyzeSourceUnits(units)
		if report.HasErrors() {
			t.Fatalf("semantic errors in tutorial-web: %v", report.Diagnostics)
		}

		r := NewRuntime()
		defer r.Free()
		for _, unit := range report.Prepared.Units {
			if unit.Program == nil {
				continue
			}
			for _, stmt := range unit.Program.Statements {
				if classStmt, ok := stmt.(*parser.ClassStatement); ok {
					r.registerClass(classStmt)
				}
			}
		}

		ctrlClass, ok := r.Classes["SaludoController"]
		if !ok || ctrlClass == nil {
			t.Fatalf("SaludoController not registered in tutorial-web")
		}
		meta := r.lookupClassMetadata("SaludoController")
		if meta == nil || meta.Methods["show"] == nil {
			t.Fatalf("SaludoController@show method not found")
		}

		inst := &Instance{Class: ctrlClass, Fields: make(map[string]interface{})}
		res := r.CallMethodEvaluated(meta.Methods["show"].Method, inst, []interface{}{"  Ana <script>alert(1)</script>  "})
		html, ok := res.(string)
		if !ok {
			t.Fatalf("expected HTML string from SaludoController@show, got %T", res)
		}
		if !strings.Contains(html, "¡Hola, Ana &lt;script&gt;alert(1)&lt;/script&gt;!") {
			t.Errorf("expected escaped greeting in rendered HTML, got:\n%s", html)
		}
	})
}

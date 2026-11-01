package std_test

import (
	"testing"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

func TestStdlib_LoadsCanonicalUnits(t *testing.T) {
	units := std.GetStdlibUnits()
	if len(units) == 0 {
		t.Fatalf("Se esperaba al menos una unidad en la biblioteca estándar, obtenido 0")
	}

	foundGranDB := false
	for _, u := range units {
		if u.Path == "<std/database/grandb.joss>" {
			foundGranDB = true
			if u.Program == nil || len(u.Program.Statements) == 0 {
				t.Fatalf("El programa parseado de <std/database/grandb.joss> está vacío")
			}
		}
	}

	if !foundGranDB {
		t.Fatalf("No se encontró la unidad <std/database/grandb.joss> en la biblioteca estándar")
	}
}

func TestMiniClass(t *testing.T) {
	code := `
public class Box {
    public string $name = "default";
    public func setName(string $n): Box {
        $this->name = $n;
        return $this;
    }
}
public func main(): int {
    var $b = new Box();
    $b->setName("test");
    return 0;
}
`
	l := parser.NewLexer(code)
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	report := core.AnalyzeSourceUnits([]semanticanalyzer.SourceUnit{{Path: "test.joss", Program: prog}})
	if report.HasErrors() {
		report.PrintReport()
		t.Fatalf("MiniClass falló con %d errores", report.Count(diagnostics.SeverityError))
	}
}

func TestStdlib_GranDB_SemanticAnalysis(t *testing.T) {
	units := std.GetStdlibUnits()
	testCode := `
public func main(): int {
    var $q = GranDB::table("users")->where("active", 1)->orderBy("id", "desc")->limit(10)
    var $sql = $q->toSql()
    print($sql)
    return 0
}
`
	l := parser.NewLexer(testCode)
	p := parser.NewParser(l)
	mainProg := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Error de parser en mainProg: %v", p.Errors())
	}

	allUnits := append(units, semanticanalyzer.SourceUnit{
		Path:    "main.joss",
		Program: mainProg,
	})

	report := core.AnalyzeSourceUnits(allUnits)
	if report.HasErrors() {
		report.PrintReport()
		t.Fatalf("AnalyzeSourceUnits falló con errores en stdlib: %d errores", report.Count(diagnostics.SeverityError))
	}
}

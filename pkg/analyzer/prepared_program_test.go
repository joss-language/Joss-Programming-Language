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
	if symbol, exists := prep.Facts.Symbols["fetch"]; !exists || symbol.Kind != "function" || symbol.File != "service.joss" || symbol.ID != "function:service.joss:fetch" {
		t.Fatalf("unexpected fetch symbol fact: %#v, exists=%v", symbol, exists)
	}
	method := program.Statements[0].(*parser.MethodStatement)
	returned := method.Body.Statements[0].(*parser.ReturnStatement).ReturnValue
	if inferred := prep.Facts.InferredTypes[returned]; inferred.Kind != typesystem.String {
		t.Fatalf("return expression inferred type = %s, want string", inferred.String())
	}

	effects, hasEffects := prep.Facts.Effects["fetch"]
	if hasEffects && len(effects) > 0 {
		t.Fatal("did not expect effects for 'fetch'")
	}
}

func TestPrepareProgramDoesNotInventEffectsFromCallableNames(t *testing.T) {
	source := `
public func read(string $path): string {
    return $path
}
`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()

	prep := PrepareProgram([]SourceUnit{{Path: "io.joss", Program: program}}, NewEnvironment())
	if effects := prep.Facts.Effects["read"]; len(effects) != 0 {
		t.Fatalf("name-based effect metadata is unsound: %#v", effects)
	}
}

func TestPrepareProgramIndexesQualifiedMethodTypes(t *testing.T) {
	p := parser.NewParser(parser.NewLexer(`
public class Service {
    public func fetch(): int { return 1 }
}
`))
	prep := PrepareProgram([]SourceUnit{{Path: "service.joss", Program: p.ParseProgram()}}, NewEnvironment())
	if got := prep.Facts.CallableTypes["Service::fetch"]; got.Kind != typesystem.Int {
		t.Fatalf("Service::fetch return = %s, want int", got.String())
	}
	if method := prep.Facts.Symbols["Service::fetch"]; method.ID != "method:service.joss:Service::fetch" {
		t.Fatalf("unexpected method symbol fact: %#v", method)
	}
}

func TestPrepareProgramRecordsOnlyStaticallyResolvedCalls(t *testing.T) {
	p := parser.NewParser(parser.NewLexer(`
public func value(): int { return 1 }
int $result = value()
`))
	program := p.ParseProgram()
	prepared := PrepareProgram([]SourceUnit{{Path: "calls.joss", Program: program}}, NewEnvironment())
	assignment := program.Statements[1].(*parser.LetStatement)
	call := assignment.Value.(*parser.CallExpression)
	fact, exists := prepared.Facts.ResolvedCalls[call]
	if !exists || fact.TargetID != "function:calls.joss:value" || fact.ReturnType.Kind != typesystem.Int {
		t.Fatalf("unexpected resolved call fact: %#v, exists=%v", fact, exists)
	}
}

func TestPrepareProgramDetachesInputContainers(t *testing.T) {
	p := parser.NewParser(parser.NewLexer(`public func value(): int { return 1 }`))
	units := []SourceUnit{{Path: "value.joss", Program: p.ParseProgram()}}
	environment := NewEnvironment()
	environment.Builtins["host"] = Callable{Name: "host", Parameters: []Parameter{{Name: "value", Type: typesystem.Type{Kind: typesystem.Int}}}}
	prepared := PrepareProgram(units, environment)

	units[0].Path = "changed.joss"
	host := environment.Builtins["host"]
	host.Parameters[0].Name = "changed"
	environment.Builtins["host"] = host
	if prepared.Units[0].Path != "value.joss" {
		t.Fatalf("prepared units changed through caller slice: %#v", prepared.Units)
	}
	if got := prepared.Environment.Builtins["host"].Parameters[0].Name; got != "value" {
		t.Fatalf("prepared environment changed through caller maps: %q", got)
	}
}

func TestPreparedProgramEntrypoint(t *testing.T) {
	first := parser.NewParser(parser.NewLexer(`echo "first"`)).ParseProgram()
	second := parser.NewParser(parser.NewLexer(`echo "second"`)).ParseProgram()
	prepared := PrepareProgram([]SourceUnit{{Path: "main.joss", Program: first}, {Path: "app/other.joss", Program: second}}, NewEnvironment())
	if prepared.Entrypoint() != first {
		t.Fatal("prepared entrypoint did not preserve the first source unit")
	}
	if (*PreparedProgram)(nil).Entrypoint() != nil {
		t.Fatal("nil prepared program must not expose an entrypoint")
	}
}

func TestPreparedProgramFindsValidatedUnitAcrossPathStyles(t *testing.T) {
	p := parser.NewParser(parser.NewLexer(`int $value = 1`))
	program := p.ParseProgram()
	prepared := PrepareProgram([]SourceUnit{{Path: "app/services/value.joss", Program: program}}, NewEnvironment())
	if got := prepared.Program(`app\services\value.joss`); got != program {
		t.Fatal("expected Program to return the validated source unit")
	}
	if got := prepared.Program("missing.joss"); got != nil {
		t.Fatal("expected missing source unit lookup to return nil")
	}
}

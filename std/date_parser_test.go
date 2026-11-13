package std

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestTimezoneModuleParsesWithoutDiagnostics(t *testing.T) {
	source, err := stdlibFS.ReadFile("date/timezone.joss")
	if err != nil {
		t.Fatal(err)
	}
	parserInstance := parser.NewParser(parser.NewLexer(string(source)))
	if program := parserInstance.ParseProgram(); program == nil {
		t.Fatal("timezone Standard Library module parsed to a nil program")
	}
	if errors := parserInstance.Errors(); len(errors) != 0 {
		t.Fatalf("timezone Standard Library module has parser diagnostics: %v", parserInstance.Diagnostics())
	}
}

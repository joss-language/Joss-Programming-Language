package std

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestAuthModuleParsesWithoutDiagnostics(t *testing.T) {
	source, err := stdlibFS.ReadFile("auth/auth.joss")
	if err != nil {
		t.Fatal(err)
	}
	parserInstance := parser.NewParser(parser.NewLexer(string(source)))
	if program := parserInstance.ParseProgram(); program == nil {
		t.Fatal("Auth Standard Library module parsed to a nil program")
	}
	if errors := parserInstance.Errors(); len(errors) != 0 {
		t.Fatalf("Auth Standard Library module has parser diagnostics: %v", parserInstance.Diagnostics())
	}
}

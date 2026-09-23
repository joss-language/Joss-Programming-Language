package main

import (
	"testing"

	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestPackagedSourceUnitsPrepareMainAndProjectSources(t *testing.T) {
	p := parser.NewParser(parser.NewLexer(`public class Service {}`))
	encoded, err := bytecode.Encode(p.ParseProgram())
	if err != nil {
		t.Fatal(err)
	}
	units, err := packagedSourceUnits(map[string][]byte{
		`main.joss`:                 []byte(`$service = new Service()`),
		`app\services\service.joss`: encoded,
		`assets/ignored.joss`:       []byte(`int $ignored = 1`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[0].Path != "main.joss" || units[1].Path != "app/services/service.joss" {
		t.Fatalf("unexpected packaged units: %#v", units)
	}
}

func TestPackagedSourceUnitsRejectParseErrors(t *testing.T) {
	_, err := packagedSourceUnits(map[string][]byte{"main.joss": []byte(`public func broken(`)})
	if err == nil {
		t.Fatal("expected packaged source parse failure")
	}
}

package pluginpkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestExtractPluginSourceUnits_FromBytecodeArchive(t *testing.T) {
	tmp := t.TempDir()
	pluginDir := filepath.Join(tmp, "plugins", "math_ext", "latest")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	src := `public func plugin_double(int $x): int {
		return $x * 2
	}`
	l := parser.NewLexer(src)
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	bcBytes, err := bytecode.Encode(prog)
	if err != nil {
		t.Fatalf("bytecode.Encode failed: %v", err)
	}

	jpBytes, err := Build(Metadata{
		Name:     "math_ext",
		Version:  "1.0.0",
		Bytecode: "main.jbc",
	}, map[string][]byte{
		"main.jbc": bcBytes,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	jpPath := filepath.Join(pluginDir, "math_ext.jp")
	if err := os.WriteFile(jpPath, jpBytes, 0644); err != nil {
		t.Fatal(err)
	}

	units := ExtractPluginSourceUnits(tmp)
	if len(units) != 1 {
		t.Fatalf("expected 1 extracted plugin unit, got %d", len(units))
	}
	if units[0].Program == nil || len(units[0].Program.Statements) == 0 {
		t.Fatalf("expected non-empty AST program in extracted plugin unit")
	}
}

package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectIncludesServerRoutes(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.joss")
	routesPath := filepath.Join(root, "routes.joss")
	if err := os.WriteFile(mainPath, []byte("public func main(): int { return 1 }"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesPath, []byte("public func broken( {"), 0600); err != nil {
		t.Fatal(err)
	}
	_, problems := LoadProject(mainPath, filepath.Join(root, "app"))
	if len(problems) == 0 || problems[0].File != routesPath {
		t.Fatalf("expected routes.joss parse diagnostic, got %#v", problems)
	}
	if err := os.WriteFile(routesPath, []byte("public func route(): int { return 2 }"), 0600); err != nil {
		t.Fatal(err)
	}
	units, problems := LoadProject(mainPath, filepath.Join(root, "app"))
	if len(problems) != 0 || len(units) != 2 {
		t.Fatalf("expected main and routes units, got units=%d diagnostics=%#v", len(units), problems)
	}
}

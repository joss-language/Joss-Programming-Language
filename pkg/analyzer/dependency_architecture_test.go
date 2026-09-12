package analyzer

import (
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestLanguageLayersDoNotImportCore(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate architecture test")
	}
	repository := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	for _, packageName := range []string{"parser", "typesystem", "diagnostics", "analyzer"} {
		files, err := filepath.Glob(filepath.Join(repository, "pkg", packageName, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			parsed, err := goparser.ParseFile(token.NewFileSet(), file, nil, goparser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, declaration := range parsed.Imports {
				path, err := strconv.Unquote(declaration.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if path == "github.com/jossecurity/joss/pkg/core" || strings.HasPrefix(path, "github.com/jossecurity/joss/pkg/core/") {
					t.Fatalf("negative architecture rule violated: %s imports %s", file, path)
				}
			}
		}
	}
}

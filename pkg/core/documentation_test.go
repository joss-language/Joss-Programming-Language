package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

// Test the exact published snippets rather than independently maintained copies.
func TestDocumentationContracts(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, filepath.Join(root, "README.md"))
	fence := regexp.MustCompile("(?s)<!-- joss-(run|check|error): (.*?) -->\\s*```(?:joss|joss-invalid)\\r?\\n(.*?)```")
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for index, match := range fence.FindAllStringSubmatch(string(data), -1) {
			t.Run(fmt.Sprintf("%s/%d", filepath.Base(path), index+1), func(t *testing.T) {
				p := parser.NewParser(parser.NewLexer(match[3]))
				program := p.ParseProgram()
				issues := p.Diagnostics()
				if len(issues) == 0 {
					issues = AnalyzeProgram(program).Diagnostics
				}
				if match[1] == "error" {
					for _, issue := range issues {
						if issue.Code == match[2] {
							return
						}
					}
					t.Fatalf("expected %s, got %v", match[2], issues)
				}
				for _, issue := range issues {
					if issue.Severity == "error" {
						t.Fatalf("published example: %s", issue.String())
					}
				}
				if match[1] == "check" {
					return
				}
				var lines []string
				if err := json.Unmarshal([]byte(match[2]), &lines); err != nil {
					t.Fatal(err)
				}
				t.Chdir(t.TempDir())
				r := benchmarkRuntimeInstance()
				r.RegisterNativeClasses()
				r.CurrentFile = path
				output := captureDocumentationOutput(t, func() { r.Execute(program) })
				want := strings.Join(lines, "\n")
				if len(lines) > 0 {
					want += "\n"
				}
				if output != want {
					t.Fatalf("output = %q; want %q", output, want)
				}
			})
		}
	}
}

func TestDocumentationNavigationAndPublicMirror(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	docs, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	link := regexp.MustCompile(`\[[^]]+\]\(([^)]+)\)`)
	indexData, err := os.ReadFile(filepath.Join(root, "docs", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append(docs, filepath.Join(root, "README.md")) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range link.FindAllStringSubmatch(string(data), -1) {
			target := strings.TrimSpace(strings.Trim(match[1], "<>"))
			if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") {
				continue
			}
			target = strings.SplitN(strings.SplitN(target, "#", 2)[0], "?", 2)[0]
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)))
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s has broken local link %q", filepath.Base(path), match[1])
			}
		}
		if filepath.Base(path) != "README.md" || filepath.Dir(path) == filepath.Join(root, "docs") {
			if !bytes.Contains(indexData, []byte("("+filepath.Base(path)+")")) && filepath.Base(path) != "README.md" {
				t.Errorf("docs/README.md does not link %s", filepath.Base(path))
			}
		}
	}

	publicDir := filepath.Join(root, "ejemplos", "Joss-Red-JosSecurity", "assets", "docs")
	if _, err := os.Stat(publicDir); err != nil {
		return
	}
	publicDocs, err := filepath.Glob(filepath.Join(publicDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(publicDocs) != len(docs) {
		t.Fatalf("public documentation has %d files; canonical docs has %d", len(publicDocs), len(docs))
	}
	menu, err := os.ReadFile(filepath.Join(root, "ejemplos", "Joss-Red-JosSecurity", "app", "views", "docs", "menu.joss.html"))
	if err != nil {
		t.Fatal(err)
	}
	controller, err := os.ReadFile(filepath.Join(root, "ejemplos", "Joss-Red-JosSecurity", "app", "controllers", "web", "DocsController.joss"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range docs {
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		canonical, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		published, err := os.ReadFile(filepath.Join(publicDir, filepath.Base(path)))
		if err != nil || !bytes.Equal(canonical, published) {
			t.Errorf("public copy differs or is missing: %s", filepath.Base(path))
		}
		if bytes.Count(menu, []byte(`data-page="`+name+`"`)) != 1 {
			t.Errorf("public menu must contain %s exactly once", name)
		}
		if bytes.Count(controller, []byte(`"`+name+`":`)) != 1 {
			t.Errorf("DocsController titles must contain %s exactly once", name)
		}
	}
	if got := len(regexp.MustCompile(`data-page="[A-Z0-9_]+"`).FindAll(menu, -1)); got != len(docs) {
		t.Errorf("public menu has %d page entries; want %d", got, len(docs))
	}
	if got := len(regexp.MustCompile(`(?m)^\s{12}"[A-Z0-9_]+"\s*:`).FindAll(controller, -1)); got != len(docs) {
		t.Errorf("DocsController has %d title entries; want %d", got, len(docs))
	}
}

func captureDocumentationOutput(t *testing.T, run func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	previous := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = previous }()
	run()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(output), "\r\n", "\n")
}

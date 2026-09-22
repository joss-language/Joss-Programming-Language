package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
)

// LoadProject parses an entrypoint and every .joss file below sourceDirs.
// Paths are retained per AST so later diagnostics never lose file identity.
func LoadProject(entrypoint string, sourceDirs ...string) ([]SourceUnit, []diagnostics.Diagnostic) {
	paths := []string{entrypoint}
	seen := map[string]bool{}
	if absolute, err := filepath.Abs(entrypoint); err == nil {
		seen[filepath.Clean(absolute)] = true
	}
	// A server entrypoint executes routes.joss from the same project root.
	// Include it in the analysis surface before execution discovers errors.
	if filepath.Base(entrypoint) == "main.joss" {
		routesPath := filepath.Join(filepath.Dir(entrypoint), "routes.joss")
		if info, err := os.Stat(routesPath); err == nil && !info.IsDir() {
			paths = append(paths, routesPath)
			if absolute, err := filepath.Abs(routesPath); err == nil {
				seen[filepath.Clean(absolute)] = true
			}
		}
	}

	for _, sourceDir := range sourceDirs {
		info, err := os.Stat(sourceDir)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info == nil {
				return nil
			}
			if info.IsDir() {
				if parser.IsIgnoredDirectory(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !parser.IsJossSourceFile(path) {
				return nil
			}
			absolute, err := filepath.Abs(path)
			if err == nil && seen[filepath.Clean(absolute)] {
				return nil
			}
			if err == nil {
				seen[filepath.Clean(absolute)] = true
			}
			paths = append(paths, path)
			return nil
		})
	}

	if len(paths) > 1 {
		sort.Strings(paths[1:])
	}
	units := make([]SourceUnit, 0, len(paths))
	issues := make([]diagnostics.Diagnostic, 0)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, diagnostics.Diagnostic{
				Code: "JOSS-IO-001", Severity: diagnostics.SeverityError,
				File: path, Message: fmt.Sprintf("Cannot read source file: %v", err),
			})
			continue
		}
		p := parser.NewParser(parser.NewLexer(string(data)))
		program := p.ParseProgram()
		if parseIssues := p.Diagnostics(); len(parseIssues) > 0 {
			for _, parseIssue := range parseIssues {
				parseIssue.File = path
				issues = append(issues, parseIssue)
			}
			continue
		}
		units = append(units, SourceUnit{Path: path, Program: program})
	}
	return units, issues
}

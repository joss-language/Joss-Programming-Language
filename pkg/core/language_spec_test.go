package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

type LanguageSpecTestCase struct {
	Name                  string   `json:"name"`
	Source                string   `json:"source"`
	ExpectedDiagnostics   []string `json:"expected_diagnostics"`
	ExpectedRuntimeInt    *int64   `json:"expected_runtime_int,omitempty"`
	ExpectedRuntimeString *string  `json:"expected_runtime_string,omitempty"`
}

func TestLanguageCompatibilitySpecificationSuite(t *testing.T) {
	specDir := filepath.Join("..", "..", "tests", "language")
	if _, err := os.Stat(specDir); os.IsNotExist(err) {
		t.Skipf("spec directory %s not found", specDir)
	}

	err := filepath.Walk(specDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		relPath, _ := filepath.Rel(specDir, path)
		t.Run(relPath, func(t *testing.T) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("failed reading %s: %v", path, readErr)
			}

			var tc LanguageSpecTestCase
			if unmarshalErr := json.Unmarshal(data, &tc); unmarshalErr != nil {
				t.Fatalf("failed unmarshaling %s: %v", path, unmarshalErr)
			}

			// 1. Lex & Parse
			p := parser.NewParser(parser.NewLexer(tc.Source))
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("[%s] parse errors: %v", tc.Name, p.Errors())
			}

			// 2. Semantic Analysis
			report := AnalyzeProgram(program)
			var diagCodes []string
			for _, d := range report.Diagnostics {
				if d.Severity == "error" {
					diagCodes = append(diagCodes, d.Code)
				}
			}

			slices.Sort(diagCodes)
			expectedDiagnostics := append([]string(nil), tc.ExpectedDiagnostics...)
			slices.Sort(expectedDiagnostics)
			if !slices.Equal(diagCodes, expectedDiagnostics) {
				t.Fatalf("[%s] unexpected diagnostics: got %v, want %v", tc.Name, diagCodes, tc.ExpectedDiagnostics)
			}

			// 3. Runtime Execution
			runtime := benchmarkPreparedRuntime(t, tc.Source)
			fn, hasMain := runtime.Functions["main"]
			if hasMain {
				result := runtime.CallMethodEvaluated(fn, nil, nil)
				if tc.ExpectedRuntimeInt != nil {
					if intVal, ok := result.(int64); !ok || intVal != *tc.ExpectedRuntimeInt {
						t.Fatalf("[%s] runtime int mismatch: got %v (%T), want %d", tc.Name, result, result, *tc.ExpectedRuntimeInt)
					}
				}
				if tc.ExpectedRuntimeString != nil {
					if strVal, ok := result.(string); !ok || strVal != *tc.ExpectedRuntimeString {
						t.Fatalf("[%s] runtime string mismatch: got %v (%T), want %q", tc.Name, result, result, *tc.ExpectedRuntimeString)
					}
				}
			}
		})
		return nil
	})

	if err != nil {
		t.Fatalf("error walking spec directory: %v", err)
	}
}

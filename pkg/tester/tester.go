package tester

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/parser"
)

type TestCase struct {
	Name     string
	File     string
	Passed   bool
	Duration time.Duration
	Error    string
}

type TestSuite struct {
	File     string
	Cases    []TestCase
	Duration time.Duration
}

type TestReport struct {
	Suites       []TestSuite
	TotalPassed  int
	TotalFailed  int
	TotalSkipped int
	Duration     time.Duration
}

type Runner struct {
	Filter string
	Env    map[string]string
}

func NewRunner() *Runner {
	return &Runner{
		Env: make(map[string]string),
	}
}

// Run executes tests in a given file or recursively in a directory.
func (r *Runner) Run(targetPath string) (*TestReport, error) {
	testFiles, err := r.discoverTestFiles(targetPath)
	if err != nil {
		return nil, err
	}

	report := &TestReport{}
	start := time.Now()

	for _, file := range testFiles {
		suite, err := r.runTestFile(file)
		if err != nil {
			// File execution error (parse/syntax error)
			suite = TestSuite{
				File: file,
				Cases: []TestCase{
					{
						Name:   "File Setup",
						File:   file,
						Passed: false,
						Error:  err.Error(),
					},
				},
			}
			report.TotalFailed++
		} else {
			for _, tc := range suite.Cases {
				if tc.Passed {
					report.TotalPassed++
				} else {
					report.TotalFailed++
				}
			}
		}
		report.Suites = append(report.Suites, suite)
	}

	report.Duration = time.Since(start)
	return report, nil
}

func (r *Runner) discoverTestFiles(targetPath string) ([]string, error) {
	fi, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		return []string{targetPath}, nil
	}

	var files []string
	err = filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), "_test.joss") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func (r *Runner) runTestFile(filePath string) (TestSuite, error) {
	suiteStart := time.Now()
	suite := TestSuite{File: filePath}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return suite, err
	}

	l := parser.NewLexer(string(data))
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return suite, fmt.Errorf("syntax error in test file: %v", p.Errors()[0])
	}

	rt := core.NewRuntime()
	if r.Env != nil {
		rt.Env = r.Env
	}
	rt.CurrentFile = filePath

	var pendingTests []struct {
		name string
		fn   interface{}
	}

	testFn := func(args []interface{}) interface{} {
		if len(args) < 2 {
			panic("test() requiere nombre y función de prueba")
		}
		name, _ := args[0].(string)
		pendingTests = append(pendingTests, struct {
			name string
			fn   interface{}
		}{name: name, fn: args[1]})
		return nil
	}

	// Register built-in assertion helpers in rt.Variables and mark as HostGlobals
	rt.Variables["test"] = testFn
	rt.Variables["it"] = testFn

	rt.Variables["assert"] = func(args []interface{}) interface{} {
		if len(args) == 0 || !isTruthy(args[0]) {
			msg := "Assertion failed"
			if len(args) > 1 && args[1] != nil {
				msg = fmt.Sprintf("Assertion failed: %v", args[1])
			}
			panic(msg)
		}
		return true
	}

	rt.Variables["assertTrue"] = func(args []interface{}) interface{} {
		if len(args) == 0 || !isTruthy(args[0]) {
			msg := "Expected true, got false"
			if len(args) > 1 && args[1] != nil {
				msg = fmt.Sprintf("%v", args[1])
			}
			panic(msg)
		}
		return true
	}

	rt.Variables["assertFalse"] = func(args []interface{}) interface{} {
		if len(args) == 0 || isTruthy(args[0]) {
			msg := "Expected false, got true"
			if len(args) > 1 && args[1] != nil {
				msg = fmt.Sprintf("%v", args[1])
			}
			panic(msg)
		}
		return true
	}

	rt.Variables["assertEqual"] = func(args []interface{}) interface{} {
		if len(args) < 2 {
			panic("assertEqual requiere 2 argumentos (actual, expected)")
		}
		if !valuesEqual(args[0], args[1]) {
			msg := fmt.Sprintf("Assertion failed: expected %v (%T), got %v (%T)", args[1], args[1], args[0], args[0])
			if len(args) > 2 && args[2] != nil {
				msg += fmt.Sprintf(" - %v", args[2])
			}
			panic(msg)
		}
		return true
	}

	rt.Variables["assertNotEqual"] = func(args []interface{}) interface{} {
		if len(args) < 2 {
			panic("assertNotEqual requiere 2 argumentos (actual, unexpected)")
		}
		if valuesEqual(args[0], args[1]) {
			msg := fmt.Sprintf("Assertion failed: expected value not to equal %v", args[1])
			if len(args) > 2 && args[2] != nil {
				msg += fmt.Sprintf(" - %v", args[2])
			}
			panic(msg)
		}
		return true
	}

	rt.Variables["assertNull"] = func(args []interface{}) interface{} {
		if len(args) == 0 || args[0] != nil {
			msg := fmt.Sprintf("Expected null, got %v", args[0])
			if len(args) > 1 && args[1] != nil {
				msg = fmt.Sprintf("%v", args[1])
			}
			panic(msg)
		}
		return true
	}

	rt.Variables["assertNotNull"] = func(args []interface{}) interface{} {
		if len(args) == 0 || args[0] == nil {
			msg := "Expected not null, got null"
			if len(args) > 1 && args[1] != nil {
				msg = fmt.Sprintf("%v", args[1])
			}
			panic(msg)
		}
		return true
	}

	rt.Variables["assertThrows"] = func(args []interface{}) interface{} {
		if len(args) == 0 {
			panic("assertThrows requiere una closure")
		}
		threw := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					threw = true
				}
			}()
			rt.ApplyFunction(args[0], nil)
		}()
		if !threw {
			panic("Expected exception to be thrown, but execution succeeded")
		}
		return true
	}

	rt.MarkHostGlobals("test", "it", "assert", "assertTrue", "assertFalse", "assertEqual", "assertNotEqual", "assertNull", "assertNotNull", "assertThrows")

	// Execute top level file to register all `test(...)` calls
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("%v", rec)
			}
		}()
		rt.Execute(prog)
	}()
	if err != nil {
		return suite, err
	}

	// Now run each registered test
	for _, tCase := range pendingTests {
		if r.Filter != "" && !strings.Contains(strings.ToLower(tCase.name), strings.ToLower(r.Filter)) {
			continue
		}

		tStart := time.Now()
		tc := TestCase{
			Name: tCase.name,
			File: filePath,
		}

		func() {
			defer func() {
				if rec := recover(); rec != nil {
					tc.Passed = false
					tc.Error = fmt.Sprintf("%v", rec)
				}
			}()
			rt.ApplyFunction(tCase.fn, nil)
			tc.Passed = true
		}()

		tc.Duration = time.Since(tStart)
		suite.Cases = append(suite.Cases, tc)
	}

	suite.Duration = time.Since(suiteStart)
	return suite, nil
}

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int64:
		return val != 0
	case int:
		return val != 0
	case float64:
		return val != 0.0
	case string:
		return val != "" && val != "0"
	default:
		return true
	}
}

func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Numeric comparison tolerance
	switch va := a.(type) {
	case int64:
		if vb, ok := b.(int64); ok {
			return va == vb
		}
		if vb, ok := b.(int); ok {
			return va == int64(vb)
		}
		if vb, ok := b.(float64); ok {
			return float64(va) == vb
		}
	case int:
		if vb, ok := b.(int); ok {
			return va == vb
		}
		if vb, ok := b.(int64); ok {
			return int64(va) == vb
		}
		if vb, ok := b.(float64); ok {
			return float64(va) == vb
		}
	case float64:
		if vb, ok := b.(float64); ok {
			return va == vb
		}
		if vb, ok := b.(int64); ok {
			return va == float64(vb)
		}
		if vb, ok := b.(int); ok {
			return va == float64(vb)
		}
	case string:
		if vb, ok := b.(string); ok {
			return va == vb
		}
	case bool:
		if vb, ok := b.(bool); ok {
			return va == vb
		}
	}
	return reflect.DeepEqual(a, b)
}

func (report *TestReport) PrintSummary() {
	fmt.Println()
	for _, suite := range report.Suites {
		fmt.Printf("PASS/FAIL [%s]\n", filepath.Base(suite.File))
		for _, tc := range suite.Cases {
			if tc.Passed {
				fmt.Printf("  ✓ %s (%v)\n", tc.Name, tc.Duration)
			} else {
				fmt.Printf("  ✗ %s (%v)\n", tc.Name, tc.Duration)
				if tc.Error != "" {
					fmt.Printf("    Error: %s\n", tc.Error)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 40))
	if report.TotalFailed == 0 {
		fmt.Printf("Resultado: PASS (%d tests passed, 0 failed in %v)\n", report.TotalPassed, report.Duration)
	} else {
		fmt.Printf("Resultado: FAIL (%d passed, %d failed in %v)\n", report.TotalPassed, report.TotalFailed, report.Duration)
	}
}

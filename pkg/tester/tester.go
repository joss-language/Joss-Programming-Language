package tester

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
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

// Run executes tests in a given file or recursively in a directory using Joss Native.
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

func nativeTestPrelude(filter string) string {
	filterEsc := strings.ReplaceAll(strings.ToLower(filter), `\`, `\\`)
	filterEsc = strings.ReplaceAll(filterEsc, `"`, `\"`)
	return fmt.Sprintf(`
public func __joss_test_matches(string $name): bool {
	string $f = "%s"
	($f == "") ? { return true }
	return str_contains(strtolower($name), $f)
}

public func test(string $name, mixed $fn): void {
	(!__joss_test_matches($name)) ? { return }
	try {
		$fn()
		print("__JOSS_TEST_PASS__|" . $name)
	} catch ($e) {
		print("__JOSS_TEST_FAIL__|" . $name . "|" . strval($e))
	}
}

public func it(string $name, mixed $fn): void {
	test($name, $fn)
}

public func assert(mixed $cond, mixed $msg = null): bool {
	(!$cond) ? {
		($msg != null) ? { throw "Assertion failed: " . strval($msg) }
		throw "Assertion failed"
	}
	return true
}

public func assertTrue(mixed $cond, mixed $msg = null): bool {
	(!$cond) ? {
		($msg != null) ? { throw strval($msg) }
		throw "Expected true, got false"
	}
	return true
}

public func assertFalse(mixed $cond, mixed $msg = null): bool {
	($cond) ? {
		($msg != null) ? { throw strval($msg) }
		throw "Expected false, got true"
	}
	return true
}

public func assertEqual(mixed $actual, mixed $expected, mixed $msg = null): bool {
	(strval($actual) != strval($expected)) ? {
		string $base = "Assertion failed: expected " . strval($expected) . ", got " . strval($actual)
		($msg != null) ? { throw $base . " - " . strval($msg) }
		throw $base
	}
	return true
}

public func assertNotEqual(mixed $actual, mixed $unexpected, mixed $msg = null): bool {
	(strval($actual) == strval($unexpected)) ? {
		string $base = "Assertion failed: expected value not to equal " . strval($unexpected)
		($msg != null) ? { throw $base . " - " . strval($msg) }
		throw $base
	}
	return true
}

public func assertNull(mixed $val, mixed $msg = null): bool {
	($val != null) ? {
		($msg != null) ? { throw strval($msg) }
		throw "Expected null, got " . strval($val)
	}
	return true
}

public func assertNotNull(mixed $val, mixed $msg = null): bool {
	($val == null) ? {
		($msg != null) ? { throw strval($msg) }
		throw "Expected not null, got null"
	}
	return true
}

public func assertThrows(mixed $fn): bool {
	bool $threw = false
	try {
		$fn()
	} catch ($e) {
		$threw = true
	}
	(!$threw) ? { throw "Expected exception to be thrown, but execution succeeded" }
	return true
}
`, filterEsc)
}

func (r *Runner) runTestFile(filePath string) (TestSuite, error) {
	suiteStart := time.Now()
	suite := TestSuite{File: filePath}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return suite, err
	}

	// Validate user test file syntax first so error messages refer cleanly to user code.
	userParser := parser.NewParser(parser.NewLexer(string(data)))
	_ = userParser.ParseProgram()
	if len(userParser.Errors()) > 0 {
		return suite, fmt.Errorf("syntax error in test file: %v", userParser.Errors()[0])
	}

	combined := nativeTestPrelude(r.Filter) + "\n" + string(data)
	p := parser.NewParser(parser.NewLexer(combined))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return suite, fmt.Errorf("syntax error in test file: %v", p.Errors()[0])
	}

	units := []semanticanalyzer.SourceUnit{{
		Path:    filePath,
		Program: prog,
	}}
	report := core.AnalyzeSourceUnits(units)
	if report.HasErrors() {
		for _, d := range report.Diagnostics {
			if d.Severity == "error" {
				return suite, fmt.Errorf("semantic error in test file: %s", d.String())
			}
		}
	}

	irProg, err := ir.LowerAndVerify(report.Prepared, filepath.Base(filePath), nil)
	if err != nil {
		return suite, fmt.Errorf("native IR lowering failed: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "joss-native-test-*")
	if err != nil {
		return suite, err
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "test_runner")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	if _, err := native.BuildProgram(irProg, native.BuildOptions{
		Target:     native.HostTarget(),
		Backend:    native.BackendLLVM,
		OutputPath: binPath,
	}); err != nil {
		return suite, fmt.Errorf("native test build failed: %w", err)
	}

	cmd := exec.Command(binPath)
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()

	lines := strings.Split(strings.ReplaceAll(outBuf.String(), "\r\n", "\n"), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "__JOSS_TEST_PASS__|") {
			name := strings.TrimPrefix(line, "__JOSS_TEST_PASS__|")
			suite.Cases = append(suite.Cases, TestCase{
				Name:   name,
				File:   filePath,
				Passed: true,
			})
		} else if strings.HasPrefix(line, "__JOSS_TEST_FAIL__|") {
			rest := strings.TrimPrefix(line, "__JOSS_TEST_FAIL__|")
			parts := strings.SplitN(rest, "|", 2)
			name := parts[0]
			errMsg := "assertion failed"
			if len(parts) > 1 {
				errMsg = parts[1]
			}
			suite.Cases = append(suite.Cases, TestCase{
				Name:   name,
				File:   filePath,
				Passed: false,
				Error:  errMsg,
			})
		}
	}

	if runErr != nil && len(suite.Cases) == 0 {
		return suite, fmt.Errorf("native test binary failed: %v\n%s", runErr, strings.TrimSpace(errBuf.String()))
	}

	suite.Duration = time.Since(suiteStart)
	return suite, nil
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

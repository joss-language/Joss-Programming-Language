package mobile

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/version"
)

// DiagnosticItem represents an individual diagnostic for mobile editors and UI.
type DiagnosticItem struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Explanation string `json:"explanation,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ExecutionResult represents the complete outcome of executing a Joss script.
type ExecutionResult struct {
	Success     bool             `json:"success"`
	Stdout      string           `json:"stdout"`
	Stderr      string           `json:"stderr"`
	Error       string           `json:"error,omitempty"`
	TimedOut    bool             `json:"timed_out,omitempty"`
	DurationMs  int64            `json:"duration_ms"`
	Diagnostics []DiagnosticItem `json:"diagnostics,omitempty"`
}

// AnalysisResult represents the static check output without running the code.
type AnalysisResult struct {
	Valid       bool             `json:"valid"`
	Diagnostics []DiagnosticItem `json:"diagnostics"`
}

var execMutex sync.Mutex

// Version returns the current Joss language version.
func Version() string {
	return version.Version
}

// Analyze parses and validates Joss code without executing it.
// Returns a JSON-serialized AnalysisResult.
func Analyze(source string) string {
	res := AnalyzeDirect(source)
	data, _ := json.Marshal(res)
	return string(data)
}

// AnalyzeDirect performs static analysis and returns an AnalysisResult struct.
func AnalyzeDirect(source string) *AnalysisResult {
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()

	diags := p.Diagnostics()
	if len(diags) == 0 {
		report := core.AnalyzeProgram(program)
		if report != nil {
			diags = report.Diagnostics
		}
	}

	items := make([]DiagnosticItem, 0, len(diags))
	hasError := false

	for _, d := range diags {
		if d.Severity == diagnostics.SeverityError {
			hasError = true
		}
		items = append(items, DiagnosticItem{
			Code:        d.Code,
			Severity:    string(d.Severity),
			Message:     d.Message,
			Line:        d.Range.Start.Line,
			Column:      d.Range.Start.Column,
			Explanation: d.Explanation,
			Suggestion:  d.Suggestion,
		})
	}

	return &AnalysisResult{
		Valid:       !hasError,
		Diagnostics: items,
	}
}

// Run executes Joss source code with an optional timeout (in milliseconds).
// Standard output and standard error are captured.
// Returns a JSON-serialized ExecutionResult.
func Run(source string, timeoutMs int) string {
	res := RunDirect(source, timeoutMs)
	data, _ := json.Marshal(res)
	return string(data)
}

// RunDirect executes Joss source code and returns an ExecutionResult struct.
func RunDirect(source string, timeoutMs int) *ExecutionResult {
	if timeoutMs <= 0 {
		timeoutMs = 5000 // default 5 seconds
	}

	start := time.Now()

	// 1. Parsing
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()

	// 2. Syntax errors check
	if len(p.Diagnostics()) > 0 {
		items := make([]DiagnosticItem, 0, len(p.Diagnostics()))
		var errMsgs []string
		for _, d := range p.Diagnostics() {
			if d.Severity == diagnostics.SeverityError {
				errMsgs = append(errMsgs, d.Message)
			}
			items = append(items, DiagnosticItem{
				Code:        d.Code,
				Severity:    string(d.Severity),
				Message:     d.Message,
				Line:        d.Range.Start.Line,
				Column:      d.Range.Start.Column,
				Explanation: d.Explanation,
				Suggestion:  d.Suggestion,
			})
		}
		return &ExecutionResult{
			Success:     false,
			Error:       strings.Join(errMsgs, "\n"),
			DurationMs:  time.Since(start).Milliseconds(),
			Diagnostics: items,
		}
	}

	// 3. Semantic Analysis
	report := core.AnalyzeProgram(program)
	if report != nil && report.HasErrors() {
		items := make([]DiagnosticItem, 0, len(report.Diagnostics))
		var errMsgs []string
		for _, d := range report.Diagnostics {
			if d.Severity == diagnostics.SeverityError {
				errMsgs = append(errMsgs, d.Message)
			}
			items = append(items, DiagnosticItem{
				Code:        d.Code,
				Severity:    string(d.Severity),
				Message:     d.Message,
				Line:        d.Range.Start.Line,
				Column:      d.Range.Start.Column,
				Explanation: d.Explanation,
				Suggestion:  d.Suggestion,
			})
		}
		return &ExecutionResult{
			Success:     false,
			Error:       strings.Join(errMsgs, "\n"),
			DurationMs:  time.Since(start).Milliseconds(),
			Diagnostics: items,
		}
	}

	// 4. Thread-safe execution with output capture and timeout guard
	execMutex.Lock()
	defer execMutex.Unlock()

	stdoutPipeR, stdoutPipeW, errOut := os.Pipe()
	stderrPipeR, stderrPipeW, errErr := os.Pipe()

	if errOut != nil || errErr != nil {
		if stdoutPipeR != nil {
			_ = stdoutPipeR.Close()
			_ = stdoutPipeW.Close()
		}
		if stderrPipeR != nil {
			_ = stderrPipeR.Close()
			_ = stderrPipeW.Close()
		}
		return &ExecutionResult{
			Success:    false,
			Error:      "Failed to allocate IO capture pipes",
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = stdoutPipeW
	os.Stderr = stderrPipeW

	stdoutChan := make(chan string, 1)
	stderrChan := make(chan string, 1)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutPipeR)
		stdoutChan <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrPipeR)
		stderrChan <- buf.String()
	}()

	rt := core.NewRuntime()
	defer rt.Free()

	done := make(chan struct{})
	var panicErr any

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				panicErr = rec
			}
			close(done)
		}()
		rt.Execute(program)
	}()

	timedOut := false
	select {
	case <-done:
		// Completed within time limit
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		timedOut = true
	}

	// Restore original IO streams
	_ = stdoutPipeW.Close()
	_ = stderrPipeW.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	stdout := <-stdoutChan
	stderr := <-stderrChan

	_ = stdoutPipeR.Close()
	_ = stderrPipeR.Close()

	duration := time.Since(start).Milliseconds()

	if timedOut {
		return &ExecutionResult{
			Success:    false,
			Stdout:     normalizeOutput(stdout),
			Stderr:     normalizeOutput(stderr),
			Error:      "Execution timed out",
			TimedOut:   true,
			DurationMs: duration,
		}
	}

	if panicErr != nil {
		return &ExecutionResult{
			Success:    false,
			Stdout:     normalizeOutput(stdout),
			Stderr:     normalizeOutput(stderr),
			Error:      core.FormatPanicAsError(panicErr),
			DurationMs: duration,
		}
	}

	return &ExecutionResult{
		Success:    true,
		Stdout:     normalizeOutput(stdout),
		Stderr:     normalizeOutput(stderr),
		DurationMs: duration,
	}
}

func normalizeOutput(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

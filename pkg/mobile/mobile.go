package mobile

import (
	"bytes"
	"context"
	"encoding/json"
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

	// 4. Thread-safe execution with isolated output buffers and timeout guard
	execMutex.Lock()
	defer execMutex.Unlock()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	core.GetAssetManager().Initialized = true
	rt := core.NewRuntime()
	rt.Out = &stdoutBuf
	rt.ErrOut = &stderrBuf

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	rt.SetExecutionContext(ctx)
	if rt.Env == nil {
		rt.Env = make(map[string]string)
	}
	rt.Env["APP_ENV"] = "mobile"
	rt.Env["APP_KEY"] = "mobile-dev-key"

	done := make(chan struct{})
	var panicErr any

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				panicErr = rec
			}
			rt.Free()
			close(done)
		}()
		rt.Execute(program)
	}()

	timedOut := false
	select {
	case <-done:
		// Completed within time limit
	case <-ctx.Done():
		timedOut = true
		// Cooperatively wait for goroutine termination to avoid returning
		// while runtime is still finalizing or accessing resources.
		select {
		case <-done:
		case <-time.After(250 * time.Millisecond):
		}
	}

	duration := time.Since(start).Milliseconds()

	if timedOut {
		return &ExecutionResult{
			Success:    false,
			Stdout:     normalizeOutput(stdoutBuf.String()),
			Stderr:     normalizeOutput(stderrBuf.String()),
			Error:      "Execution timed out",
			TimedOut:   true,
			DurationMs: duration,
		}
	}

	if panicErr != nil {
		return &ExecutionResult{
			Success:    false,
			Stdout:     normalizeOutput(stdoutBuf.String()),
			Stderr:     normalizeOutput(stderrBuf.String()),
			Error:      core.FormatPanicAsError(panicErr),
			DurationMs: duration,
		}
	}

	return &ExecutionResult{
		Success:    true,
		Stdout:     normalizeOutput(stdoutBuf.String()),
		Stderr:     normalizeOutput(stderrBuf.String()),
		DurationMs: duration,
	}
}

func normalizeOutput(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

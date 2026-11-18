package ir

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/analyzer"
)

// PipelineError identifies the stage that rejected a PreparedProgram while
// producing backend-ready IR.
type PipelineError struct {
	Stage string
	Err   error
}

func (e *PipelineError) Error() string {
	return fmt.Sprintf("native pipeline %s: %v", e.Stage, e.Err)
}

func (e *PipelineError) Unwrap() error { return e.Err }

// LowerAndVerify is the canonical PreparedProgram-to-IR pipeline used by
// execution modes and native builds. It applies the same lowering and verifier
// before a caller chooses an execution sink (memory or executable file).
func LowerAndVerify(prepared *analyzer.PreparedProgram, name string, reachability *analyzer.ReachabilityGraph) (*Program, error) {
	if prepared == nil {
		return nil, &PipelineError{Stage: "input", Err: fmt.Errorf("PreparedProgram es nil")}
	}

	lowerer := NewLowerer(prepared, reachability)
	program, err := lowerer.LowerProgram(name)
	if err != nil {
		return nil, &PipelineError{Stage: "lowering", Err: err}
	}

	if err := NewVerifier().Verify(program); err != nil {
		return nil, &PipelineError{Stage: "verification", Err: err}
	}
	return program, nil
}

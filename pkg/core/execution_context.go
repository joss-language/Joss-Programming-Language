package core

import "context"

// SetExecutionContext applies cooperative cancellation to AST execution.
// Native blocking operations still need their own context-aware adapters.
func (r *Runtime) SetExecutionContext(ctx context.Context) {
	r.executionContext = ctx
}

func (r *Runtime) checkExecutionCancelled() {
	if r.executionContext == nil {
		return
	}
	select {
	case <-r.executionContext.Done():
		panic(&JossError{Type: "ExecutionCancelled", Message: r.executionContext.Err().Error(), File: r.CurrentFile})
	default:
	}
}

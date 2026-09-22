package core

import (
	"context"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestExecutionContextCancelsEmptyInfiniteLoop(t *testing.T) {
	p := parser.NewParser(parser.NewLexer("while (true) {}"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse: %v", p.Errors())
	}
	r := newRuntimeState().(*Runtime)
	defer r.Free()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	r.SetExecutionContext(ctx)
	done := make(chan interface{}, 1)
	go func() {
		defer func() { done <- recover() }()
		r.Execute(program)
	}()
	select {
	case recovered := <-done:
		if err, ok := recovered.(*JossError); !ok || err.Type != "ExecutionCancelled" {
			t.Fatalf("expected execution cancellation, got %#v", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("interpreter did not stop after context cancellation")
	}
}

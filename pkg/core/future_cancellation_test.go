package core

import (
	"context"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestFutureCancelPropagatesToForkedRuntime(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime.SetExecutionContext(ctx)
	p := parser.NewParser(parser.NewLexer(`async { while (true) {} }`))
	parsed := p.ParseProgram()
	if errors := p.Errors(); len(errors) != 0 {
		t.Fatalf("parse errors: %v", errors)
	}
	statement := parsed.Statements[0].(*parser.ExpressionStatement)
	call := statement.Expression.(*parser.CallExpression)
	value, handled := runtime.callBuiltinAsync("async", []interface{}{call.Arguments[0]})
	if !handled {
		t.Fatal("async builtin was not handled")
	}
	task := value.(*Future)
	task.Cancel()
	select {
	case <-task.done:
	case <-time.After(time.Second):
		t.Fatal("cancelled future did not finish")
	}
}

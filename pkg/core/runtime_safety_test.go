package core

import (
	"testing"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
)

func safetyExpression(t *testing.T, source string) interface{} {
	t.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if parseErrors := p.Errors(); len(parseErrors) > 0 {
		t.Fatalf("parse errors: %v", parseErrors)
	}
	statement, ok := program.Statements[0].(*parser.ExpressionStatement)
	if !ok {
		t.Fatalf("source produced %T", program.Statements[0])
	}
	runtime := benchmarkRuntimeInstance()
	return runtime.evaluateExpression(statement.Expression)
}

func TestRuntimeStringIndexUsesUnicodeGraphemeClusters(t *testing.T) {
	tests := map[string]string{
		`"Joss"[1]`:   "o",
		`"México"[1]`: "é",
		`"école"[0]`: "é",
		`"语言"[1]`:     "言",
		`"🙂!"[0]`:     "🙂",
		`"👩‍💻!"[0]`:   "👩‍💻",
		`"🇲🇽!"[0]`:    "🇲🇽",
	}
	for source, want := range tests {
		if got := safetyExpression(t, source); got != want {
			t.Errorf("%s = %q, want %q", source, got, want)
		}
	}
}

func TestRuntimeStringIndexOutOfRangeIsStructured(t *testing.T) {
	defer func() {
		recovered := recover()
		err, ok := recovered.(*JossError)
		if !ok || err.Type != "IndexError" || err.Code != diagnostics.CodeIndexOutOfRange {
			t.Fatalf("expected structured IndexError, got %#v", recovered)
		}
	}()
	safetyExpression(t, `"é"[1]`)
}

func TestRuntimeIntegerArithmeticKeepsExactPrecision(t *testing.T) {
	if got := safetyExpression(t, `9007199254740993 + 1`); got != int64(9007199254740994) {
		t.Fatalf("large integer result = %#v", got)
	}
}

func TestRuntimeIntegerOverflowIsStructured(t *testing.T) {
	defer func() {
		recovered := recover()
		err, ok := recovered.(*JossError)
		if !ok || err.Type != "ArithmeticError" || err.Code != diagnostics.CodeArithmeticOverflow {
			t.Fatalf("expected structured ArithmeticError, got %#v", recovered)
		}
	}()
	safetyExpression(t, `9223372036854775807 + 1`)
}

func TestRuntimeDivisionByZeroIsStructured(t *testing.T) {
	defer func() {
		recovered := recover()
		err, ok := recovered.(*JossError)
		if !ok || err.Type != "ArithmeticError" || err.Code != diagnostics.CodeDivisionByZero {
			t.Fatalf("expected structured division error, got %#v", recovered)
		}
	}()
	safetyExpression(t, `1 / 0`)
}

func TestRuntimeTypedCollectionValidation(t *testing.T) {
	runtime := benchmarkPreparedRuntime(t, `
public func takeInts(array<int> $items): int {
    return $items[0]
}

`)
	fn := runtime.Functions["takeInts"]
	valid := runtime.CallMethodEvaluated(fn, nil, []interface{}{[]interface{}{int64(10), int64(20)}})
	if valid != int64(10) {
		t.Fatalf("takeInts valid call = %#v, want 10", valid)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic on invalid collection element type")
		}
	}()
	runtime.CallMethodEvaluated(fn, nil, []interface{}{[]interface{}{"not an int"}})
}

func TestChannelOperationsReturnStructuredErrors(t *testing.T) {
	ch := &Channel{Ch: make(chan interface{}, 1)}
	ch.Send("ok")
	if got := <-ch.Ch; got != "ok" {
		t.Fatalf("received %#v", got)
	}
	ch.Close()
	for _, run := range []func(){func() { ch.Close() }, func() { ch.Send("late") }} {
		func() {
			defer func() {
				got, ok := recover().(*JossError)
				if !ok || got.Code != diagnostics.CodeChannelClosed {
					t.Fatalf("expected structured closed-channel error, got %#v", got)
				}
			}()
			run()
		}()
	}
}

func TestNegativeChannelCapacityIsStructured(t *testing.T) {
	r := benchmarkRuntimeInstance()
	defer func() {
		got, ok := recover().(*JossError)
		if !ok || got.Code != diagnostics.CodeChannelCapacity {
			t.Fatalf("expected capacity error, got %#v", got)
		}
	}()
	r.callBuiltinAsync("make_chan", []interface{}{int64(-1)})
}

func TestAsyncForkReleasesBorrowedDriverBeforeFutureCompletes(t *testing.T) {
	r := newRuntimeState().(*Runtime)
	defer r.Free()
	driver := &NativeDriverDefinition{Name: "borrowed"}
	if err := r.installNativeDriver("borrowed", driver); err != nil {
		t.Fatal(err)
	}
	value, ok := r.callBuiltinAsync("async", []interface{}{int64(7)})
	if !ok || value.(*Future).Wait() != int64(7) {
		t.Fatalf("unexpected async result: %#v", value)
	}
	driver.Mu.Lock()
	owners := driver.owners
	driver.Mu.Unlock()
	if owners != 1 {
		t.Fatalf("async fork left %d driver owners, want parent only", owners)
	}
}

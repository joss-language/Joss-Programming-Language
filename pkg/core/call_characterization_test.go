package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestCallBindingCharacterization(t *testing.T) {
	source := `
public func describe(string $first, string $second = "B", string $third = "C"): string {
    return $first . $second . $third
}
`

	tests := []struct {
		name      string
		arguments []interface{}
		want      interface{}
		wantPanic string
	}{
		{
			name: "named arguments preserve defaults",
			arguments: []interface{}{
				&NamedValue{Name: "third", Value: "3"},
				&NamedValue{Name: "first", Value: "1"},
			},
			want: "1B3",
		},
		{
			name: "positional arguments take their declared positions",
			arguments: []interface{}{
				"1",
				&NamedValue{Name: "third", Value: "3"},
			},
			want: "1B3",
		},
		{
			name: "missing required named argument",
			arguments: []interface{}{
				&NamedValue{Name: "third", Value: "3"},
			},
			wantPanic: "ArgumentError: Falta el argumento requerido '$first' en describe()",
		},
		{
			name: "unknown named argument",
			arguments: []interface{}{
				&NamedValue{Name: "first", Value: "1"},
				&NamedValue{Name: "unknown", Value: "x"},
			},
			wantPanic: "ArgumentError: Parámetro desconocido 'unknown' en describe()",
		},
		{
			name: "duplicate named argument",
			arguments: []interface{}{
				&NamedValue{Name: "first", Value: "1"},
				&NamedValue{Name: "first", Value: "2"},
			},
			wantPanic: "proporcionado más de una vez",
		},
		{
			name: "positional and named duplicate",
			arguments: []interface{}{
				"1",
				&NamedValue{Name: "first", Value: "2"},
			},
			wantPanic: "proporcionado por posición y nombre",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := benchmarkPreparedRuntime(t, source)
			method := runtime.Functions["describe"]
			if test.wantPanic != "" {
				defer func() {
					panicValue := recover()
					if panicValue == nil {
						t.Fatalf("expected panic containing %q", test.wantPanic)
					}
					if got := fmt.Sprint(panicValue); !strings.Contains(got, test.wantPanic) {
						t.Fatalf("panic = %q, want substring %q", got, test.wantPanic)
					}
				}()
				runtime.CallMethodEvaluated(method, nil, test.arguments)
				return
			}

			if got := runtime.CallMethodEvaluated(method, nil, test.arguments); got != test.want {
				t.Fatalf("result = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestCallReturnContractCharacterization(t *testing.T) {
	runtime := benchmarkPreparedRuntime(t, `
public func invalidReturn(): int {
    return "not-an-int"
}
`)

	defer func() {
		panicValue := recover()
		if panicValue == nil {
			t.Fatal("expected ReturnTypeError")
		}
		runtimeError, ok := panicValue.(*JossError)
		if !ok {
			t.Fatalf("panic type = %T, want *JossError", panicValue)
		}
		if runtimeError.Type != "ReturnTypeError" {
			t.Fatalf("error type = %q, want ReturnTypeError", runtimeError.Type)
		}
	}()
	runtime.CallMethodEvaluated(runtime.Functions["invalidReturn"], nil, nil)
}

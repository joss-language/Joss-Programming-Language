package core

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestSourceStaticMethodOverridesNativeClassHandler(t *testing.T) {
	source := `
public class Session {
    public static map $cache = {}
    public static func marker(): string { return "joss"; }
    public static func put(string $key, string $value): void { Session::$cache[$key] = $value; }
    public static func read(string $key): mixed { return Session::$cache[$key]; }
    public func get(string $key): mixed { return "source-instance"; }
}
print(Session::marker());
print(Session::get("legacy"));
Session::put("shared", "shared-value");
`
	program := parser.NewParser(parser.NewLexer(source)).ParseProgram()
	runtime := NewRuntime()
	defer runtime.Free()
	var output bytes.Buffer
	runtime.Out = &output
	runtime.Variables["$__session"] = map[string]interface{}{"legacy": "native-fallback"}
	runtime.Execute(program)

	if got, want := output.String(), "joss\nnative-fallback\n"; got != want {
		t.Fatalf("dispatch output = %q, want %q", got, want)
	}
	fork := runtime.Fork()
	defer fork.Free()
	fork.Execute(parser.NewParser(parser.NewLexer(`print(Session::read("shared"));`)).ParseProgram())
	if got, want := output.String(), "joss\nnative-fallback\nshared-value\n"; got != want {
		t.Fatalf("static property value across request fork = %q, want %q", got, want)
	}
	if strings.Contains(output.String(), "source-instance") {
		t.Fatal("a non-static Joss method replaced the legacy static Session contract")
	}
}

package core

import (
	"testing"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
)

var (
	benchmarkValue   interface{}
	benchmarkProgram *parser.Program
)

const benchmarkStartupSource = `
public class Counter {
    public int $value = 0
    public func add(int $amount): int {
        $this->value = $this->value + $amount
        return $this->value
    }
}
public func sum(int $limit): int {
    int $total = 0
    int $i = 0
    while ($i < $limit) {
        $total = $total + $i
        $i++
    }
    return $total
}
int $seed = 10
`

func benchmarkParse(tb testing.TB, source string) *parser.Program {
	tb.Helper()
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	if errors := p.Errors(); len(errors) > 0 {
		tb.Fatalf("benchmark source does not parse: %v", errors)
	}
	return program
}

func benchmarkRuntimeInstance() *Runtime {
	r := &Runtime{
		Env:               map[string]string{"BENCHMARK": "1"},
		Variables:         make(map[string]interface{}),
		VarTypes:          make(map[string]string),
		Constants:         make(map[string]bool),
		HostGlobals:       make(map[string]bool),
		Classes:           make(map[string]*parser.ClassStatement),
		Functions:         make(map[string]*parser.MethodStatement),
		Routes:            make(map[string]map[string]interface{}),
		CustomMiddlewares: make(map[string]interface{}),
		NativeHandlers:    make(map[string]NativeHandler),
		MaxCallDepth:      DefaultMaxCallDepth,
	}
	r.Variables["cout"] = &Cout{}
	r.Variables["cin"] = &Cin{}
	r.Variables["cerr"] = &Cerr{}
	r.Variables["endl"] = "\n"
	r.Constants["endl"] = true
	r.markCurrentVariablesAsHostGlobals()
	return r
}

func benchmarkExpression(tb testing.TB, source string) parser.Expression {
	tb.Helper()
	program := benchmarkParse(tb, source)
	if len(program.Statements) != 1 {
		tb.Fatalf("benchmark expression produced %d statements", len(program.Statements))
	}
	statement, ok := program.Statements[0].(*parser.ExpressionStatement)
	if !ok {
		tb.Fatalf("benchmark expression produced %T", program.Statements[0])
	}
	return statement.Expression
}

func benchmarkPreparedRuntime(tb testing.TB, source string) *Runtime {
	tb.Helper()
	runtime := benchmarkRuntimeInstance()
	runtime.Execute(benchmarkParse(tb, source))
	return runtime
}

func BenchmarkJossStartup(b *testing.B) {
	b.Run("Parse", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkProgram = benchmarkParse(b, benchmarkStartupSource)
		}
	})

	program := benchmarkParse(b, benchmarkStartupSource)
	b.Run("Analyze", func(b *testing.B) {
		environment := semanticanalyzer.NewEnvironment()
		units := []semanticanalyzer.SourceUnit{{Path: "benchmark.joss", Program: program}}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkValue = semanticanalyzer.Analyze(units, environment)
		}
	})

	payload, err := bytecode.Encode(program)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("LoadSerializedAST", func(b *testing.B) {
		b.SetBytes(int64(len(payload)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			decoded, decodeErr := bytecode.Decode(payload)
			if decodeErr != nil {
				b.Fatal(decodeErr)
			}
			benchmarkProgram = decoded
		}
	})

	b.Run("FirstExecution", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			runtime := benchmarkRuntimeInstance()
			runtime.Execute(program)
			benchmarkValue = runtime.Variables["seed"]
		}
	})

	b.Run("RepeatedExecution", func(b *testing.B) {
		runtime := benchmarkPreparedRuntime(b, `public func work(int $value): int { return $value + 1 }`)
		function := runtime.Functions["work"]
		arguments := []interface{}{int64(41)}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkValue = runtime.CallMethodEvaluated(function, nil, arguments)
		}
	})
}

func BenchmarkJossBasicOperations(b *testing.B) {
	runtime := benchmarkRuntimeInstance()
	runtime.Variables["x"] = int64(41)
	runtime.VarTypes["x"] = "int"

	benchExpression := func(name, source string) {
		b.Run(name, func(b *testing.B) {
			expression := benchmarkExpression(b, source)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkValue = runtime.evaluateExpression(expression)
			}
		})
	}

	benchExpression("Assignment", `$x = 42`)
	benchExpression("VariableLocalMap", `$x`)
	benchExpression("ArithmeticInt", `$x + 1`)
	benchExpression("Comparison", `$x >= 40`)
	benchExpression("BooleanShortCircuit", `true && ($x > 0)`)
	benchExpression("StringConversion", `"42"`)
	benchExpression("ArrayRead", `[1, 2, 3, 4][2]`)
	benchExpression("MapRead", `{"id": 7, "name": "joss"}["id"]`)
	benchExpression("StringIndexASCII", `"joss"[2]`)
	benchExpression("StringIndexUnicode", `"México"[1]`)

	b.Run("VariableGlobalMap", func(b *testing.B) {
		runtime.HostGlobals["x"] = true
		expression := benchmarkExpression(b, `$x`)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkValue = runtime.evaluateExpression(expression)
		}
	})

	b.Run("TypedStringToInt", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkValue = runtime.coerceToTypedValue("123456", "int")
		}
	})
}

func BenchmarkJossControlFlow(b *testing.B) {
	cases := []struct {
		name   string
		source string
		call   string
		args   []interface{}
	}{
		{"LoopSmall", `public func loop(int $limit): int { int $i = 0 while ($i < $limit) { $i++ } return $i }`, "loop", []interface{}{int64(10)}},
		{"LoopLarge", `public func loop(int $limit): int { int $i = 0 while ($i < $limit) { $i++ } return $i }`, "loop", []interface{}{int64(10000)}},
		{"Match", `public func choose(int $value): string { return match ($value) { 1 => "one", 2 => "two", default => "other" } }`, "choose", []interface{}{int64(2)}},
		{"TernaryBlock", `public func choose(bool $value): int { return $value ? { return 1 } : { return 2 } }`, "choose", []interface{}{true}},
		{"Recursion", `public func factorial(int $n): int { return $n <= 1 ? 1 : $n * factorial($n - 1) }`, "factorial", []interface{}{int64(10)}},
	}
	for _, item := range cases {
		b.Run(item.name, func(b *testing.B) {
			runtime := benchmarkPreparedRuntime(b, item.source)
			function := runtime.Functions[item.call]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkValue = runtime.CallMethodEvaluated(function, nil, item.args)
			}
		})
	}
}

func BenchmarkJossFunctions(b *testing.B) {
	cases := []struct {
		name   string
		source string
		call   string
		args   []interface{}
	}{
		{"Simple", `public func identity(int $value): int { return $value }`, "identity", []interface{}{int64(7)}},
		{"Nested", `public func inner(int $value): int { return $value + 1 } public func outer(int $value): int { return inner(inner($value)) }`, "outer", []interface{}{int64(7)}},
		{"Ref", `public func increment(ref int $value): int { $value = $value + 1 return $value }`, "increment", nil},
	}
	for _, item := range cases {
		b.Run(item.name, func(b *testing.B) {
			runtime := benchmarkPreparedRuntime(b, item.source)
			function := runtime.Functions[item.call]
			args := item.args
			if item.name == "Ref" {
				runtime.Variables["value"] = int64(1)
				runtime.VarTypes["value"] = "int"
				args = []interface{}{runtime.referenceTo("value")}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkValue = runtime.CallMethodEvaluated(function, nil, args)
			}
		})
	}

	b.Run("Closure", func(b *testing.B) {
		runtime := benchmarkRuntimeInstance()
		assignment := benchmarkExpression(b, `$closure = func(int $value): int { return $value + 1 }`).(*parser.AssignExpression)
		closure := assignment.Value
		args := []interface{}{int64(7)}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkValue = runtime.applyFunction(closure, args)
		}
	})
}

func BenchmarkJossObjects(b *testing.B) {
	source := `
public class BaseCounter {
    public int $value = 1
    public func get(): int { return $this->value }
}
public class Counter extends BaseCounter {
    public string $name = "counter"
    public func increment(): int { $this->value = $this->value + 1 return $this->value }
    public static func twice(int $value): int { return $value * 2 }
}
`
	runtime := benchmarkPreparedRuntime(b, source)
	newExpression := benchmarkExpression(b, `new Counter()`)
	instance := runtime.evaluateExpression(newExpression).(*Instance)
	runtime.Variables["counter"] = instance
	runtime.VarTypes["counter"] = "Counter"

	benchExpression := func(name, source string) {
		b.Run(name, func(b *testing.B) {
			expression := benchmarkExpression(b, source)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkValue = runtime.evaluateExpression(expression)
			}
		})
	}

	benchExpression("Create", `new Counter()`)
	benchExpression("PropertyRead", `$counter->name`)
	benchExpression("PropertyWrite", `$counter->name = "updated"`)
	benchExpression("InheritedPropertyRead", `$counter->value`)
	benchExpression("MethodCall", `$counter->get()`)
	benchExpression("InheritedMethodResolution", `$counter->increment()`)
	benchExpression("StaticMethod", `Counter::twice(21)`)
}

func BenchmarkJossCollections(b *testing.B) {
	runtime := benchmarkRuntimeInstance()
	runtime.Variables["items"] = []interface{}{int64(1), int64(2), int64(3), int64(4)}
	runtime.VarTypes["items"] = "array"
	runtime.Variables["record"] = map[string]interface{}{"id": int64(7), "name": "joss"}
	runtime.VarTypes["record"] = "map"

	benchExpression := func(name, source string) {
		b.Run(name, func(b *testing.B) {
			expression := benchmarkExpression(b, source)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if name == "ArrayGrowth" {
					runtime.Variables["items"] = []interface{}{int64(1), int64(2), int64(3), int64(4)}
				}
				benchmarkValue = runtime.evaluateExpression(expression)
			}
		})
	}

	benchExpression("ArrayConstruct", `[1, 2, 3, 4, 5, 6, 7, 8]`)
	benchExpression("ArrayRead", `$items[2]`)
	benchExpression("ArrayWrite", `$items[2] = 9`)
	benchExpression("ArrayGrowth", `$items[] = 9`)
	benchExpression("MapConstruct", `{"id": 7, "name": "joss", "active": true}`)
	benchExpression("MapRead", `$record["name"]`)
	benchExpression("MapWrite", `$record["name"] = "runtime"`)

	b.Run("Iteration", func(b *testing.B) {
		program := benchmarkParse(b, `foreach ($items as $item) { $last = $item }`)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runtime.Execute(program)
		}
		benchmarkValue = runtime.Variables["last"]
	})
}

func BenchmarkJossRuntimeFeatures(b *testing.B) {
	b.Run("Exception", func(b *testing.B) {
		runtime := benchmarkPreparedRuntime(b, `public func fail() { throw "boom" }`)
		function := runtime.Functions["fail"]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			func() {
				defer func() { _ = recover() }()
				runtime.CallMethodEvaluated(function, nil, nil)
			}()
		}
	})

	b.Run("AsyncAwait", func(b *testing.B) {
		runtime := benchmarkRuntimeInstance()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			future, _ := runtime.callBuiltinAsync("async", []interface{}{int64(42)})
			benchmarkValue, _ = runtime.callBuiltinAsync("await", []interface{}{future})
		}
	})

	b.Run("ChannelRoundTrip", func(b *testing.B) {
		runtime := benchmarkRuntimeInstance()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			channel, _ := runtime.callBuiltinAsync("make_chan", []interface{}{int64(1)})
			_, _ = runtime.callBuiltinAsync("send", []interface{}{channel, int64(42)})
			benchmarkValue, _ = runtime.callBuiltinAsync("recv", []interface{}{channel})
		}
	})
}

func BenchmarkJossApplicationScenarios(b *testing.B) {
	b.Run("JSONProcessing", func(b *testing.B) {
		payload := `{"users":[{"id":1,"name":"Ada"},{"id":2,"name":"Lin"}],"active":true}`
		b.SetBytes(int64(len(payload)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkValue = JsonDecode(payload)
		}
	})

	cases := []struct {
		name   string
		source string
		call   string
		args   []interface{}
	}{
		{"CRUDMapping", `public func mapRow(map $row): map { return {"id": $row["id"], "name": $row["name"]} }`, "mapRow", []interface{}{map[string]interface{}{"id": int64(1), "name": "Ada"}}},
		{"HTTPHandler", `public func handle(map $request): map { return {"status": 200, "body": $request["path"]} }`, "handle", []interface{}{map[string]interface{}{"path": "/users"}}},
		{"ArrayTransform", `public func transform(array $items): array { $out = [] foreach ($items as $item) { $out[] = $item * 2 } return $out }`, "transform", []interface{}{[]interface{}{int64(1), int64(2), int64(3), int64(4), int64(5), int64(6), int64(7), int64(8)}}},
		{"TemplateRendering", `public func render(string $name, int $count): string { return "Hello " . $name . ": " . $count }`, "render", []interface{}{"Ada", int64(12)}},
		{"DBMapping", `public func hydrate(array $rows): array { $ids = [] foreach ($rows as $row) { $ids[] = $row["id"] } return $ids }`, "hydrate", []interface{}{[]interface{}{map[string]interface{}{"id": int64(1)}, map[string]interface{}{"id": int64(2)}, map[string]interface{}{"id": int64(3)}}}},
	}
	for _, item := range cases {
		b.Run(item.name, func(b *testing.B) {
			runtime := benchmarkPreparedRuntime(b, item.source)
			function := runtime.Functions[item.call]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkValue = runtime.CallMethodEvaluated(function, nil, item.args)
			}
		})
	}
}

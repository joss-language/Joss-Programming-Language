# Runtime audit before optimizing [Index](README.md) Audited status: commit `9d27239`, Go 1.26.2, Windows/amd64.This document describes the runtime before optimizations.Reproducible measurements are at `benchmarks/BASELINE_9D27239.md`.## Actual flow```text
.joss
  -> lexer (Token por valor)
  -> parser Pratt (nodos AST enlazados por interfaces)
  -> analyzer (scopes y símbolos por map[string]*symbol)
  -> diagnostics
  -> Runtime.Execute
  -> executeStatement (type switch)
  -> evaluateExpression (type switch)
  -> helper especializado
  -> valor Go en interface{}
```
`pkg/bytecode` does not alter this flow: it restores via gob+flate the same AST that the evaluator loops back through.There is no typed IR or persisted semantic resolution between analyzer and runtime.## Layers traversed by an expression A normal top-level expression traverses at least four decisions: 1. `Execute` registers statements and selects script/Main mode.2. `executeStatement` makes a type switch of statements.3. `evaluateExpression` makes another type switch of expressions.4. The helper (`evaluateInfix`, `evaluateMember`, `executeCall`, etc.) again discriminates Go types, searches for names or traverses ASTs.A call adds argument evaluation and materialization, name resolution, callable classification, frame creation, parameter binding and checking, body execution, return `panic/recover`, and return type validation.A method access adds class lookup, inheritance traversal, linear traversal of the statements of each class, and construction of a `BoundMethod`.## Representations and structures created |Concept |Previous representation |Relevant cost ||---|---|---||Values ​​|`interface{}` |boxing of primitives when they escape;repeated type switches ||Variables |`Runtime.Variables map[string]interface{}` |hashing by local and global read/write ||Runtime types |`VarTypes map[string]string` |repeated parsing of type name ||Constants |`Constants map[string]bool` |separate lookup ||Frame |temporary replacement of three maps of the `Runtime` |three new maps per call and copy of globals ||Features |`map[string]*parser.MethodStatement` |resolution by name and execution of the original AST ||Closures |AST + copy of three maps |full copy on capture;mutex per captured environment ||`ref` |three maps + name at `VariableReference` |lookup by name and possible nested reference ||Objects |`Instance{Class, Fields map, Constants map}` |property by hash;metadata/method per AST traversal ||Arrays |`[]interface{}` |boxed elements;literals grow with `append` without capacity ||Maps |`map[string]interface{}` |dynamically evaluated keys;literals without initial capacity ||Strings |`string` Go UTF-8 |previous indexing by byte, not by character ||Futures |goroutine + cloned runtime + `chan bool` |fork copies maps even for a trivial value ||Channels |`chan interface{}` |dynamic border without element contract |The package `pkg/core` contained 104 Go files and 17,623 lines.The static search found 651 mentions of `interface{}`, 243 of `map[string]interface{}`, 17 uses of reflection, 100 calls to `panic`, and 29 to `recover`.These figures include stdlib/framework and tests;Not all of them are on the hot path, but they quantify the concentration of responsibilities.## Execution maps ### Hot path```text
while
  -> evaluate condition
     -> identifier -> Variables[name]
     -> infix -> conversión numérica + operador
  -> execute block
     -> postfix
        -> identifier -> Variables[name]
        -> box int64
        -> Variables[name] = value
```
The CPU profile of the 10,000 iteration loop places `evaluateExpression` at 82.13% cumulatively, `evaluateInfix` at 30.88%, `evaluatePostfix` at 46.71%, and map/hash primitives among the highest flat costs.### Allocation path```text
call
  -> []interface{} de argumentos
  -> frameVariables map
  -> frameVarTypes map
  -> frameConstants map
  -> parameterNames map
  -> ReturnPanic heap object
```
In nested calls, `callMethodEvaluated` represents a flat 93.12% of the allocated space.In the loop, `evaluatePostfix` represents 86.18% of the allocated space: the new `int64` escapes by being saved as `interface{}`.### Dispatch path```text
CallExpression
  -> evaluar argumentos
  -> IsBuiltin map lookup
  -> hasta cinco switches de familias builtin
  -> Functions[name]
  -> Variables[name]
  -> applyFunction
     -> PluginCallable / CapturedFunction / BoundMethod /
        MethodStatement / FunctionLiteral / NativeHandler /
        func([]interface{}) / reflect.Func
```
User method resolution loops through class statements on each access.Plugin class/method resolution can loop through all plugins and their symbols.Static methods also create a dummy instance.### Error path Four policies coexist: - `panic(*JossError)` for some language errors;- `panic(string)` or `panic(error)` for others;- `fmt.Print*` followed by `nil` for recoverable faults;- typed panics (`ReturnPanic`, `BreakPanic`, `ContinuePanic`) for normal flow.`try/catch`, calls, loops, async and various framework components recover panics with different criteria.The previous structured error had no stable code, Joss stack, context, hint or cause.### Type-check path```text
binding/assignment/call/return/property write
  -> typesystem.Parse(typeName string)
  -> runtimeTypeOf(interface{})
  -> typesystem.Assignable
  -> si es instancia: recorrido de herencia
```
The analyzer already knows types of symbols, parameters, returns and members, but that information is not recorded in the AST.The runtime again parses type strings, resolves bindings, locates members, and validates contracts.`mixed`, plugins and external bytecode force a dynamic slow path to be preserved;They do not require penalizing verified call sites.## Audited semantics - `mixed` is represented as a string of type and value `interface{}`;intentionally disables static/dynamic incompatibility.- `ref` does not expose Go pointers;preserves the binding maps and the name.It cannot cross plugins, async or native handlers.- Exceptions and the control `return/break/continue` use panic/recover.- `async` clones the runtime before launching the goroutine.DB and immutable records are shared;variables, types, constants, maps, slices and instances are partially copied.- Retained closures copy the environment and serialize its use with a mutex.- Arrays and maps are heterogeneous.The analyzer only knew `array`/`map`, so an index returned `mixed`/`unknown`.- Go reflection appears mainly in array buildins and host callable fallback.- The above arithmetic converted integers to `float64` and then to `int64`.This loses precision above 2^53 and the overflow of `int64` was silent.- `string[index]` used `str[idx]`: returned a UTF-8 byte converted to a string.## Hotspots classified |Priority |Hotspot |Evidence ||---|---|---||P0 |Local variables in maps + postfix boxing |loop: ~2.10 ms, 9,752 allocs;hash/map dominates CPU;postfix 86.18% of alloc_space ||P0 |Construction of frames per call |simple: 760 B/7 allocs;nested: 2,312 B/23;`callMethodEvaluated` 93.12% of alloc_space ||P1 |Repeated resolution of methods/fields |object profile: `evaluateMember` 16.18% cumulative and `lookupInstanceFieldOwner` 6.08% ||P1 |Return by panic/recover |each return call creates `ReturnPanic`;appears in 65.54% accumulated of the allocations profile of nested calls ||P1 |Serialized AST Loading |~255 us, 80,208 B and 857 allocs for the startup program ||P1 |Integer arithmetic via float64 |demonstrable loss of precision and duplicate work per operation ||P2 |Dispatch builtin by cascade of switches |initial lookup followed by up to five families ||P2 |Literals without capacity and metadata per string |small arrays/maps allocate more than necessary;types are parsed again ||P2 |Full fork by async |~4.17 us, 1,680 B and 18 allocs for trivial async+await ||P3 |Host Compatibility Reflection |did not appear as a hotspot in pure Joss programs;conserve as slow path ||P3 |DB/HTTP/IO Framework |architecturally important, but did not dominate the profiles of the pure evaluator |## Concurrency and GC The pre-existing suite passes:```text
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
```
The async blocking profile attributes the expected blocking to `Future.Wait` and `runtime.chanrecv1`;showed no application mutex contention.The mutex profile was almost entirely runtime/GC.This does not prove absence of risks in plugins/framework: it only establishes the baseline covered by tests.## Conclusion before changes The first optimization should eliminate allocations and hashing inside the loop and in calls before introducing a new `Value` representation.The evidence does not yet justify globally replacing `interface{}` or building a full VM.It does justify keeping an AST path and adding pre-parsed metadata, compact frames/simple caches, and secure integer fast paths.
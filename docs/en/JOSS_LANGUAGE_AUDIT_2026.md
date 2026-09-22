# Comprehensive Joss language audit — September 2026

[Index](README.md) · [Architecture](ARQUITECTURA.md) · [Types](SISTEMA_TIPOS.md) · [Diagnostics](DIAGNOSTICOS.md)

**Scope and method.** Review of the current repository tree, not of the thesis or historical audits as authority. Parser, AST, analyzer, typesystem, runtime, server, CLI, VM, bytecode, plugins, mobile, tools, tests, benchmarks, documentation and the reference web project were compared. `go test ./...` completed with code 0 during this audit. Observations depending on a code path are indicated as such; performance measurements or security exploits are not attributed without a specific test. This report is an assessment and a plan: **it does not implement the improvements**.

## 1. Executive Summary

Joss already fulfills an important part of the goal «analyze → validate → execute»: the CLI analyzes the project before running it, the analyzer has symbols and scopes, mandatory parameter types, nominal contracts, checks of known calls and members, declared returns, structured diagnostics and detection of certain constant arithmetic operations. Published execution remains an AST interpreter in Go with callable slot plans. `JOSSBC2Z` packages compressed AST and the VM is experimental; neither is a general semantic compilation phase.

The largest current gaps are in **concurrent lifecycle and resources**, **native API and collection contracts**, **state-sensitive flow**, and **discrepancies between entry paths**. The mobile path implements a timeout that reports failure without stopping the goroutine; the `async`, `Task` and instance finalization paths create forks without freeing their ownership; `GranDB::transaction` opens a `sql.Tx` that ordinary callback queries do not use. Channels delegate invalid operations to Go panics. These are findings from current code, not aesthetic proposals.

**Verdict:** the language foundation is promising and modular in analysis layers, but does not yet offer guarantees equivalent to a compiled language for programs with `mixed`, natives, external resources or concurrency. P0 must prioritize lifecycle/atomicity failures and regression tests; then expand static contracts. Performance improvements should only proceed after profiling representative workloads.

## 2. Current Architecture

```text
.joss → parser.Lexer → parser Pratt → parser.Program / AST
                                  ├→ analyzer.LoadProject (entrypoint + app/**/*.joss)
                                  │    → collectDeclarations → projectScope
                                  │    → validateNominalContracts → analyzeSourceBodies
                                  │    → diagnostics.Diagnostic
                                  ├→ bytecode.Encode → JOSSBC2Z → runner Go → intérprete
                                  └→ core.Runtime.Execute → registro de clases/funciones
                                       → runtime/plan.Callable → runtime/frame.Slot
                                       → evaluator/executor → nativos/host/servidor
                       VM experimental ← subconjunto AST (sin ruta CLI principal)
             plugins JP ← SymbolIndex + AST/JPBC propios
```

**Boundaries and dependencies.** `pkg/parser` defines tokens, lexer, precedences, AST and parser; `pkg/typesystem` defines names, assignability and integer checks; `pkg/analyzer` imports those layers and `pkg/diagnostics`, but not `pkg/core`. `pkg/core/analyzer.go` projects built-ins, native classes and plugins into the semantic environment. `pkg/core` executes AST and integrates SQL, files, network, views, auth, WebSocket and plugins. `pkg/server` adapts HTTP to forked runtimes. `cmd/joss` orchestrates the project; `cmd/runner` consumes builds. `pkg/mobile` provides embedded API and C exports; `sdk/dart` adapts that surface and downloads release binaries. There is no separate package named `libjoss` in this tree. `vscode-joss` provides LSP; `pkg/formatter`, `pkg/linter`, `pkg/fixer` and `pkg/tester` are separate tools. `pkg/plugincompiler`, `pkg/pluginpkg`, `pkg/pluginruntime` and `pkg/vfs` form the plugin/package boundary. `pkg/i18n`, `pkg/template`, `pkg/viewtemplate` and `pkg/crypto` support application surfaces.

**Sound decisions already applied.** Canonical catalogs of tokens, built-ins and primitive methods; semantic signatures decoupled from execution; isolated frames for named functions; closures with lexical capture; temporary and invariant `ref`; idempotent `Free` with `atomic.Bool`; counted ownership for drivers; session snapshots; shared directive scanner; cached class metadata. See [Architecture](ARQUITECTURA.md) and `pkg/core/runtime_lifecycle.go`, `pkg/core/call_arguments.go`, `pkg/analyzer/analyzer.go`.

**Remaining coupling.** `core.Runtime` bundles language and host services (`pkg/core/types.go`); `cmd/joss/main.go` and `pkg/server/handler.go` orchestrate many routes; `core/evaluator_member.go` retains dynamic lookups by name and plugin scans. This is real functional coupling, but splitting files by size alone would not be a solution. Intentionally duplicated contracts (analyzer vs runtime) must share metadata, not state. The analysis boundary ends upon producing diagnostics: the AST does not become a typed IR reused by all execution paths. `runtime/plan` is calculated upon callable execution, not certifying the entire program.

## 3. Language Pipeline

Real source syntax requires `$` for variables and explicit visibility on global functions/classes. Thus `func suma(int a, int b)` and `class Usuario` are conceptual examples, not valid Joss: they would be `public func suma(int $a, int $b): int { return $a + $b }` and `public class Usuario { public string $nombre }`. The loop is `foreach ($items as $item) { ... }`; `await($futuro)` is a native call, not a prefix operator; `async { ... }` creates the future.

| Source | Tokens / Primary AST | Analyzer and known type | Runtime and pending resolution |
|---|---|---|---|
| `int $edad = 20` | type, variable, assignment, integer → `LetStatement(IntegerLiteral)` | declared type `int`, initializer compatibility | stores typed binding, `int64`; runtime revalidates |
| `var $nombre = "Joss"` | `VAR`, literal string → `LetStatement` | infers and fixes `string` | slot/map with inferred type; revalidation on reassign |
| `mixed $valor = obtenerValor()` | declaration + `CallExpression` | explicit `mixed`; call signature if resolvable | searches function/callable and effective runtime value |
| `public func suma(int $a, int $b): int` | `MethodStatement`, `ReturnStatement(InfixExpression)` | parameters, arity, `+`, return type and exit paths | plans slots; resolves call, frame and operation |
| `public class Usuario { public string $nombre }` | `ClassStatement` + property | nominal contracts, members and known visibility | cached metadata; instance uses `Fields map[string]interface{}` |
| `await($f)` | `CallExpression` | known built-in; result frequently `mixed` | `Future.Wait()`, may block or propagate error |
| `foreach ($items as $item)` | `ForeachStatement` | iterable is inferred; element becomes `unknown` in binding | executes iteration of array/map/channel/generator, per value |

Precise tokens and precedence come from `pkg/parser/token.go`, `lexer.go` and `parser.go`; the table summarizes categories, not an instrumented `NextToken` trace. The AST retains tokens/positions and written types; it does not retain a `SymbolID` or universal semantic reference to each call. `runtime/plan/callable.go` adds `IdentifierSlots` and `NameSlots` for parameters/locals, but globals, properties, methods, plugins and natives continue with name lookups.

In classes/methods: the analyzer collects declarations before bodies, validates inheritance/interfaces and visibility, and types calls when receiver/signature is known. Runtime still checks constructor, property, access and effective value type. In closures: `FunctionLiteral` captures current maps and slots; upon invocation it uses a planned frame and captured environment mutex. In `try/catch`, `throw`, `defer`, generators, `select`, `match` and `async`, syntax exists, but state/effect analysis is partial. For `null`, `T?` normalizes to union; narrowing applies in `is`, comparisons with null and some `guard`/ternaries (`pkg/analyzer/infer_narrowing.go`).

## 4. Type System

| Type/concept | Real guarantee | Limit |
|---|---|---|
| `int` | `int64`; additions/subtractions/multiplications and negation protected against overflow; division by zero protected | native values or `mixed` tested only at execution |
| `float` | `float64`; accepts assignment from `int` | integers larger than 2^53 are not all represented exactly; review NaN/Inf per specific operation |
| `decimal` | `shopspring/decimal`; accepts int/float promotion; decimal literal | converting `float` may import its prior rounding |
| `string`, `bool` | canonical types; UTF-8 strings and specific Unicode operations | string coercion to int/float/decimal/bool exists upon assignment to explicit type; rule must remain explicit in diagnostics |
| `array<T>`, `map<K,V>` | element/value type verifiable upon declaration and full value validation; runtime maps use string key | not universal generics; native functions and mutations may return `unknown/mixed`; no proof of safe aliasing/variance |
| `object`, classes, interfaces | nominal type, inheritance and project interfaces; known members checked | fields are maps, not offsets; `mixed` receiver requires dynamic lookup |
| `channel` | channel identity | no message type or static open/closed state |
| `mixed` | deliberate dynamism | accepts any source/target in `Assignable`; defers errors to runtime |
| `var` / first assignment | infers first concrete type and fixes it; null defers inference | conditional flow and unknown return calls reduce precision |
| `const` | prevents reassignment; constant properties protected | does not make referenced structures deeply immutable |
| `T|null`, `T?` | normalized nullable union; assignment checking and local narrowing | no general interprocedural null-state analysis |

`typesystem.Assignable` accepts `Unknown` permissively to avoid false positives (`pkg/typesystem/types.go`); therefore «clean analysis» does not mean «no type error possible». `int $edad = "hola"` gives incompatibility if the string is not coercible; `int $edad = "20"` can be accepted via `CoerceString`. `var $contador = 10; $contador = "texto"` is rejected; `mixed` is the dynamic option. `ref T` is invariant and does not escape. Nominal compatibility is complemented in `pkg/analyzer/infer_nominal.go` and in `core.checkParsedType`; there are two necessary defenses, but testing their agreement against a common corpus is advisable.

**Consistency finding:** `Assignable` treats `array<T>` and `map<K,V>` covariantly and accepts collections without type arguments as target/source. With mutable/aliased containers this can admit an assignment that subsequently permits introducing incompatible elements. The actual extent of exposure depends on each mutation path: this is a soundness risk requiring aliasing regressions before changing the rule. `runtimeTypeOf` of an array/map drops generic arguments; `checkParsedType` traverses elements for a typed variable, but a subsequent native operation may lose that check.

**Numerical accuracy:** `typesystem.Assignable` documents `int → float` as «losslessly», but a `float64` cannot represent every `int64` (for instance, 2^53+1). Compatibility is implemented, not the precision guarantee of the comment. This warrants a value test and an explicit policy: warning for potentially inexact conversion, explicit cast, or preserving behavior while documenting loss. `decimal` preserves decimal precision upon receiving an integer; converting from `float` does not restore already lost digits.

## 5. Static Safety

The analyzer detects nonexistent symbols and classes, duplicates, visibility, unknown source types, incompatible initializers/reassignments/arguments/returns, known arity and argument names, illegal references, nonexistent known members, invalidly typed indices, invalid known operations, constant overflow and division by zero, demonstrably missing returns, and code after an unconditional exit. It emits `JOSS-...` codes from `pkg/analyzer`, supported by `pkg/diagnostics`. `pkg/analyzer/flow.go` is deliberately conservative: it does not build a general CFG, and `hasYield` treats a generator as a valid exit.

| Error / decision | Current phase | Reasonable move | Fallback |
|---|---|---|---|
| syntax, mandatory visibility | parser | already early | do not execute |
| unknown variable/function/class | analyzer when resolvable | close entrypoint and plugin differences | runtime for dynamic loading |
| incompatible type / return / `ref` | analyzer | deepen aliasing and flow | mandatory runtime |
| call/member with `mixed` or unsigned native | runtime | publish verified signatures and narrowing | runtime |
| use before initialization | partial; runtime slot `Initialized` | definite assignment via CFG | runtime |
| null dereference | partial narrowing / runtime | state analysis and `T?` | runtime |
| division by zero / index out of range | constants: analyzer; variables: runtime | low-cost constant propagation | runtime |
| closed channel / blocking send/recv | runtime/Go panic | state only if local and demonstrable | structured runtime |
| unbound transaction / unclosed resource | runtime/host | effects and API ownership | runtime and integration tests |

The key rule is to preserve runtime defenses: dynamic loading, `mixed`, I/O and concurrency are generally undecidable before execution.

## 6. Runtime Safety

Joss errors use `pkg/runtime/errors` and `pkg/core/errors.go`; arithmetic and indices have stable codes. `MaxCallDepth` limits recursion to 1024 frames by default. The runtime validates slot types, fields, parameters, returns, visibility and `ref`; the pool is cleaned in `Free`. Go failures originating in native APIs do not always convert to `JossError`: duplicate `close(ch.Ch)` and `send` to a closed channel can panic; `make_chan` with negative capacity as well. This gives a different experience compared to structured arithmetic/indexing failures. `await` and `recv` may block indefinitely if there is no producer; no general structured cancellation exists.

Classification of controls: parser for impossible construction; analyzer/typesystem for demonstrable incompatibility, flow and null; planner for stable references and metadata; runtime for channel state, limits, resources and external values; stdlib for network/FS/SQL contracts; tooling for diagnostics, tracing and dependency audits. FFI and plugins with host permission do not constitute an OS sandbox: package signing verifies provenance/integrity, not effect isolation. Any permission proposal must distinguish host capabilities from actual isolation.

## 7. Memory Model

Values are stored in `interface{}` and Go structs: arrays `[]interface{}`, maps `map[string]interface{}`, instances with `Fields map`, Go strings and decimal. Go's GC manages ordinary memory. `executionFrame` uses tagged typed slots and `sync.Pool`; `Runtime` also uses pool and `Free` clears caches, globals, request state, generators, defers and plugins. `Fork` copies tables, shares AST/plans and `*sql.DB`; it clones certain instances/maps/slices only shallowly (`pkg/core/runtime.go`). A nested mutable structure may remain aliased across forks if introduced through an uncovered path; require a specific test before claiming deep isolation.

`CapturedFunction` stores a lexical snapshot in mutex-protected maps, with the aliasing semantics inherent to internal values. `ref` retains binding during the call, not a general pointer or escapable value. Ownership of `DB` is external to runtime; that of native drivers is counted. Ownership of asynchronous forks and finalizers is not closed across all paths. `evaluateNew` creates a `Fork()` per instance for a finalizer, and `AutoDestroy` can create another fork for a destructor without visible `Free` (`pkg/core/evaluator_member.go`, `instance_lifecycle.go`). Furthermore, Go finalizers do not guarantee timely execution; they must not be the sole strategy for critical resources.

## 8. Concurrency

`async { ... }` projects to the built-in `async`: it creates a `Future`, performs `Fork` and executes a goroutine; `await` awaits its result/error. `channel`, `send`, `recv`, `close`, `foreach` over channel and `select` exist. `pkg/server` uses one fork per request and has characterizations of WebSocket, sessions and cleanup; its safety must not be extrapolated to other goroutines. `ClosureEnvironment.mu` serializes shared use of a capture.

Risks proven by inspection: forks without `Free` in `builtins_async.go` and `task.go`; tasks without cancellation or mandatory join; `Future` may go unobserved; channel operations without state and with Go panics; `mobile.RunDirect` returns timeout while the goroutine may still be using the runtime that its caller frees. The latter path can cause race/use-after-recycling, not just delay. Prioritize context/cooperative cancellation and explicit ownership before promising an «execution timeout» in the mobile SDK. `go test -race` on the existing suite is necessary but does not prove absence of races in uncovered interleavings.

## 9. Performance

Useful prior work exists: `runtime/plan` assigns slots per AST identifier, `runtime/frame` reduces maps for locals, and `classMetadataCache` avoids rebuilding hierarchies per access. `pkg/core/runtime_benchmark_test.go` covers startup, operators, loops, functions, objects, collections and application scenarios; `pkg/vm/vm_benchmark_test.go` covers only a subset. There are no CPU/heap profiles in this audit that would allow declaring a dominant hot path or promising percentages.

Candidates for **measurement**: `evaluateNew` looks up plugin classes by traversing registry/classes and creates fork/finalizer per instance; `evaluateMember` and native dispatch resolve names; classes store fields in maps; `typesystem.Parse` on unions reparses names; collection coercion and validation walk values; closures copy maps; `Fork` copies multiple registries. A monomorphic/polymorphic method cache or IDs only provide value for stable classes with clear invalidation. Before optimizing: bench cold/hot calls/properties, `benchmem`, pprof profiles and semantic comparison of fast path vs general path.

## 10. Diagnostics

`diagnostics.Diagnostic` contains code, severity, file, range, message, explanation and suggestion; the analyzer sorts by file/line/column. Parser and LSP consume diagnostics. Runtime arith/index have codes; several natives still throw strings or Go panics without code or precise span. A `JossError` supports stack, but translation in CLI/mobile/server does not share identical presentation. The parser recovers partially from errors and may emit derivative ones after the first. The priority is to align frequent native errors and add full location/range to runtime failures, without inventing a new code if a canonical one already exists. Examples in `docs/DIAGNOSTICOS.md` must accompany each new code.

## 11. Tooling

The CLI implements `run`, `build`, `check`, `analyze`, `test`, `format`, `lint`, `fix`, `eval` and **`repl`** (`cmd/joss/main.go`), along with application/package commands. `format` preserves trivia with its own scanner consuming canonical symbols; `fix` uses transformation rules/regexes and requires tests against strings/comments. `vscode-joss` declares completion, hover, definition, references, signature help, diagnostics, document symbols and formatting; no rename provider appears in `server.ts`. There are tests for parser fuzzing, typesystem fuzzing, Unicode value, VM differential and benchmarks. Integrated Joss debugger/profiler and a project effect/dependency inspector are missing; «missing» does not imply high priority.

**Concrete documentation discrepancies:** [Implementation status](ESTADO_IMPLEMENTACION.md) states that there is no REPL, interfaces, `defer` or `select`; CLI, AST, analyzer and executor do have them. The historical report that used to occupy this page also claimed absence of `guard`/subsequent narrowing, but `body_analysis.go` and `infer_narrowing.go` already implement both. `sdk/dart/README.md` describes mobile distribution that must be verified against actual release artifacts before promising automated installation. Correct those pages in the subsequent documentation phase, with executable contracts; do not use their former assertions as a design foundation.

## 12. Technical Debt

1. **HIGH:** mutable collection contracts and `Unknown/Mixed` leave soundness holes; `typesystem.Assignable` and `runtimeTypeOf` need aliasing tests and explicit policy.
2. **HIGH:** entry paths do not share a `PreparedProgram` with persisted analysis and metadata; CLI, mobile, runner and server orchestrate analysis/registration differently.
3. **HIGH:** ownership of forks outside HTTP is not expressed in types/API, permitting oversights.
4. **MEDIUM:** native metadata publishes reliable returns, but `ArityKnown=false` across many APIs; analyzer cannot anticipate arity/types.
5. **MEDIUM:** method/class metadata and plugin resolution still use strings/maps and scans in potentially frequent paths.
6. **MEDIUM:** `pkg/core` bundles semantics and infrastructure; decouple only where boundary contracts/tests justify it.
7. **MEDIUM:** status documentation and historical audits contradict current features; language audit cannot rely on those pages without revalidating.
8. **LOW:** generated LSP symbols coexist with rich manually maintained signatures; risk of incomplete metadata.

## 13. Bugs Found

| Severity | Evidence and root cause | Effect / pending verification |
|---|---|---|
| **CRITICAL** | `pkg/core/database.go` branch `transaction` opens `tx := db.Begin()`, invokes callback via `r.CallFunction` and commits `tx`; ordinary queries use `r.GetDB()`, lacking `tx` context. Already recorded as a limitation in [Status](ESTADO_IMPLEMENTACION.md). | A callback operation may persist even if the callback fails and `tx` rolls back. SQLite test in `t.TempDir`: insert + throw + verify table empty. |
| **HIGH** | `pkg/mobile/mobile.go` selects on `time.After` but neither cancels nor awaits the goroutine; restores `os.Stdout/Stderr` and `defer rt.Free()` may recycle the still-active runtime. | Timeout is not execution termination; risk of persistent goroutine, delayed output and race. Timeout test with signaling/join and `-race`. |
| **HIGH** | `pkg/core/builtins_async.go` and `pkg/core/task.go` call `r.Fork()` without `Free()` in goroutine; `evaluator_member.go` and `instance_lifecycle.go` repeat pattern in finalizers/destructor. | State/handles retained and counted ownership not released. Retained driver test and fork cycles; define owner closure. |
| **HIGH** | `builtins_async.go` executes `close(ch.Ch)`, `ch.Ch <- value` and `make(chan, size)` without guarding against double close, send on closed, or negative capacity. | Go panic or hang without structured Joss error. Tests for all three cases and concurrent execution with `-race`. |
| **MEDIUM** | `typesystem.Assignable` permits `int → float` and describes it as lossless, although `float64` loses precision beyond 2^53. | Surprising numerical result in large IDs/counters. Test with 2^53+1 and compatibility decision. |
| **MEDIUM** | `pkg/analyzer/project.go` loads entrypoint and `app/**/*.joss`; `routes.joss` is loaded per server according to [documentation audit](DOCUMENTATION_AUDIT.md). | `joss analyze` may overlook route errors; verify web fixture and unify execution sources manifest. |
| **MEDIUM** | `docs/ESTADO_IMPLEMENTACION.md` denies REPL/interfaces/defer/select; implementation present in `cmd/joss/main.go`, AST and `core/executor.go`. | User decisions and roadmap based on inaccurate information. Correct via documentation contracts. |

The former block `match`, ternary `break` escape, and pool reset are not classified as current bugs: subsequent paths/guards exist and require fresh reproduction before reopening. Prior code findings still require regression tests upon remediation; inspection is no substitute for an end-to-end test.

## 14. Missing Language Features

Real problems: lack of general definite assignment and null-state analysis; no channel message types or task cancellation; no result type for expected I/O failures; no effect/capability contract for native calls; collection precision degrades under `mixed` APIs. Value `match` exists, but is not exhaustive over enum/union types. Interfaces and enums **do exist**; they must not be proposed as missing. Source imports do not exist by explicit zero-imports project decision; adding them now would break architecture without solving the priority problem. Nor are Rust-style general ownership, universal generics, traits, AOT/LLVM or a full VM justified in this phase.

## 15. Proposed Language Features

Each syntax is a **proposal**, not an existing contract.

| Proposal and example | Problem | Parser/AST | Analyzer | Runtime/tooling | Compatibility; complexity |
|---|---|---|---|---|---|
| **Definite assignment and null-state**: `T? $x`, guard and subsequent access | use before initialization / nullable dereference | no new syntax; CFG over current AST | states per symbol and joins; narrowing invalidated by mutation/alias | defenses preserved; hover shows state | compatible with warning → opt-in error; medium |
| **Lightweight `Result<T,E>`**: `Result<Usuario, DbError>` with `match` | expected DB/HTTP failures currently mix nil, false, panic | parameterized type grammar already exists for collections; expand AST type refs and construction | variants and exhaustiveness; explicit propagation only if designed | tagged value and SDK; LSP completion | experimental; high |
| **Typed channel and safe close**: `channel<string>` | sending wrong value / closing twice | extend TypeReference/AST without changing `send` | check message type and local state diagnostics | state wrapper, Joss error, hover; race tests | compatible opt-in; medium |
| **Exhaustive `match` for enum/union** | missing branch resolves to null | current AST has arms/default; possible new simple pattern | case coverage and duplicates | runtime retains fallback; LSP quick fix | warning first; medium |
| **Effect attribute for trusted natives**, e.g. metadata `effects: io, blocking` (no source syntax initially) | analyzer unaware of blocking/resources | no initial parser/AST change | validate `await`, resources and critical paths using signatures | catalog and LSP; host wrapper | compatible; medium |

Do not recommend superficial `readonly` while aliased maps/slices remain mutable; a partial guarantee must be explicitly named «immutable binding». Safe casts would only provide value after defining failures as `Result`/nullable and testing interaction with `mixed`. Records/sealed classes can be reevaluated after nominal exhaustiveness; extension methods/traits and overloads add complex resolution without a demonstrated bug demanding them.

**Selective comparison:** Kotlin/Dart/Swift inspire null-state and local promotion, but Joss needs to invalidate narrowing upon writing `mixed` or aliases; TypeScript demonstrates utility and boundaries of flow control with dynamism; Rust contributes `Result` and explicit resources, without requiring full borrow checking; Go provides channels and cancellation contexts, while Joss must prevent visible Go panics; Java/C# exhibit stable metadata and early dispatch, useful only for nominal receivers; Lua/JVM demonstrate a viable VM following semantic equivalence; Python/PHP recall the value of `mixed` and rapid iteration alongside the cost of late failures. Copying external syntax without resolving internal contracts provides no guarantees.

## 16. Compiled-Like Experience

Incremental target architecture, derived from current layers:

```text
parser.Program + SourceUnits
  → analyzer: símbolos, tipos, contratos, CFG/estados
  → diagnostics + AnalysisFacts (ID de símbolo, tipo, efectos, spans)
  → PreparedProgram (AST inmutable + planes de callable/clase + referencias resueltas)
  → intérprete core (ruta publicada) / VM sólo para subset diferencial
  → runtime host (recursos, IO, requests, plugins)
```

First add **sidecar** semantic facts indexed by AST nodes, avoiding mutating AST shared across forks. Immutable IDs per project/version can stabilize symbols; `SlotID` already exists for locals. Pre-resolve functions and methods only when class and table are stable; `mixed`, dynamic plugins and host globals retain guards/fallback. A full typed IR would carry a high cost of semantic duplication: require a prototype with differential comparison and metrics before adopting it. CFG for returns, assignment and null-state has value prior to optimizations. Constant folding/propagation only for pure operations with `typesystem.CheckedIntBinary`; never anticipate native effects. Field offsets and inline caches require class versioning/invalidation; they are not phase one.

## 17. Architecture Improvements

1. One `PreparedProgram` per project with file provenance, diagnostics, `AnalysisFacts`, plans and signature catalog; CLI/mobile/runner/server consume the identical validation. Keep `analyzer` from importing `core`.
2. An ownership API for forks/tasks with mandatory `defer Free()` in goroutine and explicit finalization of resource-bearing instances. Do not shift `DB` ownership to pool.
3. A transaction context in DB adapter, propagated to all callback operations; wrapping `Begin/Commit` alone is insufficient.
4. Metadata `NativeMethodDefinition` with parameters and effects only where the contract is verified; `ArityKnown=false` must remain for unknown cases.
5. Single project source file manifest covering web routes actually executed; preserve zero-imports policy.
6. Error contexts with code/span/stack at native and SDK boundaries, without ceremonial public managers.

## 18. Security Improvements

Priority: atomic transactions and resource management; thereafter, cooperative request/task cancellation; explicit capabilities for filesystem/network/process/FFI in host/plugin; size, time and depth limits at operation level. Verify paths and VFS against traversal, and do not conflate JP signing with sandbox. For `null`, index, division and overflow, analyzer detects demonstrable constants/states and runtime preserves checks. Async error propagation must be observable even if the Future is not awaited; an orphaned task policy (log/propagation to request or cancellation) requires specification before implementation. Native `panic` must convert to structured error without silencing unexpected internal faults.

## 19. Performance Improvements

Hypotheses requiring benchmark: pre-index exported plugin classes for `new`; reduce forks/finalizers per instance; share immutable class metadata across forks; resolve nominal receiver to precomputed method; property access cache with invalidation; specialize known-type operators in plans. The first improvement may be in **safety and memory** as well as time. Minimum metrics: ns/op, B/op, allocs/op, request p50/p95, heap retained per 10k objects/futures, cold start cost and pprof. Reject an optimization if it breaks equivalence, increases retention or benefits only irrelevant microbenchmarks.

## 20. Roadmap

Priorities are P0 (safety/correctness flaw), P1 (core guarantee), P2 (conditional evolution), P3 (optional). Each task stems from preceding problem and evidence; when executing: reproduce → characterize → compare alternatives → fix → tests → benchmark where applicable.

### Phase 1 — Correctness

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| Real transaction | P0 | Callback operates outside `sql.Tx`; introduce transactional executor in all callback queries, rollback on error | `core/database*.go`, SQL tests | compatible, bug change | medium risk due to nesting; high atomicity | SQLite rollback/commit/nested/error |
| Fork ownership | P0 | async/task/finalizer forks without closure; `defer Free`, destructor and resource policy | `core/builtins_async.go`, `task.go`, `evaluator_member.go`, `instance_lifecycle.go`, lifecycle | compatible | medium risk; high memory/handles | owner count, GC, panic, race |
| Mobile timeout | P0 | goroutine outlives timeout and runtime is freed; cooperative cancellation + join or isolate process if hard timeout is promised | `mobile/mobile.go`, `core/executor.go` | deprecation of «hard» timeout if unachieved | high risk; high isolation | infinite loop, blocking IO, race, stdout |
| Safe channels | P0 | Go panics on close/send/capacity; state wrapper and Joss errors | `core/builtins_async.go`, channel tests | compatible except error type | medium risk; high stability | closed/duplicate/negative/concurrent |

### Phase 2 — Type Safety

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| Collection soundness | P1 | covariance/aliasing and type loss; decide invariance or revalidated wrapper and document | `typesystem/types.go`, analyzer, core collections | warning → deprecation if breaking | high risk; high safety | alias, nested, mutation, natives |
| Native signatures | P1 | unknown arity/types; complete only trustworthy contracts | `core/native_signatures.go`, analyzer, catalog | compatible with warning | low/medium risk; high DX | signature-handler parity, negatives |

### Phase 3 — Static Analysis

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| CFG and definite assignment | P1 | initialization/return partially resolved; minimal CFG per callable with joins | `analyzer/flow.go`, `body_analysis.go` | warning → opt-in error | medium risk; high safety | branches, loops, try/catch, defer, yield |
| Null-state/exhaustiveness | P1 | dereference and incomplete match; narrowing with invalidation and enum coverage | `analyzer/infer_narrowing.go`, `flow.go`, LSP | warning → opt-in error | medium risk; high DX | alias, mutation, union, defaults |
| Complete web source | P1 | `routes.joss` outside project analysis; shared execution manifest | `analyzer/project.go`, CLI/server | compatible | low risk; early errors | web fixture project |

### Phase 4 — Runtime Safety

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| Cancellation and native errors | P1 | blocking/panics without code; context per execution and error adapters | `core`, server, mobile, runtime/errors | compatible except messages | high risk; high stability | timeout/IO/channel/panic/stack |
| Host capability | P2 | FFI/FS/network without universal declarative contract; verified host permissions | pluginruntime/core/native | experimental | high risk; high safety | rejection and authorization per resource |

### Phase 5 — Execution Architecture

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| AnalysisFacts/PreparedProgram | P2 | analysis and plans not uniformly persisted; immutable sidecar, IDs, sources | analyzer, runtime/plan, core, CLI/mobile | internal compatible | high risk; high predictability | differential prior/new path, invalidation |
| Selective VM | P3 | VM covers subset; expand only after differential corpus | `pkg/vm` | experimental | high risk; potential performance | differential, fuzz, bench |

### Phase 6 — Language Features

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| Typed channel + Result | P2 | late message/I-O errors; independent prototypes, without forced adoption | parser, typesystem, analyzer, core, docs | experimental | high risk; clear API | syntax +/-; analyzer; runtime; LSP |
| Exhaustive match | P2 | nominal cases missing; warning before error | analyzer, docs, LSP | compatible with warning | medium risk; robustness | enums/unions/default |

### Phase 7 — Tooling and Documentation

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| Correct status/documentation | P1 | pages deny existing features; update from code, mirror and translations | `docs/*.md`, mirror, translations | compatible | low risk; high confidence | contracts, navigation, docsi18n |
| Rename and traces | P2 | LSP lacks rename, debugging limited; use semantic IDs before editing references | `vscode-joss`, analyzer | compatible | medium risk; high DX | workspace edits, shadowing |

### Phase 8 — Performance

| Task | Priority | Problem / solution | Affected files | Compatibility | Risk / benefit | Required tests |
|---|---|---|---|---|---|---|
| Profile and optimize hot paths | P2 | scans/maps/forks potentially expensive; pprof first, cache with invalidation after | `core/evaluator_member.go`, runtime/plan, class metadata, plugin registry | internal compatible | medium/high risk; performance to measure | benchmem, pprof, differential, race |

### Recommendation matrix

| Proposal | Safety | Stability | Performance | DX | Complexity | Priority |
|---|---|---|---|---|---|---|
| Transaction bound to Tx | high: prevents false rollback | high | neutral | medium | medium | P0 |
| Fork closure and mobile timeout | high: prevents recycled concurrent state | high | medium due to retention | medium | high | P0 |
| Channels with Joss errors | high: prevents host panic | high | neutral | high | medium | P0 |
| CFG/null-state | high: advances failures | medium | neutral | high | medium | P1 |
| Sound collections | high: protects aliasing | high | possible check cost | medium | high | P1 |
| Verified native signatures | medium | medium | neutral | high | medium | P1 |
| PreparedProgram/IDs | medium | medium | high potential | medium | high | P2 |
| Expanded VM | low until equivalence | uncertain | high potential | low | high | P3 |

Categories describe demonstrable causality and costs in code; «potential» indicates lack of benchmark, not invented scoring. Ordering preserves semantics prior to optimization. Implementation of this roadmap is not recommended until its behavioral contracts and regression tests have been reviewed and accepted.

**Verification of this edition.** `go test ./...`, `go vet ./...`, `go build ./...`, `go run ./tools/cataloggen --check`, `go run ./tools/docgen --check`, `go test ./pkg/core -run TestDocumentation -v`, `npm run compile` and `git diff --check` completed with code 0. A temporary copy of VS Code completed `npm ci --ignore-scripts` and `npm run compile`; regular `npm ci` failed with `EPERM spawn` both in the checkout and in a clean copy, so its installation scripts remained unvalidated. Go printed host warnings due to telemetry/cache ACLs without impacting approved commands. `go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core` could not build `runtime/race` on this host (`package testmain: cannot find package`); it is not presented as approved. `go run ./tools/docsi18n -check` reported pre-existing outdated translations and mirrors and, following this edition, en/pt versions of the report pending translation. The Spanish JosSecurity mirror was synchronized byte for byte; full translation remains as a documentation task, without semantic changes.

# Core technical audit and alignment with thesis (August 2026)

[Index](README.md)

## Baseline

Before the changes, `go test ./...` and `go build ./...` passed; `go vet ./...` failed due to manually constructing an SMTP address incompatible with IPv6. `joss analyze` on JosSecurity produced 10 false errors and 14 warnings without a file: two built-in names implemented but absent from the catalog (`html_escape`, `unlink`) and seven uses of classes exported by plugins that the analyzer did not consult (`BrevoClient`, `Notify`).

## Root causes found

- Global analyzer based on `map[string]int`, without scopes, types or source unit.
- CLI concatenated ASTs and removed file identity.
- Catalog of built-ins divergent from the dispatcher: it included non-existent functions and omitted real functions.
- Native classes and plugins were resolved in different ways.
- `let $name` mistakenly constructed a symbol called `$`.
- `VarTypes` only protected typed statements; first assignments and parameters lost their type.
- `CallMethod` and `CallMethodEvaluated` duplicated binding and validation.
- The editor maintained separate manual lists of keywords and native classes/methods.
- CI only existed for manual distribution; there was no push/PR workflow.
- `go vet` revealed the use of `fmt.Sprintf("%s:%s")` for SMTP instead of `net.JoinHostPort`.
- The pool preserved `PluginRegistry` while deleting exposed symbols, so a reuse could skip the plugin reload.

## Decisions applied

- New and consumed layers: `pkg/typesystem`, `pkg/diagnostics`, `pkg/analyzer`.
- Scopes for callable and declaration resolution at the project level.
- Fixed inference on first assignment; `var` inferred; `let $x` explicit dynamic.
- Structured and deterministic diagnostics per file/line/column.
- Analyzer environment adapted from real runtime logs and JP v2 symbols.
- Catalog generated for VS Code, validated by CI.
- The publisher's rich signatures are filtered against that catalogue; Outdated metadata can no longer publish symbols that the runtime does not register.
- Unified method binding in `CallMethodEvaluated`.
- Isolated invocation frames, direct/mutual/method recursion, depth limit and optional return contracts.
- Lexical frames without dynamic scope: named functions do not inherit localities from the caller; Closures retain their capture.
- Joins `T|U`, nullable `T?`, provable exhaustive returns, and explicit return signatures for the entire native kernel.
- `const` and typed/constant properties validated by analyzer and runtime.
- Parser errors stored directly as structured diagnostics; removing lines from strings was removed.
- Physical elimination of `ImportStatement`, import tables/tokens, plugin textual linker and uncompressed bytecode format.
- Removal of compatibility APIs without canonical consumers: raw routes, inserts for arrays, Schema for maps and `where(..., "json")`.
- Complete reset of plugin registration when returning a runtime to the pool.
- Integration test especially JosSecurity.
- Project fixture versioned in `testdata/analyzer-project`; JosSecurity remains an ignored external repository and its test is skipped only when it is not available.

## Result in JosSecurity

By eliminating the causes of false positives, six previously hidden real problems appeared:

- `Math::length` does not exist; was replaced by `count`.
- `Str::endsWith` does not exist; was replaced by `str_ends_with`.
- `Str::upper` does not exist; was replaced by `strtoupper`.
- Three models inherited from the removed class `GranMySQL`; now they inherit from `GranDB`.

A second review detected five false warnings: model variables with the same name as their class were confused with static access and were not marked as used as they were recipients of `->`. Fixed precedence so that the lexical symbol shadows the class and added a regression.

The end result is zero errors and five warnings, all inspected against the code: `$id`, `$licenseData`, `$offset`, `$user` and `$domain` are initialized but not read back into their respective callables. They are real debt to JosSecurity, they do not block execution, and they were not modified just to get empty output.

## Comparison with the thesis

The implementation matches the ALIM vision in the integrated runtime, Pratt parser, shared AST, isolated plugins and single toolchain. The thesis, however, mixes real state, conceptual syntax and roadmap:

| Thesis statement | Checked repository status |
|---|---|
| Pipeline includes type checker | There is an initial semantic checker with fixed inference, constants, signatures/returns and recursive calls; it does not yet cover taint, escaping or DB schemas.

|
| AOT/LLVM/Cranelift and machine code | The main build packages serialized AST and the Go interpreter. LLVM/Cranelift are not implemented.

|
| Lexer/parser in Rust | The current implementation is in Go.

|
| Immutability by default/ownership | There is no ownership semantics or immutability by default.

|
| Imports/modules with network and cycles | The historical syntax was removed completely and will not return. Plugins and conventional files are loaded automatically; Joss deliberately adopts a zero-imports project.

|
| Routes/DBs as first order AST nodes | Today they are calls to native classes (`Router`, `GranDB`), not specific nodes.

|
| 1,420 tests and 91.4% coverage | The repository contains a much smaller Go suite. Current focused measurement: parser 52.3%, typesystem 44.9%, analyzer 47.7% and core 14.8%; CI validates execution and does not assert non-existent coverage.

|
| Taint analysis and 83% of vulnerabilities | There is a heuristic security analyzer in the LSP, not a formal taint engine in the compiler.

|

These differences were not “corrected” by inventing features. The proposal for source modules in chapter 11 is expressly ruled out for Joss; ALIM modularity is preserved through runtime components and isolated plugins. The rest must be resolved in the thesis, distinguishing validated implementation, conceptual syntax and future work.

## Incremental architectural audit — September 11, 2026

The revision of commit 4b41742 used size only as a signal and contrasted responsibilities, dependencies, shared state, and duplicate knowledge. The baseline passed except pkg/pluginpkg/TestLoadOrCreateSigningKey, which attempted to write outside of the sandbox to C:\Users\Asus\.joss\keys.

### Prioritized map

| Priority | Archive | Lines approx.

| Diagnosis | Action |
|---|---|---:|---|---|
| P0 | cmd/joss/pub_cli.go | 1169 | credentials, HTTP, resolution, cache, ZIP, YAML, lockfile and publishing | Extract client, resolver, installer and storage |
| P0 | pkg/analyzer/infer.go | 1075 | inference, calls, members, access, hierarchy and narrowing | Extract nominal resolver and call validation |
| P0 | pkg/analyzer/analyzer.go | 927 | symbols, nominal contracts, statements and diagnostics | Separate collector, contracts and bodies |
| P0 | pkg/server/handler.go | 1042 | runtime, CORS, WebSocket, request, session, CSRF and response | God function; progressive extraction started |
| P0 | pkg/core/evaluator_infix.go | 772 | ternary, streams, pipeline, arithmetic, postfix and match | Separate flow, arithmetic and increments |
| P0 | pkg/core/runtime.go | 691 | pool, lifecycle, fork, env, DB, instances and preload | Extract lifecycle, config and loader |
| P0 | pkg/core/evaluator_call.go | 550 | binding, frames, references, host calls and built-ins | Separate binder/frame from dispatch |
| P1 | pkg/parser/parser_statements.go | 1027 | declarations, nominal types, control, exceptions and select | Extract by grammatical domains |
| P1 | pkg/parser/parser_expressions.go | 1088 | Pratt, literals, interpolation, collections, calls and match | Extracted interpolation; continue for collections/callables |
| P1 | pkg/core/builtins_array.go | 681 | conversions, collections and higher-order | Separate conversions and algorithms |
| P1 | pkg/core/native_seo.go | 588 | SEO and Sitemap | Separate aggregates |
| P1 | pkg/core/view.go | 614 | eval, directives, sections and shared state | Extract template compiler |
| Maintain | pkg/parser/ast_statements.go | 460 | declarative AST model | Big but cohesive |
| Maintain | pkg/typesystem/types.go | 402 | types, parse, assignability, inference and coercion | Cohesive canonical source |
| Maintain | pkg/runtime/plan/callable.go | 443 | callable plans | Specialized |

### Sources of truth

| Concept | Before | Result/proposal |
|---|---|---|
| Keywords | Already canonical in parser/token.go | Maintain KeywordNames |
| Symbols | switch lexer + multichar list + switch formatter | Canonical SymbolDefinition |
| Numerical types | typesystem + analyzer.isNumeric | Type.IsNumeric |
| Built-ins | list + six return sets + five dispatchers | descriptor name/domain/return |
| global JSON | duplicate in string and IO | builtins_serialization.go |
| Native methods | separate names and returns | Pending: NativeMethodDefinition |
| Primitive methods | analyzer and core/primitives | Pending: minimum catalog of signatures |
| @json directive | regex in linter and view | Pending: shared compiler |
| VM | parallel experimental semantics | Require differential suite before integrating it |

### Architecture and applied refactor

The lexicon remains in parser, the types in typesystem, the semantics in analyzer and the native execution/surface in core. A global language package is not created: centralizing all domains there would increase coupling and risk of cycles.

1. parser/token.go defines symbols by maximal munch; lexer and formatter consume the same projection.
2. parser_interpolation.go encapsulates AST segmentation and composition without new public API.
3. Type.IsNumeric overrides the analyzer's local sorting.
4. core/builtins.go declares name, domain and return once; analyzer and runtime consume the descriptor.
5. builtins_serialization.go removes the two JSON implementations.
6. server/rate_limiter.go and request_data.go isolate state and the HTTP→Joss border.

| Area | Before | After |
|---|---|---|
| Symbols | 3 representations | 1 registration + screenings |
| Lexer | 445 lines | 218 |
| Parser expressions | 1088 with interpolation | 958 + cohesive module of 129 |
| Built-ins | list + 6 sets + cascade | single descriptor + direct dispatch |
| JSON | 2 implementations | 1 |
| HTTP handler | 1042 | 925 + cohesive modules of 51 and 100 |
| Guards | partial | exhaustive tests of symbols and handlers |

The total number of lines was not the objective: tests and contracts were added. Now a symbol changes into a definition and a built-in into a descriptor plus its domain handler.

### Remaining risks

- MainHandler still contains session/CSRF, WebSocket and response writing; requires HTTP characterization tests before extracting.
- Native classes still separate method names and precise returns; must be migrated class by class.
- infer.go and analyzer.go remain semantic risks and require nominal resolution, refs and narrowing tests during their split.
- Formatter needs trivia; symbol sharing is fine, replacing it with the lexer that discards trivia would not be.
- pub_cli.go needs filesystem, network and lockfile characterization before splitting.
- Built-in aliases are publicly supported; They should not be eliminated by superficial duplication.
- VM and JPBC should not be promoted without differential tests.

## Second phase of refactoring — September 2026

### Revalidation and order

The audit was revalidated against the current tree before modifying it. The seven P0s were still active: `pub_cli.go` 1312 lines, `infer.go` 1101, `analyzer.go` 964, `handler.go` 1024, `evaluator_infix.go` 835, `runtime.go` 762 and `evaluator_call.go` 601. The reference coverage was analyzer 57.7%, core 47.9%, server 8.0% and cmd/joss 16.2%.

The order chosen was calls → operators → inference. Calls had good characterization and an already unified semantic route; operators could be frozen with differential tests by domain; inference required preserving the typesystem/analyzer boundary. Runtime lifecycle, HTTP, and publishing remain afterward because their external state requires specific fixtures before extracting.

### Changes applied

​​| Area | Previous status | Resulting architecture |
|---|---|---|
| Invocation | `evaluator_call.go` mixed argument evaluation, binding, frames, callable kinds and built-ins | `call_arguments.go` has evaluation/binding/ref; `call_method.go` has frame, return and recursion; `callable_dispatch.go` has callable types; `evaluator_call.go` coordinates source resolution and built-ins |
| Operators | `evaluateInfix` mixed control, pipeline, numbers, streams, ranges, prefix/postfix and match; It also contained a second unreachable pipeline | `evaluator_control.go`, `evaluator_pipeline.go`, `evaluator_numeric.go`, `evaluator_stream.go` and `evaluator_update.go`; `evaluator_infix.go` preserves the order of evaluation and short-circuit |
| Nominal inference | `infer.go` contained nominal assignability, hierarchy and narrowing along with base inference | `infer_nominal.go` has project hierarchy/assignability; `infer_narrowing.go` has scope refinement; `infer.go` preserves the inference entry and still contains pending call/member resolution |
| Primitive methods | duplicate names and returns in analyzer; implementation listed separately in core | `typesystem.PrimitiveMethodDefinition` is the canonical metadata; analyzer and runtime consult it and a test requires runtime implementation for each definition |
| Extension | `js-yaml` 4.3.1 (high) and `qs` 6.15.3 (moderate), both transitive of development tools | compatible overrides to 4.3.2 and 6.16.0; `npm audit` becomes zero without `--force` |

The divergence `array.pop` was real: the analyzer published element return, but the array runtime did not implement the method. It was removed from shared metadata without inventing mutable semantics. If it is designed later, it must be added as an explicit decision with implementation and tests.

### Characterization and regressions

- `call_characterization_test.go` freezes named/positional/default binding, missing/unknown errors and structured return contracts. Existing suites cover multilevel refs, closures, recursion, inheritance, and frame recycling.
- `infix_characterization_test.go` freezes pipeline, Elvis/ternary, strict match, comparisons, exact decimal, ranges and postfix.
- `primitive_methods_architecture_test.go` prevents advertising metadata without runtime implementation; `primitive_methods_test.go` validates receiver-dependent returns.
- The suite detected a negative capacity in ascending ranks during the extraction. It was a regression from the refactor, it was fixed before continuing and the documentation contracts passed again.

### Comparison and Change Surface

| Metric | Before | After |
|---|---:|---:|
| `evaluator_call.go` | 601 lines / 6 responsibilities | 91 coordinating lines + 3 modules of 143/216/158 lines |
| `evaluator_infix.go` | 835 lines / 7 domains | 120 coordinating lines + 5 modules per domain of 58–269 lines |
| `infer.go` | 1101 lines / inference + nominal + narrowing | 907 lines; nominal 86 and narrowing 65; call/member resolution still pending |
| Pipeline `|>` | 2 deployments, one dead | 1 canonical implementation |
| Primitive signatures | analyzer + runtime switches without common guard | 1 metadata + runtime implementations protected by test |

Approximate area of ​​change: adding a primitive method goes from editing analyzer and runtime without checking (2 places prone to divergence) to editing metadata and implementation (2 places necessary) with automatic projection to the analyzer and parity test. Adding an operator still requires parser/analyzer/runtime and, if applicable, VM; Behavior that belongs to different layers was not centralized. Adding a built-in continues to require a domain descriptor and handler. Adding a nominal analyzer rule is located in `infer_nominal.go`.

### Reordered debt

- **P0:** `analyzer.go`; call/member resolution remaining in `infer.go`; lifecycle/pool in `runtime.go`; `MainHandler`; `pub_cli.go`. None are considered resolved by size.
- **P1:** metadata `NativeMethodDefinition` class by class; shared view directive compiler; HTTP characterization and pooling; remaining parser statements/expressions.
- **P2:** VM/interpreter differential suite before promoting the VM; publish an array of primitive methods when a reliable contract exists.

Rule for next iteration: don't pull lifecycle, HTTP or log/publish until your tests freeze reset/reuse/fork, CORS/session/CSRF/WebSocket/status and cache/archive/lockfile respectively.

Later focused coverage: analyzer 59.1%, core 48.2%, server 8.0% and cmd/joss 16.2%. The improvement is small because the new tests prioritize critical contracts; low server/CLI coverage confirms that they should not be refactored yet without additional characterization.

## Pending risks

- Parser recovery can produce derived diagnostics after the first invalid token, although now they all use the structured model and preserve column without extracting it from messages.
- Native returns are explicit, but many APIs retain variadic parameters until publishing reliable arity contracts.
- There is no sensitive refinement to branches, infrastructure contracts, taint or formal escape. Module cycles do not apply because there are no source modules.
- `pkg/core` remains broad; Splitting infrastructure subsystems requires specific testing and was not done just for aesthetics.
- The LSP rich signature catalog is manual metadata; The existence of names does come from the generated catalog.

## Third architecture phase — September 2026

This phase prioritized stabilizing the analyzer and building characterization before extracting infrastructure.

### Analyzer

`Analyzer.Analyze` now expresses the pipeline `collectDeclarations → projectScope → validateNominalContracts → analyzeSourceBodies → diagnostics`. The façade retains its project status; statement collection, nominal contracts, body analysis, call resolution, and member resolution live in files in the same package (`declaration_collection.go`, `nominal_contracts.go`, `body_analysis.go`, `call_resolution.go`, `member_resolution.go`). No public managers were created nor was a dependency on `core` introduced. The file/class/return cursor is restored by callable, avoiding contamination between units.

Calls go through a semantic signature of `Callable` and a unique validation of arity, names, references and compatibility; Global, native, primitive, and plugin methods are projected to that representation without sharing a runtime implementation. Member resolution distinguishes receiver, visibility, hierarchy, fields and methods. The common representation is metadata: analyzer and runtime do not share execution state.

### Pre-infrastructure characterization

Added lifecycle and pool tests (`runtime_lifecycle_characterization_test.go`) that fix creation, fork, reset and reuse. The test revealed and corrected actual contamination of `Env`, metadata caches, current source, generators and defers in `Runtime.Free`; `DB` remains external share and is not closed from `Free`. The state is classified as persistent configuration, per-runtime, per-execution or resettable cache.

The HTTP border has absent runtime characterization, 503, CORS, sessions/CSRF, and mapping to a Joss response (`handler_characterization_test.go`). WebSocket and unsupported response cases are still explicitly pending so as not to invent semantics. CLI/pub incorporates temporary fixtures and in-memory registry server for secure ZIP extraction, traversal rejection, deterministic lockfile, manifests and error resolution (`cmd/joss/pub_cli_test.go`). This allows refactoring later without confusing infrastructure changes with language changes.

### Directives and metadata

The inventory confirms that `@json` is an isolated view transformation together with `@extends`/`@section`; linter and view still interpret in different ways. Remains as debt P1: a policy representation must first be defined, not a regex shared. `NativeMethodDefinition` P1 also remains: classes keep separate names and returns and no invented arities are published for variadic APIs.

### Change Surface and metrics

| Change | Before | Phase 3 status | Protection |
|---|---:|---:|---|
| analyzer rule | `infer.go` + implicit state | explicit phase and files by concept | `semantic_pipeline_test.go` |
| lifecycle/reset runtime | without full pool contract | state reset by centralized execution | `runtime_lifecycle_characterization_test.go` |
| HTTP response | monolithic handler without board | characterized border, deferred extraction | `handler_characterization_test.go` |
| publication/ZIP/lock | external effects without fixtures | temporary fixtures and registry `httptest` | `pub_cli_test.go` |

Fan-in/out is still observed at the package level: `analyzer` consumes parser/typesystem/diagnostics but not core; `core` maintains the highest fan-out for integrating language and host; server and cmd depend on core/adapters. These numbers are not used as gates. The remaining semantic duplication is intentional (static validation vs. execution) and shares types/metadata, not effects code.

### Remaining debt

- **P0:** `MainHandler` and `Runtime` still coordinate multiple domains; They should be extracted only after extending WebSocket, response mapping, and concurrent contamination.
- **P1:** `NativeMethodDefinition`, full primitive signatures, template directive compiler, unique named/ref argument normalization, and automated fan-in/change surface metrics.
- **P2:** Interpreter/VM differential suite, selective ZIP/lock/template fuzzing and comparable inference/calls benchmarks.

## Fourth architecture phase — September 2026

### Revalidation and reproducible environment

The current P0s were lifecycle/fork of `Runtime`, `MainHandler` and `pub_cli.go`; The P1s were native metadata, arguments and directives. The focused baseline was analyzer 60.3%, core 48.3%, server 29.7% and cmd/joss 20.4% after this phase. `pluginpkg`'s fault was not with the product: his test wrote in the real HOME; now redefines `USERPROFILE`/`HOME` to `t.TempDir()`. The Go global cache has bad ACLs on this host; The reproducible workaround is to assign `GOCACHE` to a temporary directory. `npm ci` inside the checkout it collides with files opened by VS Code; a temporary copy without `node_modules`, with cache/network enabled, executed `npm ci`, `npm run compile` and `npm audit` (0 vulnerabilities).

### Runtime State Model

| Field | Responsibility | Lifetime | Forks | Free | Shared / owner |
|---|---|---|---|---|---|
| `Env` | effective configuration | runtime configuration | copy | clean | runtime |
| `Variables` | bindings and globals | per-execution | copy with known clones | clean + standard bindings | runtime/frame |
| `VarTypes` | runtime types | per-execution | copy | clean | runtime |
| `Constants` | constancy | per-execution | copy | clean | runtime |
| `HostGlobals` | host visibility | runtime lifecycle | copy | rebuilds | runtime |
| `Classes` | native classes/project | runtime configuration | map copy, shared immutable AST | clean | runtime/project |
| `Interfaces` | project contracts | runtime configuration | map copy | clean | runtime/project |
| `Enums` | project enums | runtime configuration | map copy | clean | runtime/project |
| `Functions` | project functions | runtime configuration | map copy, shared AST | clean | runtime/project |
| `DB` | pool SQL | external resource | share | survives, does not close | application/`database/sql`thread-safe |
| `Routes` | HTTP table | runtime configuration | copy on two levels | clean | runtime |
| `CurrentMiddleware` | stack when registering routes | per-execution | restart | clean | runtime |
| `CustomMiddlewares` | callable middleware | runtime configuration | copy | clean | runtime |
| `NativeHandlers` | native implementation | global immutable after registration | copy | clean/rebuild upon purchase | core |
| `NativePlugins` | payloads plugin | runtime configuration | map copy; definitions treated as immutable | clean | runtime/plugin loader |
| `NativeDrivers` | native handles | external resource | share definitions/handles | clean reference, no download handle | loader |
| `PluginRegistry` | symbols/instances plugin | runtime lifecycle | explicitly shares with parent | remove reference | parent runtime/pluginruntime |
| `ProjectRoot` | project root | runtime configuration | copy | clean | runtime |
| `SEO` | response accumulator | per-request | restart | clean | runtime request |
| `SitemapEntries` | sitemap configuration | runtime configuration | copy slice | clean | runtime |
| `SitemapProviders` | sitemap closures | runtime configuration | copy slice; shared closures | clean | runtime |
| `SitemapExclusions` | sitemap configuration | runtime configuration | copy slice | clean | runtime |
| `CurrentSource` | source cursor | per-execution | restart | clean | evaluator |
| `CurrentFile` | cursor file | per-execution | restart | clean | evaluator |
| `MaxCallDepth` | limit | runtime configuration | copy | default | runtime |
| `callDepth` | current depth | per-execution | restart | clean | evaluator |
| `currentClass` | nominal cursor | per-execution | restart | clean | evaluator |
| `callStack` | diagnostic stack | per-execution | restart | clean | evaluator |
| `callablePlans` | method plans | cache | map copy; immutable plans | clean | runtime |
| `functionPlans` | closure plans | cache | map copy | clean | runtime |
| `classMetadataCache` | derived metadata | cache | restart | clean | runtime; protected by `planMu` |
| `currentFrame` | active frame | per-execution | restart | clean | evaluator |
| `planMu` | cache protection | runtime lifecycle | new mutex | remains | runtime |
| `captureEnvironment` | temporary capture | per-execution | restart | clean | evaluator |
| `cinReader`, `cinTokens` | tokenized entry | per-execution | restart | clean | IO runtime |
| `currentGenerator`, `generatorIndex` | cursor generator | per-execution | restart | clean | evaluator |
| `topDefers` | top-level defers | per-execution | restart | clean | evaluator |

Invariants: after `Free` no request, execution, cursors or caches survive; host bindings are rebuilt; `DB` does not close because `Runtime` is not its owner. `Fork` Share only AST/plans/configuration treated as immutable, plugin registry and external resources; mutable request maps are isolated. The tests detected that `Fork` omitted `Enums`; was corrected. The pool supports concurrent acquire/free under race detector.

`runtime_lifecycle.go` contains construction, pool, acquisition, reset and invariants. `runtime.go` preserves environment/loading and declaration registration. `Runtime` is still coordinator; no managers were introduced.

### HTTP Lifecycle and response mapping

The request runtime is released by a single `defer` immediately after `Fork`, including rate limiting, CORS, session/storage, panic, and WebSocket. `response_writer.go` is the Joss→HTTP boundary.

| Result | Status/Content-Type | Conduct |
|---|---|---|
| `string` | 200, default HTML | write text and hot reload HTML |
| map/Instance `JSON` | `status_code` or 200, JSON | encode of `data` |
| map/Instance `RAW` | `status_code` or 200, configurable | string/bytes/textual representation + headers |
| Instance `FILE` | 200, MIME detected | attachment or 500 if not read |
| Instance `STREAM` | 200, SSE | flush and callback with Stream |
| map/Instance `REDIRECT` | 302/map or `status_code`/Instance | Location; Instance applies cookies/flash |
| int/float/bool/null/array/map without `_type` | no published representation | continue to public-file/404 |

The characterization fixed a bug: `Response::json(..., status)` stores `status_code`, but the handler read `status`, returning 200. WebSocket covers invalid upgrade and valid upgrade/close without path; full message callbacks continue P1.

### Metadata and signatures

`NativeMethodDefinition` separates name, return, optional parameters, `ArityKnown` and implementation variety `NativeHandler`. Stack, Queue and Math are the first migration; their names/returns are projected to the AST runtime, analyzer, LSP catalog and documentation. The arity remains unknown when the handler tolerates dynamic inputs. Parity tests prevent announcing methods without a handler. The other classes retain the legacy adapter until migrating class by class.

Static normalization creates a single parameter→argument relationship for positional, named, defaults, ref, unknown, duplicate, and missing. Runtime retains its binder, but both consume the same semantic parameters and reject duplicates; analyzer does not execute runtime code.

### Templates, CLI and differential semantics

The view inventory includes `@extends`, `@section`/`@endsection`, `@yield`, `@include`, `@json`, and `@foreach`/`@endforeach`. `pkg/viewtemplate` provides minimal scanning with ranges, quotes and nested parentheses. Runtime and linter consume `RewriteJSON`; They no longer maintain distinct regex. Inheritance/sections/includes/foreach retain their current consumers until progressive migration. There is fuzz target for determinism/no-panic/ranges.

CLI/pub uses an internal HTTP client with timeout, configurable cache for tests, registry `httptest`, secure ZIP and temporaries. Added 401/403/429/500, truncated/invalid JSON, timeout, cache hit/stale and missing partials. The ZIP fuzz target verifies that it is not written outside the destination.

The first differential corpus declares only integer arithmetic/comparison, local assignment and integer prefix. Analyzer must accept, Interpreter and VM must produce the same value. VM remains experimental and any new features must be explicitly incorporated into the inventory, not inferred as supported.

| Features | Analyzer | Interpreter | LSP | VM | State |
|---|---|---|---|---|---|
| integers/basic arithmetic | canonical types | published reference | syntax catalog | green differential | supported in corpus |
| calls/methods/closures | semantic signatures | published reference | partial projection | not comparable | pending VM |
| natives | metadata projection | handlers | generated catalog | not supported | progressive metadata |
| templates | validate compiled script | render | snippets/diagnostics | not applicable | partial common scanner |
| match/pipeline/references | active analyzer | published reference | syntax | not full parity | P2 differential |

Negative guards verify that parser/typesystem/diagnostics/analyzer do not import core. The generated JSON catalog contains `_generated: DO NOT EDIT` and is still validated by `cataloggen --check`.

### Performance baseline

Windows/amd64 i5-10300H, `-benchtime=100ms`: simple call 794 ns/op, nested 2936 ns/op, ref 1496 ns/op, closure 973 ns/op; frame recycling 1213 ns/op, 24 B/op, 2 allocs/op. Fixture startup analyzer: 10095 ns/op, 7315 B/op, 69 allocs/op. Lifecycle: construct 235536 ns/op, pooled acquire/free 289634 ns/op and fork/free 26039 ns/op. The pool includes recharge/autoload; It was not optimized without outlining that responsibility.

### Reordered debt (Fourth phase)

- **P0:** complete isolation/ownership of plugin registries and drivers before mutable concurrency; complete WebSocket callbacks and backend session errors.
- **P1:** migrate NativeMethodDefinition class by class; extract session/CSRF from MainHandler; separate registry/cache/manifest/publisher from pub CLI; migrate the rest of the directives to the scanner.
- **P2:** expand corpus Analyzer↔Interpreter↔VM, LSP signature catalog, type parser fuzz and lockfile round-trip.
- **P3:** automate fan-in/change surface and establish comparable historical benchmarks in CI.

---

## Fifth architecture phase — September 2026

The fifth phase advances critical ownership, concurrency, session, plugin, and WebSocket debt. The guarantees below distinguish verified behavior from work that remains pending.

### 1. Ownership and Concurrency of Plugins and Native Drivers (P0-A)

- **Isolation by Runtime (`PluginAwareHost`)**: The `PluginAwareHost` interface was introduced in `pkg/pluginruntime` to decouple the global package registration from the specific execution. `Runtime` implements this interface by maintaining its own AST engines (`pluginASTEngines map[string]*PluginASTEngine`) and namespaces (`PluginNamespace`).
- **Fork Semantics**: When executing `Runtime.Fork()`, the engine facades are duplicated linked to the new child runtime, safely sharing the plugin's immutable AST while fully isolating the evaluated state and local frames. Two different plugins with identical functions (e.g. `run()`) no longer collide with each other or cross request scopes.
- **Counted driver ownership**: The loading runtime owns each `NativeDriverDefinition`; forks retain the handle and `Free()` releases that share. The last owner invokes `FreeLibrary`/`dlclose`. `Unload()` is idempotent, serialized with active calls, and rejects unloading while borrower runtimes still exist.

| Component | Sharing Level | Life Cycle | Concurrent Policy |
|---|---|---|---|
| Plugin AST (`*parser.Program`) | Immutable / Shared | Process | Read-only thread-safe |
| `PluginASTEngine` | By `Runtime` / Instance | Request/Fork | No contention between threads |
| `PluginNamespace` | By `Runtime` / Instance | Request/Fork | Instances cloned in `Fork()` |
| `NativeDriverDefinition.handle` | Shared with counted ownership | Last owner or explicit `Unload()` | Protected by `Mu`; calls and unloading are mutually exclusive |

### 2. Complete Lifecycle and WebSocket Callbacks (P0-B)

- **Implemented callbacks**: WebSocket connections execute `onMessage` and `onClose`, with panic recovery and idempotent cleanup. `onConnect` and `onError` remain pending.
- **Parameter propagation**: The setup handler receives the socket followed by route parameters as positional arguments. A `$params` binding does not exist yet.
- **Multi-connection isolation**: Concurrent connections operate on independent sockets and runtimes, without filtering frames or lexical state between clients.

### 3. Flash Redirect and Session Storage Contracts (P0-C)

- **Unified persistence**: Flash serialization in HTTP redirects (`persistRedirectFlash` in `response_writer.go`) was unified under the canonical contract `saveSession(sessionID, store)`.
- **Strict error handling**: If the session backend fails during a redirect, the response immediately aborts with HTTP 500 and an explicit error message, suppressing the `Location` header to prevent the client from following a redirect with corrupted or non-persisted flash or session data.
- **Backends and isolation**: Only `memory`, `file`, and `redis` are valid configurations. Snapshots recursively clone JSON maps and slices so that a request cannot modify nested persisted state before `saveSession`.

### 4. Double Release Protection (`sync.Pool`)

- **Defense against double-free**: `Runtime.Free()` uses `atomic.Bool.CompareAndSwap`; only the caller that performs the active-to-released transition may clean and return the pointer to the pool. A concurrent regression test verifies that the pool cannot hand the same instance to two owners. That test also exposed a race in `AssetManager.Initialize`, which is now serialized.

### 5. Native Metadata Expansion (`NativeMethodDefinition`)

Ten classes were migrated to `NativeMethodDefinition` with verified names and return types. None publishes exact arity yet (`ArityKnown=false`):
- `Stack`: `push`, `pop`, `peek`
- `Queue`: `enqueue`, `dequeue`, `peek`
- `Math`: `random`, `floor`, `ceil`, `abs`
- `JSON`: `parse`, `stringify`, `decode`, `encode`
- `Markdown`: `toHtml`, `readFile`
- `Str`: `length`, `random`, `startsWith`, `substring`, `indexOf`, `contains`, `trim`, `replace`
- `UUID`: `generate`, `v4`
- `Lang`: `get`, `set`, `locale`, `locales`
- `Console`: `green`, `red`, `yellow`, `blue`, `cyan`, `magenta`, `gray`, `bold`, `clear`, `color`, `log`
- `Zip`: `extract`

The definitions are consumed by the analyzer, the catalog generator (`tools/cataloggen`), the documentation generator (`tools/docgen`) and the language server (LSP).

### 6. View Templates and Column Counting by UTF-8 Runes

- **Rune-aware source positions**: In `pkg/viewtemplate/directives.go`, the calculation of columns for diagnostics was adjusted to iterate by UTF-8 runes (`utf8.RuneCountInString`), correcting the offset in accented or multibyte characters and guaranteeing exact alignment of ranges with the parser and LSP.
- **Fuzz testing**: Continuous fuzzing on the ZIP extractor (`FuzzExtractPluginZip`) and the policy scanner (`FuzzDirectiveScanner`).

### Reordered debt (Fifth phase)

- **P0:** No known P0 issues remain in this batch. Double-free protection, driver ownership, session backends, and the published WebSocket callbacks (`onMessage`/`onClose`) have focused contract and race tests.
- **P1:** Explicitly decide whether `onConnect`, `onError`, or `$params` should extend the WebSocket API; formalize freezing of AST catalogs before serving requests.
- **P1:** Continue progressive migration of the remaining native classes (`GranDB`, `Crypto`, `File`, `Http`, `Router`, etc.) to `NativeMethodDefinition`; extract session/CSRF from `MainHandler` to dedicated submodules; migrate remaining policies (`@extends`, `@section`, `@yield`, `@include`, `@foreach`) to the `pkg/viewtemplate` unified scanner.
- **P2:** Extend Analyzer↔Interpreter↔VM differential corpus for complex types and closures; expand type parser fuzzing and lockfile round-trip.
- **P3:** Automate fan-in/fan-out and change surface metrics in CI; establish comparable performance benchmarks.

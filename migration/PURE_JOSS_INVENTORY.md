# Pure Joss migration inventory

This is the working inventory for moving the Joss toolchain to a Joss-owned
implementation. It records the repository state inspected on 2026-11-11; source
and executable tests remain authoritative.

## Phase tracking

| Phase | State | Evidence / next action |
|---|---|---|
| Inventory and contracts | Drafted; architecture contract suite passes, full repository validation pending | `go test ./tests/architecture -count=1` passes; focused stdlib/docs tests, `docsi18n --check` (`50 files x 3 locales`) and `git diff --check` pass (Git prints a non-fatal fsmonitor IPC warning). `go test ./...` is not clean in this sandbox: local listener tests are denied permission to connect to `127.0.0.1`, and a core listener test waited until the 11-minute timeout. Full validation must be rerun in an environment that allows loopback tests; `go vet ./...`/`go build ./...` remain outstanding. |
| Joss runtime and Standard Library | In progress: HTTP request parsing, session handoff and first Auth policy methods | Form/query/cookie/JSON parsing is canonical in `std/http/request.joss`; both `HttpServer::handle` and the interpreter dispatcher invoke it, and the duplicate Go HTTP decoder/JSON merge have been removed. Tests pass for form/query/cookies, JSON fields, malformed input and preserved raw JSON body. `Session` emits/accepts canonical `joss_session` while retaining uppercase compatibility. `Auth::id`, `Auth::check`, `Auth::guest`, `Auth::logout` and the role decision in `Auth::hasRole` now have Joss source implementations. Interpreter tests cover cookie-hydrated identity, guest state, session cleanup and matching-role/admin behavior. A host AOT differential calls `hasRole` only with no session and is not evidence that authenticated roles propagate into AOT: AMD64 `Session::get` is currently a stub and AArch64 has no session getter implementation. Session identity/role transport therefore remains a native boundary and a parity gap. Standard-library class declarations preserve unshadowed native signatures in the analyzer. Auth source emits for all six primary OS/architecture combinations, but this is compile evidence only. The individual `explode` Windows AMD64 crash from mutating read-only string literals is fixed by splitting a copy, and its host differential passes. AArch64 `strlen` no longer aliases array length; structural emitter checks cover Linux, Windows and macOS ARM64, but ARM64 execution is not available here. `strpos`, `substr` and `urldecode` remain unresolved, so form parser AOT parity still fails; cross-target compilation is not execution evidence. Session persistence/lifecycle, CSRF enforcement and remaining Auth flows are open. |
| Compiler and native backend | Partial foundation; not a Joss-owned compiler | Canonical IR and Go emitters now have global load/store operations for static properties; `map<K,V>` lowers as a native map pointer. The JIT assigns global offsets and keeps them outside the Windows IAT reservation. Differential execution passes on the host target, and the comprehensive format/emitter test now compiles a real Joss static property on Linux, Windows and macOS AMD64/ARM64 plus Android ARM64. These checks validate emitted artifacts, not execution on all six primary OS/architecture combinations. Joss-owned lowering/emission and cross-target runtime parity remain unstarted. |
| CLI, LSP and generators | Not started | No Joss-owned replacements inventoried beyond this table. |
| Joss bootstrap and Go/ASM retirement | Not started | Requires a reproducible self-hosted compiler and complete target matrix. |

## Ownership rules

| Area | Canonical implementation target | Allowed boundary | Parity gate |
|---|---|---|---|
| Syntax, types, diagnostics | Joss source/compiler semantics | OS file/process primitives | parser, analyzer, runtime and differential tests |
| Interpreter/runtime | Joss Standard Library | memory allocation, clocks, secure random/crypto, OS APIs | `joss run` vs AOT |
| HTTP, sessions, Auth | `std/http`, `std/auth` | sockets, database, crypto, email/OAuth transport | request lifecycle and failure-injection tests |
| IR/lowering/optimization | Joss compiler sources | bootstrap only during transition | verifier, differential and feature coverage gates |
| Machine-code backend | Joss source emitting bytes | temporary ABI/startup stubs | AMD64/ARM64 x Linux/Windows/macOS |
| CLI, LSP, generators | Joss toolchain sources | bootstrap launcher during transition | command/LSP/generator contract tests |
| Drivers and external integrations | Joss policy and APIs | minimal native driver/transport operations | driver-specific error and capability tests |

Go and handwritten assembly are transitional implementation languages, not
semantic authorities. A subsystem is removable from Go/ASM only after its Joss
implementation passes the parity gate for every supported execution mode and
target.

## Single-source implementation rule

Every behavior expressible in Joss is implemented once in canonical Joss source
and reused by the interpreter, JIT and AOT through the same `PreparedProgram`
and IR. Do not add a second semantic implementation to a Go host bridge or an
architecture-specific machine-code emitter. Extend common IR only when Joss
cannot express a required operation; keep such operations small, typed and
platform-neutral, and let each target backend translate that shared operation
to its ABI/instruction set. A missing feature in one backend is a parity gap to
close, not a reason to fork the feature's behavior by architecture.

## Current state and migration owner

| Capability | Current source of behavior | Joss counterpart | Status / next parity requirement |
|---|---|---|---|
| Parser, analyzer, types, diagnostics | Go packages under `pkg/` | None; compiler implementation is Go | Not started; first compiler self-hosting milestone |
| Interpreter and runtime | `pkg/core` | `std/` modules cover selected APIs | Not started; port module-by-module while preserving `PreparedProgram` semantics |
| HTTP request dispatch | `pkg/core/dispatcher.go` route matching and `__joss_http_dispatch` adapter | `std/http/server.joss`, `std/http/request.joss` | Partial; request decoding now runs in Joss for both interpreter and Joss server paths. Native lowering synthesizes route dispatch from the same route AST, but form parser execution still fails AOT parity; converge route/session context and test POST/API, forms, JSON, cookies, status and errors |
| HTTP server | Go server removed; native socket emitters plus `std/http/server.joss` | `std/http` | Partial; request reads are bounded and WebSocket lifecycle/policies are not fully integrated |
| Sessions, CSRF, rate limit | Historical Go server behavior; current partial Joss modules | `std/http/session.joss`, `csrf.joss`, `ratelimit.joss` | Partial; persistence, corruption, isolation, enforcement and failure rollback require coverage |
| Authentication | `pkg/core/auth*.go`, native Auth stubs | `std/auth/auth.joss` | Partial: `id`, `check`/`guest`/`logout` policy and `hasRole` decision are Joss; session access is still a native boundary. Port login/register, reset/verification, token policy, 2FA and OAuth incrementally with database/crypto/transport boundaries |
| WebSockets | `pkg/core` callbacks and native socket/server code | `std/websocket/websocket.joss` | Partial; handshake exists, connection read/callback/cleanup behavior needs parity |
| Native IR and backend | `pkg/ir`, `pkg/backend/native` in Go; backend contains machine-code emitters | No Joss-owned compiler/backend | Not started; keep Go backend as differential oracle until Joss emitter covers target matrix |
| CLI and project orchestration | `cmd/joss` Go | None | Not started; command and exit-code compatibility suite required |
| LSP and editor | `vscode-joss/src/server` implementation and extension JS | None | Not started; protocol/cancellation/diagnostic parity required |
| Generators and validation tools | `tools/*.go` | None | Not started; deterministic output and stale-output failure tests required |
| Storage OCI integration | Historical Go implementation removed | `std/storage/storage.joss` | Migrated directionally; retain only minimal transport/crypto primitives and prove parity |
| Legacy VM | `pkg/vm` removed | No counterpart planned | Intentionally excluded: it must not become a second semantic authority |

## Deletion audit against `v3.6.8.0`

The audited comparison is `git diff --name-status v3.6.8.0...HEAD` (tag resolves
to commit `v3.6.8.0`; current audited HEAD is `05f85b0`). A deletion is not
automatically a regression: it must either have a Joss-owned replacement or be
classified as historical/tooling material before it is considered intentional.

| Removed area | Evidence in the tag | Current classification | Remaining work |
|---|---|---|---|
| `pkg/server/**` | HTTP lifecycle, request decoding, response mapping, sessions, CORS, CSRF, rate limits, hot reload, WebSockets and characterization tests | Migration intended; parity incomplete | Restore the deleted contracts as Joss tests and implement missing behavior in `std/http`; do not restore the Go server as a second authority |
| `pkg/vm/**` | Separate bytecode VM/compiler and differential tests | Intentionally retired per current architecture | Keep excluded; do not reintroduce a second execution semantics |
| `pkg/core/lib_storage_oci.go`, `lib_storage_nooci.go` | Go OCI implementation and build-tag fallback | Migration intended; `std/storage/storage.joss` exists | Verify OCI request signing/transport and failure behavior across interpreter/AOT before closing |
| `vscode-joss/src/diagnostics.js`, `hover.js` | Early extension-side regex diagnostics and hard-coded hover text | Replaced by typed TypeScript LSP providers | Verify current diagnostics/hover behavior; do not restore stale duplicate implementations |
| `audit/**`, `remediation/**`, `branchmark.md`, installation notes, prompt file | Audits, remediation reports, deployment instructions and workspace prompt | Historical/operator documents, not language/runtime capabilities | Keep their useful findings in tracked migration/security docs where still applicable; do not add them to the public language manual without a current contract |

The deleted server tests are authoritative evidence of old public behavior,
not permission to restore the old Go implementation. In addition to the earlier
summary, the tag tests specifically cover:

- request query + JSON/form body + headers + cookies + host/scheme metadata;
- memory/file/Redis session selection, unavailable and corrupt storage, atomic
  persistence, rollback, nested snapshot isolation, and unknown drivers;
- CORS explicit allow/deny/wildcard, CSRF enforcement and API exemption;
- response mapping for string/JSON/RAW/redirect/file, cookies, headers and
  flash persistence failure;
- rate window reset, forwarded IPv4 and remote IPv6 address selection;
- WebSocket invalid handshake, unregistered-route closure, cleanup and later
  request isolation; hot reload and IndexNow endpoint behavior.

The Joss `HttpRequestParser` now handles request-line metadata, query/form
fields, JSON fields, headers and cookies, and the interpreter dispatcher invokes
it instead of maintaining a second Go parser. Its standalone AOT differential
previously produced invalid output when an unresolved runtime label was silently
filled by a pass-through adapter. AMD64 and ARM64 now reject unresolved global
labels, while a small set of framework method families retains explicit
transitional adapters. `strpos`, `substr` and `urldecode` remain unresolved in
AOT. The analyzer contract for `strpos` now reflects its documented `int|bool`
result, but native AOT still lacks the operation and the current untagged word
representation cannot distinguish a `false` result from integer zero. `explode`
previously wrote NUL terminators into its input and crashed when
the input was a read-only literal on Windows; the AMD64 emitter now copies the
input before splitting, and the host Windows AMD64 differential confirms both
the split element and the original string remain correct. This fix is not yet
verified on the other OS/architecture targets and does not establish full
Unicode or multi-character delimiter parity. Empty maps now lower to a `mixed`
value type; an empty-map string write/read differential passes on the host AMD64
runtime, and the same IR compiles across the supported target matrix. This does
not yet prove general map parity or ARM64 execution. The HTTP form parser is not
complete in AOT; the remaining string operations need a Joss/IR implementation
with shared semantics before Auth flows can be accepted. The
Joss `Session`, `Csrf`, and `RateLimiter` classes are
partial helpers; they do not yet satisfy the deleted lifecycle/backend
contracts or enforce those policies in `HttpServer::handle`.

## Deleted server contracts to restore as tests

The deleted `pkg/server` characterization tests covered contracts that must
move to Joss-owned HTTP tests rather than be restored as a Go server:

- session backend availability/corruption, unknown drivers, snapshots,
  isolation, persistence and rollback after write failure;
- WebSocket handshake rejection, connection cleanup and isolation between
  requests/connections;
- CORS allow/deny/wildcard behavior, CSRF enforcement and API exemption;
- Joss response mapping, redirects, flash persistence/failure, and endpoint
  behavior.

## Target and size matrix

Required primary matrix: Linux, Windows and macOS on AMD64 and ARM64. The repo
also carries Android ARM64 and additional cross-target checks; those remain
covered where the existing target matrix supports them. Track each target as
`not-started`, `parity`, or `blocked` per phase; a target is not complete merely
because its output has a valid executable header.

Track artifact sizes separately for a compact CLI, a minimal HTTP server and a
full web application. Approximately 80 KB is an optimization reference for
compact profiles, not a behavior or feature limit.

## Phase gates

| Phase | Required negative/failure tests | Exit gate |
|---|---|---|
| Inventory/contracts | missing owner, missing target/backend, undeclared dependency, changed public contract | every active capability has an owner, target set and parity test |
| Joss runtime/stdlib | malformed input, expired/absent/corrupt state, injected disk/network/DB/crypto/mail failures, cleanup after exception/cancel | identical documented contract in interpreter and AOT |
| Compiler/backend | invalid IR, missing lowering/emitter, ABI mismatch, malformed binary, overflow, linker/loader failure | differential output/error/exit code and runnable artifacts on each target |
| CLI/LSP/generators | malformed/missing config, stale generated output, invalid source range, cancellation, inaccessible files, clean environment | stable commands/protocol and deterministic generators |
| Bootstrap/removal | absent bootstrap, non-reproducible self-build, target rejection, forbidden dynamic dependency | Joss compiler rebuilds itself; Go/ASM owner removed only after all relevant gates pass |

## Baseline evidence to capture before each subsystem cutover

- exact public API and diagnostics from source/tests (not only docs);
- `joss run` stdout, stderr and exit code for success and failure cases;
- AOT outputs and artifact size for the same program;
- target/backend support and any OS-specific runtime dependency;
- generated documentation/catalog hashes and architecture-gate results.

The working tree already contains local native-server, Auth, backend and
documentation changes. Those changes must be reviewed and preserved during
implementation; this inventory does not declare them accepted or complete.

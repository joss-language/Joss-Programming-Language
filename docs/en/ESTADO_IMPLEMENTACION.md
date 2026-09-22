# Deployment status and limits

[Index](README.md) · [Architecture](ARQUITECTURA.md) · [Audit](DOCUMENTATION_AUDIT.md)

This page separates available capabilities, partial implementations, and
design objectives. It corresponds to the audited code, not guarantees of a
previously downloaded version.

| Area | Real status | Reference |
|---|---|---|
| Language | Parser Pratt, explicit fixed or mixed type variables, classes/inheritance/interfaces/enums/visibility, functions/closures/ref, ternaries, guard, loops, value matching, try/catch, defer and channel select. | [Syntax](SINTAXIS.md) |
| Types | int64, float64, decimal, strings, arrays, maps, object, channel, classes, unions and nullable. Runtime analysis and defense with registered differences. | [Types](SISTEMA_TIPOS.md) |
| Concurrency | Goroutines using async, Future, blocking wait and channels. Mobile execution supports cooperative cancellation and safe capture of late output; a blocking native call may continue after timeout. No general structured cancellation or deep isolation exists. | [Concurrency](CONCURRENCIA.md) |
| Analyzer | Declarations, scopes, assignability, known members, returns and diagnostics; local narrowing in ternaries, guard and null comparisons. No general CFG or complete termination proof exists. | [Parser](ANALIZADOR.md) |
| Main run | AST interpreted with callable plans, frames and caches. Native build packages JOSSBC2Z compressed AST with runner Go. | [Architecture](ARQUITECTURA.md) |
| Experimental VM | pkg/vm contains separate compiler and VM; it is not default backend of CLI/core. | [Internal](ARQUITECTURA.md) |
| Web | HTTP/WS router, views, Request/Response, session, CSRF, CORS, TLS and configurable limits. | [Web project](PROYECTO_WEB.md) |
| SQL | SQLite/MySQL/PostgreSQL/SQL Server adapters, builder, Schema and migrations. Partial portability per operation; ordinary runtime SQL queries during GranDB::transaction use the active Tx. Nested transactions are not supported. | [Models](MODELOS.md) |
| Plugins | Signed container and index, AST and JPBC; partial compilers. Route Wasm generates text stubs, it does not run Wasm. | [Plugins](PLUGINS.md) |
| Plugin permissions | Guard for mapped host calls; no WASI/OS sandbox or per-packet consent. | [Plugins](PLUGINS.md) |
| Tools | CLI with REPL, formatter, linter/fix, test runner and VS Code extension with generated catalog. No integrated debugger exists. | [CLI](CLI.md) |

No general ownership, default immutability, general pointers,
traits, protocols, function generics, finally,
nor LLVM/Cranelift backend exist. Collection annotations are not equivalent to universal generics
nor do they guarantee that each mutation revalidates elements. Generic classes
exist, but their parameters still need stronger guarantees.

The absence of source imports/exports/namespaces is a permanent decision,
not a pending feature. ALIM modularity uses integrated capabilities,
Physical organization and auto-loading plugins.

The thesis combines target architecture and implementation. His statements of
backend, isolation and modules must be checked against this state and with the
[historical technical audit](AUDITORIA_TECNICA_2026.md). No objectives present
as services already completed.

# Deployment status and limits

[Index](README.md) · [Architecture](ARQUITECTURA.md) · [Audit](DOCUMENTATION_AUDIT.md)

This page separates available capabilities, partial implementations, and
design objectives. It corresponds to the audited code, not guarantees of a
previously downloaded version.

| Area | Real status | Reference |
|---|---|---|
| Language | Parser Pratt, explicit fixed or mixed type variables, classes/inheritance/visibility, functions/closures/ref, ternaries, loops, value matching, try/catch. | [Syntax](SINTAXIS.md) |
| Types | int64, float64, decimal, strings, arrays, maps, object, channel, classes, unions and nullable. Runtime analysis and defense with registered differences. | [Types](SISTEMA_TIPOS.md) |
| Concurrency | Goroutines using async, Future, blocking wait and channels. No structured cancellation or deep isolation. | [Concurrency](CONCURRENCIA.md) |
| Analyzer | Symbols in two passes, scopes, assignability, known members, returns and diagnostics. No general proof of completion or refinement by branches. | [Parser](ANALIZADOR.md) |
| Main run | AST interpreted with callable plans, frames and caches. Native build packages JOSSBC2Z compressed AST with runner Go. | [Architecture](ARQUITECTURA.md) |
| Experimental VM | pkg/vm contains separate compiler and VM; it is not default backend of CLI/core. | [Internal](ARQUITECTURA.md) |
| Web | HTTP/WS router, views, Request/Response, session, CSRF, CORS, TLS and configurable limits. | [Web project](PROYECTO_WEB.md) |
| SQL | SQLite/MySQL/PostgreSQL/SQL Server adapters, builder, Schema and migrations. Partial portability per operation; transaction does not bind queries to the Tx. | [Models](MODELOS.md) |
| Plugins | Signed container and index, AST and JPBC; partial compilers. Route Wasm generates text stubs, it does not run Wasm. | [Plugins](PLUGINS.md) |
| Plugin permissions | Guard for mapped host calls; no WASI/OS sandbox or per-packet consent. | [Plugins](PLUGINS.md) |
| Tools | CLI, formatter, linter/fix, test runner and VS Code extension with generated catalog. There is no REPL command or integrated debugger. | [CLI](CLI.md) |

There are no ownership, default immutability, general pointers, interfaces,
traits, protocols, function/class generics, defer/finally, channel select,
nor LLVM/Cranelift backend. Collection annotations are not equivalent to generics
universals nor do they guarantee that each mutation revalidates elements.

The absence of source imports/exports/namespaces is a permanent decision,
not a pending feature. ALIM modularity uses integrated capabilities,
Physical organization and auto-loading plugins.

The thesis combines target architecture and implementation. His statements of
backend, isolation and modules must be checked against this state and with the
[historical technical audit](AUDITORIA_TECNICA_2026.md). No objectives present
as services already completed.

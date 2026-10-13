# JOSS NATIVE AOT: Standard Library & Host Services

In Joss Native AOT, the Standard Library and Host Services (like GranDB, HTTP router, Filesystem) are NOT re-implemented in assembly or C.

## Architectural Separation

- **Native Runtime:** Responsible for low-level primitives necessary for executing machine code. This includes memory allocation, strings, basic arrays, objects, maps, exceptions, basic I/O, and the ABI itself.
- **Standard Library / Host Services:** Responsible for high-level capabilities like GranDB, Storage, HTTP, WebSocket, JSON, filesystem, templates, crypto, etc.

## The Integration Strategy

The canonical implementations of Host Services live in `pkg/core` and related packages.

When `joss build` compiles a program that uses `GranDB::where(...)`:
1. The Semantic Analyzer resolves this as a `host_service` call.
2. The IR Lowerer generates a `CallHostServiceInst` explicitly targeting `GranDB::where`.
3. The Machine Code Generator emits a native call using the `Joss Native ABI` into the `Canonical Host Bridge`.

The final standalone binary produced by `joss build` will statically link the necessary Go routines behind the Host Bridge, meaning that if `GranDB` is updated in `pkg/core`, the AOT compilation will immediately reflect these changes without duplicating the logic in `pkg/backend/native`.

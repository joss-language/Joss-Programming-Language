# JOSS NATIVE AOT: Conformance Testing

Conformance testing ensures absolute parity between `joss run` and `joss build`.

## The Rule

A program executed with `joss run` must produce identical behavior (stdout, stderr, exit code, and functional side-effects like database writes) as when compiled via `joss build` and executed standalone.

## No Fallbacks Allowed

If a feature is not supported in the Native Backend, the compilation (`joss build`) must explicitly fail with a descriptive diagnostic indicating an "AOT implementation gap". Under no circumstances will `joss build` secretly bundle the AST interpreter as a fallback mechanism.

## Test Boundaries

Conformance tests (e.g. `tests/native/differential_test.go`) must cover:
- Primitive arithmetic and coercion.
- String interpolation, Arrays, Maps.
- Classes, Inheritance, Interfaces, and Structs/Records.
- Closures with lexical capture and higher-order functions.
- Exceptions (`try`/`catch`/`throw`/`finally`).
- Full Stdlib interaction (HTTP, GranDB, JSON, Crypto, WebSockets) spanning the Host Bridge.

If a canonical implementation requires changes, it is updated in `pkg/core`. If the AOT conformance fails, the issue is resolved in the IR Lowerer (`pkg/ir`) or the Native Backend (`pkg/backend/native`), NEVER by creating a second implementation of the capability.

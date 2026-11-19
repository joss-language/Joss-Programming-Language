# Interoperability Strategy (Pure Joss + Go + Static C)

## 1. Three-Pillar Ecosystem
1. **Pure Joss (`std/`)**: High-level language logic, standard library APIs, HTTP/router/ORM contracts.
2. **Go (Compiler & Tooling)**: Lexer, Pratt parser, semantic analyzer, IR lowerer/optimizer, CLI, LSP, test runner. Never embedded in AOT production binaries.
3. **Static C (`runtime/native/`, external `.a` libs)**: Memory allocator, string/buffer primitives, C-ABI FFI, crypto/TLS/SQLite/compression via static linking.

## 2. C-ABI FFI Design
- **Syntax**: `extern "C" func c_add(int $a, int $b): int` (and `extern "C" from "libfoo.a" func ...`).
- **Calling Conventions**:
  - Linux/macOS x86_64: System V AMD64 ABI (`rdi, rsi, rdx, rcx, r8, r9`).
  - Windows x86_64: Microsoft x64 ABI (`rcx, rdx, r8, r9` + 32-byte shadow space).
  - Linux/macOS/Android AArch64: AAPCS64 (`x0-x7`).
- **Type Mapping**:
  - `int` <-> `int64_t`, `float` <-> `double`, `bool` <-> `bool`/`uint8_t`, `string` <-> `const char*` (NUL-terminated), `ByteSlice`/`ptr` <-> `void*`/`uint8_t*`.
- **Linking**: Static linking into final standalone executable via `lld` / `clang` (no Joss DLLs).

## 3. Controlled Go Reuse (Go-Lite Experiment)
- Evaluate translating a strict pure-algorithmic Go subset (no goroutines, no `reflect`, no GC-dependent runtime) into Joss IR for reuse of small pure algorithms.

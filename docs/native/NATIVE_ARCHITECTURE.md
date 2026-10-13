# JOSS NATIVE AOT: Architecture Overview

This is the definitive design document for the unified Joss Architecture.

```text
                         JOSS SOURCE
                              │
                              ▼
                         Lexer / Parser
                              │
                              ▼
                     Semantic Analysis
                              │
                              ▼
                   Canonical Joss Semantics
                              │
                    ┌─────────┴─────────┐
                    │                   │
                    ▼                   ▼
              Interpreter            Native IR
                    │                   │
                    ▼                   ▼
               joss run           Native Backend
                                        │
                                        ▼
                                  Machine Code
                                        │
                                        ▼
                                Native Linker (w/ Host Bridge)
                                        │
                                        ▼
                              Standalone Executable
```

## Guiding Principles
- **One Language, One Semantics:** There is no "Joss Native" subset. Every construct (Classes, Closures, Structs, Exceptions) that exists in Joss must be representable in Native IR and compiled by the AOT backend.
- **One Source of Truth:** We never duplicate canonical logic (`GranDB`, `HTTP`, `Filesystem`) into C or manually-written assembly.
- **Standalone Artifacts:** The user running `joss build` does not need Go, GCC, Clang, or CMake. `joss build` emits self-contained binaries (PE32+, ELF64).
- **Zero Interpreter in AOT:** The AOT binary never serializes the AST or bundles a script runner. It is pure machine code interacting with the native runtime and canonical host services via the `Host Bridge`.

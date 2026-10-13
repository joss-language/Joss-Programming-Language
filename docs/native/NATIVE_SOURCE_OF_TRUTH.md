# JOSS NATIVE AOT: The Single Source of Truth

This document formally describes the single source of truth architectural rule for Joss Native AOT. 

## The Rule

> **Joss has a single definition of the language, a single semantics, and a single source of truth for each capability. There are NEVER parallel implementations of the same logic.**

The difference between `joss run` (Interpreter) and `joss build` (AOT) is solely **HOW** Joss is executed, not **WHAT** Joss means.

## Canonical Semantic Flow

1. **Joss Source Code**
2. **Lexer / Parser Pratt** (`pkg/parser`): Generates the immutable AST.
3. **Semantic Analyzer** (`pkg/analyzer`): Enforces nominal typing, validates scopes, builds signatures, and produces the `PreparedProgram` containing sidecar `AnalysisFacts`.
4. **Native IR Lowerer** (`pkg/ir`): Converts the AST + Facts directly into typed Native IR (SSA/3AC blocks). AST interpretation NEVER occurs beyond this point.
5. **Standalone x86_64 Machine Code Generator** (`pkg/backend/native`): Generates direct PE32+ / ELF64 machine code mapping purely from IR semantics.

## The Canonical Host Bridge

Historically, there was an attempt to write complex modules (like GranDB, HTTP, Storage) in native machine code or assembly manually (e.g. `native_grandb_stdlib_amd64.go`). This violated the single source of truth, as `pkg/core` already contained these capabilities.

Under the unified architecture, this problem is solved via the **Canonical Host Bridge**.

- If an IR instruction is `CallBuiltinInst`, `CallPrimitiveMethodInst`, or `CallHostServiceInst`, the Native Generator emits a native ABI call across the Host Bridge interface.
- This allows AOT binaries to link to the canonical implementations of these features directly (e.g., statically linked via Go `c-archive` or similar cross-platform mechanism) WITHOUT re-implementing databases, string allocations, or networking stacks in assembly.
- All types, scopes, and target resolutions are perfectly preserved across the boundary because `AnalysisFacts.ResolvedCalls` guarantees the method target is explicit in IR.


# Target Next-Generation Compiler Architecture

## 1. Core Principles
- **One Language. One Semantics. Multiple Execution Modes.** (`joss run`, `joss server`, `joss program`, `joss build`).
- **Multi-Language Implementation. Single Joss Semantics.** (Pure Joss + Go compiler/tooling + static C libraries/runtime).
- **Native Performance. Minimal Runtime. Maximum Reusability.**

## 2. Target Compilation Pipeline

```text
Joss Source (.joss)
  -> Lexer & Pratt Parser (pkg/parser)
  -> Semantic Analyzer (pkg/analyzer -> PreparedProgram)
  -> High-Level IR (HIR) + Ownership/Escape Analysis
  -> Typed SSA Low-Level IR (LIR) + Verifier (pkg/ir)
  -> Backend Selection:
       [A] LLVM + LLD AOT Backend (object files + static C runtime/libs -> ELF/PE/Mach-O)
       [B] Integrated Fast Machine Code Emitter (JIT & zero-dep fast AOT)
```

## 3. Target Runtime & Memory Architecture
- Minimal static C runtime (`runtime/native/joss_rt.c` + headers) compiled to a static archive (`libjoss_rt.a`) or linked via `clang`/`lld`, replacing thousands of lines of hand-coded ASM intrinsics.
- Tagged dynamic value representation (`JossValue` 16-byte `{tag, payload}` or NaN-boxed/tagged struct) eliminating pointer-vs-int heuristics.
- Deterministic memory management: Stack allocation -> Escape-analyzed arenas -> Explicit `ByteBuffer`/`StringBuilder` ownership & `free`.
- C-ABI FFI (`extern "C" func ...`) with static library linking (`-l` / `.a` archives), zero Joss DLLs, zero embedded Go runtime.

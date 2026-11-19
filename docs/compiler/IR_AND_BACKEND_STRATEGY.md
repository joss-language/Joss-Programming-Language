# IR and Backend Evolution Strategy

## 1. IR Evolution: `PreparedProgram -> HIR -> LIR SSA -> Backend`
- Keep `PreparedProgram` as the canonical semantic contract.
- Extend `pkg/ir` with:
  - Explicit `BoxValue` / `UnboxValue` / `TypeTag` instructions for `mixed`.
  - `ExternCallInst` for C-ABI foreign function calls.
  - `AllocBuffer` / `FreeBuffer` / `BufferGet` / `BufferSet` with bounds checks.
  - `StructLayout` with deterministic field offsets and sizes.
  - Dominator-tree and SSA form verification in `pkg/ir/verifier.go`.

## 2. Backend Strategy
- **Retain & Harden** the integrated machine-code emitter (`x86_64`, `aarch64`) for zero-dependency fast builds and in-memory JIT.
- **Enable & Modernize LLVM + LLD Backend** (`pkg/backend/native/llvm.go` + `compiler.go`):
  - Emit valid, typed LLVM IR (`.ll`) with `JossValue`, `ExternCallInst`, and runtime C declarations.
  - Compile & statically link with `libjoss_rt.a` and user `.a` libraries using `llc`/`clang`/`lld` when `--backend=llvm` or C-FFI/LLVM features are used.
  - Support dead-code elimination, `-Os`/`-O2`, `--gc-sections`, and symbol stripping.

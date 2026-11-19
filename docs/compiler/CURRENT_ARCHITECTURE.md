# Current Compiler & Runtime Architecture (Verified Baseline)

**Date:** 2026-11-19
**Branch:** `feat/next-gen-compiler-arch`

## 1. Pipeline Overview

```text
Joss Source (.joss)
  -> pkg/parser (Lexer + Pratt Parser -> AST)
  -> pkg/analyzer (collectDeclarations -> projectScope -> validateNominalContracts -> analyzeSourceBodies -> PreparedProgram)
  -> pkg/ir (Lowerer -> SSA/3AC *ir.Program -> Verifier)
  -> pkg/backend/native (x86_64 / AArch64 MachineCodeBuilder OR LLVM IR emitter)
  -> PE32+ (pe_emitter.go) / ELF64 (elf_emitter.go) / Mach-O (macho_emitter.go)
```

## 2. Verified Findings from Repository Inspection

### 2.1 Silent Native Stubs & False Results
- `pkg/backend/native/machine_code_aarch64.go`:
  - `fn_Auth_check`, `fn_Auth_id`, `fn_Auth_user`, `fn_Auth_logout` emit `mov x0, #0; ret`.
  - `fn_file_exists` emits `mov x0, #1; ret` (always returns true!).
  - `__joss_rt_route_matches` emits `mov x0, #1; ret` (always matches!).
  - `fn_Turnstile_siteKey`, `fn_Markdown_toHtml`, `fn_redirect`, `fn_json` fall through to returning `""`.
- `pkg/backend/native/native_intrinsics_x86_64.go`:
  - Legacy placeholders for `fn_Auth_guest` (`return 1`), `fn_Auth_*` (`return 0`), `fn_Turnstile_siteKey` (`return "1x00000000000000000000AA"`). Although `capability_gaps.go` rejects direct calls in most cases, the emitted labels still exist as silent stubs if reached.

### 2.2 Dynamic Value (`mixed`) Representation
- `joss_print_mixed` in `machine_code_x86_64.go` (line 1405) and `machine_code_aarch64.go` (line 367) uses a raw pointer-vs-integer heuristic (`val < 0x10000` => int, `val >= 0x10000` => `char*` string pointer).
- **Bug:** Any `mixed` integer `>= 65536` (`0x10000`) is dereferenced as a string pointer, causing a segmentation fault. Booleans and floats cannot be distinguished from integers in `mixed`.

### 2.3 Security & Secret Management
- **Build-time environment embedding:** `pkg/backend/native/native_env.go` (`readBuildTimeEnv()`) reads `env.joss`, `env.enc`, `.env` from `.`, `JOSS_PROJECT_DIR`, `..`, `../..` during compilation and embeds them into `b.embeddedEnv` in both `machine_code_x86_64.go` and `machine_code_aarch64.go`. This leaks developer/build-machine secrets into compiled binaries.
- **Hardcoded master secret:** `"JOSSECURITY_MASTER_SECRET_2025"` is hardcoded in `pkg/backend/native/native_env.go`, `pkg/core/env_loader.go`, `cmd/joss/build.go`, `cmd/joss/native_builder.go`, and `cmd/runner/main.go`.
- **HTTP Path Traversal:** `handleVendorAssets` (`pkg/server/handler_routing.go`, `pkg/server/server.go`) only checks `strings.Contains(relPath, "..")`, lacking `filepath.Abs` + `filepath.Rel` containment checks, backslash rejection, and symlink/null-byte protection. `servePublicFile` (`pkg/server/handler.go`) also lacks `filepath.Rel` root containment verification.

### 2.4 Memory Management
- `__joss_os_alloc` (`sys_mmap` on Linux, `HeapAlloc` on Windows) allocates zeroed memory with no `free`, no arena reset, and no ownership tracking. Every string concatenation, array, map, or object leaks for the lifetime of the process.

### 2.5 Excessive Manual ASM & Backend State
- `pkg/backend/native/machine_code_x86_64.go` (4,519 lines), `native_intrinsics_x86_64.go` (1,873 lines), `machine_code_aarch64.go` (2,794 lines), `native_server_*.go`, `native_strings_aarch64.go` encode thousands of lines of hand-written machine code bytes.
- `pkg/backend/native/llvm.go` already emits textual LLVM IR (`.ll`), and `clang`/`llc`/`lld` (LLVM 14.0.6) are available on the system, but `pkg/backend/native/compiler.go` currently disables `BackendLLVM`.
- `pkg/backend/native/abi/` exists for ABI definitions.

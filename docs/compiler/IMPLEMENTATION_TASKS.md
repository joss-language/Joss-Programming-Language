# Implementation Tasks

## Phase 0: Stabilization & Security
- [x] **P0.1** Remove silent native stubs in `machine_code_aarch64.go` and `native_intrinsics_x86_64.go` (fail explicitly if reached, or ensure capability validation rejects them before emission).
- [x] **P0.2** Stop automatic build-time secret embedding (`readBuildTimeEnv()`) in `native_env.go`, `machine_code_x86_64.go`, `machine_code_aarch64.go`.
- [x] **P0.3** Remove hardcoded `"JOSSECURITY_MASTER_SECRET_2025"` across `native_env.go`, `env_loader.go`, `cmd/joss/build.go`, `cmd/joss/native_builder.go`, `cmd/runner/main.go` (require `JOSS_ENV_KEY` / explicit key, fail clearly when missing).
- [x] **P0.4** Harden HTTP path traversal in `pkg/server/handler.go` (`servePublicFile`), `pkg/server/handler_routing.go` (`handleVendorAssets`), `pkg/server/server.go`, and `pkg/server/response_writer.go` with tests.
- [x] **P0.5** Fix `mixed` value representation (`joss_print_mixed` `>= 0x10000` segfault) with proper tagged/boxed representation in IR & native backends + differential tests.

## Phase 1: Memory & Type Foundations
- [x] **P1.1** Design & implement `runtime/native/joss_rt.h` & `runtime/native/joss_rt.c` (tagged `JossValue`, arena allocator, `JossByteBuffer`, `JossStringBuilder`, bounds-checked slices).
- [ ] **P1.2** Add `ByteBuffer` / `StringBuilder` / `ByteSlice` types and IR instructions (`AllocBuffer`, `FreeBuffer`, `BufferLen`, `BufferGet`, `BufferSet`).

## Phase 2: Native C Interop
- [ ] **P2.1** Add `extern "C"` syntax in `pkg/parser`, type-checking in `pkg/analyzer`, `ExternCallInst` in `pkg/ir`.
- [ ] **P2.2** Implement static `.a` linking and C-ABI call emission in LLVM/LLD backend.
- [ ] **P2.3** End-to-end test: Joss calling a C function compiled to `.a`, linked into a standalone ELF without Go runtime or Joss DLLs.

## Phase 3: IR Evolution
- [ ] **P3.1** Upgrade `pkg/ir/verifier.go` with dominance & SSA validation and new opcodes.
- [ ] **P3.2** Add explicit `BoxValue` / `UnboxValue` and `StructLayout` support.

## Phase 4: Modern Native Backend (LLVM + LLD)
- [ ] **P4.1** Upgrade `pkg/backend/native/llvm.go` to emit complete, valid LLVM IR linked against `libjoss_rt.a`.
- [ ] **P4.2** Enable `BackendLLVM` in `pkg/backend/native/compiler.go` & `driver.go` using `clang`/`llc`/`lld` when `--backend=llvm` or C-FFI is used.
- [ ] **P4.3** Validate differential parity between interpreter, integrated native emitter, and LLVM+LLD backend.

## Phase 5–9: Stdlib Migration, Go-Lite, Multi-Platform, Optimization, Self-Hosting
- [ ] **P5.1** Migrate string/buffer/JSON/URL operations to Pure Joss + `joss_rt.c`.
- [ ] **P6.1** Prototype Go-Lite pure-algorithmic translator to Joss IR.
- [ ] **P7.1** Validate multi-platform targets (execute Linux AMD64, verify ELF/PE/Mach-O/ARM64 artifacts).
- [ ] **P8.1** Benchmarks, DCE, `-Os`/`--gc-sections`, `noembedlocales` verification.
- [ ] **P9.1** Progressive self-hosting feasibility prototype & `FINAL_IMPLEMENTATION_REPORT.md`.

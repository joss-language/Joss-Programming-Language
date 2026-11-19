# Migration Roadmap

| Phase | Name | Key Deliverables | Acceptance Criteria |
|---|---|---|---|
| 0 | Stabilization & Security | Remove silent native stubs, fix `mixed` `>= 0x10000` bug, stop build-time env secret embedding, remove hardcoded master secret, harden HTTP path traversal, fix memory leaks | Zero silent stubs, reproducible security & parity tests pass |
| 1 | Memory & Type Foundations | `ByteSlice`, `ByteBuffer`, `StringBuilder`, bounds-checked indexed memory, tagged `JossValue`, deterministic free/arena | Strings/buffers/structs work without per-feature ASM; memory & parity tests pass |
| 2 | Native C Interop | `extern "C" func`, platform calling conventions, static `.a` linking, C-ABI types & structs, Joss->C test | Joss program calls C function in static lib, zero Go runtime, zero Joss DLLs |
| 3 | IR Evolution | HIR/LIR SSA extensions, dominance/type verification, `ExternCallInst`, `BoxValue`/`UnboxValue`, buffer ops | `ir.Verifier` validates CFG, dominance, types, and new opcodes |
| 4 | Modern Native Backend | Enable LLVM + LLD AOT backend alongside integrated emitter, static C runtime `libjoss_rt.a`, ELF/PE/Mach-O | Reference programs compile and run via LLVM+LLD and pass differential tests |
| 5 | Hybrid Standard Library | Migrate strings/buffers, JSON, URL encoding, crypto/HTTP primitives off manual ASM to Pure Joss + C runtime | Reduced ASM footprint; differential parity preserved |
| 6 | Controlled Go Reuse (Go-Lite) | Prototype pure-algorithmic Go subset -> Joss IR -> native machine code | Verified prototype or documented stop criteria |
| 7 | Multi-Platform Parity | Validate Linux AMD64 (real execution), cross-compile & verify ELF/PE/Mach-O/ARM64 targets | Honest matrix of executed vs cross-compiled targets |
| 8 | Optimization & Size Reduction | DCE, tree shaking, constant folding, `-Os`/`-O2`, `--gc-sections`, `noembedlocales` | Benchmarks & binary size baselines recorded |
| 9 | Progressive Self-Hosting | Evaluate & prototype compiler components in Joss | Bootstrap feasibility report |

# Risk Register

| ID | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | Breaking existing AOT tests that depend on `embeddedEnv` or legacy `env.enc` decryption | High | Provide explicit runtime env loading / `JOSS_MASTER_KEY` env var instead of hardcoded key or build-time `.env` baking; add opt-in flag if explicitly requested |
| R2 | `modernc.org/sqlite` not in offline VM Go module cache | Medium | Use `-tags "nodb noredis nowebview2"` for offline VM tests; document environment constraint |
| R3 | Regression in `tests/architecture/architecture_contract_test.go` | High | Keep `PreparedProgram` canonical, never import `pkg/core` from `pkg/analyzer`, keep `JOSS-NATIVE-001` for unsupported Core Language |
| R4 | ABI mismatch between Joss IR, C runtime (`libjoss_rt.a`), and x86_64/AArch64 emitters | High | Explicit ABI tests (`TestAuditCallingConventionAndStack`), System V AMD64 / Win64 / AAPCS64 adapters |
| R5 | Cross-target execution unavailable on non-Linux hosts | Medium | Execute Linux AMD64 binaries directly; validate ELF/PE/Mach-O headers/relocations for other targets and mark runtime execution pending |

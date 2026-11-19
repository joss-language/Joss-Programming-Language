# Validation Plan

## 1. Automated Test Gates
- `go vet -tags "nodb noredis nowebview2" ./...`
- `go test -tags "nodb noredis nowebview2" ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/ir ./pkg/backend/native ./pkg/server ./tests/architecture`
- `go test -tags "nodb noredis nowebview2" -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/ir ./pkg/backend/native ./pkg/server`
- `go test -tags "nodb noredis nowebview2" ./tests/native` (differential interpreter vs native AOT)

## 2. Security Validation
- Verify no `.env` / `env.joss` / `env.enc` secrets are embedded in `joss build` binaries by default.
- Verify no hardcoded cryptographic master secrets remain in production code.
- Verify HTTP path traversal (`..`, `\`, `%2e%2e`, symlink escape) is blocked with 403/404 in `pkg/server`.

## 3. Memory & Native Validation
- Verify `mixed` integers `>= 0x10000` (e.g., `100000`, `1000000`) print and compare without segfaults.
- Verify C-ABI static linking produces standalone executables with `ldd` / `nm` / symbol inspection showing zero Go runtime and zero Joss DLLs.
- Measure compile time, binary size, and execution speed benchmarks.

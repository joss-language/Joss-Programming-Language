# Changelog

## [3.6.8.0] - 2026-10-09

### Security
- **Auth**: Replaced non-cryptographic `math/rand` with `crypto/rand` for email OTP challenges, TOTP base32 secrets and recovery codes.
- **CSRF**: Eliminated session ID and token leaks to stdout/debug comments; implemented constant-time comparison via `subtle.ConstantTimeCompare`.
- **WebSocket**: Implemented strict origin checking in upgrader validating against request Host, loopback addresses and `APP_ALLOWED_ORIGINS` / `APP_URL`.

### Fixed
- **Compiler**: Eliminated silent fallback from native AOT build (`joss build`) to application runner bundle when IR lowering or verification fails; fails explicitly with actionable diagnostics.
- **CLI**: Added explicit `joss build bundle` / `joss build app` commands for self-contained runtime packaging.
- **Lexer**: Unterminated string literals spanning unescaped newlines or reaching EOF without matching quotes now correctly emit `ILLEGAL` token and parser errors.

## [3.6.7.8] - 2026-10-08
- Added options to skip tests and VS Code packaging in manual distribution workflow.
- Normalized newline comparisons in native differential test suite.

## [3.6.7.6] - 2026-10-06
- Hardened runtime capabilities and security boundary checks.

## [3.6.7.2] - 2026-09-28
- Implemented const declarations, immutable properties, and type normalization.

## [3.6.7.1] - 2026-09-24
- Added Microsoft SQL Server support, GranDB transactions, distinct and crossJoin queries.

## [3.6.4] - 2026-08-30

### Added

- `Redis` native class now supports transparent auto-connection from `REDIS_URL`, `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_USER`, and `REDIS_DB`.
- Added `Redis::has()`, `Redis::forget()`, `Redis::ttl()`, and `Redis::flush()` methods.
- Support for URL connection strings (`redis://...`) and separate host/port definitions in web session driver (`SESSION_DRIVER="redis"`).

## [3.6.3] - 2026-07-28

### Fixed

- WebSocket `onMessage` callbacks now retain `$ws`, route parameters, and local variables after the handler returns.
- Captured callbacks created in the same scope share state across messages and `onClose`, and their invocations are serialized per connection.
- WebSockets now support `onClose`, local channel subscriptions/publication, automatic subscription cleanup, write-error reporting, message limits, ping/pong keepalive, and safe callback panic isolation.

## [3.6.2] - 2026-07-28

### Fixed

- Verification resends reuse an unexpired token instead of invalidating links already delivered by email.
- Login failures distinguish an unverified account from an incorrect password without relying on token generation.
- Password resets are atomic, reject weak or expired credentials, consume each token once, and verify the account after proving access to its email.
- Authentication emails and expiry timestamps are normalized consistently across SQLite, MySQL, and PostgreSQL.

### Security

- Verified accounts no longer produce a truthy fake verification token.
- Password reset table initialization is only cached after successful creation.
- API and web 2FA challenges now expire after five minutes, cannot authorize protected routes, and can only be exchanged once for an access token.

## [3.6.0] - 2026-07-14

### Added

- JP v2 bytecode packages with public symbol metadata and native `joss-rpc-v1` payloads.
- Plugin SDKs for C/C++, Python, PHP, Java, Kotlin, Dart/Flutter and Rust.
- Manual multiplatform distribution workflow and platform-aware remote installers.

### Changed

- `func` is the only function keyword; the former `function` spelling is rejected with a migration diagnostic.
- Dependencies declared in `joss.yaml` autoload; source-level `import`, `use` and namespace statements have been removed.
- Documentation now describes the actual parser, runtime, CLI, server, views, database, plugins and known limits.

### Fixed

- Registered native method surfaces now match their runtime handlers.
- `Response::error()` returns structured JSON, redirects honor an optional status, and request accessors honor defaults.
- Generated projects and editor snippets use syntax accepted by the current parser.

## [3.0.7] - 2025-12-22
### Fixed
- **Auth**: Fixed `user_role` not being restored from JWT claims, preventing admins from seeing admin-only UI.
- **Request**: Fixed `Request::all()` and `Request::except()` to exclude internal `_cookies` map, preventing database errors.
- **Database**: Added safety check in `GranDB` (SQLite/MySQL) insert methods to ignore unsupported `map` types.
- **View**: Fixed `@foreach` rendering for `map` types (specifically dates) by using Regex replacement for `{{ $var }}` tags. Added support for both dot (`$item.key`) and bracket (`$item['key']`) notation.
- **Handler**: Updated Session restoration logic to correctly populate `user_role` from JWT.

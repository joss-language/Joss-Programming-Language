# Security Policy

## Scope and Execution Trust Model

**Joss Programming Language** is designed for backend server and command-line application development.

- **Application Code Trust**: Scripts executed via `joss run` or compiled via `joss build` run with the privileges of the host process and the operating system user. Joss does not implement an operating-system level sandbox (such as WASI or process cgroups) for untrusted source code.
- **Plugins (`.jp`)**: Packages distributed in `.jp` format require Ed25519 cryptographic signatures and integrity verification before extraction and loading.
- **Web Stack**: Built-in HTTP and WebSocket servers enforce CSRF protection with constant-time token comparison, Origin header validation on WebSockets, and cryptographically secure random generation for authentication challenges (MFA/TOTP).

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 3.6.x   | :white_check_mark: |
| < 3.6   | :x:                |

## Reporting a Vulnerability

If you discover a potential security vulnerability in Joss, please report it privately:

1. Send an email to the security team or maintainers at `security@joss.red`.
2. Do not file public GitHub issues for undisclosed vulnerabilities.
3. Include detailed reproduction steps, proof-of-concept code, and the affected Joss version/environment.

Security reports receive an initial response within 48 hours and high-priority vulnerability patches are released with dedicated patch versions.

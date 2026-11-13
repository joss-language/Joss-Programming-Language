# HTTP Server

[Index](README.md) · Before: [web project](PROYECTO_WEB.md) · After: [HTTP](CONTROLADORES.md)

The server converts Joss routes into HTTP responses. Requires a web project
with `main.joss`, routes and `Server::start()`.```bash
joss server start
```
The default runtime port is 8000. The generated template writes
`PORT="80"` in `env.joss`, so that application uses 80 until you change it.

## Capabilities

- Public files under `/public/` and `/assets/`.
- Hot reload of code, views, assets, translations and environment.
- CSRF, CORS, security headers and WebSockets.
- Persistent sessions in file by default, or `memory` and `redis` drivers.
- The canonical session cookie is `joss_session`; `JOSS_SESSION` remains accepted for compatibility.
- `HttpRequestParser` in `std/http/request.joss` is the canonical source for method, route/query, headers, cookies, and URL-encoded or JSON form fields. The interpreter invokes this parser when it receives a raw request.
- The AOT server dispatches by method and route, but native execution of the Joss parser still needs string and map operation parity; AOT form support is incomplete.
- `Auth::check`, `Auth::guest` and web middleware share one session identity; the `joss_token` access token can restore it on each request.
- Rate limit per IP configurable with `RATE_LIMIT_REQUESTS` and `RATE_LIMIT_WINDOW_SECONDS`.
- Direct HTTPS/WSS via `TLS_CERT_FILE` and `TLS_KEY_FILE`.
- HTTP Timeouts: reading 15 s, writing 15 s and inactivity 60 s.```env
PORT="8443"
SESSION_DRIVER="file"
RATE_LIMIT_REQUESTS="120"
RATE_LIMIT_WINDOW_SECONDS="60"
TLS_CERT_FILE="certs/fullchain.pem"
TLS_KEY_FILE="certs/private.key"
```
In public deployments you can still use a reverse proxy for compression, balancing, and automatic certificate renewal.

The Pure Joss socket server starts or resumes `joss_session` for each dynamic route, passes its data to route dispatch, and emits the cookie in the response. Session state (including temporary 2FA and CSRF tokens) is stored in process memory; it is not shared across restarts or processes.

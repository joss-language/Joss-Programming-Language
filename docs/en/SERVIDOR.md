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
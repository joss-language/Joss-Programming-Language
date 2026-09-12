# Servidor HTTP

[Índice](README.md) · Antes: [projeto web](PROYECTO_WEB.md) · Depois: [HTTP](CONTROLADORES.md)

O servidor converte rotas Joss em respostas HTTP. Requer um projeto web
com `main.joss`, rotas e `Server::start()`.```bash
joss server start
```
A porta de tempo de execução padrão é 8000. O modelo gerado grava
`PORT="80"` em `env.joss`, para que o aplicativo use 80 até você alterá-lo.

## Capacidades

- Arquivos públicos em `/public/` e `/assets/`.
- Recarga a quente de código, visualizações, ativos, traduções e ambiente.
- CSRF, CORS, cabeçalhos de segurança e WebSockets.
- Sessões persistentes em arquivo por padrão, ou drivers `memory` e `redis`.
- Limite de taxa por IP configurável com `RATE_LIMIT_REQUESTS` e `RATE_LIMIT_WINDOW_SECONDS`.
- HTTPS/WSS direto via `TLS_CERT_FILE` e `TLS_KEY_FILE`.
- HTTP Timeouts: leitura 15 s, escrita 15 s e inatividade 60 s.```env
PORT="8443"
SESSION_DRIVER="file"
RATE_LIMIT_REQUESTS="120"
RATE_LIMIT_WINDOW_SECONDS="60"
TLS_CERT_FILE="certs/fullchain.pem"
TLS_KEY_FILE="certs/private.key"
```
Em implantações públicas, você ainda pode usar um proxy reverso para compactação, balanceamento e renovação automática de certificados.
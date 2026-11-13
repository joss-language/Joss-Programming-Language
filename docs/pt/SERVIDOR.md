# Servidor HTTP

[Índice](README.md) · Antes: [projeto web](PROYECTO_WEB.md) · Depois: [HTTP](CONTROLADORES.md)

O servidor converte rotas Joss em respostas HTTP. Requer um projeto web
com `main.joss`, rotas e `Server::start()`.

`joss server start` supervisiona o processo do servidor. Ao pressionar `q` ou
`Ctrl+C`, encerra o processo filho e libera seus sockets; se o processo termina
por um erro de inicialização (por exemplo, porque a porta já está em uso), o
supervisor também sai em vez de continuar aguardando.

`joss server start` supervisiona o processo do servidor. Ao pressionar `q` ou
`Ctrl+C`, encerra o processo filho e libera seus sockets; se o processo termina
por um erro de inicialização (por exemplo, porque a porta já está em uso), o
supervisor também sai em vez de continuar aguardando.

```bash
joss server start
```
A porta de tempo de execução padrão é 8000. O modelo gerado grava
`PORT="80"` em `env.joss`, para que o aplicativo use 80 até você alterá-lo.

## Capacidades

- Arquivos públicos em `/public/` e `/assets/`.
- Recarga a quente de código, visualizações, ativos, traduções e ambiente.
- CSRF, CORS, cabeçalhos de segurança e WebSockets.
- Sessões persistentes em arquivo por padrão, ou drivers `memory` e `redis`.
- O cookie canônico de sessão é `joss_session`; `JOSS_SESSION` continua aceito por compatibilidade.
- `HttpRequestParser` em `std/http/request.joss` é a fonte canônica para método, rota/query, headers, cookies e campos de formulários URL-encoded ou JSON. O interpretador chama esse parser ao receber uma requisição bruta.
- O servidor AOT despacha por método e rota, mas a execução nativa do parser Joss ainda precisa de paridade nas operações de strings e mapas; o suporte a formulários no AOT está incompleto.
- `Auth::check`, `Auth::guest` e o middleware web compartilham a identidade da sessão; o token de acesso `joss_token` pode restaurá-la em cada requisição.
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

O servidor de sockets Pure Joss inicia ou recupera `joss_session` em cada rota dinâmica, passa seus dados ao despacho de rotas e emite o cookie na resposta. O estado da sessão (incluindo tokens temporários de 2FA e CSRF) fica na memória do processo; não é compartilhado entre reinicializações ou processos.

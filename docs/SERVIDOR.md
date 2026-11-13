# Servidor HTTP

[Índice](README.md) · Antes: [proyecto web](PROYECTO_WEB.md) · Después: [HTTP](CONTROLADORES.md)

El servidor convierte rutas Joss en respuestas HTTP. Requiere un proyecto web
con `main.joss`, rutas y `Server::start()`.

```bash
joss server start
```

El puerto predeterminado del runtime es 8000. La plantilla generada escribe
`PORT="80"` en `env.joss`, por lo que esa aplicación usa 80 hasta que lo cambies.

## Capacidades

- Archivos públicos bajo `/public/` y `/assets/`.
- Hot reload de código, vistas, assets, traducciones y entorno.
- CSRF, CORS, headers de seguridad y WebSockets.
- Sesiones persistentes en archivo por defecto, o drivers `memory` y `redis`.
- La cookie canónica de sesión es `joss_session`; se acepta `JOSS_SESSION` por compatibilidad.
- `HttpRequestParser` en `std/http/request.joss` es la fuente canónica para método, ruta/query, headers, cookies y campos de formularios URL-encoded o JSON. El intérprete invoca ese parser al recibir una petición cruda.
- El servidor AOT ya despacha por método y ruta, pero la ejecución nativa del parser Joss aún requiere paridad de operaciones de strings y mapas; el soporte de formularios en AOT no está completo.
- `Auth::check`, `Auth::guest` y el middleware web comparten la identidad de sesión; el token de acceso `joss_token` puede reconstruirla en cada petición.
- Rate limit por IP configurable con `RATE_LIMIT_REQUESTS` y `RATE_LIMIT_WINDOW_SECONDS`.
- HTTPS/WSS directo mediante `TLS_CERT_FILE` y `TLS_KEY_FILE`.
- Timeouts HTTP: lectura 15 s, escritura 15 s e inactividad 60 s.

```env
PORT="8443"
SESSION_DRIVER="file"
RATE_LIMIT_REQUESTS="120"
RATE_LIMIT_WINDOW_SECONDS="60"
TLS_CERT_FILE="certs/fullchain.pem"
TLS_KEY_FILE="certs/private.key"
```

En despliegues públicos todavía puedes usar un proxy inverso para compresión, balanceo y renovación automática de certificados.

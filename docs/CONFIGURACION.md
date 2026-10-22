# Configuración

[Índice](README.md)

El runtime busca `env.joss`, después `env.enc` y finalmente `.env`; en desarrollo también intenta directorios padre. Las variables del sistema operativo sobrescriben el archivo. `System::env("KEY", "default")` consulta el entorno cargado.

```env
APP_ENV="development"
APP_URL="https://127.0.0.1:8443"
PORT="8443"
PREFIX="js_"
JWT_SECRET="secreto-largo-y-unico"
APP_KEY="clave-larga-y-unica"
```

`PREFIX` es la clave canónica; `DB_PREFIX` se conserva como alias.

## Bases de datos

SQLite:

```env
DB="sqlite"
DB_PATH="database.sqlite"
```

MySQL usa el puerto 3306 cuando `DB_HOST` no lo incluye:

```env
DB="mysql"
DB_HOST="127.0.0.1:3306"
DB_NAME="mi_app"
DB_USER="usuario"
DB_PASS="secreto"
```

PostgreSQL usa el puerto 5432 y acepta `DB_SSLMODE`:

```env
DB="postgres"
DB_HOST="127.0.0.1:5432"
DB_NAME="mi_app"
DB_USER="usuario"
DB_PASS="secreto"
DB_SSLMODE="require"
```

Los aliases `postgresql` y `pgx` también se normalizan a PostgreSQL.

Microsoft SQL Server usa el puerto 1433 y acepta `DB_ENCRYPT`:

```env
DB="sqlserver"
DB_HOST="127.0.0.1:1433"
DB_NAME="mi_app"
DB_USER="sa"
DB_PASS="secreto"
DB_ENCRYPT="disable"
```

Los aliases `mssql` y `mssqlserver` también se normalizan a SQL Server.

## Servidor y sesiones

```env
SESSION_DRIVER="file"
SESSION_FILE="storage/sessions.json"
RATE_LIMIT_REQUESTS="120"
RATE_LIMIT_WINDOW_SECONDS="60"
TLS_CERT_FILE="certs/fullchain.pem"
TLS_KEY_FILE="certs/private.key"
CORS_WEB="https://app.example.com"
```

- `file` es el driver de sesión predeterminado y persiste reinicios. `memory` es explícitamente volátil.
- `SESSION_DRIVER="redis"` almacena las sesiones web en Redis en lugar de disco.
- Certificado y llave TLS deben configurarse juntos.
- `CORS_WEB=*` permite cualquier origen sin credenciales; una lista separada por comas crea una whitelist exacta.

## Redis y Caché Distribuida

Joss incluye un cliente nativo de Redis con auto-conexión y soporte de sesiones:

```env
# Opción A: URL completa estándar
REDIS_URL="redis://default:password@127.0.0.1:6379/0"

# Opción B: Variables individuales (puerto 6379 y DB 0 por defecto)
REDIS_HOST="127.0.0.1"
REDIS_PORT="6379"
REDIS_USER="default"
REDIS_PASSWORD="password"
REDIS_DB="0"
```

- Si `REDIS_URL` o `REDIS_HOST` están presentes, la clase `Redis::set()`, `Redis::get()`, etc. se auto-conecta en la primera llamada.
- `SESSION_DRIVER="redis"` utiliza automáticamente estas credenciales.

## Procesos y plugins

- `ALLOW_SYSTEM_RUN=true` habilita `System::Run()`.
- `JOSS_PLUGIN_SIGNING_KEY` permite seleccionar una llave privada Ed25519 existente para compilar JP. Si no se especifica, Joss genera y gestiona una llave automáticamente por plugin bajo `~/.joss/keys/`.
- Los plugins `.jp` se ejecutan directamente en memoria (bytecode VM y AST Engine), con acceso al mapa de entorno `r.Env` según su ejecutor; esto no constituye un sandbox del sistema operativo.

## Seguridad, Páginas de Error y Modo Engaño

```env
APP_DEBUG="false"
SECURITY_DECEPTION="true"
```

- **Páginas de Error Nativas**: En lugar de texto plano, Joss sirve páginas de error visuales integradas con diseño moderno (SVG animado, código de estado y navegación al inicio). Puedes personalizar las vistas por proyecto en `app/views/errors/{status}.joss.html` (por ejemplo `errors/404` o `errors/500`). Si el cliente envía `Accept: application/json` o solicita rutas bajo `/api/`, responde automáticamente un JSON estructurado.
- **Modo Engaño (`SECURITY_DECEPTION="true"`)**: Ofusca accesos no autorizados o prohibidos (códigos `403 Forbidden` y `401 Unauthorized`) respondiendo al cliente con `404 Not Found`. Para el atacante o escáner externo el recurso parece inexistente, mientras que la consola del servidor registra el evento real con el prefijo `[SECURITY DECEPTION]` y la IP del cliente para auditoría.
- **Protección de Errores Internos (`APP_DEBUG="false"`)**: Evita la filtración de trazas de pila (stack traces), rutas de archivos o código fuente ante excepciones internas no controladas (código 500), mostrando un mensaje genérico seguro y enviando el diagnóstico detallado a los logs de consola.

No publiques archivos de entorno ni llaves privadas.

## CLI de base de datos

```bash
joss change db mysql
joss change db sqlite
joss change db postgres
joss change db prefix app_
```

La migración interactiva `joss change db migrate` sigue orientada a preparar un servidor MySQL nuevo. El runtime, CRUD, migraciones y Schema Builder sí funcionan con adaptadores para cuatro motores, con diferencias por operación.

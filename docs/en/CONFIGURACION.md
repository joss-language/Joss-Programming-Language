# Configuration

[Index](README.md)

The runtime looks for `env.joss`, then `env.enc` and finally `.env`; development also tries parent directories. Operating system variables overwrite the file. `System::env("KEY", "default")` queries the loaded environment.

```env
APP_ENV="development"
APP_URL="https://127.0.0.1:8443"
PORT="8443"
PREFIX="js_"
JWT_SECRET="secreto-largo-y-unico"
APP_KEY="clave-larga-y-unica"
```

`PREFIX` is the canonical key; `DB_PREFIX` is kept as an alias.

## Databases

SQLite:

```env
DB="sqlite"
DB_PATH="database.sqlite"
```

MySQL uses port 3306 when `DB_HOST` does not include it:

```env
DB="mysql"
DB_HOST="127.0.0.1:3306"
DB_NAME="mi_app"
DB_USER="usuario"
DB_PASS="secreto"
```

PostgreSQL uses port 5432 and accepts `DB_SSLMODE`:

```env
DB="postgres"
DB_HOST="127.0.0.1:5432"
DB_NAME="mi_app"
DB_USER="usuario"
DB_PASS="secreto"
DB_SSLMODE="require"
```

The aliases `postgresql` and `pgx` are also normalized to PostgreSQL.

Microsoft SQL Server uses port 1433 and accepts `DB_ENCRYPT`:

```env
DB="sqlserver"
DB_HOST="127.0.0.1:1433"
DB_NAME="mi_app"
DB_USER="sa"
DB_PASS="secreto"
DB_ENCRYPT="disable"
```

The aliases `mssql` and `mssqlserver` are also normalized to SQL Server.

## Server and sessions

```env
SESSION_DRIVER="file"
SESSION_FILE="storage/sessions.json"
RATE_LIMIT_REQUESTS="120"
RATE_LIMIT_WINDOW_SECONDS="60"
TLS_CERT_FILE="certs/fullchain.pem"
TLS_KEY_FILE="certs/private.key"
CORS_WEB="https://app.example.com"
```

- `file` is the default session driver and persists across reboots. `memory` is explicitly volatile.
- `SESSION_DRIVER="redis"` stores web sessions in Redis instead of disk.
- TLS certificate and key must be configured together.
- `CORS_WEB=*` allows any origin without credentials; a comma separated list creates an exact whitelist.

## Redis and Distributed Cache

Joss includes a native Redis client with auto-connection and session support:

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

- If `REDIS_URL` or `REDIS_HOST` are present, the class `Redis::set()`, `Redis::get()`, etc. It auto-connects on the first call.
- `SESSION_DRIVER="redis"` automatically uses these credentials.

## Processes and plugins

- `ALLOW_SYSTEM_RUN=true` enables `System::Run()`.
- `JOSS_PLUGIN_SIGNING_KEY` allows selecting an existing Ed25519 private key to compile JP. If not specified, Joss automatically generates and manages a key per plugin under `~/.joss/keys/`.
- The `.jp` plugins run directly in memory (VM bytecode and AST Engine), with access to the environment map `r.Env` according to their executor; this does not constitute an operating system sandbox.

Do not publish environment files or private keys.

## Database CLI

```bash
joss change db mysql
joss change db sqlite
joss change db postgres
joss change db prefix app_
```

The interactive migration `joss change db migrate` is still aimed at preparing a new MySQL server. The runtime, CRUD, migrations and Schema Builder do work with adapters for four engines, with differences per operation.

#Configuração

[Índice](README.md)

O tempo de execução procura por `env.joss`, depois por `env.enc` e finalmente por `.env`; o desenvolvimento também tenta diretórios pais. Variáveis ​​do sistema operacional substituem o arquivo. `System::env("KEY", "default")` consulta o ambiente carregado.```env
APP_ENV="development"
APP_URL="https://127.0.0.1:8443"
PORT="8443"
PREFIX="js_"
JWT_SECRET="secreto-largo-y-unico"
APP_KEY="clave-larga-y-unica"
```
`PREFIX` é a chave canônica; `DB_PREFIX` é mantido como um alias.

## Bancos de dados

SQLite:```env
DB="sqlite"
DB_PATH="database.sqlite"
```
MySQL usa a porta 3306 quando `DB_HOST` não a inclui:```env
DB="mysql"
DB_HOST="127.0.0.1:3306"
DB_NAME="mi_app"
DB_USER="usuario"
DB_PASS="secreto"
```
PostgreSQL usa a porta 5432 e aceita `DB_SSLMODE`:```env
DB="postgres"
DB_HOST="127.0.0.1:5432"
DB_NAME="mi_app"
DB_USER="usuario"
DB_PASS="secreto"
DB_SSLMODE="require"
```
Os aliases `postgresql` e `pgx` também são normalizados para PostgreSQL.

O Microsoft SQL Server usa a porta 1433 e aceita `DB_ENCRYPT`:```env
DB="sqlserver"
DB_HOST="127.0.0.1:1433"
DB_NAME="mi_app"
DB_USER="sa"
DB_PASS="secreto"
DB_ENCRYPT="disable"
```
Os aliases `mssql` e `mssqlserver` também são normalizados para SQL Server.

## Servidor e sessões```env
SESSION_DRIVER="file"
SESSION_FILE="storage/sessions.json"
RATE_LIMIT_REQUESTS="120"
RATE_LIMIT_WINDOW_SECONDS="60"
TLS_CERT_FILE="certs/fullchain.pem"
TLS_KEY_FILE="certs/private.key"
CORS_WEB="https://app.example.com"
```
- `file` é o driver de sessão padrão e persiste durante as reinicializações. `memory` é explicitamente volátil.
- `SESSION_DRIVER="redis"` armazena sessões da web no Redis em vez de no disco.
- O certificado e a chave TLS devem ser configurados juntos.
- `CORS_WEB=*` permite qualquer origem sem credenciais; uma lista separada por vírgulas cria uma lista de permissões exata.

## Redis e cache distribuído

Joss inclui um cliente Redis nativo com conexão automática e suporte de sessão:```env
# Opción A: URL completa estándar
REDIS_URL="redis://default:password@127.0.0.1:6379/0"

# Opción B: Variables individuales (puerto 6379 y DB 0 por defecto)
REDIS_HOST="127.0.0.1"
REDIS_PORT="6379"
REDIS_USER="default"
REDIS_PASSWORD="password"
REDIS_DB="0"
```
- Se `REDIS_URL` ou `REDIS_HOST` estiver presente, a classe `Redis::set()`, `Redis::get()`, etc. Ela se conecta automaticamente na primeira chamada.
- `SESSION_DRIVER="redis"` usa automaticamente essas credenciais.

## Processos e plug-ins

- `ALLOW_SYSTEM_RUN=true` habilita `System::Run()`.
- `JOSS_PLUGIN_SIGNING_KEY` permite que você selecione uma chave privada Ed25519 existente para compilar JP. Se não for especificado, Joss gera e gerencia uma chave automaticamente por plugin em `~/.joss/keys/`.
- Plugins `.jp` rodam diretamente na memória (bytecode VM e AST Engine), com acesso ao mapa do ambiente `r.Env` dependendo do seu executor; isso não constitui uma sandbox do sistema operacional.

Não publique arquivos de ambiente ou chaves privadas.

## CLI do banco de dados```bash
joss change db mysql
joss change db sqlite
joss change db postgres
joss change db prefix app_
```
A migração interativa `joss change db migrate` ainda visa preparar um novo servidor MySQL. O runtime, CRUD, migrações e Schema Builder funcionam com adaptadores para quatro mecanismos, com diferenças por operação.
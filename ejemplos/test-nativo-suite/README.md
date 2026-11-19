# Joss Native Test Suite (`v3.6.8.3`)

Proyecto de verificación integral para probar la paridad entre el runtime Go y el compilador nativo (`--native`):

- **Auto-DP**: `fib(35)` memoizado automáticamente.
- **Concurrencia paralela**: `async { ... }` / `await` sobre pool multi-core `pthread`.
- **Auth**: `PBKDF2-HMAC-SHA256` (100,000 iteraciones) y **JWT HS256**.
- **MFA**: **RFC 6238 TOTP** (`HMAC-SHA1` + Base32, vector oficial `287082`).
- **GranDB + Excepciones**: CRUD SQLite3 + `GranDB::transaction` con `ROLLBACK` vía `try/catch/throw`.

## Ejecución

```bash
# Modo intérprete Go
DB_CONNECTION=sqlite DB_DATABASE=suite.sqlite joss run main.joss

# Compilación nativa standalone (cero Go runtime, cero DLLs de Joss)
DB_CONNECTION=sqlite DB_DATABASE=suite.sqlite joss build main.joss --native
DB_CONNECTION=sqlite DB_DATABASE=suite.sqlite ./build/main
```

# 11. Plan de Acción y Hoja de Ruta Post-Auditoría (Remediation Roadmap)

Esta hoja de ruta establece las acciones de remediación priorizadas para elevar la madurez de Joss Programming Language hacia un estándar de publicación y presentación pública transparente.

---

## Fase P0 — Bloqueantes de Lanzamiento y Transparencia Técnica

### Acción P0.1: Transparencia en la Compilación Standalone
- **Problema de Origen:** `HAL-01` (Fallback silencioso de `joss build` al runner empaquetado en Go).
- **Archivos Afectados:** `cmd/joss/main.go`, `cmd/joss/native_builder.go`, `docs/COMPILACION.md`.
- **Solución Recomendada:**
  1. Separar explícitamente los comandos de compilación en el CLI:
     - `joss package` o `joss build --bundle`: Empaqueta el proyecto con el runner Go autónomo (comportamiento predeterminado y estable).
     - `joss build --native`: Intenta la compilación AOT real con LLVM; si una característica no es soportada por el IR, debe **fallar con un diagnóstico explícito**, nunca recurrir a un fallback silencioso.
  2. Actualizar la documentación técnica para reflejar con exactitud la naturaleza del binario generado.
- **Criterio de Finalización:** Ningún ejecutable que contenga el intérprete Go embebido se genera sin que el usuario haya sido informado explícitamente.

---

## Fase P1 — Confiabilidad, Seguridad y Corrección Semántica

### Acción P1.1: Corrección de Entropía Criptográfica en 2FA/OTP
- **Problema de Origen:** `HAL-03` (Uso de `math/rand` en códigos MFA).
- **Archivos Afectados:** `pkg/core/auth.go`, `pkg/core/auth_fluent.go`.
- **Solución Recomendada:** Reemplazar las invocaciones a `rand.Intn` por `crypto/rand`.
- **Criterio de Finalización:** Pruebas unitarias que validen la imposibilidad de predicción de secuencias y rechacen paquetes no criptográficos.

### Acción P1.2: Remediación de Fuga de Credenciales en Middleware CSRF
- **Problema de Origen:** `HAL-04` (Impresión de sesión y tokens en stdout).
- **Archivos Afectados:** `pkg/server/handler_routing.go`.
- **Solución Recomendada:** Eliminar `fmt.Printf` de depuración y sustituir la igualdad de strings por `subtle.ConstantTimeCompare([]byte(csrfToken), []byte(reqToken)) == 1`.
- **Criterio de Finalización:** Verificación en logs de que ninguna solicitud POST/PUT emite el ID de sesión a consola.

### Acción P1.3: Validación de Origen en WebSockets
- **Problema de Origen:** `HAL-05` (`CheckOrigin` siempre retorna `true`).
- **Archivos Afectados:** `pkg/server/websocket.go`.
- **Solución Recomendada:** Implementar verificación del encabezado `Origin` contra el host configurado en la aplicación, permitiendo orígenes adicionales mediante configuración en `joss.yaml`.
- **Criterio de Finalización:** Prueba automatizada que verifique que una solicitud WebSocket con `Origin: http://evil.com` es rechazada con código HTTP 403 Forbidden.

### Acción P1.4: Corrección de Strings no Terminadas en el Lexer
- **Problema de Origen:** `HAL-02` (Lexer acepta `"texto` sin comillas de cierre).
- **Archivos Afectados:** `pkg/parser/lexer.go`.
- **Solución Recomendada:** Agregar verificación de salto de línea (`\n`) en el bucle de lectura de tokens literales de cadena; si se encuentra un salto de línea antes de la comilla de cierre, emitir diagnóstico de error de token.
- **Criterio de Finalización:** El caso de prueba `print("hola)` debe fallar con `JOSS-PARSE-001`.

---

## Fase P2 — Saneamiento y Honestidad Académica de la Tesis

### Acción P2.1: Depuración Integral de Scripts de Relleno en la Tesis
- **Problema de Origen:** `HAL-06` (Scripts de expansión programática para inflar páginas).
- **Archivos Afectados:** `C:\Users\joss\Documents\tesis\scripts\expand-*.mjs`, `mega-expand.mjs`, capítulos TSX.
- **Solución Recomendada:**
  1. Eliminar los scripts de relleno y depurar los capítulos TSX eliminando párrafos y secciones redundantes generadas artificialmente.
  2. Ajustar la tesis a su extensión natural y orgánica (aunque sea de 120-150 páginas de contenido genuino).
- **Criterio de Finalización:** Cero código repetido por inyección programática en los archivos de la tesis.

### Acción P2.2: Construcción de Repositorio de Reproducibilidad Empírica
- **Problema de Origen:** `HAL-07` (Falta de datos brutos para los benchmarks del Capítulo 9).
- **Archivos Afectados:** `research/evidence/`, arneses de benchmarking.
- **Solución Recomendada:** Crear una suite reproducible con contenedores Docker que ejecute `wrk` contra los 6 stacks tecnológicos en condiciones idénticas y registre los archivos CSV/JSON brutos.
- **Criterio de Finalización:** Cualquier revisor externo puede ejecutar `./run_benchmarks.sh` y reproducir los números presentados.

---

## Fase P3 — Higiene de Ecosistema y Documentación

### Acción P3.1: Alineación de Versionado y CHANGELOG
- **Archivos Afectados:** `CHANGELOG.md`.
- **Solución:** Documentar las versiones intermedias desde la 3.6.5 hasta la 3.6.7.8.

### Acción P3.2: Limpieza de Archivos Extraños en el Repositorio
- **Acción:** Eliminar o colocar en `.gitignore` los binarios compilados de la raíz (`runner.exe`, `joss-android-arm64`) y renombrar la carpeta `hestia cp install` corrigiendo errores tipográficos.

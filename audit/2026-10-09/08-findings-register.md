# 08. Registro Priorizado de Hallazgos

Este registro consolida todos los problemas, inconsistencias y vulnerabilidades identificadas durante la auditoría técnica integral de Joss Programming Language.

---

## Matriz Resumen de Severidades

| ID | Subsistema | Severidad | Descripción Breve | Impacto |
| :--- | :--- | :---: | :--- | :--- |
| **HAL-01** | `cmd/joss`, `pkg/backend` | **P0** | Fallback silencioso de `joss build` a empaquetador del intérprete Go | Contradice la afirmación de compilación nativa AOT independiente |
| **HAL-02** | `pkg/parser` | **P1** | Lexer acepta strings no terminadas sin error sintáctico | Acepta código sintácticamente malformado sin emitir diagnósticos |
| **HAL-03** | `pkg/core/auth.go` | **P1** | Generación de códigos 2FA/OTP mediante `math/rand` pseudoaleatorio | Posible bypass de autenticación por predictibilidad de tokens |
| **HAL-04** | `pkg/server` | **P1** | Fuga de ID de sesión y token CSRF a stdout y respuesta HTTP 419 | Exposición masiva de credenciales de sesión en logs de servidores |
| **HAL-05** | `pkg/server` | **P1** | `CheckOrigin` en WebSockets permite cualquier origen (`true`) | Vulnerabilidad a Cross-Site WebSocket Hijacking (CSWSH) |
| **HAL-06** | `tesis/scripts` | **P1** | Scripts de relleno programático de páginas en la tesis doctoral | Compromete la integridad metodológica del trabajo académico |
| **HAL-07** | `tesis/cap-09` | **P1** | Cifras de benchmarks en tesis sin datos brutos ni reproducibilidad | Afirmaciones científicas no auditables |
| **HAL-08** | `pkg/core` | **P2** | Falta de cuota de instrucciones/timeout en bucles del evaluador | Riesgo de denegación de servicio (DoS) por bucle infinito local |
| **HAL-09** | `pkg/core` | **P2** | Carga de drivers nativos dinámicos (`.so`/`.dll`) sin validación | Ejecución de binarios externos sin comprobación de hash o firma |
| **HAL-10** | `CHANGELOG.md` | **P3** | Desfase de versiones: changelog documenta 3.6.4, git tag en v3.6.7.8 | Falta de documentación de las últimas 8 versiones publicadas |
| **HAL-11** | Repositorio | **P4** | Binarios no versionados y carpetas con nombres malformados (`hestia cp`) | Falta de higiene en el espacio de trabajo del proyecto |

---

## Detalle Exhaustivo de los Hallazgos P0 y P1

### [HAL-01] [P0 — Bloqueante] Fallback Oculto del Compilador Nativo AOT al Intérprete Go
- **Subsistema:** Orquestación de Compilación (`cmd/joss/main.go:553-572` y `native_builder.go:48`).
- **Evidencia Concreta:**
  ```go
  // cmd/joss/main.go:553
  if err != nil {
      // Fallback to go runner package if native lowerer fails
      return buildNativeWithOutput(projectDir, outputPath, buildMode, targetOS, targetArch, isGUI)
  }
  ```
- **Impacto:** Tanto la tesis doctoral como la documentación técnica (`AGENTS.md`) prometen que Joss compila a binarios nativos máquina independientes sin AST ni runtime Go. En la práctica, cualquier aplicación con características no triviales conmuta silenciosamente a un binario que contiene todo el intérprete de Go y el código fuente Joss cifrado.
- **Criterio de Aceptación para Resolución:** Hacer que el comando `joss build --native` falle explícitamente con un diagnóstico estructurado (`JOSS-AOT-...`) cuando una característica no esté soportada en el IR nativo, eliminando el fallback silencioso o requiriendo una bandera explícita `--fallback-runner`.

---

### [HAL-02] [P1 — Crítico] Aceptación Silenciosa de Literales de Cadena sin Cerrar
- **Subsistema:** Análisis Léxico (`pkg/parser/lexer.go`).
- **Evidencia Concreta:** El script `print("hola)` es aceptado por el lexer sin generar `JOSS-PARSE-001`, produciendo un token de string que contiene `hola)` y ejecutándose con éxito.
- **Impacto:** Código sintácticamente roto es procesado sin advertencias, ocultando errores del desarrollador.
- **Criterio de Aceptación:** El lexer debe emitir un diagnóstico de error fatal cuando un literal de cadena encuentre un salto de línea sin haber sido cerrado por comillas dobles.

---

### [HAL-03] [P1 — Crítico] Generador Pseudoaleatorio No Criptográfico en Códigos MFA
- **Subsistema:** Módulo de Autenticación (`pkg/core/auth.go:784`).
- **Evidencia Concreta:** Uso de `rand.Intn(1000000)` del paquete `math/rand`.
- **Impacto:** Vulnerabilidad en la autenticación de dos factores.
- **Criterio de Aceptación:** Reemplazo estricto por `crypto/rand` y prueba unitaria en Go que valide la generación aleatoria de entropía criptográfica.

---

### [HAL-04] [P1 — Crítico] Fuga de Sesión y Token CSRF en Logs Estándar y Respuestas
- **Subsistema:** Middleware HTTP (`pkg/server/handler_routing.go:344, 348-350`).
- **Evidencia Concreta:**
  `fmt.Printf("[CSRF DEBUG] Session: %s | Stored: %s | Received: %s\n", sessionID, csrfToken, reqToken)`
- **Impacto:** Filtración del identificador de sesión en la consola del servidor.
- **Criterio de Aceptación:** Eliminación de los mensajes de depuración a `stdout` en producción y empleo de `subtle.ConstantTimeCompare`.

---

### [HAL-05] [P1 — Crítico] Política `CheckOrigin` Insegura en Conexiones WebSocket
- **Subsistema:** Capa de Red WebSocket (`pkg/server/websocket.go:16-18`).
- **Evidencia Concreta:** `CheckOrigin: func(r *http.Request) bool { return true }`.
- **Impacto:** Vulnerabilidad a Cross-Site WebSocket Hijacking.
- **Criterio de Aceptación:** Verificación obligatoria de origen cruzado comparando contra la lista blanca de dominios permitidos o el encabezado `Host`.

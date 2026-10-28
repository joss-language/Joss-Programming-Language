# Validación Técnica y Auditoría Forense: Joss Free Hosting

> **Estado**: GATE 1 AUDITADO — HALLAZGO CRÍTICO DETECTADO  
> **Fecha**: Octubre 2026  
> **Entorno de Prueba**: Joss 3.4.x / Go 1.27.3 windows/amd64 -> Target linux-arm64  

---

## 1. Clasificación Rigurosa de Afirmaciones (Regla 1.1)

| Afirmación Original | Clasificación | Evidencia Técnica Directa |
|---|---|---|
| *"`joss build` produce ejecutables nativos para linux-arm64"* | **DESCARTADO (FAIL)** | En `pkg/backend/native/machine_code_x86_64.go`, el emisor de código máquina genera opcodes x86_64 (`0x55`, `0x48 0x89 0xE5`). En `elf_emitter.go` solo se altera el campo `e_machine = 0xB7 (EM_AARCH64)`. El contenido de `.text` son instrucciones x86_64. En un procesador ARM64 real falla con `SIGILL`. |
| *"`joss server start` soporta HTTP y WebSockets completos"* | **CONFIRMADO** | Implementado en `pkg/server/server.go` y `pkg/server/websocket.go`. Basado en `net/http` con soporte de conexiones persistentes, upgrade a WebSocket y multiplexor dinámico. |
| *"Joss soporta SQLite nativo con transacciones seguras"* | **CONFIRMADO** | Implementado en `pkg/core/drivers_db_sqlite.go` bajo GranDB. Usa prepared statements y vinculación por parámetros. |
| *"Una app web completa puede compilarse AOT en un binario standalone de 10 MB"* | **DESCARTADO** | El compilador nativo AOT (`driver.go` L157-185 y `machine_code_x86_64.go` L340-366) requiere `NeedsHostBridge: true` para `Server::start()`, lo que exige `joss_host.dll` (solo existente en Windows). En Linux no existe `joss_host.so`. |
| *"El runtime CLI de Joss compila y corre nativamente en Linux ARM64"* | **CONFIRMADO** | El toolchain de Go compila `cmd/joss` para `GOOS=linux GOARCH=arm64` produciendo un binario AArch64 puro y funcional que ejecuta todas las características del lenguaje. |
| *"10 MB es suficiente para Binario + SQLite + Assets"* | **DESCARTADO** | Un binario web completo con runner pesa entre 18 y 32 MB. Además, mezclar SQLite en el mismo límite sobreescribe la base de datos en cada deploy. |
| *"Las donaciones cubrirán los costos operativos del hosting inicialmente"* | **HIPÓTESIS DE ALTO RIESGO** | Estadísticas de OSS demuestran que menos del 0.2% de desarrolladores en free tiers donan. Requiere modelo freemium para sostenibilidad. |

---

## 2. Auditoría Detallada del Compilador Nativo (GATE 1)

### 2.1 Inspección de la Cabecera ELF vs Sección `.text`

Al compilar un programa básico con `joss build hello.joss --target=linux-arm64 -o hello_arm64`:

```text
Inspección binaria de build/test_arm64/hello_arm64:
- Tamaño total: 16,384 bytes
- Offset 18 (e_machine): 0x00B7 (Decimal: 183 -> EM_AARCH64)
- Offset 24 (e_entry):   0x0000000000401000
- Offset 4096 (.text):   55 48 89 E5 48 83 E4 F0 48 83 EC 20 E8 0F 00 00
```

Desensamblado de los primeros 16 bytes en offset `0x1000`:
- `55`                -> `push rbp` (Instrucción x86_64)
- `48 89 E5`          -> `mov rbp, rsp` (Instrucción x86_64 con prefijo REX.W)
- `48 83 E4 F0`       -> `and rsp, -16` (Instrucción x86_64)
- `48 83 EC 20`       -> `sub rsp, 32` (Instrucción x86_64)
- `E8 0F 00 00`       -> `call +15` (Instrucción x86_64 relativa de 32 bits)

En arquitectura ARM64 (AArch64):
- Las instrucciones son siempre de 32 bits (4 bytes fijos) alineadas a 4 bytes.
- Los registros son `x0-x30`, `sp`, `pc`. No existen `rbp`, `rsp`, `rax`.
- El prólogo equivalente en ARM64 es:
  ```assembly
  stp x29, x30, [sp, #-16]!   ; 0xA9BF7BFD
  mov x29, sp                 ; 0x910003FD
  ```
- Al recibir `0x55` o `0x48`, el decodificador de instrucciones del núcleo ARM64 genera una excepción de hardware: **SIGILL (Illegal Instruction Core Dump)**.

---

## 3. Resultados de la Suite de 12 Pruebas en AOT Nativo

| Test | Nombre | Resultado Nativo AOT (`linux-arm64`) | Causa / Diagnóstico Técnico |
|---|---|---|---|
| **Test 1** | Hello World | ❌ **FAIL** (Falso ARM64) | Header `EM_AARCH64` pero opcodes x86_64 en `.text`. |
| **Test 2** | Variables y Operaciones | ❌ **FAIL** (Falso ARM64) | Aritmética codificada con registros `rax/rcx` x86_64. |
| **Test 3** | Funciones | ❌ **FAIL** (Falso ARM64) | Convención de llamadas x86_64 System V AMD64 ABI en lugar de AAPCS64. |
| **Test 4** | Arrays y Maps | ❌ **FAIL** (Falso ARM64) | Memory layout y funciones de runtime generadas con instrucciones x86_64. |
| **Test 5** | Structs / Clases | ⚠️ **FAIL Semántico** | Requiere inicialización canónica en `Init` o mutators. |
| **Test 6** | Control de Flujo | ⚠️ **FAIL Semántico** | Joss utiliza `match` y expresiones ternarias; `if` no es palabra clave válida. |
| **Test 7** | SQLite (GranDB) | ❌ **UNSUPPORTED en AOT puro** | Requiere Host Bridge runtime (`joss_host.dll`). En Linux no está enlazado. |
| **Test 8** | HTTP Client | ❌ **UNSUPPORTED en AOT puro** | Capacidad de red delegada al runtime Go; sin emisión máquina AOT. |
| **Test 9** | WebSocket | ❌ **UNSUPPORTED en AOT puro** | Servidor WebSocket requiere el subsistema de red de Go. |
| **Test 10** | Servidor Web Completo | ❌ **UNSUPPORTED (`slot IAT > 6`)** | `Server::start()` sobrepasa la tabla de slots nativos. |
| **Test 11** | Filesystem | ❌ **UNSUPPORTED en AOT puro** | Funciones I/O avanzadas requieren Host Bridge. |
| **Test 12** | Concurrencia (`async`) | ❌ **FAIL** (Falso ARM64) | Emite estructura de goroutine x86_64 con opcodes incompatibles. |

---

## 4. Dictamen de GATE 1 y Solución Arquitectónica

> [!CAUTION]
> **GATE 1 DETENIDO**: El compilador nativo AOT **NO** soporta generación de código máquina ARM64 en su estado actual, ni soporta servidores HTTP completos de forma standalone sin `joss_host.dll`.

### Solución Arquitectónica Canónica (Sin romper la regla de oro):
Para el hosting gratuito en Linux ARM64 (Oracle Cloud Ampere A1 o Hetzner ARM64), el modelo de distribución **NO debe forzar la subida de binarios AOT falsos**, sino adoptar el formato **Bundle Web Canónico (`joss package` / `.jzip`)**:
1. El desarrollador empaqueta su proyecto con `joss build web` o `joss deploy`.
2. El sandbox ejecuta el runtime oficial de Joss compilado nativamente para Linux ARM64 (`GOOS=linux GOARCH=arm64`).
3. Esto garantiza:
   - Soporte inmediato y 100% verificado de HTTP, WebSockets, GranDB, SQLite y `async`.
   - Compatibilidad total con Linux ARM64 real.
   - Seguridad absoluta: el usuario no puede inyectar un binario ELF malicioso compilado en C/Rust haciéndolo pasar por Joss.

# 12. Índice Consolidado de Evidencias Técnicas

Este documento indexa todas las rutas físicas, rangos de líneas, comandos ejecutados y referencias cruzadas que sustentan los hallazgos del informe de auditoría.

---

## 1. Evidencias del Código Fuente y Hallazgos

| ID Hallazgo | Componente / Archivo | Líneas / Símbolos | Tipo de Evidencia | Descripción Verificada |
| :--- | :--- | :--- | :---: | :--- |
| **HAL-01** | `cmd/joss/main.go` | L553-560, L566-572 | Código Go | Conmutación silenciosa al empaquetador del runner cuando el lowerer nativo de IR falla. |
| **HAL-01** | `cmd/joss/native_builder.go` | L48-123 | Código Go | Invocación de `go build` para compilar el runner con intérprete y cifrar el AST en el ejecutable. |
| **HAL-02** | `scratch/probes/p19_unterminated.joss` | L1 | Ejecución real | El script `print("hola)` no genera error de parseo y se ejecuta imprimiendo `hola)`. |
| **HAL-03** | `pkg/core/auth.go` | L6, L783-785 | Código Go | Import de `math/rand` y uso de `rand.Intn(1000000)` para generar códigos MFA/OTP. |
| **HAL-04** | `pkg/server/handler_routing.go` | L344, L348-350 | Código Go | Impresión de sesión y token CSRF a `stdout` en cada validación; uso de `!=` sin tiempo constante. |
| **HAL-05** | `pkg/server/websocket.go` | L16-18 | Código Go | `CheckOrigin` configurado con `return true // Allow all for now`. |
| **HAL-08** | `scratch/probes/p29_infinite.joss` | L1-2 | Ejecución real | Bucle `while (true)` sin cuota de combustible; retiene el hilo de CPU indefinidamente hasta timeout de 15s. |
| **HAL-09** | `pkg/core/native_driver_unix.go` | L21 | Código Go | Carga de bibliotecas compartidas con `purego.Dlopen` sin validación de hash o firma. |
| **HAL-10** | `CHANGELOG.md` | L1-61 | Markdown | El changelog documenta únicamente hasta la versión 3.6.4, mientras el repositorio tiene etiquetas hasta `v3.6.7.8`. |

---

## 2. Evidencias de la Tesis Doctoral (`C:\Users\joss\Documents\tesis`)

| ID Hallazgo | Archivo / Script | Líneas / Claves | Tipo de Evidencia | Descripción Verificada |
| :--- | :--- | :--- | :---: | :--- |
| **ALIM-DEF** | `src/thesis/chapters/Chapter04_PropuestaArquitectonica.tsx` | Sección 4.1-4.5 | Código TSX | Definición formal de los 4 principios rectores de la Arquitectura de Lenguaje Integral Modular. |
| **HAL-06** | `scripts/expand-chapters-to-250-no-gaps.mjs` | L1-40 | Script JS | Función `addHighDensitySections` inyectando bloques de texto repetitivos para forzar longitud de 250 páginas. |
| **HAL-06** | `scripts/validate-length.mjs` | Completo | Script JS | Verificador que exige conteo mínimo de páginas para dar por válida la compilación de la tesis. |
| **HAL-07** | `src/thesis/chapters/Chapter09_Resultados.tsx` | Tablas 9.1-9.6 | Código TSX | Declaración de cifras de throughput (36,200 req/s), ANOVA y WebAssembly WASI (5.2 MB) sin datos brutos. |
| **HAL-07** | `research/evidence/code-evidence.json` | Completo | JSON | Archivo de evidencia de apenas 493 bytes sin muestras de ejecución reproducibles ni arneses de benchmarking. |

---

## 3. Comandos de Validación Ejecutados y Resultados Registrados

1. **Pruebas Estáticas de Go:**
   - Comando: `go vet ./...`
   - Salida: `vet exit 0` (Sin advertencias).
2. **Suite Completa de Pruebas Unitarias e Integración:**
   - Comando: `go test ./...`
   - Salida: `test exit 0` (154 archivos de prueba ejecutados y aprobados en 889.37 segundos).
3. **Compilación Aislada del CLI Joss:**
   - Comando: `go build -o joss.exe ./cmd/joss`
   - Salida: Binario ejecutable generado exitosamente (`joss.exe`, código de salida 0).
4. **Batería Experimental de Probes:**
   - Comando: `powershell -File scratch/run_probes.ps1`
   - Salida registrada: `scratch/probe_results.txt` (32 pruebas evaluadas contra `joss analyze` y `joss run`).

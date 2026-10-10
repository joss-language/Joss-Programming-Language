# 01. Mapa del Repositorio, Arquitectura y Pipelines de Ejecución

**Proyecto:** Joss Programming Language  
**Commit Analizado:** `10fd5fc` (v3.6.7.8)  
**Fecha de Auditoría:** 9 de octubre de 2026  
**Auditor:** Comité Técnico de Auditoría Independiente  

---

## 1. Inventario Estructural y Métricas del Repositorio

El repositorio principal de Joss está implementado íntegramente en el lenguaje **Go** (versión Go 1.27.1 comprobada en entorno Windows x86-64).

### 1.1 Distribución de Paquetes y Código Fuente

| Paquete / Directorio | Archivos `.go` | Archivos `*_test.go` | LOC Aprox. | Propósito Principal |
| :--- | :---: | :---: | :---: | :--- |
| `pkg/core` | 170 | 59 | 35,118 | Evaluador AST de ejecución, runtime en Go, frames léxicos, built-ins, clases nativas y ORM GranDB. |
| `cmd/joss` | 38 | 8 | 8,757 | CLI unificada, comandos de scaffolding (`new`, `make:*`), orquestación de compilación y servidor. |
| `pkg/analyzer` | 36 | 19 | 6,831 | Pipeline de análisis semántico, verificación de tipos nominales/uniones, alcance léxico y alcance de programa canónico (`PreparedProgram`). |
| `pkg/parser` | 26 | 15 | 5,901 | Lexer de tokens, parser descendente de precedencia Pratt y definiciones de AST. |
| `pkg/server` | 19 | 7 | 3,309 | Servidor HTTP multinivel, multiplexor, WebSockets nativos, CSRF y sesiones. |
| `pkg/ir` | 10 | 4 | 2,773 | Representación Intermedia (Joss Native IR), lowering desde `PreparedProgram`, 3AC/SSA y Verificador de tipos de bajo nivel (`ir.Verifier`). |
| `pkg/template/files` | 10 | 0 | 2,260 | Plantillas embebidas para generación de proyectos y CRUDs. |
| `pkg/pluginruntime` | 12 | 4 | 2,020 | Aislamiento y ejecución en runtime de paquetes de plugins `.jp`. |
| `pkg/backend/native`| 7 | 1 | 1,641 | Backends de compilación nativa AOT (emisión LLVM `.ll` y backend Standalone en código Go). |
| `pkg/pluginpkg` | 7 | 3 | 1,267 | Empaquetado, descompresión, índice de símbolos y validación criptográfica de paquetes `.jp`. |
| `pkg/formatter` | 5 | 2 | 1,127 | Formateador canónico del código fuente Joss preservando trivia. |
| `pkg/typesystem` | 7 | 5 | 1,010 | Sistema de tipos canónico, compatibilidad (`Assignable`), tipos primitivos y uniones. |
| `pkg/vm` | 8 | 3 | 794 | Máquina Virtual experimental de bytecode basada en pila. |
| `pkg/plugincompiler`| 3 | 2 | 610 | Frontend experimental políglota para traducir otros lenguajes a IRModule/JPBC. |
| `pkg/diagnostics` | 2 | 0 | 93 | Modelo formal de diagnósticos estructurados (`JOSS-...`). |
| **Total General** | **340+** | **154** | **~73,000** | **Base de código activa** |

---

## 2. Los Flujos y Pipelines de Ejecución Reales

### 2.1 Flujo Canónico del Intérprete (`joss run`)
Punto de entrada: `cmd/joss/main.go:625` (`executeScript`).

```
[Código .joss] 
      │
      ▼
pkg/parser (Lexer + Parser Pratt) ──▶ AST
      │
      ▼
pkg/analyzer (LoadProject -> AnalyzeSourceUnits)
  - collectDeclarations
  - projectScope
  - validateNominalContracts
  - analyzeSourceBodies
      │
      ├─ [¿Errores Semánticos?] ──▶ Aborta con código de salida 1 (JOSS-...)
      ▼
PreparedProgram (Inmutable)
      │
      ▼
pkg/core (Runtime.ExecutePrepared)
      │
  - Creación de Frame Global
  - Instanciación de Clases Nativas y Builtins
  - Evaluación del AST
      │
      ▼
[Salida Observable / Efectos]
```

**Verificación:** La ejecución directa mediante `joss run <script.joss>` ejecuta rigurosamente el pipeline del analizador estático antes de interpretar el AST. Si se detectan errores de tipos o símbolos no declarados, la ejecución se bloquea inmediatamente.

### 2.2 Flujo del Servidor Web (`joss server start`)
Punto de entrada: `cmd/joss/main.go:94` y `pkg/server/server.go`.
1. Inicializa el entorno (`env.joss`) e inicializa el runtime core.
2. Despacha `main.joss` o las rutas declaradas en `routes.joss` mediante el multiplexor nativo.
3. El watchdog de desarrollo (`hotreload.go`) supervisa cambios en archivos locales. Se detecta una discrepancia: en la recarga rápida de ciertos controladores, el código parsed se ejecuta directamente en `currentRuntime.Execute` sin pasar por todo el pipeline formal del `analyzer`.

### 2.3 Flujo del Compilador Nativo (`joss build`) y el Fallback Oculto
Punto de entrada: `cmd/joss/main.go:408` (`handleBuildCommand`) y `cmd/joss/main.go:505` (`buildNativeProgram`).

La regla de arquitectura en `AGENTS.md` estipula:
> *"pkg/backend/native: Generadores de código nativo para compilación AOT standalone y emisión LLVM; genera ejecutables independientes de máquina real sin AST serializado ni runtime Go embebido."*

**Hallazgo Crítico de Auditoría (HAL-01):**
Al auditar `cmd/joss/main.go` (líneas 553-560 y 566-572), se descubrió que cuando `ir.LowerProgram` o `ir.Verify` fallan, o cuando el programa utiliza características avanzadas del lenguaje que no están soportadas en el backend LLVM/IR (como el ORM GranDB, HTTP Server o llamadas dinámicas completas), el comando `joss build` conmuta silenciosamente a `buildNativeWithOutput` (`cmd/joss/native_builder.go:48`).
Este constructor:
1. Recopila y cifra el AST/código fuente del proyecto (`collectAndEncryptAssets`).
2. Invoca al compilador de Go del sistema (`exec.Command("go", "build", ...)`).
3. Compila el binario `cmd/runner` con todo el runtime de Go y el intérprete AST embebido.
4. Concatena el runner compilado con el payload cifrado, produciendo un ejecutable standalone de 30-40 MB.

**Dictamen:** Aunque funcional como ejecutable autónomo para el usuario final, **no es una compilación nativa directa del código fuente a lenguaje máquina real**, sino un empaquetado de runtime Go autoextraíble con intérprete de AST embebido.

---

## 3. Violaciones Arquitectónicas y Componentes Extraños

1. **Dirección de Dependencias:**  
   Se verificó que `pkg/analyzer` no importa `pkg/core` ni `pkg/server`, cumpliendo estrictamente con la regla unidireccional.
2. **Archivos Binarios no Versionados en el Workspace:**  
   En la raíz del proyecto existen binarios como `runner.exe` (38.5 MB), `runner` (36.7 MB), `joss-android-arm64` (28.5 MB) y en `cmd/joss/runner_windows.exe` (32.5 MB). No están versionados en Git pero ocupan espacio significativo en el entorno de trabajo.
3. **Carpetas con Errores Tipográficos:**  
   Existe el directorio `hestia cp install` en la raíz con el archivo `IMSTALLATION_HESTIA_CP.md`, lo cual denota falta de higiene de directorios previa a empaquetado público.

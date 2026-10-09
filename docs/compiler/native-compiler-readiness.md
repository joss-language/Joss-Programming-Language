# AUDITORÍA ARQUITECTÓNICA DE JOSS: PREPARACIÓN PARA COMPILADOR NATIVO REAL

**Fecha de Auditoría:** Octubre 2026  
**Objetivo:** Determinar el estado real del lenguaje Joss y evaluar su viabilidad técnica para evolucionar de un modelo empaquetado/interpretado hacia un compilador generador de binarios nativos reales (código máquina / ejecutables autocontenidos).

---

## 1. INVENTARIO COMPLETO DEL PROYECTO

### 1.1 Estructura del Repositorio

El repositorio no es un compilador nativo hoy en día, sino una infraestructura robusta de intérprete AST + empaquetador distribuido en Go:

```text
├── cmd/
│   ├── joss/              # CLI principal: comandos 'run', 'build', 'lint', 'check', 'format', etc.
│   ├── runner/            # Binario stub ejecutable que desempaqueta VFS + evalúa AST
│   └── setup_db/          # Utilitario para provisionar bases de datos de desarrollo
├── pkg/
│   ├── parser/            # Lexer, Tokens, Pratt Parser y AST canónico
│   ├── typesystem/        # Sistema canónico de tipos, inferencia, compatibilidad y uniones
│   ├── diagnostics/       # Modelo formal de diagnósticos de error y advertencia
│   ├── analyzer/          # Análisis semántico, scopes, tipado estático, reachability y facts
│   ├── core/              # Runtime Go principal: evaluador de AST, frames, clases nativas y built-ins
│   ├── bytecode/          # Formato de serialización AST comprimido ('JOSSBC2Z')
│   ├── vm/                # Prototipo experimental de máquina virtual de pila (bytecode opcode-based)
│   ├── plugincompiler/    # Compilador de plugins Joss con IR propio (Joss Plugin IR) y backend JPBC/PHP
│   ├── pluginpkg/         # Empaquetado y metadatos de paquetes de plugins (.jp)
│   ├── pluginruntime/     # Motor de ejecución y aislamiento para plugins externos
│   ├── buildmanifest/     # Generación de manifiestos de compilación (.joss/cache)
│   ├── buildcache/        # Cache de compilación incremental de artefactos
│   ├── server/            # Servidor HTTP/WebSocket y enrutador web integrado
│   ├── viewtemplate/      # Motor de plantillas (.joss.html) con cálculo UTF-8
│   ├── template/          # Generadores de código y scaffolding (make:controller, make:model, etc.)
│   ├── linter/            # Reglas de estilo y calidad de código
│   ├── fixer/             # Correcciones automáticas AST/token-safe (joss fix)
│   ├── formatter/         # Formateador de código con preservación de trivia
│   ├── tester/            # Runner de pruebas unitarias (*_test.joss)
│   ├── mobile/            # Adaptador y bindings para Android/Termux e iOS
│   ├── vfs/               # Sistema de archivos virtual en memoria (MemFS)
│   ├── crypto/            # Primitivas criptográficas (AES-256-GCM, PBKDF2) para empaquetado seguro
│   └── i18n/              # Internacionalización CLI con soporte de 30 idiomas (.arb)
├── docs/                  # Documentación canónica y especificación del lenguaje
├── ejemplos/              # Proyectos de demostración (incluyendo JosSecurity Web Suite)
├── tools/                 # Generadores de tooling: cataloggen, docgen, docsi18n
└── vscode-joss/           # Extensión oficial de editor y Language Server (LSP)
```

---

## 2. PIPELINE ACTUAL DE JOSS

### Flujo de Ejecución: `joss archivo.joss` o `joss run archivo.joss`

```text
Source File (.joss)
       │
       ▼ [pkg/analyzer.LoadProject]
Lexer + Pratt Parser (pkg/parser) ──► AST (parser.Program)
       │
       ▼ [pkg/analyzer.Analyze / core.AnalyzeSourceUnits]
Semantic Analyzer (pkg/analyzer)
   ├── Declaration Collection (classes, methods, functions)
   ├── Nominal Contracts Validation (heritage, interfaces)
   ├── Type Inference & Definite Assignment Flow
   └── Sidecar Analysis Facts (PreparedProgram)
       │
       ▼ (Si hay errores semánticos JOSS-*, aborta)
Preload Application Files (app/**/*.joss)
       │
       ▼ [pkg/core.Runtime.Execute(program)]
Evaluador AST / Tree-Walk Runtime (pkg/core/evaluator.go)
   ├── Frame Runtime / Lexical Scopes (call_arguments.go, call_method.go)
   ├── Builtins Dispatcher (builtins.go)
   └── Native Class Dispatchers (native.go, GranDB, Server, etc.)
       │
       ▼
Resultado de Ejecución
```

### Flujo de Construcción Actual: `joss build native`

```text
joss build native [--target=os-arch]
       │
       ▼ [cmd/joss/native_builder.go: buildNative()]
1. Analizador de Alcanzabilidad (pkg/analyzer/reachability.go)
   ├── Entrypoint: main.joss
   ├── Calcula grafo de símbolos vivos (LiveFiles, LiveClasses, LiveMethods)
   └── Detecta RuntimeCapabilities requeridas (CapCore, CapHTTP, CapDatabase...)
       │
       ▼
2. Serialización y Compresión de Archivos (.joss ──► JOSSBC2Z)
   ├── Pruning de archivos muertos
   ├── Poda de métodos muertos del AST
   ├── Codificación AST via gob + flate: bytecode.Encode() ──► header "JOSSBC2Z"
   └── Cifrado AES-256 de assets y entorno en memoria (Virtual File System)
       │
       ▼
3. Compilación del Runner Go (compileRunnerBinary)
   ├── Ejecuta `go build -ldflags="-s -w" ./cmd/runner` con CGO_ENABLED=0
   └── Pasa tags condicionales (-tags=cli) si no se requiere HTTP/Server
       │
       ▼
4. Empaquetado Monolítico (assembleFinalExecutable)
   ├── Escribe binario compilado de runner
   ├── Agrega al final (append) el payload de assets cifrados
   ├── Agrega la clave simétrica de 32 bytes + longitud de 8 bytes
   └── Agrega el marcador final: "JOSS_RUNNER_DATA" (16 bytes)
       │
       ▼
5. Compresión UPX (opcional/automática si está disponible)
       │
       ▼
app.exe / app (27 MB - 35 MB)
```

**Diagnóstico Clave:** `joss build` **no genera código máquina nativo desde Joss**. Genera un stub precompilado en Go (`cmd/runner`) que contiene el runtime completo de Joss + intérprete tree-walk y le concatena al final un filesystem virtual cifrado con el AST serializado (`JOSSBC2Z`). Al ejecutarse, el runner lee su propio pie de archivo, descifra el VFS en memoria y ejecuta el AST con el evaluador Go.

---

## 3. AUDITORÍA DEL PARSER Y AST

* **Ubicación:** `pkg/parser/` (`lexer.go`, `parser.go`, `ast.go`, `ast_statements.go`, `ast_expressions.go`).
* **Tipo de Parser:** Pratt Parser (Top-Down Operator Precedence) con precedencias fijas en tabla.
* **Calidad del AST:** **Muy Alta**.
* **Nodos Representados:**
  * Declaraciones: `LetStatement` (`var`, `mixed`, tipos explícitos), `MultiLetStatement`, `ClassStatement`, `MethodStatement`, `InterfaceStatement`, `EnumStatement`, `RecordStatement` (inmutable), `InitStatement` (constructor con parameter promotion).
  * Control Flow: `IfStatement`, `WhileStatement`, `DoWhileStatement`, `ForeachStatement`, `TryCatchStatement`, `ThrowStatement`, `ReturnStatement`, `BreakStatement`, `ContinueStatement`.
  * Expresiones: `InfixExpression`, `PrefixExpression`, `CallExpression`, `MethodCallExpression`, `IndexExpression`, `MemberAccessExpression`, `TernaryExpression`, `NullCoalesceExpression`, `LambdaFunctionLiteral`, `MatchExpression`, `PipeExpression` (`|>`), `SpawnExpression` (`async`), `AwaitExpression`.
  * Tipos de Literales: Entero (`int64`), Flotante (`float64`), Decimal de precisión fija (`decimal.Decimal`), String, Booleano, Array literal, Map literal, Null literal.
* **Preparación para Backend Nativo:** El AST es completamente exhaustivo y tipado a nivel de representación sintáctica. Contiene la información estructural requerida para alimentar un generador de Código Intermedio (IR).

---

## 4. AUDITORÍA DEL SISTEMA DE TIPOS (TYPE SYSTEM)

* **Ubicación:** `pkg/typesystem/types.go`, `primitive_methods.go`.
* **Tipos Canónicos:**
  * Primitivos: `void`, `int` (64-bit int), `float` (float64 IEEE 754), `decimal` (precisión fija con decimal.Decimal), `string`, `bool`.
  * Especiales: `null`, `mixed` (dinámico voluntario), `unknown`.
  * Compuestos / Paramétricos: `array<T>`, `map<K, V>`, `channel<T>`, `Result<T, E>`.
  * Nominales: `class`, `record`, `interface`, `enum`.
  * Uniones: `T|null` (nullable / `T?`), `int|string`, etc.
  * Funciones de primera clase: `callable` con firma semántica `func(T1, T2): R`.
* **Inferencia y Chequeo:**
  * `Assignable(target, source)` implementa reglas de subtipado y uniones.
  * Inferencia de tipos en inicializaciones de variables (`var $x = 10` ➔ `int`).
  * Sin promoción implícita insegura (solo `int ➔ decimal`; `int ➔ float` requiere casteo explícito con advertencia `JOSS-ARITH-003`).
* **Estado frente a Compilación Nativa:**
  * El sistema de tipos es sólido y semánticamente estricto.
  * **Reto para Nativo:** `mixed` y tipos de unión no discriminados (`T1|T2`) requerirán representación como tagged unions / boxed values (`Value` struct con discriminator tag + payload de 64 bits/puntero), mientras que los tipos primitivos fijos (`int`, `float`, `bool`) pueden compilarse a tipos nativos sin overhead (i64, double, i1).

---

## 5. AUDITORÍA DEL ANALIZADOR SEMÁNTICO (SEMANTIC ANALYZER)

* **Ubicación:** `pkg/analyzer/` (`analyzer.go`, `declaration_collection.go`, `nominal_contracts.go`, `body_analysis.go`, `infer.go`, `prepared_program.go`).
* **Estructuras Existentes de Valor Incalculable:**
  * `PreparedProgram`: Empaqueta unidades de código fuente validadas junto con sus sidecar facts.
  * `AnalysisFacts`:
    * `InferredTypes: map[parser.Node]typesystem.Type`: Almacena el tipo estático deducido para cada nodo del AST sin mutarlo.
    * `ResolvedCalls: map[*parser.CallExpression]ResolvedCallFact`: Almacena el `TargetID`, `Kind` y `ReturnType` de cada invocación demostrable estáticamente.
    * `Symbols: map[string]SymbolFact`: Símbolos calificados globalmente.
    * `CallableTypes: map[string]typesystem.Type`: Firmas completas de funciones y closures.
* **Evaluación para Generador de Código:**
  * El analizador semántico ya resuelve en un 80% lo que un generador de código nativo necesita saber: tipos estáticos de expresiones y resolución de llamadas directas.
  * Esto permite que una fase de generación de IR nativo consulte directamente `PreparedProgram.Facts` para emitir instrucciones fuertemente tipadas en lugar de llamadas dinámicas genéricas.

---

## 6. AUDITORÍA DE ALCANZABILIDAD (REACHABILITY)

* **Ubicación:** `pkg/analyzer/reachability.go`.
* **Capacidades Actuales:**
  * `ReachabilityGraph` calcula:
    * `LiveFiles`: Archivos fuente realmente alcanzados desde el punto de entrada.
    * `LiveClasses`: Clases instanciadas o referenciadas.
    * `LiveMethods`: Métodos invocados (soporta poda de métodos muertos).
    * `LiveFunctions`: Funciones globales alcanzables.
    * `RuntimeCapabilities`: Deduce qué subsistemas del host son necesarios (`CapCore`, `CapHTTP`, `CapDatabase`, `CapSQLite`, `CapGUI`, etc.).
* **Limitaciones Frente a Compilación Nativa:**
  * El reachability actual se diseñó para podar archivos de un virtual file system (`MemFS`) y recortar métodos del AST antes de empaquetar en el runner Go.
  * El grafo actual no puede podar código nativo de Go porque Go ya linkea estáticamente todo el paquete `core` dentro de `cmd/runner`.
  * **Para Nativo Real:** Este mismo grafo de alcanzabilidad será el pilar del **Dead Code Elimination (DCE)** a nivel de funciones y librerías de runtime nativo.

---

## 7. AUDITORÍA DE BYTECODE Y MÁQUINAS VIRTUALES

### 7.1 El Bytecode Oficial Actual: `JOSSBC2Z`
* **Ubicación:** `pkg/bytecode/bytecode.go`.
* **Formato Real:** **No es un bytecode de instrucciones**, sino la serialización binaria del AST de Joss mediante `encoding/gob` comprimido con DEFLATE (`flate.BestCompression`) bajo el encabezado mágico `JOSSBC2Z`.
* **Conclusión:** `JOSSBC2Z` **no sirve como IR de compilador**. Es un formato de persistencia/caché de sintaxis abstracta para acelerar el arranque del intérprete.

### 7.2 El Prototipo Experimental de VM: `pkg/vm/`
* **Ubicación:** `pkg/vm/` (`opcodes.go`, `chunk.go`, `compiler.go`, `vm.go`, `value.go`).
* **Arquitectura:** Máquina virtual basada en pila (stack-based), con llamadas por marcos (`CallFrame`), `IP`, `BP` y array fijo de locales.
* **Opcodes Implementados:** Aritmética básica (`OpAddInt`, `OpSubInt`, `OpMulInt`), control de flujo simple (`OpJump`, `OpJumpIfFalse`), almacenamiento local (`OpLoadLocal`, `OpStoreLocal`), constantes (`OpConst`).
* **Estado:** Es un prototipo aislado para benchmarks y pruebas diferenciales de aritmética básica. **No implementa clases, objetos, strings complejos, uniones, closures ni llamadas nativas**.

### 7.3 El IR de Plugins: `pkg/plugincompiler/ir/`
* **Ubicación:** `pkg/plugincompiler/ir/ir.go`.
* **Arquitectura:** Estructura modular basada en bloques básicos (`IRBlock`), funciones (`IRFunction`), campos (`IRField`), estructuras (`IRStruct`) e instrucciones operacionales (`OpConst`, `OpLoad`, `OpStore`, `OpCallStatic`, `OpCallVirtual`, `OpBranch`, `OpBranchIf`, `OpNewObject`).
* **Relevancia:** Este paquete demuestra que la arquitectura de un IR basado en bloques de control de flujo (CFG) ya fue concebida en el ecosistema Joss para la compilación de plugins y puede ser la inspiración directa para el **Joss Native IR**.

---

## 8. AUDITORÍA DEL RUNTIME / EVALUADOR ACTUAL

* **Modelo:** Intérprete tree-walk evaluando directamente el AST en memoria Go (`pkg/core/evaluator.go`).
* **Estructuras de Datos:**
  * Objetos e instancias: `*core.Instance` (apunta a la clase y contiene un `map[string]interface{}` de campos).
  * Variables y valores: `interface{}` en Go.
  * Frames de llamada: `*core.Frame` que gestiona scopes léxicos, `$this`, slots de variables y límites de recursión (1024 frames máx).
* **Gestión de Memoria:** 100% dependiente del Garbage Collector de Go.
* **Despacho de Métodos:** Dinámico en runtime mediante inspección de mapas de métodos en la definición de clase (`r.Classes[className].Methods`).

---

## 9. AUDITORÍA DE CLASES NATIVAS Y DEPENDENCIAS VINCULADAS

En `pkg/core/native.go: RegisterNativeClasses()`, el runtime registra más de 40 clases nativas globales:

| Dominio | Clases Registradas | Drivers / Librerías Go Subyacentes |
| :--- | :--- | :--- |
| **Bases de Datos** | `GranDB`, `Model`, `Schema`, `Blueprint`, `SQLite` | `modernc.org/sqlite`, `github.com/go-sql-driver/mysql`, `github.com/jackc/pgx/v5`, `github.com/microsoft/go-mssqldb` |
| **Red y Servidor** | `Server`, `Router`, `Request`, `Response`, `WebResponse`, `WebSocket`, `Stream` | `net/http`, `github.com/gorilla/websocket` |
| **GUI Desktop** | `Window`, `WebView` (cmd/runner) | `github.com/jchv/go-webview2` (Windows WebView2) |
| **Caché y Mensajería**| `Redis`, `Cache`, `Queue`, `Stack` | `github.com/redis/go-redis/v9` |
| **Utilidades y I/O** | `JSON`, `Markdown`, `Str`, `UUID`, `Zip`, `SmtpClient`, `FileStream` | `gomarkdown/markdown`, `google/uuid`, `archive/zip`, `net/smtp` |
| **Sincronización** | `Mutex`, `RWMutex`, `WaitGroup` | `sync.Mutex`, `sync.RWMutex`, `sync.WaitGroup` (Go runtime) |

### Impacto en el Tamaño del Binario:
Debido a que `cmd/runner` importa `pkg/core` de manera monolítica, **todos los drivers anteriores (motores SQL completos en Go puro, parser de Markdown, WebSockets, WebView2, Redis, etc.) se compilan dentro del ejecutable final**, independientemente de si el script del usuario es un simple `echo "Hola mundo";`.

---

## 10. AUDITORÍA DEL BUILD SYSTEM Y MEDIDAS DEL EJECUTABLE

### 10.1 Prueba Experimental con Binario Mínimo

Se compiló y analizó el binario base `runner` con diferentes configuraciones de stripping y tags:

```text
Configuración                           Tamaño del Binario
─────────────────────────────────────────────────────────────
cmd/runner (Completo, sin strip)         37.60 MB
cmd/runner (-ldflags="-s -w")            27.19 MB (28,513,280 bytes)
cmd/runner (-tags=cli, stripped)         25.86 MB (27,117,056 bytes)
cmd/runner + UPX (Compresión LZMA)        7.80 MB - 9.50 MB
```

### 10.2 Análisis de Símbolos (`go tool nm`):
Un análisis de símbolos en el ejecutable demostró que contiene más de **11,600 símbolos** correspondientes exclusivamente a dependencias de terceros embebidas:
* `modernc.org/sqlite`: Emulador C-to-Go del motor completo de SQLite (~7 MB).
* `github.com/microsoft/go-mssqldb`: Driver TDS de SQL Server.
* `github.com/jackc/pgx/v5`: Driver PostgreSQL.
* `github.com/gorilla/websocket`: Stack completo de WebSocket.
* `gomarkdown/markdown`: Motor de renderizado Markdown.
* `github.com/jchv/go-webview2`: Wrappers de COM y Win32 para WebView2.

---

## 11. MAPA DE DEPENDENCIAS

```text
JOSS SCRIPT
    │
    ▼
pkg/core (Runtime Monolítico)
    ├── modernc.org/sqlite (SQLite en Go puro)
    ├── github.com/jackc/pgx/v5 (PostgreSQL)
    ├── github.com/go-sql-driver/mysql (MySQL)
    ├── github.com/microsoft/go-mssqldb (SQL Server)
    ├── github.com/redis/go-redis/v9 (Redis Client)
    ├── github.com/gorilla/websocket (RFC 6455 WebSockets)
    ├── github.com/gomarkdown/markdown (CommonMark Parser)
    ├── github.com/jchv/go-webview2 (MSHTML/Edge WebView2)
    ├── github.com/ebitengine/purego (C Callbacks / FFI)
    ├── golang.org/x/crypto (Bcrypt, Argon2, PBKDF2)
    └── golang.org/x/term (Terminal Raw Mode)
```

* **CGO:** Afortunadamente, **CGO_ENABLED=0** en todos los builds actuales. No hay dependencias dinámicas de librerías C externas (libsqlite3, etc.), lo cual facilita enormemente la portabilidad cruzada hoy en día.
* **Problema para Nativo:** El acoplamiento monolítico impide generar ejecutables ligeros (< 1 MB).

---

## 12. PLATAFORMAS SOPORTADAS ACTUALMENTE

Configuradas formalmente en `cmd/joss/native_builder.go`:

| Sistema Operativo | Arquitecturas Soportadas | Estado | Método de Build |
| :--- | :--- | :--- | :--- |
| **Windows** | `amd64`, `arm64`, `386` | Activo / Estable | Cross-compiling de Go (`GOOS=windows`) |
| **Linux** | `amd64`, `arm64`, `arm`, `386`, `riscv64` | Activo / Estable | Cross-compiling de Go (`GOOS=linux`) |
| **macOS (Darwin)** | `amd64`, `arm64` | Activo | Cross-compiling de Go (`GOOS=darwin`) |
| **Android** | `arm64`, `arm`, `amd64`, `386` | Activo (vía package `mobile`) | Cross-compiling de Go (`GOOS=android`) |

---

## 13. RESULTADO DE LA SUITE DE PRUEBAS DEL PROYECTO

Ejecutada con `go test -short ./pkg/... ./cmd/...`:

```text
PAQUETE                                       RESULTADO      DURACIÓN
───────────────────────────────────────────────────────────────────────
github.com/jossecurity/joss/pkg/analyzer      PASS           0.98s
github.com/jossecurity/joss/pkg/buildcache    PASS           2.73s
github.com/jossecurity/joss/pkg/buildmanifest PASS           4.28s
github.com/jossecurity/joss/pkg/bytecode      PASS           4.05s
github.com/jossecurity/joss/pkg/core          PASS          13.99s
github.com/jossecurity/joss/pkg/fixer         PASS           2.73s
github.com/jossecurity/joss/pkg/formatter     PASS           5.26s
github.com/jossecurity/joss/pkg/i18n          PASS           9.11s
github.com/jossecurity/joss/pkg/linter        PASS           7.89s
github.com/jossecurity/joss/pkg/mobile        PASS           7.18s
github.com/jossecurity/joss/pkg/parser        PASS           5.62s
github.com/jossecurity/joss/pkg/plugincompiler PASS          9.06s
github.com/jossecurity/joss/pkg/pluginpkg     PASS           4.93s
github.com/jossecurity/joss/pkg/pluginruntime PASS           8.07s
github.com/jossecurity/joss/pkg/runtime/frame PASS           0.82s
github.com/jossecurity/joss/pkg/runtime/plan  PASS           3.51s
github.com/jossecurity/joss/pkg/runtime/value PASS           4.07s
github.com/jossecurity/joss/pkg/server        PASS           6.23s
github.com/jossecurity/joss/pkg/template      PASS           5.81s
github.com/jossecurity/joss/pkg/tester        PASS           5.45s
github.com/jossecurity/joss/pkg/typesystem    PASS           0.99s
github.com/jossecurity/joss/pkg/viewtemplate  PASS           0.92s
github.com/jossecurity/joss/pkg/vm            PASS           4.82s
github.com/jossecurity/joss/cmd/joss          PASS           3.18s
github.com/jossecurity/joss/cmd/runner        PASS           1.28s

TOTAL: 25 paquetes probados, 0 fallos, 0 errores.
PASS: 100% de la suite ejecutada.
```

---

## 14. MATRIZ DE CARACTERÍSTICAS DEL LENGUAJE

| Característica | Sintaxis Existe | Testeado | En Bytecode Real | En Runtime Actual | Native Readiness |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Variables (`var`, `int`, etc.)** | Sí | Sí | No (AST Gob) | Sí (Tree-walk) | **ALTA** (Tipadas y analizadas) |
| **Aritmética estricta / Overflow** | Sí | Sí | Sí (en pkg/vm) | Sí (Tree-walk) | **ALTA** (Reglas canónicas en Go) |
| **Funciones y Closures** | Sí | Sí | No (AST Gob) | Sí (Lexical Frame) | **MEDIA** (Captura de scope requiere layout) |
| **Clases, Métodos, Herencia** | Sí | Sí | No (AST Gob) | Sí (Map dispatch) | **MEDIA** (Requiere vtables estáticas) |
| **Records Inmutables** | Sí | Sí | No (AST Gob) | Sí (Object fields) | **ALTA** (Layout C-struct directo) |
| **Channels y Concurrencia** | Sí | Sí | No (AST Gob) | Sí (Go channels) | **BAJA** (Depende fuertemente de Go runtime) |
| **Strings (UTF-8)** | Sí | Sí | No (AST Gob) | Sí (Go strings) | **MEDIA** (Requiere runtime string struct) |
| **Arrays y Maps** | Sí | Sí | No (AST Gob) | Sí (Go slices/maps) | **BAJA** (Requiere runtime hash table / vec) |
| **GranDB / ORM** | Sí | Sí | No (AST Gob) | Sí (Librerías Go) | **MUY BAJA** (Enorme dependencia externa) |
| **HTTP / Server** | Sí | Sí | No (AST Gob) | Sí (net/http Go) | **MUY BAJA** (Enorme dependencia externa) |
| **GUI (WebView2)** | Sí | Sí | No (AST Gob) | Sí (purego/DLLs) | **BAJA** (Requiere FFI o C-bindings) |
| **Primitivas de Sincronización**| Sí | Sí | No (AST Gob) | Sí (sync Go) | **ALTA** (Se mapean a pthreads/Win32) |

---

## 15. EVALUACIÓN DETALLADA PARA COMPILACIÓN NATIVA

### A. Parser: PREPARADO (95/100)
El parser y AST de Joss están completamente maduros, estables y bien testeados. No requieren cambios estructurales para compilación nativa.

### B. Type System: PREPARADO (85/100)
El sistema canónico en `pkg/typesystem` tiene tipos bien definidos, reglas estrictas de asignabilidad y narrowing. Falta formalizar el modelo de tagged unions (discriminator en bajo nivel).

### C. Semantic Analysis: PREPARADO (90/100)
La infraestructura `PreparedProgram` y `AnalysisFacts` (`InferredTypes`, `ResolvedCalls`) provee exactamente la metadata necesaria para bajar a un IR lineal.

### D. AST: PREPARADO (90/100)
El AST es de muy alto nivel para emitir código máquina directo, pero es ideal para alimentar un traductor AST ➔ IR.

### E. Bytecode Actual: NO PREPARADO (0/100 como IR)
`JOSSBC2Z` es un wrapper comprimido del AST, no un IR. La VM experimental (`pkg/vm`) es sólo un prototipo incompleto.

### F. Runtime: NO PREPARADO (15/100 para compilación nativa directa)
El runtime actual está 100% acoplado al runtime de Go (`runtime.GC`, `reflect`, `sync`, `net/http`, `map`, `slices`).

### G. Modelo de Memoria: NO EXISTE
Actualmente Joss delega la asignación de memoria (`new`, arreglos, instancias) al heap de Go. No hay layout de memoria nativo especificado (alineación de structs, tamaño de punteros, vtable offsets).

### H. Garbage Collection: NO EXISTE
Joss no tiene su propio GC. Depende del GC de Go. Un compilador nativo real requerirá:
* Integrar un GC conservador / conciso como **Boehm GC** (`libgc`) o **immix**, o
* Implementar conteo de referencias automático (ARC) similar a Swift.

### I. Despacho de Métodos: DINÁMICO
El runtime actual busca métodos por nombre en tablas hash en cada llamada. Para código nativo se deben compilar **vtables** (Virtual Method Tables) estáticas indexadas por offset entero para llamadas polimórficas, o llamadas directas en métodos sellados/estáticos.

### J. Build System: REQUIERE NUEVA CAPA
Actualmente `joss build` invoca al compilador de Go. Para binarios nativos reales, `joss build` deberá generar código intermedio y orquestar un backend nativo (ej. LLVM o emisor C) y el linker del sistema.

---

## 16. ANÁLISIS DE OPCIONES PARA EL BACKEND NATIVO

### Opción 1: Joss AST ➔ LLVM IR Directo
* **Dificultad:** Alta.
* **Problema:** El AST de Joss es demasiado rico y complejo (expresiones pipe, pattern matching, uniones, strings interpolados). Traducir eso directamente a SSA LLVM IR sin un paso intermedio de desazucarado resulta en un compilador monolítico, frágil e incomprensible.

### Opción 2: Joss AST ➔ Joss IR (SSA/CFG) ➔ LLVM IR ➔ Machine Code (RECOMENDADA A LARGO PLAZO)
* **Dificultad:** Alta.
* **Ventajas:** Arquitectura limpia estándar de la industria (similar a Rust MIR ➔ LLVM, Swift SIL ➔ LLVM). Joss IR realiza comprobaciones semánticas, desazucara construcciones complejas y optimiza antes de generar LLVM IR.
* **Desventajas:** Dependencia de la librería de LLVM (tamaño considerable del toolchain del compilador).

### Opción 3: Joss AST ➔ Joss IR ➔ C Transpilation ➔ Clang/GCC (RECOMENDADA PARA BOOTSTRAPPING)
* **Dificultad:** Media.
* **Ventajas:** Portabilidad inmediata a todas las plataformas, integración sencilla con librerías nativas (Boehm GC, pthreads, libc), no requiere instalar LLVM de decenas de gigabytes.
* **Desventajas:** Dependencia de un compilador C en la máquina anfitriona o distribución de un toolchain C embebido (como Zig).

---

## 17. DEFINICIÓN DEL "BINARIO NATIVO" PARA JOSS

Se analizaron las cuatro opciones planteadas:
* **Opción A (Bytecode + VM):** No cumple el requerimiento del usuario (no es código máquina nativo real).
* **Opción B (Native Code + Runtime Go Monolítico):** Es lo que existe hoy (~27 MB a 37 MB). No es un compilador nativo real.
* **Opción C (Native Code + Minimal Native Runtime):** **LA ELECCIÓN CORRECTA**. El código Joss compila a código máquina real, enlazado contra un runtime mínimo escrito en C o Zig (< 500 KB) que provee manejo de strings UTF-8, asignación de memoria (Boehm GC) y I/O básico de consola/archivos.
* **Opción D (Native Code Autocontenido sin Runtime):** Inviable para un lenguaje dinámico/de alto nivel con soporte para excepciones, strings dinámicos y uniones. Incluso C requiere `libc`, y Rust requiere `libstd` o un runtime de unwinding.

**Veredicto:** El objetivo formal de Joss debe ser la **Opción C: Native Code + Minimal Native Runtime**.

---

## 18. EL CAMINO DE `print("Hello World")` A `hello.exe` REAL

Para que un archivo `hello.joss`:
```joss
echo "Hello World\n";
```
se convierta en un binario `hello.exe` de **< 200 KB** (o < 50 KB con strip) sin Go, sin runner y sin VFS cifrado:

1. **Joss Frontend (Go):**
   * Parsear `hello.joss` ➔ `parser.Program`.
   * Analizar con `analyzer` ➔ `PreparedProgram`.
2. **Generador de Joss IR:**
   * Transformar AST en bloques básicos lineales:
     ```text
     block entry:
       %str0 = const_string "Hello World\n"
       call_extern @joss_print(%str0)
       ret void
     ```
3. **Backend Nativo (LLVM o C):**
   * Emitir código objeto nativo (`.obj` o `.o`) con llamada a la función de runtime C `joss_print`.
4. **Minimal Runtime (`libjoss_rt.a`):**
   * Implementado en C/C++ (~10 KB compilado): inicializa la consola, maneja buffers e invoca la syscall nativa (`WriteFile` en Windows, `write` en Linux).
5. **Linker Nativo (lld / link.exe / ld):**
   * Enlaza el código objeto con `libjoss_rt.a` y las librerías del sistema operativo (`kernel32.lib` o `libc.so`).

---

## 19. ROADMAP TÉCNICO PROPUESTO HACIA EL COMPILADOR NATIVO

### FASE 0: Baseline y Congelación Arquitectónica (ESTADO ACTUAL)
* Auditoría completa documentada.
* Suite de pruebas del compilador actual en estado verde (100% PASS).
* Ninguna modificación en la semántica existente.

### FASE 1: Diseño e Implementación de Joss IR (Linear / CFG IR)
* Crear `pkg/ir/` en Go.
* Definir instrucciones para: constantes, cargas/almacenamientos, operaciones aritméticas, llamadas y saltos condicionales.
* Implementar lowered AST ➔ Joss IR utilizando `PreparedProgram.Facts`.

### FASE 2: Especificación del Minimal C Runtime (`joss-rt`)
* Crear `runtime/native/` en C o C++ minimalista:
  * Modelo de valor nativo (uniones etiquetadas para `mixed`/uniones).
  * Estructura de Strings inmutables UTF-8 (`length`, `capacity`, `bytes`).
  * Asignador de memoria integrado con Boehm GC (`GC_malloc`).
  * Primitivas de consola (`print`, `echo`, `readline`).

### FASE 3: Emisión de Código Nativo (Fase Bootstrap con C o LLVM C-API)
* Emitir representaciones ejecutables desde Joss IR.
* Compilar funciones independientes con tipos primitivos (`int`, `float`, `bool`).
* Pruebas de ejecución: operaciones matemáticas, recursión (Fibonacci) y control de flujo compilados a código máquina real.

### FASE 4: Cadenas de Texto y Manejo de Memoria
* Operaciones sobre strings (concatenación, interpolación, slicing).
* Manejo automático de memoria sin intervención manual del usuario.

### FASE 5: Tipos Compuestos (Arrays y Diccionarios)
* Implementar `Array<T>` dinámico nativo y `Map<K, V>` (Hash Table).
* Iteradores `foreach` nativos.

### FASE 6: Modelo de Objetos y Clases
* Layout estático de memoria para campos de instancias.
* Compilación de constructores `Init`.
* Generación de VTables para despacho polimórfico de métodos.

### FASE 7: Excepciones y Unwinding
* Implementación de `try`, `catch`, `throw` usando setjmp/longjmp o DWARF unwinding.

### FASE 8: Estrategia de Modularización de Clases Nativas
* **Separar las clases masivas de Go:** `GranDB`, `Server`, `WebSocket` deben extraerse a módulos/librerías dinámicas opcionales (`.so` / `.dll` o plugins compilados), de modo que los binarios CLI estándar no incluyan dependencias no usadas.

---

## 20. RIESGOS CRÍTICOS (CRITICAL RISKS)

1. **[CRITICAL] Acoplamiento de Clases Nativas con Go Runtime:**
   * La biblioteca estándar (`GranDB`, `Model`, `Server`) depende íntimamente de la concurrencia (`goroutines`, `net/http`) y paquetes de terceros de Go. Reescribir todo esto en C/Nativo es un trabajo titánico. Debe abordarse aislando el compilador nativo para CLI/Lógica de negocio primero y modulando los servidores.
2. **[CRITICAL] Manejo de Memoria y Garbage Collection:**
   * Joss no tiene gestión de memoria manual (`malloc`/`free` no existen en la sintaxis). Sin un GC integrado (como Boehm GC), los programas nativos fugarán memoria masivamente.
3. **[HIGH] Características Dinámicas (`mixed` y reflection):**
   * Variables de tipo `mixed` requieren boxed values en runtime y dispatch dinámico en tiempo de ejecución, lo que añade complejidad al backend de código nativo.
4. **[HIGH] Complejidad del Toolchain de Compilación:**
   * Exigir que los usuarios de Joss instalen LLVM o Clang/MSVC romperá la experiencia "out-of-the-box" actual (`joss build` que funciona sin dependencias gracias a Go). Se debe evaluar distribuir un backend linker/emisor autónomo o integrado.

---

## 21. QUÉ NO DEBEMOS HACER TODAVÍA (DO NOT DO YET)

1. **NO modificar `pkg/parser`:** El parser está listo y cualquier cambio innecesario arriesga romper la suite de compatibilidad.
2. **NO modificar `pkg/typesystem` ni `pkg/analyzer`:** Son el activo más valioso del proyecto; su estado actual debe mantenerse como la fuente de verdad canónica.
3. **NO intentar reescribir `cmd/runner` todavía:** El runner actual debe seguir existiendo para dar soporte a la suite web JosSecurity mientras el compilador nativo se desarrolla en paralelo.
4. **NO implementar LLVM directamente en el AST:** Saltarse la capa de Joss IR resultará en deuda técnica inmediata e insostenible.

---

## 22. RESUMEN EJECUTIVO DEL ESTADO ACTUAL

```text
JOSS CURRENT STATE
------------------

Compiler:
Actualmente Joss opera como un intérprete AST tree-walk en Go, asistido por un análisis estático estricto (analizador semántico con inferencia, chequeo de tipos y facts sidecar).

Parser:
Pratt Parser completo, robusto y validado. Cubre clases, uniones, records inmutables, pattern matching, expresiones pipe y constructores modernos.

Type system:
Canónico, estricto y desacoplado (`pkg/typesystem`). Sin dependencias circulares. Valida asignabilidad, narrowing y tipos compuestos.

Analyzer:
Altamente avanzado (`pkg/analyzer`). Emite diagnósticos formales estables, facts de llamadas resueltas y tipos inferidos indexados por nodo.

IR:
Existe un IR para plugins (`pkg/plugincompiler/ir`), pero NO existe un IR general de propósito completo para el lenguaje Joss principal.

Bytecode:
El formato `JOSSBC2Z` es AST comprimido vía Gob+DEFLATE, no código de máquina ni bytecode real. La VM en `pkg/vm` es un prototipo experimental limitado a aritmética básica.

Runtime:
Motor en Go (`pkg/core`) que evalúa AST. Fuertemente acoplado a dependencias ricas de Go (drivers SQL, HTTP, WebSockets, WebView2).

Build:
El comando `joss build native` ensambla un stub en Go (`cmd/runner`) + VFS cifrado con el AST serializado. Produce binarios de 27 MB a 37 MB.

Native compilation readiness:
48 / 100

Biggest blockers:
1. Ausencia de un Joss IR (Linear / Control Flow Graph) con generador desde el AST.
2. Ausencia de un runtime nativo minimalista propio en C/Nativo (con GC o allocador nativo, strings y manejo de excepciones).
3. Acoplamiento monolítico del runtime actual con drivers pesados de Go (SQLite, MySQL, Postgres, MSSQL, WebView2, gorilla/websocket).

Best next step:
Diseñar formalmente la especificación de Joss IR (Linear / SSA) en Go que consuma `PreparedProgram` y demuestre la compilación de un subconjunto esencial (enteros, control de flujo y funciones) sin tocar el pipeline existente.
```

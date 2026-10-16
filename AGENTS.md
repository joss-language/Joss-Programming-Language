# Guía de arquitectura y reglas operativas para agentes y desarrolladores

Este archivo es una guía operativa y de gobernanza técnica del repositorio, no un changelog. Antes de modificar semántica, código o documentación, lea `docs/ARQUITECTURA.md`, `docs/SISTEMA_TIPOS.md`, `docs/DIAGNOSTICOS.md`, `docs/DOCUMENTATION_AUDIT.md` y los tests del subsistema afectado.

---

## 1. Principio fundamental de desarrollo

> **El código fuente y sus tests ejecutados son la única fuente de verdad.**

No asuma que un comentario, un archivo histórico o una tesis describe el comportamiento real del sistema:
- Verifique siempre la implementación concreta en `pkg/parser`, `pkg/typesystem`, `pkg/analyzer` y `pkg/core`.
- Cualquier cambio en la semántica del lenguaje exige pruebas unitarias, de análisis semántico y de ejecución runtime.

---

## 2. El Pipeline Real de Joss: Un solo lenguaje, Múltiples Modos

```text
                                  JOSS SOURCE (.joss)
                                           │
                                           ▼
                                 lexer → parser Pratt
                                           │
                                           ▼
                                          AST
                                           │
                                           ▼
                             Semantic Analyzer Pipeline
                     (collectDeclarations → projectScope →
                      validateNominalContracts → analyzeSourceBodies)
                                           │
                                           ▼
                              PreparedProgram (Canónico)
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    ▼                      ▼                      ▼
               `joss run`             `joss server`          `joss build`
               Intérprete            Servidor HTTP         Native Compiler
              (`pkg/core`)           (`pkg/server`)           (`pkg/ir` →
                   │                      │              `pkg/backend/native`)
                   ▼                      ▼                       ▼
             Salida Consola        Respuesta HTTP        Binario Standalone (.exe/ELF)
```

- `pkg/parser`: Tokens, lexer, parser Pratt con tabla de precedencias canónicas y AST.
- `pkg/typesystem`: Tipos canónicos, compatibilidad de asignación (`Assignable`), inferencia (`MergeInference`) y coerción explícita (`CoerceString`).
- `pkg/analyzer`: Unidades fuente (`SourceUnit`), scopes léxicos, tablas de símbolos, firmas, comprobación de tipos y generación inmutable de `PreparedProgram`. **Nunca** debe importar `pkg/core`.
- `pkg/diagnostics`: Modelo de errores y advertencias (`Diagnostic`) con código estable, severidad, rango, explicación y sugerencia.
- `pkg/core`: Evaluador de AST, runtime Go, frames léxicos, slots, built-ins globales y clases nativas integradas; consume directamente `PreparedProgram`.
- `pkg/ir`: Representación intermedia canónica (*Joss Native IR*), lowering de `PreparedProgram`, instrucciones SSA/3AC y verificador de tipos de bajo nivel (`ir.Verifier`).
- `pkg/backend/native`: Generadores de código nativo para compilación AOT standalone y emisión LLVM; genera ejecutables independientes de máquina real sin AST serializado ni runtime Go embebido.
- `pkg/server`: Runtime HTTP multinivel, multiplexor, WebSockets y controladores MVC basados en el mismo modelo semántico.
- `pkg/bytecode`: Serialización comprimida del AST (`JOSSBC2Z`) para plugins `.jp`, no para ejecutables nativos AOT.
- `pkg/pluginruntime`, `pkg/pluginpkg`, `pkg/plugincompiler`: Runtime, paquetes y JPBC de plugins.
- `cmd/joss`: CLI y orquestación unificada del proyecto (`run`, `server`, `program`, `build`, `analyze`, `emit-ir`, `repl`).
- `vscode-joss`: Servidor de lenguaje (LSP) y extensión de editor.

La dirección estricta de dependencias es:
`parser / typesystem / diagnostics → analyzer → ir → backend/native`
`analyzer → core / server`. `pkg/analyzer` **nunca** debe importar `pkg/core`.

---

## 3. Fuentes de verdad canónicas (Nunca duplicar)

1. **Keywords y símbolos léxicos**: Definidos exclusivamente en `pkg/parser/token.go`; tooling consume `parser.KeywordNames()` y `parser.SymbolDefinitions()`. Lexer y formatter no deben recrear listas de operadores o delimitadores.
2. **Tipos y compatibilidad**: Residen exclusivamente en `pkg/typesystem`, incluidas clasificaciones como `Type.IsNumeric()` y firmas de métodos primitivos en `primitive_methods.go`. Analyzer y runtime consumen `PrimitiveMethod`; no reintroducir aliases ni listas primitivas locales.
3. **Built-ins globales**: Nombre, dominio de dispatcher y retorno se declaran juntos en `pkg/core/builtins.go`. Toda entrada debe tener un handler alcanzable; `TestBuiltinCatalogHasUniqueDefinitionsAndReachableHandlers` protege esta regla.
4. **Clases y métodos nativos**: Registrados en `Runtime.RegisterNativeClasses()` y tipados en `pkg/core/native_signatures.go`. Usar `GetNativeClassMethods()` para inspección.
5. **Plugins**: Índice `pluginpkg.SymbolIndex` del paquete `.jp`.
6. **Diagnósticos**: Códigos estables `JOSS-...` emitidos como `diagnostics.Diagnostic`.
7. **Catálogo de VS Code**: `vscode-joss/src/server/generated/languageCatalog.json`, generado automáticamente por `go run ./tools/cataloggen`. **Nunca editarlo manualmente**.
8. **Catálogo nativo de documentación**: `docs/CATALOGO_NATIVO.md`, generado automáticamente por `go run ./tools/docgen`. **Nunca editarlo manualmente**.

Al modificar invocación u operadores:
- binding y referencias pertenecen a `pkg/core/call_arguments.go`; ciclo del frame y contratos a `call_method.go`; tipos invocables a `callable_dispatch.go`;
- `evaluator_infix.go` sólo coordina orden/short-circuit; aritmética, control, pipeline, streams e incremento pertenecen a sus módulos de dominio;
- mantenga una sola implementación de pipeline y una sola ruta evaluada de llamadas. Los entry points de compatibilidad deben delegar.

---

## 4. Reglas semánticas de variables y tipos

- `var $x = 1`: Declaración canónica por inferencia estricta y fija.
- `$x = 1`: La asignación directa sin declaración previa está en deprecación; emite `JOSS-DECL-007` y en modo estricto o fuera de contextos históricos es rechazada como variable no declarada (`JOSS-SYM-001`). Toda variable debe declararse previamente con `var`, tipo explícito, `mixed` o `const`.
- `int $x = 1`: Tipo explícito. `let int $x = 1` y `let $x = 1` son sintaxis histórica en transición (`JOSS-DECL-006`); use `mixed $x = 1` para dinamismo explícito voluntario.
- `mixed $x = 1`: Dinamismo explícito voluntario; permite cambiar de tipo en reasignaciones. No existe un modo dinámico global en `joss.yaml`.
- **Todo parámetro fuente debe declarar un tipo explícito**. Usa `mixed $x` si es dinámico; `$x` sin tipo ya no es válido (`JOSS-TYPE-011`).
- Los antiguos aliases `integer`, `double`, `boolean`, `dynamic`, `any` y `list` fueron retirados. Usarlos emite `JOSS-TYPE-009`. No deben reintroducirse accidentalmente.
- Una inicialización con `nil`/`null` pospone la inferencia hasta la asignación de un valor concreto.
- `T|null` declara una unión nullable; `T?` es solo un atajo sintáctico normalizado a `T|null`.
- `const $x = ...` infiere un tipo fijo inmutable; `const int $x = ...` lo declara explícitamente. También se protegen propiedades constantes.
- `public func name(...): Type` declara el tipo de retorno. Analyzer y runtime validan cada retorno explícito (`JOSS-TYPE-008`), y el analyzer exige retorno o throw en todas las rutas demostrables (`JOSS-TYPE-010`).
- Cada llamada a función o método usa un marco (*frame*) aislado. Las funciones con nombre solo ven sus parámetros, locales, `$this` y bindings del host; no ven variables de nivel superior del archivo ni del caller. Las closures sí capturan su entorno léxico. La recursión está limitada a 1024 llamadas por defecto (`Runtime.MaxCallDepth`).
- `ref T $x` y `call(ref $valor)` crean una referencia mutable temporal, estrictamente invariante y no escapable. Solo acepta variables no constantes; no admite defaults, campos, índices, almacenamiento en variables, retorno ni paso a llamadas nativas o `async`.
- Clases y funciones globales exigen `public` o `private`; métodos y propiedades exigen `public`, `protected` o `private`. `static` nunca añade visibilidad implícita. `Init` y closures no llevan modificador.

### Semántica estabilizada en FASE 1

- La precedencia canónica vive en `pkg/parser/parser.go`: `*`, `/` y `%`
  comparten nivel; `&&` tiene mayor precedencia que `||`; suma tiene mayor
  precedencia que shifts. Toda modificación exige tests de la tabla completa.
- `??` sólo maneja `null`. Nunca debe recuperar panics, excepciones, errores de
  tipo, índice o aritmética.
- Truthiness falsa: `null`, `false`, cero de cualquier tipo numérico, string
  vacío, array vacío y map vacío. `"0"` es verdadero.
- La única promoción numérica implícita entre tipos distintos es
  `int → decimal`. `int → float`, `float → decimal` y sus operaciones mixtas
  requieren conversión explícita. Analyzer y runtime usan `JOSS-ARITH-003`
  como defensa contra pérdida de precisión.
- `sum` conserva `decimal`, comprueba overflow entero y rechaza mezclas
  `float`/`decimal` o enteros no representables exactamente al promover a
  `float`.

### Contratos y migración de FASE 2

- `void` es un tipo canónico: sólo acepta una terminación sin valor. Las
  callables nombradas deben migrar a un retorno explícito; `JOSS-TYPE-014` es
  advertencia de transición y `: void` expresa intención deliberada.
- Las closures tienen un tipo callable inferido. Cuando sus parámetros y
  retorno se conocen, el analyzer debe conservarlos; no los degrade a
  `unknown`.
- `let`, `nil`, `Init constructor` y declaraciones implícitas se mantienen
  sólo durante la transición. Los diagnósticos `JOSS-DECL-006` a
  `JOSS-DECL-008` explican la forma moderna. `joss fix` sólo automatiza
  transformaciones léxicamente seguras y nunca reescribe strings o comentarios.
- Un `Future` es dueño de un contexto hijo cancelable. `Fork` propaga el
  contexto padre y `Free` nunca conserva una tarea o contexto de ejecución.

---

## 5. Reglas obligatorias para nuevas funcionalidades

1. **Definir la semántica antes de programar**: Determine invariantes y posibles casos de borde.
2. **Cambios sintácticos**:
   - Agregar tokens en `pkg/parser/token.go`.
   - Modificar lexer, parser Pratt (`parser.go`, `parser_expressions.go`, `parser_statements.go`) y AST.
   - Agregar pruebas unitarias positivas y negativas en `pkg/parser/`.
   - Actualizar la gramática formal en `docs/GRAMATICA.md` y la referencia en `docs/SINTAXIS.md`.
3. **Cambios en el sistema de tipos**:
   - Incorporar el `Kind` y nombre canónico en `pkg/typesystem`.
   - Implementar las reglas en `Assignable`, `MergeInference` y `CoerceString` con tests exhaustivos.
   - Enseñar al analizador (`pkg/analyzer/infer.go`) a inferirlo.
   - Enseñar al runtime a reconocerlo y validarlo.
   - Regenerar catálogos con `go run ./tools/cataloggen`.
4. **Nuevos diagnósticos**:
   - Usar un código estable dentro de la familia `JOSS-...`.
   - Emitir `diagnostics.Diagnostic`, nunca strings libres ni ad-hoc.
   - Incluir severidad, archivo, rango, explicación y sugerencia útil.
   - Añadir un caso inválido y su vecino válido en `docs/DIAGNOSTICOS.md`.
5. **Ejemplos verificables en la documentación**:
   - Todo ejemplo completo nuevo en `docs/*.md` o `README.md` debe usar un marcador de contrato:
     - `<!-- joss-run: ["salida esperada"] -->` para ejemplos ejecutables.
     - `<!-- joss-check: descripción -->` para fragmentos que requieren servidor, base de datos o contexto externo.
     - `<!-- joss-error: JOSS-CODIGO -->` para verificar la emisión del diagnóstico.
   - Estos marcadores son validados automáticamente por `pkg/core.TestDocumentationContracts`.
6. **Tests obligatorios de compilación nativa y paridad diferencial**:
   - **Toda nueva funcionalidad que se integre al lenguaje debe incluir obligatoriamente pruebas de compilación nativa.**
   - No es suficiente con implementar y probar el intérprete: se debe implementar el lowering en `pkg/ir/lower.go`, la verificación en `pkg/ir/verifier.go` y la emisión en `pkg/backend/native`.
   - Se debe crear o ampliar un test diferencial en `tests/native/differential_test.go` que compile el programa a un ejecutable nativo real (`joss build`), lo ejecute y verifique que la salida (stdout, stderr y exit code) coincide byte a byte con la ejecución interpretada (`joss run`).
   - Ninguna característica del lenguaje se considera completa sin su correspondiente test de compilación nativa exitoso.
7. **Actualización obligatoria de documentación e internacionalización**:
   - **Siempre que se agregue o modifique una funcionalidad, sintaxis, API o comportamiento del lenguaje, es obligatorio actualizar la documentación correspondiente en `docs/`.**
   - No se aceptan cambios de código que dejen la documentación obsoleta, incompleta o desfasada.
   - Si se introducen nuevos conceptos o archivos en `docs/`, deben actualizarse sus traducciones en `docs/en/` y `docs/pt/` mediante `go run ./tools/docsi18n --accept-current` y sincronizarse con el mirror público mediante `go run ./tools/docsi18n --sync`.
   - El comando `go run ./tools/docsi18n --check` debe pasar con código 0 antes de dar por finalizada la tarea.

---

## 6. Sincronización de documentación y JosSecurity

`docs/*.md` es la fuente canónica de documentación del proyecto.

La copia pública que sirve la aplicación web JosSecurity vive en:
`ejemplos/Joss-Red-JosSecurity/assets/docs/`

**Debe coincidir archivo por archivo y byte por byte con `docs/*.md`**:
- El menú de navegación en `ejemplos/Joss-Red-JosSecurity/app/views/docs/menu.joss.html` debe tener exactamente una entrada `data-page="NOMBRE"` para cada archivo `.md`.
- El controlador en `ejemplos/Joss-Red-JosSecurity/app/controllers/web/DocsController.joss` debe tener exactamente una entrada en el mapa `$titles` para cada archivo.
- Todo archivo debe estar enlazado en `docs/README.md`.
- Ningún enlace relativo Markdown puede estar roto.
- La prueba `TestDocumentationNavigationAndPublicMirror` en `pkg/core/documentation_test.go` valida automáticamente esta paridad.

---

## 7. Comandos de validación obligatorios

Antes de dar por concluida cualquier modificación:

```bash
# Formateo de código Go modificado
gofmt -w <archivos-go-modificados>

# Verificación de generadores automáticos
go run ./tools/cataloggen --check
go run ./tools/docgen --check

# Análisis estático y pruebas en Go
go vet ./...
go test ./...
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
go build ./...

# Pruebas de documentación y contratos de snippets
go test ./pkg/core -run TestDocumentation -v

# Verificación y sincronización de documentación internacionalizada (i18n)
go run ./tools/docsi18n --check

# Validación de la extensión VS Code
cd vscode-joss
npm ci
npm run compile
cd ..
```

## Reglas de Ciclo de Vida, Runtime Pool, Web y ORM

- **Pipeline y Aislamiento**: `analyzer` (`collectDeclarations → projectScope → validateNominalContracts → analyzeSourceBodies`) nunca importa `core`; `server` nunca define semántica; `formatter` conserva trivia.
- **Ciclo de Vida de `Runtime` (`runtime_lifecycle.go`)**:
  - Todo campo nuevo en `Runtime` debe declarar lifetime, owner, política de `Fork()` y política de `Free()`. El estado por request nunca sobrevive al pool. `DB` es compartido/externo y no se cierra en `Free()`.
  - `Runtime` mantiene una guarda `atomic.Bool` compare-and-swap en `Free()` contra *double-free*.
  - En HTTP, el cleanup se registra inmediatamente tras `Fork()`. La adaptación de resultados Joss→HTTP pertenece a `response_writer.go`.
- **Plugins y Drivers Nativos**: `PluginRegistry` y `NativeDriverDefinition` usan ownership contado (`Fork()` retiene, `Free()` libera y el último owner descarga).
- **Sesiones HTTP y WebSockets**:
  - Backends válidos de sesión: `memory`, `file` y `redis` (nombres desconocidos fallan explícitamente).
  - WebSockets usan callbacks `onMessage` y `onClose` con cleanup idempotente y runtime forkeado por conexión.
- **Directivas de Plantillas (`pkg/viewtemplate`)**: Cálculo de columnas basado en runas Unicode (`utf8.RuneCountInString`); prohibido duplicar regex de directivas fuera de `pkg/viewtemplate`.
- **Model ORM y GranDB**:
  - La metadata de subclases de `Model` se construye desde literales del AST y es inmutable en `classMetadataCache`.
  - Valores SQL siempre viajan como bindings; tablas, columnas y operadores se validan estructuralmente. Un `string` de Joss siempre es dato, nunca SQL crudo.
  - Diferencias entre motores SQL pertenecen exclusivamente a `databaseDialect`.
  - `update` y `delete` requieren filtros obligatorios.
  - Accessors/mutators usan `get{Studly}Attribute` / `set{Studly}Attribute`. Hydration no ejecuta mutators.
- **Clases Genéricas, Records y Canales**:
  - `public class Box<T>` y `public record Nombre(...)` se validan nominalmente.
  - `channel<T>` emite `JOSS-CHANNEL-001` (cerrado) y `JOSS-CHANNEL-002` (capacidad inválida).
- **Archivos de Prueba**: `*_test.joss` y carpetas `test/`/`tests/` (`parser.IsTestSourceFile()`) se excluyen del análisis estricto de proyecto en `joss check`.

---

## 8. Reglas de internacionalización (i18n) y archivos ARB

1. **Aislamiento estricto de sintaxis CLI**: En `intl_*.arb` va únicamente prosa traducible. Nunca incluir comandos (`joss run`), flags (`--write`) ni rutas de ejemplo dentro de cadenas ARB; formatearlos en Go con `cliUsageLabel`.
2. **Prompts interactivos (`s/n` vs `y/n`)**: Nunca incluir `(s/n): ` en el ARB; formatearlo en Go y aceptar `s`, `si`, `y`, `yes`.
3. **Placeholders y paridad 100%**: Toda variable `{param}` exige metadata `@key` con `"placeholders"`. Los 30 archivos `intl_*.arb` deben tener exactamente las mismas claves.
4. **Protección de códigos `JOSS-...` y `joss.yaml`**: Jamás traducir códigos de diagnóstico ni claves técnicas (`name, version, repository`).

---

## 9. Principio de Paridad Total de Lenguaje (Feature Once, Execute Everywhere)

> **One Language. One Semantics. Multiple Execution Modes.**

1. **Un solo lenguaje**: `joss run`, `joss server`, `joss program` y `joss build` ejecutan el mismo lenguaje con la misma semántica a partir de `PreparedProgram`.
2. **Prohibición de `JOSS-NATIVE-001` en Core Language**: Toda característica de `CORE LANGUAGE` debe tener lowering en `pkg/ir` y emisión en `pkg/backend/native`.
3. **Autonomía del Toolchain**: El usuario final sólo necesita `joss` para programar, ejecutar y compilar binarios nativos.

---

## 10. Contratos de Arquitectura y Verificación Continua (CI Gates)

Todo cambio debe mantener en verde `tests/architecture/architecture_contract_test.go`:
* `TestArchitecture_SemanticIntegrityAndSharedPreparedProgramReuse`
* `TestArchitecture_NativeLoweringNeverResolvesTypesOrCallsIndependently`
* `TestArchitecture_NoASTSerializationOrMonolithicRuntimeInNative`
* `TestArchitecture_CoreLanguageImplementedFeaturesMustExecuteEverywhere`
* `TestArchitecture_EveryCoreLanguageFeatureExecutesEverywhere`
* `TestArchitecture_LanguageFeatureMustBeImplementedOnceAndExecuteEverywhere`
* `TestArchitecture_Regression_InterpreterSupportedNativeUnsupportedRejected`

---

## 11. Compilación Nativa (`joss build`), Poda Selectiva de Motores (`no<driver>`) y Sellado VFS

### 11.1 Directorio de salida y Compilación Cruzada (Multi-OS / Multi-Arch)
* Todo `joss build` sin `-o` deposita los binarios compilados dentro de `build/` (`build/<nombre>.exe` o `build/<nombre>`).
* **Compilación cruzada nativa integrada (`--target` o atajo directo)**:
  - `joss build main.joss --target=linux-amd64` (o `joss build linux`)
  - `joss build main.joss --target=linux-arm64`
  - `joss build main.joss --target=darwin-arm64` (o `joss build macos` — Apple Silicon M1/M2/M3/M4)
  - `joss build main.joss --target=darwin-amd64` (macOS Intel)
  - `joss build main.joss --target=windows-amd64` / `--target=windows-arm64`
  - `joss build main.joss --target=android-arm64` (o `joss build android` — Android / Termux)

### 11.2 Separación entre Binarios CLI Puros y Aplicaciones con Host Bridge
* **Programas CLI puros**: Se compilan a código máquina puro (~5.5 KB - 32 KB) sin `joss_host.dll` y sin trailer VFS (`JOSS_RUNNER_DATA`).
* **Aplicaciones completas con servicios de servidor/DB**: Enlazan selectivamente `_joss_host_bridge` y sellan el VFS encriptado en el ejecutable.

### 11.3 Regla Obligatoria para Nuevos Motores, Bases de Datos o SDKs Externos (Patrón `no<servicio>`)
> **REGLA ESTRICTA DE ARQUITECTURA**: Ninguna librería externa pesada (nuevo motor de base de datos como MongoDB, Cassandra, ClickHouse, o SDK de nube como AWS S3, Firebase, Kafka, etc.) puede importarse de forma incondicional en `pkg/core` ni en `pkg/server`.

Siempre que se agregue un nuevo driver o integración externa (ejemplo: **MongoDB**):
1. **Aislamiento por Build Tag (`!nomongodb` / `nomongodb`)**:
   - Crear `pkg/core/drivers_db_mongodb.go` (o `pkg/core/lib_<servicio>.go`) con `//go:build !nodb && !nomongodb` que contenga el `import` del paquete externo.
   - Crear su contraparte ligera `pkg/core/drivers_db_mongodb_stub.go` con `//go:build nodb || nomongodb` que devuelva un error claro sin importar la dependencia externa.
   - Drivers actualmente aislados bajo este estándar:
     - SQLite: `!nodb && !nosqlite` (`drivers_db_sqlite.go`)
     - MySQL: `!nodb && !nomysql` (`drivers_db_mysql.go`)
     - PostgreSQL: `!nodb && !nopostgres` (`drivers_db_postgres.go`)
     - SQL Server: `!nodb && !nomssql` (`drivers_db_mssql.go`)
     - Oracle Cloud OCI: `!nooci` / `nooci` (`lib_storage_oci.go` / `lib_storage_nooci.go`)
     - Redis: `!noredis` / `noredis` (`redis.go` / `redis_stub.go`)
2. **Detección Automática en `cmd/joss/native_builder.go`**:
   - Registrar la detección del nuevo servicio en `collectProjectFiles` leyendo `core.DetectAndLoadEnv(nil)` (ej. `DB=mongodb` → `capsMap["mongodb"] = true`) y en el grafo de alcanzabilidad AST.
   - En `optimizeHostBridgeDLL` y `compileRunnerBinary`, si `!detectedCaps["mongodb"]`, inyectar automáticamente el tag `nomongodb` en `-tags=...` para que los proyectos que no usen MongoDB jamás carguen su peso en el binario ni en `joss_host.dll`.

### 11.4 Sellado VFS Encriptado (`env.enc` AES-256) y Optimización del `.exe`
* `appendSealedAssetsToExecutable` empaqueta el proyecto dentro del ejecutable final:
  - Encripta las variables de entorno con **AES-256** bajo `env.enc` y excluye `.env` / `env.joss` en texto plano.
  - Excluye directorios y archivos de desarrollo (`bruno/`, `.joss/`, `tests/`, `package-lock.json`, `Dockerfile`, `.dockerignore`, `translation_history.json`, `AGENTS.md`).
  - Optimiza en memoria imágenes PNG > 150 KB (`optimizeLargePNG` a 720px con `png.BestCompression`) sin modificar los archivos fuente en disco.
  - En runtime, `_joss_host_bridge` desencripta el VFS en RAM y `Runtime.writeEnvJoss()` nunca escribe `env.joss` a disco cuando `GlobalFileSystem != nil`.




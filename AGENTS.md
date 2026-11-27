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
              Joss Native JIT        Joss Native HTTP      Joss Native AOT
           (Tier-1 / Tier-2 LLVM)    (`std/http` + C11)   (`pkg/ir` → backend)
                   │                      │                       │
                   ▼                      ▼                       ▼
             Salida Consola        Respuesta HTTP        Binario Standalone (.exe/ELF)
```

- `pkg/parser`: Tokens, lexer, parser Pratt con tabla de precedencias canónicas y AST.
- `pkg/typesystem`: Tipos canónicos, compatibilidad de asignación (`Assignable`), inferencia (`MergeInference`) y coerción explícita (`CoerceString`).
- `pkg/analyzer`: Unidades fuente (`SourceUnit`), scopes léxicos, tablas de símbolos, firmas, comprobación de tipos y generación inmutable de `PreparedProgram`. **Nunca** debe importar `pkg/core`.
- `pkg/diagnostics`: Modelo de errores y advertencias (`Diagnostic`) con código estable, severidad, rango, explicación y sugerencia.
- `pkg/core`: Catálogo de built-ins globales, firmas nativas (`native_signatures.go`), utilidades de entorno/estilos y oráculo de pruebas de referencia; el CLI de usuario (`joss run`, `server`, `program`, `build`, `test`, `eval`, `repl`) **nunca** ejecuta apps en el intérprete Go.
- `pkg/ir`: Representación intermedia canónica (*Joss Native IR*), lowering de `PreparedProgram`, instrucciones SSA/3AC y verificador de tipos de bajo nivel (`ir.Verifier` con verificación de dominancia SSA).
- `pkg/backend/native`: Generadores de código nativo (`x86_64` y `AArch64`) y emisor de LLVM IR (`llvm.go`, incluyendo `Auto-DP`). **Nunca** debe importar `os/exec` (protegido por `TestAuditZeroExternalProcessesAndZeroGoRuntime` y `TestCleanEnvironment003`).
- `pkg/backend/llvm`: Integración externa con `clang` + `lld` registrada mediante `native.RegisterExternalLLVMCompiler`; compila y enlaza condicionalmente los módulos de `runtime/native/` y librerías C del sistema (`-lsqlite3`, `-lpq`, `-lmariadb`/`-lmysqlclient`, `-lodbc`, `-lcrypto`, `-pthread`).
- `runtime/native`: Runtime modular en C11 sin runtime Go ni DLLs propias de Joss (`joss_rt.{h,c}`, `joss_web_rt.c`, `joss_concurrency_rt.c`, `joss_db_rt.c`, `joss_crypto_rt.c`, `joss_gui_rt.c`).
- `pkg/golite`: Prototipo de compilación/reutilización controlada de algoritmos Go compatibles hacia el pipeline nativo.
- `pkg/bytecode`: Serialización comprimida del AST (`JOSSBC2Z`) para plugins `.jp`, no para ejecutables nativos AOT.
- `pkg/pluginruntime`, `pkg/pluginpkg`, `pkg/plugincompiler`: Runtime, paquetes y JPBC de plugins.
- `cmd/joss`: CLI y orquestación unificada 100% nativa del proyecto (`run`, `server`, `program`, `build`, `test`, `eval`, `analyze`, `emit-ir`, `repl`).
- `vscode-joss`: Servidor de lenguaje (LSP) y extensión de editor.

La dirección estricta de dependencias es:
`parser / typesystem / diagnostics → analyzer → ir → backend/native ← backend/llvm (registro por hook)`.
`pkg/analyzer` **nunca** debe importar `pkg/core`, y `pkg/backend/native` **nunca** debe importar `os/exec`. `pkg/server`, `cmd/runner`, `legacy_build.go` y `go-webview2` están retirados permanentemente (protegido por `TestArchitecture_ZeroGoRuntimeEngineOrLegacyRunner`).

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
   - **Paridad Obligatoria Multi-Arquitectura (`x86_64` y `AArch64`)**: Siempre se deben considerar tanto `machine_code_aarch64` como `machine_code_x86_64` para asegurar que el backend emita código para ambas arquitecturas, garantizando que funcione en Linux, Windows y macOS sin importar su arquitectura.
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

# Validación estricta del runtime nativo C11 (0 warnings obligatorio)
clang -Wall -Wextra -Werror -O2 \
  runtime/native/joss_rt.c runtime/native/joss_web_rt.c \
  runtime/native/joss_concurrency_rt.c runtime/native/joss_db_rt.c \
  runtime/native/joss_crypto_rt.c runtime/native/joss_rt_test.c \
  -lsqlite3 -lcrypto -pthread -o /tmp/joss_rt_test && /tmp/joss_rt_test

# Nota: En entornos sin acceso a red / caché parcial de módulos Go, usar:
# go vet -tags "nodb noredis nowebview2" ./pkg/backend/native ./pkg/backend/llvm ./tests/native
# go test -tags "nodb noredis nowebview2" ./pkg/backend/native ./pkg/backend/llvm ./std ./tests/architecture ./tests/native

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
* `TestArchitecture_ZeroGoRuntimeEngineOrLegacyRunner`

---

## 11. Compilación Nativa (`joss build`), Modo Autónomo Standalone y Cero DLLs

### 11.1 Principio de Binario Único y selección automática
> **REGLA ESTRICTA DE ARQUITECTURA**: Todo ejecutable generado por `joss build` es un binario único, autocontenido y autónomo (`build/<nombre>.exe` en Windows o `build/<nombre>` en Linux/macOS).
* **Sin DLLs complementarias**: No generar, distribuir ni requerir bibliotecas dinámicas junto al ejecutable final. El runtime de plataforma del sistema operativo no cuenta como archivo complementario distribuido por Joss.
* **Arquitectura 100% Nativa (Cero Runtime Go)**: `joss run`, `joss server`, `joss program` y `joss build` compilan y ejecutan el 100% de las aplicaciones mediante el pipeline nativo `PreparedProgram → IR → LLVM/AArch64/x86_64 + Runtime C11`. `pkg/server`, `cmd/runner`, `legacy_build.go` y `go-webview2` están retirados permanentemente (protegido por `TestArchitecture_ZeroGoRuntimeEngineOrLegacyRunner`).
* **Gestión de Memoria Unificada (Arena + JossOwnedScope + Mini-GC)**:
  - En servidores HTTP multihilo (`joss server`), toda asignación per-request (strings, JSON, rutas, vistas, `html_escape`, `DateTime`, `Model`, arrays, mapas) usa el arena `_Thread_local` (`joss_req_alloc` / `joss_req_alloc_zeroed` / `joss_sb_to_req_string`) con reset `O(1)`, reducción de chunks sobredimensionados (`> 256 KB`) y `malloc_trim(0)` periódico en Linux (`glibc`).
  - Los snapshots persistentes (Sesiones y Caché) viven en `JossOwnedScope` independientes que se destruyen al sobrescribir/expirar/olvidar (`Cache::get` clona al arena del request actual). Las sesiones aplican eviction TTL/LRU (`SESSION_MAX_ENTRIES`, default `2048`) y persistencia atómica con reintentos.
  - En programas CLI/batch monohilo, el Mini-GC recolecta objetos cíclicos, arrays, mapas (con compactación de tombstones), strings (`JOSS_GC_RAW`) y structs (`JOSS_GC_STRUCT`).
  - La clase nativa `Memory` (`rss`, `arenaUsed`, `gcStats`, `gcCollect`, `trim`, `sizeOf`, `snapshot`, `diff`, `report`, `checkGrowth`) expone diagnósticos en tiempo real estilo Rust.
* **Desacoplamiento Estricto de Catálogos de Terminal (i18n)**: Los archivos de traducción de consola CLI (`intl_*.arb`) pertenecen exclusivamente a la herramienta interactiva de desarrollo (`joss`) y nunca deben compilarse ni embeberse en binarios de producción generados con `joss build` (gobernado por la poda `noembedlocales`).

### 11.2 Directorio de salida y Compilación Cruzada (Multi-OS / Multi-Arch)
* Todo `joss build` sin `-o` deposita los binarios compilados dentro de `build/` (`build/<nombre>.exe` o `build/<nombre>`).
* **Compilación cruzada nativa integrada (`--target` o atajo directo)**:
  - `joss build main.joss --target=linux-amd64` (o `joss build linux`)
  - `joss build main.joss --target=linux-arm64`
  - `joss build main.joss --target=darwin-arm64` (o `joss build macos` — Apple Silicon M1/M2/M3/M4)
  - `joss build main.joss --target=darwin-amd64` (macOS Intel)
  - `joss build main.joss --target=windows-amd64` / `--target=windows-arm64`
  - `joss build main.joss --target=android-arm64` (o `joss build android` — Android / Termux)

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
     *(Nota: Storage y Oracle Cloud OCI fueron migrados 100% a Pure Joss Standard Library en `std/storage/storage.joss`, eliminando dependencias de SDKs en Go)*
     - Redis: `!noredis` / `noredis` (`redis.go` / `redis_stub.go`)
2. **Detección Automática en `cmd/joss/native_builder.go`**:
   - Registrar la detección del nuevo servicio en `collectProjectFiles` leyendo `core.DetectAndLoadEnv(nil)` (ej. `DB=mongodb` → `capsMap["mongodb"] = true`) y en el grafo de alcanzabilidad AST.
   - En la compilación y empaquetado, si `!detectedCaps["mongodb"]`, inyectar automáticamente el tag `nomongodb` en `-tags=...` para que los proyectos que no usen MongoDB jamás carguen su peso en el binario final.

### 11.4 Implementación Canónica en Lenguaje Joss Puro (`Pure Joss Standard Library` en `std/`)
* **Regla de Única Fuente de Verdad (Full Joss)**: Siempre que una funcionalidad, subsistema, query builder o estructura de datos pueda implementarse lógicamente en código fuente Joss (ejemplo canónico: **GranDB** en `std/database/grandb.joss`), **es obligatorio escribirla en código puro Joss**. Esto garantiza que `joss run`, `joss server`, `joss program` y `joss build` compartan una **única fuente de verdad** canónica.
* **Prohibición de bifurcación de código**: Queda terminantemente prohibido mantener dos implementaciones de una misma característica (ej. escribir implementaciones ad-hoc en ensamblador para un motor específico y otra en Go). Si se da soporte a múltiples motores (MySQL, PostgreSQL, SQL Server, SQLite), debe hacerse de forma consistente y unificada a través de la arquitectura canónica de Joss.

---

## 12. Arquitectura Nativa de Nueva Generación (Cómo desarrollar ahora)

La arquitectura de compilación nativa ha evolucionado a un modelo modular (**Pure Joss + LLVM/C ABI + Runtime C11 mínimo**, sin runtime Go ni DLLs propias de Joss). Para contexto completo de diseño, decisiones y métricas, consultar `docs/compiler/` (`FINAL_IMPLEMENTATION_REPORT.md`, `MEMORY_MODEL.md`, `IR_AND_BACKEND_STRATEGY.md`, `IMPLEMENTATION_PROGRESS.md`).

### 12.1 Separación estricta `pkg/backend/native` vs `pkg/backend/llvm`
- `pkg/backend/native`: Contiene los emisores de código máquina (`machine_code_x86_64.go`, `machine_code_aarch64.go`), la emisión de LLVM IR (`llvm.go`) y validación de capacidades (`capability_gaps.go`). **Prohibido importar `os/exec`** dentro de `pkg/backend/native`.
- `pkg/backend/llvm/compiler.go`: Registra el compilador LLVM externo vía `native.RegisterExternalLLVMCompiler`, invoca `clang`/`lld`, compila únicamente los módulos de `runtime/native/` que el programa necesita y enlaza las librerías C disponibles en el sistema.
- Todo binario o test que requiera `BackendLLVM` debe importar el hook con `_ "github.com/jossecurity/joss/pkg/backend/llvm"`.

### 12.2 Runtime C11 Modular (`runtime/native/`)
Cada responsabilidad del runtime nativo vive en su propio módulo; **no mezclar dominios ni duplicar implementaciones**:
1. **`runtime/native/joss_rt.{h,c}` + `joss_memory_rt.inc` (Core + Memoria + Diagnósticos)**:
   - Tipos base, strings, arrays (`JOSS_ARR_MAGIC`), maps (`JOSS_MAP_MAGIC` con compactación de tombstones), `ByteBuffer`, `StringBuilder`.
   - En 64-bit, `JOSS_MIN_PTR_ADDR` es `0x100000000ULL` (no reducirlo: evita dereferenciar enteros grandes como punteros).
   - **Request-Scoped Arena Allocator (`_Thread_local`) + `JossOwnedScope` + Mini-GC**: `joss_req_arena_begin`, `joss_req_arena_reset`, `joss_req_alloc`, `joss_req_alloc_zeroed`, `joss_req_strdup`, `joss_sb_to_req_string` y `joss_owned_scope_{begin,end,destroy}` garantizan liberación `O(1)` por request HTTP, aislamiento de snapshots de sesión/caché y recolección de ciclos/strings/structs en CLI.
   - `joss_memory_rt.inc` implementa la clase nativa `Memory` (`rss`, `arenaUsed`, `gcStats`, `gcCollect`, `trim`, `sizeOf`, `snapshot`, `diff`, `report`, `checkGrowth`).
2. **`runtime/native/joss_web_rt.c` + `joss_redis_rt.inc` + `joss_zip_rt.inc` (HTTP, Sockets, JSON, Sesiones, Caché, Redis, ZIP)**:
   - Contexto de request/sesión `_Thread_local`, sockets con `TCP_NODELAY`, `SO_RCVTIMEO`/`SO_SNDTIMEO`, tope 10 MB de lectura, guarda anti-path-traversal (`..`, `%2e`, `%5c`, null-byte), serializador/parser JSON, redirects/cookies, SHA-1+Base64 (RFC 3174), fechas (`joss_date`, `joss_strtotime`), sesiones con eviction TTL/LRU (`SESSION_MAX_ENTRIES`) y persistencia atómica con reintentos.
   - Conecta el Arena al ciclo de vida de cada conexión en `fn_Socket_accept`, `fn_Socket_receive` y `fn_Socket_close`.
   - `joss_redis_rt.inc` contiene el cliente RESP2 TCP (`fn_Redis_*`) y `joss_zip_rt.inc` contiene el extractor ZIP/DEFLATE/CRC-32 (`Zip::extract`).
3. **`runtime/native/joss_concurrency_rt.c` (Gorutinas Nativas + Auto-DP)**:
   - Pool de hilos multi-core sobre `pthread`/Win32 para `async { ... }` / `await` (`joss_async`, `joss_async_detached`, `joss_await`) y canales acotados (`joss_make_chan`, `joss_send`, `joss_recv`, `joss_close` con liberación de buffer al drenar).
   - Caché `_Thread_local` sin bloqueos para **Programación Dinámica Automática (`Auto-DP`)** (`joss_dp_get`, `joss_dp_put`, 8192 slots con reserva perezosa).
4. **`runtime/native/joss_db_rt.c` + `joss_db_wire.inc` (GranDB Nativo Multi-Motor)**:
   - Drivers C nativos parametrizados para **SQLite3** (`sqlite3.h` con `PRAGMA journal_mode=WAL` y `busy_timeout=5000`), **PostgreSQL** (`libpq-fe.h`, reescritura `?` → `$1..$N`, `PQexecParams`), **MySQL/MariaDB** (`mysql.h`, `MYSQL_STMT`) y **SQL Server / ODBC** (`sql.h`/`sqlext.h`, `SQLPrepare`/`SQLBindParameter`), además de tablas *in-memory* de respaldo.
   - Nunca duplicar lógica SQL en `joss_web_rt.c`; toda consulta nueva de BD pasa por `joss_db_rt.c`.
5. **`runtime/native/joss_crypto_rt.c` + `joss_crypto_primitives.inc` (Criptografía Real OpenSSL EVP)**:
   - `joss_sha256` (EVP SHA-256), `joss_random_hex` (CSPRNG `RAND_bytes`), `joss_password_hash` (`PBKDF2-HMAC-SHA256`, 100,000 iteraciones + salt aleatorio) y `joss_password_verify` (comparación en tiempo constante con `CRYPTO_memcmp`).
   - Prohibido introducir hashes débiles, RNG no criptográfico para secretos o claves hardcodeadas.

**Regla de calidad C**: Todo cambio en `runtime/native/*.c` y `*.inc` debe compilar con `clang -Wall -Wextra -Werror -O2` (**0 warnings**) y pasar `runtime/native/joss_rt_test.c`.

### 12.3 Programación Dinámica Automática (`Auto-DP`)
- En `pkg/backend/native/llvm.go`, `isAutoDPEligible(fn)` identifica funciones puras recursivas con 1 o 2 parámetros enteros e inyecta automáticamente memoización (`dp_entry`/`dp_ret` + `joss_dp_put`).
- **Invariante obligatorio**: Nunca activar `Auto-DP` sobre funciones con efectos secundarios, E/S, mutación global o llamadas impuras.

### 12.4 Reglas de Seguridad y Cero Stubs Silenciosos
- **Cero stubs silenciosos**: Si una instrucción IR o capacidad nativa no está soportada, el compilador o runtime debe fallar explícitamente con un error claro; nunca devolver `0`, `null` o `""` silenciosamente fingiendo éxito.
- **Secretos y rutas**: Nunca incrustar variables de `.env` en ejecutables compilados (`pkg/core/env_loader.go` / `pkg/backend/native/native_env.go`). Toda resolución de rutas estáticas HTTP debe validar contra *path traversal* (`std/http/server.joss` y `runtime/native/joss_web_rt.c`) y respetar `TRUST_PROXY` para `X-Forwarded-For`.
- **Suites nativas de regresión obligatorias** (en `tests/native/`):
  - `phase0_security_and_mixed_test.go` (seguridad y valores dinámicos).
  - `memory_leak_guard_test.go` (API `Memory::*` y guardia automática de crecimiento de memoria).
  - `tutorial_web_probe_test.go` (app web MVC real compilada a ELF y servida en vivo).
  - `concurrency_and_autodp_test.go` (`async`/`await` paralelo, canales y `Auto-DP`).
  - `grandb_and_crypto_native_test.go` (CRUD real SQLite3 en disco y OpenSSL PBKDF2/SHA-256).
  - `native_vs_go_benchmark_test.go` (paridad y rendimiento Nativo vs Go).


# Guía de arquitectura y reglas operativas para agentes y desarrolladores

Este archivo es una guía operativa y de gobernanza técnica del repositorio, no un changelog. Antes de modificar semántica, código o documentación, lea `docs/ARQUITECTURA.md`, `docs/SISTEMA_TIPOS.md`, `docs/DIAGNOSTICOS.md`, `docs/DOCUMENTATION_AUDIT.md` y los tests del subsistema afectado.

---

## 1. Principio fundamental de desarrollo

> **El código fuente y sus tests ejecutados son la única fuente de verdad.**

No asuma que un comentario, un archivo histórico o una tesis describe el comportamiento real del sistema:
- Verifique siempre la implementación concreta en `pkg/parser`, `pkg/typesystem`, `pkg/analyzer` y `pkg/core`.
- Cualquier cambio en la semántica del lenguaje exige pruebas unitarias, de análisis semántico y de ejecución runtime.

---

## 2. El Pipeline Real de Joss

```text
.joss → lexer → parser Pratt → AST → semantic analyzer → diagnostics
                                      ↓ (si no hay errores)
                                 intérprete / runtime Go
```

- `pkg/parser`: Tokens, lexer, parser Pratt con tabla de precedencias y AST.
- `pkg/typesystem`: Tipos canónicos, compatibilidad de asignación (`Assignable`), inferencia (`MergeInference`) y coerción explícita (`CoerceString`).
- `pkg/analyzer`: Unidades fuente (`SourceUnit`), scopes léxicos, tablas de símbolos, firmas, comprobación de tipos y flujo alcanzable exhaustivo. No debe importar `pkg/core`.
- `pkg/diagnostics`: Modelo de errores y advertencias (`Diagnostic`) con código estable, severidad, rango, explicación y sugerencia.
- `pkg/core`: Evaluador de AST, runtime Go, frames léxicos, slots, built-ins globales y clases nativas integradas; adapta sus registros al analyzer.
- `pkg/bytecode`: Serialización comprimida del AST (`JOSSBC2Z`), no código máquina ni LLVM IR.
- `pkg/pluginruntime`, `pkg/pluginpkg`, `pkg/plugincompiler`: Runtime, paquetes y JPBC de plugins.
- `cmd/joss`: CLI y orquestación del proyecto.
- `vscode-joss`: Servidor de lenguaje (LSP) y extensión de editor.

La dirección estricta de dependencias es:
`parser / typesystem / diagnostics → analyzer → core adapter`. `pkg/analyzer` **nunca** debe importar `pkg/core`.

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

- `$x = 1`: La primera asignación declara e infiere `int`; las siguientes deben ser compatibles.
- `var $x = 1`: Inferencia explícita, también fija.
- `int $x = 1` o `let int $x = 1`: Tipo explícito.
- `let $x = 1`: `mixed` explícito; permite cambiar de tipo (no significa constante).
- `mixed $x = 1`: Dinamismo explícito equivalente; no existe un modo de tipado en `joss.yaml`.
- **Todo parámetro fuente debe declarar un tipo explícito**. Usa `mixed $x` si es dinámico; `$x` sin tipo ya no es válido (`JOSS-TYPE-011`).
- Los antiguos aliases `integer`, `double`, `boolean`, `dynamic`, `any` y `list` fueron retirados. Usarlos emite `JOSS-TYPE-009`. No deben reintroducirse accidentalmente.
- Una inicialización con `nil`/`null` pospone la inferencia hasta la asignación de un valor concreto.
- `T|null` declara una unión nullable; `T?` es solo un atajo sintáctico normalizado a `T|null`.
- `const $x = ...` infiere un tipo fijo inmutable; `const int $x = ...` lo declara explícitamente. También se protegen propiedades constantes.
- `public func name(...): Type` declara el tipo de retorno. Analyzer y runtime validan cada retorno explícito (`JOSS-TYPE-008`), y el analyzer exige retorno o throw en todas las rutas demostrables (`JOSS-TYPE-010`).
- Cada llamada a función o método usa un marco (*frame*) aislado. Las funciones con nombre solo ven sus parámetros, locales, `$this` y bindings del host; no ven variables de nivel superior del archivo ni del caller. Las closures sí capturan su entorno léxico. La recursión está limitada a 1024 llamadas por defecto (`Runtime.MaxCallDepth`).
- `ref T $x` y `call(ref $valor)` crean una referencia mutable temporal, estrictamente invariante y no escapable. Solo acepta variables no constantes; no admite defaults, campos, índices, almacenamiento en variables, retorno ni paso a llamadas nativas o `async`.
- Clases y funciones globales exigen `public` o `private`; métodos y propiedades exigen `public`, `protected` o `private`. `static` nunca añade visibilidad implícita. `Init` y closures no llevan modificador.

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

# Validación de la extensión VS Code
cd vscode-joss
npm ci
npm run compile
cd ..
```

## Tercera fase de arquitectura — septiembre de 2026

El pipeline del analyzer debe permanecer explícito (`collectDeclarations → projectScope → validateNominalContracts → analyzeSourceBodies`) y sus fases se mantienen en archivos cohesivos del mismo package, no en una colección de managers públicos. La resolución de llamadas y miembros consume firmas semánticas; analyzer y runtime pueden compartir metadata/typesystem, pero nunca estado ni ejecución.

Toda extracción de `Runtime`, `MainHandler` o `pub_cli` exige primero characterization tests reproducibles: pool/fork/reset, HTTP con `httptest`, y CLI con `t.TempDir`/registry en memoria. Un runtime devuelto al pool debe limpiar estado por request/ejecución; configuración persistente y recursos externos compartidos deben documentarse explícitamente. No se deben introducir contratos falsos para métodos nativos variádicos.

Las reglas arquitectónicas negativas siguen vigentes: analyzer no importa core; server no define semántica; formatter conserva trivia; VM no promociona semántica experimental. Para directivas de templates, compartir interpretación requiere una representación sintáctica, no una regex duplicada. Las métricas de fan-in/out y Change Surface son observación, no objetivos cosméticos.

## Cuarta fase de arquitectura — septiembre de 2026

- `Runtime` se adquiere/libera mediante `runtime_lifecycle.go`. Todo campo nuevo debe declarar lifetime, owner, política de Fork y política de Free; estado de request/ejecución nunca sobrevive al pool. `DB` es externo y no se cierra en `Free`.
- En HTTP, registre el cleanup inmediatamente después de `Fork`. Resultados Joss→HTTP pertenecen a `response_writer.go`; request adaptation, rate limit y session storage no deben volver a mezclarse allí.
- Métodos nativos nuevos deben usar `NativeMethodDefinition` cuando el contrato sea fiable. No represente aridad desconocida como cero parámetros. Runtime implementa; analyzer/LSP/docs consumen proyecciones.
- Binding runtime y validación analyzer permanecen separados, pero ambos deben respetar positional/named/default/ref/duplicate/missing a partir de la misma firma semántica.
- La sintaxis de directivas pertenece a `pkg/viewtemplate`; runtime y tooling no deben añadir regex locales para `@json` u otras directivas migradas.
- Tests nunca escriben en HOME real. Use `t.TempDir`, `httptest` y `GOCACHE` temporal si el host tiene ACLs defectuosas. Para validar la extensión con VS Code bloqueando `node_modules`, use un checkout/directorio temporal limpio.
- La VM experimental sólo entra en `differentialFeatures` cuando Interpreter y VM soportan realmente la feature. Interpreter y documentación continúan siendo autoridad runtime publicada.

## Quinta fase de arquitectura — septiembre de 2026

- **Ownership de Plugins y Drivers Nativos**: `PluginRegistry` y `NativeDriverDefinition` separan el AST inmutable y los handles dinámicos del estado mutable por ejecución. `PluginAwareHost` (`Runtime`) mantiene engines AST y namespaces aislados por cada instancia/fork. `Runtime.Fork()` duplica facades de engine y namespaces apuntando al fork hijo sin compartir estado léxico ni colisionar nombres idénticos entre plugins. `NativeDriverDefinition.Unload()` y la función de plataforma `unloadNativeDriverHandle` descargan handles (`FreeLibrary`/`dlclose`) de forma idempotente con protección concurrente `driverMu`.
- **Sesiones HTTP y Redirect Flash**: Los backends de sesión (`memory`, `file`, `redis`) encapsulan almacenamiento y recuperación de IDs. La persistencia de flash en redirecciones (`persistRedirectFlash`) reutiliza unificadamente `saveSession`; en caso de fallo de I/O en la sesión, la respuesta aborta inmediatamente con HTTP 500 y mensaje de error sin enviar cabecera `Location` ni filtrar estado parcial.
- **Ciclo de vida y aislamiento WebSocket**: La actualización HTTP a WebSocket aisla completamente el socket y sus callbacks Joss (`onConnect`, `onMessage`, `onClose`, `onError`). Los parámetros de ruta (`$params`) se propagan al contexto de callback y dos conexiones concurrentes ejecutan en runtimes forkeados independientes sin interferencia de estado o variables.
- **Protección contra Double-Free en Runtime Pool**: `Runtime` mantiene una guarda booleana `freed` protegida por `poolMu` para prevenir que llamadas duplicadas a `Free()` corrompan el `sync.Pool` o introduzcan carreras de datos en estructuras compartidas (`Classes`, `GlobalScope`).
- **Metadata Nativa Declarativa**: 10 clases nativas (`Stack`, `Queue`, `Math`, `JSON`, `Markdown`, `Str`, `UUID`, `Lang`, `Console`, `Zip`) se migran formalmente a `NativeMethodDefinition`, distinguiendo firmas canónicas, nombres tipados y aridad fija o variable hacia el analyzer, catálogo LSP y documentación.
- **Directivas y Posicionamiento de Plantillas**: `pkg/viewtemplate` implementa cálculo de columnas basado en runas Unicode (`utf8.RuneCountInString`), garantizando que caracteres acentuados o multibyte en directivas mantengan rangos 1-based exactos y compatibles con LSP/diagnósticos.

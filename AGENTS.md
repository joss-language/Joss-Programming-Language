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

- **Ownership de Plugins y Drivers Nativos**: `PluginRegistry` y `NativeDriverDefinition` separan el AST inmutable y los handles dinámicos del estado mutable por ejecución. `PluginAwareHost` (`Runtime`) mantiene engines AST y namespaces aislados por instancia/fork. Los handles usan ownership contado: `Fork()` retiene, `Free()` libera y el último owner descarga. `Unload()` explícito no puede invalidar un driver prestado y se serializa con llamadas activas.
- **Sesiones HTTP y Redirect Flash**: Los únicos backends válidos son `memory`, `file` y `redis`; nombres desconocidos deben fallar, no degradarse a memoria. Los snapshots de request clonan mapas/slices JSON anidados. La persistencia de flash reutiliza `saveSession`; un fallo aborta con HTTP 500 sin `Location` ni estado parcial.
- **Ciclo de vida y aislamiento WebSocket**: La actualización HTTP a WebSocket dispone de callbacks Joss implementados `onMessage` y `onClose`, cleanup idempotente y runtime forkeado por request. Los parámetros de ruta se entregan posicionalmente al setup handler. No anuncie `onConnect`, `onError` ni `$params` hasta que existan implementación y tests.
- **Protección contra Double-Free en Runtime Pool**: `Runtime` mantiene una guarda `atomic.Bool`; `Free()` usa una transición compare-and-swap para que un puntero sólo pueda reingresar una vez a `sync.Pool`. El singleton de assets debe conservar inicialización sincronizada.
- **Metadata Nativa Declarativa**: 10 clases nativas (`Stack`, `Queue`, `Math`, `JSON`, `Markdown`, `Str`, `UUID`, `Lang`, `Console`, `Zip`) usan `NativeMethodDefinition`. Sólo se publican nombres y retornos comprobados; `ArityKnown=false` significa desconocimiento legítimo y no debe describirse como aridad exacta.
- **Directivas y Posicionamiento de Plantillas**: `pkg/viewtemplate` implementa cálculo de columnas basado en runas Unicode (`utf8.RuneCountInString`), garantizando que caracteres acentuados o multibyte en directivas mantengan rangos 1-based exactos y compatibles con LSP/diagnósticos.

## Sexta fase de arquitectura — Model ORM

- La metadata de una subclase de `Model` se construye desde literales del AST, se conserva inmutable dentro de `classMetadataCache` y nunca usa un cache global mutable.
- Valores de DB pasan por el hydrator y sus casts antes de entrar en `Instance.Fields`; `NULL` permanece `nil`. La serialización aplica `hidden`/`visible` y relaciones cargadas.
- `fillable`/`guarded` solo gobiernan asignación masiva. Hydration y asignación explícita no dependen de esa política.
- `save()` usa el estado explícito `exists`, compara atributos actuales con `original`, omite UPDATE sin cambios y sincroniza el estado solo después de SQL exitoso.
- Consultas de `Model` componen el builder de GranDB. El builder directo conserva mapas; el builder de modelo hidrata instancias.
- `belongsTo`, `hasOne` y `hasMany` producen la misma consulta de modelo. `with`, `load` y `loadMissing` comparten un cargador eager por lotes e indexan resultados por clave.
- `belongsToMany` conserva atributos pivot fuera de los atributos relacionados. `sync` calcula attach/detach y toda escritura relacional compuesta reutiliza `Runtime.activeTx`; arrays vacíos nunca se convierten en borrado global.
- Soft delete es un scope de query removible y opt in mediante metadata. `forceDelete` es la única ruta de borrado físico para una instancia soft deleted.
- Hooks de modelo previos pueden cancelar retornando `false`; hooks posteriores se ejecutan únicamente después de SQL exitoso. No añada buses de eventos paralelos.

## Reglas de evolución de GranDB

- Los valores SQL se envían como bindings. Tablas, columnas, aliases, operadores y direcciones son estructura: deben validarse y compilarse, nunca tratarse como values ni interpolarse desde entrada de usuario.
- Una expresión del ORM debe tener un tipo interno explícito. Un `string` de Joss siempre representa datos, aunque su contenido sea `CURRENT_TIMESTAMP`, `NULL` o parezca una función SQL.
- Las diferencias entre SQLite, MySQL/MariaDB, PostgreSQL y SQL Server pertenecen a `databaseDialect`. No añada nuevas ramas de motor al dispatcher o a operaciones individuales cuando la diferencia pueda compilarse en el dialecto.
- Las escrituras desde maps ordenan columnas antes de construir SQL y bindings. Operaciones bulk exigen filas homogéneas, respetan el límite de parámetros del dialecto y son atómicas cuando abarcan varios lotes.
- Un método GranDB solo puede publicarse en `native.go` cuando el dispatcher tiene una ruta alcanzable, el retorno está proyectado al analyzer y existe al menos una prueba positiva. La compatibilidad de un motor solo se declara cuando una prueba de integración se ejecuta contra ese motor; los golden tests de SQL demuestran compilación, no ejecución.
- `update` y `delete` requieren filtros. Las variantes que afecten toda una tabla deben tener un nombre explícito y pruebas que demuestren la intención destructiva.
- Logging SQL es opt in. Nunca imprima bindings automáticamente; cualquier observer futuro debe admitir redacción y documentar su lifetime en `Runtime`, `Fork` y `Free`.

---

## 8. Reglas de internacionalización (i18n) y archivos ARB

Para mantener la consistencia entre los 30 idiomas soportados y evitar que los motores de traducción corrompan sintaxis de comandos o flujos interactivos de consola:

1. **Aislamiento estricto de sintaxis CLI**:
   * En los archivos `.arb` va **únicamente prosa humana, descripciones y etiquetas legibles**.
   * **Nunca** incluir comandos ejecutables (`joss run`, `joss migrate`, `joss make:...`), flags (`--write`, `--dry-run`), ni nombres de archivo de ejemplo (`main.joss`, `tu_script.joss`) dentro de los valores de cadenas a traducir.
   * La sintaxis CLI se formatea siempre en código Go con `fmt.Printf`, inyectando únicamente las etiquetas o descripciones traducidas.
   * Para prefijar líneas de ayuda con "Uso:", usar la clave canónica `cliUsageLabel` (`"Uso:"`), interpolando el comando en Go:
     ```go
     fmt.Printf("%s joss run [archivo.joss]\n", i18n.Tr("cliUsageLabel"))
     ```

2. **Prompts interactivos de consola (`s/n` vs `y/n`)**:
   * **Nunca** incluir el sufijo selector `(s/n): ` o `(y/n): ` dentro del texto de la pregunta en el ARB.
   * La cadena ARB debe contener únicamente la pregunta limpia en prosa (ej: `"¿Deseas descargar los archivos desde OCI hacia local?"`).
   * El indicador de opciones se formatea directamente en Go:
     ```go
     fmt.Printf("%s (s/n): ", i18n.Tr("storagePromptDownloadOci"))
     ```
   * La validación de entrada por teclado en Go debe aceptar indistintamente opciones afirmativas en español e internacional:
     ```go
     if text == "s" || text == "y" || text == "si" || text == "yes" {
     ```

3. **Placeholders y metadata obligatoria**:
   * Toda cadena que use variables interpoladas `{param}` debe tener obligatoriamente su bloque `@key` con `"placeholders": { "param": {} }`.
   * El tooling de traducción (`Traductor-de-Proyectos-arb-xml`) y los characterization tests (`pkg/i18n/i18n_test.go`) validan esta paridad.

4. **Paridad de claves al 100%**:
   * Los 30 archivos `intl_*.arb` deben contener exactamente el mismo conjunto de claves. No se admiten claves huérfanas ni traducciones que dejen el valor del comando en español en archivos de otros idiomas.

5. **Protección absoluta de códigos de diagnóstico y nombres canónicos**:
   * Los códigos estables de diagnóstico (`JOSS-SYM-008`, `JOSS-TYPE-011`, `JOSS-PARSE-001`, `JOSS-CALL-001`, etc.) son invariantes arquitectónicas del compilador, analizador y linter. **Bajo ninguna circunstancia se deben borrar, traducir, alterar o reemplazar**.
   * Nombres de archivos de configuración técnica (`joss.yaml`) y nombres de propiedades canónicas (`name, version, repository`) se mantienen estrictamente literales en todos los idiomas sin traducir ni añadir sufijos gramaticales de otros idiomas.

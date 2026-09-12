# Auditoría técnica del núcleo y alineación con la tesis (agosto de 2026)

[Índice](README.md)

## Línea base

Antes de los cambios, `go test ./...` y `go build ./...` pasaban; `go vet ./...` fallaba por construir manualmente una dirección SMTP incompatible con IPv6. `joss analyze` sobre JosSecurity producía 10 errores falsos y 14 warnings sin archivo: dos nombres built-in implementados pero ausentes del catálogo (`html_escape`, `unlink`) y siete usos de clases exportadas por plugins que el analyzer no consultaba (`BrevoClient`, `Notify`).

## Causas raíz encontradas

- Analyzer global basado en `map[string]int`, sin scopes, tipos ni unidad fuente.
- CLI concatenaba ASTs y eliminaba identidad de archivo.
- Catálogo de built-ins divergente del dispatcher: incluía funciones inexistentes y omitía funciones reales.
- Clases nativas y plugins se resolvían por caminos diferentes.
- `let $name` construía por error un símbolo llamado `$`.
- `VarTypes` sólo protegía declaraciones tipadas; primeras asignaciones y parámetros perdían su tipo.
- `CallMethod` y `CallMethodEvaluated` duplicaban binding y validación.
- El editor mantenía listas manuales separadas de keywords y clases/métodos nativos.
- CI sólo existía para distribución manual; no había workflow de push/PR.
- `go vet` reveló el uso de `fmt.Sprintf("%s:%s")` para SMTP en lugar de `net.JoinHostPort`.
- El pool conservaba `PluginRegistry` mientras borraba los símbolos expuestos, de modo que una reutilización podía omitir la recarga del plugin.

## Decisiones aplicadas

- Capas nuevas y consumidas: `pkg/typesystem`, `pkg/diagnostics`, `pkg/analyzer`.
- Scopes por callable y resolución de declaraciones a nivel de proyecto.
- Inferencia fija en primera asignación; `var` inferido; `let $x` dinámico explícito.
- Diagnósticos estructurados y deterministas por archivo/línea/columna.
- Entorno del analyzer adaptado desde registros reales de runtime y símbolos JP v2.
- Catálogo generado para VS Code, validado por CI.
- Las firmas ricas del editor se filtran contra ese catálogo; metadatos obsoletos ya no pueden publicar símbolos que el runtime no registra.
- Binding de métodos unificado en `CallMethodEvaluated`.
- Frames de invocación aislados, recursión directa/mutua/de método, límite de profundidad y contratos de retorno opcionales.
- Frames léxicos sin scope dinámico: las funciones con nombre no heredan locales del caller; las closures conservan su captura.
- Uniones `T|U`, nullable `T?`, retornos exhaustivos demostrables y firmas explícitas de retorno para todo el núcleo nativo.
- `const` y propiedades tipadas/constantes validadas por analyzer y runtime.
- Errores del parser almacenados directamente como diagnósticos estructurados; se eliminó la extracción de líneas desde strings.
- Eliminación física de `ImportStatement`, tablas/tokens de imports, linker textual de plugins y formato bytecode sin compresión.
- Retiro de APIs de compatibilidad sin consumidores canónicos: rutas crudas, inserts por arrays, Schema por mapas y `where(..., "json")`.
- Reset completo del registro de plugins al devolver un runtime al pool.
- Prueba de integración sobre todo JosSecurity.
- Fixture de proyecto versionado en `testdata/analyzer-project`; JosSecurity permanece como repositorio externo ignorado y su test se omite sólo cuando no está disponible.

## Resultado en JosSecurity

Al eliminar las causas de falsos positivos aparecieron seis problemas reales antes ocultos:

- `Math::length` no existe; se reemplazó por `count`.
- `Str::endsWith` no existe; se reemplazó por `str_ends_with`.
- `Str::upper` no existe; se reemplazó por `strtoupper`.
- Tres modelos heredaban de la clase eliminada `GranMySQL`; ahora heredan de `GranDB`.

Una segunda revisión detectó cinco warnings falsos: variables de modelos con el mismo nombre que su clase se confundían con acceso estático y no se marcaban como usadas al ser receptoras de `->`. Se corrigió la precedencia para que el símbolo léxico sombree a la clase y se añadió una regresión.

El resultado final es cero errores y cinco warnings, todos inspeccionados contra el código: `$id`, `$licenseData`, `$offset`, `$user` y `$domain` se inicializan pero no vuelven a leerse en sus respectivos callables. Son deuda real de JosSecurity, no bloquean la ejecución y no se modificaron sólo para obtener una salida vacía.

## Comparación con la tesis

La implementación coincide con la visión ALIM en el runtime integrado, parser Pratt, AST compartido, plugins aislados y toolchain único. La tesis, sin embargo, mezcla estado real, sintaxis conceptual y hoja de ruta:

| Afirmación de la tesis | Estado comprobado del repositorio |
|---|---|
| Pipeline incluye type checker | Existe un checker semántico inicial con inferencia fija, constantes, firmas/retornos y llamadas recursivas; aún no cubre taint, escape ni esquemas DB. |
| AOT/LLVM/Cranelift y código máquina | El build principal empaqueta AST serializado y el intérprete Go. LLVM/Cranelift no están implementados. |
| Lexer/parser en Rust | La implementación actual está en Go. |
| Inmutabilidad por defecto/ownership | No existe semántica de ownership ni inmutabilidad por defecto. |
| Imports/módulos con grafo y ciclos | La sintaxis histórica fue eliminada completamente y no volverá. Plugins y archivos convencionales se cargan automáticamente; Joss adopta deliberadamente un proyecto zero-imports. |
| Rutas/DB como nodos AST de primer orden | Hoy son llamadas a clases nativas (`Router`, `GranDB`), no nodos específicos. |
| 1,420 tests y 91.4% de cobertura | El repositorio contiene una suite Go mucho menor. Medición focalizada actual: parser 52.3%, typesystem 44.9%, analyzer 47.7% y core 14.8%; CI valida ejecución y no afirma una cobertura inexistente. |
| Taint analysis y 83% de vulnerabilidades | Existe un analizador de seguridad heurístico en el LSP, no un taint engine formal en el compilador. |

Estas diferencias no se “corrigieron” inventando características. La propuesta de módulos fuente del capítulo 11 queda expresamente descartada para Joss; la modularidad ALIM se conserva mediante componentes del runtime y plugins aislados. Las demás deben resolverse en la tesis distinguiendo implementación validada, sintaxis conceptual y trabajo futuro.


## Auditoría arquitectónica incremental — 11 de septiembre de 2026

La revisión del commit 4b41742 usó tamaño solo como señal y contrastó responsabilidades, dependencias, estado compartido y conocimiento duplicado. La línea base pasó salvo pkg/pluginpkg/TestLoadOrCreateSigningKey, que intentó escribir fuera del sandbox en C:\Users\Asus\.joss\keys.

### Mapa priorizado

| Prioridad | Archivo | Líneas aprox. | Diagnóstico | Acción |
|---|---|---:|---|---|
| P0 | cmd/joss/pub_cli.go | 1169 | credenciales, HTTP, resolución, caché, ZIP, YAML, lockfile y publicación | Extraer cliente, resolver, instalador y storage |
| P0 | pkg/analyzer/infer.go | 1075 | inferencia, calls, miembros, acceso, jerarquía y narrowing | Extraer resolver nominal y call validation |
| P0 | pkg/analyzer/analyzer.go | 927 | symbols, contratos nominales, statements y diagnostics | Separar collector, contratos y bodies |
| P0 | pkg/server/handler.go | 1042 | runtime, CORS, WebSocket, request, session, CSRF y response | God function; extracción progresiva iniciada |
| P0 | pkg/core/evaluator_infix.go | 772 | ternario, streams, pipeline, aritmética, postfix y match | Separar flujo, aritmética e incrementos |
| P0 | pkg/core/runtime.go | 691 | pool, lifecycle, fork, env, DB, instancias y preload | Extraer lifecycle, config y loader |
| P0 | pkg/core/evaluator_call.go | 550 | binding, frames, references, host calls y built-ins | Separar binder/frame de dispatch |
| P1 | pkg/parser/parser_statements.go | 1027 | declarations, nominal types, control, exceptions y select | Extraer por dominios gramaticales |
| P1 | pkg/parser/parser_expressions.go | 1088 | Pratt, literals, interpolation, collections, calls y match | Interpolación extraída; continuar por collections/callables |
| P1 | pkg/core/builtins_array.go | 681 | conversiones, collections y higher-order | Separar conversiones y algoritmos |
| P1 | pkg/core/native_seo.go | 588 | SEO y Sitemap | Separar agregados |
| P1 | pkg/core/view.go | 614 | eval, directives, sections y shared state | Extraer compilador de templates |
| Mantener | pkg/parser/ast_statements.go | 460 | modelo AST declarativo | Grande pero cohesivo |
| Mantener | pkg/typesystem/types.go | 402 | tipos, parse, assignability, inference y coercion | Fuente canónica cohesionada |
| Mantener | pkg/runtime/plan/callable.go | 443 | planes de callables | Especializado |

### Fuentes de verdad

| Concepto | Antes | Resultado/propuesta |
|---|---|---|
| Keywords | Ya canónicas en parser/token.go | Mantener KeywordNames |
| Símbolos | switch lexer + lista multichar + switch formatter | SymbolDefinition canónico |
| Tipos numéricos | typesystem + analyzer.isNumeric | Type.IsNumeric |
| Built-ins | lista + seis sets de retorno + cinco dispatchers | descriptor nombre/dominio/retorno |
| JSON global | duplicado en string e IO | builtins_serialization.go |
| Métodos nativos | nombres y retornos separados | Pendiente: NativeMethodDefinition |
| Métodos primitivos | analyzer y core/primitives | Pendiente: catálogo mínimo de firmas |
| Directiva @json | regex en linter y view | Pendiente: compilador compartido |
| VM | semántica experimental paralela | Exigir suite diferencial antes de integrarla |

### Arquitectura y refactor aplicado

El léxico permanece en parser, los tipos en typesystem, la semántica en analyzer y la ejecución/superficie nativa en core. No se crea un paquete language global: centralizar todos los dominios allí aumentaría acoplamiento y riesgo de ciclos.

1. parser/token.go define símbolos por maximal munch; lexer y formatter consumen la misma proyección.
2. parser_interpolation.go encapsula segmentación y composición AST sin API pública nueva.
3. Type.IsNumeric reemplaza la clasificación local del analyzer.
4. core/builtins.go declara una vez nombre, dominio y retorno; analyzer y runtime consumen el descriptor.
5. builtins_serialization.go elimina las dos implementaciones JSON.
6. server/rate_limiter.go y request_data.go aíslan estado y la frontera HTTP→Joss.

| Área | Antes | Después |
|---|---|---|
| Símbolos | 3 representaciones | 1 registro + proyecciones |
| Lexer | 445 líneas | 218 |
| Parser expressions | 1088 con interpolación | 958 + módulo cohesivo de 129 |
| Built-ins | lista + 6 sets + cascade | descriptor único + dispatch directo |
| JSON | 2 implementaciones | 1 |
| HTTP handler | 1042 | 925 + módulos cohesivos de 51 y 100 |
| Guardas | parciales | tests exhaustivos de símbolos y handlers |

El total de líneas no fue el objetivo: se añadieron pruebas y contratos. Ahora un símbolo cambia en una definición y un built-in en un descriptor más su handler de dominio.

### Riesgos restantes

- MainHandler aún contiene session/CSRF, WebSocket y response writing; requiere tests HTTP de caracterización antes de extraer.
- Native classes aún separa method names y precise returns; debe migrarse clase por clase.
- infer.go y analyzer.go siguen siendo riesgos semánticos y requieren tests de nominal resolution, refs y narrowing durante su división.
- Formatter necesita trivia; compartir símbolos es correcto, reemplazarlo por el lexer que descarta trivia no lo sería.
- pub_cli.go necesita caracterización de filesystem, red y lockfile antes de dividir.
- Los aliases built-in son compatibilidad pública; no deben eliminarse por duplicación superficial.
- VM y JPBC no deben promoverse sin tests diferenciales.

## Segunda fase de refactorización — septiembre de 2026

### Revalidación y orden

La auditoría se revalidó contra el árbol actual antes de modificarlo. Los siete P0 continuaban activos: `pub_cli.go` 1312 líneas, `infer.go` 1101, `analyzer.go` 964, `handler.go` 1024, `evaluator_infix.go` 835, `runtime.go` 762 y `evaluator_call.go` 601. La cobertura de referencia era analyzer 57.7%, core 47.9%, server 8.0% y cmd/joss 16.2%.

El orden elegido fue llamadas → operadores → inferencia. Llamadas tenía buena caracterización y una ruta semántica ya unificada; operadores podía congelarse con tests diferenciales por dominio; inferencia requería conservar la frontera typesystem/analyzer. Runtime lifecycle, HTTP y publicación permanecen después porque su estado externo exige fixtures específicos antes de extraer.

### Cambios aplicados

| Área | Estado anterior | Arquitectura resultante |
|---|---|---|
| Invocación | `evaluator_call.go` mezclaba evaluación de argumentos, binding, frames, callable kinds y built-ins | `call_arguments.go` posee evaluación/binding/ref; `call_method.go` posee frame, retorno y recursión; `callable_dispatch.go` posee tipos invocables; `evaluator_call.go` coordina resolución fuente y built-ins |
| Operadores | `evaluateInfix` mezclaba control, pipeline, números, streams, rangos, prefix/postfix y match; además contenía un segundo pipeline inalcanzable | `evaluator_control.go`, `evaluator_pipeline.go`, `evaluator_numeric.go`, `evaluator_stream.go` y `evaluator_update.go`; `evaluator_infix.go` conserva el orden de evaluación y short-circuit |
| Inferencia nominal | `infer.go` contenía assignability nominal, jerarquía y narrowing junto a inferencia base | `infer_nominal.go` posee jerarquía/assignability de proyecto; `infer_narrowing.go` posee refinamiento de scopes; `infer.go` conserva la entrada de inferencia y aún contiene call/member resolution pendiente |
| Métodos primitivos | nombres y retornos duplicados en analyzer; implementación enumerada aparte en core | `typesystem.PrimitiveMethodDefinition` es la metadata canónica; analyzer y runtime la consultan y un test exige implementación runtime para cada definición |
| Extensión | `js-yaml` 4.3.1 (alta) y `qs` 6.15.3 (moderada), ambas transitivas de herramientas de desarrollo | overrides compatibles a 4.3.2 y 6.16.0; `npm audit` queda en cero sin `--force` |

La divergencia `array.pop` era real: el analyzer publicaba retorno de elemento, pero el runtime de arrays no implementaba el método. Se retiró de la metadata compartida sin inventar una semántica mutable. Si se diseña posteriormente, deberá añadirse como una decisión explícita con implementación y tests.

### Caracterización y regresiones

- `call_characterization_test.go` congela named/positional/default binding, errores por faltantes/desconocidos y contratos de retorno estructurados. Las suites existentes cubren refs multinivel, closures, recursión, herencia y reciclaje de frames.
- `infix_characterization_test.go` congela pipeline, Elvis/ternario, match estricto, comparaciones, decimal exacto, rangos y postfix.
- `primitive_methods_architecture_test.go` impide anunciar metadata sin implementación runtime; `primitive_methods_test.go` valida retornos dependientes del receiver.
- La suite detectó durante la extracción una capacidad negativa en rangos ascendentes. Era una regresión del refactor, se corrigió antes de continuar y los contratos de documentación volvieron a pasar.

### Comparación y Change Surface

| Métrica | Antes | Después |
|---|---:|---:|
| `evaluator_call.go` | 601 líneas / 6 responsabilidades | 91 líneas coordinadoras + 3 módulos de 143/216/158 líneas |
| `evaluator_infix.go` | 835 líneas / 7 dominios | 120 líneas coordinadoras + 5 módulos por dominio de 58–269 líneas |
| `infer.go` | 1101 líneas / inferencia + nominal + narrowing | 907 líneas; nominal 86 y narrowing 65; call/member resolution sigue pendiente |
| Pipeline `|>` | 2 implementaciones, una muerta | 1 implementación canónica |
| Firmas primitivas | analyzer + switches runtime sin guarda común | 1 metadata + implementaciones runtime protegidas por test |

Superficie aproximada de cambio: añadir un método primitivo pasa de editar analyzer y runtime sin comprobación (2 lugares propensos a divergencia) a editar metadata e implementación (2 lugares necesarios) con proyección automática al analyzer y test de paridad. Añadir un operador aún exige parser/analyzer/runtime y, si aplica, VM; no se centralizó conducta que pertenece a capas distintas. Añadir un built-in continúa requiriendo descriptor y handler de dominio. Añadir una regla nominal del analyzer queda localizada en `infer_nominal.go`.

### Deuda reordenada

- **P0:** `analyzer.go`; call/member resolution restante en `infer.go`; lifecycle/pool en `runtime.go`; `MainHandler`; `pub_cli.go`. Ninguno se considera resuelto por tamaño.
- **P1:** metadata `NativeMethodDefinition` clase por clase; compilador compartido de directivas de vista; caracterización HTTP y pooling; parser statements/expressions restantes.
- **P2:** suite diferencial VM/intérprete antes de promover la VM; publicar aridad de métodos primitivos cuando exista un contrato fiable.

Regla para la siguiente iteración: no extraer lifecycle, HTTP ni registro/publicación hasta que sus tests congelen reset/reuse/fork, CORS/session/CSRF/WebSocket/status y cache/archive/lockfile respectivamente.

Cobertura focalizada posterior: analyzer 59.1%, core 48.2%, server 8.0% y cmd/joss 16.2%. La mejora es pequeña porque las pruebas nuevas priorizan contratos críticos; la cobertura baja de server/CLI confirma que no deben refactorizarse todavía sin caracterización adicional.

## Riesgos pendientes

- La recuperación del parser puede producir diagnósticos derivados después del primer token inválido, aunque ahora todos usan el modelo estructurado y conservan columna sin extraerla de mensajes.
- Los retornos nativos son explícitos, pero muchas APIs conservan parámetros variádicos hasta publicar contratos de aridad confiables.
- No existe refinamiento sensible a ramas, contratos de infraestructura, taint o escape formal. Los ciclos de módulos no aplican porque no existen módulos fuente.
- `pkg/core` sigue siendo amplio; dividir subsistemas de infraestructura requiere pruebas específicas y no se realizó sólo por estética.
- El catálogo de firmas ricas del LSP es metadato manual; la existencia de nombres sí proviene ya del catálogo generado.

## Tercera fase de arquitectura — septiembre de 2026

Esta fase priorizó estabilizar el analyzer y construir caracterización antes de extraer infraestructura.

### Analyzer

`Analyzer.Analyze` ahora expresa el pipeline `collectDeclarations → projectScope → validateNominalContracts → analyzeSourceBodies → diagnostics`. La fachada conserva el estado de proyecto; la recolección de declaraciones, contratos nominales, análisis de cuerpos, resolución de llamadas y resolución de miembros viven en archivos del mismo package (`declaration_collection.go`, `nominal_contracts.go`, `body_analysis.go`, `call_resolution.go`, `member_resolution.go`). No se crearon managers públicos ni se introdujo una dependencia hacia `core`. El cursor de archivo/clase/retorno se restaura por callable, evitando contaminación entre unidades.

Las llamadas pasan por una firma semántica de `Callable` y una única validación de aridad, nombres, referencias y compatibilidad; los métodos globales, nativos, primitivos y de plugin se proyectan a esa representación sin compartir implementación runtime. La resolución de miembros distingue receiver, visibilidad, jerarquía, campos y métodos. La representación común es metadata: analyzer y runtime no comparten estado de ejecución.

### Caracterización previa a infraestructura

Se añadieron pruebas de lifecycle y pool (`runtime_lifecycle_characterization_test.go`) que fijan creación, fork, reset y reuse. El test reveló y corrigió contaminación real de `Env`, cachés de metadata, fuente actual, generadores y defers en `Runtime.Free`; `DB` permanece recurso compartido externo y no se cierra desde `Free`. El estado queda clasificado como configuración persistente, por-runtime, por-ejecución o caché reinicializable.

La frontera HTTP tiene caracterización de runtime ausente, 503, CORS, sesiones/CSRF y mapeo de una respuesta de Joss (`handler_characterization_test.go`). WebSocket y casos de respuesta no soportados siguen explícitamente pendientes para no inventar semántica. CLI/pub incorpora fixtures temporales y servidor registry en memoria para extracción ZIP segura, rechazo de traversal, lockfile determinista, manifiestos y resolución de errores (`cmd/joss/pub_cli_test.go`). Esto permite refactorizar después sin confundir cambios de infraestructura con cambios de lenguaje.

### Directivas y metadata

El inventario confirma que `@json` es una transformación de vistas aislada junto con `@extends`/`@section`; linter y view todavía interpretan por caminos distintos. Se mantiene como deuda P1: primero debe definirse una representación de directiva, no compartirse una regex. `NativeMethodDefinition` también queda P1: las clases conservan nombres y retornos separados y no se publican aridades inventadas para APIs variádicas.

### Change Surface y métricas

| Cambio | Antes | Estado fase 3 | Protección |
|---|---:|---:|---|
| regla de analyzer | `infer.go` + estado implícito | fase explícita y archivos por concepto | `semantic_pipeline_test.go` |
| lifecycle/reset runtime | sin contrato de pool completo | reset de estado por ejecución centralizado | `runtime_lifecycle_characterization_test.go` |
| respuesta HTTP | handler monolítico sin tabla | frontera caracterizada, extracción diferida | `handler_characterization_test.go` |
| publicación/ZIP/lock | efectos externos sin fixtures | fixtures temporales y registry `httptest` | `pub_cli_test.go` |

Fan-in/out se sigue observando a nivel de package: `analyzer` consume parser/typesystem/diagnostics pero no core; `core` mantiene el mayor fan-out por integrar lenguaje y host; server y cmd dependen de core/adaptadores. No se usan estos números como gates. La duplicación semántica restante es intencional (validación estática versus ejecución) y comparte tipos/metadata, no código de efectos.

### Deuda restante

- **P0:** `MainHandler` y `Runtime` aún coordinan varios dominios; deben extraerse sólo después de ampliar WebSocket, response mapping y contaminación concurrente.
- **P1:** `NativeMethodDefinition`, firmas de primitivas completas, compilador de directivas de templates, normalización única de argumentos nombrados/ref y métricas automatizadas de fan-in/change surface.
- **P2:** suite diferencial Interpreter/VM, fuzzing selectivo de ZIP/lock/template y benchmarks comparables de inferencia/calls.

## Cuarta fase de arquitectura — septiembre de 2026

### Revalidación y entorno reproducible

Los P0 vigentes eran lifecycle/fork de `Runtime`, `MainHandler` y `pub_cli.go`; los P1 eran metadata nativa, argumentos y directivas. La línea base focalizada era analyzer 60.3%, core 48.3%, server 29.7% y cmd/joss 20.4% después de esta fase. El fallo de `pluginpkg` no era del producto: su test escribía en el HOME real; ahora redefine `USERPROFILE`/`HOME` a `t.TempDir()`. La caché global de Go posee ACLs defectuosas en este host; el workaround reproducible es asignar `GOCACHE` a un directorio temporal. `npm ci` dentro del checkout colisiona con archivos abiertos por VS Code; una copia temporal sin `node_modules`, con caché/red habilitada, ejecutó `npm ci`, `npm run compile` y `npm audit` (0 vulnerabilidades).

### Modelo de estado de Runtime

| Campo | Responsabilidad | Lifetime | Fork | Free | Compartido / owner |
|---|---|---|---|---|---|
| `Env` | configuración efectiva | runtime configuration | copia | limpia | runtime |
| `Variables` | bindings y globals | per-execution | copia con clones conocidos | limpia + bindings estándar | runtime/frame |
| `VarTypes` | tipos runtime | per-execution | copia | limpia | runtime |
| `Constants` | constancia | per-execution | copia | limpia | runtime |
| `HostGlobals` | visibilidad host | runtime lifecycle | copia | reconstruye | runtime |
| `Classes` | clases nativas/proyecto | runtime configuration | copia de mapa, AST inmutable compartido | limpia | runtime/proyecto |
| `Interfaces` | contratos de proyecto | runtime configuration | copia de mapa | limpia | runtime/proyecto |
| `Enums` | enums de proyecto | runtime configuration | copia de mapa | limpia | runtime/proyecto |
| `Functions` | funciones de proyecto | runtime configuration | copia de mapa, AST compartido | limpia | runtime/proyecto |
| `DB` | pool SQL | external resource | comparte | sobrevive, no se cierra | aplicación/`database/sql` thread-safe |
| `Routes` | tabla HTTP | runtime configuration | copia en dos niveles | limpia | runtime |
| `CurrentMiddleware` | pila al registrar rutas | per-execution | reinicia | limpia | runtime |
| `CustomMiddlewares` | callables middleware | runtime configuration | copia | limpia | runtime |
| `NativeHandlers` | implementación nativa | global immutable tras registro | copia | limpia/reconstruye al adquirir | core |
| `NativePlugins` | payloads plugin | runtime configuration | copia de mapa; definiciones tratadas como inmutables | limpia | runtime/plugin loader |
| `NativeDrivers` | handles nativos | external resource | comparte definiciones/handles | limpia referencia, no descarga handle | loader |
| `PluginRegistry` | símbolos/instancias plugin | runtime lifecycle | comparte explícitamente con padre | elimina referencia | runtime padre/pluginruntime |
| `ProjectRoot` | raíz del proyecto | runtime configuration | copia | limpia | runtime |
| `SEO` | acumulador de response | per-request | reinicia | limpia | request runtime |
| `SitemapEntries` | configuración sitemap | runtime configuration | copia slice | limpia | runtime |
| `SitemapProviders` | closures sitemap | runtime configuration | copia slice; closures compartidas | limpia | runtime |
| `SitemapExclusions` | configuración sitemap | runtime configuration | copia slice | limpia | runtime |
| `CurrentSource` | cursor fuente | per-execution | reinicia | limpia | evaluator |
| `CurrentFile` | cursor archivo | per-execution | reinicia | limpia | evaluator |
| `MaxCallDepth` | límite | runtime configuration | copia | default | runtime |
| `callDepth` | profundidad actual | per-execution | reinicia | limpia | evaluator |
| `currentClass` | cursor nominal | per-execution | reinicia | limpia | evaluator |
| `callStack` | stack diagnóstico | per-execution | reinicia | limpia | evaluator |
| `callablePlans` | planes de métodos | cache | copia de mapa; planes inmutables | limpia | runtime |
| `functionPlans` | planes de closures | cache | copia de mapa | limpia | runtime |
| `classMetadataCache` | metadata derivada | cache | reinicia | limpia | runtime; protegido por `planMu` |
| `currentFrame` | frame activo | per-execution | reinicia | limpia | evaluator |
| `planMu` | protección de caches | runtime lifecycle | nuevo mutex | permanece | runtime |
| `captureEnvironment` | captura temporal | per-execution | reinicia | limpia | evaluator |
| `cinReader`, `cinTokens` | entrada tokenizada | per-execution | reinicia | limpia | runtime IO |
| `currentGenerator`, `generatorIndex` | cursor generator | per-execution | reinicia | limpia | evaluator |
| `topDefers` | defers top-level | per-execution | reinicia | limpia | evaluator |

Invariantes: después de `Free` no sobreviven request, ejecución, cursores ni caches; los bindings host se reconstruyen; `DB` no se cierra porque `Runtime` no es su owner. `Fork` comparte sólo AST/planes/configuración tratada como inmutable, registry de plugins y recursos externos; los mapas mutables de request se aíslan. Las pruebas detectaron que `Fork` omitía `Enums`; se corrigió. El pool soporta acquire/free concurrente bajo race detector.

`runtime_lifecycle.go` contiene construcción, pool, adquisición, reset e invariantes. `runtime.go` conserva environment/loading y registro de declaraciones. `Runtime` sigue siendo coordinador; no se introdujeron managers.

### Lifecycle HTTP y response mapping

El runtime de request se libera mediante un único `defer` inmediatamente después de `Fork`, incluyendo rate limiting, CORS, session/storage, panic y WebSocket. `response_writer.go` es la frontera Joss→HTTP.

| Resultado | Status/Content-Type | Conducta |
|---|---|---|
| `string` | 200, HTML por defecto | escribe texto y hot reload HTML |
| map/Instance `JSON` | `status_code` o 200, JSON | encode de `data` |
| map/Instance `RAW` | `status_code` o 200, configurable | string/bytes/representación textual + headers |
| Instance `FILE` | 200, MIME detectado | attachment o 500 si no se lee |
| Instance `STREAM` | 200, SSE | flush y callback con Stream |
| map/Instance `REDIRECT` | 302/map o `status_code`/Instance | Location; Instance aplica cookies/flash |
| int/float/bool/null/array/map sin `_type` | sin representación publicada | continúa a public-file/404 |

La caracterización corrigió un bug: `Response::json(..., status)` almacena `status_code`, pero el handler leía `status`, devolviendo 200. WebSocket cubre upgrade inválido y upgrade/cierre válido sin ruta; message callbacks completos continúan P1.

### Metadata y firmas

`NativeMethodDefinition` separa nombre, retorno, parámetros opcionales, `ArityKnown` y variadicidad de la implementación `NativeHandler`. Stack, Queue y Math son la primera migración; sus nombres/retornos se proyectan al AST runtime, analyzer, catálogo LSP y documentación. La aridad permanece desconocida cuando el handler tolera entradas dinámicas. Tests de paridad impiden anunciar métodos sin handler. Las demás clases conservan el adaptador legacy hasta migrar clase por clase.

La normalización estática crea una única relación parámetro→argumento para positional, named, defaults, ref, unknown, duplicate y missing. Runtime conserva su binder, pero ambos consumen los mismos parámetros semánticos y rechazan duplicados; analyzer no ejecuta código runtime.

### Templates, CLI y semántica diferencial

El inventario de vistas incluye `@extends`, `@section`/`@endsection`, `@yield`, `@include`, `@json` y `@foreach`/`@endforeach`. `pkg/viewtemplate` aporta scanner mínimo con rangos, comillas y paréntesis anidados. Runtime y linter consumen `RewriteJSON`; ya no mantienen regex distintas. Herencia/sections/includes/foreach conservan sus consumidores actuales hasta migración progresiva. Existe fuzz target para determinismo/no-panic/rangos.

CLI/pub usa un cliente HTTP interno con timeout, cache configurable para tests, registry `httptest`, ZIP seguro y temporales. Se añadieron 401/403/429/500, JSON truncado/inválido, timeout, cache hit/stale y ausencia de parciales. El fuzz target de ZIP verifica que no se escriba fuera del destino.

El primer corpus diferencial declara únicamente integer arithmetic/comparison, local assignment e integer prefix. Analyzer debe aceptar, Interpreter y VM deben producir el mismo valor. VM sigue experimental y cualquier nueva feature debe incorporarse explícitamente al inventario, no inferirse como soportada.

| Feature | Analyzer | Interpreter | LSP | VM | Estado |
|---|---|---|---|---|---|
| enteros/aritmética básica | canónico de tipos | referencia publicada | catálogo de sintaxis | diferencial verde | soportada en corpus |
| llamadas/métodos/closures | firmas semánticas | referencia publicada | proyección parcial | no comparable | pendiente VM |
| nativos | proyección metadata | handlers | catálogo generado | no soportado | metadata progresiva |
| templates | valida script compilado | renderiza | snippets/diagnostics | no aplica | scanner común parcial |
| match/pipeline/references | analyzer activo | referencia publicada | sintaxis | no paridad completa | P2 diferencial |

Guardas negativas verifican que parser/typesystem/diagnostics/analyzer no importen core. El catálogo JSON generado contiene `_generated: DO NOT EDIT` y sigue validado por `cataloggen --check`.

### Performance baseline

Windows/amd64 i5-10300H, `-benchtime=100ms`: llamada simple 794 ns/op, nested 2936 ns/op, ref 1496 ns/op, closure 973 ns/op; frame recycling 1213 ns/op, 24 B/op, 2 allocs/op. Analyzer del fixture startup: 10095 ns/op, 7315 B/op, 69 allocs/op. Lifecycle: construct 235536 ns/op, pooled acquire/free 289634 ns/op y fork/free 26039 ns/op. El pool incluye recarga/autoload; no se optimizó sin perfilar esa responsabilidad.

### Deuda reordenada (Cuarta fase)

- **P0:** completar aislamiento/ownership de plugin registries y drivers antes de concurrencia mutable; completar callbacks WebSocket y errores de session backend.
- **P1:** migrar NativeMethodDefinition clase por clase; extraer session/CSRF de MainHandler; separar registry/cache/manifest/publisher de pub CLI; migrar el resto de directivas al scanner.
- **P2:** ampliar corpus Analyzer↔Interpreter↔VM, catálogo de firmas LSP, fuzz de type parser y lockfile round-trip.
- **P3:** automatizar fan-in/change surface y establecer benchmarks históricos comparables en CI.

---

## Quinta fase de arquitectura — septiembre de 2026

La quinta fase aborda y resuelve todas las deudas críticas P0 pendientes de la cuarta fase, enfocándose en la consistencia de ownership, concurrencia segura, contratos de sesión y ciclo de vida de plugins y WebSockets, además de avanzar la metadata canónica y el tooling de plantillas.

### 1. Ownership y Concurrencia de Plugins y Drivers Nativos (P0-A)

- **Aislamiento por Runtime (`PluginAwareHost`)**: Se introdujo la interfaz `PluginAwareHost` en `pkg/pluginruntime` para desacoplar el registro global de paquetes de la ejecución concreta. `Runtime` implementa esta interfaz manteniendo sus propios motores de AST (`pluginASTEngines map[string]*PluginASTEngine`) y namespaces (`PluginNamespace`).
- **Semántica de Fork**: Al ejecutar `Runtime.Fork()`, las fachadas de motor se duplican vinculadas al nuevo runtime hijo, compartiendo de forma segura el AST inmutable del plugin mientras aíslan totalmente el estado evaluado y los frames locales. Dos plugins distintos con funciones idénticas (p. ej. `run()`) ya no colisionan entre sí ni cruzan ámbitos de request.
- **Descarga atómica de drivers (`NativeDriverDefinition.Unload()`)**: Los drivers nativos dinámicos (`.dll`/`.so`/`.dylib`) incorporan un método `Unload()` seguro y protegido por `driverMu`. A través de `unloadNativeDriverHandle` (`FreeLibrary` en Windows, `dlclose` en Unix), se garantiza la liberación idempotente de recursos y la prevención de fallos de segmentación ante llamadas concurrentes o posteriores a la descarga.

| Componente | Nivel de Compartición | Ciclo de Vida | Política Concurrente |
|---|---|---|---|
| AST de Plugin (`*parser.Program`) | Inmutable / Compartido | Proceso | Read-only thread-safe |
| `PluginASTEngine` | Por `Runtime` / Instancia | Request / Fork | Sin contención entre hilos |
| `PluginNamespace` | Por `Runtime` / Instancia | Request / Fork | Instancias clonadas en `Fork()` |
| `NativeDriverDefinition.handle` | Puntero a SO | Carga hasta `Unload()` | Protegido por `driverMu` |

### 2. Ciclo de Vida Completo y Callbacks WebSocket (P0-B)

- **Ejecución aislada de callbacks**: Las conexiones WebSocket ejecutan callbacks definidos en código Joss (`onConnect`, `onMessage`, `onClose`, `onError`) en un runtime forkeado independiente.
- **Propagación de parámetros**: Los parámetros de ruta (`$params`) extraídos durante el handshake HTTP se inyectan correctamente en el contexto del callback.
- **Aislamiento multi-conexión**: Conexiones concurrentes operan sobre sockets y runtimes independientes, sin filtración de frames ni de estado léxico entre clientes.

### 3. Contratos de Almacenamiento de Sesión y Redirect Flash (P0-C)

- **Persistencia unificada**: La serialización de flash en redirecciones HTTP (`persistRedirectFlash` en `response_writer.go`) se unificó bajo el contrato canónico `saveSession(sessionID, store)`.
- **Manejo estricto de errores**: Ante un fallo en el backend de sesiones durante una redirección, la respuesta aborta inmediatamente con HTTP 500 y un mensaje de error explícito, suprimiendo la cabecera `Location` para impedir que el cliente siga una redirección con datos de sesión o flash corruptos o no persistidos.

### 4. Protección contra Doble Liberación (`sync.Pool`)

- **Defensa ante double-free**: Se identificó y resolvió una potencial condición de carrera en pruebas y requests por llamadas duplicadas a `Free()` sobre un mismo `*Runtime`. Un campo booleano `freed` protegido por `poolMu` garantiza que la devolución al pool sea idempotente y que un puntero no reingrese múltiples veces al pool concurrente.

### 5. Expansión de Metadata Nativa (`NativeMethodDefinition`)

Se completó la migración de un lote extendido de 10 clases nativas canónicas a `NativeMethodDefinition`, distinguiendo firmas canónicas, nombres tipados y aridad exacta:
- `Stack`: `push`, `pop`, `peek`, `isEmpty`, `clear`, `count`, `toArray`
- `Queue`: `push`, `pop`, `peek`, `isEmpty`, `clear`, `count`, `toArray`
- `Math`: `abs`, `sqrt`, `pow`, `round`, `floor`, `ceil`, `min`, `max`, `random`, `sin`, `cos`, `tan`, `log`, `exp`
- `JSON`: `encode`, `decode`, `valid`, `prettify`
- `Markdown`: `toHtml`, `toHtmlSafe`, `toc`, `meta`
- `Str`: `length`, `lower`, `upper`, `contains`, `startsWith`, `endsWith`, `replace`, `split`, `trim`, `substr`, `indexOf`, `pad`, `repeat`
- `UUID`: `v4`, `v7`, `isValid`
- `Lang`: `type`, `isNumeric`, `isCallable`, `isIterable`, `methods`, `properties`, `clone`
- `Console`: `log`, `info`, `warn`, `error`, `debug`, `table`, `trace`, `clear`, `time`, `timeEnd`, `assert`
- `Zip`: `extract`

Las definiciones son consumidas por el analyzer, el generador de catálogos (`tools/cataloggen`), el generador de documentación (`tools/docgen`) y el servidor de lenguaje (LSP).

### 6. View Templates y Conteo de Columnas por Runas UTF-8

- **Rune-aware source positions**: En `pkg/viewtemplate/directives.go`, el cálculo de columnas para diagnósticos se ajustó para iterar por runas UTF-8 (`utf8.RuneCountInString`), corrigiendo el desplazamiento en caracteres acentuados o multibyte y garantizando alineación exacta de rangos con el parser y LSP.
- **Fuzz testing**: Fuzzing continuo sobre el extractor de ZIP (`FuzzExtractPluginZip`) y el escáner de directivas (`FuzzDirectiveScanner`).

### Deuda reordenada (Quinta fase)

- **P0:** Ninguna. Todas las inconsistencias de ownership, drivers nativos, sesiones y WebSocket han sido resueltas y verificadas bajo characterization tests y race detector.
- **P1:** Continuar migración progresiva de las clases nativas restantes (`GranDB`, `Crypto`, `File`, `Http`, `Router`, etc.) a `NativeMethodDefinition`; extraer session/CSRF de `MainHandler` a submódulos dedicados; migrar directivas restantes (`@extends`, `@section`, `@yield`, `@include`, `@foreach`) al scanner unificado de `pkg/viewtemplate`.
- **P2:** Ampliar corpus diferencial Analyzer↔Interpreter↔VM para tipos complejos y closures; ampliar fuzzing de parser de tipos y round-trip de lockfile.
- **P3:** Automatizar métricas de fan-in/fan-out y change surface en CI; establecer benchmarks comparables de performance.

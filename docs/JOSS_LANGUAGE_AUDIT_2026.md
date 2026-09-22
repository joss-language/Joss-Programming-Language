# Auditoría integral del lenguaje Joss — septiembre de 2026

[Índice](README.md) · [Arquitectura](ARQUITECTURA.md) · [Tipos](SISTEMA_TIPOS.md) · [Diagnósticos](DIAGNOSTICOS.md)

**Alcance y método.** Revisión del árbol actual del repositorio, no de la tesis ni de auditorías históricas como autoridad. Se contrastaron parser, AST, analyzer, typesystem, runtime, servidor, CLI, VM, bytecode, plugins, móvil, herramientas, pruebas, benchmarks, documentación y el proyecto web de ejemplo. `go test ./...` terminó con código 0 durante esta auditoría. Las observaciones que dependen de una ruta de código se indican como tales; no se atribuyen mediciones de rendimiento ni explotación de seguridad sin una prueba específica. Este informe es una evaluación y un plan: **no implementa las mejoras**.

## 1. Executive Summary

Joss ya cumple una parte importante del objetivo «analizar → validar → ejecutar»: el CLI analiza el proyecto antes de correrlo, el analyzer tiene símbolos y scopes, tipos de parámetro obligatorios, contratos nominales, chequeos de llamadas y miembros conocidos, retorno declarado, diagnósticos estructurados y detección de ciertas operaciones aritméticas constantes. La ejecución publicada sigue siendo un intérprete de AST en Go con planes de slots por callable. `JOSSBC2Z` empaqueta AST comprimido y la VM es experimental; ninguno es una fase general de compilación semántica.

Las mayores brechas actuales están en **ciclo de vida concurrente y recursos**, **contratos de APIs nativas y colecciones**, **flujo sensible a estados**, y **discrepancia entre rutas de entrada**. La ruta móvil implementa un timeout que informa fallo pero no detiene la goroutine; las rutas `async`, `Task` y finalización de instancias crean forks sin liberar su ownership; `GranDB::transaction` abre un `sql.Tx` que las consultas normales del callback no usan. Los canales delegan operaciones inválidas a panics de Go. Estos son hallazgos de código actual, no propuestas estéticas.

**Juicio:** la base del lenguaje es prometedora y modular en las capas de análisis, pero todavía no ofrece garantías equivalentes a un lenguaje compilado para programas con `mixed`, nativos, recursos externos o concurrencia. P0 debe priorizar fallos de lifecycle/atomicidad y pruebas de regresión; luego ampliar contratos estáticos. La mejora de rendimiento sólo debe avanzar después de perfilar cargas representativas.

## 2. Current Architecture

```text
.joss → parser.Lexer → parser Pratt → parser.Program / AST
                                  ├→ analyzer.LoadProject (entrypoint + app/**/*.joss)
                                  │    → collectDeclarations → projectScope
                                  │    → validateNominalContracts → analyzeSourceBodies
                                  │    → diagnostics.Diagnostic
                                  ├→ bytecode.Encode → JOSSBC2Z → runner Go → intérprete
                                  └→ core.Runtime.Execute → registro de clases/funciones
                                       → runtime/plan.Callable → runtime/frame.Slot
                                       → evaluator/executor → nativos/host/servidor
                       VM experimental ← subconjunto AST (sin ruta CLI principal)
             plugins JP ← SymbolIndex + AST/JPBC propios
```

**Fronteras y dependencias.** `pkg/parser` define tokens, lexer, precedencias, AST y parser; `pkg/typesystem` define nombres, asignabilidad e integer checks; `pkg/analyzer` importa esas capas y `pkg/diagnostics`, pero no `pkg/core`. `pkg/core/analyzer.go` proyecta built-ins, clases nativas y plugins al entorno semántico. `pkg/core` ejecuta AST e integra SQL, archivos, red, vistas, auth, WebSocket y plugins. `pkg/server` adapta HTTP a runtimes forkeados. `cmd/joss` orquesta el proyecto; `cmd/runner` consume builds. `pkg/mobile` ofrece API embebida y exportación C; `sdk/dart` adapta esa superficie y descarga binarios de release. No hay un paquete separado llamado `libjoss` en este árbol. `vscode-joss` ofrece LSP; `pkg/formatter`, `pkg/linter`, `pkg/fixer` y `pkg/tester` son herramientas separadas. `pkg/plugincompiler`, `pkg/pluginpkg`, `pkg/pluginruntime` y `pkg/vfs` forman la frontera de plugins/paquetes. `pkg/i18n`, `pkg/template`, `pkg/viewtemplate` y `pkg/crypto` apoyan superficies de aplicación.

**Decisiones acertadas ya aplicadas.** Catálogos canónicos de tokens, built-ins y métodos primitivos; firmas semánticas separadas de ejecución; frames aislados para funciones con nombre; closures con captura léxica; `ref` temporal e invariante; `Free` idempotente con `atomic.Bool`; ownership contado para drivers; snapshots de sesión; escáner compartido de directivas; metadatos de clase cacheados. Véanse [Arquitectura](ARQUITECTURA.md) y `pkg/core/runtime_lifecycle.go`, `pkg/core/call_arguments.go`, `pkg/analyzer/analyzer.go`.

**Acoplamiento restante.** `core.Runtime` reúne lenguaje y servicios del host (`pkg/core/types.go`); `cmd/joss/main.go` y `pkg/server/handler.go` orquestan muchas rutas; `core/evaluator_member.go` conserva búsquedas dinámicas por nombres y scans de plugins. Es acoplamiento funcional real, pero dividir archivos por tamaño no sería una solución. Los contratos duplicados intencionalmente (analyzer vs runtime) deben compartir metadata, no estado. La frontera de análisis termina al producir diagnósticos: el AST no se convierte en un typed IR reutilizado por todas las rutas. `runtime/plan` se calcula en la ejecución de callables, no certifica el programa entero.

## 3. Language Pipeline

La sintaxis fuente real exige `$` para variables y visibilidad explícita en funciones/clases globales. Por tanto `func suma(int a, int b)` y `class Usuario` del enunciado son ejemplos conceptuales, no Joss válido: serían `public func suma(int $a, int $b): int { return $a + $b }` y `public class Usuario { public string $nombre }`. El bucle es `foreach ($items as $item) { ... }`; `await($futuro)` es llamada nativa, no operador prefijo; `async { ... }` crea el futuro.

| Fuente | Tokens / AST principal | Analyzer y tipo conocido | Ejecución y resolución pendiente |
|---|---|---|---|
| `int $edad = 20` | tipo, variable, asignación, entero → `LetStatement(IntegerLiteral)` | tipo declarado `int`, compatibilidad del inicializador | guarda binding tipado, `int64`; runtime vuelve a validar |
| `var $nombre = "Joss"` | `VAR`, literal string → `LetStatement` | infiere y fija `string` | slot/map con tipo inferido; revalidación al reasignar |
| `mixed $valor = obtenerValor()` | declaración + `CallExpression` | `mixed` explícito; firma de llamada si resoluble | busca función/callable y valor efectivo en runtime |
| `public func suma(int $a, int $b): int` | `MethodStatement`, `ReturnStatement(InfixExpression)` | parámetros, aridad, `+`, tipo de retorno y rutas de salida | planifica slots; resuelve llamada, frame y operación |
| `public class Usuario { public string $nombre }` | `ClassStatement` + propiedad | contratos nominales, miembros y visibilidad conocida | metadata cacheada; instancia usa `Fields map[string]interface{}` |
| `await($f)` | `CallExpression` | built-in conocido; resultado frecuentemente `mixed` | `Future.Wait()`, puede bloquear o propagar error |
| `foreach ($items as $item)` | `ForeachStatement` | iterable se infiere; elemento se vuelve `unknown` en el binding | ejecuta iteración de array/map/channel/generator, según valor |

Los tokens precisos y la precedencia vienen de `pkg/parser/token.go`, `lexer.go` y `parser.go`; la tabla resume categorías, no una traza de `NextToken` instrumentada. El AST retiene tokens/posiciones y tipos escritos; no retiene un `SymbolID` ni una referencia semántica universal a cada llamada. `runtime/plan/callable.go` agrega `IdentifierSlots` y `NameSlots` para parámetros/locales, pero globals, propiedades, métodos, plugins y nativos continúan con búsquedas por nombre.

En clases/métodos: el analyzer recoge declaraciones antes de cuerpos, valida herencia/interfaces y visibilidad, y tipa llamadas cuando conoce receiver/firma. El runtime aún comprueba constructor, propiedad, acceso y tipo del valor efectivo. En closures: `FunctionLiteral` captura mapas y slots actuales; al invocar usa un frame planificado y un mutex del entorno capturado. En `try/catch`, `throw`, `defer`, generadores, `select`, `match` y `async`, la sintaxis existe, pero el análisis de estados/efectos es parcial. Para `null`, `T?` normaliza a unión; narrowing se aplica en `is`, comparaciones con null y algunos `guard`/ternarios (`pkg/analyzer/infer_narrowing.go`).

## 4. Type System

| Tipo/concepto | Garantía real | Límite |
|---|---|---|
| `int` | `int64`; sumas/restas/productos y negación protegidos contra overflow; división cero protegida | valores de nativos o `mixed` sólo se prueban al ejecutar |
| `float` | `float64`; admite asignación desde `int` | enteros mayores que 2^53 no se representan todos exactamente; revisar NaN/Inf por operación específica |
| `decimal` | `shopspring/decimal`; admite promoción int/float; literal decimal | convertir `float` puede importar su redondeo previo |
| `string`, `bool` | tipos canónicos; strings UTF-8 y operaciones Unicode específicas | coerción de string a int/float/decimal/bool existe al asignar a tipo explícito; la regla debe seguir siendo explícita en diagnósticos |
| `array<T>`, `map<K,V>` | tipo de elemento/valor verificable al declarar y validar valores completos; maps runtime usan clave string | no son genéricos universales; funciones nativas y mutaciones pueden devolver `unknown/mixed`; no hay prueba de aliasing/varianza segura |
| `object`, clases, interfaces | tipo nominal, herencia e interfaces del proyecto; members conocidos se chequean | campos son maps, no offsets; el receiver `mixed` requiere lookup dinámico |
| `channel` | identidad de canal | sin tipo de mensaje ni estado abierto/cerrado estático |
| `mixed` | dinamismo deliberado | acepta cualquier fuente/destino en `Assignable`; desplaza errores al runtime |
| `var` / primera asignación | infiere el primer tipo concreto y lo fija; null difiere la inferencia | flujo condicional y llamadas de retorno desconocido reducen precisión |
| `const` | impide reasignación; propiedades constantes protegidas | no convierte estructuras referenciadas en profundamente inmutables |
| `T|null`, `T?` | unión nullable normalizada; chequeo de asignación y narrowing local | no hay análisis general de null-state interprocedural |

`typesystem.Assignable` acepta `Unknown` de manera permisiva para evitar falsos positivos (`pkg/typesystem/types.go`); por tanto «análisis limpio» no significa «sin error de tipos posible». `int $edad = "hola"` da incompatibilidad si la cadena no es coercible; `int $edad = "20"` puede aceptarse mediante `CoerceString`. `var $contador = 10; $contador = "texto"` se rechaza; `mixed` es la opción dinámica. `ref T` es invariante y no escapa. La compatibilidad nominal se complementa en `pkg/analyzer/infer_nominal.go` y en `core.checkParsedType`; hay dos defensas necesarias, pero conviene probar su concordancia con un corpus común.

**Hallazgo de consistencia:** `Assignable` trata `array<T>` y `map<K,V>` de forma covariante y acepta colecciones sin argumento de tipo como destino/fuente. Con contenedores mutables/aliasados esto puede admitir una asignación que luego permite introducir elementos incompatibles. La extensión real de la exposición depende de cada ruta de mutación: es un riesgo de soundness que exige regresiones de aliasing antes de cambiar la regla. `runtimeTypeOf` de un array/map pierde los argumentos genéricos; `checkParsedType` recorre elementos para una variable tipada, pero una operación nativa posterior puede perder esa prueba.

**Precisión numérica:** `typesystem.Assignable` documenta `int → float` como «losslessly», pero un `float64` no puede representar cada `int64` (por ejemplo, 2^53+1). La compatibilidad está implementada, no la garantía de precisión del comentario. Esto merece un test de valor y una política explícita: warning por conversión potencialmente inexacta, cast expreso o conservación del comportamiento documentando pérdida. `decimal` preserva precisión decimal al recibir un entero; convertir desde `float` no restaura dígitos ya perdidos.

## 5. Static Safety

El analyzer detecta símbolos y clases inexistentes, duplicados, visibilidad, tipos fuente desconocidos, inicializadores/reasignaciones/argumentos/retornos incompatibles, aridad y nombres de argumentos conocidos, referencias ilegales, miembros conocidos inexistentes, índices con tipo inválido, operaciones conocidas inválidas, overflow y división por cero constantes, retornos demostrablemente ausentes y código después de una salida incondicional. Emite códigos `JOSS-...` desde `pkg/analyzer`, apoyado por `pkg/diagnostics`. `pkg/analyzer/flow.go` es deliberadamente conservador: no construye un CFG general y `hasYield` considera generador una salida válida.

| Error / decisión | Fase actual | Movimiento razonable | Fallback |
|---|---|---|---|
| sintaxis, visibilidad obligatoria | parser | ya temprano | no ejecutar |
| variable/función/clase desconocida | analyzer cuando resoluble | cerrar diferencias de entrypoint y plugin | runtime para carga dinámica |
| tipo incompatible / retorno / `ref` | analyzer | profundizar aliasing y flujo | runtime obligatorio |
| llamada/miembro con `mixed` o nativo sin firma | runtime | publicar firmas verificadas y narrowing | runtime |
| uso antes de inicialización | parcial; runtime slot `Initialized` | definite assignment por CFG | runtime |
| null dereference | narrowing parcial / runtime | análisis de estado y `T?` | runtime |
| división cero / índice fuera de rango | constantes: analyzer; variables: runtime | propagación de constantes de bajo costo | runtime |
| canal cerrado / send/recv bloqueante | runtime/Go panic | estado sólo si local y demostrable | runtime estructurado |
| transacción no ligada / recurso no cerrado | runtime/host | efectos y ownership de APIs | runtime y pruebas de integración |

La regla clave es conservar defensas runtime: cargas dinámicas, `mixed`, I/O y concurrencia no son decidibles en general antes de ejecutar.

## 6. Runtime Safety

Los errores de Joss usan `pkg/runtime/errors` y `pkg/core/errors.go`; aritmética e índices tienen códigos estables. `MaxCallDepth` limita recursión a 1024 frames por defecto. El runtime valida tipos de slots, campos, parámetros, retornos, visibilidad y `ref`; el pool se limpia en `Free`. Los fallos de Go originados en APIs nativas no siempre se convierten a `JossError`: `close(ch.Ch)` duplicado y `send` a canal cerrado pueden hacer panic; `make_chan` con capacidad negativa también. Esto da una experiencia distinta a las fallas aritméticas/indexación estructuradas. `await` y `recv` pueden bloquear indefinidamente si no hay productor; no existe cancelación estructurada general.

Clasificación de controles: parser para construcción imposible; analyzer/typesystem para incompatibilidad, flujo y null demostrables; planner para referencias y metadata estables; runtime para estado de canal, límites, recursos y valores externos; stdlib para contratos de red/FS/SQL; tooling para diagnósticos, rastreo y auditoría de dependencias. FFI y plugins con permiso de host no constituyen sandbox del SO: la firma del paquete verifica procedencia/integridad, no aísla efectos. Cualquier propuesta de permisos debe distinguir capacidades del host y aislamiento real.

## 7. Memory Model

Los valores se almacenan en `interface{}` y estructuras Go: arrays `[]interface{}`, maps `map[string]interface{}`, instancias con `Fields map`, strings Go y decimal. El GC de Go gestiona memoria ordinaria. `executionFrame` usa slots tipados etiquetados y `sync.Pool`; `Runtime` también usa pool y `Free` limpia caches, globals, estado de request, generadores, defers y plugins. `Fork` copia tablas, comparte AST/planes y `*sql.DB`; clona ciertas instancias/mapas/slices sólo superficialmente (`pkg/core/runtime.go`). Una estructura anidada mutable puede seguir aliasada entre forks si se introduce por una ruta no cubierta; exigir test específico antes de afirmar aislamiento profundo.

`CapturedFunction` guarda un snapshot léxico en mapas protegidos por mutex, con la semántica de aliasing propia de valores internos. `ref` conserva binding durante la llamada, no puntero general ni valor escapable. El ownership de `DB` es externo al runtime; el de drivers nativos es contado. El ownership de forks asíncronos y finalizadores no está cerrado en todas las rutas. `evaluateNew` crea un `Fork()` por instancia para un finalizer, y `AutoDestroy` puede crear otro fork para destructor sin `Free` visible (`pkg/core/evaluator_member.go`, `instance_lifecycle.go`). Además, finalizadores de Go no garantizan ejecución puntual; no deben ser la única estrategia para recursos críticos.

## 8. Concurrency

`async { ... }` se proyecta al built-in `async`: crea `Future`, hace `Fork` y ejecuta una goroutine; `await` espera su resultado/error. `channel`, `send`, `recv`, `close`, `foreach` sobre canal y `select` existen. `pkg/server` usa un fork por request y tiene caracterizaciones de WebSocket, sesiones y cleanup; no se debe extrapolar su seguridad a otras goroutines. `ClosureEnvironment.mu` serializa uso compartido de una captura.

Riesgos probados por inspección: forks sin `Free` en `builtins_async.go` y `task.go`; tareas sin cancelación ni join obligatorio; `Future` puede no observarse; operaciones de canal sin estado y con panics Go; `mobile.RunDirect` devuelve timeout mientras la goroutine puede seguir usando el runtime que su caller libera. La última ruta puede ocasionar carrera/uso tras reciclaje, no sólo retraso. Priorizar contexto/cancelación cooperativa y ownership explícito antes de prometer «timeout de ejecución» en SDK móvil. `go test -race` de la suite existente es necesario pero no demuestra ausencia de carreras en interleavings no cubiertos.

## 9. Performance

Hay trabajo previo útil: `runtime/plan` asigna slots por identificador AST, `runtime/frame` reduce maps para locales y `classMetadataCache` evita reconstruir jerarquías por acceso. `pkg/core/runtime_benchmark_test.go` cubre startup, operadores, loops, funciones, objetos, colecciones y escenarios de aplicación; `pkg/vm/vm_benchmark_test.go` cubre sólo un subconjunto. No hay en esta auditoría perfiles CPU/heap que permitan declarar un hot path dominante ni prometer porcentajes.

Candidatos para **medición**: `evaluateNew` busca clase de plugin recorriendo registro/clases y crea fork/finalizer por instancia; `evaluateMember` y dispatch nativo resuelven nombres; clases guardan campos en maps; `typesystem.Parse` de uniones reparsea nombres; coerción y validación de colecciones caminan valores; closures copian mapas; `Fork` copia varios registros. Una caché de método monomórfica/polimórfica o IDs sólo vale para clases estables y con invalidación clara. Antes de optimizar: bench de llamadas/propiedades en frío/caliente, `benchmem`, perfiles pprof y comparación semántica de ruta rápida/general.

## 10. Diagnostics

`diagnostics.Diagnostic` contiene código, severidad, archivo, rango, mensaje, explicación y sugerencia; el analyzer ordena por archivo/línea/columna. Parser y LSP consumen diagnósticos. Arith/index runtime tienen códigos; varios nativos aún arrojan strings o panics Go sin código ni span preciso. Un `JossError` admite stack, pero la traducción en CLI/móvil/servidor no tiene idéntica presentación. El parser recupera errores parcialmente y puede emitir derivados después del primero. La prioridad es alinear errores nativos frecuentes y añadir ubicación/rango completo al fallo runtime, sin inventar un nuevo código si ya existe uno canónico. Los ejemplos de `docs/DIAGNOSTICOS.md` deben acompañar cada código nuevo.

## 11. Tooling

El CLI implementa `run`, `build`, `check`, `analyze`, `test`, `format`, `lint`, `fix`, `eval` y **`repl`** (`cmd/joss/main.go`), además de comandos de aplicación/paquetes. `format` preserva trivia con scanner propio que consume símbolos canónicos; `fix` usa reglas/regex de transformación y necesita pruebas contra strings/comentarios. `vscode-joss` declara completion, hover, definition, references, signature help, diagnostics, document symbols y formatting; no aparece proveedor de rename en `server.ts`. Hay tests de parser fuzzing, typesystem fuzzing, valor Unicode, diferencial de VM y benchmarks. Faltan debugger/profiler de Joss integrados y un inspector de efectos/dependencias de proyecto; «ausente» no implica prioridad alta.

**Diferencias documentales concretas:** [Estado de implementación](ESTADO_IMPLEMENTACION.md) dice que no hay REPL, interfaces, `defer` ni `select`; el CLI, AST, analyzer y executor sí los tienen. El informe histórico que ocupaba esta página también afirmaba ausencia de `guard`/narrowing posterior, pero `body_analysis.go` e `infer_narrowing.go` ya implementan ambos. `sdk/dart/README.md` describe distribución móvil que debe verificarse contra artefactos de release reales antes de prometer instalación automática. Corregir esas páginas en la fase documental posterior, con contratos ejecutables; no usar sus afirmaciones antiguas como base de diseño.

## 12. Technical Debt

1. **HIGH:** contratos de colecciones mutables y `Unknown/Mixed` dejan huecos de soundness; `typesystem.Assignable` y `runtimeTypeOf` necesitan pruebas de aliasing y política explícita.
2. **HIGH:** rutas de entrada no comparten un `PreparedProgram` con análisis y metadata persistida; CLI, mobile, runner y server orquestan análisis/registro de forma distinta.
3. **HIGH:** ownership de forks fuera de HTTP no está expresado en tipos/API y permite olvidos.
4. **MEDIUM:** metadatos nativos publican retornos confiables, pero `ArityKnown=false` en muchas APIs; el analyzer no puede anticipar aridad/tipos.
5. **MEDIUM:** metadata de métodos/clases y resolución de plugin aún usan strings/maps y scans en rutas potencialmente frecuentes.
6. **MEDIUM:** `pkg/core` integra semántica e infraestructura; separar únicamente los puntos donde contratos/test de frontera lo justifiquen.
7. **MEDIUM:** documentación de estado y auditorías históricas contradice features actuales; la auditoría del idioma no puede basarse en esas páginas sin revalidar.
8. **LOW:** símbolos LSP generados conviven con firmas ricas mantenidas manualmente; riesgo de metadatos incompletos.

## 13. Bugs Found

| Severidad | Evidencia y causa raíz | Efecto / verificación pendiente |
|---|---|---|
| **CRITICAL** | `pkg/core/database.go` rama `transaction` abre `tx := db.Begin()`, llama callback mediante `r.CallFunction` y confirma `tx`; las consultas ordinarias usan `r.GetDB()`, sin contexto `tx`. Ya consta como límite en [Estado](ESTADO_IMPLEMENTACION.md). | Una operación del callback puede persistir aunque éste falle y se haga rollback del `tx`. Prueba SQLite `t.TempDir`: insert + throw + comprobar tabla vacía. |
| **HIGH** | `pkg/mobile/mobile.go` selecciona `time.After` pero no cancela ni espera la goroutine; restaura `os.Stdout/Stderr` y `defer rt.Free()` puede reciclar el runtime aún activo. | Timeout no es corte de ejecución; riesgo de goroutine persistente, salida tardía y carrera. Test de timeout con señalización/join y `-race`. |
| **HIGH** | `pkg/core/builtins_async.go` y `pkg/core/task.go` hacen `r.Fork()` sin `Free()` en la goroutine; `evaluator_member.go` y `instance_lifecycle.go` repiten patrón en finalizadores/destructor. | Estado/handles retenidos y ownership contado no liberado. Test de driver retenido y ciclos de fork; definir cierre del owner. |
| **HIGH** | `builtins_async.go` ejecuta `close(ch.Ch)`, `ch.Ch <- value` y `make(chan, size)` sin controlar doble cierre, envío a cerrado o capacidad negativa. | Panic de Go o bloqueo sin error estructurado Joss. Tests de los tres casos y concurrente con `-race`. |
| **MEDIUM** | `typesystem.Assignable` permite `int → float` y lo describe como sin pérdida, aunque `float64` pierde precisión por encima de 2^53. | Resultado numérico sorprendente en IDs/contadores grandes. Test de 2^53+1 y decisión de compatibilidad. |
| **MEDIUM** | `pkg/analyzer/project.go` carga entrypoint y `app/**/*.joss`; `routes.joss` se carga por servidor según [auditoría documental](DOCUMENTATION_AUDIT.md). | `joss analyze` puede omitir errores de rutas; verificar fixture web y unificar manifest de fuentes de ejecución. |
| **MEDIUM** | `docs/ESTADO_IMPLEMENTACION.md` niega REPL/interfaces/defer/select; implementación presente en `cmd/joss/main.go`, AST y `core/executor.go`. | Decisiones de usuarios y roadmap basados en información errónea. Corregir con contratos de documentación. |

No se atribuyen como bugs actuales el antiguo `match` de bloques, el escape de `break` en ternario o el reset del pool: hay rutas/guardas posteriores y requieren nueva reproducción antes de reabrirse. Los hallazgos de código anteriores aún precisan tests de regresión al corregirse; la inspección no sustituye una prueba end-to-end.

## 14. Missing Language Features

Problemas reales: falta análisis general de definite assignment y null-state; no hay tipo de mensaje para canales ni cancelación de tareas; no hay tipo de resultado para fallos esperados de I/O; no hay contrato de efecto/capacidad para llamadas nativas; la precisión de colecciones se pierde con APIs `mixed`. `match` de valores existe, pero no es exhaustivo por enum/tipo unión. Interfaces y enums **sí existen**; no se deben proponer como si faltaran. Imports fuente no existen por decisión explícita de proyecto zero-imports; añadirlos ahora rompería arquitectura sin solucionar el problema prioritario. Tampoco se justifica ownership general estilo Rust, generics universales, traits, AOT/LLVM o una VM completa en esta fase.

## 15. Proposed Language Features

Cada sintaxis es **propuesta**, no contrato actual.

| Propuesta y ejemplo | Problema | Parser/AST | Analyzer | Runtime/tooling | Compatibilidad; complejidad |
|---|---|---|---|---|---|
| **Definite assignment y null-state**: `T? $x`, guard y acceso posterior | uso antes de inicializar / dereference nullable | sin sintaxis nueva; CFG sobre AST actual | estados por símbolo y joins; narrowing invalidado por mutación/alias | defensas conservadas; hover muestra estado | compatible con warning → error opt-in; media |
| **`Result<T,E>` ligero**: `Result<Usuario, DbError>` con `match` | fallos esperados de DB/HTTP hoy mezclan nil, false, panic | gramática de tipo parametrizado ya existe para colecciones; ampliar AST de type refs y construcción | variantes y exhaustividad; propagación explícita sólo si se diseña | valor tagged y SDK; LSP completion | experimental; alta |
| **Canal tipado y cierre seguro**: `channel<string>` | enviar valor incorrecto/cerrar dos veces | extender TypeReference/AST sin cambiar `send` | comprobar tipo de mensaje y diagnósticos locales de estado | wrapper de estado, error Joss, hover; tests race | compatible opt-in; media |
| **`match` exhaustivo para enum/unión** | rama faltante deriva a null | AST actual tiene brazos/default; posible patrón simple nuevo | cobertura de casos y duplicados | runtime conserva fallback; quick fix LSP | warning primero; media |
| **Atributo de efecto para nativos confiables**, p. ej. metadata `effects: io, blocking` (sin sintaxis fuente inicialmente) | analyzer desconoce bloqueo/recursos | ningún cambio parser/AST al inicio | validar `await`, recursos y rutas críticas usando firmas | catálogo y LSP; wrapper host | compatible; media |

No recomendar `readonly` superficial mientras maps/slices aliasados sigan mutables; una garantía parcial debe llamarse explícitamente «binding inmutable». Safe casts sólo aportarían valor después de definir fallos como `Result`/nullable y probar la interacción con `mixed`. Records/sealed classes pueden reevaluarse después de exhaustividad nominal; extension methods/traits y overloads agregan resolución compleja sin bug demostrado que los exija.

**Comparación selectiva:** Kotlin/Dart/Swift inspiran null-state y promoción local, pero Joss necesita invalidar narrowing al escribir `mixed` o alias; TypeScript muestra utilidad y límites del control de flujo con dinamismo; Rust aporta `Result` y recursos explícitos, no hace necesario borrow checking completo; Go aporta channels y contextos de cancelación, mientras Joss debe evitar panics Go visibles; Java/C# muestran metadata estable y despachos anticipados, útil sólo para receptores nominales; Lua/JVM muestran VM viable tras equivalencia semántica; Python/PHP recuerdan el valor de `mixed` e iteración rápida, junto al costo de fallos tardíos. Copiar sintaxis externa sin resolver contratos internos no aporta garantías.

## 16. Compiled-Like Experience

Arquitectura objetivo incremental, derivada de las capas actuales:

```text
parser.Program + SourceUnits
  → analyzer: símbolos, tipos, contratos, CFG/estados
  → diagnostics + AnalysisFacts (ID de símbolo, tipo, efectos, spans)
  → PreparedProgram (AST inmutable + planes de callable/clase + referencias resueltas)
  → intérprete core (ruta publicada) / VM sólo para subset diferencial
  → runtime host (recursos, IO, requests, plugins)
```

Primero añadir hechos semánticos **sidecar** indexados por nodos AST, evitando mutar AST compartido por forks. IDs inmutables por proyecto/versión pueden estabilizar símbolos; `SlotID` ya existe para locales. Resolver anticipadamente funciones y métodos sólo cuando clase y tabla sean estables; `mixed`, plugins dinámicos y host globals mantienen guardas/fallback. Un typed IR completo tendría costo alto de duplicación semántica: exigir prototipo con comparación diferencial y métricas antes de adoptarlo. CFG para retornos, asignación y null-state tiene valor antes de optimizaciones. Constant folding/propagation sólo para operaciones puras con `typesystem.CheckedIntBinary`; nunca adelantar efectos de nativos. Field offsets y inline caches necesitan versión/invalidation de clase; no son primera fase.

## 17. Architecture Improvements

1. Un `PreparedProgram` por proyecto con origen de archivos, diagnósticos, `AnalysisFacts`, planes y catálogo de firmas; CLI/mobile/runner/server consumen la misma validación. Mantener `analyzer` sin importar `core`.
2. Una API de ownership para forks/tareas con `defer Free()` obligatorio en goroutine y finalización explícita de instancias con recursos. No trasladar ownership de `DB` al pool.
3. Un contexto de transacción en el adaptador de DB, propagado a todas las operaciones del callback; no basta con envolver `Begin/Commit`.
4. Metadata `NativeMethodDefinition` con parámetros y efectos sólo donde el contrato sea comprobado; `ArityKnown=false` debe permanecer para casos desconocidos.
5. Manifest único de archivos fuente del proyecto que cubra rutas web realmente ejecutadas; preservar política zero-imports.
6. Contextos de error con código/span/stack en fronteras nativas y SDK, sin managers públicos ceremoniales.

## 18. Security Improvements

Prioridad: transacciones atómicas y manejo de recursos; después, cancelación cooperativa de requests/tareas; capacidades explícitas para filesystem/red/proceso/FFI en host/plugin; límites de tamaño, tiempo y profundidad a nivel de operación. Verificar rutas y VFS contra traversal, y no confundir firma JP con sandbox. Para `null`, índice, división y overflow, el analyzer detecta constantes/estados demostrables y el runtime conserva checks. La propagación de errores async debe ser observable aunque el Future no se await; una política de tareas huérfanas (log/propagación al request o cancelación) necesita especificación antes de implementarse. `panic` de un nativo debe convertirse a error estructurado sin silenciar fallos internos no previstos.

## 19. Performance Improvements

Hipótesis con benchmark requerido: preindexar clases exportadas de plugins para `new`; reducir forks/finalizers por instancia; compartir metadata de clase inmutable entre forks; resolver receiver nominal a método precomputado; cache de acceso a propiedad con invalidación; especializar operadores de tipos conocidos en planes. La primera mejora puede ser de **seguridad y memoria** además de tiempo. Métricas mínimas: ns/op, B/op, allocs/op, p50/p95 de request, heap retenido por 10k objetos/futures, costo de cold start y pprof. Rechazar una optimización si rompe equivalencia, aumenta retención o beneficia sólo microbenchmarks irrelevantes.

## 20. Roadmap

Las prioridades son P0 (fallo de seguridad/correctitud), P1 (garantía central), P2 (evolución condicionada), P3 (opcional). Cada tarea parte de problema y evidencia anteriores; al ejecutarla: reproducir → caracterizar → comparar alternativas → corregir → pruebas → benchmark si aplica.

### Phase 1 — Correctness

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| Transacción real | P0 | Callback opera fuera de `sql.Tx`; introducir executor transaccional en toda consulta del callback, rollback en error | `core/database*.go`, tests SQL | compatible, cambio de bug | riesgo medio por nesting; atomicidad alta | SQLite rollback/commit/nested/error |
| Ownership de forks | P0 | forks async/task/finalizer sin cierre; `defer Free`, política de destructor y recursos | `core/builtins_async.go`, `task.go`, `evaluator_member.go`, `instance_lifecycle.go`, lifecycle | compatible | riesgo medio; memoria/handles alta | owner count, GC, panic, race |
| Timeout móvil | P0 | goroutine sobrevive y runtime se libera; cancelación cooperativa + join o aislar proceso si se promete hard timeout | `mobile/mobile.go`, `core/executor.go` | deprecation del timeout «duro» si no se logra | riesgo alto; aislamiento alto | bucle infinito, IO bloqueante, race, stdout |
| Canales seguros | P0 | panics Go por close/send/capacidad; wrapper estado y errores Joss | `core/builtins_async.go`, channel tests | compatible salvo tipo de error | riesgo medio; estabilidad alta | cerrado/duplicado/negativo/concurrente |

### Phase 2 — Type Safety

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| Soundness de colecciones | P1 | covarianza/aliasing y pérdida de tipo; decidir invariancia o wrapper revalidado y documentar | `typesystem/types.go`, analyzer, core collections | warning → deprecation si rompe | riesgo alto; seguridad alta | alias, nested, mutación, nativos |
| Firmas nativas | P1 | aridad/tipos desconocidos; completar sólo contratos confiables | `core/native_signatures.go`, analyzer, catálogo | compatible con warning | riesgo bajo/medio; DX alta | paridad firma-handler, negativos |

### Phase 3 — Static Analysis

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| CFG y definite assignment | P1 | inicialización/retorno parcialmente resueltos; CFG mínimo por callable con joins | `analyzer/flow.go`, `body_analysis.go` | warning → error opt-in | riesgo medio; seguridad alta | ramas, loops, try/catch, defer, yield |
| Null-state/exhaustividad | P1 | dereference y match incompleto; narrowing con invalidación y cobertura de enums | `analyzer/infer_narrowing.go`, `flow.go`, LSP | warning → error opt-in | riesgo medio; DX alta | alias, mutación, union, defaults |
| Fuente web completa | P1 | `routes.joss` fuera del análisis de proyecto; manifest de ejecución compartido | `analyzer/project.go`, CLI/server | compatible | riesgo bajo; errores tempranos | proyecto web fixture |

### Phase 4 — Runtime Safety

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| Cancelación y errores nativos | P1 | bloqueo/panics sin código; contexto por ejecución y adaptadores de error | `core`, server, mobile, runtime/errors | compatible salvo mensajes | riesgo alto; estabilidad alta | timeout/IO/canal/panic/stack |
| Capability host | P2 | FFI/FS/red sin contrato declarativo universal; permisos de host comprobados | pluginruntime/core/native | experimental | riesgo alto; seguridad alta | rechazo y autorización por recurso |

### Phase 5 — Execution Architecture

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| AnalysisFacts/PreparedProgram | P2 | análisis y planes no persistidos uniformemente; sidecar inmutable, IDs, fuentes | analyzer, runtime/plan, core, CLI/mobile | compatible interno | riesgo alto; predictibilidad alta | diferencial ruta previa/nueva, invalidación |
| VM selectiva | P3 | VM cubre subset; ampliar sólo después de corpus diferencial | `pkg/vm` | experimental | riesgo alto; potencial rendimiento | diferencial, fuzz, bench |

### Phase 6 — Language Features

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| Canal tipado + Result | P2 | errores de mensaje/I-O tardíos; prototipos independientes, sin imponer adopción | parser, typesystem, analyzer, core, docs | experimental | riesgo alto; API clara | sintaxis +/-; analyzer; runtime; LSP |
| Match exhaustivo | P2 | faltan casos nominales; warning antes de error | analyzer, docs, LSP | compatible con warning | riesgo medio; robustez | enums/uniones/default |

### Phase 7 — Tooling and Documentation

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| Corregir estado/documentación | P1 | páginas niegan features existentes; actualizar desde código, espejo y traducciones | `docs/*.md`, espejo, traducciones | compatible | riesgo bajo; confianza alta | contratos, navegación, docsi18n |
| Rename y trazas | P2 | LSP sin rename, depuración limitada; usar IDs semánticos antes de editar referencias | `vscode-joss`, analyzer | compatible | riesgo medio; DX alta | workspace edits, shadowing |

### Phase 8 — Performance

| Tarea | Prioridad | Problema / solución | Archivos afectados | Compatibilidad | Riesgo / beneficio | Tests necesarios |
|---|---|---|---|---|---|---|
| Perfilar y optimizar hot paths | P2 | scans/maps/forks potencialmente caros; pprof primero, cache con invalidación después | `core/evaluator_member.go`, runtime/plan, class metadata, plugin registry | compatible interno | riesgo medio/alto; rendimiento por medir | benchmem, pprof, diferencial, race |

### Matriz de recomendaciones

| Propuesta | Seguridad | Estabilidad | Rendimiento | DX | Complejidad | Prioridad |
|---|---|---|---|---|---|---|
| Transacción ligada a Tx | alta: evita rollback falso | alta | neutra | media | media | P0 |
| Cierre de forks y timeout móvil | alta: evita estado concurrente reciclado | alta | media por retención | media | alta | P0 |
| Canales con errores Joss | alta: evita panic host | alta | neutra | alta | media | P0 |
| CFG/null-state | alta: adelanta fallos | media | neutra | alta | media | P1 |
| Colecciones sound | alta: protege aliasing | alta | posible costo de check | media | alta | P1 |
| Firmas nativas verificadas | media | media | neutra | alta | media | P1 |
| PreparedProgram/IDs | media | media | potencial alta | media | alta | P2 |
| VM ampliada | baja hasta equivalencia | incierta | potencial alta | baja | alta | P3 |

Las categorías describen causalidad y costos demostrables en el código; «potencial» significa que falta benchmark, no puntuación inventada. El orden preserva semántica antes de optimizar. No se recomienda iniciar implementación de esta hoja de ruta hasta revisar y aceptar sus contratos de comportamiento y pruebas de regresión.

**Verificación de esta edición.** `go test ./...`, `go vet ./...`, `go build ./...`, `go run ./tools/cataloggen --check`, `go run ./tools/docgen --check`, `go test ./pkg/core -run TestDocumentation -v`, `npm run compile` y `git diff --check` terminaron con código 0. Una copia temporal de VS Code completó `npm ci --ignore-scripts` y `npm run compile`; `npm ci` normal falló con `EPERM spawn` tanto en el checkout como en una copia limpia, por lo que sus scripts de instalación no quedaron validados. Go imprimió advertencias del host por ACL de telemetry/caché sin afectar los comandos aprobados. `go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core` no pudo construir `runtime/race` en este host (`package testmain: cannot find package`); no se presenta como aprobado. `go run ./tools/docsi18n -check` reportó traducciones y espejos desactualizados preexistentes y, tras esta edición, versiones en/pt del informe pendientes de traducción. El espejo español de JosSecurity se sincronizó byte por byte; la traducción completa queda como tarea documental, sin cambios semánticos.

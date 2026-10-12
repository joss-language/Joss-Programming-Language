# Revisión Arquitectónica Profunda — Compilación Nativa AOT, Runtime, Standard Library e Integraciones Host de Joss

> **Documento de Arquitectura e Ingeniería de Compiladores**
> **Estado del árbol analizado:** `HEAD` limpio en commit `e935150` (`fix(cli): deprecate runner bundler and enforce pure AOT compilation error reporting`)
> **Principio rector (`AGENTS.md` §1 y §3):** El código fuente y sus tests ejecutados son la única fuente de verdad. Está terminantemente prohibido duplicar manualmente en C/C++/Rust implementaciones canónicas que ya existen en los paquetes de Go (`pkg/core`, `pkg/server`, `pkg/viewtemplate`, `pkg/pluginruntime`, `pkg/typesystem`).

---

## 1. Arquitectura actual

### 1.1 Mapa de subsistemas del repositorio

```text
                                      JOSS SOURCE (.joss)
                                               │
                                               ▼
                        pkg/parser (Lexer + Pratt Parser + AST)
                                               │
                                               ▼
               pkg/analyzer (collectDeclarations → projectScope →
                             validateNominalContracts → analyzeSourceBodies)
                                               │
                                               ▼
                          analyzer.PreparedProgram (Inmutable)
                                               │
            ┌──────────────────────────────────┼──────────────────────────────────┐
            ▼                                  ▼                                  ▼
   MODO INTERPRETADO                  MODO SERVIDOR HTTP                 MODO COMPILADOR AOT
   `joss run` (`cmd/joss`)            `joss server` (`pkg/server`)       `joss build` (`cmd/joss`)
            │                                  │                                  │
            ▼                                  ▼                                  ▼
   pkg/core.Runtime                   pkg/server.StartServer             pkg/ir.Lowerer
   ExecutePrepared(Prepared)          + Runtime Pool (Fork/Free)         LowerProgram() -> *ir.Program
   + Frame/Slot Plan                  + MVC / Router / WebSocket                  │
   + Built-ins + Clases Nativas       + ViewTemplate / GranDB                     ▼
                                                                         pkg/ir.IRVerifier
                                                                         Verify(*ir.Program)
                                                                                  │
                                                                                  ▼
                                                                         pkg/backend/native.BuildProgram
                                                                         ┌────────┴────────┐
                                                                         ▼                 ▼
                                                                  BackendLLVM      BackendStandalone
                                                                  (`llvm.go` +     (`standalone.go` ->
                                                                   `joss_rt.c`)     Go source -> `go build`)
```

### 1.2 Responsabilidad real por paquete (verificada en código)

| Paquete / Directorio | Archivos clave | Responsabilidad real verificada |
|---|---|---|
| `cmd/joss` | `main.go` (859 LOC), `fix.go`, `pub_cli.go`, `NewProject.go` | CLI unificado: `run`, `server`, `build`, `emit-ir`, `check`/`analyze`, `fmt`, `fix`, `repl`, `test`, `pkg`, `pub`. En `HEAD` (`e935150`), `joss build` invoca exclusivamente `buildNativeProgram` (`cmd/joss/main.go:510-619`) y `runNativeCompile` (`cmd/joss/NewProject.go:580-597`) redirige al mismo flujo sin empaquetar AST en `runner_template.go.txt`. |
| `pkg/parser` | `token.go`, `lexer.go`, `parser.go`, `parser_expressions.go`, `parser_statements.go`, `ast.go` | Fuente canónica de tokens, keywords (`KeywordNames()`), precedencias Pratt y nodos del AST. |
| `pkg/typesystem` | `types.go`, `compatibility.go`, `primitive_methods.go` | Fuente canónica de tipos (`Kind`, `Type`), reglas de asignabilidad (`Assignable`), inferencia (`MergeInference`), coerción explícita (`CoerceString`) y catálogo de métodos primitivos (`LookupPrimitiveMethod`). |
| `pkg/diagnostics` | `diagnostic.go` | Estructura canónica `Diagnostic` con código estable `JOSS-...`, severidad, posición, explicación y sugerencia. |
| `pkg/analyzer` | `pipeline.go`, `program.go`, `scope.go`, `infer.go`, `calls.go`, `statements.go`, `contracts.go`, `reachability.go`, `loader.go` | Pipeline semántico de 4 fases que produce `PreparedProgram` (`pkg/analyzer/program.go:100-149`). No importa `pkg/core`. |
| `pkg/runtime/frame` y `pkg/runtime/plan` | `frame.go`, `callable.go` | Layout de slots locales por índice entero (`Callable.SlotNames`, `SlotIndex`) y marcos de activación (`frame.Frame`) para evitar búsquedas por mapa en variables locales de funciones interpretadas. |
| `pkg/core` | `runtime.go`, `runtime_lifecycle.go`, `evaluator.go`, `evaluator_infix.go`, `statement.go`, `call_method.go`, `call_arguments.go`, `callable_dispatch.go`, `builtins*.go`, `native*.go`, `database*.go`, `model*.go`, `view.go`, `auth*.go`, `security*.go`, `concurrency.go` | **Monolito acoplado:** contiene simultáneamente (a) el evaluador de AST (`evaluateExpression`, `executeStatement`), (b) el ciclo de vida del `Runtime` (`NewRuntime`, `Fork`, `Free`), (c) el catálogo y la implementación de ~115 built-ins globales, y (d) las ~50 clases nativas y de infraestructura (`GranDB`, `Model`, `Router`, `Auth`, `View`, `Request`, `Response`, `Session`, `Json`, `DateTime`, `FileStream`, `Http`, `Mail`, `Cron`, `Future`, `Channel`, etc.). |
| `pkg/server` | `server.go`, `handlers.go`, `response_writer.go`, `ws.go`, `security.go`, `admin_panel.go` | Servidor HTTP/MVC y WebSocket sobre `net/http`. Adquiere un `*core.Runtime` por petición mediante `baseRuntime.Fork()` (`pkg/server/handlers.go`) y ejecuta middlewares, closures o métodos de controlador evaluando AST en `pkg/core`. |
| `pkg/viewtemplate` | `directive.go`, `parser.go` | Parser sintáctico de directivas de plantillas (`@if`, `@foreach`, `@json`, `{{ ... }}`) con posicionamiento Unicode por runas. |
| `pkg/bytecode` y `pkg/plugin*` | `pkg/bytecode/bytecode.go`, `pkg/plugincompiler`, `pkg/pluginpkg`, `pkg/pluginruntime` | Serialización comprimida de AST (`JOSSBC2Z`, gob + zlib) usada exclusivamente para paquetes de plugins `.jp` y su índice de símbolos `SymbolIndex`. |
| `pkg/ir` | `types.go`, `values.go`, `instructions.go`, `program.go`, `lower.go`, `verifier.go` | Representación intermedia *Joss Native IR* (SSA/3AC sobre bloques básicos con terminadores explícitos), `Lowerer` desde `*analyzer.PreparedProgram` y verificador `IRVerifier`. |
| `pkg/backend/native` | `driver.go`, `llvm.go`, `standalone.go`, `compiler.go`, `types.go`, `target.go` | Orquestador del backend nativo (`BuildProgram`), emisor de LLVM IR textual (`LLVMEmitter`) y generador Go standalone (`StandaloneBuilder`). |
| `runtime/native` | `embed.go`, `csrc/joss_rt.h`, `csrc/joss_rt.c` | Runtime mínimo en C (304 líneas) embebido con `//go:embed` para enlazar con el módulo `.ll` cuando se usa `BackendLLVM`. |

---

## 2. Arquitectura del interpreter

### 2.1 Entrada y preparación

1. **Carga y análisis obligatorio:**
   - `joss run` (`cmd/joss/main.go:349-410`) carga las unidades fuente con `semanticanalyzer.LoadProject(filename, "app")` (`pkg/analyzer/loader.go`) y ejecuta `core.AnalyzeSourceUnits(units)` (`pkg/core/semantic_bridge.go`).
   - Si el analizador reporta errores (`report.HasErrors()`), `joss run` imprime los diagnósticos y aborta con `os.Exit(1)` antes de instanciar el runtime.
2. **Ejecución del `PreparedProgram`:**
   - `Runtime.ExecutePrepared(prepared)` (`pkg/core/runtime.go:336-393`) registra primero todas las declaraciones de clases, interfaces, funciones, enums y variables globales de las unidades en orden topológico (`prepared.OrderedUnits()`), y después ejecuta los statements ejecutables de nivel superior (`mainStmt`) mediante `r.executeStatement(stmt)`.

### 2.2 Modelo de memoria y evaluación de expresiones/statements

- **Estado de `Runtime` (`pkg/core/runtime.go:79-145`):**
  - `Variables map[string]interface{}` y `VarTypes map[string]string`: variables de alcance global y bindings del host (`cout`, `cin`, `cerr`, `endl`, `JOSS_VERSION`, y proxies de clases nativas como `$GranDB`, `$Router`, `$Auth`, `$View`, `$Session`, `$Request`, `$Response`, etc.).
  - `currentFrame *frame.Frame` (`pkg/runtime/frame/frame.go`): marco léxico activo durante la ejecución de una función o método. Posee un slice preasignado `Slots []interface{}` indexado por entero según `runtimeplan.Callable` (`pkg/runtime/plan/callable.go`), evitando lookups en mapas para locales y parámetros.
  - `Classes map[string]*parser.ClassStatement`, `Functions map[string]*parser.MethodStatement`, `Enums map[string]*EnumDefinition`: tablas de definiciones AST en memoria.
- **Representación de valores en memoria (Interpreter Boxed Values):**
  - `int` -> `int64` (o `int` normalizado por `normalizeNumeric`).
  - `float` -> `float64`.
  - `decimal` -> `decimal.Decimal` (`github.com/shopspring/decimal`).
  - `bool` -> `bool`.
  - `string` -> `string`.
  - `array` -> `[]interface{}`.
  - `map` -> `map[string]interface{}`.
  - Objetos de usuario y nativos -> `*core.Instance` (`ClassName string`, `Fields map[string]interface{}`, `FieldTypes map[string]string`, `FieldConst map[string]bool`, `TypeArgs []typesystem.Type`, `IsRecord bool`, `NativeData interface{}`).
  - Closures -> `*core.FunctionObject` (`Literal *parser.FunctionLiteral`, `Env map[string]interface{}`, `EnvTypes map[string]string`, `This *Instance`, `Plan *runtimeplan.Callable`).
  - Referencias temporales (`ref T $x`) -> `*core.ReferenceValue` (`Getter`, `Setter`, `ExpectedType`).

### 2.3 Despacho de llamadas y excepciones

- **Llamadas (`pkg/core/call_method.go`, `pkg/core/callable_dispatch.go`, `pkg/core/call_arguments.go`):**
  - Resuelve argumentos posicionales, nombrados, valores por defecto y enlaces `ref` usando la firma del método/función.
  - Verifica tipos de entrada y retorno en runtime (`JOSS-TYPE-008`, `JOSS-TYPE-011`).
  - Crea un `frame.Frame` aislado con límite de recursión `MaxCallDepth = 1024`.
- **Control de flujo no local y excepciones:**
  - `return` propaga `returnSignal{value}` o marca `r.hasReturned` en el frame.
  - `throw` y los errores runtime invocan `r.throwRuntimeException(className, message, code)` (`pkg/core/exceptions.go`), que lanza un `panic(RuntimeExceptionSignal{Instance: ...})`.
  - `TryCatchStatement` (`pkg/core/statement.go`) intercepta `RuntimeExceptionSignal` mediante `defer recover()`, verifica compatibilidad nominal con los bloques `catch (Tipo $e)`, ejecuta el bloque correspondiente y garantiza la ejecución de `finally` y `defer`.

---

## 3. Arquitectura del compiler (Frontend Semántico)

El frontend del compilador (`pkg/parser` + `pkg/analyzer`) es compartido al 100% entre `joss run`, `joss server`, `joss check`, `joss emit-ir` y `joss build`.

### 3.1 Fases de `analyzer.Analyzer.Analyze()` (`pkg/analyzer/pipeline.go:25-60`)

1. **`collectDeclarations` (`pkg/analyzer/declarations.go`):**
   - Registra clases (`ClassSymbol`), interfaces (`InterfaceSymbol`), enums (`EnumSymbol`), funciones globales (`CallableSignature`) y esquemas de tablas (`TableSymbol` de migraciones) en el catálogo global.
2. **`projectScope` (`pkg/analyzer/scope_projection.go`):**
   - Construye el grafo de unidades e imports (`import "modulo"`), resuelve visibilidad (`public`, `private`, `protected`) y proyecta los símbolos exportados, built-ins (`core/builtins.go` proyectados vía `typesystem`/catálogo) y clases nativas en el scope de cada `SourceUnit`.
3. **`validateNominalContracts` (`pkg/analyzer/contracts.go`):**
   - Valida herencia simple (`extends`), implementación de interfaces (`implements`), constructores `Init`, invariantes de `record`, restricciones genéricas (`<T>`), sobrecarga inválida y tipos de retorno explícitos (`JOSS-TYPE-014`).
4. **`analyzeSourceBodies` (`pkg/analyzer/statements.go`, `pkg/analyzer/infer.go`, `pkg/analyzer/calls.go`):**
   - Ejecuta type-checking e inferencia de tipos sobre todas las instrucciones y expresiones.
   - Registra en `Analyzer` tres mapas fundamentales para la compilación AOT:
     - `exprTypes map[parser.Expression]typesystem.Type`: el tipo estático inferido para **cada nodo `parser.Expression`** del programa (`pkg/analyzer/program.go:108`).
     - `resolvedCalls map[*parser.CallExpression]*ResolvedCall`: el destino exacto (`TargetKind`: función de usuario, método de clase, método nativo, built-in global, closure), firma (`Signature`) y tipo de retorno instanciado para **cada llamada** (`pkg/analyzer/program.go:53-98`, `109`).
     - `reachability *ReachabilityGraph`: grafo de alcanzabilidad desde el punto de entrada (`pkg/analyzer/reachability.go`).

### 3.2 `PreparedProgram` (`pkg/analyzer/program.go:100-149`)

`PreparedProgram` es una estructura inmutable que expone:
- `Units []*SourceUnit` y `Entry *SourceUnit`
- `Classes map[string]*ClassSymbol`, `Interfaces map[string]*InterfaceSymbol`, `Enums map[string]*EnumSymbol`, `Functions map[string]*CallableSignature`, `Tables map[string]*TableSymbol`
- `GlobalVars map[string]*Symbol`
- `ExprTypes map[parser.Expression]typesystem.Type` (accesible vía `PreparedProgram.ExpressionType(expr)`)
- `ResolvedCalls map[*parser.CallExpression]*ResolvedCall` (accesible vía `PreparedProgram.ResolvedCall(call)`)
- `Reachability *ReachabilityGraph`

> **Hallazgo clave del compilador frontend:** El frontend semántico de Joss **ya calcula y conserva toda la información de tipos y resolución de llamadas necesaria para compilación AOT**. El problema actual no está en el parser ni en el analyzer, sino en que `pkg/ir/lower.go` descarta o ignora más del 70% de la información presente en `PreparedProgram`.

---

## 4. Arquitectura del IR (`pkg/ir`)

### 4.1 Estructura de Joss Native IR

Definida en `pkg/ir/types.go`, `pkg/ir/values.go`, `pkg/ir/instructions.go` y `pkg/ir/program.go`:

- **Tipos (`pkg/ir/types.go:8-27`):**
  - Escalares: `void`, `bool`, `i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `f32`, `f64`, `string`.
  - Compuestos: `ptr` (`*Elem`), `func` (`Params`, `Ret`), `array` (`*Elem`, `Len`), `struct` (`Name`, `Fields`), `mixed`, `unknown`.
- **Valores (`pkg/ir/values.go`):**
  - `TempValue` (`%t0`, `%t1`...), `ParamValue` (`%$x`), `GlobalValue` (`@g`), y constantes (`ConstInt`, `ConstFloat`, `ConstBool`, `ConstString`, `ConstNull`).
- **Instrucciones (`pkg/ir/instructions.go`, 33 opcodes):**
  - Memoria local: `MoveInst`, `AllocaInst`, `LoadInst`, `StoreInst`.
  - Aritmética y lógica: `BinaryInst` (`add`, `sub`, `mul`, `div`, `mod`, `shl`, `shr`, `and`, `or`, `xor`), `UnaryInst` (`neg`, `not`), `CompareInst` (`eq`, `ne`, `lt`, `le`, `gt`, `ge`, `spaceship`), `CastInst`.
  - Llamadas: `CallInst` (función estática), `CallRuntimeInst` (helper `joss_*` del runtime).
  - Objetos, arrays y mapas: `NewObjectInst`, `LoadFieldInst`, `StoreFieldInst`, `NewArrayInst`, `ArrayGetInst`, `ArraySetInst`, `NewMapInst`, `MapGetInst`, `MapSetInst`.
  - Excepciones: `ThrowInst`, `TryCatchInst` (contiene slices anidados `TryBlocks []*BasicBlock` y `CatchBlocks []*BasicBlock`).
  - Terminadores de bloque básico: `ReturnTerminator`, `JumpTerminator`, `BranchTerminator`, `UnreachableTerminator`.
- **Verificador (`pkg/ir/verifier.go:19-176`):**
  - `IRVerifier.Verify(prog)` comprueba que toda función con bloques tenga bloque `entry`, sin etiquetas duplicadas, que todo bloque tenga `Terminator != nil`, que las instrucciones no tengan operandos `nil`, que `BranchTerminator.Cond` sea `TypeKindBool` (o entero en comparaciones), y que `ReturnTerminator` coincida con `fn.ReturnType` (permitiendo implícitamente retorno `i64(0)` en `main`).

### 4.2 Qué hace hoy `pkg/ir/lower.go` y qué información semántica pierde

`Lowerer` (`pkg/ir/lower.go:1-1170`) traduce `*analyzer.PreparedProgram` a `*ir.Program`. Un análisis línea por línea demuestra las siguientes limitaciones estructurales:

1. **Pérdida de distinción de llamadas (`lowerCallExpression`, `pkg/ir/lower.go:880-953`):**
   - Cuando el código fuente invoca una función global (`len($arr)`, `trim($s)`, `json_encode($m)`), un método de instancia (`$obj.metodo()`), un método de clase nativa (`GranDB::table("users")`, `Router::get(...)`, `View::render(...)`) o un método primitivo (`"hola".upper()`), `lowerCallExpression` simplemente concatena `ClassName + "_" + MethodName` (líneas 891-904) y emite un `CallInst` directo como si fuera una función de usuario compilada en el mismo módulo IR.
   - **No consulta `ResolvedCall.Kind`** (`CallTargetBuiltin`, `CallTargetNativeMethod`, `CallTargetPrimitiveMethod`, `CallTargetUserFunction`, `CallTargetMethod`, `CallTargetClosure`) para decidir si debe emitir una llamada a función de usuario, una intrínseca del runtime de lenguaje o una llamada al Host Bridge.
2. **Colapso de tipos a `i64` y `string`:**
   - En `lowerEchoStatement` (`pkg/ir/lower.go:450-463`): si el valor es `TypeKindString` emite `CallRuntimeInst("print_string")`; en **cualquier otro caso** (`float`, `bool`, `decimal`, `array`, `map`, `object`) emite `CallRuntimeInst("print_i64")` (línea 458).
   - En `lowerArrayLiteral` (`pkg/ir/lower.go:809-838`): fija `elemType = TypeI64` salvo que algún elemento sea `TypeKindString` (líneas 814-821). Arrays de `float`, `bool`, objetos o mapas pierden su tipo.
   - En `lowerMapLiteral` (`pkg/ir/lower.go:840-862`): todos los mapas se tipan como `PtrType(StructType("map", nil))` y en `lowerIndexExpression` (`pkg/ir/lower.go:864-878`) `MapGetInst` y `ArrayGetInst` fuerzan `valType = TypeI64` (líneas 868 y 875), ignorando `l.lookupExprType(e)`! Esto provoca que leer un `string` de un `array` o `map` produzca un `TempValue` de tipo `i64` en el IR.
   - En `lowerMemberExpression` (`pkg/ir/lower.go:798-807`): si `lookupExprType(e)` no está registrado o es `Mixed`, cae a `TypeI64`.
3. **Dependencia del orden de declaración de clases (`lowerNewExpression`, `pkg/ir/lower.go:769-775`):**
   - Busca `if l.prog.Functions[initFnName] != nil` en las funciones ya bajadas al IR en lugar de consultar `l.prepared.Classes[e.ClassName]`. Si `new Foo()` aparece en `main.joss` antes de que la unidad de `Foo` haya sido procesada, no emite la llamada al constructor `Foo_Init`.
4. **Identificadores globales y constantes ignorados (`lowerIdentifier`, `pkg/ir/lower.go:606-613`):**
   - Solo busca en `l.varLocals[e.Value]` (variables locales de la función actual). Las variables globales (`l.prog.Globals`), constantes globales y los singletons del host (`$GranDB`, `$Router`, `$Auth`, `$View`, `cout`, `endl`) devuelven `UnsupportedCapabilityError("variable local no encontrada en contexto nativo")`.
5. **Constructos rechazados con `JOSS-NATIVE-001` (`pkg/ir/lower.go:1131-1169`):**
   - Statements: `EnumStatement`, `SelectStatement`, `ForeachStatement`, `DeferStatement`, `DestructureStatement`, `ImportStatement` (fuera de unidades ya cargadas), `SchemaMigrationStatement`.
   - Expresiones: `MatchExpression`, `YieldExpression`, `AsyncExpression`, `BlockExpression`, `FunctionLiteral` (closures), `PipeExpression` (`|>`), `StreamExpression` (`cout << ...`), `PostfixExpression` (`$i++`, `$i--`), `NullCoalesceExpression` (`??`), `OptionalChainExpression` (`?.`), `SuperExpression`, `IsExpression`, `TryExpression`.

---

## 5. Arquitectura del backend (`pkg/backend/native`)

### 5.1 Flujo real de `BuildProgram` (`pkg/backend/native/driver.go:68-178`)

Cuando `joss build` invoca `native.BuildProgram(prog, opts)`:

1. Ejecuta `ir.NewVerifier().Verify(prog)` (`driver.go:83-86`).
2. Si `opts.Trace` está activo, escribe `<out>.ir` (`prog.Dump()`), `<out>.ll` (`LLVMEmitter.Emit(prog)`) y `<out>.standalone.go` (`StandaloneBuilder.GenerateSource(prog)`).
3. Selección de backend (`driver.go:118-137`):
   - `NewNativeCompiler()` (`pkg/backend/native/compiler.go:18-26`) busca **únicamente `exec.LookPath("clang")`** (no busca `gcc`, a pesar de que el mensaje de error en la línea 39 dice `"no se encontró clang o gcc en PATH"`).
   - Si se pide `--backend=llvm` y no hay `clang`, falla con error (`driver.go:121-125`).
   - Si el modo es `BackendAuto` (el predeterminado de `joss build`) y no hay `clang` o el target es cross-target, **cae silenciosamente a `BackendStandalone`** (`driver.go:127-136`).

### 5.2 Estado real de `BackendLLVM` (`pkg/backend/native/llvm.go` + `runtime/native/csrc/joss_rt.c`)

- `LLVMEmitter` (`pkg/backend/native/llvm.go:1-492`) traduce `*ir.Program` a texto LLVM IR (`.ll`) y `NativeCompiler.CompileLLVMWithOptions` (`compiler.go:37-97`) invoca `clang -O2 module.ll joss_rt.c -o <out>`.
- **Instrucciones IR que `llvm.go` NO soporta** (`llvm.go:332-334` devuelve `fmt.Errorf("llvm emitter: instrucción no soportada: %T", inst)`):
  - `*ir.TryCatchInst` y `*ir.ThrowInst` (no emite `invoke`/`landingpad` ni `setjmp`/`longjmp`).
  - `*ir.CastInst`.
  - `BinaryInst` y `CompareInst` sobre `float`/`double` (`llvm.go:205-248` usa siempre `add`, `sub`, `mul`, `sdiv`, `srem` e `icmp` enteros con `MapLLVMType(i.Dest.Type())`, lo cual produce LLVM IR inválido `add double ...` en vez de `fadd double` y `fcmp`).
  - Concatenación o igualdad de strings en `CompareInst` (`icmp eq i8*` compara punteros, no contenido de cadenas).
- **Limitaciones severas de `runtime/native/csrc/joss_rt.c` (304 líneas):**
  - Solo define 21 funciones C (`joss_print_i64/f64/bool/string`, `joss_panic`, `joss_str_concat`, `joss_obj_new/ get/set_field_i64/str`, `joss_arr_new/push/get/set_i64/str/len`, `joss_map_new/get/set_i64/str`).
  - No tiene recolector de basura ni conteo de referencias (cada `malloc`/`strdup` en `joss_rt.c` tiene fuga de memoria).
  - Un `JossArray` en `joss_rt.c:147-152` es homogéneo (`int64_t*` o `char**`); si se inserta un `int64` y luego un `string`, corrompe la memoria o trunca punteros.
  - No tiene valor dinámico etiquetado (`Tagged Value` / `Mixed`), ni soporte para `float`/`bool`/objetos dentro de arrays o mapas, ni excepciones (`try`/`catch`).

### 5.3 Estado real de `BackendStandalone` (`pkg/backend/native/standalone.go`)

- `StandaloneBuilder` (`pkg/backend/native/standalone.go:1-659`) traduce cada `*ir.Function` a una función Go (`func foo(arg_x int64) int64`) con temporales `var t0 int64` y etiquetas `block_entry: ... goto block_...`, añade un preámbulo de 29 líneas (`standalone.go:98-127`) con structs Go `JossObj`, `JossArr`, `JossMap`, escribe un único archivo temporal `main.go` y ejecuta `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o <out> main.go` (`standalone.go:55-80`).
- **Importante:** `BackendStandalone` **NO** embebe el AST, **NO** importa `pkg/core`, **NO** usa `pkg/parser` y **NO** ejecuta el intérprete. Sin embargo:
  1. Depende de tener el toolchain `go` instalado en la máquina donde se corre `joss build`.
  2. Tampoco tiene acceso a la librería estándar de Joss ni a las integraciones host (`GranDB`, `Server`, `View`, etc.), porque genera un `main.go` aislado que solo importa `"fmt"` y `"os"`.
  3. Tiene bugs de tipado heredados de `lower.go`: `ConstBool` se formatea como `"1"`/`"0"` (`standalone.go:607-611`) mientras `joss_print_bool` espera `bool`, y `CompareInst` sobre strings o floats usa operadores Go sobre tipos a veces declarados como `int64`.

---

## 6. Runtime actual

Actualmente existen **dos runtimes desconectados** en el repositorio:

### 6.1 El Runtime Canónico en Go (`pkg/core` + `pkg/runtime/frame` + `pkg/runtime/plan`)

- **Ciclo de vida (`pkg/core/runtime_lifecycle.go`):**
  - Gestionado mediante `sync.Pool` (`runtimePool`) con protección `atomic.Bool` (`freed`) contra double-free (`Free()` en `runtime_lifecycle.go:180-220`).
  - `Fork()` (`runtime_lifecycle.go:142-178`) clona el estado por petición HTTP/tarea concurrente (`Variables` globales del host, `Classes`, `Functions`, `Enums`, `Routes`, `Env`, `DB` compartido, retención contada de `NativeDrivers`) con un `context.Context` cancelable propio.
- **Servicios que provee:**
  - Gestión de frames léxicos (`frame.Frame`) y slots (`runtimeplan.Callable`).
  - Representación de objetos (`*Instance`), closures (`*FunctionObject`), referencias (`*ReferenceValue`), futuros (`*FutureObject`), canales (`*ChannelObject`) y flujos (`*FileStreamInstance`).
  - Despacho dinámico de métodos, herencia (`extends`), comprobación de interfaces (`implements`, `is`), constructores `Init`, `record` inmutables, genéricos y `method_missing`.
  - Mecanismo de excepciones tipadas (`RuntimeExceptionSignal`), `defer` LIFO y `try`/`catch`/`finally`.

### 6.2 El Micro-Runtime C (`runtime/native/csrc/joss_rt.c`)

- Creado originalmente como runtime de apoyo para `BackendLLVM`.
- Solo implementa impresión básica, concatenación de strings con `malloc`, y listas enlazadas rudimentarias para objetos/mapas con campos `int64` o `const char*`.
- **No comparte ni una sola línea ni contrato de tipos con `pkg/typesystem` ni con `pkg/core`.**

---

## 7. Standard library actual

Toda la librería estándar del lenguaje vive en `pkg/core` dividida en dos mecanismos:

### 7.1 Built-ins globales (`pkg/core/builtins.go` y `pkg/core/builtins_*.go`)

- **Catálogo canónico (`pkg/core/builtins.go:25-175`):** Declara 115 funciones globales organizadas por dominio (`builtinDomainCore`, `builtinDomainString`, `builtinDomainArrayMap`, `builtinDomainMath`, `builtinDomainJSON`, `builtinDomainCrypto`, `builtinDomainOS`, `builtinDomainNetwork`, `builtinDomainRegex`, `builtinDomainDateTime`, `builtinDomainReactive`).
- **Implementación:**
  - `builtins_strings.go`: `len`, `upper`, `lower`, `trim`, `ltrim`, `rtrim`, `replace`, `split`, `join`, `contains`, `starts_with`, `ends_with`, `substr`, `index_of`, `pad_left`, `pad_right`, `repeat`, `reverse_str`, `slug`, `camel_case`, `snake_case`, `kebab_case`, `title_case`, `ucfirst`, `nl2br`, `strip_tags`, `html_escape`, `html_unescape`, `url_encode`, `url_decode`, `sprintf`, `chr`, `ord`.
  - `builtins_collections.go`: `count`, `push`, `pop`, `shift`, `unshift`, `slice`, `merge`, `keys`, `values`, `in_array`, `array_unique`, `array_reverse`, `array_filter`, `array_map`, `array_reduce`, `array_find`, `array_some`, `array_every`, `array_chunk`, `array_flatten`, `array_pluck`, `array_sum`, `sort`, `rsort`, `ksort`, `range_list`, `zip`, `first`, `last`, `is_empty`.
  - `builtins_numeric_types.go`: `int`, `float`, `decimal`, `string`, `bool`, `type_of`, `is_null`, `is_int`, `is_float`, `is_decimal`, `is_numeric`, `is_string`, `is_bool`, `is_array`, `is_map`, `is_object`, `is_callable`, `abs`, `min`, `max`, `round`, `floor`, `ceil`, `pow`, `sqrt`, ` sum`, `clamp`, `random_int`, `random_float`.
  - `builtins_system_crypto_net.go`: `json_encode`, `json_decode`, `base64_encode`, `base64_decode`, `md5`, `sha1`, `sha256`, `sha512`, `hmac_sha256`, `hash_password`, `verify_password`, `uuid_v4`, `random_bytes`, `random_hex`, `env`, `now`, `timestamp`, `timestamp_ms`, `date_format`, `strtotime`, `sleep`, `usleep`, `exit`, `die`, `file_exists`, `file_get_contents`, `file_put_contents`, `file_append`, `unlink`, `mkdir`, `rmdir`, `read_dir`, `is_dir`, `is_file`, `path_join`, `basename`, `dirname`, `ext`, `realpath`, `glob`, `copy_file`, `move_file`, `file_size`, `http_get`, `http_post`, `ip_to_int`, `int_to_ip`, `is_valid_ip`, `is_private_ip`, `cidr_match`, `regex_match`, `regex_match_all`, `regex_replace`, `regex_split`.

### 7.2 Métodos de tipos primitivos (`pkg/typesystem/primitive_methods.go` + `pkg/core/call_method.go`)

- Definidos canónicamente en `pkg/typesystem/primitive_methods.go` para `string`, `array`, `map`, `int`, `float`, `decimal` y `bool` (por ejemplo: `$str.upper()`, `$str.trim()`, `$arr.count()`, `$arr.push(...)`, `$map.keys()`).
- En runtime son despachados por `evaluatePrimitiveMethod` (`pkg/core/call_method.go`).

### 7.3 Clases nativas puras de librería estándar (`pkg/core/native.go`, `pkg/core/native_signatures.go`)

- `Math`, `JSON` / `Json`, `Str`, `UUID`, `Console`, `Stack`, `Queue`, `DateTime` (`pkg/core/datetime.go`), `Markdown`, `Zip`, `RegEx` (`pkg/core/regex.go`), `FileStream` (`pkg/core/filestream.go`), `Csv`, `Xml`, `Yaml`, `Validator`, `Sanitizer`, `Collection`, `Option`, `Result`.

> **Observación arquitectónica crítica sobre la Standard Library:**
> El 92% de los built-ins (`builtins_strings.go`, `builtins_numeric_types.go`, `builtins_system_crypto_net.go`) y de las clases nativas de datos (`Math`, `Json`, `Str`, `UUID`, `DateTime`, `RegEx`, `FileStream`, `Zip`, `Markdown`) **NO necesitan el AST ni el intérprete**. Reciben `(r *Runtime, args []interface{}) interface{}` únicamente porque la firma histórica de `NativeHandler` pasa `*Runtime` (y en la mayoría de los casos ni siquiera leen `r`, salvo funciones de orden superior como `array_map`/`array_filter` que invocan un callback, o `env()` que lee `r.Env`).

---

## 8. Host integrations

Las capacidades de infraestructura ("Host Integrations") están implementadas en Go sobre paquetes maduros del ecosistema y expuestas a Joss como clases nativas en `pkg/core` y `pkg/server`:

| Capacidad Host | Archivos de implementación canónica | Dependencias subyacentes | Cómo se invoca desde Joss |
|---|---|---|---|
| **GranDB & QueryBuilder** | `pkg/core/database.go`, `database_API.go`, `database_builder.go`, `database_dialect.go`, `database_schema.go`, `database_write.go` | `database/sql`, `modernc.org/sqlite`, `go-sql-driver/mysql`, `lib/pq`, `microsoft/go-mssqldb` | `GranDB::table("...")`, `GranDB::raw(...)`, `GranDB::transaction(...)`, métodos fluidos `where`, `join`, `get`, `first`, `insert`, `update`, `delete` |
| **Model ORM** | `pkg/core/model_orm.go`, `model_relations.go`, `model_scopes.go`, `model_hooks.go`, `model_serialization.go` | Compone el QueryBuilder de `GranDB` + metadata inmutable de clase (`classMetadataCache`) + hydrator | Subclases de `Model`: `User::where(...)`, `$user.save()`, `$user.delete()`, relaciones `belongsTo`, `hasMany`, `belongsToMany`, `with` |
| **HTTP Server & Router** | `pkg/server/server.go`, `handlers.go`, `response_writer.go`, `security.go` + `pkg/core/router_ dispatch.go`, `request_response.go`, `session.go` | `net/http`, `golang.org/x/time/rate`, `go-redis/v9` | `Router::get/post/put/delete`, `Router::group`, `Request::input/header/file`, `Response::json/redirect/download`, `Session::get/put/flash` |
| **WebSockets** | `pkg/server/ws.go`, `pkg/core/ws_client.go` | `github.com/gorilla/websocket` | `Router::ws("/ruta", fn($ws) { $ws.onMessage(...); $ws.onClose(...); })`, `$ws.send(...)`, `$ws.broadcast(...)` |
| **View & Templates** | `pkg/core/view.go`, `pkg/viewtemplate/parser.go`, `directive.go`, `pkg/core/assets.go` | `pkg/viewtemplate` + `pkg/parser` (en `evaluateViewExpression`) | `View::render("vista", $data)`, `View::share(...)`, `View::exists(...)` |
| **Auth, Security & Crypto** | `pkg/core/auth.go`, `auth_mfa.go`, `security.go`, `jwt.go`, `oauth.go` | `golang.org/x/crypto/bcrypt`, `crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/sha256`, `crypto/rand` | `Auth::attempt/user/check/logout`, `MFA::generateSecret/verify`, `JWT::encode/decode`, `Crypto::encrypt/decrypt`, `CSRF::token/verify` |
| **Network / Socket / HTTP / Mail / SFTP** | `pkg/core/http_client.go`, `mail.go`, `sftp.go`, `network.go` | `net`, `net/http`, `net/smtp`, ` crypto/tls`, `golang.org/x/crypto/ssh`, `github.com/pkg/sftp` | `Http::get/post/withHeaders`, `Mail::send`, `SFTP::connect`, `Socket` |
| **Plugins (`.jp` & Native Drivers)** | `pkg/core/plugin_loader.go`, `native_driver.go`, `pkg/pluginruntime`, `pkg/pluginpkg`, `pkg/bytecode` | `JOSSBC2Z` + C ABI dynamic library loading (`LoadLibrary`/`dlopen`) | `Plugin::load`, namespaces de plugins, drivers nativos con ownership contado (`Fork`/`Free`) |
| **Concurrencia (`Future`, `Channel`, `Cron`)** | `pkg/core/concurrency.go`, `cron.go` | Goroutines, `context.Context`, `chan interface{}` | `async { ... }`, `await`, `Channel`, `Cron::schedule` |

---

## 9. Dependencias del interpreter (Qué depende realmente del evaluador AST hoy)

Al auditar `pkg/core` y `pkg/server`, identificamos con precisión quirúrgica los **únicos 6 puntos** donde las capacidades de librería estándar o de infraestructura invocan directamente el evaluador AST (`evaluateExpression` / `executeStatement` / `ExecuteMethod`):

1. **Interpolación y directivas en plantillas HTML (`pkg/core/view.go:16-54`):**
   - `Runtime.evaluateViewExpression(expr string, data map[string]interface{})` instancia un `parser.NewLexer(expr)` y `parser.NewParser(l)` **en tiempo de renderizado** para cada expresión `{{ $expr }}`, `@if(...)` o `@foreach(...)` dentro de un archivo `.joss.html` y la evalúa con `r.evaluateExpression`.
   - *Cómo desacoplarlo para AOT:* Las plantillas `.joss.html` del proyecto pueden pre-analizarse y compilarse durante `joss build` a funciones IR/nativas registradas en una tabla de vistas compiladas (`CompiledViewRegistry`), o bien el motor de plantillas puede resolver accesos a datos (`$user.name`, `$items[0]`, filtros `@json`) mediante un evaluador de expresiones de datos acotado o callbacks compilados sin requerir el intérprete general de programas.
2. **Despacho de rutas y controladores HTTP (`pkg/server/handlers.go` y `pkg/core/router_dispatch.go`):**
   - Cuando `Router::get("/path", "HomeController@index")` o `Router::get("/path", fn($req) { ... })` atiende una petición, `pkg/server` busca la clase `"HomeController"` en `r.Classes` (mapa de `*parser.ClassStatement`) o evalúa el `*core.FunctionObject` (que contiene un `*parser.FunctionLiteral`).
   - *Cómo desacoplarlo para AOT:* Si `Callable` / `ControllerAction` se abstrae detrás de una interfaz o puntero de función `type NativeCallable func(ctx *ExecutionContext, thisVal Value, args []Value) (Value, error)`, tanto el intérprete (que envuelve `ExecuteMethod`) como el binario AOT (que registra las funciones compiladas de cada controlador y closure en una tabla de símbolos exportada) pueden usar exactamente el mismo `pkg/server`, el mismo `Router` y el mismo `Middleware` pipeline sin tocar el AST.
3. **Callbacks de WebSocket (`pkg/server/ws.go`):**
   - `$ws.onMessage(fn($msg) { ... })` y `$ws.onClose(fn() { ... })` guardan closures que `ws.go` invoca al recibir tramas.
   - *Cómo desacoplarlo para AOT:* Usando la misma abstracción `Callable` (función nativa o closure compilada con entorno capturado) en lugar de exigir `*parser.FunctionLiteral`.
4. **Callbacks de orden superior en Built-ins y ORM (`array_map`, `array_filter`, `array_reduce`, `GranDB::transaction(fn($db) { ... })`, scopes y hooks de `Model`):**
   - Invocan `r.invokeCallableValue(callback, args)` (`pkg/core/callable_dispatch.go`).
   - *Cómo desacoplarlo para AOT:* Si `invokeCallableValue` acepta un `CompiledClosure` (puntero a función compilada + struct de variables capturadas) además de `*FunctionObject`, todos los built-ins de colecciones y transacciones de `GranDB` funcionan sin cambios sobre código compilado AOT.
5. **Hidratación, Accessors (`get{Attr}Attribute`), Mutators (`set{Attr}Attribute`) y Relaciones de `Model` (`pkg/core/model_orm.go`, `pkg/core/model_relations.go`):**
   - `Model` inspecciona `classMetadata` (extraída hoy de literales AST de la clase en `pkg/core/model_metadata.go`) y llama a métodos de la subclase (`user.posts()`, `getFullNameAttribute`) vía `ExecuteMethod`.
   - *Cómo desacoplarlo para AOT:* El compilador ya tiene toda la metadata de la clase en `analyzer.ClassSymbol`; puede emitir el descriptor estático `ModelMetadata` en la tabla de tipos del binario y despachar los métodos de la subclase a través de la vtable/method table compilada.
6. **Plugins de bytecode `.jp` (`pkg/core/plugin_loader.go`):**
   - Los paquetes `.jp` contienen `JOSSBC2Z` (AST serializado).
   - *Cómo tratarlo en AOT:* Durante `joss build`, los plugins `.jp` presentes en `plugins/` ya son cargados por el proyecto; sus unidades AST pueden incorporarse directamente al `PreparedProgram` y compilarse a código máquina junto con el resto del programa (o rechazarse explícitamente si se intenta cargar dinámicamente un `.jp` externo en tiempo de ejecución sin compilación previa).

---

## 10. Dependencias de Go

Es fundamental distinguir entre **depender del intérprete AST de Joss** (prohibido en AOT) y **depender de servicios de bajo nivel del runtime/ecosistema Go cuando se enlaza el Host Runtime**:

| Componente | ¿Por qué depende hoy del runtime de Go? | ¿Se puede compilar sin el intérprete AST? |
|---|---|---|
| **Gestión de memoria (GC)** | Objetos, strings, slices y mapas en `pkg/core` y en `BackendStandalone` usan el recolector de basura de Go. En `joss_rt.c` se usa `malloc` sin liberar. | **Sí.** La gestión de memoria (sea el GC del runtime de Go o un allocator/RC/Boehm GC en C/LLVM) es ortogonal al intérprete AST. |
| **Drivers SQL (`GranDB`)** | `modernc.org/sqlite` (SQLite puro sin CGO), `go-sql-driver/mysql`, `lib/pq` y `microsoft/go-mssqldb` son implementaciones completas de los protocolos de red de cada motor escritas en Go sobre `database/sql`. | **Sí.** Pueden invocarse como librería estática/compartida compilada sin ningún AST ni intérprete. |
| **HTTP/TLS/WebSockets (`pkg/server`)** | Usa `net/http`, `crypto/tls`, `gorilla/websocket` y `go-redis/v9`. | **Sí.** Es infraestructura de red compilada; no requiere evaluar AST si los handlers son funciones compiladas. |
| **Aritmética exacta (`decimal`)** | Usa `github.com/shopspring/decimal`. | **Sí.** Es una librería de aritmética de punto fijo arbitrario que puede exponerse mediante funciones del runtime ABI (`joss_dec_add`, `joss_dec_mul`, etc.). |
| **Concurrencia (`Future`, `Channel`)** | Usa goroutines, `sync.Mutex`, `chan` y `context.WithCancel`. | **Sí.** Las primitivas `joss_future_spawn`, `joss_future_await`, `joss_chan_send/recv` pertenecen al runtime ABI. |

---

## 11. Capacidades realmente AOT hoy (Con evidencia en código)

En el estado actual del repositorio (`HEAD` = `e935150`), una característica es **realmente AOT** si pasa por `Parser -> Analyzer -> Lowerer (pkg/ir/lower.go) -> IRVerifier (pkg/ir/verifier.go) -> Backend (llvm.go o standalone.go)` produciendo un ejecutable sin AST ni intérprete.

### 11.1 Capacidades AOT operativas hoy

| Categoría | Capacidades soportadas | Evidencia exacta en el código |
|---|---|---|
| **Tipos escalares básicos** | `int` (`i64`), `float` (`f64`, parcial), `bool` (`i1`/`i64`), `string` (`i8*`/`string`), `void` | `pkg/ir/types.go:10-27`, `pkg/ir/lower.go:1100-1129` |
| **Literales** | `IntegerLiteral`, `FloatLiteral`, `Boolean`, `StringLiteral`, `NullLiteral` | `pkg/ir/lower.go:589-605` |
| **Variables locales y asignación** | Declaración local (`var $x = ...`, `int $x = ...`), declaración múltiple (`int $a, $b = 1, 2`), lectura y reasignación de locales en la misma función (`AllocaInst`, `LoadInst`, `StoreInst`) | `pkg/ir/lower.go:257-304`, `606-613`, `955-974` |
| **Aritmética entera y bits** | `+`, `-`, `*`, `/`, `%`, `<<`, `>>`, `&`, `\|`, `^`, unarios `-` y `!` sobre `int64` | `pkg/ir/lower.go:615-681`, `pkg/backend/native/llvm.go:205-248`, `standalone.go:397-452` |
| **Comparaciones** | `==`, `!=`, `<`, `<=`, `>`, `>=`, `<=>` (spaceship) sobre enteros | `pkg/ir/lower.go:644-662`, `llvm.go:230-248`, `standalone.go:424-444` |
| **Concatenación de strings** | `"a" + "b"` (cuando al menos un operando está tipado como `string` en `lowerInfixExpression`) -> `CallRuntimeInst("str_concat")` | `pkg/ir/lower.go:621-627`, `runtime/native/csrc/joss_rt.c:50-63`, `standalone.go:104` |
| **Control de flujo estructurado** | `if` / `else` (vía `TernaryExpression` con bloques o expresiones de valor), `guard`, `while`, `do ... while`, `break`, `continue`, `return` | `pkg/ir/lower.go:315-448`, `683-725` |
| **Funciones de usuario** | Declaración de funciones globales (`func nombre(...)`), paso de parámetros por valor (`int`, `string`, punteros a objetos/arrays/mapas) y recursión directa (`CallInst`) | `pkg/ir/lower.go:118-174`, `880-953` |
| **Clases de usuario (básico)** | Declaración de clases sin herencia compleja, instanciación `new Clase(...)` (`NewObjectInst`), inicialización de propiedades con literal por defecto, invocación de `Init` (`Clase_Init`), métodos de instancia (`Clase_metodo` con `$this` como primer parámetro) y lectura/escritura de campos `int` o `string` (`LoadFieldInst`, `StoreFieldInst`) | `pkg/ir/lower.go:176-237`, `726-807`, `976-995` |
| **Arrays y Mapas (solo `int` y `string` básicos)** | Creación (`NewArrayInst`, `NewMapInst`), lectura (`ArrayGetInst`, `MapGetInst`) y escritura (`ArraySetInst`, `MapSetInst`) limitada a `i64` y `string` (con el bug de `lowerIndexExpression` que fuerza `TypeI64` en lecturas) | `pkg/ir/lower.go:809-878`, `997-1029` |
| **Salida estándar básica** | `echo` / `print` de valores `string` (`joss_print_string`) e `int` (`joss_print_i64`) | `pkg/ir/lower.go:450-463` |
| **Excepciones básicas (solo en `BackendStandalone`)** | `throw` (`ThrowInst`) y `try { ... } catch ($e) { ... }` (`TryCatchInst`) funcionan en `standalone.go` (vía `panic`/`recover`), pero **fallan en `llvm.go:333`** | `pkg/ir/lower.go:465-575`, `standalone.go:286-369`, `llvm.go:332-334` |

---

## 12. Capacidades bloqueadas (`STATUS = BLOCKED`)

Todas las capacidades listadas a continuación tienen hoy **`STATUS = BLOCKED`** para compilación AOT real. Se detalla la causa exacta verificada en el código:

### 12.1 Constructos del lenguaje bloqueados en `pkg/ir/lower.go` y `pkg/backend/native`

| Capacidad | Estado | Ubicación exacta del bloqueo | Causa técnica raíz |
|---|---|---|---|
| **Variables globales y constantes de módulo** | `BLOCKED` | `pkg/ir/lower.go:606-613` (`lowerIdentifier`) | `lowerIdentifier` solo busca en `l.varLocals`. Aunque `lowerGlobalLet` agrega entradas a `l.prog.Globals`, nunca se emiten inicializadores globales en `main` ni instrucciones de carga/almacenamiento sobre `GlobalValue`. |
| **`foreach ($arr as $k => $v)`** | `BLOCKED` | `pkg/ir/lower.go:254`, `1137` (`ForeachStatement`) | Rechazado explícitamente con `JOSS-NATIVE-001`. Falta desazucarar (`desugar`) el bucle en un índice entero `0 .. joss_arr_len(arr)-1` (para arrays) o iterador de claves (para mapas) en el CFG del `Lowerer`. |
| **`enum` y acceso `Enum::CASO`** | `BLOCKED` | `pkg/ir/lower.go:252`, `1131` (`EnumStatement`) | Rechazado con `JOSS-NATIVE-001`. Los casos de enum no se bajan a constantes globales ni a literales `int`/`string`. |
| **`match ($x) { ... }` y `select`** | `BLOCKED` | `pkg/ir/lower.go:253`, `586`, `1134`, `1149` | Rechazados con `JOSS-NATIVE-001`. Falta bajar `MatchExpression` a una cadena de bloques básicos comparativos (`CompareInst` + `BranchTerminator`) con variable temporal de resultado. |
| **`defer`** | `BLOCKED` | `pkg/ir/lower.go:255`, `1140` (`DeferStatement`) | Rechazado con `JOSS-NATIVE-001`. El IR no modela una pila LIFO de bloques/closures diferidos al salir de la función o bloque `try`. |
| **Destructuring (`var [$a, $b] = ...`)** | `BLOCKED` | `pkg/ir/lower.go:256`, `1143` (`DestructureStatement`) | Rechazado con `JOSS-NATIVE-001`. Falta desazucarar en asignaciones indexadas `ArrayGetInst` / `MapGetInst`. |
| **Closures / `FunctionLiteral` (`fn($x) => ...`)** | `BLOCKED` | `pkg/ir/lower.go:586`, `1161` (`FunctionLiteral`) | Rechazado con `JOSS-NATIVE-001`. El `Lowerer` no realiza *lambda lifting* ni construye estructuras de entorno capturado (`ClosureFrame`). |
| **Operadores `??`, `?.`, `\|>`, `cout <<`, `$i++`/`$i--`, `is`** | `BLOCKED` | `pkg/ir/lower.go:586`, `1164-1168` | Caídos en el `default:` de `lowerExpression` (`JOSS-NATIVE-001`). No tienen traducción en `lowerExpression`. |
| **Interpolación de strings (`"Hola {$nombre}"`)** | `BLOCKED` | `pkg/ir/lower.go:599-600` (`StringLiteral`) | `lowerExpression` emite `NewConstString(e.Value)` literal sin expandir las subexpresiones `{$expr}` como sí hace `pkg/core/evaluator.go`. |
| **Parámetros por referencia (`ref T $x`) y argumentos con default/named** | `BLOCKED` | `pkg/ir/lower.go:880-953` (`lowerCallExpression`) | `lowerCallExpression` pasa los argumentos posicionalmente tal como vienen en el AST sin rellenar valores por defecto de `ResolvedCall.Signature`, sin reordenar argumentos nombrados y sin pasar punteros para parámetros `ref`. |
| **Herencia (`extends`, `super`), polimorfismo de interfaces (`implements`) y `record`** | `BLOCKED` | `pkg/ir/lower.go:176-237`, `891-904` | El despacho de métodos en `lowerCallExpression` es 100% estático por nombre (`ClassName + "_" + Method`). No existe tabla virtual (vtable) ni despacho dinámico cuando el receptor tiene tipo de superclase o interfaz. |
| **Tipo `decimal` y conversiones numéricas (`CastInst`)** | `BLOCKED` | `pkg/ir/lower.go:1100-1129`, `llvm.go:333` | `decimal` no tiene representación en `pkg/ir/types.go` (cae a `TypeI64`) y `CastInst` no está implementado ni en `lower.go` ni en `llvm.go` ni en `standalone.go`. |
| **Valor dinámico (`mixed`) y colecciones heterogéneas** | `BLOCKED` | `pkg/ir/lower.go:814-878`, `joss_rt.c:68-304` | El runtime nativo carece de un tipo `JossValue` etiquetado (*tagged union*) capaz de almacenar cualquier valor de Joss (`null`, `bool`, `int64`, `float64`, `decimal`, `string`, `array`, `map`, `object`, `callable`) y operar sobre él con semántica idéntica a `pkg/typesystem`. |
| **`try` / `catch` / `throw` en `BackendLLVM`** | `BLOCKED` | `pkg/backend/native/llvm.go:332-334` | `llvm.go` falla al encontrar `*ir.ThrowInst` o `*ir.TryCatchInst`. |

### 12.2 Librería estándar e integraciones Host bloqueadas en AOT

| Capacidad | Estado | Causa exacta del bloqueo |
|---|---|---|
| **Los 115 Built-ins globales (`len`, `count`, `trim`, `split`, `json_encode`, `sha256`, `env`, `now`, etc.)** | `BLOCKED` | `lowerCallExpression` (`pkg/ir/lower.go:880-953`) emite `CallInst("len", ...)` en vez de una llamada al runtime ABI. Al compilar con `BackendLLVM` o `BackendStandalone`, el enlazador falla con símbolo indefinido (`undefined: len` / `undefined reference to len`). |
| **Métodos de tipos primitivos (`$s.upper()`, `$arr.push()`, `$m.keys()`)** | `BLOCKED` | `lowerCallExpression` intenta buscar el nombre de clase como `"string_upper"` o `"_upper"`, que no existe como función en `prog.Functions`. |
| **Clases nativas de datos (`DateTime`, `Json`, `Math`, `Str`, `UUID`, `RegEx`, `FileStream`, `Zip`, `Markdown`)** | `BLOCKED` | Implementadas en `pkg/core/*.go`. No están conectadas a ningún despacho desde `pkg/ir` ni desde `pkg/backend/native`. |
| **`GranDB` y `Model` ORM** | `BLOCKED` | Implementados en `pkg/core/database*.go` y `pkg/core/model*.go`. El IR no distingue llamadas a clases nativas del host ni existe aún la frontera ABI hacia el Host Runtime. |
| **`Server` HTTP, `Router`, `Request`, `Response`, `Session`, `WebSocket`, `View`** | `BLOCKED` | Implementados en `pkg/server` y `pkg/core`. Dependen de `Runtime` y actualmente invocan controladores/closures y vistas a través del evaluador AST. |
| **Plugins `.jp` y `NativeDriver`** | `BLOCKED` | `LoadProject` no incorpora el AST de los plugins `.jp` a las unidades bajadas por `ir.Lowerer`, y `NativeDriver` vive en `pkg/core/native_driver.go`. |

---

## 13. Puntos de extensión y Unidad de Reutilización

Para cumplir simultáneamente con:
1. **Cero duplicación manual en C/C++/Rust** de la librería estándar y las integraciones host (`AGENTS.md` §3), y
2. **Prohibición absoluta de usar el intérprete AST en modo AOT** (`GOAL 2`),

debemos identificar cuál es la **unidad de reutilización arquitectónica correcta**.

### 13.1 El diagnóstico arquitectónico central: `pkg/core` mezcla dos capas distintas

Hoy `pkg/core` agrupa en el mismo paquete Go:
- **Capa A — El Intérprete AST (`AST Evaluator`):** `evaluator.go`, `evaluator_infix.go`, `statement.go`, `ExecutePrepared`. Esta capa recorre nodos `*parser.*` en tiempo de ejecución. **Esta capa NO debe existir ni ejecutarse en un binario AOT.**
- **Capa B — El Runtime Canónico de Servicios y Librería Estándar (`Canonical Runtime & Host Services`):**
  - Los 115 built-ins (`builtins_strings.go`, `builtins_collections.go`, `builtins_numeric_types.go`, `builtins_system_crypto_net.go`).
  - Las clases nativas (`DateTime`, `RegEx`, `FileStream`, `Math`, `Json`, `Str`, `UUID`, `Zip`, `Markdown`).
  - El motor ORM y SQL (`GranDB`, `database_*.go`, `model_*.go`).
  - Los servicios web y de seguridad (`Router`, `Request`, `Response`, `Session`, `Auth`, `MFA`, `JWT`, `Crypto`, `Http`, `Mail`, `SFTP`, `View`, `WebSocket` en `pkg/server`).

### 13.2 La Unidad de Reutilización Canónica

La unidad de reutilización entre el modo interpretado (`joss run` / `joss server`) y el modo compilado AOT (`joss build`) **no es el código C escrito a mano**, sino **el Runtime de Servicios y Host de Joss consumido a través de una ABI Nativa Estable (`Joss Native & Host ABI`) y un Despacho de Callables Compilados (`CompiledCallable`)**.

Específicamente:

1. **Separación entre Evaluador AST y Servicios del Runtime:**
   - Los built-ins, métodos primitivos, `GranDB`, `Model`, `Auth`, `Session`, `View` y `pkg/server` no necesitan un `*parser.BlockStatement` para funcionar. Solo necesitan:
     - Un **contexto de ejecución (`ExecutionContext` / `*Runtime` sin AST)** que contenga la configuración de entorno (`Env`), la conexión `DB`, la sesión HTTP actual y el pool de recursos.
     - Una interfaz uniforme de invocación de callbacks (`CallableInvoker`): en modo intérprete invoca al evaluador AST; en modo AOT invoca directamente el puntero a función nativa compilada (`func(ctx, thisVal, args) Value`).
2. **Dos niveles bien definidos en la ABI del compilador AOT:**
   - **Nivel 1 — Runtime Lingüístico de Bajo Nivel (`Language Runtime ABI`):**
     - Operaciones de alta frecuencia emitidas directamente por el compilador desde el IR: aritmética escalar (`int64`, `float64`, `bool`), `JossValue` etiquetado (`mixed`), strings, arrays, mapas, instanciación de objetos, acceso a campos por índice/nombre, tablas virtuales de métodos (vtables), closures (`fn_ptr` + `env_struct`) y manejo de excepciones (`try`/`catch`/`throw`/`defer`).
   - **Nivel 2 — Puente hacia la Librería Estándar e Integraciones Host (`Canonical Host Runtime Bridge`):**
     - `ir.Lowerer` consulta `PreparedProgram.ResolvedCalls[call]`:
       - Si es un built-in (`CallTargetBuiltin`) -> emite `CallBuiltinInst(builtinID, args)`.
       - Si es un método primitivo (`CallTargetPrimitiveMethod`) -> emite `CallPrimitiveMethodInst(typeKind, methodName, recv, args)`.
       - Si es una clase/método nativo del host (`GranDB`, `Router`, `View`, `DateTime`, `Auth`, etc.) -> emite `CallHostServiceInst(className, methodName, recv, args)`.
     - Estas tres instrucciones llaman directamente a las implementaciones canónicas de Go (ya sea enlazadas estáticamente en el binario final o a través de un puente C-ABI `c-archive`/`c-shared` generado desde el propio código Go canónico), **sin pasar jamás por el parser ni por el evaluador AST, y sin reescribir una sola línea de `GranDB`, `Server`, `Auth` o `builtins` en C**.

---

## 14. Alternativas arquitectónicas

Evaluamos con rigor técnico las tres arquitecturas planteadas frente a las restricciones del proyecto:

### Arquitectura 1: `Joss Source -> Typed AST -> Verified Joss IR -> Go Low-Level Codegen -> go build` (Enlazado directo con paquetes de servicios canónicos sin AST)

- **Cómo funciona:**
  - El código Joss del usuario se compila íntegramente a través de `Parser -> Analyzer -> Joss Native IR -> IRVerifier`.
  - El backend traduce el `ir.Program` verificado a código Go de bajo nivel (funciones estáticas tipadas, bloques básicos/CFG, structs/vtables para clases de usuario, sin incluir ni un solo nodo AST ni importar `pkg/parser`).
  - Para los built-ins e integraciones host (`GranDB`, `Server`, `Auth`, `View`), el código generado enlaza directamente con el paquete de servicios de runtime canónico de Joss (en el que los callbacks de rutas, middlewares, WebSockets y vistas apuntan a las funciones compiladas).
- **Ventajas:**
  - **0% duplicación:** Reutiliza el 100% de `GranDB`, `Model`, `Server`, `WebSocket`, `Auth`, `View`, `Decimal`, `Crypto` y los 115 built-ins desde su única fuente de verdad en Go.
  - **Binario único real y autónomo:** Produce un único ejecutable `.exe` / ELF estático (`CGO_ENABLED=0`), multiplataforma (`GOOS`/`GOARCH`), con GC maduro, TLS, HTTP/2 y drivers SQL completos para SQLite, MySQL, PostgreSQL y SQL Server.
  - **Sin intérprete AST:** Ninguna función del usuario se interpreta; todo el código Joss se baja a IR verificado y luego a funciones nativas compiladas por el backend.
- **Desventajas / Límites:**
  - Requiere que el compilador de Go esté disponible durante `joss build`.
  - Si no se complementa con un backend LLVM/C para el núcleo del lenguaje, no ejercita la emisión directa a LLVM IR.

### Arquitectura 2: `Joss Source -> Typed AST -> Verified Joss IR -> LLVM IR (.ll) -> clang/lld` con reimplementación nativa pura de todo el runtime

- **Cómo funciona:**
  - Todo el código Joss se baja a LLVM IR y se compila con `clang` contra un runtime C/C++/Rust propio.
- **Ventajas:**
  - Independencia total del toolchain y del runtime de Go en el ejecutable final.
- **Desventajas críticas (Por qué es INVIABLE como única vía para el ecosistema completo de Joss):**
  - Para que una aplicación real como `JosSecurity` compile y funcione en esta arquitectura pura sin enlazar Go, **habría que reescribir manualmente en C/C++/Rust** los 115 built-ins, el motor de plantillas `viewtemplate`, el ORM `Model`, los 4 dialectos SQL y drivers de `GranDB` (SQLite, MySQL, Postgres, SQL Server), el servidor HTTP, sesiones Redis, WebSockets, `bcrypt`, `AES-GCM`, `SFTP`, `SMTP`, `Zip` y `Markdown`.
  - Eso **viola frontalmente la Regla 2 de esta revisión y `AGENTS.md` §3** (*"PROHIBIDO duplicar manualmente en C/C++/Rust implementaciones que ya existan en los paquetes canónicos de Joss"*).

### Arquitectura 3: Arquitectura Híbrida de Doble Backend sobre IR Unificado + Puente Host Canónico (`Unified Typed IR + Native ABI + Canonical Host Bridge`)

```text
                             JOSS SOURCE (.joss + .joss.html)
                                            │
                                            ▼
                         Frontend Canónico (Parser + Analyzer)
                                            │
                                            ▼
                              PreparedProgram (Inmutable)
                                            │
                                            ▼
                       Joss Native IR (SSA/3AC Completo + Verifier)
             [Instrucciones de Lenguaje + CallBuiltin + CallHost + Closures]
                                            │
                   ┌────────────────────────┴────────────────────────┐
                   ▼                                                 ▼
      Ruta A: Backend LLVM (`--backend=llvm`)          Ruta B: Backend Nativo Integrado
      Emite LLVM IR (.ll) para todo el código          Emite código de bajo nivel desde el IR
      del usuario y enlaza con:                        (CFG/bloques/funciones compiladas, CERO AST)
      1) `joss_rt` (Runtime lingüístico C ABI)         y enlaza directamente con el Host Bridge
      2) `libjoss_host` (`go build -buildmode=         canónico (`GranDB`, `Server`, `Auth`, `View`,
         c-archive/c-shared` exportado desde           `Builtins`) en un único binario estático.
         los paquetes canónicos de Go sin AST)
```

- **Cómo funciona:**
  1. **Un único IR completo y verificado (`pkg/ir`):** Todo el lenguaje Joss (incluyendo `foreach`, `match`, `enum`, `defer`, closures con captura léxica, objetos/herencia/interfaces, `try`/`catch`/`finally`, interpolación de strings, referencias `ref`, built-ins y llamadas a servicios del host) se baja a **Joss Native IR** sin excepciones ni atajos. Ningún nodo AST sobrevive más allá de `pkg/ir/lower.go`.
  2. **Una única ABI de Runtime (`Joss Native ABI`):**
     - Define el contrato exacto de valores (`JossValue`), colecciones (`JossArray`, `JossMap`), objetos (`JossObject` + `ClassDescriptor`), closures (`JossClosure`) y llamadas al host (`joss_host_call_builtin`, `joss_host_call_method`, `joss_host_register_route`, `joss_host_register_view`).
  3. **Reutilización 100% canónica de la Standard Library y Host Integrations:**
     - En lugar de reescribir `GranDB`, `Server`, `WebSocket`, `Auth`, `View` o los 115 built-ins en C, se expone un paquete puente canónico en Go (`pkg/hostbridge` o interfaz limpia en `pkg/core`) que invoca las implementaciones existentes en Go usando valores del IR/ABI y callbacks compilados (sin usar jamás `evaluateExpression` ni `executeStatement`).
     - En el backend nativo integrado (y mediante `c-archive`/`c-shared` cuando se usa el backend LLVM con `clang`), el binario compilado ejecuta **código máquina compilado desde el IR para toda la lógica Joss** y llama al **Host Bridge canónico compilado desde Go para la librería estándar y las integraciones de infraestructura**.

### 14.1 Tabla comparativa de arquitecturas

| Criterio | Arquitectura 1 (IR -> Go bajo nivel) | Arquitectura 2 (LLVM + Reescritura en C) | Arquitectura 3 (IR Unificado + Native ABI + Host Bridge Canónico) |
|---|---|---|---|
| **Elimina el intérprete AST del binario AOT** | Sí | Sí | **Sí (100%)** |
| **Cumple `AGENTS.md` §3 (Cero duplicación manual en C)** | Sí | **NO (Viola la regla)** | **Sí (100%)** |
| **Soporta `GranDB`, `Model`, `Server`, `WS`, `Auth`, `View` reales** | Sí | No (requeriría años de reescritura y mantenimiento doble) | **Sí (reutiliza la fuente canónica)** |
| **Mantiene y fortalece `pkg/ir` + `IRVerifier` + `LLVMEmitter`** | Parcial (no obliga a usar LLVM) | Sí | **Sí (ambos backends consumen el mismo IR verificado)** |
| **Riesgo de divergencia semántica entre `joss run` y `joss build`** | Muy bajo | Crítico (dos implementaciones de todo) | **Mínimo (una sola stdlib y un solo ORM/Server)** |

---

## 15. Arquitectura recomendada

Se recomienda adoptar la **Arquitectura 3: IR Nativo Completo + Frontera `Joss Native ABI` + Puente Host Canónico (`Canonical Host Bridge`) sin Evaluador AST**.

### 15.1 Principios de diseño de la arquitectura recomendada

1. **Prohibición estricta del AST en tiempo de ejecución AOT:**
   - El binario producido por `joss build` no contendrá código fuente `.joss`, ni AST serializado (`JOSSBC2Z`), ni importará el evaluador de expresiones/statements en sus rutas de ejecución.
   - Toda función, método de clase, controlador HTTP, middleware, closure y expresión de vista `.joss.html` se transforma en una `*ir.Function` verificada por `ir.IRVerifier` en tiempo de compilación.
2. **Enriquecimiento de `pkg/ir` para preservar toda la semántica del `Analyzer`:**
   - Añadir representación explícita para valores dinámicos/etiquetados (`TypeKindMixed` / `JossValue`), `TypeKindDecimal`, globales inicializados, tablas de métodos de clase (`ClassMetadata` en `ir.Program`), descriptores de closures (`MakeClosureInst`, `CallClosureInst`), e instrucciones explícitas de frontera con el runtime:
     - `CallBuiltinInst` (para los 115 built-ins canónicos).
     - `CallPrimitiveMethodInst` (para métodos de `string`, `array`, `map`, numéricos).
     - `CallHostMethodInst` / `CallHostStaticInst` (para `GranDB`, `Model`, `Router`, `Request`, `Response`, `Session`, `Auth`, `View`, `DateTime`, `FileStream`, etc.).
3. **Desacoplamiento limpio dentro de `pkg/core` entre Evaluador AST y Servicios Host:**
   - Extraer la interfaz `CallableInvoker` en `pkg/core`:
     - En modo intérprete (`joss run` / `joss server`), `CallableInvoker` ejecuta el cuerpo AST (`*parser.MethodStatement` / `*parser.FunctionLiteral`).
     - En modo AOT (`joss build`), `CallableInvoker` invoca el puntero a función compilada registrado en la tabla de símbolos del `ir.Program`.
   - Pre-compilar las expresiones `{{ ... }}`, `@if(...)` y `@foreach(...)` de las plantillas `app/views/**/*.joss.html` durante `joss build` como funciones IR dedicadas (`__joss_view_expr_<id>`), de modo que `View::render` en modo AOT evalúe funciones nativas compiladas en lugar de llamar a `parser.NewParser` en runtime.
4. **Paridad de Backends (`BackendLLVM` + `BackendStandalone` / Host-Linked):**
   - Para programas de cómputo y lenguaje puro, `BackendLLVM` (`llvm.go` + `joss_rt.c`) y el backend nativo enlazado producen exactamente el mismo resultado a partir del mismo `*ir.Program`.
   - Cuando se enlaza con `libjoss_host` (en LLVM vía `c-archive` o en el backend nativo enlazado), el binario accede además a la totalidad de la librería estándar y las integraciones host (`GranDB`, `Server`, `WebSocket`, `Auth`, `View`) sin duplicar una sola línea en C.

---

## 16. Plan incremental de implementación

El plan se estructura en **8 etapas incrementales (`N0` a `N7`)**, donde cada etapa es verificable de manera aislada y mantiene todos los tests existentes en verde:

### Etapa `N0` — Saneamiento de la frontera IR y preservación de tipos del Analyzer
- **Objetivo:** Hacer que `pkg/ir/lower.go` consuma fielmente `PreparedProgram.ExprTypes` y `PreparedProgram.ResolvedCalls` sin colapsar tipos a `i64`.
- **Alcance técnico:**
  - Corregir `lowerIndexExpression`, `lowerMemberExpression`, `lowerArrayLiteral`, `lowerMapLiteral` y `lowerEchoStatement` para usar el tipo real de `PreparedProgram.ExpressionType(expr)`.
  - Corregir la resolución de constructores en `lowerNewExpression` consultando `PreparedProgram.Classes` en lugar del orden de aparición en `prog.Functions`.
  - Soportar variables y constantes globales (`GlobalValue` + función de inicialización de módulo `__joss_init_globals`).
  - Introducir el tipo dinámico etiquetado `JossValue` (`mixed`) en el runtime ABI para colecciones heterogéneas y retornos `mixed`.

### Etapa `N1` — Completitud del Lenguaje Base en `pkg/ir/lower.go`
- **Objetivo:** Eliminar los rechazos `JOSS-NATIVE-001` en constructos fundamentales del lenguaje.
- **Alcance técnico:**
  - Implementar en `pkg/ir/lower.go`:
    - `ForeachStatement` (sobre arrays y mapas).
    - `MatchExpression` y `EnumStatement`.
    - Interpolación de strings (`"Hola {$x}"` -> cadena de conversiones a string + `str_concat`).
    - Operadores `??` (null coalescing), `?.` (optional chaining), `|>` (pipeline), `$i++`/`$i--` (postfix), `cout << ...` (stream output) e `is` (type check).
    - `DestructureStatement` y `DeferStatement` (pila LIFO de acciones diferidas por función/bloque).
    - Enlace de argumentos por defecto, argumentos nombrados y parámetros `ref T $x` usando `ResolvedCall.Signature`.

### Etapa `N2` — Objetos Completos, Herencia, Interfaces, Records y Excepciones en IR + Backends
- **Objetivo:** Soportar POO completa y manejo de errores en la cadena nativa.
- **Alcance técnico:**
  - Emitir tablas de descriptores de clase (`ClassDescriptor` con mapa de campos, valores por defecto, superclase, interfaces implementadas y tabla de métodos virtuales).
  - Soportar `super`, polimorfismo por interfaz/superclase, `record` inmutables y bloques `try` / `catch (Tipo $e)` / `finally` tanto en el backend enlazado como en `llvm.go`.

### Etapa `N3` — Closures y Callables de Primera Clase (`Lambda Lifting` + `ClosureEnv`)
- **Objetivo:** Compilar `FunctionLiteral` (`fn($x) => ...`) a funciones nativas del IR con captura léxica.
- **Alcance técnico:**
  - Analizar las variables libres capturadas por cada `FunctionLiteral`.
  - Generar una `*ir.Function` anónima (`__closure_<id>(envPtr, args...)`) y una instrucción `MakeClosureInst` que empaqueta el puntero a función con el entorno capturado.
  - Soportar `CallClosureInst` para invocar closures desde código de usuario y desde callbacks de la librería estándar.

### Etapa `N4` — Puente Canónico a la Standard Library (115 Built-ins + Métodos Primitivos + Clases de Datos)
- **Objetivo:** Conectar los 115 built-ins, los métodos primitivos (`primitive_methods.go`) y las clases nativas de datos (`DateTime`, `Json`, `Math`, `Str`, `UUID`, `RegEx`, `FileStream`, `Zip`, `Markdown`) al código compilado AOT **reutilizando su implementación Go canónica sin duplicarla en C**.
- **Alcance técnico:**
  - Introducir `CallBuiltinInst`, `CallPrimitiveMethodInst` y `CallHostServiceInst` en `pkg/ir`.
  - Conectar el despacho de estas instrucciones con el catálogo canónico de `pkg/core/builtins.go` y `pkg/core/native.go`, pasando closures compiladas de `N3` a funciones como `array_map`, `array_filter` y `array_reduce`.

### Etapa `N5` — Pre-compilación AOT de Vistas y Desacoplamiento del Evaluador AST en `View`
- **Objetivo:** Eliminar `parser.NewParser` y `evaluateExpression` de `View::render` cuando se ejecuta en modo AOT.
- **Alcance técnico:**
  - Durante `joss build`, escanear `app/views/**/*.joss.html` con `pkg/viewtemplate`, extraer todas las expresiones `{{ ... }}`, `@if(...)`, `@foreach(...)`, analizarlas y bajarlas a funciones IR compiladas.
  - Hacer que `View::render` consuma las funciones de vista compiladas y los assets estáticos sin invocar jamás el lexer/parser/evaluador en runtime.

### Etapa `N6` — Puente Canónico de Infraestructura Host (`GranDB`, `Model` ORM, `Auth`, `Session`, `Server`, `Router`, `WebSocket`)
- **Objetivo:** Permitir que controladores MVC, modelos `Model`, consultas `GranDB`, rutas HTTP y handlers WebSocket compilados a código máquina sean invocados directamente por `pkg/server` y `pkg/core` sin intérprete AST.
- **Alcance técnico:**
  - Registrar en el binario AOT la tabla de clases compiladas (controladores y subclases de `Model` con su `ModelMetadata` estática) y sus métodos compilados.
  - Hacer que `Router::get/post/ws`, el pipeline de middlewares y los hooks/relaciones de `Model` despachen a través de `CallableInvoker` hacia las funciones nativas compiladas.

### Etapa `N7` — Compilación AOT Completa de Aplicaciones Reales (Caso de estudio: `JosSecurity`)
- **Objetivo:** Compilar y verificar una aplicación web completa (`ejemplos/Joss-Red-JosSecurity`) con `joss build` sin fallbacks al intérprete.
- **Alcance técnico:**
  - Incorporar unidades de plugins `.jp` en tiempo de compilación AOT.
  - Compilar `JosSecurity`, levantar el binario resultante y verificar respuestas HTTP, renderizado de vistas, sesiones, autenticación, WebSockets y consultas a base de datos sin presencia del evaluador AST.

---

## 17. Riesgos técnicos y de mantenimiento

1. **Riesgo de conversión de valores en la frontera (`Value Marshaling Overhead`):**
   - *Descripción:* Si cada operación aritmética o concatenación cruzara el Host Bridge convirtiendo entre `int64` y `interface{}`, el rendimiento se degradaría.
   - *Mitigación:* Mantener todas las operaciones escalares (`int64`, `float64`, `bool`), control de flujo, llamadas entre funciones de usuario y acceso a slots locales directamente en el código compilado del IR; cruzar al Host Bridge únicamente en llamadas a built-ins de librería estándar e I/O (`GranDB`, `HTTP`, `Filesystem`).
2. **Riesgo de captura mutable en Closures (`ref` y variables compartidas):**
   - *Descripción:* Si una closure modifica una variable local de su función contenedora o recibe un parámetro `ref`, una copia por valor en `ClosureEnv` produciría divergencia semántica respecto al intérprete.
   - *Mitigación:* Las variables capturadas que sufren mutación se alojan en una celda compartida en el heap (`BoxedCell`) referenciada tanto por el frame padre como por el struct `ClosureEnv`.
3. **Riesgo de preservación de semántica de excepciones (`RuntimeExceptionSignal` vs `ThrowInst`):**
   - *Descripción:* Cuando un servicio del Host (por ejemplo `GranDB` o un built-in con `JOSS-ARITH-003`) lanza una excepción de Joss, el bloque `try`/`catch` compilado en el código nativo debe poder interceptarla con su clase (`DatabaseException`, `TypeError`, etc.) y su mensaje/código.
   - *Mitigación:* Unificar el objeto de excepción entre el código compilado del IR y el Host Bridge para que `TryCatchInst` desempaquete tanto excepciones lanzadas por `ThrowInst` como excepciones originadas en built-ins/servicios host.
4. **Riesgo de detección de Toolchain C/LLVM en Windows (`compiler.go:19`):**
   - *Descripción:* Hoy `NewNativeCompiler()` solo busca `clang` en `PATH` e ignora `gcc`.
   - *Mitigación:* Si se usa un compilador C para el runtime de `BackendLLVM`, separar claramente la compilación de `.ll` (que requiere `clang`/`llc`) de la detección de `gcc`, e informar con precisión qué toolchain está presente en el host.

---

## 18. Pruebas necesarias

Toda implementación derivada de esta arquitectura deberá validarse mediante cuatro niveles de pruebas automatizadas:

1. **Tests unitarios de Lowering y Verificación de IR (`pkg/ir/lower_test.go`, `pkg/ir/verifier_test.go`):**
   - Un test por cada constructo del lenguaje (`foreach`, `match`, `enum`, `defer`, destructuring, interpolación, `??`, `?.`, `|>`, `ref`, closures, herencia, interfaces, `try`/`catch`/`finally`) verificando que `LowerProgram` genera las instrucciones esperadas y que `IRVerifier.Verify` pasa sin errores.
2. **Tests de Ausencia de Intérprete AST en Binarios AOT (`tests/native/no_ast_audit_test.go`):**
   - Inspección automatizada del código/símbolos generados por `joss build --trace` verificando que:
     - No se serializa ningún blob `JOSSBC2Z` ni código fuente `.joss`.
     - No se enlazan ni invocan `Runtime.Execute`, `Runtime.ExecutePrepared`, `Runtime.evaluateExpression` ni `Runtime.executeStatement`.
3. **Suite de Paridad Diferencial (`tests/native/differential_test.go`):**
   - Ejecución comparativa (`joss run` vs ejecutable compilado con `joss build`) verificando igualdad **byte a byte** de `stdout`, `stderr` y `exit code` para:
     - Aritmética, precedencia, coerción y `JOSS-ARITH-003`.
     - Strings, interpolación, arrays, mapas y métodos primitivos.
     - Clases, herencia, interfaces, constructores `Init`, `record`, genéricos y polimorfismo.
     - Closures con captura léxica, recursión y funciones de orden superior (`array_map`, `array_filter`, `array_reduce`).
     - Excepciones tipadas (`try`/`catch`/`finally`, `defer`).
     - Built-ins de stdlib (`json_encode`/`decode`, `DateTime`, `RegEx`, `Crypto`, `FileStream`).
     - Operaciones de `GranDB` y `Model` ORM sobre SQLite en memoria/archivo temporal (`t.TempDir()`).
4. **Test End-to-End de Aplicación Web Compilada AOT (`tests/native/web_aot_test.go`):**
   - Compilar una aplicación web MVC con `Router`, controlador, `Model`/`GranDB`, `Session`, `Auth`, `View::render` (`.joss.html`) y `WebSocket` mediante `joss build`, iniciar el binario resultante en un puerto efímero y verificar peticiones HTTP y WebSocket reales contra él.


---

## 19. Diseño e Implementación del Toolchain Nativo Autocontenido (`Joss Self-Contained Native Compiler`)

### 19.1 Eliminación total de compiladores externos en `joss build`

Se ha rediseñado e implementado `pkg/backend/native` para que `joss build archivo.joss` sea un **compilador nativo 100% autocontenido**:
- **Ningún archivo de `pkg/backend/native` importa `"os/exec"` ni invoca `go build`, `gcc`, `clang`, `cl`, `lld` ni `upx`** (protegido por la prueba de regresión `TestCleanEnvironment003_NoExternalToolchainInvocationRegression` en `tests/native/clean_environment_test.go`).
- Pipeline directo ejecutado íntegramente en memoria por `joss`:
  ```text
  archivo.joss
      ↓
  Lexer & Pratt Parser (pkg/parser)
      ↓
  Semantic Analyzer & PreparedProgram (pkg/analyzer)
      ↓
  Joss Native IR & IRVerifier (pkg/ir)
      ↓
  Integrated x86_64 Machine Code Generator + Embedded Native Joss Runtime (pkg/backend/native/machine_amd64.go)
      ↓
  Integrated PE32+ / ELF64 Linker & Executable Emitter (pkg/backend/native/pe_emitter.go, elf_emitter.go)
      ↓
  archivo.exe (Windows PE32+) / archivo (Linux ELF64)
  ```

### 19.2 Especificación de la Native ABI, Calling Convention y Memory Model

1. **Convención de llamadas (`Joss Native x86_64 Calling Convention`):**
   - Los primeros 4 argumentos (`int64`, `bool`, punteros a `string`, `JossObject`, `JossArray`, `JossMap`) se pasan en los registros `rcx`, `rdx`, `r8`, `r9`.
   - Los argumentos adicionales (`idx >= 4`) se colocan en la pila en `[rsp + 32 + (idx-4)*8]`.
   - Siempre se reservan 32 bytes de *shadow space* y el puntero de pila `rsp` se mantiene alineado a 16 bytes antes de toda instrucción `call`.
   - El valor de retorno se entrega en `rax`.
2. **Modelo de memoria y Layout del Runtime Nativo (`machine_amd64.go`):**
   - **Sección `.text` (RVA `0x1000`):** Contiene el entrypoint `_start`, la capa de abstracción de SO (`__joss_os_init`, `__joss_os_write`, `__joss_os_alloc`, `__joss_os_exit`), las rutinas en código máquina del Runtime Nativo de Joss (`__joss_rt_strlen`, `__joss_rt_strcmp`, `joss_str_concat`, `joss_print_string`, `joss_print_i64`, `joss_print_bool`, `joss_panic`, `joss_obj_new`, `joss_obj_set/get_field_*`, `joss_arr_new/push/get/set/len`, `joss_map_new/set/get_*`, `__joss_rt_throw`) y todas las funciones compiladas del programa.
   - **Sección `.rdata` / `.rodata`:** Contiene la tabla de importación de sistema operativo (`KERNEL32.dll` en Windows PE32+: únicamente `GetStdHandle`, `WriteFile`, `GetProcessHeap`, `HeapAlloc`, `ExitProcess`) seguida del pool inmutable de literales de cadena UTF-8 terminados en nulo.
   - **Sección `.data`:** Contiene los handles de E/S y heap del proceso y las pilas de marcos de excepción `DATA_try_rsp[64]`, `DATA_try_rbp[64]`, `DATA_try_rip[64]` para desenrollado nativo de pila en `try`/`catch`/`throw`.
3. **Estrategia de Enlazado Integrado (`pe_emitter.go` y `elf_emitter.go`):**
   - Resuelve en memoria todas las relocalizaciones `rel32` de saltos/llamadas entre bloques y funciones, accesos relativos a `RIP` hacia `.rdata` (literales de cadena e IAT) y hacia `.data` (variables globales y estado de excepciones).
   - En Windows (`windows-amd64`), emite directamente un contenedor **PE32+** (`0x8664`, Magic `0x020B`) de ~2.5 KB que depende exclusivamente de `KERNEL32.dll`.
   - En Linux (`linux-amd64`), emite directamente un contenedor **ELF64** estático (`EM_X86_64`) con 0 dependencias de librerías dinámicas (invocando `sys_write`, `sys_mmap` y `sys_exit` vía `syscall`).
   - Si se solicita un target aún no soportado por el generador integrado (p. ej. `darwin/arm64`), aborta explícitamente con `ERROR: native target not available` sin caer en fallbacks externos.

### 19.3 Validación `JOSS-CLEAN-ENVIRONMENT-001` y Auditoría de Binarios

Implementada y verificada en `tests/native/clean_environment_test.go`:
- **`TestCleanEnvironment001_JossBuildWithoutGoGCCClangMSVC`:** Ejecuta `joss build hello.joss --trace` con un `PATH` que contiene únicamente el binario `joss.exe` (sin `go`, `gcc`, `clang`, `cl`, `mingw32-make`, `cmake`, `ninja`), produce `hello.exe` y lo ejecuta en un directorio limpio con `PATH=""`, verificando la salida exacta `"30\nHola Joss Standalone"`.
- **`TestCleanEnvironment002_PEAndELFBinaryInspection_ZeroGoZeroInterpreter`:** Analiza los ejecutables generados con `debug/pe` y `debug/elf`, confirmando un tamaño < 32 KB (~2.5 KB reales), importación exclusiva de `KERNEL32.dll` en Windows y 0 librerías en Linux, y ausencia total de símbolos del runtime de Go (`go.buildid`, `runtime.main`, `runtime/cgo`) y del intérprete de Joss (`JOSSBC2Z`, `ExecutePrepared`, `evaluateExpression`).

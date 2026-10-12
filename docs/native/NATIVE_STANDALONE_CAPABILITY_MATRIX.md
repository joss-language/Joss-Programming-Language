# JOSS NATIVE STANDALONE CAPABILITY MATRIX (`joss build`)

Esta matriz documenta de forma rigurosa y verificable el estado de soporte del compilador nativo autónomo (`joss build`) tras la eliminación de toolchains externas (`go build`, `gcc`, `clang`, `msvc`, `ld`) y la introducción del generador directo de código máquina `x86_64` (`MachineCodeBuilder`) junto con los emisores ejecutables integrados `PE32+` y `ELF64`.

---

## 1. Clasificación de Estados

Cada capacidad del lenguaje y del runtime se clasifica estrictamente en uno de los siguientes estados:

| Estado | Significado Arquitectónico |
| :--- | :--- |
| **`NATIVE_STANDALONE`** | Compilado directamente a código máquina `x86_64` mediante `pkg/ir` → `MachineCodeBuilder` → `PE32+` / `ELF64`, ejecutado sin runtime de Go, sin intérprete AST y sin compiladores externos, y validado con pruebas diferenciales (`joss run` vs `joss build`). |
| **`PARTIAL`** | La característica compila y funciona de forma nativa autónoma para el subconjunto tipado estáticamente, pero presenta restricciones documentadas (p. ej. tipos compuestos heterogéneos no declarados o métodos dinámicos). |
| **`NOT_IMPLEMENTED`** | Característica válida en el lenguaje que aún no ha sido bajada a `pkg/ir` o `MachineCodeBuilder`; `joss build` falla explícitamente con `JOSS-NATIVE-001` sin realizar ningún fallback silencioso. |
| **`INTERPRETER_ONLY`** | Capacidad intrínseca a la evaluación dinámica de AST en memoria (`eval`, inspección reflexiva del intérprete o recarga en caliente de scripts). |

---

## 2. Matriz de Capacidades por Dominio

| Feature / Dominio | Estado Actual | Implementación en `joss build` | Evidencia / Test de Verificación | Limitación / Diagnóstico |
| :--- | :--- | :--- | :--- | :--- |
| **`int` (64-bit signed)** | `NATIVE_STANDALONE` | Registros GP de 64 bits (`RAX`, `RCX`, `RDX`, `R8`–`R15`), aritmética `add`, `sub`, `imul`, `idiv`, `cqo`, shifts `sal`/`sar` y bitwise `and`/`or`/`xor`. | `TestAuditRuntimeConformanceContracts`, `TestAuditIncrementalComplexityMatrix/Level_2` | Ninguna para enteros de 64 bits con signo. |
| **`bool`** | `NATIVE_STANDALONE` | Representación entera de 64 bits (`0` / `1`), instrucciones `test`, `cmp`, `sete`/`setne`/`setl`/`setle`/`setg`/`setge` y `joss_print_bool`. | `TestAuditRuntimeConformanceContracts`, `TestDifferential_BooleansAndLogic` | Ninguna. |
| **`string` (UTF-8)** | `NATIVE_STANDALONE` | Literales internados en `.rdata`/`.rodata` (null-terminated UTF-8); concatenación dinámica en heap vía `joss_str_concat`; comparación vía `__joss_rt_strcmp`; longitud vía `__joss_rt_strlen` (`strlen`). | `TestAuditRuntimeConformanceContracts`, `TestAuditIncrementalComplexityMatrix/Level_4` | Las cadenas concatenadas se alojan en el heap del proceso sin recolección intermedia. |
| **`float` / `decimal`** | `NOT_IMPLEMENTED` | Tipos reconocidos en IR (`TypeF64`), pero las operaciones SSE2/XMM y `decimal` de punto fijo aún no están emitidas en `MachineCodeBuilder`. | `TestCleanEnvironment004_NoGoFallbackAndExplicitJOSSNative001` | Emite error explícito de compilación nativa (`JOSS-NATIVE-001`). |
| **Control Flow (`? :`, `while`, `do-while`, `guard`, `break`, `continue`)** | `NATIVE_STANDALONE` | Bloques básicos SSA/CFG con `BranchTerminator` (`test rax, rax; je/jne`) y `JumpTerminator` (`jmp rel32`), incluyendo bloques de expresión `($cond) ? { ... } : { ... }`. | `TestAuditIncrementalComplexityMatrix/Level_2`, `TestDifferential_ControlFlow` | `foreach` aún no está desazucarado a bucle indexado en IR (`JOSS-NATIVE-001`). |
| **Functions & Recursion** | `NATIVE_STANDALONE` | Convención de llamada nativa (`RCX, RDX, R8, R9` + argumentos `>= 5` en stack `[RBP+48+...]`), frames `RBP` alineados a 16 bytes, recursión arbitraria. | `TestAuditCallingConventionAndStack`, `TestAuditIncrementalComplexityMatrix/Level_3` | Hasta 4 argumentos en registros y número arbitrario de argumentos adicionales en stack. |
| **Arrays (`T[]`)** | `NATIVE_STANDALONE` | Descriptor de 24 bytes en heap (`[0]=len`, `[8]=cap`, `[16]=dataPtr`), crecimiento geométrico dinámico vía `__joss_arr_ensure_cap`, indexación `joss_arr_get_*` / `joss_arr_set_*`, soporte para arrays anidados y arrays de objetos. | `TestAuditObjectsArraysMaps`, `TestAuditIncrementalComplexityMatrix/Level_5` | Capacidad inicial de 256 elementos con realocación automática cuando `idx >= cap`. |
| **Maps (`map`)** | `NATIVE_STANDALONE` | Estructura asociativa en heap (`joss_map_new`) con lista enlazada de nodos clave-valor (`24 bytes`: `[0]=keyPtr`, `[8]=val64`, `[16]=nextPtr`), búsqueda y actualización por `__joss_rt_strcmp`. | `TestAuditObjectsArraysMaps`, `TestAuditIncrementalComplexityMatrix/Level_5` | Complejidad de búsqueda $O(n)$ por número de claves en el mapa; claves tipadas como `string`. |
| **Classes & Objects (`class`, `new`, `$this`)** | `NATIVE_STANDALONE` | Instanciación dinámica vía `joss_obj_new(className)`, propiedades dinámicas/tipadas vía `joss_obj_get_field_*` y `joss_obj_set_field_*`, constructores `constructor`/`Init` y paso implícito de `$this` en `RCX`. | `TestAuditObjectsArraysMaps`, `TestAuditIncrementalComplexityMatrix/Level_6` | Resolución de métodos nominal en tiempo de compilación; despacho virtual polimórfico (`vtable`) pendiente para interfaces dinámicas. |
| **Inheritance (`extends`)** | `PARTIAL` | Resolución de propiedades heredadas en `lookupClassPropertyType` y métodos heredados cuando el tipo estático o la jerarquía nominal se resuelve en IR. | `TestAuditIncrementalComplexityMatrix/Level_6`, `TestDifferential_ClassesAndInheritance` | `super.method()` y polimorfismo dinámico de subtipo con `vtable` aún no emitidos. |
| **Exceptions (`try` / `catch` / `throw`)** | `NATIVE_STANDALONE` | Pila de marcos de excepción en `.data` (`DATA_try_depth`, `DATA_try_rsp[64]`, `DATA_try_rbp[64]`, `DATA_try_rip[64]`, `DATA_exception_val`) y desenrollado directo de pila en `__joss_rt_throw` a través de múltiples llamadas y recursión. | `TestAuditExceptionUnwinding`, `TestAuditIncrementalComplexityMatrix/Level_7` | Profundidad máxima de `try` anidados: 64 niveles por hilo principal. |
| **Closures (`func(...) { ... }`)** | `PARTIAL` | Funciones anónimas sin captura léxica se elevan a funciones top-level `__closure_N` en IR; closures con captura de entorno léxico (`upvalues`) o invocación indirecta de variables callable no están bajadas a `MachineCodeBuilder`. | `TestAuditIncrementalComplexityMatrix/Level_8` | Closures con captura léxica o despacho indirecto fallan explícitamente con `JOSS-NATIVE-001`. |
| **Generics (`Class<T>`)** | `PARTIAL` | Validación semántica completa en `pkg/analyzer`; instanciación nominal soportada cuando las propiedades usan representación de 64 bits (`int`, punteros, objetos). | `TestLowerer_SupportsClassesAndArrays` | Monomorfización especializada por parámetro de tipo aún no implementada en IR. |
| **Modules / Multi-file (`Import`)** | `PARTIAL` | El analizador y `ir.Lowerer` recorren todas las `SourceUnit` alcanzables del `PreparedProgram` y emiten sus clases y funciones en el mismo binario nativo. | `TestCleanEnvironment001_JossBuildWithoutGoGCCClangMSVC` | Módulos que importan servicios de host no nativos (`Server`, `GranDB`) son rechazados con `JOSS-NATIVE-001`. |
| **Filesystem & CLI Env** | `NOT_IMPLEMENTED` | Actualmente el runtime nativo sólo enlaza primitivas de consola (`GetStdHandle`/`WriteFile` en Windows, `sys_write` en Linux). | `TestAuditPE32PlusAndELF64Structure` | `File`, `Env` y `Args` requieren ampliar la tabla de importaciones OS (`CreateFileA`/`ReadFile` / `sys_open`/`sys_read`). |
| **HTTP / Router / Controller / View Templates** | `NOT_IMPLEMENTED` | Subsistema residente en `pkg/server` y `pkg/viewtemplate` sobre el runtime de Go; no existe todavía pila de sockets nativa (`ws2_32.dll` / `sys_socket`) en `MachineCodeBuilder`. | `TestAuditIncrementalComplexityMatrix/Level_9_10` | Rechazo explícito con `JOSS-NATIVE-001` sin fallback al runner Go. |
| **WebSocket** | `NOT_IMPLEMENTED` | Subsistema residente en `pkg/server` (`gorilla/websocket`). | `TestAuditIncrementalComplexityMatrix/Level_9_10` | Rechazo explícito con `JOSS-NATIVE-001`. |
| **GranDB / Model ORM** | `NOT_IMPLEMENTED` | Subsistema residente en `pkg/core` sobre `database/sql` y drivers de Go. | `TestAuditIncrementalComplexityMatrix/Level_9_10` | Rechazo explícito con `JOSS-NATIVE-001`. |
| **Plugins (`.jp` / `JOSSBC2Z`)** | `INTERPRETER_ONLY` | El formato `.jp` almacena AST serializado (`JOSSBC2Z`), prohibido por contrato en ejecutables nativos standalone. | `TestAuditZeroExternalProcessesAndZeroGoRuntime` | Prohibido en modo AOT standalone; requiere compilación estática de código fuente o ABI nativa de plugins. |
| **Concurrency (`async` / `await` / `Future` / `select`)** | `NOT_IMPLEMENTED` | Basado en goroutines y `context.Context` en el intérprete (`pkg/core`). | `TestAuditIncrementalComplexityMatrix/Level_8` | Rechazo explícito con `JOSS-NATIVE-001`. |

---

## 3. Niveles de Complejidad Verificados (Sección 13)

| Nivel | Descripción | Estado en `joss build` | Paridad Diferencial (`joss run` == `joss build`) |
| :--- | :--- | :--- | :--- |
| **Nivel 1** | CLI mínimo (`echo "Hola Mundo"`) | `NATIVE_STANDALONE` | **PASS** (100% idéntico byte a byte) |
| **Nivel 2** | Aritmética, lógica booleana, bitwise, ternario y bucles `while` | `NATIVE_STANDALONE` | **PASS** (100% idéntico byte a byte) |
| **Nivel 3** | Funciones, recursión profunda y convención de llamada (`> 4` parámetros) | `NATIVE_STANDALONE` | **PASS** (100% idéntico byte a byte) |
| **Nivel 4** | Strings (`+`, `==`, `!=`, `strlen`, UTF-8, cadenas vacías y largas) | `NATIVE_STANDALONE` | **PASS** (100% idéntico byte a byte) |
| **Nivel 5** | Arrays dinámicos (`> 256` elementos, anidados) y Maps (`string` -> valor) | `NATIVE_STANDALONE` | **PASS** (100% idéntico byte a byte) |
| **Nivel 6** | Clases, constructores, métodos, propiedades, aliasing y herencia nominal | `NATIVE_STANDALONE` | **PASS** (100% idéntico byte a byte) |
| **Nivel 7** | Excepciones (`try` / `catch` / `throw`) a través de funciones, métodos y recursión | `NATIVE_STANDALONE` | **PASS** (100% idéntico byte a byte) |
| **Nivel 8** | Iteradores dinámicos (`foreach`) y closures con captura léxica | `NOT_IMPLEMENTED` | **PASS** (Falla explícitamente con `JOSS-NATIVE-001`, sin fallback) |
| **Nivel 9** | Servicios de host (`GranDB`, `File`, `Env`, concurrencia) | `NOT_IMPLEMENTED` | **PASS** (Falla explícitamente con `JOSS-NATIVE-001`, sin fallback) |
| **Nivel 10** | Aplicación web completa (`JosSecurity`: HTTP + MVC + GranDB + WebSockets + Plugins) | `NOT_IMPLEMENTED` | **PASS** (Falla explícitamente con `JOSS-NATIVE-001`, sin fallback) |

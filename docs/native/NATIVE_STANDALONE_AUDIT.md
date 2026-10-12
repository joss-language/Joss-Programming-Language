# JOSS NATIVE STANDALONE COMPILER — DEEP ARCHITECTURAL & CONFORMANCE AUDIT

**Archivo:** `docs/native/NATIVE_STANDALONE_AUDIT.md`  
**Alcance:** Auditoría técnica exhaustiva del compilador nativo autónomo (`joss build`) tras el reemplazo de toolchains externas (`go build`, `gcc`, `clang`, `msvc`, `ld`) por la cadena interna:

```text
Joss Source (.joss)
    ↓
Lexer & Pratt Parser (pkg/parser)
    ↓
Semantic Analyzer (pkg/analyzer → PreparedProgram)
    ↓
Joss Native IR & Verifier (pkg/ir → Lowerer + Verifier)
    ↓
x86_64 MachineCodeBuilder + Embedded Native Runtime (pkg/backend/native/machine_amd64.go)
    ↓
Integrated Executable Emitters: PE32+ (pe_emitter.go) & Static ELF64 (elf_emitter.go)
    ↓
Standalone Native Executable (.exe / ELF64)
```

---

## 1. Pipeline Real de `joss build`

El comando oficial de compilación nativa de Joss es exclusivamente:

```bash
joss build archivo.joss
```

No existe ningún modo alterno ni flag `--native` requerido para producir un ejecutable nativo real. El flujo exacto verificado en código es:

1. **`cmd/joss/main.go` (`executeNativeBuildCommand`)**:
   - Invoca el lexer y parser Pratt (`pkg/parser`) sobre el archivo de entrada y las unidades fuente del proyecto.
   - Ejecuta el pipeline semántico canónico (`pkg/analyzer.PrepareProgram`: `collectDeclarations → projectScope → validateNominalContracts → analyzeSourceBodies`). Si existen diagnósticos de error semántico, aborta antes de tocar el backend.
2. **`pkg/ir/lower.go` (`Lowerer.LowerProgram`)**:
   - Transforma el `PreparedProgram` validado en un módulo SSA/3AC (`*ir.Program`).
   - Si encuentra un nodo AST o capacidad aún no soportada en nativo, devuelve inmediatamente `*ir.UnsupportedCapabilityError` formateado con el código estable **`JOSS-NATIVE-001`**, sin delegar jamás al intérprete ni a un runner Go.
3. **`pkg/ir/verifier.go` (`Verifier.Verify`)**:
   - Verifica la integridad estructural de cada `Function` y `BasicBlock`: terminadores obligatorios al final de cada bloque, ausencia de instrucciones después de terminadores y coherencia de tipos en operandos.
4. **`pkg/backend/native/driver.go` (`BuildProgram`)**:
   - Resuelve el target (`windows/amd64` o `linux/amd64`). Si el target solicitado no está implementado (p. ej. `darwin/arm64`), aborta con:
     ```text
     ERROR: native target not available: <os>/<arch>
     ```
   - Invoca `CompileIRToMachineCode(prog, target.OS)` en `pkg/backend/native/machine_amd64.go`.
   - Empaqueta las secciones `.text`, `.rdata`/`.rodata` y `.data` directamente mediante `EmitPE32PlusExecutable` (`pe_emitter.go`) o `EmitELF64Executable` (`elf_emitter.go`) y escribe el binario final a disco con `os.WriteFile`.

---

## 2. Verificación de Independencia respecto de Go, GCC, Clang, MSVC y el Intérprete

### 2.1 Cero Invocaciones de Procesos Externos (`pkg/backend/native`)
Mediante análisis estático del AST de Go (`TestAuditZeroExternalProcessesAndZeroGoRuntime` y `TestCleanEnvironment003_NoExternalToolchainInvocationRegression`), se comprobó que ningún archivo de producción en `pkg/backend/native` importa `"os/exec"` ni invoca:
- `exec.Command` / `exec.CommandContext` / `exec.LookPath`
- `os.StartProcess`
- `syscall.Exec` / `syscall.ForkExec` / `syscall.StartProcess` / `syscall.CreateProcess`
- `go build`, `gcc`, `clang`, `cl.exe`, `ld`, `lld`, `link.exe`, `upx`

### 2.2 Prueba en Entorno Limpio (`JOSS-CLEAN-ENVIRONMENT-001`)
El test `TestCleanEnvironment001_JossBuildWithoutGoGCCClangMSVC` construye `joss.exe` en un directorio temporal aislado y ejecuta `joss build hello.joss --trace` con:
- `PATH=<directorio-solo-con-joss.exe>` (sin `go`, `gcc`, `clang`, `cl`, `ld`, `cmake`, `ninja`)
- `GOROOT=""`, `GOPATH=""`, `CC=""`, `CXX=""`

Posteriormente copia `hello.exe` a un segundo directorio completamente vacío y lo ejecuta con `PATH=""`, comprobando que el ejecutable corre autónomamente sobre el sistema operativo.

### 2.3 Ausencia Total de Runtime Go y Bytecode AST en el Binario Generado
El test `TestAuditZeroExternalProcessesAndZeroGoRuntime` escanea byte a byte los ejecutables `PE32+` y `ELF64` generados y confirma **0 ocurrencias** de:
- `go1.`, `go:buildid`, `Go build ID:`
- `runtime.main`, `runtime.gopark`, `runtime.gopanic`, `runtime/cgo`
- Magics de `pclntab` de Go (`0xFFFFFFFB`, `0xFFFFFFFA`, `0xFFFFFFF0`, `0xFFFFFFF1`)
- Marcadores del intérprete o bytecode AST (`JOSSBC2Z`, `JOSS_STANDALONE_PAYLOAD`, `ExecutePrepared`, `evaluateExpression`)

---

## 3. Auditoría de `MachineCodeBuilder` (`pkg/backend/native/machine_amd64.go`)

### 3.1 Modelo de Generación de Código Máquina
`MachineCodeBuilder` emite directamente bytes de instrucciones `x86_64` en un buffer `.text`, manteniendo:
- Tabla de etiquetas simbólicas (`labels map[string]int`) para funciones de usuario (`fn_<name>`), métodos de clase (`fn_<Class>_<method>`), rutinas del runtime (`__joss_*`, `joss_*`) y bloques básicos de control de flujo.
- Lista de parches relativos de 32 bits (`rel32Patch`) para saltos condicionales (`0F 84` `je`, `0F 85` `jne`, `0F 8C` `jl`, `0F 8D` `jge`, `0F 8E` `jle`), saltos incondicionales (`E9` `jmp rel32`), llamadas (`E8` `call rel32`) y direccionamiento RIP-relative (`48 8D 05/0D/15 disp32` `lea rax/rcx/rdx, [rip + disp32]`).
- Internamiento determinista de literales de cadena UTF-8 terminados en `\x00` dentro de `.rdata`/`.rodata` (`stringOffsets`).

### 3.2 Convención de Llamada y Layout de Stack Frame
`MachineCodeBuilder` utiliza una convención de llamada unificada basada en la ABI de 64 bits (`RCX, RDX, R8, R9` + stack) tanto en Windows como en Linux (en Linux, las únicas fronteras con el kernel son las instrucciones `syscall` encapsuladas en `__joss_os_write`, `__joss_os_alloc` y `__joss_os_exit`, las cuales traducen los registros al contrato `rax/rdi/rsi/rdx/r10/r8/r9` del kernel Linux):

- **Prólogo de función**:
  ```asm
  push rbp
  mov  rbp, rsp
  sub  rsp, <frameSize>   ; múltiplo de 16 bytes (incluye 32 bytes de shadow space)
  ```
- **Alineación de Stack**: `push rbp` deja `RSP` alineado a 16 bytes (`ENTRY_RSP - 8` tras `CALL`, menos 8 por `push rbp` = múltiplo de 16). Como `frameSize` se redondea siempre a múltiplo de 16 bytes (`(slots*8 + 32 + 15) & ^15`), `RSP` permanece estrictamente alineado a 16 bytes antes de cualquier instrucción `CALL`.
- **Paso de `> 4` argumentos (`emitCallWithArgs`)**:
  - Argumentos 0..3 se cargan en `RCX`, `RDX`, `R8`, `R9`.
  - Argumentos `>= 4` (`k = 4..N-1`) se evalúan y empujan en orden inverso en el stack después de reservar 32 bytes de shadow space, más 8 bytes de padding si el número de argumentos en stack es impar, garantizando `RSP % 16 == 0` en el punto de `CALL`.
  - En el prólogo de la función receptora, los parámetros `i >= 4` se leen desde `[RBP + 48 + (i-4)*8]` y se copian a su slot local `[RBP - offset]`.
- **Verificación ejecutada**: `TestAuditCallingConventionAndStack` prueba funciones de 6 y 8 parámetros que realizan I/O y alojamiento en heap dentro de su cuerpo, métodos de clase con 6 parámetros explícitos + `$this` (7 argumentos ABI) y recursión de profundidad 50 (`triSum(50)`).

---

## 4. Auditoría del Native Runtime Embebido (`machine_amd64.go`)

Cada símbolo emitido por `emitRuntime()` en `pkg/backend/native/machine_amd64.go` fue auditado instrucción por instrucción:

| Símbolo Nativo | Rol y Contrato | Estado / Límites |
| :--- | :--- | :--- |
| `_start` | Punto de entrada del proceso (`sub rsp, 40`, inicializa heap/stdout en Windows, invoca `fn_main`, pasa el código de retorno a `__joss_os_exit`). | Completo. |
| `__joss_os_write` | Escritura a `stdout` (`rcx=0`) o `stderr` (`rcx=1`). En Windows invoca `GetStdHandle(-11/-12)` + `WriteFile`; en Linux ejecuta `syscall` `sys_write` (`rax=1`). | Completo. |
| `__joss_os_alloc` | Reserva de memoria en heap inicializada a cero. En Windows invoca `GetProcessHeap()` + `HeapAlloc(hHeap, HEAP_ZERO_MEMORY=8, size)`; en Linux ejecuta `sys_mmap` (`rax=9`, `PROT_READ\|PROT_WRITE`, `MAP_PRIVATE\|MAP_ANONYMOUS`). | Completo (sin liberación individual durante el proceso). |
| `__joss_os_exit` | Terminación inmediata del proceso con código de salida en `RCX`. En Windows invoca `ExitProcess`; en Linux ejecuta `sys_exit` (`rax=60`). | Completo. |
| `__joss_rt_strlen` / `fn_strlen` | Calcula longitud en bytes de cadena UTF-8 terminada en `\x00`. Retorna `0` si el puntero es `NULL`. | Completo. |
| `__joss_rt_strcmp` | Comparación lexicográfica byte a byte de dos cadenas (`0` si son iguales, diferencia con signo en caso contrario). Maneja punteros idénticos y `NULL`. | Completo. |
| `__joss_rt_memcpy` | Copia `R8` bytes desde `RDX` hacia `RCX`. | Completo. |
| `joss_str_concat` | Calcula `len(a)` y `len(b)`, aloja `len(a)+len(b)+1` bytes vía `__joss_os_alloc`, copia ambos operandos y escribe `\x00` final. | Probado con `"" . "x"`, `"x" . ""`, `"" . ""`, UTF-8 multibyte y cadenas `> 1 KB`. |
| `joss_print_string` | Escribe cadena + `"\n"` en `stdout`. | Completo. |
| `joss_print_i64` | Convierte entero de 64 bits con signo (incluyendo `0` y negativos de 64 bits) a ASCII decimal en buffer de stack y escribe línea en `stdout`. | Completo. |
| `joss_print_bool` | Imprime `"true"` o `"false"` según valor booleano. | Completo. |
| `joss_panic` | Escribe `"Panic: <msg>\n"` en `stderr` y termina con `exit(1)`. | Completo. |
| `joss_obj_new` / `joss_map_new` | Aloja descriptor de 16 bytes (`[0]=classNamePtr`, `[8]=headNodePtr`). | Completo. |
| `joss_obj_set_field_*` / `joss_map_set_*` | Busca clave por `__joss_rt_strcmp` en la lista enlazada de propiedades (`[0]=keyPtr`, `[8]=val64`, `[16]=nextPtr`); actualiza in-place si existe o inserta nuevo nodo de 24 bytes al inicio. | Complejidad $O(n)$ por número de campos/claves. |
| `joss_obj_get_field_*` / `joss_map_get_*` | Busca clave por `__joss_rt_strcmp` y devuelve el valor de 64 bits (`0` o `""` si no existe). | Completo. |
| `joss_arr_new` | Aloja descriptor de 24 bytes (`[0]=len`, `[8]=cap`, `[16]=dataPtr`) con capacidad inicial mínima de 256 elementos (`2048` bytes). | Completo. |
| `__joss_arr_ensure_cap` | Realoca dinámicamente `dataPtr` duplicando `cap` (`newCap = max(oldCap*2, minCap*2)`), copiando los `len` elementos existentes cuando `len >= cap` o `idx >= cap`. | Probado con arrays de 600 elementos (`TestAuditObjectsArraysMaps`). |
| `joss_arr_push_*` / `fn_array_push` / `fn_append` | Verifica capacidad vía `__joss_arr_ensure_cap`, inserta en `dataPtr[len]`, incrementa `len` y retorna `arrPtr` en `RAX`. | Completo. |
| `joss_arr_get_*` / `joss_arr_set_*` | Lectura con verificación de límites (`0 <= idx < len`) y escritura con expansión automática de capacidad y actualización de `len`. | Completo. |
| `joss_arr_len` / `fn_len` / `fn_count` | Retorna `[arrPtr + 0]` (`0` si `arrPtr == NULL`). | Completo. |
| `__joss_rt_throw` | Si `DATA_try_depth > 0`, decrementa profundidad, guarda el valor lanzado en `DATA_exception_val`, restaura `RSP` y `RBP` desde `DATA_try_rsp`/`DATA_try_rbp` y salta a `DATA_try_rip`; si no hay `try` activo, invoca `joss_panic`. | Soporta hasta 64 bloques `try` anidados. |

---

## 5. Contratos de Conformidad del Runtime (Sección 5)

Se implementó y ejecutó `TestAuditRuntimeConformanceContracts` en `tests/native/audit_conformance_test.go`, verificando paridad diferencial exacta entre `joss run` y `joss build` para:
- **`joss_str_concat`**:
  - `"hello" . "world"` → `"helloworld"`
  - `"" . "x"` → `"x"`
  - `"x" . ""` → `"x"`
  - `"" . ""` → `""`
  - Cadenas Unicode UTF-8 multibyte: `"¡Hola, " . "Joss ÑÁÉÍÓÚ 🚀!"` (verificando tanto salida UTF-8 como `strlen` exacto en bytes).
  - Cadenas grandes (`128` concatenaciones sucesivas en bucle `while` = `1024` bytes).
- **`__joss_rt_strcmp`**: Igualdad y desigualdad contra cadenas idénticas, prefijos, cadenas distintas y cadena vacía.
- **`joss_print_i64`**: `0`, `-1`, `-9876543210`, `9876543210`.

---

## 6. Relación con `pkg/core` (Clasificación Arquitectónica A–F)

Para garantizar el principio de **Fuente de Verdad Canónica (`AGENTS.md`)** sin duplicar librerías de alto nivel en C ni incrustar el intérprete de Go, cada componente de `pkg/core` se clasifica en una de las 6 categorías:

| Componente en `pkg/core` | Categoría | Justificación Arquitectónica |
| :--- | :--- | :--- |
| Operadores aritméticos, relacionales, lógicos, `echo`, `strlen`, `len`, `count`, `array_push`, `append` | **A — Native Runtime Primitive** | Primitivas fundamentales de ejecución emitidas directamente como instrucciones máquina o rutinas mínimas en `machine_amd64.go`. |
| Reglas de tipos, precedencia, visibilidad, constructores, herencia nominal y contratos de retorno | **B — Language Semantic Contract** | Resueltos por `pkg/parser`, `pkg/typesystem` y `pkg/analyzer` antes de `pkg/ir`; compartidos al 100% entre `joss run` y `joss build`. |
| `eval`, AST walking (`evaluateExpression`), frames interpretados (`CallMethodEvaluated`), bytecode `JOSSBC2Z` | **C — Interpreter-Only Mechanism** | Exclusivos de `joss run` y del sistema de plugins `.jp`; prohibidos en binarios nativos AOT. |
| `Str`, `Math`, `JSON`, `UUID`, `Markdown`, `Zip`, `Console`, `Stack`, `Queue`, `GranDB`, `Model` | **D — Standard Library / Host Runtime** | Actualmente implementados en Go dentro de `pkg/core`. No deben reescribirse manualmente en C; su futura disponibilidad en AOT requiere compilar implementaciones escritas en propio Joss o vincular primitivas de sistema declarativas. |
| Consola (`stdout`/`stderr`), memoria virtual (`HeapAlloc`/`mmap`), terminación (`ExitProcess`/`sys_exit`), y futuros archivos/sockets | **E — OS Integration Layer** | Única capa dependiente del sistema operativo (5 funciones de `KERNEL32.dll` en Windows; 3 syscalls en Linux). |
| Antiguo `runtime/native/csrc/joss_rt.c` y `c_emitter.go` (eliminados) | **F — Duplicated Implementation (Eliminated)** | Revertidos y eliminados permanentemente para evitar divergencia semántica y dependencia de GCC/Clang. |

---

## 7. Auditoría del Modelo de Memoria (Sección 8)

- **Stack**: Variables locales (`AllocaInst`) y temporales SSA (`TempValue`) residen en el marco de pila de cada función (`[RBP - offset]`) y se liberan automáticamente en el epílogo (`mov rsp, rbp; pop rbp; ret`).
- **Segmento de Constantes (`.rdata` / `.rodata`)**: Los literales de cadena y nombres de clase/propiedad se internan una sola vez en memoria de sólo lectura.
- **Heap (`__joss_os_alloc`)**: Las cadenas concatenadas (`joss_str_concat`), descriptores y buffers de arrays (`joss_arr_new`, `__joss_arr_ensure_cap`), objetos (`joss_obj_new`), mapas (`joss_map_new`) y nodos clave-valor (`24 bytes`) se alojan dinámicamente mediante `HeapAlloc` (Windows) o `sys_mmap` anónimo (Linux).
- **Límite actual documentado**: El runtime nativo actual utiliza un **modelo de alojamiento de vida de proceso (arena/heap sin recolector de basura ni `free` individual)**. Para herramientas CLI y procesos de corta o mediana duración el sistema operativo recupera toda la memoria al terminar el proceso; para demonios o servidores de larga duración será necesario introducir conteo de referencias (ARC) o un recolector conservador nativo.

---

## 8. Auditoría de Objetos, Arrays y Maps (Sección 9)

Verificado mediante `TestAuditObjectsArraysMaps`:
- **Arrays vacíos**: `var $emptyArr = []; echo len($emptyArr);` → `0`.
- **Arrays grandes (`> 256` elementos)**: Inserción de `600` elementos en bucle `while` mediante `$large = array_push($large, $i * 3)`, activando múltiples realocaciones geométricas en `__joss_arr_ensure_cap` (`256 → 512 → 1024`) sin pérdida ni corrupción de datos.
- **Arrays de strings**: Lectura, concatenación y expansión dinámica.
- **Arrays anidados (matrices 2D)**: `var $matrix = [$row0, $row1]; var $fetchedRow = $matrix[1];` preserva el tipo elemento compuesto (`ArrayType(TypeI64)`) en `pkg/ir/lower.go`.
- **Arrays de objetos**: Instancias de `Item` almacenadas en un array preservan su tipo `PtrType(StructType("Item"))` al indexarse (`$picked = $items[1]`), permitiendo acceder a propiedades (`$picked->code`) e invocar métodos (`$picked->describe()`) que retornan `string`.
- **Maps y Aliasing**: Inserción, sobreescritura de claves existentes (`$m["a"] = 99`) y semántica de referencia compartida (aliasing) verificada tanto para arrays (`$aliasArr[0] = 777` refleja en `$row0[0]`) como para objetos (`$aliasObj->code = 888` refleja en `$it1->code`).

---

## 9. Auditoría de Excepciones (`try` / `catch` / `throw`) (Sección 10)

Verificado mediante `TestAuditExceptionUnwinding`:
1. **`try` / `catch` anidados**: El bloque `try` interno captura su excepción (`"inner_err"`), restaura la profundidad `DATA_try_depth` y permite que el bloque `try` externo continúe y capture su propia excepción (`"outer_err"`).
2. **`throw` a través de múltiples marcos de llamada**: `topFrame -> midFrame -> leafThrow` desenrolla limpiamente 3 marcos de pila restaurando `RSP` y `RBP` del bloque `try` activo.
3. **`throw` desde métodos de clase**: `$val->check(-5)` lanza excepción capturada en el caller.
4. **`throw` dentro de bucles `while`**: Interrumpe la iteración en `$k == 3` y transfiere control al bloque `catch`.
5. **`throw` durante recursión profunda**: `recursiveThrow(8)` desenrolla 8 marcos recursivos directamente hasta el manejador `catch` sin corromper el marco de `main`.

---

## 10. Auditoría de Ejecutables `PE32+` y `ELF64` (Secciones 11 y 12)

Verificado mediante `TestAuditPE32PlusAndELF64Structure` usando los paquetes estándar `debug/pe` y `debug/elf`:

### 10.1 Windows `PE32+` (`pkg/backend/native/pe_emitter.go`)
- **DOS Header**: Magic `MZ` (`0x5A4D`), `e_lfanew = 0x40`.
- **PE Signature & COFF Header**: `"PE\0\0"`, `Machine = 0x8664` (`IMAGE_FILE_MACHINE_AMD64`), `NumberOfSections = 3`, `Characteristics = 0x0022` (`EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE`).
- **OptionalHeader64**: Magic `0x020B` (`PE32+`), `ImageBase = 0x140000000`, `SectionAlignment = 0x1000`, `FileAlignment = 0x200`, `Subsystem = 3` (`IMAGE_SUBSYSTEM_WINDOWS_CUI`).
- **Secciones**:
  1. `.text` (`RVA 0x1000`): Código máquina `x86_64` (`CNT_CODE | MEM_EXECUTE | MEM_READ`).
  2. `.rdata` (`RVA` alineado a `0x1000`): Import Directory Table (IDT), Import Lookup Table (ILT), Hint/Name Table, Import Address Table (IAT) y literales de cadena UTF-8 (`CNT_INITIALIZED_DATA | MEM_READ`).
  3. `.data` (`RVA` alineado a `0x1000`): Estado global del runtime (`stdout`, `stderr`, `heap`, pila `try/catch`) (`CNT_INITIALIZED_DATA | MEM_READ | MEM_WRITE`).
- **Dependencias Dinámicas**: Únicamente `KERNEL32.dll` (`GetStdHandle`, `WriteFile`, `ExitProcess`, `GetProcessHeap`, `HeapAlloc`). Cero dependencias de `msvcrt.dll`, `ucrtbase.dll` o librerías de terceros.

### 10.2 Linux `ELF64` (`pkg/backend/native/elf_emitter.go`)
- **ELF64 Header**: Magic `\x7fELF`, `ELFCLASS64`, `ELFDATA2LSB`, `ET_EXEC`, `EM_X86_64`, `e_entry = 0x401000`.
- **Program Headers (`PT_LOAD`)**:
  1. Segmento `R+X` (`0x400000` / `.text` en `0x401000` + `.rodata`).
  2. Segmento `R+W` (`.data` alineado a página de `0x1000`).
- **Enlace Estático Puro**: Sin segmento `PT_INTERP`, sin segmento `PT_DYNAMIC`, **`DT_NEEDED = none` (0 librerías dinámicas)**. Invoca directamente al kernel Linux vía `syscall`.

---

## 11. Métricas de Escalado de Tamaño Binario (Sección 17)

Mediciones reales obtenidas en `TestAuditBinarySizeScaling`:

| Programa de Prueba | Descripción | Tamaño Windows `PE32+` | Tamaño Linux `ELF64` | Comparación con binario Go equivalente |
| :--- | :--- | ---: | ---: | ---: |
| `1_Tiny_Echo` | `echo 1;` | **7,168 bytes** (7.0 KB) | **12,288 bytes** (12.0 KB) | ~2,100,000 bytes (~300x más pequeño) |
| `2_Small_Arithmetic_Loop` | Bucle `while` de 100 iteraciones + acumulador | **7,680 bytes** (7.5 KB) | **12,288 bytes** (12.0 KB) | ~2,100,000 bytes |
| `3_Medium_Classes_Arrays_Maps` | Clase `Point`, método `norm()`, `array` y `map` | **8,192 bytes** (8.0 KB) | **12,288 bytes** (12.0 KB) | ~2,200,000 bytes |
| `4_Large_20_Functions` | 20 funciones encadenadas con aritmética | **11,776 bytes** (11.5 KB) | **16,384 bytes** (16.0 KB) | ~2,300,000 bytes |

---

## 12. Registro Formal de Limitaciones Detectadas (Sección 19)

| # | LIMITATION | CAUSE | IMPACT | PROPOSED ARCHITECTURAL SOLUTION |
| :--- | :--- | :--- | :--- | :--- |
| **1** | **Alojamiento en Heap sin liberación individual (Arena de proceso)** | `__joss_os_alloc` invoca `HeapAlloc` / `sys_mmap`, pero el runtime nativo aún no incluye `HeapFree`/`munmap` ni conteo de referencias (ARC) o GC. | Ningún impacto en scripts CLI y procesos de corta/mediana duración; en procesos de larga duración el consumo de memoria crece monotónicamente. | Introducir cabeceras de conteo de referencias (`refCount` de 64 bits) en bloques alojados por `joss_str_concat`, `joss_arr_new`, `joss_obj_new` y `joss_map_new`, junto con `__joss_os_free` (`HeapFree` / `free list`). |
| **2** | **Operadores `/` (división entera vs flotante) y `<<`/`>>` (shifts vs streams)** | En `pkg/core`, `/` produce `float64` cuando hay residuo y `<<`/`>>` actúan como operadores de stream (`cout <<`), mientras que en `pkg/ir/lower.go` `/` baja a `OpDiv` (`idiv` entero) y `<<`/`>>` bajan a `OpShl`/`OpShr`. | El analizador semántico tipa `int / int` como `float`, rechazando retornos `int` directos sin conversión explícita. | Implementar soporte `TypeF64` (registros `XMM0`/`XMM1`, `cvtsi2sd`, `divsd`) en `MachineCodeBuilder` y alinear la tabla de operadores de `pkg/ir/lower.go` con `pkg/core/evaluator_infix.go`. |
| **3** | **Búsqueda lineal $O(n)$ en propiedades de Objetos y Maps** | `joss_obj_get_field_*` y `joss_map_get_*` recorren una lista enlazada simple comparando claves con `__joss_rt_strcmp`. | Adecuado para objetos con pocas propiedades y mapas pequeños; subóptimo para mapas con miles de claves. |Para clases conocidas en compilación, asignar índices de campo estáticos en `StructType`; para `map`, reemplazar la lista enlazada por una tabla hash abierta con hash FNV-1a de 64 bits en `machine_amd64.go`. |
| **4** | **Estado de `try`/`catch` global de un solo hilo y profundidad fija (64)** | `DATA_try_depth` y los arreglos `DATA_try_rsp[64]`, `DATA_try_rbp[64]`, `DATA_try_rip[64]` residen en la sección `.data` global. | Soporta hasta 64 bloques `try` anidados en ejecución secuencial; no es reentrante ante múltiples hilos del sistema operativo. | Almacenar el puntero a la cadena de `try` frames en Thread-Local Storage (TLS / registro `GS:`/`FS:`) y encadenar los nodos `tryFrame` directamente en el stack de cada función. |
| **5** | **Ausencia de `foreach`, closures con captura léxica y servicios de host (`HTTP`, `GranDB`, `File`)** | `pkg/ir/lower.go` rechaza explícitamente nodos AST sin representación en `MachineCodeBuilder` emitiendo `JOSS-NATIVE-001`. | Aplicaciones web completas como `JosSecurity` no compilan todavía con `joss build` y reciben diagnóstico explícito `JOSS-NATIVE-001`. | 1) Desazucarar `foreach` en `pkg/ir/lower.go` a un bucle `while` indexado con `joss_arr_len`/`joss_arr_get_*`. 2) Representar closures como pares `{fnPtr, envPtr}`. 3) Exponer primitivas de archivos y sockets del OS en la IAT/syscalls. |

---

## 13. Respuestas Directas a las Preguntas de Auditoría (Sección 21)

- **A. ¿`joss build` ya produce ejecutables reales sin Go/GCC/Clang/MSVC?**  
  **Sí.** `joss build` genera directamente el código máquina `x86_64` en memoria y emite el contenedor ejecutable `PE32+` (Windows) o `ELF64` (Linux) sin invocar ningún proceso ni compilador externo.
- **B. ¿Los binarios generados corren realmente en una máquina limpia?**  
  **Sí.** En Windows sólo importan 5 funciones base del kernel (`KERNEL32.dll`: `GetStdHandle`, `WriteFile`, `ExitProcess`, `GetProcessHeap`, `HeapAlloc`), presentes en toda instalación de Windows de 64 bits; en Linux son binarios `ELF64` estáticos con `0` librerías dinámicas (`DT_NEEDED = none`).
- **C. ¿Qué porcentaje aproximado del lenguaje core funciona hoy en modo standalone?**  
  **~70% del lenguaje algorítmico/estructural core** (niveles 1 a 7 de la matriz incremental: enteros, booleanos, cadenas UTF-8, aritmética, comparaciones, bloques condicionales, bucles `while`, funciones con número arbitrario de parámetros, recursión, arrays dinámicos con realocación automática, arrays anidados, mapas, clases, métodos, herencia de propiedades/métodos y manejo completo de excepciones `try`/`catch`/`throw`), y **0% de los subsistemas de infraestructura web/host** (HTTP, GranDB, WebSockets, templates, plugins), los cuales fallan explícitamente con `JOSS-NATIVE-001`.
- **D. ¿Cuáles son los 5 huecos técnicos más importantes detectados en la auditoría?**  
  1. Ausencia de punto flotante (`float`/`decimal` vía instrucciones SSE2 `XMM`) en `MachineCodeBuilder`.
  2. Falta de desazucarado de `foreach` en `pkg/ir/lower.go`.
  3. Closures con captura de entorno léxico (`upvalues`) e invocación indirecta de punteros a función.
  4. Liberación de memoria en heap (ARC / `HeapFree`) para procesos de larga duración.
  5. Primitivas de sistema operativo para archivos, argumentos CLI (`GetCommandLineA` / `argc, argv`) y red.
- **E. ¿Existe algún riesgo de corrupción de stack o memoria en `machine_amd64.go`?**  
  Tras corregir el crecimiento dinámico de arrays (`__joss_arr_ensure_cap` cuando `len >= cap` o `idx >= cap`) y verificar la alineación de 16 bytes de `RSP` tanto en prólogos como en llamadas con `> 4` argumentos (`TestAuditCallingConventionAndStack`), no existe corrupción de stack ni desbordamiento del buffer fijo de 256 elementos en arrays. El único límite estático remanente es la profundidad de 64 niveles de `try` anidados en `.data`.
- **F. ¿Existe alguna dependencia oculta en Go runtime?**  
  **Ninguna.** Ni en tiempo de compilación (`joss build` no invoca `go`) ni en el binario resultante (verificado con escaneo de firmas de `pclntab`, `go:buildid` y `runtime.*`).
- **G. ¿Existe alguna dependencia oculta en el intérprete?**  
  **Ninguna.** `pkg/backend/native` no importa `pkg/core`, `pkg/parser` ni `pkg/bytecode`, y los binarios generados no contienen ningún marcador `JOSSBC2Z` ni `ExecutePrepared`.
- **H. ¿Existe alguna duplicación indebida respecto de `pkg/core`?**  
  **Ninguna.** Los archivos `runtime/native/csrc/joss_rt.c` y `pkg/backend/native/c_emitter.go` fueron eliminados, y `machine_amd64.go` contiene exclusivamente las primitivas de bajo nivel (Categorías A y E) necesarias para materializar las instrucciones de `pkg/ir`.
- **I. ¿Cuál debe ser exactamente el siguiente paso de ingeniería después de esta auditoría?**  
  1. Implementar punto flotante `TypeF64` (SSE2 `XMM0`/`XMM1`) y desazucarar `foreach` en `pkg/ir/lower.go` para completar el 100% del lenguaje algorítmico (Nivel 8).
  2. Añadir resolución de argumentos de línea de comandos (`argc`/`argv` y `GetCommandLineA`) y primitivas de sistema de archivos (`CreateFileA`/`ReadFile`/`WriteFile` y `sys_open`/`sys_read`) en el runtime nativo.

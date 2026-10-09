# Auditoría de Preparación de la IR para el Native Backend

## 1. Estado Actual de Joss Native IR (`pkg/ir`)

La inspección de `pkg/ir` realizada tras la implementación previa determina con precisión las capacidades y límites de la IR:

### Capacidades Existentes:
1. **Tipos de IR (`pkg/ir/types.go`):**
   - Tipos primitivos: `void`, `bool`, `i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `f32`, `f64`, `string`.
   - Tipos estructurados y memoria: `ptr<T>`, `array<T>`, `struct`, `func`.
   - Tipo universal: `mixed`.
2. **Modelo de Valores (`pkg/ir/values.go`):**
   - Valores temporales: `%0`, `%1_hint` (`TempValue` con ID secuencial y tipo concreto).
   - Parámetros formales: `%arg0`, `%arg_name` (`ParamValue`).
   - Símbolos globales: `@global_name` (`GlobalValue`).
   - Constantes literales: `ConstInt`, `ConstFloat`, `ConstBool`, `ConstString`, `ConstNull`.
3. **Instrucciones y Memoria (`pkg/ir/instructions.go`):**
   - Manejo de memoria / stack: `alloca T`, `load %ptr`, `store %val, %ptr`, `move %src`.
   - Aritmética y lógica: `add`, `sub`, `mul`, `div`, `mod`, `neg`, `not`, `and`, `or`, `xor`, `shl`, `shr`.
   - Comparaciones: `cmp_eq`, `cmp_ne`, `cmp_lt`, `cmp_le`, `cmp_gt`, `cmp_ge`.
   - Llamadas: `call @fn(args...)`, `call_runtime "fn"(args...)`.
   - Conversiones: `cast %val to T`.
4. **Control Flow Graph y Terminadores:**
   - Terminadores estrictos: `return [%val]`, `jump %target`, `branch %cond, %trueBlock, %falseBlock`, `unreachable`.
   - Bloques básicos (`BasicBlock`) con etiquetas unívocas y listas de instrucciones.
   - Funciones (`Function`) con bloque `entry` garantizado.
5. **Runtime Requirements:**
   - Las funciones y programas registran explícitamente qué servicios de runtime demandan (ej. `print`, `alloc`).
6. **Verificación Estructural (`IRVerifier`):**
   - Valida presencia de terminadores, ausencia de código muerto tras terminadores en un bloque, consistencia de retornos y tipos en ramas.

### Limitaciones Identificadas:
1. **No existe SSA estricto con nodos `phi` todavía:**
   - La IR actual utiliza el patrón estándar de LLVM para variables mutables: `alloca` en el stack + `load` / `store`. Este modelo es idéntico al que produce Clang sin optimizaciones (`-O0`) y se traduce de forma limpia y directa tanto a LLVM IR como a código nativo sin requerir algoritmos de dominadores previos.
2. **Clases y Objetos:**
   - La IR actual soporta funciones, llamadas, estructuras y aritmética. La resolución completa de métodos polimórficos de clases (vtables) todavía no está conectada en el lowerer, lo cual está alineado con la fase actual.
3. **Strings complejos:**
   - Se soportan literales de cadena como constantes inmutables (`ConstString`) y llamadas al runtime `print_string`. Concatenación dinámica en el heap queda reservada para fases posteriores.

---

## 2. Estrategia del Native Backend

1. **Emisión de LLVM IR textual:**
   - Para compatibilidad con toolchains externos (Clang/LLVM, llc, lld), `pkg/backend/native` implementa un generador de LLVM IR (`.ll`) estándar, fuertemente tipado y canónico.
2. **Minimal Native Runtime (`runtime/native`):**
   - Se implementa un runtime autocontenido que provee las funciones ABI requeridas:
     - `joss_print_i64(int64_t)`
     - `joss_print_f64(double)`
     - `joss_print_bool(bool)`
     - `joss_print_string(const char*)`
     - `joss_panic(const char*)`
3. **Ensamblado y Linker Nativo:**
   - Para entornos donde `clang` o `gcc` estén instalados en el sistema, se invocan directamente para compilar el LLVM IR y runtime a un ejecutable nativo.
   - Para entornos Windows sin toolchain C/LLVM externo instalado en PATH, se provee un generador de código nativo / linker directo que aprovecha las herramientas disponibles (incluyendo `go tool asm`/`link` o emisión directa de ejecutables PE/COFF) garantizando que el usuario obtenga un binario `.exe` real ejecutable sin interpretar AST.

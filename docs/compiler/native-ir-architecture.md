# Documentación Arquitectónica: Joss Native IR (Intermediate Representation)

## 1. ¿Qué es Joss Native IR?
**Joss Native IR** es la representación intermedia oficial de primer nivel del compilador de Joss Programming Language. Está modelada como un grafo de control de flujo (Control Flow Graph o CFG) linealizado y SSA-friendly, ubicado entre el frontend semántico (`PreparedProgram`) y los futuros backends de generación de código nativo (LLVM, C, o generadores directos de código máquina).

```text
  Joss Source Code (.joss)
             │
             ▼
      Lexer / Parser (pkg/parser)
             │
             ▼
       AST canónico
             │
             ▼
  Semantic Analyzer (pkg/analyzer)
             │
             ▼
  PreparedProgram + AnalysisFacts
             │
             ▼
┌──────────────────────────────────────────────┐
│        JOSS NATIVE IR (pkg/ir)               │
│                                              │
│  - Program & Function Declarations           │
│  - Basic Blocks & Terminators (CFG)          │
│  - Abstract Value Model (%0, %1, ...)        │
│  - Strongly-typed Native Types (i64, f64...) │
│  - Runtime Capabilities / Requirements       │
└──────────────────────────────────────────────┘
             │
             ▼
   IR Verifier (pkg/ir.IRVerifier)
             │
             ▼
      [FUTURO BACKEND]
LLVM / Native CodeGen -> Machine Code (.exe / .so)
```

---

## 2. ¿Por qué existe y qué problema resuelve?

1. **Desacoplamiento del AST:** El AST de Joss es una representación orientada a la sintaxis concreta del lenguaje (ternarias, constructores `Init`, expresiones `match`, tuberías `|>`). Traducir el AST directamente a LLVM o código máquina genera una explosión de complejidad.
2. **Independencia del Runtime Go (`pkg/core`):** El evaluador tradicional de Joss ejecuta directamente el AST en memoria de Go utilizando `interface{}` y delegando al Garbage Collector de Go. La IR representa operaciones atómicas de máquina sin importar `pkg/core`.
3. **Poda de dependencias monolíticas:** Mediante la sección `RuntimeRequirements`, la IR registra qué primitivas nativas requiere el programa (ejemplo: `print_i64`, `alloc`), permitiendo enlazar exclusivamente un runtime nativo mínimo en lugar de arrastrar todo `pkg/core` (bases de datos SQL, WebSockets, WebView2).

---

## 3. Modelo de Tipos (`pkg/ir/types.go`)

La IR no utiliza tipos sintácticos del AST, sino tipos de bajo nivel preparados para layout en memoria y tipos LLVM:
- **Primitivos:** `void`, `bool`, `i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `f32`, `f64`.
- **Estructurados / Punteros:** `ptr<T>`, `array<T>`, `struct{...}`, `func(...) -> T`.
- **Dinámicos (Boxing):** `mixed` (para variables de dinamismo voluntario).

---

## 4. Modelo de Valores (`pkg/ir/values.go`)

Todos los operandos y resultados se identifican mediante valores abstractos con tipo estático:
- `TempValue` (`%0`, `%1_x`, etc.): Resultados de instrucciones o temporales.
- `ConstInt`, `ConstFloat`, `ConstBool`, `ConstString`, `ConstNull`: Constantes literales.
- `ParamValue` (`%arg0`, `%arg_name`): Parámetros formales de funciones.
- `GlobalValue` (`@global_var`): Referencias a variables o símbolos globales.

---

## 5. Control Flow Graph y Bloques Básicos (`pkg/ir/instructions.go` y `program.go`)

Cada función (`ir.Function`) consta de una lista de bloques básicos (`ir.BasicBlock`). 
- **Invariante fundamental:** Cada bloque contiene una secuencia lineal de instrucciones terminada obligatoriamente por un **Terminador**.
- **Terminadores disponibles:**
  - `ReturnTerminator`: Retorna opcionalmente un valor o `void`.
  - `JumpTerminator`: Salto incondicional hacia otro bloque básico.
  - `BranchTerminator`: Salto condicional bifurcado según un valor booleano (`branch %cond, TrueBlock, FalseBlock`).
  - `UnreachableTerminator`: Señala código inalcanzable.

---

## 6. Verificador Estructural (`pkg/ir/verifier.go`)

La regla canónica del compilador es:
> **"Invalid IR MUST NEVER reach a backend."**

El `IRVerifier` valida automáticamente:
- Que cada función tenga un bloque de entrada (`entry`).
- Que cada bloque básico termine exactamente en un terminador válido.
- Que no existan instrucciones posteriores a un terminador dentro del mismo bloque.
- Que las condiciones de bifurcación (`BranchTerminator`) sean estrictamente de tipo `bool`.
- Que los tipos retornados coincidan con la firma declarada de la función.

---

## 7. Dump Textual Determinista

Para pruebas de regresión, golden tests y depuración, `Program.Dump()` produce una representación textual unívoca y canónica:

```text
; Joss Native IR: arithmetic

func @main() -> i64 {
entry:
    %0_a_ptr = alloca i64
    store 10, %0_a_ptr
    %1_b_ptr = alloca i64
    store 20, %1_b_ptr
    %2_a = load %0_a_ptr
    %3_b = load %1_b_ptr
    %4_binop = add %2_a, %3_b
    return %4_binop
dead1:
    return 0
}
```

---

## 8. CLI de Diagnóstico

Se integró en el CLI oficial de Joss la capacidad de inspeccionar o exportar la IR directamente:

```bash
# Volcar la IR por consola
joss emit-ir programa.joss

# Guardar la IR en un archivo
joss emit-ir programa.joss -o programa.ir
```

---

## 9. Rendimiento de Compilación y Memoria

Medido en los benchmarks oficiales (`BenchmarkIR_LoweringAndVerification` y `BenchmarkIR_TextualDump`):
- **Lowering + Verificación:** ~12.4 microsegundos por programa (~6.9 KB asignados).
- **Emisión Textual (Dump):** ~43.8 microsegundos (~12.0 KB asignados).
- Capacidad de compilar más de **80,000 funciones por segundo** en un único núcleo de CPU.

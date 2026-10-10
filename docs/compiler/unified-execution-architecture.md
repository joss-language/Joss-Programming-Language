# Joss: Unified Language Execution Architecture

## 1. Principio Fundamental: "Feature Once, Execute Anywhere"

> **Joss se implementa una sola vez. Los backends no implementan el lenguaje; implementan cómo ejecutar la semántica de Joss.**

Históricamente, los compiladores o intérpretes que añaden soporte nativo corren el riesgo de bifurcar el lenguaje: una semántica para el intérprete dinámico y otra semántica independiente para el compilador nativo.

En **Joss Programming Language**, la arquitectura oficial establece una **única fuente de verdad semántica**:

```text
                                  JOSS SOURCE (.joss)
                                           │
                                           ▼
                                 ┌───────────────────┐
                                 │   JOSS FRONTEND   │
                                 │                   │
                                 │ Lexer             │
                                 │ Parser Pratt      │
                                 │ Type System       │
                                 │ Semantic Analyzer │
                                 │ Reachability      │
                                 └─────────┬─────────┘
                                           │
                                           ▼
                                 ┌───────────────────┐
                                 │   JOSS SEMANTIC   │
                                 │       MODEL       │
                                 │  (PreparedProgram)│
                                 │                   │
                                 │ Functions         │
                                 │ Classes           │
                                 │ Methods & Fields  │
                                 │ Interfaces & Enums│
                                 │ Symbols & Scopes  │
                                 │ Inferred Types    │
                                 │ Resolved Calls    │
                                 │ Source Units      │
                                 └─────────┬─────────┘
                                           │
                                           ▼
                                 ┌───────────────────┐
                                 │  COMMON EXECUTION │
                                 │     CONTRACT      │
                                 └─────────┬─────────┘
                                           │
             ┌─────────────────────────────┼─────────────────────────────┐
             │                             │                             │
             ▼                             ▼                             ▼
      INTERPRETER BACKEND            SERVER BACKEND               NATIVE COMPILER
      (pkg/core)                     (pkg/server)                 (pkg/backend/native)
             │                             │                             │
             ▼                             ▼                             ▼
      ExecutePrepared                Web RT / Routes              Lowering -> Native IR
      Interpreter Runtime            HotReload                    LLVM / Standalone
```

---

## 2. Diferenciación Estricta: Semántica vs Materialización

Para preservar la integridad del lenguaje único, el proyecto divide taxativamente las responsabilidades del compilador:

### Semántica (definida exactamente una vez en Frontend & Semantic Model)
- Qué significa una función, clase, variable o constante.
- Qué tipos tienen las expresiones y qué conversiones son válidas (`pkg/typesystem`).
- Qué destino tiene una llamada a función o método estático (`ResolvedCalls`).
- Qué alcance léxico y visibilidad tienen las declaraciones (`pkg/analyzer`).

### Materialización Física (delegada exclusivamente a cada Backend)
- **Interpreter Backend (`pkg/core`)**: Materializa la semántica mediante evaluación de AST con marcos de ejecución léxica (`Frame`) y valores dinámicos Go.
- **Server Backend (`pkg/server`)**: Materializa la semántica asociando el `PreparedProgram` a rutas web, middleware, sesiones y hot-reload.
- **Native Backend (`pkg/backend/native`)**: Materializa la semántica mediante Lowering a **Joss Native IR** (`pkg/ir`), basic blocks SSA y emisión de binarios de máquina (vía LLVM IR o C/Go Standalone bootstrap).

**Regla de Oro**: Ningún backend redescubre tipos, reinventa ámbitos léxicos ni reinterpreta qué significa una sentencia de Joss.

---

## 3. Taxonomía de Capas (Layers 1 a 6)

### Layer 1 — Source
Archivos `.joss` fuente legibles por humanos, sin modificaciones sintácticas artificiales. El mismo archivo fuente ejecuta en intérprete (`joss run`) y compila en nativo (`joss build`).

### Layer 2 — Frontend
- **Lexer & Parser Pratt** (`pkg/parser`): Genera el Abstract Syntax Tree (AST) canónico.
- **Type System** (`pkg/typesystem`): Define tipos primitivos, nominales, uniones, compatibilidad de asignación (`Assignable`) y coerción.
- **Semantic Analyzer & Resolver** (`pkg/analyzer`): Resuelve tablas de símbolos, visibilidad, contratos nominales, comprobación de tipos exhaustiva, alcance de flujo y sidecar fact indexing.

### Layer 3 — Semantic Model (`PreparedProgram`)
El modelo semántico representa **qué significa el programa Joss**, completamente desacoplado de cómo se ejecuta físicamente:
- Libre de detalles de LLVM o ensamblador.
- Libre de llamadas al runtime Go o evaluador.
- Libre de drivers SQLite / PostgreSQL.
- Provee métodos canónicos de consulta:
  - `prep.Classes()`
  - `prep.Functions()`
  - `prep.Interfaces()`
  - `prep.Enums()`
  - `prep.InferredType(node)`
  - `prep.ResolvedCall(call)`
  - `prep.Symbol(name)`

### Layer 4 — Lowering & Execution Bridge
Transforma la semántica de Joss en operaciones consumibles por cada backend:
- Para el intérprete: registro unificado de símbolos en marco léxico (`ExecutePrepared`).
- Para el compilador nativo: Lowering hacia bloques básicos y SSA temporaries en **Joss Native IR** (`pkg/ir`). Si una feature aún no posee materialización nativa, emite un error formal tipado `UnsupportedCapabilityError`.

### Layer 5 — Backends
- **Interpreter Backend** (`pkg/core`): Evalúa el AST validado en base a los metadatos resueltos.
- **Server Backend** (`pkg/server`): Despacha peticiones HTTP, rutas MVC, sesiones y WebSockets consumiendo el `PreparedProgram`.
- **Native Backend** (`pkg/backend/native`): Genera código de máquina real mediante LLVM IR (`clang`) o Standalone bootstrap builder.

### Layer 6 — Runtimes
- **Interpreter Runtime**: Frames léxicos, slots, built-ins dinámicos.
- **Native Runtime** (`runtime/native`): `joss_rt.h` / `joss_rt.c` con estructuras de fat-pointer de strings, E/S de consola y gestión de memoria nativa sin runtime Go.

---

## 4. Feature Probe Canónico: Operador Spaceship (`<=>`)

Para demostrar el principio "Feature Once, Execute Anywhere" de forma empírica y comprobable, se implementó el soporte completo de compilación y ejecución para el operador de comparación de tres vías (`<=>`):

### Código fuente (`tests/native/feature_probe.joss`):
```joss
public func compareNumbers(int $a, int $b): int {
    return $a <=> $b;
}

int $res1 = compareNumbers(15, 30);
int $res2 = compareNumbers(42, 42);
int $res3 = compareNumbers(100, 50);

echo $res1;
echo $res2;
echo $res3;
```

### Viaje por el Pipeline Unificado:
1. **Parser**: Lee `SPACESHIP` (`<=>`) con precedencia de comparación.
2. **Semantic Analyzer**: `infer.go` asigna tipo de retorno `int` para `<=>`.
3. **PreparedProgram**: Empaqueta el AST y hechos inferidos en `AnalysisFacts`.
4. **Interpreter Backend**: `spaceshipCompare` calcula el valor (-1, 0, 1).
5. **Native Lowering**: `lowerExpression` baja a `OpSpaceship` (`*ir.CompareInst`).
6. **Native CodeGen**:
   - Standalone: Emite `if left < right { -1 } else if left > right { 1 } else { 0 }`.
   - LLVM IR: Emite `select (icmp slt) -1, (select (icmp sgt) 1, 0)`.

### Resultado de Ejecución:
- `joss run tests/native/feature_probe.joss`:
  ```text
  -1
  0
  1
  ```
- `joss build tests/native/feature_probe.joss -o probe.exe && ./probe.exe`:
  ```text
  -1
  0
  1
  ```
- **Discrepancia entre intérprete y binario compilado**: **0 bytes (coincidencia byte a byte)**.

---

## 5. Matriz Canónica de Capacidades (Capability Matrix)

La Matriz de Capacidades (`pkg/analyzer/capability_matrix.go`) **no** define dos lenguajes distintos. Define la disponibilidad semántica del lenguaje frente a la capacidad de materialización física de cada backend:

| Característica de Joss | Semantic Model (Lenguaje) | Interpreter (Materialización) | Server (Materialización) | Native Compiler (Materialización) |
| :--- | :---: | :---: | :---: | :---: |
| **Functions** (globales, tipadas) | Soportado | Soportado | Soportado | Soportado |
| **Primitives** (int, float, bool, string) | Soportado | Soportado | Soportado | Soportado |
| **ControlFlow** (guard, ternary, while) | Soportado | Soportado | Soportado | Soportado |
| **Recursion** (llamadas recursivas) | Soportado | Soportado | Soportado | Soportado |
| **ConsoleIO** (echo, print) | Soportado | Soportado | Soportado | Soportado |
| **StaticClassMethods** (`Class::method`) | Soportado | Soportado | Soportado | Soportado |
| **SpaceshipOperator** (`<=>`) | Soportado | Soportado | Soportado | Soportado |
| **DynamicClasses** (instanciación `new`, props, métodos) | Soportado | Soportado | Soportado | Soportado |
| **DynamicArrays** (literales `[]`, indexación) | Soportado | Soportado | Soportado | Soportado |
| **DynamicMaps** (literales `{}`, indexación clave) | Soportado | Soportado | Soportado | Soportado |
| **Exceptions** (try / catch / throw) | Soportado | Soportado | Soportado | *En roadmap* (`JOSS-NATIVE-001`) |
| **Interfaces** (contratos nominales) | Soportado | Soportado | Soportado | *En roadmap* (`JOSS-NATIVE-001`) |
| **Channels & Concurrency** (`channel<T>`) | Soportado | Soportado | Soportado | *En roadmap* (`JOSS-NATIVE-001`) |
| **Defer** (`defer`) | Soportado | Soportado | Soportado | *En roadmap* (`JOSS-NATIVE-001`) |
| **Routes & MVC** (enrutador HTTP web) | Soportado | Soportado | Soportado | *No aplica a CLI* |
| **GranDB ORM** (Modelos SQL) | Soportado | Soportado | Soportado | *No aplica a CLI* |
| **WebSockets** (eventos en tiempo real) | Soportado | Soportado | Soportado | *No aplica a CLI* |

### Diagnóstico Estructurado `JOSS-NATIVE-001`:
Cuando el compilador nativo detecta una feature que el lenguaje ya comprende pero el backend nativo aún no materializa físicamente, emite el diagnóstico estructurado:

```text
[JOSS-NATIVE-001] [JOSS-NATIVE-CAPABILITY] Característica semántica no materializable en backend nativo:

Feature:
    DynamicClasses

Language:
    supported

Interpreter:
    supported

Server:
    supported

Native compiler:
    not currently supported

Motivo:
    La característica existe y es válida en la semántica de Joss, pero el compilador nativo todavía no cuenta con la infraestructura física de runtime para materializarla.

Sugerencia: Ejecute el programa con 'joss run' para ejecución interpretada.
```

---

## 6. Pruebas de Regresión Arquitectónica

La suite `tests/native/architectural_regression_test.go` protege automáticamente los principios del compilador:
1. **`TestArchitecturalRegression_NoASTSerializationInNativeExecutable`**: Garantiza que ningún ejecutable nativo contenga `JOSSBC2Z`, `JOSS_RUNNER_DATA`, ni dependencias pesadas (`modernc.org/sqlite`, `jackc/pgx`).
2. **`TestArchitecturalRegression_LoweringConsumesPreparedProgramDirectly`**: Garantiza que `LowerProgram` consuma directamente `PreparedProgram` sin invocar al parser.
3. **`TestArchitecturalRegression_InterpreterConsumesPreparedProgramDirectly`**: Garantiza que `Runtime.ExecutePrepared` corra en memoria sin escanear el disco.
4. **`TestArchitecturalRegression_CapabilityMatrixDistinguishesSemanticFromMaterialization`**: Valida que toda característica del catálogo esté soportada semánticamente en el modelo.
5. **`TestArchitecturalRegression_ResolvedCallFactRespectedByLowering`**: Valida que el Lowering respete los hechos de `prep.Facts.ResolvedCalls`.

---

## 7. Flujo Canónico para Agregar Nuevas Características ("Feature Once")

Al incorporar una nueva característica al lenguaje, los desarrolladores deben seguir este flujo unidireccional:

```text
1. Parser & Lexer (pkg/parser)         -> Sintaxis y AST
2. Type System (pkg/typesystem)        -> Reglas formales de tipos
3. Semantic Analyzer (pkg/analyzer)    -> Scopes, contratos y Facts
4. PreparedProgram (Semantic Model)    -> Modelo común preparado
5. Interpreter Backend (pkg/core)      -> Ejecución interpretada (joss run)
6. Native Lowering (pkg/ir)            -> Emisión de Native IR (joss build)
7. Differential Tests (tests/native)   -> Verificación byte a byte
```

---

## 8. Auditoría Final de Arquitectura (Respuestas Obligatorias)

### A. ¿Existe una sola semántica de Joss?
**SÍ.** Vive exclusivamente en `pkg/parser`, `pkg/typesystem` y `pkg/analyzer`. No existe ningún segundo analizador semántico en el compilador nativo ni en el servidor.

### B. ¿Interpreter y Native consumen el mismo modelo?
**SÍ.** Consumen exactamente la estructura `*semanticanalyzer.PreparedProgram` y sus hechos asociados `*AnalysisFacts`.

### C. ¿Existe reparse/reanalysis?
**NO.** Las unidades fuente se cargan y analizan exactamente una vez por invocación. `ExecutePrepared` y `LowerProgram` reciben el AST y los hechos ya calculados.

### D. ¿Existe duplicación semántica?
**CERO duplicación semántica.** La comprobación de tipos, la inferencia de tipos de retorno y la resolución de llamadas a métodos estáticos se realizan una sola vez en `pkg/analyzer` y se almacenan en `Facts.ResolvedCalls` e `InferredTypes`.

### E. ¿PreparedProgram es suficiente?
**SÍ.** `PreparedProgram` contiene las unidades analizadas (`Units`), las tablas de símbolos del proyecto (`Environment`), y el catálogo de hechos sidecar (`Facts`). Permite tanto la ejecución interpretada directa como el lowering exhaustivo a IR.

### F. ¿Hace falta Execution IR?
**NO.** Agregar un "Execution IR" intermedio entre `PreparedProgram` y `Joss Native IR` crearía una capa redundante e innecesaria (violando la regla anti-sobrediseño). `PreparedProgram` ya provee el contrato semántico de ejecución óptimo.

### G. ¿Una feature nueva puede integrarse una sola vez?
**SÍ.** Siguiendo el pipeline: se declara en el parser y el sistema de tipos, el analizador semántico la verifica y la deposita en el `PreparedProgram`. Desde allí, cualquier backend puede ejecutarla o materializarla.

### H. ¿Puede una feature semánticamente soportada ejecutarse en interpreter y native?
**SÍ.** Demostrado empíricamente con el Feature Probe del operador spaceship `<=>` en `tests/native/feature_probe.joss`.

### I. ¿Qué diferencias quedan entre backends?
- **Semántica**: Ninguna. Ambos backends obedecen la misma semántica canónica de Joss.
- **Materialización**:
  - Intérprete: Evaluación en árbol/frames de Go.
  - Servidor: Manejadores HTTP/WebSockets con runtime interpretado en memoria.
  - Nativo: Generación de código máquina mediante Native IR, SSA y LLVM/Standalone.

### J. ¿Joss sigue siendo un único lenguaje?
**YES.**
Evidencia: Un único pipeline de frontend (`pkg/parser` $\to$ `pkg/typesystem` $\to$ `pkg/analyzer`), un único modelo semántico (`PreparedProgram`), una matriz de capacidades coherente y un conjunto de pruebas diferenciales que validan igualdad byte por byte entre intérprete y ejecutable compilado.

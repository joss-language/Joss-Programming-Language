# Contrato Canónico de Integración de Features de Joss (Feature Integration Contract)

## 1. Principio Fundamental de Arquitectura

> **Semantics Once. Materialization Per Backend.**
> *(Semántica una sola vez. Materialización por backend).*

En Joss Programming Language, una característica del lenguaje se define semánticamente exactamente una vez. Los backends de ejecución (Intérprete, Servidor, Compilador Nativo) no definen ni reinventan el significado del lenguaje: únicamente materializan cómo ejecutar físicamente la semántica ya resuelta.

```text
┌─────────────────────────────────────────────┐
│              JOSS LANGUAGE                  │
│                                             │
│  Syntax + Types + Semantics + Resolution    │
└──────────────────────┬──────────────────────┘
                       │
                       ▼
              PreparedProgram
                       │
             Common Semantics
                       │
       ┌───────────────┼────────────────┐
       ▼               ▼                ▼
 Interpreter         Server           Native
 materialization   materialization   materialization
```

---

## 2. Definición Formal de Responsabilidades

### A. Responsabilidad Semántica (Semantic Responsibility)
El Frontend y el Modelo Semántico (`pkg/parser`, `pkg/typesystem`, `pkg/analyzer`) son la **única autoridad** que determina:
- Sintaxis y gramática del lenguaje.
- Tipos de datos, inferencia estricta y compatibilidad de asignación (`Assignable`).
- Símbolos, ámbitos léxicos (*lexical scopes*) y reglas de visibilidad (`public`, `protected`, `private`).
- Resolución inequívoca de llamadas a funciones y métodos (`ResolvedCalls`).
- Comportamiento y precedencia de operadores.
- Contratos nominales de clases, interfaces, registros y enums.
- Diagnósticos y errores semánticos de código.

### B. Responsabilidad de Materialización (Backend Materialization Responsibility)
Cada backend responde exclusivamente a la pregunta:
> *«Dado este significado ya resuelto y verificado, ¿cómo lo ejecuto físicamente en mi entorno?»*

```text
Joss function / method call
            │
            ▼
       ResolvedCall
            │
   ┌────────┼────────┐
   ▼        ▼        ▼
  Core    Server   Native
   │        │        │
 Frame   Runtime  Native IR
```

**Regla Estricta**: Un backend jamás debe volver a deducir qué función fue invocada, qué tipo tiene una expresión, qué overload corresponde o si una variable existe en el ámbito.

---

## 3. Feature Ownership Map (Mapa Canónico de Propietarios)

| Feature / Capacidad | Propietario Semántico | Representación Canónica | Materialización Intérprete | Materialización Servidor | Materialización Compilador Nativo | Suite de Pruebas Diferenciales |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Functions & Calls** | `pkg/analyzer` | `PreparedProgram.Environment.Functions` + `Facts.ResolvedCalls` | `pkg/core/executor.go` | `pkg/server` | `pkg/ir/lower.go` (`CallInst`) | `tests/native/differential_test.go` |
| **Primitives & Arithmetic** | `pkg/typesystem` | `typesystem.Type` (`Int`, `Float`, etc.) | `pkg/core/evaluator_numeric.go` | `pkg/server` | `pkg/ir/types.go` + `BinaryInst` | `tests/native/differential_test.go` |
| **Control Flow (`guard`, `while`)** | `pkg/analyzer` | `PreparedProgram.Units` (AST validado) | `pkg/core/evaluator_control.go` | `pkg/server` | `pkg/ir/lower.go` (`BranchTerminator`, `JumpTerminator`) | `tests/native/differential_test.go` |
| **Recursion** | `pkg/analyzer` | `Facts.ResolvedCalls` | `pkg/core/call_method.go` | `pkg/server` | `pkg/ir/lower.go` (`CallInst`) | `tests/native/differential_test.go` |
| **Static Class Methods** | `pkg/analyzer` | `Environment.Classes` + `Facts.ResolvedCalls` | `pkg/core/executor.go` | `pkg/server` | `pkg/ir/lower.go` (`@ClassName_method`) | `tests/native/differential_test.go` |
| **Spaceship Operator (`<=>`)** | `pkg/analyzer` | `Facts.InferredTypes` (retorna `int`) | `pkg/core/evaluator_compare.go` (`spaceshipCompare`) | `pkg/server` | `pkg/ir/instructions.go` (`OpSpaceship` $\to$ `CompareInst`) | `tests/native/differential_test.go` |
| **Console IO (`echo`, `print`)** | `pkg/analyzer` | `PreparedProgram.Units` (`EchoStatement`) | `pkg/core/builtins_string.go` | `pkg/server` | `pkg/ir/lower.go` (`CallRuntimeInst` $\to$ `print_i64`, `print_string`) | `tests/native/differential_test.go` |
| **Dynamic Classes (`new`, vtables)** | `pkg/analyzer` | `Environment.Classes` | `pkg/core/evaluator_member.go` (`Instance`) | `pkg/server` | *Rechazo explícito con `JOSS-NATIVE-001`* | `tests/native/differential_test.go` |
| **Dynamic Arrays & Maps** | `pkg/analyzer` | `PreparedProgram.Units` | `pkg/core/builtins_array.go` | `pkg/server` | *Rechazo explícito con `JOSS-NATIVE-001`* | `tests/native/differential_test.go` |
| **Exceptions (`try/catch/throw`)** | `pkg/analyzer` | `PreparedProgram.Units` | `pkg/core/evaluator_control.go` | `pkg/server` | *Rechazo explícito con `JOSS-NATIVE-001`* | `tests/native/differential_test.go` |
| **Channels & Concurrency** | `pkg/analyzer` | `PreparedProgram.Units` | `pkg/core/builtins_async.go` | `pkg/server` | *Rechazo explícito con `JOSS-NATIVE-001`* | `tests/native/differential_test.go` |
| **HTTP Routing, MVC & ORM** | `pkg/analyzer` | `PreparedProgram.Units` (Routes/Models) | `pkg/server/router.go`, `pkg/core/model.go` | `pkg/server` | *No aplica a binario CLI nativo* | `pkg/server/router_test.go` |

---

## 4. El Proceso Oficial de Integración de una Nueva Feature

Cualquier futura característica de Joss debe atravesar estrictamente estos 9 pasos:

```text
1. Define syntax (pkg/parser)
       ↓
2. Define semantic meaning (pkg/typesystem)
       ↓
3. Analyzer understands it (pkg/analyzer)
       ↓
4. PreparedProgram represents it (PreparedProgram.Facts)
       ↓
5. Interpreter materializes it (pkg/core)
       ↓
6. Server materializes it when applicable (pkg/server)
       ↓
7. Native backend materializes it when supported (pkg/ir -> pkg/backend/native)
       ↓
8. Differential test (tests/native)
       ↓
9. Architecture regression test (tests/architecture)
```

1. **Definir Sintaxis**: Agregar tokens en `pkg/parser/token.go`, reglas Pratt en `parser.go` y nodos AST correspondientes.
2. **Definir Semántica**: Clasificar tipos y reglas de asignabilidad en `pkg/typesystem`.
3. **Comprobación en Analizador**: Resolver tipos, firmas y registrar hechos en `pkg/analyzer` (`InferredTypes`, `ResolvedCalls`, etc.).
4. **Exposición en Modelo Semántico**: Empaquetar en `PreparedProgram`.
5. **Materialización en Intérprete**: Implementar evaluación en `pkg/core` leyendo los metadatos analizados.
6. **Materialización en Servidor**: Integrar en `pkg/server` si corresponde al ciclo HTTP o websocket.
7. **Materialización en Compilador Nativo**:
   - Si el runtime nativo cuenta con la infraestructura requerida: agregar el opcode en `pkg/ir/instructions.go`, el lowering en `pkg/ir/lower.go` y la emisión en `pkg/backend/native`.
   - Si no cuenta con soporte inmediato: registrar en `CapabilityMatrix` como `Unsupported` para emitir limpiamente el diagnóstico estructurado `[JOSS-NATIVE-001]`. **Nunca fingir soporte con fallbacks silenciosos**.
8. **Prueba Diferencial**: Añadir caso en `tests/native/differential_test.go` demostrando que `joss run` y el binario nativo de `joss build` producen salidas idénticas byte a byte.
9. **Prueba de Arquitectura**: Asegurar que la prueba en `tests/architecture` valide que no se introdujo re-parseo, AST serializado ni resoluciones de tipos paralelas.

---

## 5. Estados del Ciclo de Vida de una Feature (Feature Completion States)

Una feature no se declara completa solo porque el parser la acepte. La arquitectura formaliza 5 estados:

1. `LANGUAGE-COMPLETE`: El lenguaje la comprende semánticamente (`pkg/parser`, `pkg/typesystem`, `pkg/analyzer`).
2. `INTERPRETER-COMPLETE`: El intérprete de desarrollo puede ejecutarla completamente (`pkg/core`).
3. `SERVER-COMPLETE`: El servidor web y entorno de ejecución HTTP la soportan (`pkg/server`).
4. `NATIVE-COMPLETE`: El compilador nativo cuenta con Lowering a Native IR y generación de binarios (`pkg/ir`, `pkg/backend/native`).
5. `FULLY-COMPLETE`: Todos los modos de ejecución aplicables soportan la feature y pasan pruebas diferenciales continuas.

---

## 6. Límite Infranqueable entre Lowering y Semántica

### Patrón Correcto (Consumo de Semántica Resuelta):
```go
// El compilador nativo consulta la verdad semántica resuelta por el analyzer:
if resolved, ok := l.prep.Facts.ResolvedCalls[callExpr]; ok {
    calleeName = resolved.TargetID
    retType = l.mapTypeSystemType(resolved.ReturnType)
}
```

### Patrón Prohibido (Redescubrimiento Semántico en Backend):
```go
// PROHIBIDO: El backend intenta resolver tipos o ámbitos por su cuenta
func lowerCall(callExpr *parser.CallExpression) {
    scope := lookupScope(callExpr)      // ERROR: reinventa resolución de scopes
    types := inferTypesAgain(callExpr)  // ERROR: segundo type checker
}
```

---

## 7. Ejemplos Reales Documentados

### Ejemplo 1: Operador Spaceship (`<=>`)
- **Sintaxis**: Token `SPACESHIP` (`<=>`) con precedencia `EQUALS`.
- **Semántica**: `infer.go` determina retorno `int`.
- **Modelo Común**: Indexado en `prep.Facts.InferredTypes`.
- **Intérprete**: Evaluado en `pkg/core/evaluator_compare.go` (`spaceshipCompare`).
- **Nativo**: Lowering genera `OpSpaceship` (`*ir.CompareInst`).
- **Resultado Observable**: `10 <=> 20` produce `-1\n` de forma idéntica en ambos modos.

### Ejemplo 2: Métodos Estáticos y Funciones Cruzadas (`feature_probe_deep.joss`)
```joss
public class MathEngine {
    public static func power(int $base, int $exp): int {
        int $result = 1;
        int $i = 0;
        while ($i < $exp) {
            $result = $result * $base;
            $i = $i + 1;
        }
        return $result;
    }
}

public func evaluateComparison(int $val, int $target): int {
    int $sq = MathEngine::power($val, 2);
    int $cmp = $sq <=> $target;
    return $cmp;
}

int $t1 = evaluateComparison(3, 10);
int $t2 = evaluateComparison(4, 16);
int $t3 = evaluateComparison(5, 20);

echo $t1;
echo $t2;
echo $t3;
```
- **Semántica**: Resuelve `MathEngine::power` a nivel de proyecto, infiere tipos y asocia la llamada en `prep.Facts.ResolvedCalls`.
- **Materialización**: El Intérprete ejecuta en frames léxicos; el Compilador Nativo baja el método a `@MathEngine_power` y genera llamadas directas en SSA.
- **Salida**: Ambos producen `-1\n0\n1\n` con 0 bytes de divergencia.

---

## 8. Las 8 Reglas Canónicas para Desarrolladores

1. **Joss Semantics Exist Once**: La semántica vive exclusivamente en el frontend.
2. **`PreparedProgram` is the Canonical Contract**: Es la única estructura compartida entre el análisis y la ejecución.
3. **Backends Consume Semantics; They Do Not Redefine Them**: Ningún backend crea reglas de tipos ni resuelve llamadas independientemente.
4. **Backend-Specific Code is Materialization, Not Language Design**: El backend decide cómo compilar o evaluar, no qué significa el código.
5. **Unsupported Backend Capabilities Fail Explicitly**: Si una característica no puede materializarse nativamente, emite el diagnóstico estructurado `[JOSS-NATIVE-001]`. Cero magia, cero fallbacks silenciosos.
6. **Differential Tests Protect Behavioral Equivalence**: Toda feature nativa exige una prueba diferencial que valide coincidencia con el intérprete.
7. **Architecture Tests Protect Semantic Ownership**: La suite `tests/architecture` valida en CI que no se reintroduzcan dependencias ni duplicaciones.
8. **No Unnecessary Intermediate Abstraction is Introduced**: No se introducen capas adicionales si `PreparedProgram` satisface la necesidad del contrato.

# Validación del Compilador Nativo (AOT vs. Bundling)

**Proyecto:** Joss Programming Language  
**Componentes:** `pkg/backend/native`, `pkg/ir`, `cmd/joss`  
**Fecha:** 9 de octubre de 2026  

---

## 1. Arquitectura Real del Pipeline AOT

El flujo de compilación nativa AOT en Joss opera de forma completamente desacoplada del intérprete:

```
[Código Fuente .joss]
         │
         ▼
[Lexer + Parser Pratt] ──▶ AST
         │
         ▼
[Analizador Semántico] ──▶ PreparedProgram (Tipado Nominal Inmutable)
         │
         ▼
[Lowerer (pkg/ir)] ──▶ Joss Native IR (3AC / SSA lineal)
         │
         ▼
[Verifier (pkg/ir)] ──▶ Validación de Tipos y CFG en bajo nivel
         │
         ├────────────────────────────────────────┐
         ▼                                        ▼
[Backend LLVM (.ll)]                    [Backend Standalone (.go ABI)]
(Requiere Clang/GCC)                    (Go Toolchain CGO=0)
         │                                        │
         ▼                                        ▼
  [Binario Nativo Máquina]                [Binario Standalone Mínimo]
  (Sin Go Runtime / Sin AST)              (Sin AST, Sin VFS, Sin Evaluador)
```

---

## 2. Diferencias Contractuales: Compilación Nativa vs. Empaquetado

| Propiedad | Compilación Nativa AOT (`joss build <file>`) | Empaquetado Autónomo (`joss build bundle`) |
| :--- | :--- | :--- |
| **Comando** | `joss build main.joss` | `joss build bundle` o `joss build app` |
| **Representación** | Joss Native IR (`ir.Program`) | Carga de proyecto VFS + Cifrado AES de assets |
| **Intérprete AST** | **NO INCLUIDO** | Embebido en el runner para soporte de frameworks |
| **Evaluador Dinámico** | **NO INCLUIDO** | Presente para controllers y templates |
| **Comportamiento ante error de IR** | **Falla explícitamente con código 1** | N/A (no pasa por el lowerer de IR) |
| **Propósito** | Lógica de cálculo, utilidades, binarios de alto rendimiento | Despliegue de aplicaciones web completas |

---

## 3. Evidencias de Ausencia del Intérprete en el Artefacto AOT

Se diseñó y ejecutó la prueba automatizada `TestNativeAOT_StandaloneBinaryDoesNotContainASTInterpreter` en `tests/native/differential_test.go`:

1. **Inspección de Dependencias del Código Fuente Generado:**
   Se verificó mediante análisis estático que el código Go generado por `StandaloneBuilder.GenerateSource(prog)` no importa:
   - `github.com/jossecurity/joss/pkg/core`
   - `github.com/jossecurity/joss/pkg/parser`
   - `github.com/jossecurity/joss/pkg/analyzer`
   - Ninguna referencia a `ExecutePrepared` ni evaluadores de AST.
2. **Ejecución y Verificación de Salida:**
   El binario resultante se compiló a una carpeta temporal aislada y se ejecutó de forma autónoma sin depender del directorio del repositorio, produciendo exactamente la salida esperada.
3. **Eliminación del Fallback:**
   Se comprobó que si un programa solicita compilación nativa y el lowering de IR o la verificación fallan, el CLI emite un error con instrucciones claras y termina con código de salida 1 sin generar ejecutables engañosos.

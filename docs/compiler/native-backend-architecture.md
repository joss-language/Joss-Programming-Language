# Arquitectura del Joss Native Backend

## 1. Visión General
El **Joss Native Backend** (`pkg/backend/native`) es el subsistema encargado de transformar la representación intermedia **Joss Native IR** (`pkg/ir`) en código ejecutable nativo puro, eliminando por completo la necesidad del intérprete Go o la serialización del AST.

```text
                  Joss Source Code (.joss)
                             │
                             ▼
                    Lexer & Pratt Parser
                             │
                             ▼
                     Semantic Analyzer
                             │
                             ▼
                      PreparedProgram
                             │
                             ▼
                    Joss Native IR (pkg/ir)
                             │
                             ▼
                        IR Verifier
                             │
                             ▼
             ┌───────────────────────────────┐
             │   Native Backend Architecture │
             │      (pkg/backend/native)     │
             ├───────────────────────────────┤
             │ - LLVM IR Emitter (llvm.go)   │
             │ - Standalone Builder          │
             │ - Minimal Native Runtime ABI  │
             └───────────────┬───────────────┘
                             │
                             ▼
                Native Linker / Compiler
                             │
                             ▼
              JOSS REAL NATIVE EXECUTABLE (.exe)
```

---

## 2. Componentes Principales

### 2.1 Emisor de LLVM IR (`pkg/backend/native/llvm.go`)
Traduce funciones, bloques básicos y valores abstractos de la IR a LLVM IR textual canónico (`.ll`):
- Mapea tipos (`i64`, `double`, `i1`, `i8*`).
- Gestiona constantes literales de cadena en secciones globales (`@.str.N`).
- Emite operaciones aritméticas (`add`, `sub`, `fadd`, `fsub`, etc.) y comparaciones (`icmp`, `fcmp`).
- Emite control de flujo con saltos condicionales (`br i1 %cond, label %true, label %false`) y retornos (`ret`).

### 2.2 Minimal Native Runtime ABI (`runtime/native/`)
El ejecutable no enlaza `pkg/core` ni librerías pesadas como SQLite, Redis o WebSockets. En su lugar, enlaza únicamente las funciones ABI declaradas por los `RuntimeRequirements` del programa:
- `joss_print_i64(int64_t)`
- `joss_print_f64(double)`
- `joss_print_bool(bool)`
- `joss_print_string(const char*)`
- `joss_panic(const char*)`

### 2.3 Standalone Native Builder (`pkg/backend/native/standalone.go`)
Construye ejecutables nativos independientes directamente a partir del programa linealizado en IR:
- Sin runtime de Go embebido.
- Sin AST serializado (`JOSSBC2Z`).
- Sin empaquetado ni Virtual File System cifrado (`JOSS_RUNNER_DATA`).
- Tamaño del binario resultante: **~1.6 MB** (frente a los **28 MB - 37 MB** del runner anterior).

---

## 3. Uso en el CLI

El backend nativo está integrado de manera opcional y segura sin romper la funcionalidad previa:

```bash
# Compilar directamente a binario nativo real
joss build native-backend main.joss -o app.exe

# O utilizando el flag de backend
joss build native --backend=native
```

---

## 4. Auditoría de Seguridad y Pureza del Binario
En las pruebas automatizadas de [`tests/native/e2e_test.go`](file:///c:/Users/joss/Documents/proyectos/Joss-language/tests/native/e2e_test.go), se comprobó que el ejecutable generado:
1. No contiene el marcador `JOSSBC2Z`.
2. No contiene el marcador `JOSS_RUNNER_DATA`.
3. No contiene librerías SQL (`modernc.org/sqlite`, `jackc/pgx`).
4. Ejecuta exitosamente llamadas entre funciones nativas, cálculos aritméticos y cadenas de texto directamente desde código máquina.

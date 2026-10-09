# Arquitectura de Ejecución Híbrida y Compilación Nativa Oficial en Joss

## 1. Visión General

Joss Programming Language define formalmente dos modalidades de ejecución según las necesidades del desarrollador o del entorno de producción, unificadas bajo una interfaz de línea de comandos (CLI) limpia y predecible:

```text
                                  JOSS
                                    │
                  ┌─────────────────┴─────────────────┐
                  │                                   │
             INTERPRETACIÓN                      COMPILACIÓN
                  │                                   │
         ┌────────┴────────┐                          │
         │                 │                          │
      joss run        joss server start          joss build [file.joss]
         │                 │                          │
     Scripts &         Servidor HTTP /            Compilador Nativo Real
    Desarrollo        Framework Web MVC               (Binario nativo)
```

### Rutas Oficiales

1. **`joss run <archivo.joss>`**:
   Ejecuta scripts y código Joss de forma inmediata utilizando el analizador semántico y el runtime/intérprete en memoria Go. Ideal para desarrollo ágil, pruebas rápidas y automatización.
2. **`joss server start`**:
   Inicia el servidor HTTP de alto rendimiento, router, pooling de runtimes y controladores MVC para aplicaciones web y APIs.
3. **`joss build [archivo.joss] [opciones]`**:
   **La única ruta oficial para compilar Joss a un ejecutable nativo real**. Transforma el código fuente a través del frontend, Joss Native IR, verificación de CFG y generador nativo, produciendo un binario de máquina sin dependencias de intérpretes, VFS empaquetado ni serialización de AST.

---

## 2. Pipeline de Compilación Nativa

Cuando el desarrollador ejecuta `joss build main.joss`, el proceso sigue un flujo determinista y estricto:

```text
Joss Source (.joss)
        ↓
Lexer & Parser Pratt
        ↓
AST Validado
        ↓
Semantic Analyzer & Type System
        ↓
PreparedProgram & AnalysisFacts
        ↓
Reachability Analysis
        ↓
Lowering a Joss Native IR
        ↓
IR Verifier (CFG & Type Soundness)
        ↓
Native Backend (LLVM IR / Standalone Bootstrap)
        ↓
Linker Nativo
        ↓
Ejecutable Nativo Real (.exe / binario ELF / Mach-O)
```

### Invariantes del Compilador

* **Cero empaquetado de intérprete**: Está prohibido empaquetar un runner Go con AST serializado (`JOSSBC2Z`) o payloads binarios cifrados (`JOSS_RUNNER_DATA`) como producto de `joss build`. El resultado debe ser un ejecutable nativo puro.
* **Cero fallback silencioso**: Si el programa contiene construcciones que aún no han sido implementadas en el Native IR (como clases completas o colecciones dinámicas), el compilador rechaza inmediatamente la compilación con `UnsupportedCapabilityError`, emitiendo una explicación clara y sugiriendo el uso de `joss run`. Nunca degrada silenciosamente a empaquetado.

---

## 3. Opciones de Compilación en CLI

El comando `joss build` admite las siguientes opciones estándar:

| Opción | Descripción | Ejemplo |
| :--- | :--- | :--- |
| `-o <salida>` | Especifica la ruta o nombre del binario generado. | `joss build main.joss -o mi_app.exe` |
| `--target=<os>-<arch>` | Define el objetivo de compilación cruzada. | `joss build --target=linux-amd64` |
| `--release` | Compila con optimizaciones y sin símbolos de depuración (`-s -w` / `-O3`). | `joss build --release` |
| `--debug` | Conserva información y símbolos de depuración (`-g` / `-O0`). | `joss build --debug` |
| `--trace` | Emite artefactos intermedios del compilador (`.ir`, `.ll`, `.standalone.go`). | `joss build --trace` |
| `--backend=<auto\|llvm\|standalone>` | (Avanzado) Selecciona el generador de código nativo (por defecto `auto`). | `joss build --backend=standalone` |

### Compatibilidad y Deprecación de Subcomandos

Subcomandos experimentales históricos como `joss build native` y `joss build native-backend` se encuentran en proceso de deprecación. Cuando son invocados, emiten un aviso descriptivo y delegan de manera transparente al flujo oficial de compilación nativa:

```text
[aviso] 'joss build native' y 'joss build native-backend' están en deprecación. Usa directamente 'joss build [archivo.joss]'
```

---

## 4. Backends Nativos Soportados

El backend nativo opera mediante dos generadores de código:

1. **Backend LLVM (`pkg/backend/native/llvm.go` & `compiler.go`)**:
   Genera LLVM IR fuertemente tipado en formato texto (`.ll`) y lo compila a código máquina utilizando `clang` en el sistema anfitrión, enlazándolo con `joss_rt.c` (runtime nativo mínimo C embebido).
2. **Backend Standalone Bootstrap (`pkg/backend/native/standalone.go`)**:
   Genera código autónomo directamente a partir de las instrucciones de 3 direcciones de Joss Native IR, permitiendo la compilación nativa en plataformas donde no existe una instalación de LLVM/Clang en el PATH. Soporta compilación cruzada instantánea mediante targets `GOOS`/`GOARCH` sin dependencias externas (`CGO_ENABLED=0`).

La selección `BackendAuto` prioriza automáticamente `llvm` si `clang` está presente en el sistema, y selecciona `standalone` si no lo está.

### Transparencia de Toolchains y Cero Fallback Silencioso

Si el entorno no dispone de la toolchain nativa principal (`clang`), el compilador no oculta la decisión: notifica explícitamente en consola que está empleando el backend auxiliar bootstrap para generar el ejecutable:

```text
  [bootstrap] Toolchain nativo primario (LLVM/Clang) no encontrado en PATH; utilizando backend auxiliar bootstrap (Standalone).
✓ Ejecutable generado con éxito (bootstrap): app.exe (1619.0 KB) [target: windows-amd64]
```

De este modo, `standalone` se reconoce formalmente como una herramienta de bootstrap y compilación cruzada sin dependencias, mientras que `llvm` permanece como el backend nativo industrial de destino.

---

## 5. Matriz Multiplataforma Real

Los objetivos de compilación cruzada verificados y soportados oficialmente por Joss son:

| Plataforma / OS | Arquitectura | Target CLI | Intérprete (`joss run`) | Servidor (`joss server`) | Compilador Nativo (`joss build`) | Formato de Binario |
| :--- | :--- | :--- | :---: | :---: | :---: | :--- |
| **Windows** | amd64 (x86_64) | `windows-amd64` | ✓ | ✓ | ✓ | PE / Windows Executable (`.exe`) |
| **Windows** | arm64 | `windows-arm64` | ✓ | ✓ | ✓ | PE / Windows Executable (`.exe`) |
| **Linux** | amd64 (x86_64) | `linux-amd64` | ✓ | ✓ | ✓ | ELF 64-bit |
| **Linux** | arm64 (aarch64) | `linux-arm64` | ✓ | ✓ | ✓ | ELF 64-bit |
| **Linux** | arm (armv7) | `linux-arm` | ✓ | ✓ | ✓ | ELF 32-bit |
| **macOS (Darwin)**| amd64 (Intel) | `darwin-amd64` | ✓ | ✓ | ✓ | Mach-O 64-bit |
| **macOS (Darwin)**| arm64 (Apple Silicon)| `darwin-arm64` | ✓ | ✓ | ✓ | Mach-O 64-bit |
| **Android** (Termux)| arm64 (aarch64) | `android-arm64` | ✓ | ✓ | ✓ | ELF 64-bit |
| **Android** (Termux)| arm (armv7) | `android-arm` | ✓ | ✓ | ✓ | ELF 32-bit |

---

## 6. Matriz de Capacidades: Compilación vs. Interpretación

| Característica del Lenguaje | Estado en `joss run` | Estado en `joss build` | Diagnóstico en Compilación |
| :--- | :---: | :---: | :--- |
| Enteros, Floats, Strings, Booleanos | ✓ | ✓ | Compilación directa a IR nativo |
| Variables locales (`var`, `int`, etc.) | ✓ | ✓ | `AllocaInst`, `StoreInst`, `LoadInst` |
| Asignaciones y operaciones aritméticas | ✓ | ✓ | Instrucciones nativas de 3 direcciones |
| Comparaciones relacionales (`==`, `<`, etc.) | ✓ | ✓ | `CompareInst` de alta precisión |
| Operaciones lógicas (`&&`, `\|\|`, `!`) | ✓ | ✓ | Branching en bloques básicos (CFG) |
| Sentencias `guard ($cond) else { ... }` | ✓ | ✓ | Branching con terminador condicional |
| Operador ternario (`$cond ? $a : $b`) | ✓ | ✓ | Bloques true/false con merge |
| Ciclos `while ($cond) { ... }` | ✓ | ✓ | Bloques cond, body y exit nativos |
| Funciones nombradas y recursión | ✓ | ✓ | `Function` con marco aislado en IR |
| Clases y Programación Orientada a Objetos | ✓ | Pendiente | `UnsupportedCapabilityError` |
| Arrays dinámicos literales (`[]`) | ✓ | Pendiente | `UnsupportedCapabilityError` |
| Mapas y diccionarios literales (`{}`) | ✓ | Pendiente | `UnsupportedCapabilityError` |
| Excepciones (`try`, `catch`, `throw`) | ✓ | Pendiente | `UnsupportedCapabilityError` |
| Concurrencia y canales (`channel<T>`) | ✓ | Pendiente | `UnsupportedCapabilityError` |

---

## 7. Pureza del Binario y Auditoría de Símbolos

El ejecutable generado por `joss build` está completamente desacoplado del intérprete:

* **Ausencia de bytecode / AST comprimido**: No contiene firmas `JOSSBC2Z` ni cargas de serialización AST.
* **Ausencia de empaquetador histórico**: No contiene `JOSS_RUNNER_DATA` ni runners monolíticos Go.
* **Ausencia de subsistemas innecesarios**: Los binarios de consola de Joss no incluyen código de `modernc.org/sqlite`, `github.com/jackc/pgx`, WebSockets, ni WebView2.
* **Tamaño reducido**: Binarios entre 1.4 MB y 1.6 MB frente a los 32.5 MB del empaquetador histórico.

---

## 8. Pruebas Diferenciales y Verificación

La coherencia entre la ejecución interpretada (`joss run`) y la compilación nativa (`joss build`) está garantizada por una suite de pruebas diferenciales ubicada en `tests/native/differential_test.go`:

* **Aritmética y Variables**: Verificación de cálculos idénticos en ambos modos.
* **Control de Flujo**: Validación de operadores ternarios y sentencias `guard`.
* **Bucles**: Ejecución y terminación idéntica en ciclos `while`.
* **Funciones y Recursión**: Paridad en marcos de pila aislados, parámetros y retornos (ej. Fibonacci recursivo).
* **Salida Estándar**: Verificación byte a byte de sentencias `echo` e impresiones de strings.
* **Contrato de Capacidades**: Garantía de que características avanzadas aún no soportadas en Native IR sean rechazadas limpiamente por el compilador mientras permanecen funcionales en el intérprete.


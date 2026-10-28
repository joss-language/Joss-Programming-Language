# Arquitectura de Compilación Nativa y Backends de Código Máquina en Joss

---

## 1. Pipeline Canónico Unificado: Del Código Fuente al Ejecutable Real

El pipeline de compilación de Joss garantiza la regla de oro:
> **Joss se implementa una sola vez. Los backends implementan cómo ejecutar las mismas semánticas.**

```text
                             JOSS SOURCE (.joss)
                                      │
                                      ▼
                            Lexer + Parser Pratt
                                      │
                                      ▼
                                     AST
                                      │
                                      ▼
                          Semantic Analyzer Pipeline
               (collectDeclarations → projectScope → validate)
                                      │
                                      ▼
                          PreparedProgram (Canónico)
                                      │
                                      ▼
                        Joss Native IR (LowerProgram)
                                      │
                                      ▼
                             ir.Verifier (CFG + SSA)
                                      │
                                      ▼
                        SelectCodeEmitter(Target, GUI)
                                      │
                ┌─────────────────────┴─────────────────────┐
                ▼                                           ▼
      MachineCodeBuilder (x86-64)                 ARM64Builder (AArch64)
    - Opcodes x86-64                            - Instrucciones AArch64 (32-bit LE)
    - Registros RAX..R15                        - Registros X0..X30, SP
    - System V AMD64 / Windows ABI              - AAPCS64 ABI
                │                                           │
                └─────────────────────┬─────────────────────┘
                                      │
                                      ▼
                          Container Format Emitter
                                      │
         ┌────────────────────────────┼────────────────────────────┐
         ▼                            ▼                            ▼
  EmitPE32PlusExecutable      EmitELF64Executable        EmitMachO64Executable
     (Windows x64/ARM64)        (Linux x64/ARM64)          (macOS x64/ARM64)
```

---

## 2. Generación Real de Código Máquina AArch64 (ARM64)

A diferencia de versiones preliminares donde únicamente se modificaba el campo `e_machine` en la cabecera ELF, el backend **AArch64 puro** (`pkg/backend/native/machine_code_aarch64.go`) emite instrucciones máquina reales codificadas en palabras fijas de 32 bits Little-Endian:

### 2.1 Convención de Llamadas AAPCS64
- **Argumentos de función**: `X0` a `X7`.
- **Valores de retorno**: `X0` (enteros, booleanos, punteros a cadenas en heap o rodata).
- **Stack Pointer (`SP`)**: Alineación estricta a 16 bytes antes de cualquier llamada `BL`.
- **Prólogo Canónico**:
  ```assembly
  stp x29, x30, [sp, #-frame_size]!   ; Salva FP (X29) y LR (X30) y reserva espacio de stack
  mov x29, sp                         ; Actualiza FP al nuevo SP
  ```
- **Epílogo Canónico**:
  ```assembly
  ldp x29, x30, [sp], #frame_size     ; Restaura FP y LR y desapila
  ret                                 ; Salta a la dirección en X30 (0xD65F03C0)
  ```

### 2.2 Llamadas al Sistema Operativo (Syscalls) por Plataforma
- **Linux ARM64**:
  - `X8`: Número de Syscall (`SYS_write = 64`, `SYS_exit = 93`).
  - `X0, X1, X2`: Descriptores, búferes y contadores.
  - Instrucción: `SVC #0` (`0xD4000001`).
- **macOS (Darwin) ARM64 / Apple Silicon**:
  - `X16`: Número de Syscall Darwin (`SYS_write = 4`, `SYS_exit = 1`).
  - `X0, X1, X2`: Argumentos.
  - Instrucción: `SVC #0x80` (`0xD4001001`).

---

## 3. Matriz de Soporte Multiplataforma Tier 1

| Target | Formato de Objeto | Arquitectura ISA | Estado | Tamaño Típico Binario Standalone |
|---|---|---|---|---|
| `windows-amd64` | PE32+ (COFF) | x86_64 |  **PASS (Ejecución real verificada)** | ~10 KB |
| `linux-amd64` | ELF64 static | x86_64 |  **PASS (Instrucciones verificadas)** | ~16 KB |
| `linux-arm64` | ELF64 static | AArch64 |  **PASS (AArch64 puro verificado)** | ~12 KB |
| `darwin-arm64` | Mach-O 64 | AArch64 |  **PASS (Mach-O ARM64 verificado)** | ~4.4 KB |
| `darwin-amd64` | Mach-O 64 | x86_64 |  **PASS (Mach-O x86_64 verificado)** | ~8.2 KB |

---

## 4. Uso Unificado de `joss build`

El desarrollador no necesita memorizar comandos complejos ni flags exóticas:

```bash
# Compilación nativa automática para el sistema actual del host
joss build main.joss

# Compilación cruzada nativa explícita (Cross-Compilation)
joss build main.joss --target=linux-arm64
joss build main.joss --target=darwin-arm64
joss build main.joss --target=windows-amd64
```

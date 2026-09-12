# Comprehensive audit report and documentary reconstruction of Joss

Before: [Contribute to Joss](CONTRIBUIR.md). After: [Technical audit 2026](AUDITORIA_TECNICA_2026.md).
General index: [Joss documentation](README.md).

---

## 1. Introduction and purpose of the audit

This report documents the comprehensive audit and deep reconstruction of the **Joss** programming language documentation, carried out on September 5, 2026.

The absolute guiding principle of this work has been:

> **The source code is the only source of truth.**

All of the above documentation was tested against the actual behavior of the lexical analyzer (`pkg/parser/lexer.go`), the Pratt parser (`pkg/parser`), the AST abstract syntax tree (`pkg/parser/ast*.go`), the type system and inference (`pkg/typesystem`), the semantic analyzer (`pkg/analyzer`), the Go tester and runtime (`pkg/core`), the registered native classes and functions, and the automated test suite.

---

## 2. Diagnosis of the initial state and documentary debt

At the beginning of the audit, it was detected that the existing documentation presented a significant gap with respect to the real capabilities of the code:

### A. Pedagogical gap for beginners
- The documentation was written almost exclusively as a compact technical reference for people who already mastered other languages ​​or knew compiler jargon.
- There were no explanations about foundational concepts such as what a variable is, why `$` is used, the difference between printing to the screen (`print`) and returning a value (`return`), or what memory and scope (*scope*) represent.
- Advanced concepts such as `async`, `await` or `channel` were presented through code fragments without previously explaining the difference between synchrony, asynchrony and concurrency.

### B. Features implemented in the code that were undocumented or underdocumented
During the line-by-line inspection of the source code, behaviors and syntax were discovered that were implemented but absent from the guides:
1. **The Pipeline operator (`|>`)**:
   - Implemented at `pkg/parser/parser.go` (precedence `PIPE_OP`), `pkg/parser/parser_expressions.go` and evaluated at `pkg/core/evaluator_infix.go`.
   - Allows you to compose functions from left to right: `" ada " |> trim |> strtoupper`. If the function requires more arguments, the left value is injected as the first parameter. It was absent in the tutorials.
2. **The Elvis operator (`?:`)**:
   - Implemented in the ternary parser (`parseTernaryExpression`) and evaluated in `evaluateTernary`. Allows evaluating default values ​​when the condition is truthy: `$val = $input ?: "default"`.
3. **The null coalescence operator (`??`)**:
   - Implemented with null error protection for clean fallbacks: `$val = $input ?? "fallback"`.
4. **The null-safe navigation operator (`?->`)**:
   - Allows access to properties and methods of potentially null instances (`$user?->nombre`) without crashing the program.
5. **Auto-append syntax in arrays (`$arr[] = $val`)**:
   - Supported in the assignment evaluator (`IndexExpression` with null index), allowing elements to be added to the end naturally.
6. **Consumption of concurrent channels through `foreach`**:
   - `executeForeach` in `pkg/core/executor.go` checks if the iterable is a `*Channel` (`for item := range ch.Ch`), allowing you to create pure producer-consumer patterns without repetitive manual calls to `recv`.
7. **Error map structure captured at `catch ($e)`**:
   - When a `JossError` occurs at runtime, the block `catch` receives an associative map structured with the keys `"message"`, `"type"`, `"file"`, `"line"` and `"error"`.
8. **Bilateral reference invariance (`ref`)**:
   - The code requires that both the function declaration and the call use `ref`, requires identical types (without extension `int` to `float`) and prohibits escaping the reference outside the frame.

### C. Obsolete documentation and misinformation removed
- **Aliases removed**: Categorically clarified that `integer`, `double`, `boolean`, `dynamic`, `any` and `list` are no longer standardized; using them triggers the error `JOSS-TYPE-009`.
- **Deprecated async syntax**: It was documented that `async(func() ...)` was removed and that the only valid syntax is the block `async { ... }`.
- **Ghost or incorrectly cataloged APIs**: Mentions to methods not registered in the dispatcher such as `Http::query` or `System::change_db` were removed.
- **Non-mutant behavior of `array_pop` and `array_shift`**: Developers were explicitly alerted that in Joss these functions return the element without reducing the length of the original array.

---

## 3. New project documentary architecture

The documentation was restructured into four complementary levels to satisfy all audiences without degrading technical rigor:

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. APRENDER JOSS (Niveles 0 al 10 - Progresivo y pedagógico) │
│    - PRIMEROS_PASOS: De cero a un programa ejecutable.      │
│    - FUNDAMENTOS: Memoria, tipos primitivos y variables.    │
│    - CONTROL_FLUJO: Ternarios con bloques, match y bucles.  │
│    - FUNCIONES: Scope, closures, ref y pipelines.           │
│    - COLECCIONES: Arrays, maps y texto Unicode.             │
│    - SISTEMA_TIPOS: Inferencia, uniones y conversiones.     │
│    - CLASES: POO, Init, encapsulación y herencia.           │
│    - ERRORES: Fases, diagnósticos y try/catch.              │
│    - CONCURRENCIA: Async, Future y canales.                 │
│    - PROYECTO_CONSOLA: Proyecto real con persistencia JSON. │
│    - PROYECTO_WEB: Aplicación MVC con el stack nativo.      │
│    - GLOSARIO: Diccionario conceptual para principiantes.   │
├─────────────────────────────────────────────────────────────┤
│ 2. REFERENCIA TÉCNICA DEL LENGUAJE Y HERRAMIENTAS           │
│    - SINTAXIS: Tokens, precedencias y operadores.           │
│    - GRAMATICA: EBNF formal y correspondencia con el AST.   │
│    - DIAGNOSTICOS: Catálogo completo de códigos JOSS-*.     │
│    - FUNCIONES_GLOBALES: Las 117 funciones built-in.        │
│    - MODULOS_NATIVOS: Clases integradas en Go.              │
│    - CATALOGO_NATIVO: Catálogo sincronizado por docgen.     │
│    - CLI: Referencia de comandos joss.                      │
│    - VSCODE_EXTENSION: LSP y tooling del editor.            │
│    - ESTADO_IMPLEMENTACION: Estado real vs límites.         │
├─────────────────────────────────────────────────────────────┤
│ 3. DESARROLLO DE APLICACIONES                               │
│    - ESTRUCTURA_PROYECTO, CONFIGURACION, MODULOS_IMPORTS.   │
│    - SERVIDOR, CONTROLADORES, MIDDLEWARE, VISTAS, ASSETS.   │
│    - MODELOS, SCHEMA_BUILDER, MIGRACIONES, AUTENTICACION.   │
│    - WEBSOCKETS, PLUGINS, SEO_SITEMAP.                      │
├─────────────────────────────────────────────────────────────┤
│ 4. INTERNALS Y GUÍA PARA CONTRIBUIDORES                     │
│    - ARQUITECTURA: Pipeline del compilador y runtime Go.    │
│    - CONTRIBUIR: Cómo extender sintaxis, tipos y built-ins. │
│    - AUDITORIAS Y NOVEDADES: Historial y optimizaciones.    │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. Developer experience (UX) issues detected in the language

During the source code audit and test execution, the following points were identified where the current behavior of the language may be confusing or require future attention:

1. **Detection of `break` / `continue` in nested ternaries**:
   - The loop plan optimizer (`pkg/runtime/plan`) looks for direct jumps in the AST. If a `break` or `continue` is inside the block of a ternary in the loop, in certain situations the plan does not detect it and escapes as an internal panic instead of being absorbed by the loop.
2. **Arms from `match` that are blocks**:
   - In the current implementation of `pkg/core`, an arm `match` whose right value is a block `{ ... }` can return the block's syntax node as a value instead of executing its internal declarations, even though the semantic analyzer counts it as a valid return. It is recommended to use direct expressions in the arms of `match`.
3. **Non-atomic behavior of `GranDB::transaction`**:
   - `GranDB::transaction` opens a SQL transaction in Go (`tx, err := db.Begin()`), but normal calls executed within the user closure use the instance's standard database connection and not the `*sql.Tx` pointer, so they do not guarantee complete atomicity on errors within the callback.
4. **Discrepancy in `array_pop` and `array_shift`**:
   - Developers coming from PHP or JavaScript expect `array_pop` to remove the element from the original array. In Joss, it returns the value but the underlying array retains its size.
5. **Static analysis asymmetry in web projects**:
   - The command `joss analyze main.joss` scans `main.joss` and the files inside `app/**/*.joss`. However, `routes.joss` is processed only when you start the HTTP server using its own loader, so a syntax error in `routes.joss` is only discovered when running `joss server start`.
6. **Security in pseudorandom number generation**:
   - Certain native utility functions (such as `Str::random` and the TOTP module) use the `math/rand` package with predictable seeds instead of the cryptographically secure generator `crypto/rand`.

---

## 5. Formal verification and tests executed

To ensure that no documents had broken links, outdated examples, or inconsistencies with the runtime, the full validation suite was run:

1. **Catalog verification and native documentation**:
   ```bash
   go run ./tools/cataloggen --check
   go run ./tools/docgen --check
   ```
2. **Unit and integration test suite**:
   ```bash
   go test ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
   ```
3. **Verification of contracts and navigation (`TestDocumentationNavigationAndPublicMirror` and `TestDocumentationContracts`)**:
   - Checking all local Markdown links between documents.
   - Byte-by-byte parity check between `docs/*.md` and `ejemplos/Joss-Red-JosSecurity/assets/docs/*.md`.
   - Ejecución y análisis de todos los bloques etiquetados con `<!-- joss-run: ... -->`, `<!-- joss-check: ... -->` y `<!-- joss-error: ... -->`.
4. **General construction of the repository**:
   ```bash
   go build ./...
   ```

---

## 6. Conclusion

With this reconstruction, Joss documentation ceases to be a simple catalog of syntax and becomes a **comprehensive pedagogical and technical system**. It allows anyone with no prior experience to learn to program from scratch step by step, while providing experienced developers and compiler contributors with a comprehensive, honest, and verifiable reference based 100% on the source code.

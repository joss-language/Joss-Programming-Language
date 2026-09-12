# Deep Audit of the Joss Programming Language (2026) > **Canonical Document of Technical Audit, Ergonomics, Type System, Tooling and Architecture** > **Date of Preparation:** September 2026 > **Scope:** Official repository `jossecurity/joss`, kernel subsystems(`pkg/*`), CLI tools, LSP server (`vscode-joss`) and real reference project `Joss-Red-JosSecurity`.> **Purpose:** Exhaustive diagnosis, empirical evidence in code, catalog of proposals and technological evolution plan towards a more expressive, coherent and secure language.--- ## General Index 1. [Executive Summary](#1-resumen-ejecutivo) 2. [Current Philosophy Detected in Joss](#2-filosofía-actual-detectada-en-joss) 3.[Current Strengths](#3-fortalezas-actuales) 4. [Structural Weaknesses](#4-debilidades-estructurales) 5. [Friction Found atWrite Code](#5-fricción-encontrada-al-escribir-código) 6. [Boilerplate and Ceremony Identified](#6-boilerplate-y-ceremonia-identificados) 7. [Language Inconsistencies](#7-inconsistencias-del-lenguaje) 8. [Conceptual Complexity](#8-complejidad-conceptual) 9. [Type System Audit](#9-auditoría-del-sistema-de-tipos) 10.[Runtime Security and Defense Audit](#10-auditoría-de-seguridad-y-defensas-runtime) 11. [Error and Diagnostic Audit](#11-auditoría-de-errores-y-diagnósticos) 12.[Standard Library (Stdlib) Audit](#12-auditoría-de-la-biblioteca-estándar-stdlib) 13. [Tooling and Ecosystem Audit](#13-auditoría-de-tooling-y-ecosistema) 14.[Official Formatter Audit](#14-auditoría-del-formatter-oficial) 15. [Analyzer and Linter Audit](#15-auditoría-del-analyzer-y-linter) 16. [Selective Comparison with Other Languages](#16-comparación-selectiva-con-otros-lenguajes) 17. [Simplification Opportunities](#17-oportunidades-de-simplificación) 18. [Features that should NOT be implemented](#18-características-que-no-conviene-implementar) 19. [Catalog of Prioritized Proposals (P1 to P8)](#19-catálogo-de-propuestas-priorizadas-p1-a-p8) 20. [Changes that Would Require Deprecation](#20-cambios-que-necesitarían-deprecación) 21. [Possible Breaking Changes Justified](#21-posibles-breaking-changes-justificados) 22. [Proposal and Architecture of `joss fix`](#22-propuesta-y-arquitectura-de-joss-fix) 23. [Definition of "Idiomatic Joss Code"](#23-definición-de-código-joss-idiomático) 24. [Recommended Roadmap (Phases 0 to 5)](#24-roadmap-recomendado-fases-0-a-5) --- ## 1. Executive Summary Joss is a modern multi-paradigm programming language developed in Go, conceived for the development of web services, high-performance APIs, microservices and infrastructure utilities.Its founding proposal seeks to combine the **iteration agility and syntactic familiarity** of dynamic server languages ​​(PHP, JavaScript, Python) with the **static security, concurrent robustness and speed** of the Go ecosystem.Through its evolution, Joss has established exceptional architectural pillars:- **Decoupled compilation pipeline:** Formal architecture based on a specialized lexer, a Pratt operator precedence parser, a unified AST, a strict semantic parser (`pkg/analyzer`), structured diagnostics and a lexical slot-optimized tree evaluation runtime (`pkg/runtime/plan`).- **Zero-Imports Architecture:** Automatic loading and resolution of symbols at the project level based on structure conventions, eliminating manual management of dependency graphs in web applications.- **Arithmetic and memory safety:** 64-bit integers with overflow detection at compile and run time (`JOSS-ARITH-001`), native monetary fixed point (`decimal` with literal `m`), safe temporal references and invariants(`ref`), and deep recursion control.- **Clean asynchrony:** Native support for goroutines using `async { ... }` and channels with direct operators (`$c << $msg`), with the `await(...)` function enabled in any context without forcing code to be fragmented into colored functions.### The Audit Diagnosis Notwithstanding its strengths, this technical audit has detected that **Joss currently imposes notable accidental friction on the developer**: 1. **Type Checker Asymmetry:** The compiler requires strict annotations in parameters (`JOSS-TYPE-011`) and exhaustive validation of branches in returns (`JOSS-TYPE-010`), but lacks**propagation of type refinement (flow-sensitive type narrowing)** after guard clauses.This pushes programmers in production to undo typing using generalized `mixed`.2. **Forced Flow Control:** The dogmatic eradication of classical conditional statements in favor of the generalized ternary operator with blocks produces nesting pyramids of up to 6 levels and the proliferation of false empty branches `: {}`.3. **Loss of Object Identity in Exceptions:** Error instances captured in `catch ($e)` are downgraded to text using `fmt.Sprintf`, destroying the OOP in exception handling.4. **Inconsistent Standard Library:** Duplicate names of global functions inherited from PHP coexist with absent fluid methods in strings and collections.5. **Fragmented Tooling:** Formatter and linter reimplement independent parsers instead of sharing the official kernel AST.This document presents the detailed analysis, the empirical evidence collected in the code and the prioritized catalog of solutions to consolidate Joss as a predictable, safe and productive language.--- ## 2. Current Philosophy Detected in Joss The analysis of the implementation reveals the following premises that define the current character of Joss:```text
┌─────────────────────────────────────────────────────────────────────────┐
│                        PRINCIPIOS REALES DE JOSS                        │
├─────────────────────────────────────────────────────────────────────────┤
│ 1. Seguridad estática demostrable sin compilación pesada a máquina.     │
│ 2. Cero fricción de importación de archivos de proyecto (Zero-Imports). │
│ 3. Unificación sintáctica: las bifurcaciones son expresiones evaluables.│
│ 4. Tipado numérico defensivo: protección monetaria y anti-overflow.     │
│ 5. Concurrencia de paso de mensajes inspirada en CSP / Go.              │
│ 6. Baterías incluidas para desarrollo web y servicios backend.          │
└─────────────────────────────────────────────────────────────────────────┘
```
### Historical Inconsistencies and Philosophical Tensions 1. **Expressivity vs.Bureaucracy in Declarations:** - While Joss seeks to eliminate ceremony in imports, it imposes formal rigidity in callable signatures: mandatory explicit visibility (`public`/`private`/`protected`), mandatory parameter types and exhaustive return contracts.In practice, production code circumvents this rigidity by typing with `mixed` and omitting the return.2. **"Everything is Expression" vs.Imperative Code with Side Effects:** - The unification of decisions under the ternary operator `($cond) ? { ... } : { ... }` works elegantly for simple assignments:     ```joss-snippet
     $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
     ```
However, in business controller code where it is required to validate preconditions, log audits and abort prematurely, the ternary is used as a statement, forcing the writing of empty blocks `: {}` or deep nesting that contradicts the goal of clarity.3. **Strong Typing vs.Procedural Library Not Knowing Types:** - The type system allows you to specify `array<int>` and `map<string, Usuario>`, but almost all functions in `builtins_array.go` return `[]interface{}` or`mixed`, stripping the collection of its type guarantees.4. **Zero-Imports vs.Contaminated Global Space:** - Since there are no namespaces in the Joss source code, classes must be renamed with artificial prefixes (`XiaomiCategory`, `CmsPost`, `AuthUser`) to prevent collisions in the project's global dictionary.--- ## 3. Current Strengths Joss has outstanding technical pillars that must be preserved as competitive advantages: 1. **Unidirectional Architectural Pipeline:** - The compiler strictly respects the layer boundary: `parser`, `typesystem` and `analyzer` areruntime agnostic and never import databases or network services.2. **Structured Diagnostics Model:** - Deterministic output via `pkg/diagnostics` with accurate line/column ranges, stable codes, and human-readable fix suggestions and consumables by IDEs.3. **Financial Numerical Accuracy (`decimal`):** - Built-in base ten fixed point support (`decimal $precio = 99.99m`) via `shopspring/decimal` eliminates IEEE-754 rounding errors common in PHP, Python or JavaScript.4. **Rigorous Overflow Defense:** - Interception of overflow in 64-bit arithmetic via `typesystem.CheckedIntBinary` before data corruptions occur.5. **Pragmatic and Light Concurrency:** - Channels as first-order citizens (`channel $c = make_chan(10)`), sending operator `$c << $msg`, sequential consumption `foreach ($c as $msg)` and multiple control `select`.- `async { ... }` produces concurrent `Future` objects in goroutines without imposing "function coloring" along the call tree.6. **Constructor Property Promotion:** - The `Init(public string $nombre, public int $edad = 30) {}` syntax eradicates the ceremonial field assignment code.7. **Automated Documentation Verification:** - `pkg/core/documentation_test.go` automatically validates each executable fragment of the official documentation, ensuring that the manuals never get out of sync with the actual behavior of the engine.--- ## 4. Structural Weaknesses 1. **Absence of Flow-Sensitive Type Narrowing Exterior:** - The parser only recognizes the narrowing of a nullable type within the branches of the ternary.After a guard clause with premature exit, the outer variable is not promoted.2. **Friction in Flow Control:**- Absence of a flat decision statement (`guard` or `if`), resulting in forced builds with empty false blocks and branches `: {}`.3. **Instance Destruction in Exception Handling:** - The runtime degrades class instances launched via `throw new CustomException(...)` to simple text strings `string` when trapped in `catch ($e)`.4. **Surprising Behavior in Equality (`==`) and Falsehood (`isFalsy`):** - The operator `==` resorts to conversion to text using `fmt.Sprintf` when the operands are not numeric, generating that`null == ""` and `[1, 2] == ["1", "2"]` evaluate to true.- In `isFalsy`, the float number `0.0` and the empty maps `{}` are treated as true, while the string `"0"` is treated as false.5. **Fragmented Standard Library:** - Survival of duplicate names (`str_contains` vs `contains`, `len` vs `count` vs`strlen`) and complete absence of object-oriented instance methods in strings and collections.6. **Tooling Decoupled from AST Central:** - The formatter reimplements a standalone lexical scanner, the fixer relies on regular expressions, and the VS Code extension replicates TypeScript parses.--- ## 5. Friction Found When Writing Code ### Evidence 1: Pyramid of Ternaries Nested in Controllers In the actual file [BackupController.joss:10-66](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss#L10-L66) of the reference project, the following chain validation pattern is observed:```joss-snippet
// CÓDIGO REAL EN JOSS-RED-JOSSECURITY:
public func saveOrUpdateBackup(mixed $appName) {
    return ($appName == "otp_backup") ? json({"error": "..."}, 400) : {
        $allowedExtension = $this->getAllowedExtension($appName)

        return (!$allowedExtension) ? json({"error": "..."}, 404) : {
            $file = Request::file("file")
            
            return (!$file) ? json({"error": "..."}, 422) : {
                $originalName = $file["name"]
                $parts = explode(".", $originalName)
                $ext = end($parts)

                return ($ext != $allowedExtension) ? json({"error": "..."}, 422) : {
                    $u = Auth::user()
                    return (!$u) ? json({"error": "..."}, 401) : {
                        // ... Lógica de negocio desplazada a más de 24 espacios de sangría ...
                    }
                }
            }
        }
    }
}
```
**Diagnosis:** The programmer is forced to convert sequential linear validations into a hierarchy of nested false blocks.Any mid-method modification requires reformatting and re-entering dozens of lines of code.### Evidence 2: The Phantom Fake Branch `: {}` On [AuthController.joss:3](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/auth/AuthController.joss#L3) and on multiple controllers:```joss-snippet
// CÓDIGO REAL:
public func showLogin() {
    (!Auth::guest()) ? { return redirect("/dashboard") } : {}
    SEO::title("Iniciar Sesión — Joss Red")
    return view("auth.login", { ... })
}
```
**Diagnosis:** The developer writes `: {}` at the end of each conditional ternary to be sure that the compiler will not interpret the next line as a continuation of the operator.### Evidence 3: Unnecessary Procedural Transformations In [BackupController.joss:70-97](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss#L70-L97):```joss-snippet
// CÓDIGO REAL:
$files = $db->table("backups")->where("user_token", $userToken)->get()
$filesList = $files
$mapped = []

foreach ($filesList as $file) {
    $parts = explode("/", $file["file_name"])
    $fileName = end($parts)
    
    $mapped[] = {
        "id": $file["id"],
        "name": $fileName
    }
}
return json({"files": $mapped})
```
**Diagnosis:** 16 lines of procedural code with mutable accumulator `$mapped[] = ...` to perform an operation that is conceptually a single mapping transformation.--- ## 6. Boilerplate and Ceremony Identified |Ceremonial Pattern |Impact on Lines |Underlying Cause |Proposed Solution ||---|:---:|---|---||**Branch `: {}` in conditional statements** |1 line per conditional |Forced ternary syntax for flow control |Introduction of `guard` or flat conditional statement ||**Parameters marked as `mixed`** |1 per parameter |Lack of flow narrowing in variables with union types |Automatic scope narrowing (smart casts) ||**Manual file name/extension extraction** |3-4 lines |Absence of methods in strings or utility class `Path` |`$archivo->extension()` or `Path::extension($archivo)` ||**Repetitive instantiation `new GranDB()`** |1 line per query |Non-unified instance access with static facades |Unify at `GranDB::table(...)` ||**Cumulative mapping `$acc[] = ...`** |10-15 lines per loop |Arrays without fluent functional methods |`$files->map(func($f) => ...)` ||**Manual pre-read existence check** |2-3 lines |Lack of secure methods in dictionaries/maps |`$map->get("clave", "default")` |--- ## 7. Language Inconsistencies ### 1. Stealth Inconsistency `$` - Local variables: Mandatory (`$usuario = new Usuario()`).- Parameters: Mandatory (`func(string $nombre)`).- Properties in declaration: Mandatory (`public string $nombre = ""`).- Access to instance property: **Prohibited** (`$this->nombre`, not `$this->$nombre`).- Static property access: **Required** (`Clase::$contador`).- Catch parameter: **Required** (`catch ($e)`).### 2. Duplicity of Paradigms in Libraries Three irreconcilable styles coexist in the integrated APIs: - **Historical PHP style:** `str_contains`, `str_replace`, `array_keys`, `file_get_contents`.- **Modern Style brief:** `contains`, `keys`, `values`, `file_read`.- **C++ style:** Stream operators `cout << $val` and `cin >> $var`.- **Web Facade Style:** `Response::json()`, `View::render()`, `Request::input()`.### 3. Asymmetric Typing Requirement - In effect: `func($x)` is strictly prohibited (`JOSS-TYPE-011`);requires `func(mixed $x)`.- At `catch`: `catch (MiError $e)` generates a compiler syntax error;strictly requires `catch ($e)`.### 4. Out of Range Indexing - In arrays and strings: `$arr[99]` causes a **fatal runtime panic** (`JOSS-INDEX-001`).- On maps: `$map["clave_inexistente"]` returns **`null` silently**.--- ## 8. Conceptual Complexity Currently, a developer must memorize an unnecessary number of rules for identical tasks:```text
DECLARACIÓN DE VARIABLES (7 Formas Convivientes):
  1. $x = 10         (inferencia fija)
  2. var $x = 10     (inferencia fija explícita)
  3. int $x = 10     (tipo estático estricto)
  4. let int $x = 10 (tipo estático con prefijo let)
  5. let $x = 10     (variable dinámica mixed)
  6. mixed $x = 10   (variable dinámica explícita)
  7. const $x = 10   (inmutable)
```
**Unification proposal:** Reduce to three intuitive and predictable forms: - `$x = 10` or `var $x = 10`: Fixed inference of the assigned value.- `int $x = 10`: Explicitly declared static type.- `mixed $x = 10`: Voluntary dynamism (recommending `let $x`).- `const $x = 10`: Immutable constant.--- ## 9. Type System Audit The typing core (`pkg/typesystem/types.go`) has solid mathematical foundations: - Strict numerical promotion: `int` → `float` → `decimal`.- Safe operations with normalized (`T|U`) and optional (`T?` → `T|null`) union types.- Nominal compatibility of classes and interfaces.### The Critical Flaw: Isolated Scope Refinement In `pkg/analyzer/infer.go:929` (`narrowScopeFromCondition`), narrowing of nullable types operates by creating isolated scopes:```go
trueScope := newScope(current)
falseScope := newScope(current)
```
These scopes **only affect the expressions located within the ternary**.Once parsing continues to the next statement in the main block, the variable returns to its original type with `null`.```joss-snippet
public func formatearNombre(string? $nombre): string {
    ($nombre == null) ? {
        return "Anónimo"
    }

    // EL COMPILADOR FALLA AQUÍ:
    // $nombre sigue siendo inferido como string|null.
    return $nombre // JOSS-TYPE-008: se esperaba 'string', se obtuvo 'string|null'.
}
```
This limitation prevents writing idiomatic guard clauses and leads to abuse of `mixed` in real software projects.--- ## 10. Security Audit and Runtime Defenses ### Validated and Successful Defenses: - **Overflow Detection:** `CheckedIntBinary` blocks additions or products that exceed the range `int64`.- **Controlled Division by Zero:** Launches immediate diagnostics without producing corrupted memory states.- **Infinite Recursion Protection:** Configurable maximum limit of call frames.### Hidden Gaps and Vulnerabilities: 1. **The Operator `??` Hides Critical Errors:** - At [evaluator_infix.go:54-60](file:///c:/Users/Asus/Documents/proyectos/Joss-language/pkg/core/evaluator_infix.go#L54-L60):   ```go
   if ie.Operator == "??" {
       var left interface{}
       func() {
           defer func() {
               if rec := recover(); rec != nil {
                   left = nil
               }
           }()
           left = r.evaluateExpression(ie.Left)
       }()
   ```
Any exception thrown on the left branch (division by zero, overflow, or database exception) is silently caught and replaced by the default value, making it impossible to diagnose bugs in production.2. **Silent Downgrade of UTF-8 Characters in the Lexer:** - In [lexer.go:327-332](file:///c:/Users/Asus/Documents/proyectos/Joss-language/pkg/parser/lexer.go#L327-L332), the lexer discards without notification any bytes greater than 127 outside of string literals.An identifier like `$año` is silently transformed into `$ao`.--- ## 11. Error Audit and Diagnostics The `pkg/diagnostics` subsystem is of high architectural quality, but requires greater contextual empathy: 1. **Technically Correct but Not Guidance Messages:** - *Current:* `error[JOSS-TYPE-001] app.joss:15:5: Asignación incompatible: se esperaba 'int', se obtuvo 'string'.` - *Guidance:*`error[JOSS-TYPE-001] app.joss:15:5: No se puede asignar 'string' a la variable '$edad' porque fue inferida como 'int' en la línea 4. Sugerencia: Modifica el valor asignado o declara explícitamente 'mixed $edad'.` 2. **Cascade of Errors in Parser:** - In the absence of a delimiter or key, the Pratt parser issues multiple derived diagnoses.It is a priority to synchronize the parser to the following `SEMICOLON` or `NEWLINE`.--- ## 12. Standard Library Audit (Stdlib) ### Main Redesign Opportunities: 1. **Unification of Strings and Object-Oriented Collections:** - Replace nested functions with fluid calls:     ```joss-snippet
     // Actual:
     $limpio = trim(strtolower(substr($texto, 0, 10)))

     // Propuesto:
     $limpio = $texto->slice(0, 10)->lower()->trim()
     ```
2. **Lazy Evaluation of the Range Operator (`..`):** - Modify `evaluator_infix.go:458` so that `1..1000000` returns a lightweight iterator instead of immediately allocating an array of one million elements in the heap.--- ## 13. Tooling and Ecosystem Audit - **`joss run` and `joss analyze`:** They work flawlessly, ensuring that no code with semantic analysis errors is executed.- **VS Code Extension (`vscode-joss`):** Duplicates the path parsing and syntax validation logic in TypeScript.It should evolve towards a pure Language Server in Go powered by `pkg/analyzer`.--- ## 14. Official Formatter Audit - The file `pkg/formatter/scanner.go` maintains a parallel token table that diverges from `pkg/parser/token.go`.- The formatter should be refactored to operate directly on the token sequence generated by the canonical lexer, preserving intentional line breaks and applying strict canonical formatting a la `gofmt`.--- ## 15. Analyzer and Linter Audit It is recommended to clearly segregate the severity levels: - **Analyzer:** Validates the correctness of the program (types, symbols, call termination, invariants).It broadcasts exclusively `Error` and `Warning`.- **Linter:** Evaluates style and good practices (naming convention, key detection in hard code `JOSS-SEC-001`).Issues `Warning` and `Info`.--- ## 16. Selective Comparison with Other Languages ​​|Language |Flow Control Approach |Nullability Management |Lesson for Joss ||---|---|---|---||**Dart** |Maintains traditional `if/else` |Sound Null Safety with Flow Promotion |Allows you to write linear code while the compiler removes `null` after a save.||**Go** |Simple statements `if err != nil` |Explicit error pointers and tuples |Linear code is more readable than nested expressions.||**Swift** |Mandatory sentence `guard cond else { return }` |Strict optionals (`T?`) |The `guard` clause guarantees the function output without pyramid indentation.||**Kotlin** |`if` is expression;supports *Smart Casts* |Nullable types `T?` with Elvis operator `?:` |If a variable is checked against `null`, the compiler should update its type automatically.|--- ## 17. Simplification Opportunities 1. **Remove Obsolete Aliases from Functions:** Gradually deprecate the prefixes `str_` and `array_`.2. **Discourage `let $x` in favor of `mixed $x`:** Remove conceptual ambiguity about type mutability.3. **Unify Facades and Helpers:** Ensure that `view()` and `View::render()` share the same formal signature contract.--- ## 18. Features that should NOT be implemented It is recommended to explicitly reject: - ❌ **Traditional Imports Graphs:** Preserve the simplicity of the Zero-Imports model.- ❌ **Complex Higher Order Generics:** Keep the parameterization limited to `array<T>` and `map<K, V>`.- ❌ **Multiple Inheritance or Complex Traits:** Preserve simple inheritance with clean interfaces.- ❌ **Function Coloring in Async:** Prohibit requiring keywords `async func`.--- ## 19. Catalog of Prioritized Proposals (P1 to P8) Below is the technical catalog of proposals for the evolution of the Joss language, prioritized according to their impact on the reduction of accidental complexity, elimination of defensive code and direct improvement of developer productivity and security.### Decision and Prioritization Matrix |ID |Proposal |Main Problem |Benefit |Complexity |Risk |Compatibility |Priority |Subsystem ||:---|:---|:---|:---|:---:|:---:|:---:|:---:|:---||**P1** |**Sentence `guard`** |Nesting pyramids by previous validations |Linear control flow and sequential reading |Medium |Low |100% Compatible |**P0** |Parser, Analyzer, Evaluator ||**P2** |**Type Narrowing Propagation** |Loss of rate refinement after early exits |Eliminate redundant assertions and checks |Medium |Low |100% Compatible |**P0** |Analyzer (`flow.go`, `infer.go`) ||**P3** |**Fluid Methods in Primitives** |Friction due to mixing between procedural PHP and static classes |Modern API, autocompletion and chaining |Medium |Low |100% Compatible |**P1** |Evaluator, Stdlib, Analyzer ||**P4** |**Preservation of Objects at `catch`** |Exceptions downgraded to `string` flat in catch |Typed error handling, access to trace and metadata |Low |Minimum |100% Compatible |**P0** |Evaluator (`executor.go`) ||**P5** |**Coalescence Security `??`** |Indiscriminate silencing of real panics |Immediate detection of logical bugs and divisions by zero |Low |Low |Compatible (semantic fix) |**P1** |Evaluator (`evaluator_infix.go`) ||**P6** |**Declarative Destructuring** |5 to 10 repetitive unpacking lines per controller |60% reduction in boilerplate allocation |Medium |Low |100% Compatible |**P1** |Parser, Analyzer, Evaluator ||**P7** |**Property Promotion at `Init`** |Quadruple declaration of properties in classes and DTOs |Concise and declarative definition of models |Medium |Minimum |100% Compatible |**P1** |Parser, Analyzer, Core (`classes.go`) ||**P8** |**Autofix AST Engine (`joss fix`)** |Technical debt and manual syntax fixes |Automatic migration and modernization with 0 effort |Medium |Low |External CLI Tool |**P0** |CLI, Fixer, Formatter |--- ### P1: Guard Statement and Flat Flow Control (`guard`) #### 1. Current Problem In Joss controllers, middlewares and services, the methods require checking multiple preconditions (authentication, existence of records in database, validity of tokens, presence of uploaded files).Since developers often use ternaries with blocks or nested conditionals to avoid duplicate returns, the code collapses into the so-called "pyramid of doom."Each validation adds an extra level of indentation and encloses the main logic within the body of the false or true branch.#### 2. Evidence in Real Code - `ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss`: Up to 5 consecutive ternary nesting levels to verify `$user`, role permissions, backup existence and request parameters.- `ejemplos/Joss-Red-JosSecurity/app/controllers/api/ApiRepositoryController.joss` (lines 60-80): Three ternaries are nested to verify if the file exists, if the JSON is parsed and if the optional fields are present.- `ejemplos/Joss-Red-JosSecurity/app/middleware/MiddlewareLoader.joss` (lines 30-93): Defensive validation blocks that force read skips.#### 3. Developer Impact - **Severely degraded readability:** The happy business logic (*happy path*) is hidden at the deepest level of indentation.- **Risk of errors in refactoring:** Modifying a nested block requires adjusting paired braces dozens of lines apart.- **Audit difficulty:** It is not obvious at first glance what the prerequisites are for an endpoint to execute its core logic.#### 4. Proposed Technical Solution Incorporate the sentence `guard (condición) else { ... }`.- **Semantics:** The condition must evaluate to boolean.If the condition is true, execution continues immediately on the next statement at the same indentation level.If false, the block `else` is executed.- **Static invariant:** The semantic analyzer (`pkg/analyzer`) exhaustively requires that the block `else` of a `guard` terminate the function flow (via `return`,`throw` or terminal output).If the `else` block does not break the flow, the compiler issues a static error `JOSS-FLOW-005`.#### 5. Current Code vs.Proposed Code```joss-snippet
// --- ACTUAL (Anidamiento y ramas de escape complejas) ---
public func download(mixed $id) {
    $item = GranDB::table("repos")->where("id", $id)->first()
    return (!$item) ? json({"error": "No encontrado"}, 404) : {
        $user = Auth::user()
        return (!$user) ? json({"error": "No autenticado"}, 401) : {
            $path = $item["file_path"]
            return (!file_exists($path)) ? json({"error": "Archivo perdido"}, 404) : {
                return Response::download($path)
            }
        }
    }
}

// --- PROPUESTO (Flujo plano y lectura lineal con guard) ---
public func download(mixed $id) {
    $item = GranDB::table("repos")->where("id", $id)->first()
    guard ($item != null) else {
        return json({"error": "No encontrado"}, 404)
    }

    $user = Auth::user()
    guard ($user != null) else {
        return json({"error": "No autenticado"}, 401)
    }

    $path = $item["file_path"]
    guard (file_exists($path)) else {
        return json({"error": "Archivo perdido"}, 404)
    }

    return Response::download($path)
}
```
#### 6. Affected Subsystem - `pkg/parser`: New keyword `guard`, AST node `GuardStatement`.- `pkg/analyzer`: Boolean condition validation and guaranteed completion check in block `else` using `flow.go:blockTerminatesCallable`.- `pkg/core`: Evaluation at `executor.go` (if `isTruthy(cond)`, continue; if not, evaluate `elseBlock`).#### 7. Estimation and Compatibility - **Benefit:** Very High (Eliminates 80% of accidental nesting in controllers).- **Difficulty:** Medium.- **Risk:** Low.- **Compatibility:** 100% backward compatible (contextual or reserved keyword with verification at `token.go`).- **Priority:** **P0**.--- ### P2: Propagation of Type Narrowing in the Main Scope (*Smart Casts*) #### 2.1.Current Problem The Joss type system implements type refinement (`narrowScopeFromCondition` in `pkg/analyzer/infer.go`), but only within the inner body of a branch `if` or the true/false arm of a ternary operator.When a developer validates the null of a variable at the start of a function and immediately returns if it is null, the semantic analyzer **forgets the refinement** on subsequent lines of the main scope.#### 2.2.Evidence in the Royal Code - `ejemplos/Joss-Red-JosSecurity/app/controllers/auth/ProfileController.joss` (line 18-28):  ```joss-snippet
  $u = Auth::user() // Tipo inferido: User|null
  (!$u) ? { return redirect("/login") }
  // En las siguientes líneas, $u->email o $u->first_name siguen considerando User|null
  ```
- `pkg/analyzer/infer.go` (lines 90-96): `narrowScopeFromCondition` generates two isolated child scopes (`trueScope`, `falseScope`), but neither of them feeds back to the parent scope`current`.#### 23.Developer Impact - Developer is forced to use `mixed` in variables to silence analyzer warnings.- Causes distrust in the type system, incentivizing duplicate defensive checks (`if ($u != null && $u->email)`) throughout the same method.#### 2.4.Proposed Technical Solution Integrate flow termination analysis (`flow.go`) with scope refinement in `infer.go`: - When a conditional statement (`if`, `guard`, or statement ternary) comprehensively demonstrates that itsescaping branch terminates the execution of the function (`return`, `throw`), the subsequent parent scope assumes the refined type of the unescaped branch.- Example: if `$x` is `User|null` and you run `if ($x == null) { return }`, the type of `$x` in the main scope automatically becomes `User`.#### 2.5.Current Code vs.Proposed Code```joss-snippet
// --- ACTUAL: Analyzer reporta posible desreferencia nula en $user->email ---
public func getEmail(): string {
    User|null $user = Auth::user()
    if ($user == null) {
        return ""
    }
    // El analyzer todavía considera que $user puede ser null
    return $user->email // Genera fricción o exige let mixed
}

// --- PROPUESTO: Smart Cast automático en flujo secuencial ---
public func getEmail(): string {
    User|null $user = Auth::user()
    if ($user == null) {
        return ""
    }
    // Type Narrowing propagado al scope exterior: $user promovido a User estricto
    return $user->email // 100% tipado, autocompletado y validado
}
```
#### 2.6.Affected Subsystem - `pkg/analyzer/flow.go`: Expose `statementTerminatesCallable(stmt parser.Statement) bool`.- `pkg/analyzer/infer.go`: In `inferStatement`, if a conditional block is escaped, apply the type mutations of `narrowScopeFromCondition` on the `currentScope`.#### 2.7.Estimation and Compatibility - **Benefit:** Very High (The type system works for the developer, not against them).- **Difficulty:** Medium.- **Risk:** Low.- **Compatibility:** 100% backwards compatible (does not invalidate existing valid code; only resolves false positives).- **Priority:** **P0**.--- ### P3: Fluent Instance Methods in Primitives (`string`, `array`, `map`) #### 3.1.Current Problem There is currently a confusing dichotomy in the standard library and basic types: 1. Procedural-style global functions inherited from PHP (`strlen`, `str_contains`, `substr`, `array_keys`,`array_push`, `json_encode`).2. Native classes with static methods (`Str::contains`, `Str::length`, `Arr::has`, `JSON::encode`).The developer must constantly memorize when to call a global function, when to use a static class, and in what order the parameters go (e.g. `$needle` vs `$haystack`).Chained data transformations are made unreadable by nesting of calls inward.#### 3.2.Evidence in the Royal Code - `ejemplos/Joss-Red-JosSecurity/app/services/PageBuilderService.joss`:  ```joss-snippet
  $clean = Str::trim(Str::lower(Str::replace($input, " ", "-")))
  ```
Reading is done from the inside out, requiring 3 static calls for a trivial operation on a string.- `ShopController.joss`: List manipulations that combine `count($items)`, `array_slice($items, ...)` and `Arr::map(...)`.#### 3.3.Developer Impact - Continued cognitive friction from switching between incompatible syntactic styles.- Lack of natural autocompletion in the editor: When writing `$cadena->`, the LSP cannot offer fluid transformation methods because the primitives do not expose instance methods.#### 3.4.Proposed Technical Solution Enable invocation of virtual instance methods directly on values of type `string`, `array` and `map`: - `$cadena->trim()->lower()->replace(" ", "-")` - `$lista->map(fn($x) => $x * 2)->filter(fn($x) => $x > 10)->join(", ")` -`$mapa->keys()`, `$mapa->values()`, `$mapa->has("clave")` - **Free implementation:** At `pkg/core/evaluator_member.go`, if the receiver is a native Go primitive value (`string`,`[]interface{}`, `map[string]interface{}`), dispatch internally to the already optimized evaluators of `StrMethods` and `ArrMethods` without creating intermediate wrapper objects.#### 3.5.Current Code vs.Proposed Code```joss-snippet
// --- ACTUAL: Llamadas estáticas anidadas de adentro hacia afuera ---
public func slugify(string $title): string {
    return Str::lower(Str::trim(Str::replace($title, " ", "-")))
}

// --- PROPUESTO: Encadenamiento fluido natural de izquierda a derecha ---
public func slugify(string $title): string {
    return $title->trim()->replace(" ", "-")->lower()
}
```
#### 3.6.Affected Subsystem - `pkg/core/evaluator_member.go`: Intercept member calls on non-instantiated types and index in native primitive dispatchers.- `pkg/analyzer/infer.go`: Declare return signatures for members of types `String`, `Array` and `Map`.- `tools/cataloggen`: Export primitive methods to the VS Code autocomplete catalog.#### 3.7.Estimation and Compatibility - **Benefit:** Very High (Dramatically modernizes the ergonomics of the language).- **Difficulty:** Medium.- **Risk:** Low.- **Compatibility:** 100% compatible.Global functions and static classes continue to exist without alterations.- **Priority:** **P1**.--- ### P4: Preservation of Object Instances and Exceptions at `catch` #### 4.1.Current Issue In the current Joss runtime implementation (`pkg/core/executor.go`, line 459), when an exception is caught using a `try / catch ($ex)` block, the returned value is forcibly converted to a plain text string using `fmt.Sprint(evalErr.Value)`.If the developer threw a class instance (`throw new ValidationException("Error", 422, $errores)`), the block `catch` receives an empty string or an inert formatted representation (`"Instance of ValidationException"`), instead of the live instance of the object.#### 4.2.Evidence in the Royal Code - `pkg/core/executor.go` (line 459):  ```go
  r.currentEnvironment.Set(node.Variable.Value, fmt.Sprint(evalErr.Value))
  ```
- `ejemplos/Joss-Red-JosSecurity/app/controllers/web/FlaskController.joss` (lines 28-30):  ```joss-snippet
  } catch ($ex) {
      return json({"error": "Plugin error: " . $ex}, 500)
  }
  ```
Developers cannot access `$ex->getCode()` or inspect specific properties because `$ex` is always a string.#### 4.3.Developer Impact - Inability to implement granular exception handling by type (`if ($ex instanceof NotFoundException)`).- Irremediable loss of the stack trace, associated HTTP status codes, metadata and structured failure context.#### 4.4.Proposed Technical Solution - In `pkg/core/executor.go`, directly assign the value `evalErr.Value` (be it `*Instance`, `error`, map or string) to the frame of the block's lexical environment`catch`.- If the thrown value is a generic error or a string, transparently wrap it in an instance of the native base class `Exception` with methods `$ex->getMessage()` and `$ex->getFile()`.#### 4.5.Current Code vs.Proposed Code```joss-snippet
// --- ACTUAL: $ex es forzado a string plano, sin métodos ni propiedades ---
try {
    PaymentGateway::charge($amount)
} catch ($ex) {
    // $ex es string: "CardDeclinedException"
    // $ex->getCode() provoca error de runtime
    return json({"error": $ex}, 500)
}

// --- PROPUESTO: $ex preserva la instancia original lanzada ---
try {
    PaymentGateway::charge($amount)
} catch ($ex) {
    if ($ex instanceof CardDeclinedException) {
        return json({"error": $ex->getMessage(), "decline_code": $ex->declineCode}, 402)
    }
    return json({"error": $ex->getMessage()}, 500)
}
```
#### 4.6.Affected Subsystem - `pkg/core/executor.go`: Replace coercion `fmt.Sprint` with direct value assignment and consistent packaging at `Exception`.- `pkg/core/classes.go`: Ensure that the base class `Exception` offers canonical methods `getMessage()`, `getCode()`, `getLine()`,`getFile()`.#### 4.7.Estimation and Compatibility - **Benefit:** Critical (Restores the integrity of the object-oriented paradigm in error management).- **Difficulty:** Low.- **Risk:** Minimal.- **Compatibility:** 100% compatible (concatenating `$ex` as string still works thanks to `CoerceString`).- **Priority:** **P0** (Immediate correction).--- ### P5: Safety in Coalescent Operator `??` and Non-Panic Silencer Rescue #### 5.1.Current Issue The null-coalescing operator `??` is designed to provide an alternative value when an expression evaluates to `null` or accesses an undefined index/property on a collection.However, in the current implementation (`pkg/core/evaluator_infix.go`, lines 88-94), the evaluation of the left operand is wrapped in an indiscriminate `recover()` that traps any Go panic or Joss exception, silencing serious programming errors like division by zero, invalid types, or internal logic errors.#### 5.2.Evidence in the Royal Code - `pkg/core/evaluator_infix.go` (lines 88-94):  ```go
  defer func() {
      if r := recover(); r != nil {
          result = r.evaluateExpression(ie.Right)
      }
  }()
  ```
- If a developer types `$val = ($total / $count) ?? 0` and `$count` is 0, instead of alerting about the failure or forbidden split, the operator silently hides the error and returns the right value.#### 5.3.Impact on the Developer - **Silent bugs that are difficult to debug:** Critical defects in algorithms go unnoticed because `??` catches any exceptions that occur in the evaluation of complex expressions on its left side.- Violates the principle of least surprise and Joss's robustness guarantees.#### 5.4.Proposed Technical Solution - Refactor the `??` evaluator so that it only recovers missing value errors (`UndefinedVariable`, `MissingKeyError` or return value equal to`nil`/`NullValue`).- Fatal runtime errors (explicitly thrown exceptions, non-existent method invocation errors, or strict type failures) should not be consumed by `??` and should be propagated to the upper error handler or block `catch`.#### 5.5.Current Code vs.Proposed Code```joss-snippet
// --- ACTUAL: Silenciamiento accidental de fallos de ejecución ---
// Si calculateDiscount() tiene un bug y arroja excepción, ?? lo oculta
$precioFinal = calculateDiscount($producto) ?? 0 // Devuelve 0 en silencio

// --- PROPUESTO: Coalescencia estricta solo ante null/no definido ---
// Si calculateDiscount() retorna null, asigna 0.
// Si calculateDiscount() arroja una excepción, la excepción se propaga y se diagnostica.
$precioFinal = calculateDiscount($producto) ?? 0
```
#### 5.6.Affected Subsystem - `pkg/core/evaluator_infix.go`: Delete blind `recover()` at `evaluateNullCoalesceExpression` and evaluate the value by checking if it is null or index not found.#### 5.7.Estimation and Compatibility - **Benefit:** High (Prevents silent failures in production).- **Difficulty:** Low.- **Risk:** Low.- **Compatibility:** Compatible (improves semantic correctness without breaking idiomatic code).- **Priority:** **P1**.--- ### P6: Declarative Destructuring of Tuples, Arrays and Maps #### 6.1.Current Problem In Joss web applications (like the actual JosSecurity project), controller and service methods constantly receive maps or tuples with multiple values ​​(form data, headers, validation results, 2FA secrets).To extract these values, the developer is forced to write 5-10 individual repetitive mappings line by line.#### 6.2.Evidence in the Royal Code - `ejemplos/Joss-Red-JosSecurity/app/controllers/auth/ProfileController.joss` (lines 43-48):  ```joss-snippet
  $first_name = request("first_name")
  $last_name  = request("last_name")
  $phone      = request("phone")
  $password   = request("password")
  ```
- `AuthController.joss` (lines 125-140): Manual unpacking of arrays returned by authentication and 2FA services (`$secret = $totp["secret"]`, `$qrCode = $totp["qr_url"]`).#### 6.3.Developer Impact - Large volume of ceremonial and repetitive code.- Increased typos in manual matching between variable name and map key.#### 6.4.Proposed Technical Solution Incorporate destructuring patterns (*destructuring assignment*) in assignment and declaration statements: 1. **Destructuring of Lists/Tuples by position:** `[$id, $nombre, $rol] = $usuarioArray` 2. **Destructuring of Maps by key:** `{"email": $email, "password": $password} = request()` 3. **Optional default values:**`{"role": $role = "cliente", "active": $active = true} = $data` #### 6.5.Current Code vs.Proposed Code```joss-snippet
// --- ACTUAL: 6 líneas ceremoniales de extracción individual ---
public func registerUser() {
    $req = request()
    $name = $req["name"]
    $email = $req["email"]
    $password = $req["password"]
    $role = $req["role"] ?? "user"
    $terms = $req["terms"] ?? false
}

// --- PROPUESTO: 1 sola línea declarativa y expresiva ---
public func registerUser() {
    {"name": $name, "email": $email, "password": $password, "role": $role = "user", "terms": $terms = false} = request()
}
```
#### 6.6.Affected Subsystem - `pkg/parser`: Support for patterns `ArrayPattern` and `MapPattern` on the left side of assignment statements (`parser_statements.go`).- `pkg/analyzer`: Typing and inference of each individual variable from the container type (`infer.go`).- `pkg/core/executor.go`: Sequential assignment of slots from the iterated or indexed object.#### 6.7.Estimation and Compatibility - **Benefit:** Very High (Reduces controller boilerplate by 60%).- **Difficulty:** Medium.- **Risk:** Low.- **Compatibility:** 100% compatible (new syntactic construction without lexical conflicts).- **Priority:** **P1**.--- ### P7: Promotion of Properties in Builder (`Init`) #### 7.1.Current Problem To create simple domain classes, DTOs (*Data Transfer Objects*), entities or services in Joss, the programmer must declare the field name in four different places: 1. As a class property (`public string $name`).2. As a parameter in the constructor `Init(string $name)`.3. As an assignment to `$this` in the constructor body (`$this->name = $name`).4. In the documentation or return types.#### 7.2.Evidence in Real Code - `ejemplos/Joss-Red-JosSecurity/app/services/LicenseService.joss`: Multiple service classes with 5 or more identically assigned properties in the constructor.- `ejemplos/plugins/joss_ai/src/plugin.joss`: Repetition of configuration parameters assigned one by one to `$this->propiedad`.#### 7.3.Developer Impact - Resistance to creating strongly typed DTOs due to the ceremonial verbosity required for each class.- Slow refactorings: renaming a property requires modifying multiple points within the same file.#### 7.4.Proposed Technical Solution Allow visibility (`public`, `protected`, `private`) and constancy (`const`) modifiers directly in the parameters of the constructor function`Init`: - When declaring a parameter with visibility (e.g. `func Init(public string $titulo, private GranDB $db = new GranDB())`), the compiler and runtime automatically declare the property in the class and generate the assignment `$this->titulo = $titulo` before executing the body of `Init`.#### 7.5.Current Code vs.Proposed Code```joss-snippet
// --- ACTUAL: Cuádruple repetición del identificador ---
public class UserDTO {
    public int $id
    public string $email
    public string $role

    public func Init(int $id, string $email, string $role) {
        $this->id = $id
        $this->email = $email
        $this->role = $role
    }
}

// --- PROPUESTO: Constructor conciso con promoción de propiedades ---
public class UserDTO {
    public func Init(
        public int $id,
        public string $email,
        public string $role = "cliente"
    ) {}
}
```
#### 7.6.Affected Subsystem - `pkg/parser`: Recognize `public`, `protected`, `private` before type in function parameter list `Init`.- `pkg/analyzer`: Synthesize class properties from the promoted parameters.- `pkg/core/classes.go`: Automatic instantiation of slots in the construction of the object.#### 7.7.Estimation and Compatibility - **Benefit:** Very High in object-oriented ergonomics and clean architecture.- **Difficulty:** Medium.- **Risk:** Minimal.- **Compatibility:** 100% compatible.The traditional syntax of `Init` still works exactly the same.- **Priority:** **P1**.--- ### P8: Mechanical Autofix Engine Based on AST (`joss fix`) #### 8.1.Current Problem The evolution of a language inevitably generates technical debt in real projects when patterns are modernized (such as the 370 empty branches `: {}` that we just cleaned up in JosSecurity, or the requirement for explicit visibility in global functions and methods).Currently, developers rely on manual regular expression searches, which introduces risks of modifying text within literal strings or comments.#### 8.2.Evidence in Real Code - The massive presence of `: {}` branches across 65 files in JosSecurity demonstrated that programmers retain old syntactic habits in the absence of an official modernization tool.- Multiple stable compiler diagnostics (`JOSS-VIS-001`, `JOSS-TYPE-009`, `JOSS-DEPR-001`) already calculate precise solution suggestions (`Diagnostic.Suggestion`), but today they are only printed to the console without being able to be automatically applied to the source code.#### 8.3.Impact on the Developer - Friction and delay in the adoption of new versions of Joss.- Fear of refactoring or updating the compiler due to the manual workload of correcting style warnings or deprecations.#### 8.4.Proposed Technical Solution Consolidate the CLI command `joss fix [directorio]` as a mechanical rewriting engine based directly on the Abstract Syntax Tree (AST) and the diagnostics table: 1. **Structured detection:** The semantic analyzer identifies diagnostics that have a `FixAvailable` or standardized suggestion.2. **Safe transformation:** Instead of replacing text using regular expressions, the fixer replaces specific nodes in the AST or applies text deltas delimited by exact ranges of tokens (`Token.StartLine`, `Token.StartCol`, `Token.EndCol`).3. **Preserved formatting:** Upon completion of applying fixes, automatically invokes the `pkg/formatter` engine to ensure that the project's indentation and styling remain consistent.4. **Initial rules supported in `joss fix`:** - Automatic removal of redundant empty branches `: {}`.- Insertion of automatic visibility modifiers `public` where they are missing.- Replacement of deprecated global functions with canonical calls to static classes (`str_contains` -> `Str::contains`).- Normalization of deprecated historical types (`list` -> `array`, `dynamic` -> `mixed`).#### 8.5.Terminal Workflow Example```bash
# Diagnosticar problemas corregibles automáticamente en el proyecto
joss check ./app

# Aplicar correcciones mecánicas automáticas con informe detallado
joss fix ./app

# Salida esperada:
# [joss fix] Analizando 65 archivos...
# [joss fix] Eliminadas 370 ramas ternarias vacías ': {}' innecesarias.
# [joss fix] Actualizadas 14 llamadas deprecadas a métodos canónicos de Str/Arr.
# [joss fix] Formateado completado satisfactoriamente. 0 errores restantes.
```
#### 8.6.Affected Subsystem - `cmd/joss/fix.go`: CLI subcommand and project orchestrator.- `pkg/fixer/`: Token delta rewriting engine on the AST.- `pkg/diagnostics`: Incorporation of the optional field `TextEdit` in `Diagnostic`.#### 8.7.Estimation and Compatibility - **Benefit:** Extraordinary for the health of the ecosystem and developer loyalty.- **Difficulty:** Medium.- **Risk:** Low.- **Compatibility:** 100% compatible (opt-in tool that does not modify the semantics of the language).- **Priority:** **P0** (Essential for the life cycle of the language).--- --- ## 20. Changes That Would Require Deprecation - Flag with diagnostic warning `JOSS-DEPR-001` the procedural names of global functions that duplicate modern names (`str_contains`, `array_keys`, `file_get_contents`).- Offer mechanical autofix through `joss fix`.--- ## 21. Possible Justified Breaking Changes 1. **Restriction from `null == ""` to `false`:** No null type should evaluate as equivalent to an empty string under ordinary equality.2. **Panic Propagation at `??`:** Eradicate silent trapping of fatal errors on the operator's left branch.--- ## 22. Proposal and Architecture of `joss fix` Modernize the CLI tool to use syntax tree transformations (AST rewrite) instead of regex: - Automatic insertion of required visibility modifiers.- Replacement of calls to deprecated functions with their canonical equivalents.- Automatic removal of redundant empty `: {}` branches.- Automatic launch of the official formatter upon completion of the repair.--- ## 23. Definition of "Idiomatic Joss Code" Idiomatic Joss code is defined by the following characteristics: 1. **Declarative and Secure Typing:** Explicit public contracts, clean inference on local variables, and use of `mixed` reserved for dynamic input boundaries.2. **Linear Flow without Nesting:** Disciplined use of guard clauses with early termination.3. **Fluid Expressiveness:** Use of the pipeline operator `|>` and chained calls on collections.4. **Structured Error Handling:** Exceptions represented as domain objects, avoiding flat text strings.--- ## 24. Recommended Roadmap (Phases 0 to 5) |Phase |Title |Main Initiatives |Priority |Impact ||---|---|---|:---:|---||**Phase 0** |**Immediate Corrections** |Preserve instances at `catch ($e)`;do not silence fatal panics at `??`;report invalid UTF-8 characters in lexer.|**P0** |Critical ||**Phase 1** |**Quick Wins** |Flow-sensitive narrowing after returns in the analyzer;correction of `isFalsy` (treat `0.0` and `{}` as false);coercion of numerical keys in maps.|**P0** |Very High ||**Phase 2** |**Unified Tooling** |Refactor `pkg/formatter` using the canonical lexer;modernize `joss fix` with AST rewriting.|**P1** |High ||**Phase 3** |**Collections Ergonomics** |Native instance methods on strings and arrays;lazy operator `..` (lazy iterator).|**P1** |High ||**Phase 4** |**Language Evolution** |Judgment `guard (...) else { ... }`;support for `catch (TipoException $e)` and block `finally`.|**P2** |Very High ||**Phase 5** |**Future Architecture** |Native unified LSP server in Go;gradual connection of `pkg/vm` for bytecode optimization.|**P3** |Strategic |--- *End of Audit Document 2026. This report constitutes the official reference base for making design decisions and technical evolution of Joss.*
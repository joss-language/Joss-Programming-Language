# Catalog and reference of language diagnoses

Before: [Error and exception handling](ERRORES.md). After: [AST Semantic Analyzer](ANALIZADOR.md).
General index: [Joss documentation](README.md).

---

## What is a diagnosis in Joss?

A **diagnosis** is a structured technical report issued by Joss tools (the compiler, the semantic analyzer `joss analyze`, the linter, or the defensive runtime) when a violation of the language's syntax, type, or safety rules is detected.

Unlike generic error messages from older tools, each Joss diagnostic is designed following these principles:
1. **Unique and stable identifier**: Each rule has an immutable code (for example `JOSS-TYPE-001`).
2. **Exact location**: Precise file, line and column where the conflict originated.
3. **Pedagogical explanation**: Clearly describe which rule was broken and why.
4. **Actionable Suggestion**: Proposes the recommended canonical correction.

### Severity levels

- **`error`**: Prevents the execution of the program (`joss run` and the compiler stops). It represents a structural or safety failure.
- **`warning`**: Informational notice (such as a declared variable that was never used or a misaligned naming convention). `joss analyze` ends with exit code `0` if there are only warnings, allowing execution to continue.

---

## 1. Parser, project loading and symbol table

| Code | Category | Meaning and Cause | Wrong example | Solution and Correct Case |
|---|---|---|---|---|
| `JOSS-IO-001` | Entry/Exit | Cannot read the source file on disk (permissions or path does not exist). | `joss run fantasma.joss` | Verify that the file exists in the indicated path with read permissions. |
| `JOSS-PARSE-001` | Syntax | Invalid token or grammatical structure: missing quotation mark, curly brace, or required visibility. | `class MiClase {}`<br>`func prueba() {}` | Use canonical syntax:<br>`public class MiClase {}`<br>`public func prueba() {}` |
| `JOSS-SYM-001` | Symbols | Undefined variable: an attempt is made to read a variable before being created. | `print($variable)` | Declare and initialize the variable before using it:<br>`$variable = "valor"` |
| `JOSS-SYM-002` | Symbols | Redeclaration of a variable in the same lexical scope. | `int $x = 1`<br>`int $x = 2` | Reassign without redeclaring:<br>`$x = 2` |
| `JOSS-SYM-003` | Symbols | Unresolved function: A function that does not exist in the project or built-ins is invoked. | `calcularTotal()` | Define the function with `public func` or check the spelling of the name. |
| `JOSS-SYM-004` | Symbols | Unresolved class: Attempt to instantiate (`new`) an undeclared and unregistered class. | `$p = new Persona()` | Declare `public class Persona {}` or load the plugin that exposes it. |
| `JOSS-SYM-005` | Symbols | Invalid inheritance: The class specified in `extends` does not exist. | `public class A extends B {}` | Make sure the superclass `B` is declared in the project. |
| `JOSS-SYM-006` | Symbols | Attempt to reassign an immutable constant. | `const int $MAX = 5`<br>`$MAX = 10` | If the value must change, declare it as a mutable variable: `$MAX = 5`. |
| `JOSS-SYM-007` | Symbols | Unresolved interface: The interface specified in `implements` or `extends` does not exist. | `public class A implements IDesconocida {}` | Make sure you declare the interface or check your spelling. |
| `JOSS-SYM-008` | Symbols | Cyclic inheritance in interfaces: An interface extends itself directly or indirectly. | `public interface A extends B {}`<br>`public interface B extends A {}` | Break the inheritance cycle between interfaces. |
| `JOSS-DECL-001` | Declarations | Name conflict: Two global functions have exactly the same identifier. | Two files with:<br>`public func procesar() {}` | Rename one of the two functions. In Joss there are no source namespaces per folder. |
| `JOSS-DECL-002` | Declarations | Class conflict: Two global classes have the same name in the project. | Two files with:<br>`public class Usuario {}` | Maintain a single canonical declaration of the class throughout the project. |
| `JOSS-DECL-003` | Declarations | Duplicate methods: the same class declares two methods with the same name. | `public func id() {}`<br>`public func id(int $x) {}` | Joss does not support method overloading by signature; Use different descriptive names. |
| `JOSS-DECL-004` | Declarations | Name conflict between interface and class or duplicate interfaces. | `public class Repo {}`<br>`public interface Repo {}` | Use unique names for each type; Suggested convention: Prefix `I` for interfaces (`IRepo`). |
| `JOSS-DECL-005` | Declarations | Breach of interface contract: a method is missing, or differs in number/type of parameters, return or visibility. | `public class MiClase implements IFigura {}` (without `calcularArea()`) | Implement all interface methods with `public` visibility and compatible types. |

---

## 2. Type system, calls and accessibility

| Code | Category | Meaning and Cause | Wrong example | Solution and Correct Case |
|---|---|---|---|---|
| `JOSS-TYPE-001` | Types | Incompatible reassignment: Data of a different type is assigned to a typed or inferred variable. | `$edad = 20`<br>`$edad = "veinte"` | Keep the type homogeneous, or use voluntary dynamism:<br>`mixed $edad = 20`<br>`$edad = "veinte"` |
| `JOSS-TYPE-002` | Types | Initial value incompatible with explicit type annotation. | `int $x = "texto"` | Provide a value of the expected type or a convertible string according to `CoerceString`. |
| `JOSS-TYPE-003` | Types | Argument incompatible with the typed parameter of a function or method. | `public func f(int $n) {}`<br>`f("hola")` | Pass the correct type or cast the argument before the call. |
| `JOSS-TYPE-004` | Types | Operator applied to unsupported operands (e.g. adding text with `+`). | `"hola" + " mundo"` | For arithmetic use numbers; To join text use the dot operator (`.`): `"hola" . " mundo"`. |
| `JOSS-TYPE-005` | Types | Key type not supported in an associative map. | `{[1, 2]: "valor"}` | The keys of a `map` must be of type `string`. |
| `JOSS-TYPE-006` | Types | Incorrect index type for a known collection. | `$arr["clave"]` (in an array)<br>`$map[true]` (in a map) | Arrays are indexed with integers (`$arr[0]`); maps with strings (`$map["clave"]`). |
| `JOSS-TYPE-007` | Types | Attempt to index with `[...]` a value that is not indexable (e.g. an integer or boolean). | `$n = 42`<br>`print($n[0])` | Index only compatible arrays, maps, strings or instances. |
| `JOSS-TYPE-008` | Types | The value returned by `return` does not match the type promised in the signature `: Tipo`. | `public func f(): int {`<br>`    return "no es int"`<br>`}` | Return a value compatible with the declared signature. |
| `JOSS-TYPE-009` | Types | Non-existent data type or class (includes deleted aliases such as `integer`, `double`, `boolean`, `any`, `list`). | `integer $x = 10`<br>`boolean $flag = true` | Use Joss canonical types:<br>`int $x = 10`<br>`bool $flag = true` |
| `JOSS-TYPE-010` | Types | Function with annotated return type may terminate without executing a `return` or `throw`. | `public func f(int $n): string {`<br>`    $n > 0 ? { return "si" } : {}`<br>`}` | Ensure that all possible routes return a value of the promised type. |
| `JOSS-TYPE-011` | Types | A parameter without an explicit type was declared. | `public func f($x) {}` | In Joss all parameters must declare their type:<br>`public func f(int $x) {}` or `public func f(mixed $x) {}` |
| `JOSS-CALL-001` | Calls | Incorrect number of arguments regarding known signature parameters. | `public func f(int $a, int $b) {}`<br>`f(1)` | Provide all mandatory arguments required by the function. |
| `JOSS-MEMBER-001` | Members | An attempt is made to invoke a method that does not exist in the resolved receiving class. | `$usuario->metodoInexistente()` | Check the method name in the class definition or native catalog. |
| `JOSS-ACCESS-001` | Visibility | An attempt is made to use a class or function declared as `private` from another file. | Call a private function from another file. | Declare the function or class as `public` if it must be shared in the project. |
| `JOSS-ACCESS-002` | Visibility | Attempt to access a property or method `private` or `protected` outside its authorized scope. | `$cuenta->saldo` (being private) | Access through authorized public methods (getters/setters). |

---

## 3. Temporary references (`ref`)

| Code | Category | Meaning and Cause | Solution |
|---|---|---|---|
| `JOSS-REF-001` | References | Missing bilateral marker `ref` in the call or definition, or illegal crossover to native/async. | If the function expects `ref int $x`, the call must be `f(ref $x)`. |
| `JOSS-REF-002` | References | An expression, literal, object field, or array index is passed as a reference. | Only pass direct mutable local variables (`ref $miVariable`). |
| `JOSS-REF-003` | References | The variable passed as a reference is an immutable constant (`const`). | A reference mutates the original value; you cannot pass constants to a parameter `ref`. |
| `JOSS-REF-004` | References | Type mismatch in reference (the type of the variable does not exactly match the parameter). | References are strictly invariant: a `ref float` requires exactly one variable `float`. |
| `JOSS-REF-005` | References | Attempt to store, capture in closure, return or escape a reference. | A reference only lives during the call; to return data use `return`. |
| `JOSS-REF-006` | References | Parameter `ref` declared with a default value. | Parameters by reference do not allow default values; remove the `= valor`. |

---

## 4. Flow control, linter and templates

| Code | Severity | Meaning and Cause | Solution |
|---|---|---|---|
| `JOSS-FLOW-001` | Warning | Unreachable code (*dead code*): Instructions written after an unconditional `return`. | Move the instructions before `return` or delete them. |
| `JOSS-LINT-001` | Warning | Local variable declared but never read into the body. | Use the variable or remove it to keep the code clean. |
| `JOSS-SYNTAX-001` | Error | Syntax error captured during the linter parsing phase. | Correct the punctuation or structure indicated by the parser. |
| `JOSS-LINT-002` | Error | Parameter without explicit type reported by the linter. | Add the corresponding type annotation (`int`, `string`, `mixed`). |
| `JOSS-LINT-007` | Warning | Deviation from project naming conventions (classes in PascalCase, functions in camelCase). | Adjust the name to the canonical standard. |
| `JOSS-SEC-001` | Warning | Heuristic detection of a possible sensitive secret written in plain text (API keys, tokens). | Move the secret to the environment configuration file (`env.joss`). |
| `JOSS-VIEW-001` | Error | Critical failure when compiling or parsing an HTML view template. | Verify closing directives (`@foreach`, `@endforeach`) and included files. |
| `JOSS-VIEW-SYNTAX` | Error | Malformed Joss expression inside a view tag `{{ ... }}`. | Correct the syntax of the embedded expression. |
| `JOSS-VIEW-UNDEF` | Warning | View variable accessed without protection against undefined or null values. | Protect with the null coalescence operator: `{{ $variable ?? 'default' }}`. |

---

## 5. Arithmetic errors and runtime indexing

| Code | Category | Invalid operation in Runtime | How to prevent it |
|---|---|---|---|
| `JOSS-ARITH-001` | Runtime | 64-bit signed integer overflow (−2⁶³ to 2⁶³−1). | Validate the limits before operating or use the type `decimal` for large scale calculations. |
| `JOSS-ARITH-002` | Runtime | Division or module by zero (`$x / 0` or `$x % 0`). | Check that the divisor is non-zero before executing the division: `($divisor != 0) ? ($x / $divisor) : 0.0`. |
| `JOSS-INDEX-001` | Runtime | Index out of range: access to negative position or greater/equal to length. | Verify `count($arr)` before indexing or checking for existence with `array_key_exists`. |
| `JOSS-INDEX-002` | Runtime | Index type not supported by the data structure. | Index arrays with integers and maps with text strings. |

---

## 6. Complete Verified Test Cases

Below are executable examples that formally validate the issuance of the diagnoses and their correct counterpart:

### Non-existent type or deleted alias (`JOSS-TYPE-009`)

<!-- joss-error: JOSS-TYPE-009 -->
```joss-invalid
integer $edad = 20
```

Fixed case with canonical type `int`:

<!-- joss-run: ["20"] -->
```joss
int $edad = 20
print($edad)
```

---

### Parameter without explicit type (`JOSS-TYPE-011`)

<!-- joss-error: JOSS-TYPE-011 -->
```joss-invalid
public func duplicar($valor) { return $valor * 2 }
```

Corrected case by typing the parameter and the return:

<!-- joss-run: ["6"] -->
```joss
public func duplicar(int $valor): int { return $valor * 2 }
print(duplicar(3))
```

---

### Remap with incompatible type (`JOSS-TYPE-001`)

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
$edad = 20
$edad = "veinte"
```

Corrected case using voluntary dynamism (`mixed`):

<!-- joss-run: ["veinte"] -->
```joss
mixed $dato = 20
$dato = "veinte"
print($dato)
```

---

### Return missing in flow paths (`JOSS-TYPE-010`)

<!-- joss-error: JOSS-TYPE-010 -->
```joss-invalid
public func signo(int $n): string {
    $n > 0 ? { return "positivo" } : {}
}
```

Corrected case guaranteeing return on all possible routes:

<!-- joss-run: ["no positivo"] -->
```joss
public func signo(int $n): string {
    return $n > 0 ? "positivo" : "no positivo"
}
print(signo(0))
```

---

## Next step

Now that you know all the diagnostics and how to resolve them, you can dive deeper into how the semantic analyzer examines the abstract syntax tree to output these codes:

Continue with: [AST Static Analyzer](ANALIZADOR.md).

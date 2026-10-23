# Language diagnostic catalog and reference

Before: [Error and exception handling](ERRORES.md), [Type system](SISTEMA_TIPOS.md). After: [AST semantic analyzer](ANALIZADOR.md).
General index: [Joss Documentation](README.md).

---

## What is a diagnostic in Joss?

A **diagnostic** is a structured technical report emitted by Joss tools (the compiler, the semantic analyzer `joss analyze`, the linter, or the defensive runtime) when a violation of the language's syntactic, type, or safety rules is detected.

Unlike generic error messages in older tools, every Joss diagnostic is designed following these principles:
1. **Unique and stable identifier**: Every rule has an immutable code (for example `JOSS-TYPE-001`).
2. **Exact location**: File, line, and precise column where the conflict originated.
3. **Pedagogical explanation**: Clearly describes which rule was broken and why.
4. **Actionable suggestion**: Proposes the recommended canonical fix.

### Severity levels

- **`error`**: Prevents program execution (`joss run` and the compiler stop). Represents a structural or safety failure.
- **`warning`**: Informational notice (such as a declared variable that was never used or a misaligned naming convention). `joss analyze` exits with code `0` if there are only warnings, allowing execution to continue.

---

## 1. Parser, project loading, and symbol table

| Code | Category | Meaning and Cause | Incorrect example | Solution and Correct case |
|---|---|---|---|---|
| `JOSS-IO-001` | Input / Output | Cannot read the source file on disk (permissions or non-existent path). | `joss run fantasma.joss` | Verify that the file exists at the indicated path with read permissions. |
| `JOSS-PARSE-001` | Syntax | Invalid token or grammatical structure: missing quote, brace, or mandatory visibility. | `class MiClase {}`<br>`func prueba() {}` | Use canonical syntax:<br>`public class MiClase {}`<br>`public func prueba() {}` |
| `JOSS-SYM-001` | Symbols | Undeclared variable: attempting to read or assign to a variable that has not been previously declared. | `print($variable)`<br>`$nueva = 10` | Declare the variable before using it:<br>`var $variable = "valor"`<br>`var $nueva = 10` |
| `JOSS-SYM-002` | Symbols | Variable redeclaration in the same lexical scope. | `int $x = 1`<br>`int $x = 2` | Reassign without redeclaring:<br>`$x = 2` |
| `JOSS-SYM-003` | Symbols | Unresolved function: invoking a function that does not exist in the project or built-ins. | `calcularTotal()` | Define the function with `public func` or check the spelling of the name. |
| `JOSS-SYM-004` | Symbols | Unresolved class: attempting to instantiate (`new`) an undeclared and unregistered class. | `var $p = new Persona()` | Declare `public class Persona {}` or load the plugin that exposes it. |
| `JOSS-SYM-005` | Symbols | Invalid inheritance: the class specified in `extends` does not exist or creates circular inheritance. | `public class A extends B {}` | Make sure superclass `B` is declared in the project without cycles. |
| `JOSS-SYM-006` | Symbols | Attempt to reassign an immutable constant. | `const int $MAX = 5`<br>`$MAX = 10` | If the value must change, declare it as a mutable variable: `var $MAX = 5`. |
| `JOSS-SYM-007` | Symbols | Unresolved interface: the interface specified in `implements` or `extends` does not exist. | `public class A implements IDesconocida {}` | Make sure to declare the interface or check its spelling. |
| `JOSS-SYM-008` | Symbols | Cyclic inheritance in interfaces: an interface extends itself directly or indirectly. | `public interface A extends B {}`<br>`public interface B extends A {}` | Break the inheritance cycle between the interfaces. |
| `JOSS-DECL-001` | Declarations | Name conflict: two global functions have the exact same identifier. | Two files with:<br>`public func procesar() {}` | Rename one of the two functions. In Joss there are no folder-based source namespaces. |
| `JOSS-DECL-002` | Declarations | Class conflict: two global classes have the same name in the project. | Two files with:<br>`public class Usuario {}` | Keep a single canonical declaration of the class across the entire project. |
| `JOSS-DECL-003` | Declarations | Duplicate methods in the same class/interface, or unknown/duplicate interface in `implements`. | `public func id() {}`<br>`public func id(int $x) {}` | Joss does not support method overloading by signature; use distinct descriptive names. |
| `JOSS-DECL-004` | Declarations | Name conflict between nominal types (class/interface/enum) or breach of interface contract (`implements`). | `public class MiClase implements IFigura {}` (missing `calcularArea()`) | Use unique names for each type and implement all interface methods with compatible signatures. |
| `JOSS-DECL-005` | Declarations | Concrete class without implementing an inherited `abstract` method, or `abstract` method declared in a non-abstract class. | `public class Perro extends Animal {}` (missing `hablar()`) | Implement all abstract methods in the concrete subclass or mark the class as `abstract`. |
| `JOSS-DECL-006` | Declarations / Migration | Attempt to instantiate an `abstract` class with `new` (`error`), or use of the historical keyword `let` (`warning` in migration mode). | `new AnimalAbstracto()`<br>`let $valor = 1` | Instantiate a concrete subclass; for variables use `var`, an explicit type, or `mixed`. |
| `JOSS-DECL-007` | Migration | An assignment created a variable implicitly without prior declaration. | `$valor = 1` | Declare beforehand with `var`, an explicit type, `mixed`, or `const`. |
| `JOSS-DECL-008` | Migration | The historical form `Init constructor(...)` was used. | `Init constructor() {}` | Migrate to `public func constructor()` or `Init()`. |
| `JOSS-DECL-010` | Migration | `nil`, historical alias of `null`, was used. | `var $valor = nil` | Use `null`; `joss fix` applies the change without touching strings or comments. |
| `JOSS-SEM-001` | Semantics | Interface method implemented with non-public visibility. | `private func calcularArea(): int` | All methods implementing an interface must be `public`. |
| `JOSS-CONST-001` | Constants | Attempt to modify an immutable field of a `record` or a `const` property. | `$coordenada->x = 5` | Fields of a `record` and `const` properties are read-only after initialization. |

---

## 2. Type system, calls, and accessibility

| Code | Category | Meaning and Cause | Incorrect example | Solution and Correct case |
|---|---|---|---|---|
| `JOSS-TYPE-001` | Types | Incompatible reassignment: assigning data of a different type to a typed or inferred variable. | `var $edad = 20`<br>`$edad = "veinte"` | Keep the type homogeneous, or use voluntary dynamism:<br>`mixed $edad = 20`<br>`$edad = "veinte"` |
| `JOSS-TYPE-002` | Types | Initial value incompatible with the explicit type annotation. | `int $x = "texto"` | Provide a value of the expected type or a convertible string according to `CoerceString`. |
| `JOSS-TYPE-003` | Types | Argument incompatible with the typed parameter of a function or method. | `public func f(int $n) {}`<br>`f("hola")` | Pass the right type or convert the argument before the call. |
| `JOSS-TYPE-004` | Types | Operator applied to unsupported operands (e.g. adding text with `+`). | `"hola" + " mundo"` | Use numbers for arithmetic; to join text use the dot operator (`.`): `"hola" . " mundo"`. |
| `JOSS-TYPE-005` | Types | Unsupported key type in an associative map. | `{[1, 2]: "valor"}` | Keys of a `map` must be of type `string`. |
| `JOSS-TYPE-006` | Types | Incorrect index type for a known collection. | `$arr["clave"]` (on an array)<br>`$map[true]` (on a map) | Arrays are indexed with integers (`$arr[0]`); maps with strings (`$map["clave"]`). |
| `JOSS-TYPE-007` | Types | Attempt to index with `[...]` a value that is not indexable (e.g. an integer or boolean). | `var $n = 42`<br>`print($n[0])` | Index only arrays, maps, strings, or compatible instances. |
| `JOSS-TYPE-008` | Types | The value returned by `return` does not match the type promised in the `: Tipo` signature. | `public func f(): int {`<br>`    return "no es int"`<br>`}` | Return a value compatible with the declared signature. |
| `JOSS-TYPE-009` | Types | Non-existent data type or class (includes removed aliases such as `integer`, `double`, `boolean`, `any`, `list`). | `integer $x = 10`<br>`boolean $flag = true` | Use Joss canonical types:<br>`int $x = 10`<br>`bool $flag = true` |
| `JOSS-TYPE-010` | Types | Function with an annotated return type can finish without executing a `return` or `throw`. | `public func f(int $n): string {`<br>`    $n > 0 ? { return "si" } : {}`<br>`}` | Guarantee that all possible paths return a value of the promised type. |
| `JOSS-TYPE-011` | Types | A parameter was declared without an explicit type. | `public func f($x) {}` | In Joss all parameters must declare their type:<br>`public func f(int $x) {}` or `public func f(mixed $x) {}` |
| `JOSS-TYPE-012` | Warning | An unparameterized mutable collection is used as a typed collection. The original alias could insert incompatible values. | `array $origen = [1]`<br>`array<int> $numeros = $origen` | Keep the type from declaration or validate and copy at the boundary: `array<int> $origen = [1]`. |
| `JOSS-TYPE-014` | Migration | A named callable omitted its return contract. | `public func guardar() {}` | Declare `: Tipo` or `: void` when it does not return a value. |
| `JOSS-ARITH-003` | Arithmetic | An operation requires an implicit numeric conversion that may lose precision. | `9007199254740993 / 1`<br>`1 + 0.5`<br>`0.1 + 1.00m` | Use operands of the same type and convert explicitly; for money, keep the entire operation in `decimal`. |
| `JOSS-CALL-001` | Calls | Wrong number of arguments for the parameters of the known signature. | `public func f(int $a, int $b) {}`<br>`f(1)` | Provide all mandatory arguments required by the function. |
| `JOSS-MEMBER-001` | Members | Attempting to invoke a method that does not exist on the resolved receiver class. | `$usuario->metodoInexistente()` | Check the method name in the class definition or in the native catalog. |
| `JOSS-ACCESS-001` | Visibility | Attempting to use a class or function declared as `private` from another file. | Calling a private function from another file. | Declare the function or class as `public` if it must be shared across the project. |
| `JOSS-ACCESS-002` | Visibility | Attempting to access a `private` or `protected` property or method outside its authorized scope. | `$cuenta->saldo` (being private) | Access through authorized public methods (getters/setters). |

---

## 3. Temporary references (`ref`)

| Code | Category | Meaning and Cause | Solution |
|---|---|---|---|
| `JOSS-REF-001` | References | Missing bilateral `ref` marker at the call site or definition, or illegal crossing into native/async. | If the function expects `ref int $x`, the call must be `f(ref $x)`. |
| `JOSS-REF-002` | References | Passing an expression, literal, object field, or array index as a reference. | Pass only direct mutable local variables (`ref $miVariable`). |
| `JOSS-REF-003` | References | The variable passed by reference is an immutable constant (`const`). | A reference mutates the original value; you cannot pass constants to a `ref` parameter. |
| `JOSS-REF-004` | References | Type mismatch in reference (the variable type does not exactly match the parameter). | References are strictly invariant: a `ref float` requires an exact `float` variable. |
| `JOSS-REF-005` | References | Attempt to store, capture in a closure, return, or let a reference escape. | A reference only lives during the call; to return data use `return`. |
| `JOSS-REF-006` | References | `ref` parameter declared with a default value. | Reference parameters do not accept default values; remove `= valor`. |

---

## 4. Control flow, linter, and templates

| Code | Severity | Meaning and Cause | Solution |
|---|---|---|---|
| `JOSS-FLOW-001` | Warning | Unreachable code (*dead code*): statements written after an unconditional `return`. | Move the statements before the `return` or remove them. |
| `JOSS-FLOW-002` | Warning / Error | Non-exhaustive match on an enum (without covering all cases or `default`), or `break`/`continue` outside a loop. | Add the missing enum cases or include `default => ...`; use `break`/`continue` only inside loops. |
| `JOSS-FLOW-003` | Warning | Member access on a value whose type still includes `null`. | Check the value against `null` or use the null-safe operator `?->`. |
| `JOSS-FLOW-005` | Error | The `: { ... }` block of a `guard` clause does not terminate callable flow with `return` or `throw`. | Make sure the `guard` block always ends with `return` or `throw`. |
| `JOSS-LINT-001` | Warning | Local variable declared but never read in the body. | Use the variable or remove it to keep the code clean. |
| `JOSS-SYNTAX-001` | Error | Syntax error caught during the linter analysis phase. | Fix the punctuation or structure pointed out by the parser. |
| `JOSS-LINT-002` | Error | Untyped parameter reported by the linter. | Add the corresponding type annotation (`int`, `string`, `mixed`). |
| `JOSS-LINT-007` | Warning | Deviation from project naming conventions (classes in PascalCase, functions in camelCase). | Adjust the name to the canonical standard. |
| `JOSS-SEC-001` | Warning | Heuristic detection of a possible sensitive secret written in plain text (API keys, tokens). | Move the secret to the environment configuration file (`env.joss`). |
| `JOSS-VIEW-001` | Error | Critical failure when compiling or parsing an HTML view template. | Check closing directives (`@foreach`, `@endforeach`) and included files. |
| `JOSS-VIEW-SYNTAX` | Error | Malformed Joss expression inside a view tag `{{ ... }}`. | Fix the syntax of the embedded expression. |
| `JOSS-VIEW-UNDEF` | Warning | View variable accessed without protection against undefined or null values. | Protect with the null-coalescing operator: `{{ $variable ?? 'default' }}`. |

---

## 5. Runtime arithmetic and indexing errors

| Code | Category | Invalid operation at Runtime | How to prevent it |
|---|---|---|---|
| `JOSS-ARITH-001` | Runtime | 64-bit signed integer overflow (−2⁶³ to 2⁶³−1). | Validate bounds before operating or use the `decimal` type for large-scale calculations. |
| `JOSS-ARITH-002` | Runtime | Division or modulo by zero (`$x / 0` or `$x % 0`). | Check that the divisor is non-zero before dividing: `($divisor != 0) ? ($x / $divisor) : 0.0`. |
| `JOSS-INDEX-001` | Runtime | Index out of bounds: accessing a negative position or one greater than/equal to the length. | Check `count($arr)` before indexing or verify existence with `array_key_exists`. |
| `JOSS-INDEX-002` | Runtime | Index type incompatible with the data structure. | Index arrays with integers and maps with text strings. |

### Storage (`joss doctor storage`)

Configuration findings reported by `joss doctor storage`. They are not language syntax/type errors, but share the same diagnostic model.

| Code | What it means | How to fix it |
|---|---|---|
| `JOSS-STORAGE-000` | Check passed (severity `info`). | Nothing to do. |
| `JOSS-STORAGE-001` | `STORAGE` has an unknown value. | Use `STORAGE=local` or `STORAGE=OCI`. |
| `JOSS-STORAGE-002` | Private key is missing, not PEM, not RSA, or encrypted without `OCI_PASSPHRASE`. | Fix `OCI_PRIVATE_KEY_PATH` or generate a new API key. |
| `JOSS-STORAGE-003` | Local PEM key fingerprint does not match `OCI_FINGERPRINT`. | Use the PEM corresponding to the registered fingerprint in OCI, or update `OCI_FINGERPRINT`. |
| `JOSS-STORAGE-004` | Missing `OCI_*` variable or invalid format. | Copy exact values from OCI console. |
| `JOSS-STORAGE-005` | OCI rejected authentication (401) or was unreachable. | Register public key in user profile; if recently added, wait a couple minutes. |
| `JOSS-STORAGE-006` | `OCI_NAMESPACE` does not belong to tenancy; suggestion includes real one. | Use suggested namespace. |
| `JOSS-STORAGE-007` | Bucket does not exist in namespace or lacks IAM permissions. | Create bucket, fix `OCI_BUCKET_NAME`, or review IAM policies. |
| `JOSS-STORAGE-008` | `./storage` is not writable when `STORAGE=local`. | Review permissions and disk space. |
| `JOSS-STORAGE-009` | Provider (`s3`, `aws`, `azure`) has no runtime driver yet. | Use `local` or `OCI`. |

Invalid example: `OCI_NAMESPACE=axghxqmjl3jf` with credentials of another tenancy produces `JOSS-STORAGE-006`. Valid neighbor: same file with the namespace indicated in suggestion.

---

## 6. Verified complete test cases

Channel operations also have stable runtime defenses. `JOSS-CHANNEL-001`
occurs when closing a channel twice or sending after closing; the valid
neighbor is sending while open and closing it only once. `JOSS-CHANNEL-002`
occurs with `make_chan(-1)`; `make_chan(0)` is a valid capacity for an
unbuffered channel. These states may depend on another concurrent task, so
the analyzer cannot replace the runtime defense.

Below are executable examples that formally validate the emission of diagnostics and their correct counterpart:

### Non-existent type or removed alias (`JOSS-TYPE-009`)

<!-- joss-error: JOSS-TYPE-009 -->
```joss-invalid
integer $edad = 20
```

Corrected case using the canonical `int` type:

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

Corrected case typing both the parameter and return value:

<!-- joss-run: ["6"] -->
```joss
public func duplicar(int $valor): int { return $valor * 2 }
print(duplicar(3))
```

---

### Reassignment with incompatible type (`JOSS-TYPE-001`)

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
var $edad = 20
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

### Unsafe narrowing of mutable collection (`JOSS-TYPE-012`)

Legacy compatibility allows the operation, but the analyzer warns because both names may share the same mutable collection.

<!-- joss-error: JOSS-TYPE-012 -->
```joss-invalid
array $origen = [1]
array<int> $numeros = $origen
```

Safe case keeping the type from the origin:

<!-- joss-run: ["1"] -->
```joss
array<int> $origen = [1]
array<int> $numeros = $origen
print($numeros[0])
```

---

### Missing return across control-flow paths (`JOSS-TYPE-010`)

<!-- joss-error: JOSS-TYPE-010 -->
```joss-invalid
public func signo(int $n): string {
    $n > 0 ? { return "positivo" } : {}
}
```

Corrected case guaranteeing a return across all possible paths:

<!-- joss-run: ["no positivo"] -->
```joss
public func signo(int $n): string {
    return $n > 0 ? "positivo" : "no positivo"
}
print(signo(0))
```

---

### Lossy numeric conversion (`JOSS-ARITH-003`)

<!-- joss-error: JOSS-ARITH-003 -->
```joss-invalid
var $resultado = 9007199254740993 / 1
```

Corrected case preserving decimal precision:

<!-- joss-run: ["9007199254740993"] -->
```joss
decimal $resultado = 9007199254740993m / 1m
print($resultado)
```

---

### Non-exhaustive enum match (`JOSS-FLOW-002`)

<!-- joss-error: JOSS-FLOW-002 -->
```joss-invalid
public enum Estado {
    case Activo;
    case Inactivo;
}

public func describir(Estado $e): string {
    return match ($e) {
        Estado::Activo => "activo",
    }
}
```

Corrected case exhaustively covering all enum cases:

<!-- joss-run: ["activo"] -->
```joss
public enum Estado {
    case Activo;
    case Inactivo;
}

public func describir(Estado $e): string {
    return match ($e) {
        Estado::Activo => "activo",
        Estado::Inactivo => "inactivo",
    }
}
print(describir(Estado::Activo))
```

---

### Potentially null member access (`JOSS-FLOW-003`)

<!-- joss-error: JOSS-FLOW-003 -->
```joss-invalid
public class Usuario { public string $nombre = "Ada" }
public func nombre(Usuario|null $usuario): string {
    return $usuario->nombre
}
```

Corrected case using safe access:

<!-- joss-run: ["sin usuario"] -->
```joss
public class Usuario { public string $nombre = "Ada" }
public func nombre(Usuario|null $usuario): string {
	return $usuario == null ? "sin usuario" : $usuario->nombre
}
print(nombre(null))
```

---

## Next step

Now that you know all the diagnostics and how to resolve them, you can dive deeper into how the semantic analyzer inspects the abstract syntax tree to emit these codes:

Continue with: [AST static analyzer](ANALIZADOR.md).

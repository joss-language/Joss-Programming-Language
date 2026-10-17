# Type system, inference and conversions

Before: [Complete web project](PROYECTO_WEB.md), [Collections](COLECCIONES.md). After: [Syntax reference](SINTAXIS.md), [Diagnostics](DIAGNOSTICOS.md).
Technical reference: [Diagnostics](DIAGNOSTICOS.md), [Syntax](SINTAXIS.md).

---

## What are you going to learn here?

A **type system** is the set of rules that governs how a computer interprets ones and zeros in memory. Without types, a 64-bit sequence in memory could be a number, a letter, an image, or a processor instruction; there would be no way to know.

In Joss, the type system serves a dual purpose:
1. **Safety and robustness**: Detects inconsistencies (such as trying to multiply text by a list) before the code runs in production.
2. **Documentation clarity**: Helps any developer immediately understand what data a function expects and what result it will produce.

In this guide you will learn:
1. The canonical list of data types in Joss and why certain old names (*aliases*) are no longer valid.
2. How **type inference** works and the differences between `var`, explicit types, `mixed`, and `const`.
3. The operation of **union types** (`T|U`), optional nullable values (`T?`), and the `: void` return type.
4. How compatibility and exact numeric promotion (`int → decimal`) work and why mixing with `float` requires explicit conversion (`JOSS-ARITH-003`).
5. Typed collections with generics (`array<T>`, `map<K, V>`, `channel<T>`, `Result<T, E>`).
6. The string coercion rule (`typesystem.CoerceString`) and arithmetic safety protections.

---

## 1. Inventory of canonical source types

The compiler's canonical source of truth (`pkg/typesystem/types.go`) recognizes the following valid types:

| Source type | Meaning | Physical representation in the Runtime | Usage example |
|---|---|---|---|
| `int` | 64-bit signed integer | `int64` (−9,223,372,036,854,775,808 to 9,223,372,036,854,775,807) | `int $id = 101` |
| `float` | Standard IEEE-754 binary floating point | 64-bit `float64` | `float $ratio = 0.75` |
| `decimal` | Base-ten fixed-point decimal number | `shopspring/decimal.Decimal` (high precision) | `decimal $precio = 99.99m` |
| `string` | UTF-8 character sequence | Go `string` with grapheme support | `string $email = "ada@joss.red"` |
| `bool` | Logical truth value | `bool` (`true` or `false`) | `bool $valido = true` |
| `array` | Dynamic sequence of elements | `[]interface{}` (Go slice) | `array $items = [1, 2, 3]` |
| `map` | Associative table with text keys | `map[string]interface{}` (Go hashmap) | `map $datos = {"rol": "admin"}` |
| `object` | Generic instance of a class | User or native class instance | `object $instancia = new Persona()` |
| `channel` | Concurrent communication channel | `*core.Channel` (Go message channel) | `channel $c = make_chan(1)` |
| `mixed` | Explicit dynamism and polymorphism | Any valid runtime value | `mixed $dato = "dinamico"` |
| `void` | Value-free termination (callable return only) | No return value (`nil`) | `public func log(): void` |
| `null` | Absence of value | `nil` representation | `null` |
| Class / interface / enum name | User-defined or native nominal type | Corresponding instance or case | `Persona $p = new Persona()` |

### Old aliases removed

> [!WARNING]
> In older versions of the language there were alternative names inherited from other ecosystems such as `integer`, `double`, `boolean`, `dynamic`, `any`, or `list`. **These names were completely removed from the grammar**.
>
> If you write `integer $x = 10`, the analyzer will not convert it to `int`; it will look for a user class called `integer`. Upon not finding it, it will emit the diagnostic error `JOSS-TYPE-009` (Unresolved type). Always use the canonical names: `int`, `float`, `bool`, `mixed`, and `array`.

---

## 2. Inference and ways to declare variables

Joss combines the agility of modern inference with the safety of strongly typed languages:

<!-- joss-run: ["30", "Ada", "pendiente"] -->
```joss
var $edad = 20
$edad = 30
string $nombre = "Ada"
mixed $resultado = 10
$resultado = "pendiente"
print($edad)
print($nombre)
print($resultado)
```

### Semantic declaration rules:

1. **Canonical inference with `var` (`var $x = 20`)**:
   - Upon declaration, Joss inspects the initial value and fixes the concrete type (in this case `int`).
   - Subsequent reassignments (`$x = 30`) **must be compatible** with that type. If you try to assign text, the analyzer will reject the program with `JOSS-TYPE-001`.
2. **Explicit declaration (`string $nombre = "Ada"`)**:
   - Fixes the type in a visible and documented way.
3. **Voluntary dynamism (`mixed $resultado = 10`)**:
   - Tells the analyzer that this variable will change its nature over time. You can reassign text, a map, or a class to it without errors.
   - `let $resultado = 10` is historical transitional syntax (`JOSS-DECL-006`); always prefer `mixed $resultado = 10`.
4. **Prohibition of undeclared variables (direct `$x = 20`)**:
   - Assigning for the first time to a variable without having previously declared it with `var`, an explicit type, `mixed`, or `const` emits `JOSS-SYM-001` (and the transition warning `JOSS-DECL-007`).
5. **Initialization with `null`**:
   - If you write `var $x = null`, inference is **postponed** until the first assignment containing a concrete value.

---

## 3. Union Types (`T|U`) and nullable types (`T?`)

In real-world development it is very common for an operation to return either a concrete piece of data or `null` if nothing was found (for example, looking up a user in the database).

For these cases, Joss offers **union types**:

<!-- joss-run: ["10", "A-10", "sin dato"] -->
```joss
int|string $id = 10
print($id)
$id = "A-10"
print($id)
int? $cantidad = null
print($cantidad ?? "sin dato")
```

### Union type rules:

1. **Pipe syntax (`|`)**: `int|string` means the variable will only accept integers or strings, while rejecting booleans or lists.
2. **Question mark shortcut (`?`)**: Writing `int?` is exact shorthand for `int|null`. The compiler's AST automatically normalizes it to a union with `null`.
3. **Type refinement (Smart Cast Narrowing)**:
   If you have a variable `string? $nombre` and filter it with `guard ($nombre != null) : { return "" }` or with a ternary `($nombre != null) ? { ... }`, the analyzer automatically narrows its type to non-null `string` along the validated path.

---

## 4. Compatibility and numeric promotion rules

When can a value of a source type be assigned to a variable of a target type?

```text
int ──────────► decimal
    promoción exacta (única implícita)
```

1. **Same type**: Always allowed.
2. **`int → decimal`**: This is the **only implicit numeric promotion** between distinct types because it preserves the exact base-ten value without precision loss.
3. **`int → float` and `float → decimal`**: **Require explicit conversion** (`floatval($entero)` or `decimal($flotante)`). The former can lose precision on large integers ($> 2^{53}$) and the latter only preserves the prior binary approximation. Attempting to mix them implicitly emits `JOSS-ARITH-003` or `JOSS-TYPE-001`/`002`.
4. **`Clase → object`**: Any class instance is compatible with the universal type `object`.
5. **`Subclase → Superclase`**: A derived class that extends a base class is accepted wherever the base class is expected.
6. **`Clase → Interfaz`**: A class that implements an interface (`implements`) is compatible wherever that interface is declared as a type.
7. **`mixed`**: Universally compatible in both directions.

Operations that mix `int` with `float`, or `float` with `decimal`, also
require deliberately converting one of the operands. Dividing two
integers produces `float`; if an operand cannot be represented exactly in IEEE-754, the
analyzer emits `JOSS-ARITH-003` when known statically and the runtime maintains the same
defense. For monetary amounts, percentages, and taxes, use `decimal` literals (`19.99m`) and avoid
passing through `float` first.

---

## 5. Typed collections (First-class generics)

Joss allows parameterizing collections and containers:

<!-- joss-run: ["6", "2"] -->
```joss
array<int> $cantidades = [2, 4, 6]
map<string, int> $inventario = {"pan": 2}
print($cantidades[2])
print($inventario["pan"])
```

- `array<T>`: An array where all elements must be of type `T`.
- `map<K, V>`: A map with keys of type `K` (must be `string`) and values of type `V`.
- `channel<T>`: A typed concurrency channel where transmitted messages must be of type `T` (e.g. `channel<string>`).
- `Result<T, E>`: Canonical type for operation results that can succeed with a value of type `T` or fail with an error of type `E`.
- `Clase<T>`: Classes with formal generic type parameters (e.g. `Caja<T>`).

`array<T>`, `map<K, V>`, and `channel<T>` are mutable and therefore their type parameters are **invariant**. For example, `array<int>` cannot be reused as `array<decimal>`, even though an individual `int` can be promoted to `decimal`: allowing the alias would let code later insert a fractional `decimal` visible through a reference that promises to contain only `int`. Unparameterized collections retain transitional compatibility, but crossing into a typed collection emits `JOSS-TYPE-012` because another alias could still mutate them without respecting the element type.

When indexing a parameterized collection (for example `$cantidades[0]`), the analyzer immediately infers that the result is of type `int`, guaranteeing type safety across the rest of the code.

---

## 6. Enumerations (`enum`)

**Enumerations** let you define a closed type with a finite set of possible cases, avoiding scattered constants or magic strings.

In Joss there are two kinds of enumerations:

### Pure enums (Unit Enums)
Each case represents a unique symbolic value with the `->name` property:

<!-- joss-run: ["Pendiente", "Aprobado"] -->
```joss
public enum Estado {
    case Pendiente
    case Aprobado
    case Rechazado
}

var $e = Estado::Pendiente
print($e->name)
var $e2 = Estado::Aprobado
print($e2->name)
```

### Backed enums (Backed Enums)
Associate each case with a primitive scalar value (`string` or `int`):

<!-- joss-run: ["admin", "admin", "Admin", "3"] -->
```joss
public enum Rol: string {
    case Admin = "admin"
    case Editor = "editor"
    case Lector = "lector"
}

var $r = Rol::Admin
print($r->value)

// Instanciar desde valor escalar con from() o tryFrom()
var $desdeValor = Rol::from("admin")
print($desdeValor->value)
print($desdeValor->name)

// Obtener todos los casos con cases()
var $todos = Rol::cases()
print(count($todos))
```

---

## 7. Type-checking operators: `is` and `instanceof`

The **`is`** operator (and its alias **`instanceof`**) lets you check at runtime whether a value belongs to a primitive type (`int`, `string`, `bool`, etc.), a class, or an interface:

<!-- joss-run: ["true", "true", "true"] -->
```joss
var $numero = 42
var $texto = "hola"
print($numero is int)
print($texto is string)
print($numero instanceof int)
```

---

## 8. Typed textual coercion (`typesystem.CoerceString`)

In web applications, data arriving from HTTP forms or JSON requests consists of raw text strings (for example, `"8080"` or `"true"`).

Joss implements a shared textual coercion policy (`CoerceString`) in both the static analyzer and the runtime when you assign a literal or string to an explicitly typed variable:

<!-- joss-run: ["9000", "49.99", "true"] -->
```joss
int $puerto = "9000"
decimal $precio = "49.99"
bool $activo = "yes"
print($puerto)
print($precio)
print($activo)
```

### Table of accepted textual inputs:

| Target type | Text strings that Joss converts automatically |
|---|---|
| `int` | Strings with integer digits (`"9000"`, `"-42"`), or floating numbers without a fractional part (`"100.0"`). |
| `float` | Any string with valid decimal notation (`"3.1416"`, `"-0.05"`). |
| `decimal` | Clean numeric strings (`"49.99"`, `"120.50m"`). |
| `bool` | Case-insensitively accepts: `"true"`, `"1"`, `"yes"` (as `true`); and `"false"`, `"0"`, `"no"`, `""` (as `false`). |

If the string cannot be converted (for example `int $x = "manzana"`), the analyzer emits a static error `JOSS-TYPE-002` or the runtime defensively rejects it.

---

## 9. Numeric precision and runtime defenses

| Safety rule | Behavior in Joss | Diagnostic |
|---|---|---|
| Integer overflow | Operations on 64-bit integers (`+`, `-`, `*`) are checked against overflow. If they exceed 64-bit limits, execution halts immediately. | `JOSS-ARITH-001` |
| Division by zero | Dividing or computing the remainder of a number by zero (`$n / 0` or `$n % 0`) produces a controlled error instead of unexpected `NaN` or `Infinity` values. | `JOSS-ARITH-002` |
| Implicit precision loss | Mixing `int` and `float`, `float` and `decimal`, or dividing integers outside the exact IEEE-754 range without explicit conversion is rejected. | `JOSS-ARITH-003` |
| Index out of bounds | Accessing an index that is negative or greater than the length of a list or string stops the program in a structured manner. | `JOSS-INDEX-001` |

---

## 10. Common type system diagnostics

| Code | Meaning | Usual solution |
|---|---|---|
| `JOSS-TYPE-001` | Reassignment with an incompatible type (e.g. `var $x = 1; $x = "hola"`). | Keep the type homogeneous or explicitly declare the variable as `mixed $x`. |
| `JOSS-TYPE-002` | Initial value incompatible with the type annotation. | Fix the initial value to match the declared type. |
| `JOSS-TYPE-008` | The value returned by a function does not match the type promised in `: Tipo`. | Check the `return` expression so that it returns the promised type. |
| `JOSS-TYPE-009` | Non-existent type or class name (includes retired aliases such as `integer`). | Replace `integer`, `double`, `boolean`, `any` with their canonical names: `int`, `float`, `bool`, `mixed`. |
| `JOSS-TYPE-010` | A typed function can finish without executing a `return` or `throw`. | Make sure all ternary branches conclude with a returned value. |
| `JOSS-TYPE-011` | An untyped parameter (`func($x)`) was declared. | Write the parameter type: `func(int $x)` or `func(mixed $x)`. |
| `JOSS-TYPE-014` | A named function or method omitted its return type. | Add `: Tipo` or `: void` to the function signature. |

---

## Next step

To look up all analyzer diagnostic codes and their correction examples, continue with: [Diagnostic catalog and reference](DIAGNOSTICOS.md).

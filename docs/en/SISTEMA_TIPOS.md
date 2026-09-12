# Type system, inference and conversions

Before: [Collections: Arrays, Maps and Text](COLECCIONES.md). After: [Classes and objects](CLASES.md).
Technical reference: [Diagnostics](DIAGNOSTICOS.md), [Syntax](SINTAXIS.md).

---

## What are you going to learn here?

A **type system** is the set of rules that governs how a computer interprets ones and zeros in memory. Without types, a 64-bit sequence in memory could be a number, a letter, an image, or a processor instruction; there would be no way to know.

In Joss, the type system serves a dual purpose:
1. **Security and robustness**: Detects inconsistencies (such as trying to multiply text by a list) before the code is run in production.
2. **Document clarity**: Helps any developer immediately understand what data a function expects and what result it will produce.

In this guide you will learn:
1. The canonical list of data types in Joss and why certain old names (*aliases*) are no longer valid.
2. How **type inference** works and the differences between `$x = ...`, `var`, `mixed` and explicit declarations.
3. The operation of **union types** (`T|U`) and optional nullable values ​​(`T?`).
4. How number compatibility and expansion works (`int → float → decimal`).
5. Collections typed with generics (`array<T>` and `map<K, V>`).
6. The string coerce rule (`typesystem.CoerceString`) and arithmetic security protections.

---

## 1. Inventory of canonical source types

The compiler's canonical source of truth (`pkg/typesystem/types.go`) recognizes the following valid types:

| Font type | Meaning | Physical representation in Runtime | Usage example |
|---|---|---|---|
| `int` | 64-bit signed integer | `int64` (−9,223,372,036,854,775,808 to 9,223,372,036,854,775,807) | `int $id = 101` |
| `float` | IEEE-754 standard binary floating point | 64-bit `float64` | `float $ratio = 0.75` |
| `decimal` | Fixed point decimal number in base ten | `shopspring/decimal.Decimal` (high precision) | `decimal $precio = 99.99m` |
| `string` | UTF-8 character sequence | Go `string` with grapheme support | `string $email = "ada@joss.red"` |
| `bool` | Logical truth value | `bool` (`true` or `false`) | `bool $valido = true` |
| `array` | Dynamic sequence of elements | `[]interface{}` (Go slice) | `array $items = [1, 2, 3]` |
| `map` | Associative table with text keys | `map[string]interface{}` (Go hashmap) | `map $datos = {"rol": "admin"}` |
| `object` | Generic instance of a class | Native or user class instance | `object $instancia = new Persona()` |
| `channel` | Concurrent communication channel | `*core.Channel` (Go message channel) | `channel $c = make_chan(1)` |
| `mixed` | Explicit dynamism and polymorphism | Any valid runtime value | `mixed $dato = "dinamico"` |
| `null` / `nil` | Absence of value | `nil` representation | `null` || Class name | User-defined or native nominal type | Corresponding class instance | `Persona $p = new Persona()` |

### Old aliases removed

> [!WARNING]
> In old versions of the language there were alternative names inherited from other ecosystems such as `integer`, `double`, `boolean`, `dynamic`, `any` or `list`. **These names were completely removed from the grammar**.
>
> If you write `integer $x = 10`, the parser will not convert it to `int`; will look for a user class called `integer`. If it is not found, it will issue the diagnostic error `JOSS-TYPE-009` (Unresolved Type). Always use the canonical names: `int`, `float`, `bool`, `mixed` and `array`.

---

## 2. Inference and ways to declare variables

Joss combines the agility of dynamic languages with the security of strongly typed languages:<!-- joss-run: ["30", "Ada", "pendiente"] -->
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

1. **Inference by simple assignment (`$x = 20`) or with `var` (`var $x = 20`)**:
   - On the first assignment, Joss inspects the value and sets the concrete type (in this case `int`).
   - Subsequent assignments **must be compatible** with that type. If you try to enter text, the parser will reject the program with `JOSS-TYPE-001`.
2. **Explicit declaration (`string $nombre = "Ada"`)**:
   - Set the type in a visible and documented way.
3. **Voluntary dynamism (`mixed $resultado = 10`)**:
   - Indicates to the analyzer that this variable will change in nature over time. You can reassign a text, a map or a class without errors.
   - `let $resultado = 10` is a syntactic shortcut that produces exactly one `mixed` variable.
4. **Initialization with `null`**:
   - If you write `$x = null` without a type, inference is **postponed** until the first assignment that contains a specific value.

---

## 3. Union Types (`T|U`) and Nullable Types (`T?`)

In real development it is very common for an operation to return a specific piece of data or return `null` if nothing was found (for example, searching for a user in the database).

For these cases, Joss offers **union types**:<!-- joss-run: ["10", "A-10", "sin dato"] -->
```joss
int|string $id = 10
print($id)
$id = "A-10"
print($id)
int? $cantidad = null
print($cantidad ?? "sin dato")
```
### Union type rules:

1. **Syntax with pipe (`|`)**: `int|string` means that the variable will only accept integers or texts, but will reject booleans or lists.
2. **The question mark shortcut (`?`)**: Typing `int?` is exactly equivalent to typing `int|null`. The compiler's AST automatically normalizes it to a union with `null`.
3. **Type refinement (Narrowing) in ternaries**:
   If you have a variable `string? $nombre` and questions `($nombre != null)`, inside the true branch the parser knows that `$nombre` can no longer be null, allowing you to access its text operations safely.

---

## 4. Compatibility and assignment rules

When can a value of type source be assigned to a variable of type destination?```text
       int ──────────► float ──────────► decimal
(Exacto 64 bits)    (Binario IEEE)     (Base 10 exacta)
```
1. **Same type**: Always allowed.
2. **`int → float`**: Automatically allowed. An integer can be promoted to float.
3. **`int → decimal` or `float → decimal`**: Automatically allowed. Joss converts the value to the exact decimal representation.
4. **`Clase → object`**: Any class instance is compatible with the universal type `object`.
5. **`Subclase → Superclase`**: A derived class that extends a base class is accepted wherever the base class is expected.
6. **`Clase → Interfaz`**: A class that implements an interface (`implements`) is compatible where said interface is declared as a type.
7. **`mixed`**: It is universally compatible in both directions.

Any other mixing (such as trying to put a `string` in an `int` or a `bool` in an `array`) will be blocked by the parser with `JOSS-TYPE-001` (Type Mismatch).

---

## 5. Typed Collections (First Level Generics)

Although Joss does not have complex generic templates in user functions, it does allow parameterization of the two main data structures:<!-- joss-run: ["6", "2"] -->
```joss
array<int> $cantidades = [2, 4, 6]
map<string, int> $inventario = {"pan": 2}
print($cantidades[2])
print($inventario["pan"])
```
- `array<T>`: An array where all elements must be of type `T`.
- `map<K, V>`: A map with keys of type `K` (must be `string`) and values ​​of type `V`.

When indexing a parameterized collection (for example `$cantidades[0]`), the parser immediately infers that the result is of type `int`, ensuring safety in the rest of the code.

---

## 6. Enums (`enum`)

**enumerations** allow defining a closed type with a finite set of possible cases, avoiding the use of sparse constants or magic strings.

In Joss there are two types of enumerations:

### Pure Enums (Unit Enums)
Each case represents a unique symbolic value with the `->name` property:<!-- joss-run: ["Pendiente", "Aprobado"] -->
```joss
public enum Estado {
    case Pendiente
    case Aprobado
    case Rechazado
}

$e = Estado::Pendiente
print($e->name)
$e2 = Estado::Aprobado
print($e2->name)
```
### Backed Enums
They associate each case with a primitive scalar value (`string` or `int`):<!-- joss-run: ["admin", "admin", "Admin", "3"] -->
```joss
public enum Rol: string {
    case Admin = "admin"
    case Editor = "editor"
    case Lector = "lector"
}

$r = Rol::Admin
print($r->value)

// Instanciar desde valor escalar con from() o tryFrom()
$desdeValor = Rol::from("admin")
print($desdeValor->value)
print($desdeValor->name)

// Obtener todos los casos con cases()
$todos = Rol::cases()
print(count($todos))
```
---

## 7. Type checking operators: `is` and `instanceof`

The **`is`** operator (and its alias **`instanceof`**) allows you to query at run time whether a value belongs to a primitive type (`int`, `string`, `bool`, etc.), a class or an interface:<!-- joss-run: ["true", "true", "true"] -->
```joss
$numero = 42
$texto = "hola"
print($numero is int)
print($texto is string)
print($numero instanceof int)
```
---

## 8. Typed Textual Coercion (`typesystem.CoerceString`)

In web applications, data arriving from HTTP forms or JSON requests are raw text strings (for example, `"8080"` or `"true"`).

Joss implements a shared textual coercion policy (`CoerceString`) in both the static analyzer and runtime:<!-- joss-run: ["9000", "49.99", "true"] -->
```joss
int $puerto = "9000"
decimal $precio = "49.99"
bool $activo = "yes"
print($puerto)
print($precio)
print($activo)
```
### Table of accepted textual entries:

| Destination type | Text strings that Joss automatically converts |
|---|---|
| `int` | Texts with integer digits (`"9000"`, `"-42"`), or floating numbers without a fractional part (`"100.0"`). |
| `float` | Any text with valid decimal notation (`"3.1416"`, `"-0.05"`). |
| `decimal` | Clean numeric texts (`"49.99"`, `"120.50m"`). |
| `bool` | Accepts case-insensitively: `"true"`, `"1"`, `"yes"` (like `true`); and `"false"`, `"0"`, `"no"`, `""` (like `false`). |

If the text cannot be converted (for example `int $x = "manzana"`), the parser issues a static error `JOSS-TYPE-002` or the runtime rejects it defensively.

---

## 9. Numerical precision and runtime defenses

| Safety rule | Behavior in Joss | Diagnosis |
|---|---|---|
| Integer overflow | Operations on 64-bit integers (`+`, `-`, `*`) are checked against overflow. If they exceed the 64-bit limits, execution stops immediately. | `JOSS-ARITH-001` |
| Division by zero | Dividing or calculating the remainder of a number by zero (`$n / 0` or `$n % 0`) produces a controlled error instead of unexpected `NaN` or `Infinity` values. | `JOSS-ARITH-002` |
| Index out of range | Accessing an index that is negative or greater than the length of a list or string stops the program in a structured manner. | `JOSS-INDEX-001` |

---

## 10. Common type system diagnostics

| Code | Meaning | Usual solution |
|---|---|---|
| `JOSS-TYPE-001` | Remapping with incompatible type (e.g. `$x = 1; $x = "hola"`). | Keep the type homogeneous or declare the variable explicitly as `mixed $x`. |
| `JOSS-TYPE-002` | Initial value incompatible with type annotation. | Correct the initial value to match the declared type. |
| `JOSS-TYPE-008` | The value returned by a function does not match the type promised in `: Tipo`. | Check the `return` expression so that it returns the promised type. |
| `JOSS-TYPE-009` | Non-existent type or class name (includes retired aliases such as `integer`). | Replace `integer`, `double`, `boolean`, `any` with their canonical names: `int`, `float`, `bool`, `mixed`. |
| `JOSS-TYPE-010` | A typed function can terminate without executing a `return` or `throw`. | Make sure all ternary branches conclude with a return value. |
| `JOSS-TYPE-011` | An untyped parameter (`func($x)`) was declared. | Enter the type of the parameter: `func(int $x)` or `func(mixed $x)`. |

---

## Next step

Now that you understand the type system, unions, and safe conversions, we'll take the next step into domain modeling and object-oriented programming:

Continue with: [Classes, objects, methods and inheritance](CLASES.md).
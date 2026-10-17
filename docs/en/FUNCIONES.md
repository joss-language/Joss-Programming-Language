# Functions, Variable Scope, Closures, and References

Previous: [Collections: Arrays, Maps, and Text](COLECCIONES.md). Next: [Error and exception handling](ERRORES.md).
Technical reference: [Syntax](SINTAXIS.md), [Recursion](RECURSION.md), [Type system](SISTEMA_TIPOS.md).

---

## What will you learn here?

As programs grow, writing hundreds of instructions in a row becomes unmanageable. If you need to calculate an invoice total with taxes in ten different places in your application, copying and pasting the same formula ten times is a recipe for disaster: if the tax law changes, you would have to find and fix ten files, and sooner or later you will forget one.

In this guide you will learn:
1. What a **function** is and why it is the fundamental building block of software.
2. The critical difference between **parameters**, **arguments**, and **printing (`print`)** versus **returning (`return`)**, including `: void` functions.
3. How to declare functions with safe types, default values, named arguments, and unpacking (`...`).
4. The mandatory visibility rules in Joss (`public` and `private`).
5. How to write clean early returns with **`guard`** and schedule guaranteed cleanup actions with **`defer`**.
6. What **variable scope** is and how memory isolation works on each call.
7. What **anonymous functions (closures)** are and how to use them to transform lists with `map` and `filter`.
8. How to safely modify external variables using **references (`ref`)**.
9. How to chain clean data transformations with the **pipeline (`|>`)** operator.

---

## 1. What is a function?

A **function** is an autonomous block of instructions to which we assign a name. Think of it as a small specialized machine:
1. It receives raw materials (input data, called **arguments**).
2. It processes the information inside in isolation.
3. It returns a finished product (the result, called the **return value**).

<!-- joss-run: ["5", "12"] -->
```joss
public func sumar(int $a, int $b): int {
    return $a + $b
}
print(sumar(2, 3))
print(sumar(5, 7))
```

Terminal output:

```text
5
12
```

### Anatomy of a function in Joss:

- `public`: **Visibility modifier**. In Joss, global functions strictly require visibility:
  - `public`: The function is available throughout the entire project and other files can use it without imports.
  - `private`: The function can only be called from the same file where it was written.
- `func`: Keyword indicating the start of the declaration (Joss does not use `function`).
- `sumar`: The name of the function.
- `(int $a, int $b)`: The list of **parameters**. It defines what data types the function requires to work.
- `: int`: The **return type**. It declares what type of value the function promises to return to its caller.
- `{ ... }`: The **body** of the function, where the instructions are written.
- `return $a + $b`: The `return` statement ends the execution of the function immediately and sends the result back to the caller.

---

## 2. Crucial Conceptual Differences

### Parameters vs Arguments
- **Parameter**: The variable you declare in the function signature (for example, `$a` and `$b`). It is the slot or placeholder that expects a value.
- **Argument**: The concrete value you pass when calling the function (for example, `2` and `3`).

### Returning (`return`) vs Printing (`print`) and the `: void` type
This is one of the most common stumbling blocks for people learning to program:
- `print` is a physical action: it writes text on the terminal screen for a human to read. The rest of the program **cannot reuse that output**.
- `return` is an internal memory action: it hands the calculated data back to the line that called the function so it can keep working with it (store it in a variable, save it to a database, send it over the internet, etc.).

What if your function only performs an action (like displaying a header on screen or saving a log entry) and does not need to return any data? In that case, you declare its return type as **`: void`** (empty):

<!-- joss-run: ["=== REPORTE DE VENTAS ==="] -->
```joss
public func mostrarEncabezado(string $titulo): void {
    print("=== " . $titulo . " ===")
}

mostrarEncabezado("REPORTE DE VENTAS")
```

Terminal output:

```text
=== REPORTE DE VENTAS ===
```

A `: void` function returns no value and can finish without `return` or with an empty `return` to exit early.

---

## 3. Type Contracts and Default Values

In Joss, **every source parameter must have an explicit data type** (`JOSS-TYPE-011`). If a function truly needs to accept any dynamic value, you must state so by writing `mixed`.

### Parameters with default values (Defaults)

You can make certain arguments optional by assigning them a default value with `=`:

<!-- joss-run: ["Hola, visitante", "Hola, Ada"] -->
```joss
public func saludo(string $nombre = "visitante"): string {
    return "Hola, " . $nombre
}
print(saludo())
print(saludo("Ada"))
```

Terminal output:

```text
Hola, visitante
Hola, Ada
```

- If you call `saludo()` with no arguments, Joss will automatically use `"visitante"`.
- If you call `saludo("Ada")`, the provided value replaces the default value.

> [!TIP]
> Always place parameters with default values at the end of the parameter list. Otherwise, Joss would not know which parameter to assign an argument to if you only pass one.

### Named Arguments

You can pass arguments by explicitly specifying the parameter name followed by a colon (`nombre: valor`). This lets you skip intermediate parameters that have default values or pass arguments in any order:

<!-- joss-run: ["Estimada Ada"] -->
```joss
public func bienvenida(string $nombre, string $titulo = "Estimado/a"): string {
    return $titulo . " " . $nombre
}

print(bienvenida(titulo: "Estimada", nombre: "Ada"))
```

Terminal output:

```text
Estimada Ada
```

### Spread Operator (`...`) in Calls

If you have a list (which you met in the previous chapter) and want to unpack its elements as separate positional arguments for a function call, prefix it with `...`:

<!-- joss-run: ["10"] -->
```joss
public func sumarTres(int $a, int $b, int $c): int {
    return $a + $b + $c
}

var $numeros = [2, 3, 5]
print(sumarTres(...$numeros))
```

Terminal output:

```text
10
```

---

## 4. Early Return (`guard`) and Deferred Cleanup (`defer`)

### Guard clauses with `guard` or ternary

The `return` statement stops the function immediately. To validate that input data meets a condition before continuing, Joss provides the dedicated **`guard (condición) : { ... }`** statement (in addition to the traditional ternary):

<!-- joss-run: ["agotado", "disponible (2 unidades)"] -->
```joss
public func disponibilidad(int $cantidad): string {
    guard ($cantidad > 0) : {
        return "agotado"
    }
    return "disponible (${cantidad} unidades)"
}
print(disponibilidad(0))
print(disponibilidad(2))
```

Terminal output:

```text
agotado
disponible (2 unidades)
```

How do you read `guard ($cantidad > 0) : { return "agotado" }`?
- **"Make sure that `$cantidad > 0`. Otherwise (`:`), execute this immediate exit block"**.
- You can also express it with a ternary: `($cantidad <= 0) ? { return "agotado" } : {}`.
- This pattern is called a **guard clause**. It avoids creating deeply nested conditional structures (*spaghetti code*).

> [!IMPORTANT]
> **Return exhaustiveness (`JOSS-TYPE-010`)**:
> If a function declares a return type other than `void` (such as `: string`), the semantic analyzer requires that **all possible execution paths** end with a `return` of the specified type or with a `throw` exception.

### Deferred execution on exit with `defer`

Sometimes a function opens a resource or starts an operation and you want to guarantee that, no matter what happens, a closing action runs right before the function finishes. That is what **`defer { ... }`** is for:

<!-- joss-run: ["1. Procesando pago", "2. Cerrando sesión de auditoría", "OK"] -->
```joss
public func procesarPago(): string {
    defer {
        print("2. Cerrando sesión de auditoría")
    }
    print("1. Procesando pago")
    return "OK"
}

print(procesarPago())
```

Terminal output:

```text
1. Procesando pago
2. Cerrando sesión de auditoría
OK
```

The `defer` block is registered at the start, but Joss **postpones its execution** until the exact instant the function is about to return (even if there are multiple `return` statements across different branches). If you declare multiple `defer` blocks, they run in reverse order (last registered runs first, LIFO order).

---

## 5. Variable Scope and Pass-by-Value

**Scope** is the region of the program where a variable exists and is accessible.

In Joss:
1. Every call to a function creates an **isolated memory frame**.
2. Parameters and variables declared inside the function **only exist while the function is running**. As soon as it reaches `return`, they disappear from memory.
3. Named functions **do not have automatic access to top-level variables in the file**. If a function needs a piece of data, you must pass it as a parameter.
4. **Pass-by-value**: When passing a primitive value (such as an `int` or `string`) to a function, Joss hands it a copy. Modifying that variable inside the function **does not alter the original variable outside**:

<!-- joss-run: ["20", "10"] -->
```joss
public func duplicar(int $valor): int {
    $valor = $valor * 2
    return $valor
}
var $numero = 10
print(duplicar($numero))
print($numero)
```

Terminal output:

```text
20
10
```

The original variable `$numero` keeps its value `10` intact.

---

## 6. Anonymous Functions and Closures

A function does not always need a global name. You can create a function as if it were a value, store it in a variable, or pass it as an argument to another function. This is called an **anonymous function** or **first-class function**.

When an anonymous function uses variables that were created in the surrounding outer block, it becomes a **closure**: it "captures" and remembers those variables to use them later:

<!-- joss-run: ["Hola, Ada"] -->
```joss
var $prefijo = "Hola, "
var $saludar = func(string $nombre): string {
    return $prefijo . $nombre
}
print($saludar("Ada"))
```

Terminal output:

```text
Hola, Ada
```

- Closures do not take visibility modifiers (`public` or `private`).
- They are ideal for transforming or filtering collections with the native `map` and `filter` functions:

<!-- joss-run: ["[20 40 60]"] -->
```joss
var $precios = [10, 20, 30]
var $dobles = map($precios, func(int $p): int {
    return $p * 2
})
print($dobles)
```

Terminal output:

```text
[20 40 60]
```

---

## 7. Temporary Mutable References with `ref`

What if you really want a function to modify the original variable you passed in from the outside? Instead of returning a copy, Joss allows the use of **references (`ref`)**.

For safety, Joss requires the intent to be **two-sided**: both the function signature and the call site must include the `ref` keyword:

<!-- joss-run: ["2"] -->
```joss
public func incrementar(ref int $valor): int {
    $valor = $valor + 1
    return $valor
}
var $contador = 1
incrementar(ref $contador)
print($contador)
```

Terminal output:

```text
2
```

Now `$contador` did change its original value to `2`.

### Safety rules for references in Joss:
1. **Only simple, mutable variables**: You cannot pass constants, mathematical expressions, or literals (for example, `incrementar(ref 5)` is illegal with `JOSS-REF-002`).
2. **Strict type invariance**: The variable's type must match the parameter's type exactly (passing an `int` to a `ref float` is not allowed).
3. **They cannot escape**: A reference only lives for the duration of the call. You cannot store a reference in a global variable, return it with `return`, or send it to an `async` function.

---

## 8. The Pipeline Operator (`|>`)

In programming, it is very common to pass a piece of data through a series of successive transformations. In classic languages, this forces you to nest calls from the inside out:

```joss
// Difícil de leer: debes leer de derecha a izquierda o de adentro hacia afuera
strtoupper(trim("  ada  "))
```

Joss natively includes the **Pipeline (`|>`)** operator, which takes the result on the left and sends it as the first argument to the function on the right:

<!-- joss-run: ["ADA"] -->
```joss
print("  ada  " |> trim |> strtoupper)
```

Terminal output:

```text
ADA
```

The flow reads naturally from left to right:
1. Take the string `"  ada  "`.
2. Pass it through `trim` (which removes the spaces, resulting in `"ada"`).
3. Pass it through `strtoupper` (which converts to uppercase, resulting in `"ADA"`).
4. Pass it to `print` to display it on screen.

You can also combine pipelines with custom functions, collections, and closures to create very expressive data-processing flows:

<!-- joss-run: ["Precio: 25 USD"] -->
```joss
public func formatearMoneda(int $monto, string $moneda = "USD"): string {
    return "Precio: ${monto} ${moneda}"
}

var $resultado = 25 |> formatearMoneda
print($resultado)
```

Terminal output:

```text
Precio: 25 USD
```

And with lists using `filter`:

<!-- joss-run: ["60"] -->
```joss
var $ventas = [5, 10, 20, 30]
var $altas = $ventas |> filter(func(int $x): bool { return $x >= 10 })
print(sum($altas))
```

Terminal output:

```text
60
```

---

## 9. Recursive Functions

A **recursive** function is one that calls itself to solve a problem by breaking it down into smaller versions of the same problem.

Every recursive function must have two essential components:
1. **Base case**: A stopping condition where the function returns a simple result without calling itself again.
2. **Recursive step**: Where the function calls itself with data closer to the base case.

Joss protects your memory by limiting the maximum depth of recursive calls to 1024 levels by default (see [Recursion](RECURSION.md) for more details).

---

## 10. Common Function Mistakes

| Error | Code / Cause | Solution |
|---|---|---|
| Writing `function f()` | `function` was removed from Joss. | Use `public func f()` or `private func f()`. |
| Forgetting `public` or `private` on global functions | Named functions require mandatory visibility. | Add `public func nombre(...)` or `private func nombre(...)`. |
| Untyped parameter: `func($x)` | Parameters must be typed (`JOSS-TYPE-011`). | Declare the type: `func(int $x)` or `func(mixed $x)`. |
| Typed function without return on all paths | `JOSS-TYPE-010`: The checker detected a path that finishes without returning anything. | Make sure all ternary branches return or throw an error. |
| Forgetting `ref` at the call site | Calling `f($x)` when the function expects `ref T $param` (`JOSS-REF-001`). | Add `ref` at the call site: `f(ref $x)`. |

---

## 11. Practical Exercises

### Level 1 — Quick check
Write a function `public func presentar(string $nombre, string $rol = "Estudiante"): string` that returns `"${nombre} (${rol})"`. Call it first passing only `"Ada"` and then passing `"Grace"` and `"Arquitecta"`. Print both results.

### Level 2 — Guided exercise
1. Write a function `public func calcularIVA(decimal $subtotal, decimal $tasa = 0.16m): decimal`.
2. Use a guard clause `guard ($subtotal > 0.0m) : { return 0.0m }` to avoid calculating tax on zero or negative amounts.
3. If the subtotal is valid, return `$subtotal * $tasa`.
4. Test calling it with `0.0m`, `100.0m` (using the default rate), and `100.0m` with rate `0.08m`.

### Level 3 — Challenge
Given the list `var $precios = [50, 120, 80, 200, 30]`:
1. Use the pipeline operator (`|>`) together with `filter` to keep only prices greater than or equal to `100`.
2. Chain another `|>` with `map` to apply a discount by subtracting `15` from each filtered price.
3. Chain `|> sum` at the end and print the resulting total sum (it should print `290`).

---

## Next step

Now that you know how to organize your logic into clean, reusable functions, you will learn how to detect, throw, and recover from unexpected situations without your program crashing.

Continue with: [Error handling, exceptions, and diagnostics](ERRORES.md).

# Functions, variable scope, closures and references

Before: [Flow control and decisions](CONTROL_FLUJO.md). After: [Collections: Arrays, Maps and Text](COLECCIONES.md).
Technical reference: [Syntax](SINTAXIS.md), [Recursion](RECURSION.md), [Type system](SISTEMA_TIPOS.md).

---

## What are you going to learn here?

As programs grow, writing hundreds of instructions in a row becomes unmanageable. If you need to calculate the total of a tax invoice in ten different places in your application, copying and pasting the same formula ten times is a recipe for disaster: if the tax law changes, you would have to find and correct ten files, and sooner or later you will forget one.

In this guide you will learn:
1. What is a **function** and why it is the fundamental building block of software.
2. The critical difference between **parameters**, **arguments** and **impression** versus **return**.
3. How to declare functions with safe types and default values.
4. The mandatory visibility rules in Joss (`public` and `private`).
5. What is variable scope and how memory isolation works in each call.
6. What **anonymous functions (closures)** are and how they capture data from their environment.
7. How to modify external variables safely using **references (`ref`)**.
8. How to chain clean data transformations with the **pipeline (`|>`)** operator.

---

## 1. What is a function?

A **function** is an autonomous block of instructions to which we assign a name. Think of it as a small specialized machine:
1. Receives raw materials (input data, called **arguments**).
2. It processes the information inside in isolation.
3. Returns a finished product (the result, called **return value**).

<!-- joss-run: ["5", "12"] -->
```joss
public func sumar(int $a, int $b): int {
    return $a + $b
}
print(sumar(2, 3))
print(sumar(5, 7))
```

### Anatomy of a function in Joss:

- `public`: **Visibility modifier**. In Joss, global functions require visibility:
  - `public`: The function is available throughout the project and other files can use it without imports.
  - `private`: The function can only be called from the same file where it was written.
- `func`: Keyword indicating the start of the declaration (Joss does not use `function`).
- `sumar`: The name of the function.
- `(int $a, int $b)`: The list of **parameters**. Defines what types of data the function requires to work.
- `: int`: The **return type**. Declares what type of value the function promises to return to the caller.
- `{ ... }`: The **body** of the function, where the instructions are written.
- `return $a + $b`: The instruction `return` ends the execution of the function immediately and sends the result back to the caller.

---

## 2. Crucial conceptual differences

### Parameters vs Arguments
- **Parameter**: It is the variable that you declare in the function signature (for example, `$a` and `$b`). It is the slot or space that expects a value.
- **Argument**: It is the specific value that you provide when calling the function (for example, `2` and `3`).

### Return (`return`) vs Print (`print`)
This is one of the most common pitfalls for those who start programming:
- `print` is a physical action: write ink on the terminal screen for a human to read. The rest of the program **cannot reuse that output**.
- `return` is an internal memory action: it delivers the calculated data to the line that called the function so that it can continue operating with it (save it in a variable, save it in a database, send it over the internet, etc.).

---

## 3. Contract types and default values

In Joss, **every source parameter must have an explicit data type**. If a function really needs to accept any dynamic value, you must indicate this by writing `mixed`.

### Parameters with default values ​​(Defaults)

You can make certain arguments optional by assigning them a default value with `=`:

<!-- joss-run: ["Hola, visitante", "Hola, Ada"] -->
```joss
public func saludo(string $nombre = "visitante"): string {
    return "Hola, " . $nombre
}
print(saludo())
print(saludo("Ada"))
```

- If you call `saludo()` without arguments, Joss will automatically use `"visitante"`.
- If you call `saludo("Ada")`, the delivered value replaces the default value.

> [!TIP]
> Always place parameters with default values ​​at the end of the parameter list. Otherwise Joss wouldn't know which parameter to assign an argument to if you only pass one.

### Named Arguments

You can pass arguments by explicitly stating the parameter name followed by a colon (`nombre: valor`). This allows you to skip intermediate parameters that have default values ​​or pass arguments in any order:

<!-- joss-run: ["Estimada Ada"] -->
```joss
public func bienvenida(string $nombre, string $titulo = "Estimado/a"): string {
    return $titulo . " " . $nombre
}

print(bienvenida(titulo: "Estimada", nombre: "Ada"))
```

### Spread operator (`...`) in calls

If you have a list and want to unpack its elements as separate positional arguments for a function call, prepend `...`:

<!-- joss-run: ["10"] -->
```joss
public func sumarTres(int $a, int $b, int $c): int {
    return $a + $b + $c
}

$numeros = [2, 3, 5]
print(sumarTres(...$numeros))
```

---

## 4. Early return and Guard Clauses

The statement `return` stops the function immediately. If you put it inside a ternary block, you can check for errors at the beginning of the function and exit before processing the rest:

<!-- joss-run: ["agotado", "disponible"] -->
```joss
public func disponibilidad(int $cantidad): string {
    ($cantidad <= 0) ? { return "agotado" } : {}
    return "disponible"
}
print(disponibilidad(0))
print(disponibilidad(2))
```

This pattern is called **guard clause**. Avoid creating deeply nested conditional structures (*spaghetti code*).

> [!IMPORTANT]
> **Return completeness (`JOSS-TYPE-010`)**:
> If a function declares a return type (such as `: string`), the semantic analyzer requires that **all possible** execution paths end with a `return` of the indicated type or with an exception `throw`. You can't leave unfinished branches where the function simply ends without returning anything.

---

## 5. Scope of variables (Scope) and passing by value

The **scope** is the area of ​​the program where a variable exists and is accessible.

In Joss:
1. Each function call creates an **isolated memory frame (frame)**.
2. Parameters and variables declared within the function **only exist while the function is executing**. As soon as you reach `return`, they disappear from memory.
3. Named functions **do not have automatic access to file global variables**. If a function needs data, you must pass it to it as a parameter.
4. **Pass by value**: When you pass a primitive value (such as a `int` or `string`) to a function, Joss gives it a copy. Modifying that variable inside the function **does not alter the original variable that was outside**:

<!-- joss-run: ["20", "10"] -->
```joss
public func duplicar(int $valor): int {
    $valor = $valor * 2
    return $valor
}
$numero = 10
print(duplicar($numero))
print($numero)
```
The original variable `$numero` keeps its value `10` intact.

---

## 6. Anonymous Functions and Closures

A function does not always need a global name. You can create a function as a value, store it in a variable, or pass it as an argument to another function. This is called **anonymous function** or **first class function**.

When an anonymous function uses variables that were created in the outer block surrounding it, it becomes a **closure**: "capture" and remembers those variables for later use, even if the outer block has already ended:

<!-- joss-run: ["Hola, Ada"] -->
```joss
$prefijo = "Hola, "
$saludar = func(string $nombre): string {
    return $prefijo . $nombre
}
print($saludar("Ada"))
```

- Closures are widely used as **callbacks**: functions that you give to a service (for example, to an HTTP server or a WebSocket) to execute when an event occurs (such as the arrival of a client).
- Closures do not have visibility modifiers (`public` or `private`).

---

## 7. Temporary mutable references with `ref`

What if you really want a function to modify the original variable you passed to it from outside? Instead of returning a copy, Joss allows the use of **references (`ref`)**.

For security, Joss requires that the intent be **bilateral**: both the function signature and the call must include the keyword `ref`:

<!-- joss-run: ["2"] -->
```joss
public func incrementar(ref int $valor): int {
    $valor = $valor + 1
    return $valor
}
$contador = 1
incrementar(ref $contador)
print($contador)
```

Now `$contador` did change its original value to `2`.

### Reference security rules in Joss:
1. **Simple and mutable variables only**: You cannot pass constants, mathematical expressions or literals (for example, `incrementar(ref 5)` is illegal with `JOSS-REF-002`).
2. **Strict type invariance**: The type of the variable must exactly match the type of the parameter (passing a `int` to a `ref float` is not allowed).
3. **They cannot escape**: A reference only lives during the call. You cannot save a reference to a global variable, return it with `return`, or send it over an asynchronous channel.

---

## 8. The Pipeline operator (`|>`)

In programming it is very common to have to pass a data through a series of successive transformations. In classical languages, this forces calls to be nested from the inside out:

```joss
// Difícil de leer: debes leer de derecha a izquierda o de adentro hacia afuera
strtoupper(trim("  ada  "))
```

Joss natively includes the **Pipeline (`|>`)** operator, which takes the result on the left and sends it as the first argument to the function on the right:

<!-- joss-run: ["ADA"] -->
```joss
print("  ada  " |> trim |> strtoupper)
```

The flow reads naturally from left to right:
1. Take the text `"  ada  "`.
2. Pass it through `trim` (which removes the spaces, resulting in `"ada"`).
3. Pass it through `strtoupper` (which converts to uppercase, resulting in `"ADA"`).
4. Pass it to `print` to display it on the screen.

If the receiving function needs more than one argument, write them normally in parentheses:
```joss
$resultado = $texto |> str_replace("a", "o")
```
Joss will place the value on the left as the first parameter of `str_replace`.

---

## 9. Recursive functions

A **recursive** function is one that calls itself to solve a problem by dividing it into smaller versions of the same problem.

Every recursive function must have two essential components:
1. **Base case**: A stopping condition where the function returns a simple result without being called again.
2. **Recursive step**: Where the function calls itself with a piece of data closest to the base case.

Joss protects your memory by limiting the maximum depth of recursive calls to 1024 levels by default (see [Recursion](RECURSION.md) for details).

---

## 10. Common mistakes with functions

| Error | Code / Cause | Solution |
|---|---|---|
| Write `function f()` | `function` was removed from Joss. | Use `public func f()` or `private func f()`. |
| Forget `public` or `private` in global functions | Named functions require mandatory visibility. | Add `public func nombre(...)` or `private func nombre(...)`. |
| Untyped parameter: `func($x)` | The parameters must be typed (`JOSS-TYPE-011`). | Declare the type: `func(int $x)` or `func(mixed $x)`. |
| Function typed without return in all routes | `JOSS-TYPE-010`: The checker detected a path that ends without returning anything. | Make sure all ternary branches return or throw an error. |
| Forget `ref` in call | Call `f($x)` when the function expects `ref T $param` (`JOSS-REF-001`). | Add `ref` in the call: `f(ref $x)`. |

---

## 11. Practical exercise

1. **Calculator with functions**:
   - Write a function `public func calcularIVA(decimal $subtotal, decimal $tasa = 0.16m): decimal`.
   - The function must return the tax amount (`$subtotal * $tasa`).
   - Try calling it with a single argument (`calcularIVA(100.0m)`) and then with a custom rate of 8% (`calcularIVA(100.0m, 0.08m)`).
   - Show both results in the console.

---

## Next step

Now that you know how to structure modular code with safe functions, it's time to learn how to manipulate complex data collections: lists of elements and associative key-value maps.

Continue with: [Collections: Arrays, Maps and Text Manipulation](COLECCIONES.md).

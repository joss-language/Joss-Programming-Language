# Flow control and decision making

Before: [Values ​​and variables](FUNDAMENTOS.md). After: [Functions and closures](FUNCIONES.md).
Technical reference: [Syntax and operators](SINTAXIS.md), [Grammar](GRAMATICA.md).

---

## What are you going to learn here?

So far, all of our programs have run in a straight line: the computer reads line 1, then 2, then 3, and is done. This natural order is called **sequential flow**.

However, the true power of programming lies in the ability to respond to changing circumstances:
- *If* the user typed the correct password, let him enter; *otherwise* it displays an error message.
- *As long as* there are emails left to send, continue sending them one by one.
- *For each* product in the shopping cart, add its price to the total.

In this guide you will learn:
1. What is a **logical condition** and how to evaluate it.
2. How to make decisions in Joss using the **ternary with blocks** operator and why Joss does not use the classic syntax `if/else`.
3. The Elvis operator `?:` and the null coalescence operator `??`.
4. How to structure elegant multiple selections with the expression `match`.
5. How to repeat code with loops `while`, `do...while` and `foreach` (including their use with concurrency and key-value channels).
6. How to interrupt or advance a cycle with `break` and `continue`.
7. How to ensure resource cleanup and task completion with `defer`.

---

## 1. Logical decisions: Ask the computer

A **condition** is any expression that the computer evaluates to obtain a logical response of type Boolean (`bool`): either it is true (`true`) or it is false (`false`).

<!-- joss-run: ["true", "Entrada permitida"] -->
```joss
$edad = 20
print($edad >= 18)
print(($edad >= 18) ? "Entrada permitida" : "Debes esperar")
```

### Comparison operators

| Operator | Logical question | Example | Result |
|---|---|---|---|
| `==` | Are the values ​​the same? | `5 == 5` | `true` |
| `!=` | Are the values ​​different? | `5 != 3` | `true` |
| `===` | Are they strictly identical in value **and type**? | `5 === "5"` | `false` |
| `!==` | Aren't they strictly identical? | `5 !== "5"` | `true` |
| `<` | Is the one on the left smaller? | `3 < 5` | `true` |
| `<=` | Is it less or equal? | `5 <= 5` | `true` |
| `>` | Is it strictly older? | `10 > 2` | `true` |
| `>=` | Is it greater or equal? | `20 >= 18` | `true` |
| `<=>` | Spaceship operator (Spaceship) | `$a <=> $b` | Returns `-1` if `$a < $b`, `0` if `$a == $b`, `1` if `$a > $b`. |

---

## 2. Joss' philosophy: The ternary operator as a control structure

Unlike other languages ​​that have a reserved word `if` and another `else`, **Joss unifies all decisions under the ternary operator**.

The basic structure of the ternary is:

```text
(condición) ? resultado_si_es_verdadero : resultado_si_es_falso
```

### Why did Joss choose this design?

1. **It is an expression, not an isolated statement**: You can assign the result of a decision directly to a variable without creating intermediate empty variables:
   ```joss
   $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
   ```
2. **Clean and unambiguous visual structure**: Avoid the classic problems of `if` without confusing braces or nesting.

### Execute multiple instructions with `{ ... }` blocks

When you need to run multiple lines of code in one of the branches, simply put a block between braces `{` and `}`:

<!-- joss-run: ["Hay existencias", "Preparando pedido"] -->
```joss
$existencias = 4
($existencias > 0) ? {
    print("Hay existencias")
    print("Preparando pedido")
} : {
    print("Producto agotado")
}
```

### What if I don't need the fake branch?

If you only want to do something when the condition is true and you don't need a false alternative, you can skip the `: { ... }` branch completely:

<!-- joss-run: ["Bienvenido de nuevo"] -->
```joss
$usuarioAutenticado = true
($usuarioAutenticado) ? {
    print("Bienvenido de nuevo")
}
```

It is also valid to write the symmetric form with empty block `: {}` if you prefer to keep both sides explicit.

### Guard Clauses with ternaries and the statement `guard`

In Joss, if you execute an instruction `return` inside a ternary block, the `return` **immediately bubbles out of the containing function**. This allows you to write clean *guard clauses* and avoid deep nesting:

```joss
public func procesarPago(decimal $monto): bool {
    ($monto <= 0.0m) ? {
        print("Error: Monto inválido")
        return false
    }

    // El código continúa en línea recta
    print("Procesando pago de: " . $monto)
    return true
}
```

### Native statement `guard ... :` with mandatory termination and Smart Casts

Consistent with Joss philosophy where `if` and `else` do not exist, the statement **`guard`** adopts the colon `:` of the ternary operator to define its escape block:

- Expresses in the condition the **desired** state to continue execution in a straight line.
- If the condition is not met, the block after the colon `:` is executed.
- The escape block **must terminate the function** using `return` or `throw`; otherwise, the analyzer issues the diagnostic error `JOSS-FLOW-005`.
- When exiting the escape branch, the type system performs **Smart Cast** (*Type Narrowing*), narrowing types like `T|null` directly to `T` in the subsequent main stream:

<!-- joss-run: ["Procesando: 50"] -->
```joss
public func procesar(int $monto): string {
    guard ($monto > 0) : {
        return "Monto inválido"
    }
    return "Procesando: " . $monto
}
print(procesar(50))
```

---

## 3. Elvis Operators `?:` and Null Coalescence `??`

Joss provides two very powerful shortcuts for assigning default values:

### The Elvis Operator (`?:`)
Evaluate the expression on the left. If true (or has a non-empty, non-zero value), it returns that expression; if false or empty, returns the value on the right:

```joss
$apodo = $aliasIngresado ?: "Anónimo"
```

### The null coalescence operator (`??`)
It focuses exclusively on the existence of a value. If the variable on the left is `null` (or is not defined), it returns the alternative on the right:

```joss
$configuracion = $opcionUsuario ?? "valor_predeterminado"
```

---

## 4. Multiple selection with `match`

When a variable can have many possible values ​​(for example, the status of a submission, the role of a user, or the response code of a server), chaining ternaries becomes difficult to read.

For these cases, Joss offers the expression **`match`**:

<!-- joss-run: ["En camino"] -->
```joss
$estado = "enviado"
$mensaje = match ($estado) {
    "nuevo" => "Preparando",
    "enviado", "reparto" => "En camino",
    default => "Consulta el pedido"
}
print($mensaje)
```

### Features of `match`:
- **Multiple arms**: Each line is composed of one or more patterns, followed by a thick arrow `=>` and the resulting value or block.
- **Support for statement blocks**: Arms can contain multiline blocks enclosed in braces `{ ... }` to execute several consecutive instructions or update the program state:

<!-- joss-run: ["Opción 1 ejecutada"] -->
```joss
$opcion = 1
match ($opcion) {
    1 => {
        print("Opción 1 ejecutada")
    },
    default => {
        print("Opción por defecto")
    }
}
```
- **Grouping with commas**: You can associate several values ​​with the same result in a single line (for example `"enviado", "reparto"`).
- **Default arm (`default`)**: Covers any values ​​that do not match the previous ones. It is good practice to always include it to avoid undefined results.
- **No fall-through**: Unlike the old `switch` in C or Java, `match` only runs the first matching arm and terminates; does not require keywords like `break`.

---

## 5. Loops and repetition structures

A **loop** (or cycle) tells the computer to execute a block of code over and over again as long as a condition remains true.

### The loop `while` (repeat while)

The loop `while` evaluates the condition **before** entering the body of the loop. If the condition is false from the beginning, the body is not even executed once:

<!-- joss-run: ["1", "2", "3"] -->
```joss
$numero = 1
while ($numero <= 3) {
    print($numero)
    $numero++
}
```

> [!CAUTION]
> **Beware of infinite loops**:
> Within the body of a `while`, there should always be a statement that modifies the condition variables (such as `$numero++`). If the condition never becomes false, the program will be stuck forever consuming processor until you force stop it in your terminal (in Joss, you can press the `q` or `Ctrl + C` key).

### The loop `do ... while` (do at least once)

Unlike `while`, the loop `do ... while` runs the body **first** and checks the condition at the end. This guarantees that the instructions will be executed at least once, regardless of the initial condition:

<!-- joss-run: ["Intento 1"] -->
```joss
$intento = 0
do {
    $intento++
    print("Intento " . $intento)
} while ($intento < 1)
```

---

## 6. Browse collections and channels with `foreach`

When you have a list of data (such as a `array` of names or products), you don't need to manually manage a numerical counter: you use **`foreach`**.

<!-- joss-run: ["pan", "leche"] -->
```joss
$compras = ["pan", "leche"]
foreach ($compras as $producto) {
    print($producto)
}
```

At each turn of the loop, Joss takes the next element from the collection `$compras`, deposits it into the temporary variable `$producto`, and executes the code block.

### Iteration over numeric ranges (`..`)

You can also iterate continuous number sequences without manually creating arrays using the range operator `..`:

<!-- joss-run: ["Paso 1", "Paso 2", "Paso 3"] -->
```joss
foreach (1..3 as $paso) {
    print("Paso ${paso}")
}
```

### Iteration over concurrency channels (`channel`)
A distinctive feature of Joss is that `foreach` is not only used to traverse static lists in memory: it can also consume **concurrent communication channels** (`channel`). The loop will read messages from the channel in real time until the channel is closed with `close($canal)`.

---

## 7. Loop control: `break` and `continue`

Within any loop (`while`, `do...while` or `foreach`), you can alter the repetition flow with two fundamental instructions:

### `break`: End the loop immediately
Abort the loop and jump directly to the first line after the loop:

<!-- joss-run: ["1"] -->
```joss
foreach ([1, 2, 3, 4] as $numero) {
    print($numero)
    break
}
```
In this example, only the `1` is printed because the statement `break` cancels the loop immediately.

### `continue`: Skip to next lap
Skips the remaining instructions from the loop body for the current loop only, advancing to the next iteration:

```joss
foreach ([1, 2, 3, 4, 5] as $n) {
    ($n % 2 == 0) ? {
        continue // Si es par, sáltatelo
    } : {}
    print("Impar: " . $n)
}
```

---

## 8. Guaranteed resource cleanup: `defer`

The `defer` instruction postpones the execution of a block or expression until the exact moment the current function or file ends, guaranteeing the release of memory, closing of files or connections. Multiple `defer` statements are executed in **LIFO** (Last In, First Out) order:

<!-- joss-run: ["inicio", "fin", "limpieza 2", "limpieza 1"] -->
```joss
public func tarea() {
    print("inicio")
    defer {
        print("limpieza 1")
    }
    defer {
        print("limpieza 2")
    }
    print("fin")
}
tarea()
```

---

## 9. Common mistakes and good practices

| Error | Cause | Solution |
|---|---|---|
| Write `if ($x > 0)` | `if` does not exist in Joss's grammar. | Use ternary syntax: `($x > 0) ? { ... } : { ... }`. |
| Forget the `:` in the ternary | The false branch is missing. | If you have nothing to do in the fake branch, type `{}`: `($x > 0) ? { print("ok") } : {}`. |
| Infinite loop at `while` | Forgetting to increase or change the control variable. | Make sure to update the counter or flag inside the loop body. |
| `break` or `continue` outside a cycle | Place them at the top level of a file or function. | They are only valid within a `while`, `do...while` or `foreach`. |

---

## 10. Practical exercises

1. **Note Sorter**:
   - Declare an integer variable `$nota = 85`.
   - Using a ternary with blocks, print:
     - "Outstanding" if `$nota >= 90`.
     - "Approved" if `$nota >= 60` and `$nota < 90`.
     - "Failed" if `$nota < 60`.
2. **Summation with `foreach`**:
   - Create an array `$precios = [10, 25, 5, 40]`.
   - Initializes a variable `$total = 0`.
   - Walk through the array with `foreach`, adding each price to `$total`.
   - Print the final total (must give `80`).

---

## Next step

Now that you can make decisions and repeat operations, we'll learn how to package reusable blocks of logic with proper names, parameters, and return values:

Continue with: [Functions, variable scope (scope), closures and references](FUNCIONES.md).

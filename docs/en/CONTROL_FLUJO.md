# Flow control and decision making

Before: [Values and variables](FUNDAMENTOS.md). Next: [Collections: Arrays, Maps and Text](COLECCIONES.md).
Technical reference: [Syntax and operators](SINTAXIS.md), [Grammar](GRAMATICA.md).

---

## What will you learn here?

So far, our programs have executed in a straight line: the computer reads line 1, then line 2, then line 3 and finishes. This natural order is called **sequential flow**.

However, the true power of programming lies in responding to changing situations and repeating tasks without getting tired:
- *If* the user is 18 or older, allow entry; *otherwise*, ask them to wait.
- *While* the counter has not reached 3, keep counting.
- *For each* product in a list, display its name on screen.

In this guide you will learn step by step:
1. What a **logical condition** (`bool`) is and how to compare data.
2. How to combine logical questions with `&&` (AND), `||` (OR) and `!` (NOT).
3. How to make decisions in Joss using the **ternary operator (`? :`)** with values and with blocks `{ ... }`.
4. How to assign default values with the Elvis operator `?:` and null coalescing `??` / `??=`.
5. How to cleanly choose among many options with **`match`**.
6. How to repeat instructions with **`while`**, **`do ... while`** and **`foreach`** loops.
7. How to stop or skip an iteration of a loop with **`break`** and **`continue`**.

---

## 1. Logical decisions: Asking the computer questions

A **condition** is a question that the computer evaluates to answer solely with a boolean value (`bool`): **`true`** or **`false`**.

<!-- joss-run: ["true", "Entrada permitida"] -->
```joss
var $edad = 20
print($edad >= 18)
print(($edad >= 18) ? "Entrada permitida" : "Debes esperar")
```

Output:

```text
true
Entrada permitida
```

### Comparison operators

| Operator | Question it asks | Example | Result |
|---|---|---|---|
| `==` | Do they have the same value? | `5 == 5` | `true` |
| `!=` | Are they different? | `5 != 3` | `true` |
| `===` | Are they identical in value **and also in type**? | `5 === "5"` | `false` |
| `!==` | Are they not strictly identical? | `5 !== "5"` | `true` |
| `<` | Is the left side smaller? | `3 < 5` | `true` |
| `<=` | Is it less than or equal? | `5 <= 5` | `true` |
| `>` | Is the left side greater? | `10 > 2` | `true` |
| `>=` | Is it greater than or equal? | `20 >= 18` | `true` |
| `<=>` | Three-way comparison (*Spaceship*) | `$a <=> $b` | Returns `-1` if `$a < $b`, `0` if equal and `1` if `$a > $b`. |

### Combining multiple conditions (`&&`, `||`, `!`)

When a decision depends on two or more questions at the same time, we use **logical operators**:

<!-- joss-run: ["true", "true", "false"] -->
```joss
var $edad = 22
var $tieneEntrada = true

print($edad >= 18 && $tieneEntrada)
print($edad < 18 || $tieneEntrada)
print(!$tieneEntrada)
```

Output:

```text
true
true
false
```

- **`&&` (Logical AND)**: Returns `true` only if **both** conditions are met.
- **`||` (Logical OR)**: Returns `true` if **at least one** of the conditions is met.
- **`!` (Negation)**: Inverts the logical value (`!true` is `false`, and `!false` is `true`).

---

## 2. Making decisions in Joss: The ternary operator (`? :`)

Unlike other languages that use the words `if` and `else`, **Joss unifies all decisions under a single clear construct: the ternary operator**.

Its basic form reads like a natural question:

```text
(¿se cumple la condición?) ? resultado_si_es_verdadero : resultado_si_es_falso
```

### 1. Choose between two values and store them in a variable

Because the ternary produces a value directly, you can store the answer in a variable in a single line:

<!-- joss-run: ["Aprobado"] -->
```joss
var $puntos = 75
var $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
print($estado)
```

Output:

```text
Aprobado
```

- First `($puntos >= 60)` is evaluated. Since `75 >= 60` is `true`, Joss picks the text after the `?` sign (`"Aprobado"`) and stores it in `$estado`.
- If `$puntos` had been `40`, it would have picked the text after the colon `:` (`"Reprobado"`).

### 2. Execute multiple lines with `{ ... }` blocks

A **block** is a group of instructions enclosed in braces `{` and `}`. When you want a branch to execute several steps in a row, simply place a block `{ ... }`:

<!-- joss-run: ["Hay existencias", "Preparando pedido"] -->
```joss
var $existencias = 4
($existencias > 0) ? {
    print("Hay existencias")
    print("Preparando pedido")
} : {
    print("Producto agotado")
}
```

Output:

```text
Hay existencias
Preparando pedido
```

### 3. What if I only need to do something when it is true?

If you do not need to do anything when the condition is false, you can **completely omit the `: { ... }` part**:

<!-- joss-run: ["Bienvenido de nuevo"] -->
```joss
var $usuarioAutenticado = true
($usuarioAutenticado) ? {
    print("Bienvenido de nuevo")
}
```

Output:

```text
Bienvenido de nuevo
```

*(It is also valid to write `: {}` at the end if you prefer to leave the empty block explicit).*

> [!NOTE]
> **What about guard (`guard`) and cleanup (`defer`) clauses?**
> Joss also features `guard (condicion) : { ... }` statements (to validate conditions and exit a function early) and `defer { ... }` (to run closing tasks when a function finishes). You will learn them in the [Functions](FUNCIONES.md) chapter as soon as you know how `public func` and `return` work.

---

## 3. Quick default values: Elvis (`?:`) and Null Coalescing (`??` and `??=`)

Many times you want to say: *"use this data if it exists, or use this fallback value if it is empty"*. Joss has three operators designed for this:

<!-- joss-run: ["Anónimo", "Invitado", "oscuro"] -->
```joss
var $aliasIngresado = ""
var $apodo = $aliasIngresado ?: "Anónimo"
print($apodo)

var $opcionUsuario = null
var $perfil = $opcionUsuario ?? "Invitado"
print($perfil)

var $tema = null
$tema ??= "oscuro"
print($tema)
```

Output:

```text
Anónimo
Invitado
oscuro
```

| Operator | When it uses the fallback value (right) | When to choose it |
|---|---|---|
| **`?:` (Elvis)** | When the left side is falsy or empty (`null`, `false`, `0`, `""`, `[]`). | When an empty string `""` or `0` should be replaced by the default value. |
| **`??` (Null coalescing)** | **Only** when the left side is `null`. | When `0`, `false` or `""` are valid values you want to keep and you only want to replace `null`. |
| **`??=` (Null assignment)** | Stores the right-hand value into the variable only if it currently holds `null`. | To initialize variables that might be `null`. |

---

## 4. Multi-way selection with `match`

When a variable can take many different values (such as traffic light colors, days of the week or an order status), using the **`match`** expression makes the code clean and easy to read:

<!-- joss-run: ["En camino"] -->
```joss
var $estado = "enviado"
var $mensaje = match ($estado) {
    "nuevo" => "Preparando paquete",
    "enviado", "reparto" => "En camino",
    "entregado" => "Pedido completado",
    default => "Consulta el estado de tu pedido"
}
print($mensaje)
```

Output:

```text
En camino
```

### How `match` works:
1. Joss compares the value of `$estado` with the options on each line from top to bottom.
2. You can group several cases with a comma (for example `"enviado", "reparto" => "En camino"`).
3. As soon as it finds the first match, it returns what is to the right of the `=>` arrow and finishes (you do not need to write `break`).
4. **`default`**: Is the wildcard option executed if no previous case matched.
5. **Blocks in `match`**: If instead of returning a value you want to execute multiple instructions in each case, you can also use braces `{ ... }`:

<!-- joss-run: ["Opción 1 ejecutada"] -->
```joss
var $opcion = 1
match ($opcion) {
    1 => {
        print("Opción 1 ejecutada")
    },
    default => {
        print("Opción por defecto")
    }
}
```

Output:

```text
Opción 1 ejecutada
```

---

## 5. Loops: Repeating instructions with `while` and `do ... while`

A **loop** tells the computer to repeat a `{ ... }` block while a condition remains true (`true`).

### The `while` loop (repeat while the condition holds)

`while` checks the condition **before** each iteration:

<!-- joss-run: ["1", "2", "3"] -->
```joss
var $numero = 1
while ($numero <= 3) {
    print($numero)
    $numero++
}
```

Output:

```text
1
2
3
```

#### What happens iteration by iteration in memory?
1. **Start**: `$numero` is `1`.
2. **Iteration 1**: Is `1 <= 3`? Yes (`true`). It prints `1` and `$numero++` increases it to `2`.
3. **Iteration 2**: Is `2 <= 3`? Yes (`true`). It prints `2` and `$numero++` increases it to `3`.
4. **Iteration 3**: Is `3 <= 3`? Yes (`true`). It prints `3` and `$numero++` increases it to `4`.
5. **Final check**: Is `4 <= 3`? No (`false`). The loop ends and the program continues below.

> [!CAUTION]
> **Avoid infinite loops**:
> Inside the braces of a `while` loop, the condition variable must always change (for example with `$numero++`). If you forget that line, `$numero` will remain `1` forever and the loop will never end (if this happens in your terminal, press `Ctrl + C` to stop it).

### The `do ... while` loop (execute at least once)

Unlike `while`, the `do ... while` loop executes the block **first** and checks the condition **at the end**. That way you ensure the block runs at least once:

<!-- joss-run: ["Intento 1"] -->
```joss
var $intento = 0
do {
    $intento++
    print("Intento " . $intento)
} while ($intento < 1)
```

Output:

```text
Intento 1
```

---

## 6. Iterating over sequences and lists with `foreach`

When you want to iterate over a sequence of numbers from `1` to `3` or step through every element of a list without worrying about manually incrementing a counter, use **`foreach`** (*for each one*).

### 1. Iterate over a range of numbers (`inicio..fin`)

The double-dot operator `..` generates the integers from start to end:

<!-- joss-run: ["Paso 1", "Paso 2", "Paso 3"] -->
```joss
foreach (1..3 as $paso) {
    print("Paso ${paso}")
}
```

Output:

```text
Paso 1
Paso 2
Paso 3
```

### 2. Iterate over a simple list

> **Quick preview**: In the next chapter ([Collections](COLECCIONES.md)) we will learn everything about lists (`array`). For now, it is enough to know that writing several values inside brackets `["pan", "leche"]` creates an ordered list of elements.

See how easy it is to iterate over each element of a list with `foreach`:

<!-- joss-run: ["pan", "leche"] -->
```joss
var $compras = ["pan", "leche"]
foreach ($compras as $producto) {
    print($producto)
}
```

Output:

```text
pan
leche
```

- **On the first iteration**, `$producto` takes the value `"pan"` and prints it.
- **On the second iteration**, `$producto` takes the value `"leche"` and prints it.
- Once no more elements remain in `$compras`, the loop stops on its own.

---

## 7. Controlling the inside of a loop: `break` and `continue`

Inside any loop (`while`, `do ... while` or `foreach`) you can use two special commands:

- **`break`**: Stops the loop completely right at that moment and continues on the line after the end of the loop.
- **`continue`**: Skips the rest of the current iteration and jumps immediately to the next iteration of the loop.

For example, if we want to iterate over a sequence but stop as soon as we have printed the first element:

<!-- joss-run: ["Primero: 10"] -->
```joss
var $datos = [10, 20, 30]
foreach ($datos as $valor) {
    print("Primero: " . $valor)
    break
}
```

Output:

```text
Primero: 10
```

When `break` runs at the end of the first iteration, the values `20` and `30` are no longer processed.

Similarly, you can use a ternary or `break`/`continue` to skip or stop a loop when a condition is met:

```joss
while ($intentos < 10) {
    $intentos++
    ($intentos == 5) ? {
        break
    }
}
```

---

## 8. Common mistakes when making decisions and repeating code

| Common mistake | Why it happens | How to fix it |
|---|---|---|
| Writing `if ($x > 0)` or `for (...)` | `if`, `else`, `switch` and `for` do not exist in Joss (`JOSS-PARSE-001`). | Use the ternary operator `($x > 0) ? { ... }`, `match` and `foreach` / `while`. |
| Confusing `=` with `==` | `=` stores a value; `==` compares two values. | Inside a logical question always use `==` or `===` (`$edad == 18`). |
| Forgetting to update the variable in `while` | The condition never changes and the loop repeats endlessly. | Make sure to increment (`$i++`) or modify the variable inside the `{ ... }` block. |
| Using `break` or `continue` outside a loop | They only make sense inside `while`, `do...while` or `foreach` (`JOSS-FLOW-002`). | Place them only inside the body of a loop. |

---

## 9. Chapter practice

### 1. Quick test
In the `match` example from section 4:
1. Change the initial value of `var $estado = "enviado"` to `"entregado"`, and then to `"cancelado"`.
2. Run `joss run main.joss` in each case and observe which arm of `match` is activated.

### 2. Guided exercise: Grade classifier
Write a program in your `main.joss` that evaluates a score following these steps:
1. Declare `int $nota = 85`.
2. Use a ternary to check if `$nota >= 90` and print `"Sobresaliente"`.
3. In the `: { ... }` branch, check if `$nota >= 60` to print `"Aprobado"` or `"Reprobado"`.
4. Test your program with the scores `95`, `70` and `45`.

### 3. Challenge: Multiplication table and sum of even numbers
1. Declare a variable `int $tabla = 7`.
2. Use `foreach (1..10 as $i)` to print the full multiplication table of 7 in the format `"7 x 1 = 7"`, `"7 x 2 = 14"`, etc.
3. Declare a variable `int $sumaPares = 0` before the loop and, inside the loop, use a ternary `(($tabla * $i) % 2 == 0) ? { ... }` to add into `$sumaPares` only the results that are even numbers. Print the sum at the end.

---

## Next step

You now know how to make decisions and iterate over sequences. The natural next step is to learn how to store and organize complete collections of data in memory: dynamic lists (**arrays**), named dictionaries (**maps**) and text processing.

Continue with: [Collections: Arrays, Maps and Unicode Text Manipulation](COLECCIONES.md).

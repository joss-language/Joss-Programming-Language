# Fundamental values, variables and operations

Before: [First steps](PRIMEROS_PASOS.md). Next: [Flow control and decisions](CONTROL_FLUJO.md).
Technical reference: [Type system](SISTEMA_TIPOS.md), [Syntax and operators](SINTAXIS.md).

---

## What will you learn here?

Every computer program exists to process information: calculate an invoice total, store a user's name, verify whether a password is correct or count how many pending messages you have.

In this guide you will learn from scratch:
1. What a **value** is and what a **data type** is.
2. The essential primitive types in Joss: integers, approximate decimals, exact decimals, text and booleans.
3. What a **variable** is, how it works in memory and the difference between **declaring** it and **reassigning** it.
4. The four canonical ways to declare variables in Joss: inferred (`var $x = ...`), explicit (`int $x = ...`), dynamic (`mixed $x = ...`) and constant (`const`).
5. How to operate on numbers, join or interpolate text (`"${...}"`) and format screen output.
6. What common mistakes are made and how to prevent them.

---

## 1. Values and data types: What information do we handle?

A **piece of data** or **value** is any piece of information that a program uses. For example:
- `42` is an integer number.
- `"Ada Lovelace"` is a text.
- `true` is a logical answer.

In a computer, not all data is stored or manipulated in the same way. Adding two numeric quantities (`10 + 5 = 15`) is a mathematical operation; by contrast, joining two names (`"Ana" . " Gómez"`) is a text operation.

The **data type** defines:
1. What kind of information the value represents.
2. Which operations are allowed on it.
3. How much memory it requires and how it is stored internally.

### The fundamental canonical types of Joss

| Data type | What it represents | Examples | When to use it |
|---|---|---|---|
| `int` | Integer number (no decimal part) | `0`, `42`, `-15`, `1000` | For counters, ages, numeric identifiers and indivisible quantities. |
| `float` | Number with decimal point (binary approximation) | `3.1416`, `0.5`, `-12.8` | For scientific calculations, coordinates, graphics or measurements where tiny approximations are acceptable. |
| `decimal` | High-precision base-ten decimal number | `0.10m`, `19.99m`, `100.00M` | **Essential in finance, prices, taxes and balances**, where losing even a single cent to binary approximation is unacceptable. Carries the `m` or `M` suffix. |
| `string` | Sequence of characters (text) | `"Hola"`, `'Joss'`, `"admin@ejemplo.com"` | For names, emails, descriptions, HTML content and messages. |
| `bool` | Logical truth value | `true`, `false` | For making decisions: is the user authenticated?, is it in stock?, is the person an adult? |
| `array` | Ordered list of values | `[1, 2, 3]`, `["pan", "leche"]` | For sequences of elements (you will learn this in [Collections](COLECCIONES.md)). |
| `map` | Associative dictionary (key → value) | `{"id": 1, "nombre": "Ada"}` | For labeled data records (you will learn this in [Collections](COLECCIONES.md)). |
| `null` | Absence of value | `null` (or `nil` for compatibility) | To indicate that a piece of data does not exist yet, is empty or was not found. |

---

## 2. Variables: Named boxes in memory

A computer has millions of memory cells. If we store a number and do not give it a name, we will have no way to find it a millisecond later.

A **variable** is simply a human-readable name that we assign to a space in memory to store a piece of data, read it whenever we need it or replace it with another value.

### The `$` rule in Joss

In Joss, **all variable names must start with the dollar sign (`$`)**:
- `$edad`
- `$nombre_completo`
- `$totalPagar`

> [!TIP]
> **Why does Joss use `$` for variables?**
> The `$` prefix allows both you and the analyzer and compiler to immediately distinguish a variable from a language keyword (`return`, `func`, `class`), a type (`int`, `string`) or a built-in function (`print`). In addition, Joss is case-sensitive: `$edad` and `$Edad` are two different variables.

Let's look at a minimal example:

<!-- joss-run: ["21"] -->
```joss
var $edad = 20
$edad = $edad + 1
print($edad)
```

Output:

```text
21
```

### What happens step by step in this program?

1. `var $edad = 20` (**Declaration**):
   - It is the **first time** `$edad` appears, so we use `var` to create it.
   - The `=` sign is the **assignment operator**: it takes the `20` on the right and stores it in `$edad`.
   - Joss automatically **infers** (deduces) that `$edad` is of type `int`.
2. `$edad = $edad + 1` (**Reassignment**):
   - Since `$edad` **already exists**, we do not write `var` again.
   - The computer first calculates the right-hand side (`20 + 1` = `21`) and stores the new result `21` inside `$edad`.
3. `print($edad)`:
   - Reads the current value of `$edad` (`21`) and displays it in the terminal.

---

## 3. The four ways to declare a variable

In Joss you have complete control over how strict or flexible you want your variable types to be:

### 1. Fixed canonical inference: `var $x = valor`
This is the most convenient and recommended form for everyday use. Joss deduces the type from the initial value and, from that moment on, protects the variable so it cannot change type by accident:

```joss
var $contador = 0       // Infiere int
var $titulo = "Reporte" // Infiere string
var $peso = 72.5        // Infiere float
```

If later on you try to store text inside a variable that started as an integer, the semantic analyzer will stop the program before execution with the code `JOSS-TYPE-001`:

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
var $cantidad = 2
$cantidad = "muchas"
```

### 2. Explicit type declaration: `int $x = valor`
When you want the data type to be 100% visible when reading the code (and mandatorily in function parameters, which we will see later):

```joss
int $puerto = 8080
string $usuario = "admin"
decimal $saldo = 1500.50m
bool $activo = true
```

### 3. Explicit dynamic variable: `mixed $x = valor`
Sometimes you legitimately need the same variable to hold a number first and text later. To allow type changes without error, specify `mixed`:

<!-- joss-run: ["pendiente"] -->
```joss
mixed $resultado = 2
$resultado = "pendiente"
print($resultado)
```

Output:

```text
pendiente
```

> [!NOTE]
> **Compatibility note (`let`)**:
> In older Joss code you may find `let $x = 2` (equivalent to `mixed $x = 2`) or `let int $x = 2` (equivalent to `int $x = 2`). The language still accepts `let` for compatibility and the `joss fix` tool automatically updates it to the modern canonical forms.

### 4. Immutable constants: `const`
A **constant** is a value defined once that **can never change** while the program runs. It is ideal for tax rates, maximum limits or configuration names:

<!-- joss-run: ["3"] -->
```joss
const int $maximo = 3
print($maximo)
```

Output:

```text
3
```

If you try to write `$maximo = 4`, the analyzer will emit the error `JOSS-SYM-006`, preventing the modification.

---

## 4. Text, comments and interpolation

### Text delimiters and escape characters

In Joss you can write text strings using double quotes (`"..."`) or single quotes (`'...'`):

<!-- joss-run: ["Hola, Ada", "Primera línea", "Segunda línea"] -->
```joss
// Este comentario explica el código; no se ejecuta.
var $nombre = 'Ada'
print("Hola, " . $nombre)
/* Un comentario también puede
   ocupar varias líneas. */
print("Primera línea\nSegunda línea")
```

Output:

```text
Hola, Ada
Primera línea
Segunda línea
```

- **Single-line comments**: Start with `//` (or `#`). Everything written to their right is ignored by the computer.
- **Multi-line comments**: Start with `/*` and end with `*/`.
- **Escape characters** (in double or single quotes):
  - `\n`: Inserts a line break.
  - `\t`: Inserts a horizontal tab.
  - `\"` or `\'`: Allows literal quotes inside the text.
  - `\\`: Inserts a backslash.

### Concatenation with the dot operator (`.`)

Other languages use `+` both to add numbers and to join text, causing silent bugs when numbers and text are mixed. In Joss the rule is unambiguous:
- The `+` sign is reserved **exclusively for mathematical addition**.
- The dot `.` is used **exclusively to join text**.

<!-- joss-run: ["1020"] -->
```joss
var $a = "10"
var $b = "20"
print($a . $b)
```

Output:

```text
1020
```

### String interpolation: `${nombre}` and `${$edad * 2}`

Instead of joining several pieces with dots (`"Hola " . $nombre . ", tienes " . $edad . " años"`), inside **double-quoted** strings you can embed variables or calculations directly using `${...}`:

<!-- joss-run: ["Hola Ada, tienes 21 años", "El doble es 42"] -->
```joss
var $nombre = "Ada"
var $edad = 21
print("Hola ${nombre}, tienes ${edad} años")
print("El doble es ${$edad * 2}")
```

Output:

```text
Hola Ada, tienes 21 años
El doble es 42
```

- **When embedding a single variable**: You can write `${nombre}` (or `${$nombre}`). Joss recognizes your variable name automatically.
- **When embedding an operation or calculation**: Write the full expression with the `$` sign on each variable, for example `${$edad * 2}` or `${$precio + $impuesto}`.
- **Literal escape**: If you need to literally display the characters `${` on screen, write `\${`.

### Formatting with positional templates (`printf`)

When you want to format text by indicating the exact position of each string (`%s`) or integer number (`%d`), `printf` is also available:

```joss
var $item = "Teclado"
var $cantidad = 2
printf("Producto: %s | Cantidad: %d\n", $item, $cantidad)
```

---

## 5. Numeric and mathematical operations

<!-- joss-run: ["5", "2.5", "1", "0.3"] -->
```joss
print(2 + 3)
print(5 / 2)
print(5 % 2)
print(0.10m + 0.20m)
```

Output:

```text
5
2.5
1
0.3
```

### Arithmetic operators

| Operator | Operation | Example | Result | Explanation |
|---|---|---|---|---|
| `+` | Addition | `10 + 5` | `15` | Mathematical addition. |
| `-` | Subtraction | `10 - 4` | `6` | Subtraction. |
| `*` | Multiplication | `3 * 4` | `12` | Product. |
| `/` | Division | `5 / 2` | `2.5` | Dividing integers **returns `float`** so the fractional part (`2.5`) is not lost. |
| `%` | Modulo (remainder) | `5 % 2` | `1` | The remainder of an integer division (5 divided by 2 is 2 with a remainder of `1`). Very useful to check if a number is even (`$n % 2 == 0`). |
| `++` | Post-increment | `$i++` | Previous value | Adds `1` to the variable. |
| `--` | Post-decrement | `$i--` | Previous value | Subtracts `1` from the variable. |

### Shorthand assignment operators (`+=`, `-=`, `*=`, `/=`)

When you want to add, subtract or multiply onto the value a variable already holds (such as adding points to a scoreboard), you can shorten `$puntos = $puntos + 5` by writing `$puntos += 5`:

<!-- joss-run: ["15", "12", "24"] -->
```joss
var $puntos = 10
$puntos += 5
print($puntos)

$puntos -= 3
print($puntos)

$puntos *= 2
print($puntos)
```

Output:

```text
15
12
24
```

| Operator | Equivalent to writing | What it does |
|---|---|---|
| `$x += $y` | `$x = $x + $y` | Adds `$y` to the current value of `$x`. |
| `$x -= $y` | `$x = $x - $y` | Subtracts `$y` from the current value of `$x`. |
| `$x *= $y` | `$x = $x * $y` | Multiplies the current value of `$x` by `$y`. |
| `$x /= $y` | `$x = $x / $y` | Divides the current value of `$x` by `$y`. |

### Order of precedence and use of parentheses
Just like in school mathematics, multiplication `*` and division `/` are calculated before addition `+` and subtraction `-`. Use parentheses `(` and `)` whenever you want to specify which operation must be performed first:

<!-- joss-run: ["14", "20"] -->
```joss
print(2 + 3 * 4)
print((2 + 3) * 4)
```

Output:

```text
14
20
```

### Automatic protection in numeric calculations

Joss watches over your mathematical operations to prevent silent failures:
- **Division by zero (`JOSS-ARITH-002`)**: If you try to divide by `0` (`10 / 0`), Joss stops the operation with a clear message.
- **Integer overflow (`JOSS-ARITH-001`)**: If a calculation with `int` exceeds the maximum capacity of an integer number, Joss alerts you instead of returning a bogus negative number.
- **Precision protection (`JOSS-ARITH-003`)**: Joss lets you combine `int` with `decimal` directly because no digits are lost, but requires an explicit conversion function if you try to mix `float` with `decimal` or `int`.

---

## 6. The key difference for money: `float` vs `decimal`

Why are there two types for numbers with a decimal point (`float` and `decimal`)?

Computers internally count in binary (zeros and ones). In binary, fractions like `0.1` or `0.2` have infinite repeating decimals, so the standard `float` type stores a very close approximation:

```joss
print(0.1 + 0.2) // Muestra 0.30000000000000004
```

For moving a character in a video game or measuring temperature, that tiny difference does not matter. However, for **prices, invoices, salaries or accounting**, we need complete accuracy in base ten.

That is why Joss includes the **`decimal`** type out of the box, written by adding an `m` (or `M`) at the end of the number:

<!-- joss-run: ["0.3", "23.2"] -->
```joss
print(0.10m + 0.20m)
var $precio = 11.60m
print($precio * 2)
```

Output:

```text
0.3
23.2
```

> **Practical rule**: Use `int` for whole quantities, `float` for physical or scientific measurements and **`decimal` (with `m`) whenever you represent money**.

---

## 7. Stream output and input (`cout <<` and `cin >>`)

In addition to `print(...)`, Joss provides the output stream `cout <<` and the line-break manipulator `endl`:

<!-- joss-run: ["Hola mundo", "Linea 1", "Linea 2"] -->
```joss
// Con print: agrega salto de línea automático al final
print("Hola mundo")

// Con cout: encadena varias piezas con << y cierra la línea con endl
cout << "Linea " << 1 << endl
cout << "Linea " << 2 << endl
```

Output:

```text
Hola mundo
Linea 1
Linea 2
```

- `print(...)`: Ideal for displaying a complete line directly.
- `cout << ...`: Very useful when you want to display a message without a line break (for example, before asking for input with `cin >>`) or chain several values in a row.
- `cerr << ...`: Sends messages to the terminal's error channel (`stderr`).

---

## 8. Converting data from one type to another (Casting)

If you read a piece of data that comes as text (for example `"42"` or `"19.99"`) and need to perform mathematical calculations with it, use the built-in conversion functions:

<!-- joss-run: ["52", "29.99"] -->
```joss
var $numero = intval("42") + 10
var $precio = decimal("19.99") + 10.00m
print($numero)
print($precio)
```

Output:

```text
52
29.99
```

| Function | Converts the value to | Example | Result |
|---|---|---|---|
| `intval($v)` | `int` (integer) | `intval("42")` | `42` |
| `floatval($v)` | `float` (approximate decimal) | `floatval("3.14")` | `3.14` |
| `decimal($v)` | `decimal` (exact decimal) | `decimal("19.99")` | `19.99m` |
| `strval($v)` | `string` (text) | `strval(100)` | `"100"` |
| `boolval($v)` | `bool` (`true` or `false`) | `boolval(1)` | `true` |

> [!WARNING]
> If you convert text that does not contain numbers with `intval("manzana")`, the result will be `0`. Whenever you want to check beforehand whether a string represents a valid number, use `is_numeric($texto)`.

---

## 9. Common mistakes when working with variables and types

| Common mistake | Why it happens | How to fix it |
|---|---|---|
| Using `+` to join text (`"Total: " + $precio`) | In Joss `+` is exclusively for adding numbers (`JOSS-TYPE-004`). | Use the dot `.` (`"Total: " . $precio`) or interpolation (`"Total: ${precio}"`). |
| Forgetting `var` when creating a variable (`$edad = 20`) | Creating a variable without declaring it emits `JOSS-SYM-001`. | Write `var $edad = 20` (or `int $edad = 20`) the first time you create it. |
| Changing the type of a variable created with `var` | `var $x = 10` fixes the type to `int` (`JOSS-TYPE-001`). | If you need it to change type later, declare it with `mixed $x = 10`. |
| Mixing `int` and `float` without converting (`10 + 2.5`) | Joss prevents implicit conversions with potential loss of precision (`JOSS-ARITH-003`). | Use both of the same type (`10.0 + 2.5`), convert with `floatval(10) + 2.5`, or use `decimal` (`10 + 2.5m`). |
| Trying to change a constant (`const $A = 1; $A = 2`) | Constants are immutable (`JOSS-SYM-006`). | If the value must change during the program, declare it with `var` instead of `const`. |

---

## 10. Guided mini-project: Financial Bill and Tip Calculator

Let's put variables, the exact `decimal` type, mathematical operations and string interpolation into practice in a complete program that you can place inside your `main.joss`:

<!-- joss-run: ["Subtotal: 50", "Propina (15%): 7.5", "Total a pagar: 57.5", "Cada persona paga: 28.75"] -->
```joss
// 1. Datos iniciales con precisión monetaria decimal (sufijo m)
decimal $subtotal = 50.00m
decimal $tasaPropina = 0.15m
int $personas = 2

// 2. Cálculos matemáticos
decimal $propina = $subtotal * $tasaPropina
decimal $total = $subtotal + $propina
decimal $porPersona = $total / $personas

// 3. Mostrar resumen en pantalla
print("Subtotal: ${subtotal}")
print("Propina (15%): ${propina}")
print("Total a pagar: ${total}")
print("Cada persona paga: ${porPersona}")
```

Output:

```text
Subtotal: 50
Propina (15%): 7.5
Total a pagar: 57.5
Cada persona paga: 28.75
```

---

## 11. Chapter practice

### 1. Quick test
In the mini-project from section 10:
1. Change `$subtotal` to `120.00m` and `$personas` to `4`.
2. Run `joss run main.joss` and verify how much each person should pay.

### 2. Guided exercise: Temperature Converter (°C to °F)
1. Declare a variable with the Celsius degrees: `float $celsius = 25.0`.
2. Calculate its Fahrenheit equivalent using parentheses: `float $fahrenheit = ($celsius * 9.0 / 5.0) + 32.0`.
3. Display the result using interpolation: `print("${celsius} °C equivalen a ${fahrenheit} °F")` (it should display `25 °C equivalen a 77 °F`).

### 3. Challenge: Invoice with discount and tax
Create a program that:
1. Declares a constant `const decimal $IVA = 0.16m`.
2. Declares a product price `decimal $precio = 200.00m` and first applies a discount of `20.00m` using the `-=` operator.
3. Calculates the tax (`$precio * $IVA`) and the final total (`$precio + $impuesto`).
4. Displays the discounted price, the tax and the final total on screen.

---

## Next step

Now that you master values, data types and in-memory operations, it is time to teach your programs how to **make decisions** and **repeat tasks automatically**.

Continue with: [Flow control: decisions and repetition structures](CONTROL_FLUJO.md).

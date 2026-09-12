# Fundamental values, variables and operations

Before: [Getting started](PRIMEROS_PASOS.md). After: [Flow control and decisions](CONTROL_FLUJO.md).
Technical reference: [Type system](SISTEMA_TIPOS.md), [Syntax and operators](SINTAXIS.md).

---

## What are you going to learn here?

Every computer program exists to process information: calculate the total of a bill, save a user's name, check if a password is correct, or count how many messages you have pending.

In this guide you will learn from scratch:
1. What is a **value** and what is a **data type**.
2. Joss's essential primitive types: integers, approximate decimals, exact decimals, text, and booleans.
3. What is a **variable**, how it works in memory and how to declare it.
4. The four ways to declare variables in Joss: inferred (`$x = ...`), explicit (`int $x = ...`), dynamic (`mixed $x = ...`) and constant (`const`).
5. How to operate numbers, join texts and format screen output.
6. What common mistakes are made and how to prevent them.

---

## 1. Values ​​and data types: What information do we handle?

A **data** or **value** is any piece of information that a program uses. For example:
- `42` is a number.
- `"Ada Lovelace"` is a text.
- `true` (true) is a logical answer.

On a computer, not all data is stored or manipulated in the same way. Adding two numerical quantities (`10 + 5 = 15`) is a mathematical operation; on the other hand, joining two names (`"Ana" . " Gómez"`) is a text operation.

The **data type** defines:
1. What kind of information represents the value.
2. What operations are allowed on it.
3. How much memory it requires and how it is stored internally.

### Joss's fundamental canonical types

| Data type | What does it represent | Examples | When to use it |
|---|---|---|---|
| `int` | Whole number (without decimal part) | `0`, `42`, `-15`, `1000` | For counters, ages, numerical identifiers and indivisible quantities. |
| `float` | Number with decimal point (binary approximation) | `3.1416`, `0.5`, `-12.8` | For scientific calculations, coordinates, graphs or measurements where small approximations are acceptable. |
| `decimal` | High precision decimal number in base ten | `0.10m`, `19.99m`, `100.00M` | **Essential in finance, prices, taxes and balance sheets**, where losing a single cent due to binary approximation is unacceptable. It is suffixed `m` or `M`. |
| `string` | Character sequence (text) | `"Hola"`, `'Joss'`, `"admin@ejemplo.com"` | For names, emails, descriptions, HTML content and messages. |
| `bool` | Logical truth value | `true` (true), `false` (false) | To make decisions: is it authenticated? Is there stock? Is it of legal age? |
| `array` | Ordered list of values ​​| `[1, 2, 3]`, `["pan", "leche"]` | For sequences of elements (detailed at [Collections](COLECCIONES.md)). |
| `map` | Associative dictionary (key → value) | `{"id": 1, "nombre": "Ada"}` | For data records with properties (detailed in [Collections](COLECCIONES.md)). |
| `null` / `nil` | Absolute absence of value | `null` or `nil` | To indicate that a piece of data does not yet exist, is empty, or has not been found. |

---

## 2. Variables: Named boxes in memory

A computer has millions of memory cells. If we save a number and don't give it a name, we will have no way of finding it a millisecond later.

A **variable** is simply a human name that we assign to a memory space to store data, read it when we need it or change it to another value.

### The `$` rule in Joss

In Joss, **all variable names are required to begin with a dollar sign (`$`)**:
- `$edad`
- `$nombre_completo`
- `$totalPagar`

> [!TIP]
> **Why does Joss use `$` for variables?**
> The `$` prefix allows you, the parser, and the compiler to instantly distinguish a variable from a language keyword (`return`, `func`, `class`), from a type (`int`, `string`) or from a native function (`print`). Additionally, Joss is case sensitive: `$edad` and `$Edad` are two different variables.

Let's look at a minimal example:

<!-- joss-run: ["21"] -->
```joss
$edad = 20
$edad = $edad + 1
print($edad)
```

### What happens step by step in this program?

1. `$edad = 20`:
   - The sign `=` is the **assignment operator**. It evaluates what is to its right (`20`) and deposits it in the variable `$edad`.
   - As this is the first time that `$edad` appears in the program, Joss automatically **infers** that `$edad` is of type `int`.
2. `$edad = $edad + 1`:
   - The computer evaluates the right side first: it looks for the current value of `$edad` (which is `20`), adds `1` to it, resulting in `21`.
   - Then, the `=` operator saves that new value `21` to `$edad`, overwriting the previous `20`.
3. `print($edad)`:
   - Reads the current value from `$edad` and displays it in the console.

---

## 3. The four ways to declare a variable

In Joss you have complete control over how strict or flexible you want the typing of your variables to be:

### 1. Fixed automatic inference: `$x = valor` (or `var $x = valor`)
It is the fastest and recommended way for everyday life. Joss deduces the type in the first assignment and from that moment on the variable is protected:

```joss
$contador = 0       // Infiere int
$titulo = "Reporte" // Infiere string
var $peso = 72.5    // 'var' solicita inferencia explícita; también fija float
```

If you later try to put text inside an integer, the semantic analyzer will stop the program with the code `JOSS-TYPE-001`:

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
$cantidad = 2
$cantidad = "muchas"
```

### 2. Explicitly typed declaration: `int $x = valor`
When you want the data contract to be 100% visible to anyone reading the code, or in function parameters:

```joss
int $puerto = 8080
string $usuario = "admin"
decimal $saldo = 1500.50m
bool $activo = true
```

### 3. Dynamic variable: `mixed $x = valor` (or `let $x = valor`)
Sometimes you are creating an algorithm that legitimately needs to transform a number into a text or a pending state. To allow type changes without error, indicate this explicitly with `mixed`:

<!-- joss-run: ["pendiente"] -->
```joss
mixed $resultado = 2
$resultado = "pendiente"
print($resultado)
```

> [!NOTE]
> `let $resultado = 2` is exactly equivalent to `mixed $resultado = 2`. In Joss, `let` typeless **does not mean constant**, but explicit dynamism.

### 4. Immutable constants: `const`
A **constant** is a value that is defined only once and can never be reassigned during the life of the program. It serves to shield business rules and configuration parameters:

<!-- joss-run: ["3"] -->
```joss
const int $maximo = 3
print($maximo)
```

If you try to type `$maximo = 4`, the parser will issue an error `JOSS-SYM-006` indicating that a constant cannot be reassigned.

---

## 4. Text, comments and formatted output

### Text delimiters and escape characters

In Joss you can write text strings using double quotes (`"..."`) or single quotes (`'...'`):

<!-- joss-run: ["Hola, Ada", "Primera línea", "Segunda línea"] -->
```joss
// Este comentario explica el código; no se ejecuta.
$nombre = 'Ada'
print("Hola, " . $nombre)
/* Un comentario también puede
   ocupar varias líneas. */
print("Primera línea\nSegunda línea")
```

- **One line comments**: They start with `//` (or `#`). Everything you type to the right is ignored by the computer.
- **Multiline comments**: They start with `/*` and end with `*/`.
- **Escape characters**:
  - `\n`: Insert a line break.
  - `\t`: Inserts a horizontal tab stop.
  - `\"` or `\'`: Allows you to include literal quotes within the text.
  - `\\`: Insert a backslash.

### Concatenation with the dot operator (`.`)

Many languages ​​use `+` to join text, which causes serious errors when numbers and text are accidentally mixed together. In Joss:
- The operator `+` is reserved **exclusively for mathematical addition**.
- The dot operator `.` is used **exclusively to concatenate text**.

```joss
$a = "10"
$b = "20"
print($a . $b) // Imprime "1020" (unión de textos)
```

### Modern string interpolation: `${variable}` or `${expresión}`

Instead of chaining multiple fragments together with the dot operator (`"Hola " . $nombre . " tienes " . $edad . " años"`), Joss allows **embedding variables and calculations directly within double-quoted texts** using the `${...}` syntax (identical to modern languages ​​like Flutter/Dart or Kotlin):

<!-- joss-run: ["Hola Ada, tienes 21 años", "El doble es 42"] -->
```joss
$nombre = "Ada"
$edad = 21
print("Hola ${nombre}, tienes ${edad} años")
print("El doble es ${$edad * 2}")
```

- **Variables**: Write `${variable}` inside the double quotes to automatically inject its value.
- **Calculations and expressions**: You can place complete mathematical or logical operations between the braces: `${$precio * $cantidad}`.
- **Literal escaping**: If you need the text to literally display `${`, type a backslash before it: `\${`.

### Advanced formatting with `printf`

When you need to put together messages with numerical variables and texts in exact positions without chaining many points, use `printf`:

```joss
$item = "Teclado"
$cantidad = 2
printf("Producto: %s | Cantidad: %d\n", $item, $cantidad)
```
- `%s` is replaced by a string (`string`).
- `%d` is replaced by an integer (`int`).

---

## 5. Numerical and mathematical operations

<!-- joss-run: ["5", "2.5", "1", "0.3"] -->
```joss
print(2 + 3)
print(5 / 2)
print(5 % 2)
print(0.10m + 0.20m)
```

### Arithmetic operators

| Operator | Operation | Example | Result | Explanation |
|---|---|---|---|---|
| `+` | Sum | `10 + 5` | `15` | Mathematical addition. |
| `-` | Subtraction | `10 - 4` | `6` | Subtraction. |
| `*` | Multiplication | `3 * 4` | `12` | Product. |
| `/` | Division | `5 / 2` | `2.5` | In Joss, dividing integers **returns `float`**, avoiding accidental loss of decimals. |
| `%` | Module (rest) | `5 % 2` | `1` | The remainder of the integer division (5 divided by 2 gives 2 with remainder 1). Very useful to know if a number is even (`$n % 2 == 0`). |
| `++` | Post-increment | `$i++` | Current value | Increase the variable by 1 and return its previous value. |
| `--` | Post-decrement | `$i--` | Current value | Decrements the variable by 1 and returns its previous value. |

### Composite assignment operators (`+=`, `-=`, `*=`, `??=`)

When you want to modify the value that a variable already has (for example, adding points in a game or deducting lives), you do not need to repeat the name of the variable (`$puntos = $puntos + 5`). You can use quick assignment operators:

<!-- joss-run: ["15", "12", "24", "Invitado"] -->
```joss
$puntos = 10
$puntos += 5
print($puntos)

$puntos -= 3
print($puntos)

$puntos *= 2
print($puntos)

$nombre = null
$nombre = $nombre ?? "Invitado"
print($nombre)
```

| Operator | Equivalence | Description |
|---|---|---|
| `$x += $y` | `$x = $x + $y` | Adds `$y` to the current value of `$x`. |
| `$x -= $y` | `$x = $x - $y` | Subtract `$y` from the current value of `$x`. |
| `$x *= $y` | `$x = $x * $y` | Multiplies the current value of `$x` by `$y`. |
| `$x ??= $y` | `$x = $x ?? $y` | Assigns `$y` only if `$x` is currently `null`. |

### Mathematical priority (precedence)
As in algebra, multiplication and modulus are calculated before addition and subtraction. Use parentheses `(` `)` to clearly define what should be resolved first:

```joss
print(2 + 3 * 4)   // Da 14 (3 * 4 = 12, luego + 2)
print((2 + 3) * 4) // Da 20 (2 + 3 = 5, luego * 4)
```

### Arithmetic safety against overflow (Overflow)

On conventional 64-bit systems, if you add 1 to the largest possible integer, the number becomes a huge negative value without warning you. Joss prevents this at its core:
- If an integer operation exceeds the 64-bit signed range (−9,223,372,036,854,775,808 to 9,223,372,036,854,775,807), Joss stops execution immediately with the structured error `JOSS-ARITH-001` (Arithmetic Overflow).
- If you try to divide by zero (`$x / 0`), Joss stops it with `JOSS-ARITH-002` (Division by Zero).

---

## 6. The critical difference: `float` vs `decimal`

Why do `float` and `decimal` exist?

Computers represent floating point numbers (`float`) using powers of two in binary (IEEE-754 standard). There are decimal fractions like `0.1` or `0.2` that do not have exact finite binary representation (similar to trying to write one-third `1/3` in decimal like `0.33333...`).

Therefore, in almost all traditional languages:
```joss
print(0.1 + 0.2) // Imprime aproximadamente 0.30000000000000004
```

If you're calculating the trajectory of a projectile in a video game, that millionth of a difference doesn't matter. But if you're calculating bank interest on a million transactions, that difference is an accounting disaster.

**Joss' solution is the type `decimal`:**
By adding the suffix `m` or `M`:
```joss
print(0.10m + 0.20m) // Imprime exactamente 0.3
```
Calculations are performed in exact base ten using fixed point arithmetic.

---

## 7. Console input and output (`print`, `cout`, `cin`)

### Console output: `print` and `cout`

Joss offers two modern and complementary ways to display information in the terminal:

1. **`print(...)`**: The standard function for issuing messages. Each argument is printed to the screen by adding an automatic line break at the end.
2. **`cout << ...`**: The standard output stream (C++ style). It allows chaining expressions with the operator `<<`, does not add automatic line breaks (you can use the manipulator `endl` or `"\n"`) and can also be called directly as a function `cout(...)`.
3. **`cerr << ...`**: The standard errors output stream (`stderr`).
4. **`endl`**: Native constant that represents the line break (`"\n"`).

<!-- joss-run: ["Hola mundo", "Linea 1", "Linea 2"] -->
```joss
// Con print: genera salto de línea automático
print("Hola mundo")

// Con cout: encadenamiento con << y manipulador endl
cout << "Linea " << 1 << endl
cout << "Linea " << 2 << endl
```

### Input from the console with `cin >>`

To create interactive programs where a person writes data to the console during execution, Joss provides the input stream `cin` with the operator `>>`:

```joss
cout << "¿Cómo te llamas? "
string $nombre = ""
cin >> $nombre

cout << "¿Cuántos años tienes? "
int $edad = 0
cin >> $edad

print("Hola ${nombre}, el próximo año tendrás ${$edad + 1} años.")
```

**Features of `cin` and `cout`:**
1. **Automatic conversion in `cin`**: If the target variable is numeric (`int` or `float`), `cin` converts the entered text directly to the expected type.
2. **Full text reading**: If the variable is `string`, captures the entire line typed by the user.
3. **Multiple chaining**: You can chain both inputs and outputs: `cin >> $a >> $b` or `cout << "A: " << $a << " B: " << $b << endl`.

### Coalesce null assignment (`??=`)

If a variable has value `null` or has not yet been initialized, you can assign it a default value only if it is empty using `??=`:

<!-- joss-run: ["oscuro", "oscuro"] -->
```joss
$tema = null
$tema ??= "oscuro"
print($tema)
$tema ??= "claro" // No sobreescribe porque ya tiene "oscuro"
print($tema)
```

---

## 8. Type Conversion Functions (Casting)

If you receive data as text (for example `"25"`) and you need to add a quantity to it, you must explicitly convert it to a number:

| Function | Convert to | Entry example | Result |
|---|---|---|---|
| `intval($v)` | `int` | `intval("42")` | `42` |
| `floatval($v)` | `float` | `floatval("3.14")` | `3.14` |
| `decimal($v)` | `decimal` | `decimal("19.99")` | `19.99m` |
| `strval($v)` | `string` | `strval(100)` | `"100"` |
| `boolval($v)` | `bool` | `boolval(1)` | `true` |

> [!WARNING]
> Converting is not the same as validating. If you try to run `intval("manzana")`, the function will return `0` without failing. If you need to first check whether a text contains valid numbers, use `is_numeric($texto)`.

---

## 9. Common mistakes when starting out

| Common mistake | Code / Cause | How to fix it |
|---|---|---|
| Use `+` to join texts | `print("Total: " + $precio)` | Always use the period for text: `print("Total: " . $precio)`. |
| Forget the `$` in a variable | `edad = 20` | All variables must have `$`: `$edad = 20`. |
| Change type of an inferred variable | `$x = 10; $x = "hola"` (`JOSS-TYPE-001`) | If you need it to change type, declare it as `mixed $x = 10`. |
| Forget the suffix `m` in financial amounts | `0.10 + 0.20` | Use `0.10m + 0.20m` to ensure monetary accuracy. |
| Reassign a constant | `const $A = 1; $A = 2` (`JOSS-SYM-006`) | If the value must change, do not use `const`; use `$A = 1`. |

---

## 10. Guided mini-project: Bill and Tip Calculator

To put into practice everything you learned in this guide (variables, types, mathematical operations and concatenated text), here is a complete program that calculates the tip and divides the total between friends:

<!-- joss-run: ["Subtotal: 50", "Propina: 7.5", "Total: 57.5", "Por persona: 28.75"] -->
```joss
// 1. Datos iniciales
$subtotal = 50.0
$propina = $subtotal * 0.15
$total = $subtotal + $propina
$porPersona = $total / 2

// 2. Mostrar resumen en pantalla
print("Subtotal: " . $subtotal)
print("Propina: " . $propina)
print("Total: " . $total)
print("Por persona: " . $porPersona)
```

### Step by step explanation:
1. `$subtotal = 50.0`: We save the account amount as a decimal number (`float`).
2. `$propina = $subtotal * 0.15`: We calculate 15% by multiplying by `0.15`.
3. `$total = $subtotal + $propina`: We add the cost of consumption and the tip.
4. `$porPersona = $total / 2`: We divide the bill equally between two people.
5. Calls to `print(...)`: Join the long text with the numeric value using the dot concatenation operator `.`.

---

## 11. Practical exercises for beginners

1. **Temperature converter**:
   - Declare a variable `$celsius = 25.0`.
   - Apply the formula to convert to Fahrenheit: `$fahrenheit = ($celsius * 9 / 5) + 32`.
   - Print the concatenated result: `print($celsius . " °C equivalen a " . $fahrenheit . " °F")`.
2. **Quick Assign Game Scoreboard**:
   - Starts with `$puntos = 0`.
   - Add 100 points with `+=`, subtract 20 with `-=` for a penalty and double the score with `*= 2` for a bonus.
   - Shows the final score on the console.

---

## Next step

Now that you've mastered data, types, and in-memory math, it's time to give your programs decision-making and repeatability: how to execute a block only if a condition is met, and how to create loops.

Continue with: [Flow control and repetition structures](CONTROL_FLUJO.md).

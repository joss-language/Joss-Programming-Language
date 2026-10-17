# Error Handling, Exceptions, and Diagnostics

Previous: [Functions, closures, and references](FUNCIONES.md). Next: [Classes, objects, and inheritance](CLASES.md).
Technical reference: [Structured diagnostics](DIAGNOSTICOS.md), [Semantic analyzer](ANALIZADOR.md).

---

## What will you learn here?

In an ideal world, programs would always receive the right data, hard drives would never fill up, and network connections would never fail. In the real world, errors are inevitable:
- A user types letters where a card number was expected.
- A configuration file that the program needs was accidentally deleted.
- The database server restarts in the middle of a transaction.

A professional program is not one that never encounters problems, but one that **knows how to anticipate them, contain them, and recover without crashing**.

In this guide you will learn:
1. The three time phases where errors are detected: syntax, static analysis, and execution time (*runtime*).
2. How to read and interpret a structured Joss diagnostic message.
3. The exception control mechanism with **`try`**, **`catch`**, and **`throw`**.
4. What information the caught error variable (the `$error` map) contains.
5. Why `return`, `break`, and `continue` statements inside a `try` block are not interfered with by `catch`.
6. When to handle errors using exceptions vs when to check return values (`null`).

---

## 1. The Three Phases of an Error

To solve a problem quickly, the first step is to identify in which phase of the program's lifecycle it occurred:

```text
Código fuente (.joss)
         │
         ▼
[ 1. PARSER ] ────────► ¿Error de Sintaxis?
         │              (Falta cerrar paréntesis, comilla o llave)
         ▼
[ 2. ANALYZER ] ──────► ¿Error Semántico o de Tipo?
         │              (Variable no definida, tipo incompatible, retorno ausente)
         ▼
[ 3. RUNTIME ] ───────► ¿Excepción en Ejecución?
                        (Archivo no encontrado, división por cero, throw explícito)
```

| Phase | When it occurs | Example | How to resolve it |
|---|---|---|---|
| **Syntax** | During the initial reading of the text | `print("Hola)` (unclosed quote) | Fix the punctuation pointed out by the parser. |
| **Analysis** | During prior static verification | `int $x = "texto"` (`JOSS-TYPE-001`) | Fix the type or name mismatch before running. |
| **Execution (Runtime)** | While the program is running in memory | `$n / 0` or a network failure | Intercept and handle with `try / catch` blocks. |

---

## 2. Anatomy of a Diagnostic Message

When Joss detects a problem before running, it prints a structured diagnostic. For example:

```text
error[JOSS-TYPE-001] app.joss:15:5: Asignación incompatible: se esperaba 'int', se obtuvo 'string'.
  --> app.joss:15:5
   |
15 |     $edad = "veinte"
   |     ^^^^^
Sugerencia: Modifica el valor asignado o declara la variable como 'mixed'.
```

Each part of the message has a purpose:
1. **Severity (`error` or `warning`)**:
   - `error`: Critical problem. Joss will refuse to run the program to avoid unpredictable behavior.
   - `warning`: Informational notice (such as a declared variable that was never used). The program can run, but cleaning it up is recommended.
2. **Stable code (`JOSS-TYPE-001`)**: A unique identifier that lets you look up the exact cause and examples in the [Diagnostics reference](DIAGNOSTICOS.md).
3. **Location (`app.joss:15:5`)**: The exact file, line number (`15`), and column (`5`) where the mismatch was detected.
4. **Explanation and Suggestion**: A natural-language description of what rule was broken and how to fix it.

---

## 3. Exception Handling: `try`, `catch`, and `throw`

An **exception** is an alarm signal that interrupts the normal flow of the program when an unexpected situation occurs that the current code cannot resolve on its own.

- **`throw`**: Raises the alarm (the exception).
- **`try`**: Delimits a protected zone of code where we suspect something might fail.
- **`catch ($error)`**: Is the emergency crew. If something blows up inside the `try` block, execution jumps immediately to the `catch` block to mitigate the problem:

<!-- joss-run: ["No se pudo continuar: faltan datos"] -->
```joss
try {
    throw "faltan datos"
} catch ($error) {
    print("No se pudo continuar: " . $error)
}
```

Terminal output:

```text
No se pudo continuar: faltan datos
```

### What happens in this example?
1. The computer enters the `try` block.
2. It executes `throw "faltan datos"`. At that instant, normal execution stops.
3. Control jumps directly to the `catch` block.
4. The message `"faltan datos"` is placed into the `$error` variable.
5. The `catch` block prints the notice.
6. The program does not crash; it continues executing the lines after `catch`.

---

## 4. Advanced Inspection of the Error Object in `catch`

When the Joss execution engine itself generates an internal error (called `JossError`, such as an arithmetic overflow or an index out of range), the `$e` variable in `catch` receives at runtime an **associative map** with detailed technical information:

```joss
try {
    var $arr = [1, 2]
    print($arr[99]) // Índice fuera de rango
} catch ($e) {
    print("Mensaje: " . $e["message"])
    print("Tipo: " . $e["type"])
    print("Archivo: " . $e["file"])
    print("Línea: " . $e["line"])
}
```

### Available fields in the internal error map:
- `$e["message"]`: The descriptive message of the failure.
- `$e["type"]`: The internal category of the error (for example, `"IndexOutOfRange"` or `"ArithmeticFault"`).
- `$e["file"]`: The path to the source file where the failure originated.
- `$e["line"]`: The exact line number.
- `$e["error"]`: The full textual representation of the error.

### Structured exceptions with maps or class instances

You can also throw an associative map with your own context fields, or an instance of a custom class (we will see how to build classes in detail in the next chapter, [Classes, objects, and inheritance](CLASES.md)):

<!-- joss-run: ["Campo: email", "Motivo: Formato inválido"] -->
```joss
public class ErrorValidacion {
    Init(public string $campo, public string $motivo) {}
}

try {
    throw new ErrorValidacion("email", "Formato inválido")
} catch ($e) {
    print("Campo: " . $e->campo)
    print("Motivo: " . $e->motivo)
}
```

Terminal output:

```text
Campo: email
Motivo: Formato inválido
```

---

## 5. Flow Guarantee: `return` and Loops Inside `try`

In many languages, putting control statements inside a protected block can cause unexpected behavior. In Joss:

- If you execute `return $valor` inside a `try` block, the function will return immediately and the `catch` block **will not intervene**.
- If you execute `break` or `continue` inside a `try` that is inside a loop, the loop will break or advance normally without `catch` confusing the jump with an exception.

Joss internally distinguishes flow-control signals from real user errors, guaranteeing that your guard clauses work 100% predictably.

---

## 6. Exceptions vs Checking Return Values

Not every problem should be handled with `try / catch`. In the Joss standard library, many operations return a special value (`null` or `false`) when a query simply finds no results.

For example, reading a file that does not exist:

<!-- joss-run: ["No se pudo leer el archivo"] -->
```joss
var $contenido = file_get_contents("archivo-que-no-existe.txt")
($contenido == null) ? {
    print("No se pudo leer el archivo")
} : {
    print($contenido)
}
```

Terminal output:

```text
No se pudo leer el archivo
```

- `file_get_contents(...)`: Returns the text of the file if it exists, or `null` if it could not be read. It does not throw a destructive exception; it lets you check the result with a simple ternary.
- `file_put_contents(...)`: Returns `true` if the file was written to disk or `false` if there was a permission failure.

### When should you use each approach?
- **Use return checking (`$res == null`)**: When the absence of data is a normal possibility in the application (a user searching for a product that does not exist in the catalog).
- **Use `throw` and `try / catch`**: When the failure represents a critical or abnormal condition from which local code cannot recover (connection to the payment server was lost mid-charge, or essential environment variables are missing at startup).

---

## 7. Anti-patterns: What You Should NOT Do

> [!CAUTION]
> **Never silence errors with an empty `catch`**:

```joss
// MALO: Esconde bugs catastróficos
try {
    iniciarBaseDeDatos()
} catch ($e) {}
```

> If the database failed to start, the program will keep running blindly and fail incomprehensibly ten lines later. At a minimum, log the error to the console with `print($e)` or abort execution.

---

## 8. Practical Exercises

### Level 1 — Quick check
Write a `try / catch ($err)` block where inside the `try` you throw the exception `throw "Conexión rechazada"`. In the `catch`, print `"Alerta: " . $err`.

### Level 2 — Guided exercise
1. Write a function `public func registrarEdad(int $edad): string`.
2. If `$edad < 0`, throw an exception: `throw "La edad no puede ser negativa"`.
3. If `$edad < 18`, throw: `throw "Debe ser mayor de edad para registrarse"`.
4. If valid, return `"Registro exitoso"`.
5. Call the function inside a `try / catch` block, testing with `-5`, `15`, and `25`, and print the result or the caught error message.

### Level 3 — Challenge
Create a function `public func dividirSeguro(int $numerador, int $denominador): int` that:
1. Uses a `try / catch ($e)` block to perform the operation `$numerador / $denominador`.
2. If an arithmetic error occurs (such as dividing by `0`), catches the map `$e`, prints `"Fallo detectado: " . $e["type"]`, and returns `0`.
3. Test calling it with `dividirSeguro(20, 4)` and `dividirSeguro(20, 0)`.

---

## Next step

Now that you know how to organize functions, manipulate collections, and protect your program against failures, you are ready to package data and behaviors together using Object-Oriented Programming.

Continue with: [Classes, objects, methods, and inheritance](CLASES.md).

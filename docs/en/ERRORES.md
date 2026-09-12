# Error, exception and diagnostic handling

Before: [Classes, objects and inheritance](CLASES.md). After: [Concurrency, asynchrony and channels](CONCURRENCIA.md).
Technical reference: [Structured diagnostics](DIAGNOSTICOS.md), [Semantic analyzer](ANALIZADOR.md).

---

## What are you going to learn here?

In an ideal world, programs would always receive the correct data, hard drives would never fill up, and network connections would never fail. In the real world, mistakes are inevitable:
- A user types letters where a card number is expected.
- A configuration file that the program needs was deleted by accident.
- The database server is restarted in the middle of a transaction.

A professional program is not one that never encounters problems, but one that **knows how to anticipate them, contain them and recover without collapsing**.

In this guide you will learn:
1. The three time phases where errors are detected: syntax, static analysis and runtime.
2. How to read and interpret a Joss structured diagnostic message.
3. The exception handling mechanism with **`try`**, **`catch`** and **`throw`**.
4. What information is contained in the captured error variable (the map `$error`).
5. Why the instructions `return`, `break` and `continue` within a block `try` are not interfered with by `catch`.
6. When to handle errors using exceptions vs when to check codes or return values ​​(`null`).

---

## 1. The three phases of an error

To fix a problem quickly, the first thing is to identify in which phase of the program's life cycle it occurred:

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

| Phase | When does it occur | Example | How is it resolved |
|---|---|---|---|
| **Syntax** | During the initial reading of the text | `print("Hola)` (unclosed quote) | Corrects the punctuation indicated by the parser. |
| **Analysis** | During static pre-check | `int $x = "texto"` (`JOSS-TYPE-001`) | Correct the type or name mismatch before running. |
| **Runtime** | While the program is running in memory | `$n / 0` or a network failure | It is intercepted and managed with `try / catch` blocks. |

---

## 2. Anatomy of a diagnostic message

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
   - `error`: Critical issue. Joss will refuse to run the program to avoid unpredictable behavior.
   - `warning`: Informational notice (as a declared variable that was never used). The program can run, but it is recommended to clean it.
2. **Stable code (`JOSS-TYPE-001`)**: A unique identifier that allows you to search for the exact cause and examples in the [Diagnostic Reference](DIAGNOSTICOS.md).
3. **Location (`app.joss:15:5`)**: The exact file, line number (`15`) and column (`5`) where the discrepancy was detected.
4. **Explanation and Suggestion**: A natural language description of which rule was broken and how to fix it.

---

## 3. Exception handling: `try`, `catch` and `throw`

An **exception** is an alarm signal that interrupts the normal flow of the program when an unforeseen situation occurs that the current code cannot resolve itself.

- **`throw`**: Raise the alarm (the exception).
- **`try`**: Delimits a protected area of ​​code where we suspect that something could fail.
- **`catch ($error)`**: It is the emergency brigade. If something explodes inside the `try` block, execution immediately jumps to the `catch` block to mitigate the problem:

<!-- joss-run: ["No se pudo continuar: faltan datos"] -->
```joss
try {
    throw "faltan datos"
} catch ($error) {
    print("No se pudo continuar: " . $error)
}
```

### What happens in this example?
1. The computer enters the block `try`.
2. Run `throw "faltan datos"`. At that moment, normal execution stops.
3. The control jumps directly to the `catch` block.
4. The message `"faltan datos"` is stored in the variable `$error`.
5. The `catch` block prints the notice.
6. The program does not crash; continues executing the lines after `catch`.

---

## 4. Advanced error object inspection at `catch`

When the Joss runtime itself generates an internal error (called `JossError`, such as an arithmetic overflow or index out of range), the variable `$error` in `catch` becomes an **associative map** with detailed technical information:

```joss
try {
    $arr = [1, 2]
    print($arr[99]) // Índice fuera de rango
} catch ($e) {
    print("Mensaje: " . $e["message"])
    print("Archivo: " . $e["file"])
    print("Línea: " . $e["line"])
}
```

### Available fields in the internal error map:
- `$e["message"]`: The descriptive message of the failure.
- `$e["type"]`: The internal category of the error (for example, `"IndexOutOfRange"` or `"ArithmeticFault"`).
- `$e["file"]`: The path to the source file where the failure originated.
- `$e["line"]`: The exact line number.
- `$e["error"]`: The complete textual representation of the error.

### Custom exceptions with class instances

If your application returns an instance of a class (`throw new MiExcepcion(...)`), the `catch ($e)` block preserves the **live instance of the object**, allowing direct access to its specialized methods and properties:

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

---

## 5. Flow Guarantee: `return` and loops inside `try`

In many languages, putting control statements inside a protected block can cause unexpected behavior. In Joss:

- If you execute `return $valor` inside a block `try`, the function will return immediately and the block `catch` **will not intervene**.
- If you run `break` or `continue` inside a `try` that is in a loop, the loop will break or proceed normally without `catch` confusing the jump with an exception.

Joss internally distinguishes flow control signals from actual user errors, ensuring your guard clauses work 100% predictably.

---

## 6. Exceptions vs Return Checking

Not all issues should be handled with `try / catch`. In the Joss standard library, many operations return a special value (`null` or `false`) when a query simply finds no results.

For example, reading a file that does not exist:

<!-- joss-run: ["No se pudo leer el archivo"] -->
```joss
$contenido = file_get_contents("archivo-que-no-existe.txt")
($contenido == null) ? {
    print("No se pudo leer el archivo")
} : {
    print($contenido)
}
```

- `file_get_contents(...)`: Returns the text of the file if it exists, or `null` if it could not be read. It doesn't throw a destructive exception; allows you to check the result with a simple ternary.
- `file_put_contents(...)`: Returns `true` if the file was written to disk or `false` if there was a permissions failure.

### When to use each approach?
- **Use return verification (`$res == null`)**: When the absence of the data is a normal possibility of the application (a user searching for a product that does not exist in the catalog).
- **Use `throw` and `try / catch`**: When the failure represents a critical or abnormal condition from which the local code cannot recover (the connection to the payment server was lost in the middle of the payment, or essential environment variables to boot are missing).

---

## 7. Antipatterns: What NOT to do

> [!CAUTION]
> **Never silence errors with an empty `catch`**:

```joss
// MALO: Esconde bugs catastróficos
try {
    iniciarBaseDeDatos()
} catch ($e) {}
```

> If the database failed to start, the program will continue running blindly and fail incomprehensibly ten lines later. At a minimum, log the error in the console with `print($e["error"])` or cancel the execution.

---

## 8. Practical exercise

1. **User validator with exceptions**:
   - Write a function `public func registrarEdad(int $edad): string`.
   - If `$edad < 0`, throw an exception: `throw "La edad no puede ser negativa"`.
   - If `$edad < 18`, launch: `throw "Debe ser mayor de edad para registrarse"`.
   - If valid, returns `"Registro exitoso"`.
   - Call the function inside a `try / catch` block, testing with `-5`, `15` and `25`, and print the result or the captured error message.

---

## Next step

Now that your code knows how to defend against failures and recover gracefully, it's time to learn one of the most powerful features of Joss: how to perform tasks in parallel, delegate operations to the background, and communicate processes without blocking.

Continue with: [Concurrency, Asynchrony, Future and Channels](CONCURRENCIA.md).

# Practical Project: Console Application with JSON Persistence

Previous: [Concurrency and channels](CONCURRENCIA.md). Next: [Complete web project](PROYECTO_WEB.md).
Technical reference: [Command line (CLI)](CLI.md), [Project structure](ESTRUCTURA_PROYECTO.md).

---

## What Will You Build Here?

A **console application** (CLI) is a program that runs directly in the terminal without a graphical interface. It is the type of software used in automations, *batch processing*, server maintenance scripts, and developer utilities.

In this guided tutorial we will build a **purchasing and inventory manager**:
1. Create the official project structure with `joss new console`.
2. Structure a list of items with quantities and exact decimal prices.
3. Save the data to a physical file on disk (`compras.json`) in structured JSON format.
4. Read the file from disk, validate its syntactic integrity (`json_verify`), and reconstruct the data structures in memory (`json_decode`).
5. Process and total the amounts with financial decimal precision using modular functions.
6. Compile the project into a standalone native executable with `joss build`.

---

## 1. Prepare the Working Environment with `joss new console`

Open your terminal and generate an official console project with the Joss CLI:

```bash
joss new console proyecto-compras
cd proyecto-compras
```

Joss will have created the base structure of the project (including `main.joss`, `env.joss`, `joss.yaml`, and the `app/` folder). Open `main.joss` in your editor to write the logic of our application.

---

## 2. The Complete Program Code

Replace the contents of `main.joss` with the following complete program:

<!-- joss-run: ["Articulos: 2", "Total: 42.5", "Archivo guardado"] -->
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}

var $compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]

var $guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }

var $texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }

var $leidas = json_decode($texto)
print("Articulos: " . count($leidas))
print("Total: " . totalizar($leidas))
print("Archivo guardado")
```

---

## 3. Step-by-Step Architecture Explanation

Let's analyze how the different language subsystems interact in this program:

### 1. The Calculation Function (`totalizar`)
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}
```
- Receives an `array` of purchases and promises to return a value of type `decimal`.
- Initializes an exact accumulator: `decimal $total = 0m`.
- Iterates through each element with `foreach ($compras as $compra)`.
- Extracts `"precio"` and `"cantidad"` using map keys. Notice how we convert explicitly:
  - `decimal($compra["precio"])`: Converts the numeric string to a fixed-point decimal.
  - `intval($compra["cantidad"])`: Converts the quantity to a 64-bit integer.
- Multiplies both values and adds them to `$total`.

### 2. In-Memory Data Structure
```joss
var $compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]
```
- We define an array whose elements are associative maps (`{"clave": valor}`).
- Storing prices as text (`"12.50"`) inside JSON is good accounting practice: it prevents standard JSON decoders from introducing binary inaccuracies when reading floating-point numbers.

### 3. Disk Persistence with JSON
```joss
var $guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }
```
- `json_encode($compras)`: Transforms the in-memory Joss structure into a standard JSON text string.
- `file_put_contents("compras.json", ...)`: Writes that text into the physical file on the hard drive. Returns `true` if it succeeded or `false` if it failed due to permissions or lack of space.
- The ternary expression acts as a safeguard: if `$guardado` is false, it throws an error with `throw`.

### 4. Reading and Security Validation
```joss
var $texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }
```
- `file_get_contents(...)`: Retrieves the file bytes into a text string. If the file does not exist, it returns `null`.
- `json_verify($texto)`: Native Joss function that checks whether a string complies with valid JSON specification without parsing the entire tree into memory. If the file is corrupted or was edited improperly by a user, it detects it immediately.
- `json_decode($texto)`: Reconstructs the JSON data into native Joss arrays and maps ready to be processed.

---

## 4. Analysis, Execution, and Native Compilation

First, let's run static analysis to ensure there are no inconsistencies:

```bash
joss analyze main.joss
```

If everything is correct, run the program with the interpreter:

```bash
joss run main.joss
```

Output produced in the console:

```text
Articulos: 2
Total: 42.5
Archivo guardado
```

If you check your folder with `ls` (or `dir` on Windows) or the file explorer, you will see that the physical file `compras.json` has been created:

```json
[{"cantidad":2,"nombre":"Cuaderno","precio":"12.50"},{"cantidad":5,"nombre":"Lapiz","precio":"3.50"}]
```

### Compile to a Standalone Native Binary (`joss build`)

When you want to distribute your console tool to other machines without requiring Joss to be installed or handing over your source code, compile it with:

```bash
joss build main.joss
```

Joss will generate a native executable inside the `build/` folder (`build/main.exe` on Windows or `./build/main` on Linux/macOS) ready to be executed directly.

---

## 5. Colored Visual Output: The Native `Console` Module

So that your console application offers an attractive and professional visual experience, you can color and highlight messages in the terminal using the native **`Console`** class:

```joss
print(Console::green("✓ Archivo guardado con éxito"))
print(Console::yellow("⚠ Advertencia: El stock es bajo"))
print(Console::red("✗ Error al procesar datos"))
print(Console::bold("Total a pagar: S/ 42.50"))
```

### Available `Console` Methods:

| Method | Purpose and Color | Typical Use |
|---|---|---|
| `Console::green($t)` | Green | Successful operations, confirmations (`✓ OK`). |
| `Console::red($t)` | Red | Errors, validation failures, exceptions. |
| `Console::yellow($t)` | Yellow | Warnings, notices requiring attention. |
| `Console::blue($t)` / `Console::cyan($t)` | Blue / Cyan | Titles, links, descriptive information. |
| `Console::bold($t)` | Bold | Numeric totals, highlighted names. |
| `Console::clear()` | Clear screen | Resets the terminal before displaying a menu. |

---

## 6. Practical Exercises

### Level 1 — Quick Test
Add a third item to the `$compras` array (for example, `{"nombre": "Mochila", "precio": "35.00", "cantidad": 1}`), run `joss run main.joss` again, and verify that the total updates to `77.5` and that `compras.json` reflects all 3 items.

### Level 2 — Guided Exercise
1. Create a function `public func articulosCaros(array $compras, decimal $umbral): array` that iterates over `$compras` with `foreach` and returns a new array containing only the products whose `decimal($compra["precio"])` exceeds `$umbral`.
2. Print how many items exceed `10.00m` using `count(articulosCaros($leidas, 10.00m))`.
3. Color the final `"Archivo guardado"` message using `Console::green("Archivo guardado")`.

### Level 3 — Challenge
Modify the program so that before saving the file it prompts the user for an additional item via keyboard:
1. Declare `string $nuevoNombre = ""`, `string $nuevoPrecio = ""`, and `int $nuevaCantidad = 0`.
2. Prompt for each piece of data using `cin >> $nuevoNombre`, `cin >> $nuevoPrecio`, and `cin >> $nuevaCantidad`.
3. Append it to `$compras` with `$compras[] = {"nombre": $nuevoNombre, "precio": $nuevoPrecio, "cantidad": $nuevaCantidad}`, save the JSON, and compile your application with `joss build main.joss`.

---

## Next Step

Now that you have mastered disk data persistence and console utility development, it is time to explore Joss's strongest area: high-performance web application development with routes, controllers, databases, and HTML views.

Continue with: [Build a complete web application with the native stack](PROYECTO_WEB.md).
# Practical project: Console application with JSON persistence

Before: [Concurrency and channels](CONCURRENCIA.md). After: [Complete web project](PROYECTO_WEB.md).
Technical reference: [Command Line (CLI)](CLI.md), [Project Structure](ESTRUCTURA_PROYECTO.md).

---

## What are you going to build here?

A **console application** (CLI) is a program that runs directly in the terminal without a graphical interface. It is the type of software used in automation, batch processing, server maintenance scripts, and developer utilities.

In this guided tutorial we will build a **purchases and inventory manager**:
1. Structure a list of items with exact decimal quantities and prices.
2. Save the data to a physical file on disk (`compras.json`) in structured JSON format.
3. Reads the file from disk, validates its syntactic integrity (`json_verify`) and rebuilds the in-memory data structures (`json_decode`).
4. Process and total amounts with financial decimal precision using modular functions.
5. Manages possible read or write failures in a robust way.

---

## 1. Prepare the work environment

Open your terminal and create a clean directory for this project:```bash
mkdir proyecto-compras
cd proyecto-compras
```
Open your editor and create a file called `main.joss`.

---

## 2. The complete program code

Write or copy the following complete program into `main.joss`:<!-- joss-run: ["Articulos: 2", "Total: 42.5", "Archivo guardado"] -->
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}

$compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]

$guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }

$texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }

$leidas = json_decode($texto)
print("Articulos: " . count($leidas))
print("Total: " . totalizar($leidas))
print("Archivo guardado")
```
---

## 3. Step-by-step explanation of the architecture

Let's analyze how the different language subsystems interact in this program:

### 1. The calculation function (`totalizar`)```joss
public func totalizar(array $compras): decimal {
    return 0m
}
```
- Receives an `array` of purchases and promises to return a value of type `decimal`.
- Initializes an exact accumulator: `decimal $total = 0m`.
- Loop through each element with `foreach ($compras as $compra)`.
- Extract `"precio"` and `"cantidad"` using map keys. Notice how we convert explicitly:
  - `decimal($compra["precio"])`: Converts numeric text to fixed-point decimal.
  - `intval($compra["cantidad"])`: Converts the quantity to a 64-bit integer.
- Multiplies both values ​​and adds them to `$total`.

### 2. In-memory data structure```joss
$compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]
```
- We define an array whose elements are associative maps (`{"clave": valor}`).
- Saving prices as text (`"12.50"`) within the JSON is good accounting practice: it prevents standard JSON decoders from introducing binary inaccuracies when reading float numbers.

### 3. Persistence on disk with JSON```joss
$guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }
```
- `json_encode($compras)`: Transforms the Joss in-memory structure to a standard JSON text.
- `file_put_contents("compras.json", ...)`: Write that text to the physical file on the hard drive. Returns `true` if it succeeded or `false` if it failed due to permissions or lack of space.
- The ternary expression acts as a safeguard: if `$guardado` is false, it throws an error with `throw`.

### 4. Security reading and validation```joss
$texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }
```
- `file_get_contents(...)`: Retrieves the file bytes into a text string. If the file does not exist, returns `null`.
- `json_verify($texto)`: Native Joss function that checks if a string meets the valid JSON specification without parsing the entire tree into memory. If the file is corrupt or was incorrectly edited by a user, it detects it immediately.
- `json_decode($texto)`: Reconstructs JSON data into native Joss arrays and maps ready to be processed.

---

## 4. Analysis and Execution

First, let's run static analysis to ensure there are no inconsistencies:```bash
joss analyze main.joss
```
If everything is correct, run the program:```bash
joss run main.joss
```
Output produced in console:```text
Articulos: 2
Total: 42.5
Archivo guardado
```
If you check your folder with `ls` or file explorer, you will see that the physical file `compras.json` has been created. If you open it, you will see:```json
[{"cantidad":2,"nombre":"Cuaderno","precio":"12.50"},{"cantidad":5,"nombre":"Lapiz","precio":"3.50"}]
```
---

## 5. Visual output with colors: The native `Console` module

To make your console application provide an attractive and professional visual experience, you can color and highlight messages in the terminal using the native **`Console`** class:```joss
print(Console::green("✓ Archivo guardado con éxito"))
print(Console::yellow("⚠ Advertencia: El stock es bajo"))
print(Console::red("✗ Error al procesar datos"))
print(Console::bold("Total a pagar: S/ 42.50"))
```
### Available `Console` methods:

| Method | Purpose and Color | Typical use |
|---|---|---|
| `Console::green($t)` | Green | Successful operations, confirmations (`✓ OK`). |
| `Console::red($t)` | Red | Errors, validation failures, exceptions. |
| `Console::yellow($t)` | Yellow | Warnings, notices that require attention. |
| `Console::blue($t)` / `Console::cyan($t)` | Blue / Cyan | Titles, links, descriptive information. |
| `Console::bold($t)` | Bold | Numerical totals, notable names. |
| `Console::clear()` | Clear screen | Restart the terminal before displaying a menu. |

---

## 6. Exercises to expand the project

1. **Add items dynamically**:
   - Modify the program to ask the user for the name, price and quantity of the following product using `cin >> $nombre`.
   - Add it to the array with `$compras[] = ...` before saving the file.
2. **Filter expensive items**:
   - Create a function `public func articulosCaros(array $compras, decimal $umbral): array` that returns a new array with only those products whose price exceeds the threshold.
3. **Color the final report**:
   - Use `Console::green(...)` to show `"Archivo guardado"` and `Console::bold(...)` for the total.

---

## Next step

Now that you've mastered disk data persistence and console utility development, it's time to explore Joss's strongest area: developing high-performance web applications with routes, controllers, databases, and HTML views.

Continue with: [Build a complete web application with the native stack](PROYECTO_WEB.md).
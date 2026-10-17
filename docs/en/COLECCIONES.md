# Collections: Arrays, Maps, and Unicode Text Manipulation

Previous: [Control flow and decisions](CONTROL_FLUJO.md). Next: [Functions, scope, and pipelines](FUNCIONES.md).
Technical reference: [Native modules](MODULOS_NATIVOS.md), [Global functions](FUNCIONES_GLOBALES.md).

---

## What will you learn here?

So far we have worked with variables that store a single piece of data at a time: a number, a name, or a boolean. But in real life, data almost never comes in isolation:
- A list of products in an online store.
- The comments on a post.
- A user profile with their name, email, phone number, and address.

To group and organize multiple pieces of data into a single structure, we have **collections**.

In this guide you will learn step by step:
1. What an **array** is, how zero-based numbering (**indices**) works, and how to add elements.
2. What a **map** (key → value dictionary) is using braces `{}` or associative style `["clave" => valor]`.
3. How to iterate over collections with `foreach`, extracting both the position or key and the value (`$clave => $valor`).
4. How to unpack multiple values at once using **destructuring** (`[$a, $b] = ...`).
5. How collections work in memory (copy vs shared reference) and useful functions such as `count`, `merge`, `explode`, and `implode`.
6. How Joss handles modern Unicode text (accented letters and emojis) and how to convert collections to **JSON**.

---

## 1. Arrays: Ordered Lists of Elements

An **array** is an ordered list of values enclosed in square brackets `[` and `]`. Each value occupies a numbered slot called an **index**.

In Joss (and in most programming languages), **indices start counting from zero (`0`)**, not from one:
- The first element is at index `0`.
- The second element is at index `1`.
- The third element is at index `2`.

<!-- joss-run: ["pan", "3", "fruta"] -->
```joss
var $compras = ["pan", "leche"]
print($compras[0])
$compras[] = "fruta"
print(count($compras))
print($compras[2])
```

Output:

```text
pan
3
fruta
```

### Fundamental array operations:

1. **Creation**: Written inside square brackets `[` and `]`, separating elements with commas: `["pan", "leche"]`. An empty list is written `[]`.
2. **Reading by index**: `$compras[0]` gets the first element (`"pan"`).
3. **Modifying a position**: `$compras[1] = "queso"` changes the second element.
4. **Appending to the end with `[]`**: Writing `$compras[] = "fruta"` automatically adds the new element to the end of the list.
5. **Counting elements**: `count($compras)` (or `len($compras)`) returns how many elements the list has (`3`).
6. **Bounds protection**: If you try to read a slot that does not exist (such as `$compras[99]` or `$compras[-1]`), Joss stops the operation immediately with the error `JOSS-INDEX-001` (*Index Out of Range*), preventing invalid memory reads.

### Arrays with a fixed element type (`array<Tipo>`)
By default, an `array` allows storing different data types. If you want to require all elements in a list to be integers, you can declare it with `array<int>`:

```joss
array<int> $edades = [18, 25, 30]
```

### Expanding one list inside another with `...` (Spread)

You can insert all the elements of an existing list inside a new list by prefixing it with three dots `...`:

<!-- joss-run: ["1", "2", "3", "4"] -->
```joss
var $primeros = [2, 3]
var $todos = [1, ...$primeros, 4]
print($todos[0])
print($todos[1])
print($todos[2])
print($todos[3])
```

Output:

```text
1
2
3
4
```

---

## 2. Maps: Key → Value Dictionaries

An array is ideal when only order matters (like a queue). But if you want to store a person's record with their name, age, and email, remembering that "the name is at 0 and the age is at 1" is awkward.

That is why **maps** (dictionaries or associative arrays) exist. In a map, each piece of data is stored under a **text label called a key**:

<!-- joss-run: ["Ada", "21", "sin teléfono"] -->
```joss
var $persona = {"nombre": "Ada", "edad": 20}
print($persona["nombre"])
$persona["edad"] = 21
print($persona["edad"])
print($persona["telefono"] ?? "sin teléfono")
```

Output:

```text
Ada
21
sin teléfono
```

### Characteristics of Maps in Joss:
1. **Creation with braces `{}`**: Each text key is associated with its value using a colon: `{"nombre": "Ada", "edad": 20}`. An empty map is written `{}`.
2. **Reading and modification**: Write the key name inside square brackets and quotes: `$persona["nombre"]`.
3. **Keys that do not exist**: If you look up a key that has not been saved yet (such as `$persona["telefono"]`), Joss returns `null` without failing. Combining it with `??` lets you display clean fallback text.
4. **Checking if a key exists**: The built-in function `array_key_exists("telefono", $persona)` returns `true` or `false`.

### Alternative notation with square brackets and fat arrow (`=>`)

In addition to braces `{}`, Joss also accepts creating maps using square brackets `[]` and the fat arrow `=>` (very familiar if you know PHP):

<!-- joss-run: ["JosSecurity", "v1.0", "Ada"] -->
```joss
var $config = [
    "app" => "JosSecurity",
    "version" => "v1.0",
    "autor" => ["nombre" => "Ada"]
]
print($config["app"])
print($config["version"])
print($config["autor"]["nombre"])
```

Output:

```text
JosSecurity
v1.0
Ada
```

### Combining maps with `...` (Spread)

Just like in arrays, you can merge the keys of one map into a new one using `...`:

<!-- joss-run: ["localhost", "8080", "true"] -->
```joss
var $base = ["host" => "localhost", "puerto" => "8080"]
var $completo = [...$base, "seguro" => "true"]
print($completo["host"])
print($completo["puerto"])
print($completo["seguro"])
```

Output:

```text
localhost
8080
true
```

---

## 3. Iterating Over Indices and Keys with `foreach ($coleccion as $clave => $valor)`

In the previous chapter we saw how to iterate over the values of a list. If you also need to know **at what position or under which key** each value is located, write `$clave => $valor` inside the `foreach`:

<!-- joss-run: ["0: manzana", "1: pera", "a => alfa", "b => beta"] -->
```joss
var $frutas = ["manzana", "pera"]
foreach ($frutas as $indice => $fruta) {
    print($indice . ": " . $fruta)
}

var $letras = ["a" => "alfa", "b" => "beta"]
foreach ($letras as $k => $v) {
    print($k . " => " . $v)
}
```

Output:

```text
0: manzana
1: pera
a => alfa
b => beta
```

In addition, you have two direct functions to extract the parts of a map:
- `keys($mapa)`: Returns an array with all the keys.
- `values($mapa)`: Returns an array with all the values.

---

## 4. Array and Map Destructuring

When you want to extract multiple values from an array or a map and store them in individual variables in a single line, you can use **destructuring**, including default values in case any piece of data is missing:

<!-- joss-run: ["10", "20", "30", "Ada", "cliente"] -->
```joss
[$x, $y, $z = 30] = [10, 20]
print($x)
print($y)
print($z)

{"nombre": $nombre, "rol": $rol = "cliente"} = {"nombre": "Ada"}
print($nombre)
print($rol)
```

Output:

```text
10
20
30
Ada
cliente
```

> [!NOTE]
> For compatibility, Joss also accepts the parenthesized form `let ($x, $y) = [10, 20]`, although the bracketed form `[$x, $y] = [10, 20]` is more expressive and supports default values.

---

## 5. Direct Methods on Strings, Lists, and Maps (`->`)

In Joss, strings (`string`), lists (`array`), and maps (`map`) come with **built-in tools (methods)** that you can invoke by writing a thin arrow **`->`** after the variable:

<!-- joss-run: ["hola-mundo", "a-b"] -->
```joss
var $txt = "  Hola Mundo  "
print($txt->trim()->lower()->replace(" ", "-"))

var $mapa = {"a": 1, "b": 2}
print($mapa->keys()->join("-"))
```

Output:

```text
hola-mundo
a-b
```

Notice how it reads from left to right:
1. `$txt->trim()`: Removes extra spaces from both ends (`"Hola Mundo"`).
2. `->lower()`: Converts the letters to lowercase (`"hola mundo"`).
3. `->replace(" ", "-")`: Replaces the space with a hyphen (`"hola-mundo"`).

### Functional list processing

Joss also lets you filter, transform, and sum entire lists with functions such as `sum`, `filter`, `map`, `find`, `any`, and `all` (which we will explore in depth in the next chapter on [Functions](FUNCIONES.md)):

<!-- joss-run: ["60", "20", "true"] -->
```joss
var $numeros = [1, 2, 3, 4, 5]
var $pares = $numeros |> filter(func(int $x): bool { return $x % 2 == 0; })
var $escalados = $pares |> map(func(int $x): int { return $x * 10; })
print(sum($escalados))
var $encontrado = $numeros |> find(func(int $x): bool { return $x == 2; })
print($encontrado * 10)
print($numeros |> any(func(int $x): bool { return $x == 3; }))
```

<!-- joss-run: ["hola-mundo", "6, 8", "a-b"] -->
```joss
var $txt = "  Hola Mundo  "
print($txt->trim()->lower()->replace(" ", "-"))

var $nums = [1, 2, 3, 4]
var $filtrados = $nums->map(func(int $n, int $i): int { return $n * 2; })->filter(func(int $n, int $i): bool { return $n > 4; })
print($filtrados->join(", "))

var $mapa = {"a": 1, "b": 2}
print($mapa->keys()->join("-"))
```

---

## 6. Memory Behavior: Independent Copy or Shared List?

There is a very important difference between simple data and collections:

- When you assign a number or a string to another variable (`var $b = $a`), an **independent copy** is created.
- In contrast, when you assign an **array** or a **map** to another variable (`var $copia = $original`), **both variables point to the same list in memory**:

<!-- joss-run: ["9", "nuevo"] -->
```joss
var $original = [1, 2]
var $copia = $original
$copia[0] = 9
print($original[0])

var $datos = {"estado": "inicial"}
var $alias = $datos
$alias["estado"] = "nuevo"
print($datos["estado"])
```

Output:

```text
9
nuevo
```

Changing `$copia[0]` also changed `$original[0]` because both variables look at the same list.

> [!TIP]
> **How do you create a separate copy of an array?**
> If you want to duplicate a list so you can modify the new one without affecting the original, use `merge`:

```joss
var $clon = merge([], $original)
```

### List end functions (`array_pop`, `array_shift`, `array_push`)

Keep in mind how these three functions work in Joss:

<!-- joss-run: ["2", "2", "3"] -->
```joss
var $numeros = [1, 2]
print(array_pop($numeros))
print(count($numeros))
$numeros = array_push($numeros, 3)
print(count($numeros))
```

Output:

```text
2
2
3
```

1. `array_pop($arr)`: Returns the last element without removing it from the original list.
2. `array_shift($arr)`: Returns the first element without removing it.
3. `array_push($arr, $item)` (or `append($arr, $item)`): Returns a new list with the added element, so you can reassign it (`$numeros = array_push($numeros, 3)`) or directly use `$numeros[] = 3`.

---

## 7. Unicode Text: Visible Characters vs Bytes

In modern languages we use accents (`á`, `é`), special characters (`ñ`), and emojis (`🚀`). In computer memory, a single visible symbol can take up several bytes.

See how Joss lets you measure exactly what you need:

<!-- joss-run: ["3", "2", "é"] -->
```joss
var $texto = "é"
print(len($texto))
print(strlen($texto))
print($texto[0])
```

Output:

```text
3
2
é
```

| Operation | What it measures or returns | When to use it |
|---|---|---|
| `$texto[0]` | The **complete visible character** (*grapheme*) | When working with people's names or messages containing accents and emojis, without splitting them in half. |
| `strlen($texto)` | Number of **Unicode code points** | For standard Unicode character counting. |
| `len($texto)` | Number of **bytes in memory (UTF-8)** | For file sizes or network transmissions. |

---

## 8. Converting Collections to JSON Format (`json_encode` and `json_decode`)

**JSON** is the universal text format for saving data in files or sending it over the internet. Joss converts any array or map to JSON and vice versa with a single instruction:

### With maps using braces `{}`:
<!-- joss-run: ["Ada", "Joss"] -->
```joss
var $perfil = {
    "nombre": "Ada",
    "lenguajes": ["Joss", "Go"]
}
var $jsonMapa = json_encode($perfil)
var $datosMapa = json_decode($jsonMapa)
print($datosMapa["nombre"])
print($datosMapa["lenguajes"][0])
```

Output:

```text
Ada
Joss
```

### With maps using square brackets `[]` and `=>`:
<!-- joss-run: ["Carlos", "admin"] -->
```joss
var $usuario = [
    "nombre" => "Carlos",
    "roles" => ["admin", "editor"]
]
var $jsonArray = json_encode($usuario)
var $datosArray = json_decode($jsonArray)
print($datosArray["nombre"])
print($datosArray["roles"][0])
```

Output:

```text
Carlos
admin
```

- **`json_encode($datos)`**: Converts your array or map into a JSON string.
- **`json_decode($textoJson)`**: Rebuilds the JSON string into ready-to-use Joss arrays and maps.
- **`json_verify($textoJson)`**: Returns `true` if the string is valid JSON or `false` if it is malformed.

---

## 9. Summary of Essential Collection and Text Functions

| Function | Purpose | Example | Result |
|---|---|---|---|
| `count($col)` / `len($col)` | Number of elements in an array or map | `count([10, 20])` | `2` |
| `in_array($val, $arr)` | Does the value exist inside the array? | `in_array(2, [1, 2, 3])` | `true` |
| `keys($map)` | List with all keys of the map | `keys({"a": 1})` | `["a"]` |
| `values($map)` | List with all values of the map | `values({"a": 1})` | `[1]` |
| `explode($sep, $str)` | Splits a string into an array using a separator | `explode(",", "a,b,c")` | `["a", "b", "c"]` |
| `implode($sep, $arr)` | Joins an array of strings with a separator | `implode("-", ["2026", "10", "17"])` | `"2026-10-17"` |
| `array_reverse($arr)` | Reverses the order of an array | `array_reverse([1, 2, 3])` | `[3, 2, 1]` |

---

## 10. Chapter Practice

### 1. Quick check
In the example from section 1:
1. Add a fourth product to `$compras` by writing `$compras[] = "café"`.
2. Display on screen the first element (`$compras[0]`) and the total number of products with `count($compras)`.

### 2. Guided exercise: Inventory record with a Map
1. In your `main.joss`, create a map named `$producto` with three keys: `"nombre": "Laptop"`, `"precio": 1200.00m`, and `"stock": 5`.
2. Display its data: `print("Producto: " . $producto["nombre"] . " | Stock: " . $producto["stock"])`.
3. Simulate a sale by subtracting `1` from `$producto["stock"]` (`$producto["stock"] = $producto["stock"] - 1`).
4. Convert `$producto` to JSON with `print(json_encode($producto))`.

### 3. Challenge: Task list manager
1. Create an array `var $tareas = ["Instalar Joss", "Crear proyecto"]`.
2. Add two more tasks using `$tareas[] = ...`.
3. Iterate over the list with `foreach ($tareas as $indice => $tarea)` and print each task numbered starting from `1` (hint: use `${$indice + 1}: ${tarea}`).

---

## Next step

Now that you know how to represent lists, dictionaries, and JSON in memory, we will learn how to organize our code into reusable, clean, named building blocks: **functions**.

Continue with: [Functions, variable scope, `guard`, `defer`, closures, and pipelines](FUNCIONES.md).

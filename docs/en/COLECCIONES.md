# Collections: Arrays, Maps and Unicode Text Manipulation

Before: [Functions and closures](FUNCIONES.md). After: [Type system and inference](SISTEMA_TIPOS.md).
Technical reference: [Native modules](MODULOS_NATIVOS.md), [Global functions](FUNCIONES_GLOBALES.md).

---

## What are you going to learn here?

Until now we have worked with variables that store a single piece of data at a time: a number, a name, or a boolean. But in real life data almost never comes isolated:
- A list of products in an online store.
- Comments on a publication.
- A user's file with their name, email, telephone number and address.

To group and organize multiple data in a single structure, there are **collections**.

In this guide you will learn:
1. What is an **array**, how numbering from scratch (indexes) works and how to add elements.
2. What is a **map** (key-value dictionary) with brace syntax `{}` or PHP associative style `["clave" => valor]`.
3. How to traverse collections with `foreach` extracting pairs `$clave => $valor`.
4. How to process lists with functional functions and pipelines: `map`, `filter`, `reduce`, `find`, `any`, `all`, `sum`.
5. How collections behave in memory: reference copying vs data duplication.
6. The peculiarities of functions like `array_pop`, `array_push` and `array_shift` in Joss.
7. Text as a collection: the fundamental difference between **bytes**, **Unicode code points** and **graphemes (visible characters)**.

---

## 1. Arrays: Ordered sequences of elements

An **array** is an ordered list of values. Each value occupies a numbered box called **index**.

In Joss (and the vast majority of modern languages), **indexes start counting from zero (`0`)**, not from one:
- The first element is in the index `0`.
- The second element is in the index `1`.
- The third element is in the index `2`.

<!-- joss-run: ["pan", "3", "fruta"] -->
```joss
$compras = ["pan", "leche"]
print($compras[0])
$compras[] = "fruta"
print(count($compras))
print($compras[2])
```

### Basic operations with arrays:

1. **Creation**: `[` and `]` are delimited with square brackets, separating the elements by commas: `["pan", "leche"]`.
2. **Read by index**: `$compras[0]` accesses the first element (`"pan"`).
3. **Add to end with `[]`**: Typing `$compras[] = "fruta"` automatically adds the new element to the end of the list.
4. **Count Elements**: `count($compras)` (or `len($compras)`) returns the total number of elements (in this case, `3`).
5. **Boundary Protection**: If you try to access an index that does not exist (for example `$compras[99]` or a negative index `$compras[-1]`), Joss stops execution immediately with the security error `JOSS-INDEX-001` (Index Out of Range), protecting your program from reading junk memory.

### Collection typing: Homogeneous arrays
By default, an array `array` can contain mixed types. If you want to ensure that all elements are integers, you can use the parameterized syntax:

```joss
array<int> $edades = [18, 25, 30]
```

### Declarative destructuring of arrays and maps

When you have an array or a map and you need to extract its elements into separate variables, you don't need to write repetitive individual assignments. You can unpack them directly into a single line using **declarative destructuring**, including optional default values:

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

This is very convenient for unpacking web request parameters, coordinate pairs, or results returned by functions without ceremonial code.

### Spread operator (`...`) on arrays

You can expand the elements of an existing array into a new array by prepending `...`:

<!-- joss-run: ["1", "2", "3", "4"] -->
```joss
$primeros = [2, 3]
$todos = [1, ...$primeros, 4]
print($todos[0])
print($todos[1])
print($todos[2])
print($todos[3])
```

---

## 2. Maps: Key → Value Dictionaries

An array is perfect when the order of the elements matters (like a waiting list). But if you want to represent an entity with tagged properties (like a user), remembering that "name is at index 0 and mail is at index 1" is brittle and confusing.

That's why **maps** exist (also known as dictionaries, hash tables or associative maps). In a map, each value is saved and retrieved using a **text key**:

<!-- joss-run: ["Ada", "21", "sin teléfono"] -->
```joss
$persona = {"nombre": "Ada", "edad": 20}
print($persona["nombre"])
$persona["edad"] = 21
print($persona["edad"])
print($persona["telefono"] ?? "sin teléfono")
```

### Features of Maps in Joss:
1. **Creation**: They are delimited with braces `{}` associating with `:` (`{"clave": valor}`) or with square brackets `[]` associating with `=>` (`["clave" => valor]`).
2. **Empty map**: `{}` creates an empty map.
3. **Access and modification**: Brackets are used with the key name in quotes: `$persona["nombre"]`.
4. **Non-existent keys**: If you try to read a key that does not exist (such as `$persona["telefono"]`), Joss safely returns `null` instead of failing. You can use the `??` operator to provide an elegant default value.
5. **Existence check**: You can use `array_key_exists("telefono", $persona)` to know for sure if a key was defined, even if its associated value is `null`.

### PHP-style associative notation (`=>`)

Joss supports both the classic braces notation `{}` with a colon `:` and the square bracket notation `=>`, including nested multidimensional maps:

<!-- joss-run: ["JosSecurity", "v1.0", "Ada"] -->
```joss
$config = [
    "app" => "JosSecurity",
    "version" => "v1.0",
    "autor" => ["nombre" => "Ada"]
]
print($config["app"])
print($config["version"])
print($config["autor"]["nombre"])
```

### Spread operator (`...`) in maps

As with arrays, you can expand and merge key-value pairs from one map into another using the `...` operator:

<!-- joss-run: ["localhost", "8080", "true"] -->
```joss
$base = ["host" => "localhost", "puerto" => "8080"]
$completo = [...$base, "seguro" => "true"]
print($completo["host"])
print($completo["puerto"])
print($completo["seguro"])
```

---

## 3. Browse Collections with `foreach ($coleccion as $clave => $valor)`

You can traverse arrays and associative maps by directly accessing the key (or numeric index in arrays) and the value using the syntax `$clave => $valor`:

<!-- joss-run: ["0: manzana", "1: pera", "a => alfa", "b => beta"] -->
```joss
$frutas = ["manzana", "pera"]
foreach ($frutas as $indice => $fruta) {
    print($indice . ": " . $fruta)
}

$letras = ["a" => "alfa", "b" => "beta"]
foreach ($letras as $k => $v) {
    print($k . " => " . $v)
}
```

- `keys($persona)`: Returns an array with all the names of the map keys.
- `values($persona)`: Returns an array with only the values ​​contained in the map.

---

## 4. Higher Order Functions and Pipelines (`map`, `filter`, `reduce`, `find`, `sum`)

Joss includes built-in functions to transform and filter collections in functional style, designed to be chained with the pipeline operator `|>`:

<!-- joss-run: ["60", "20", "true"] -->
```joss
$numeros = [1, 2, 3, 4, 5]
$pares = $numeros |> filter(func(int $x): bool { return $x % 2 == 0; })
$escalados = $pares |> map(func(int $x): int { return $x * 10; })
print(sum($escalados))
$encontrado = $numeros |> find(func(int $x): bool { return $x == 2; })
print($encontrado * 10)
print($numeros |> any(func(int $x): bool { return $x == 3; }))
```

### Fluent instance methods on collections and strings

In addition to the pipeline operator, Joss allows fluent methods to be invoked directly on primitive values ​​(`string`, `array`, `map`):

<!-- joss-run: ["hola-mundo", "6, 8", "a-b"] -->
```joss
$txt = "  Hola Mundo  "
print($txt->trim()->lower()->replace(" ", "-"))

$nums = [1, 2, 3, 4]
$filtrados = $nums->map(func(int $n, int $i): int { return $n * 2; })->filter(func(int $n, int $i): bool { return $n > 4; })
print($filtrados->join(", "))

$mapa = {"a": 1, "b": 2}
print($mapa->keys()->join("-"))
```

---

## 5. Memory behavior: Copy or reference?

This is a fundamental concept in Joss architecture:

- When you assign a number or text to another variable (`$b = $a`), the value is copied independently.
- On the other hand, **arrays** and **maps** are managed internally using pointers and shared structures (Go slices and maps).

If you assign an existing array or map to a new variable, **both variables point to the same data structure in memory**:

<!-- joss-run: ["9", "nuevo"] -->
```joss
$original = [1, 2]
$copia = $original
$copia[0] = 9
print($original[0])
$datos = {"estado": "inicial"}
$alias = $datos
$alias["estado"] = "nuevo"
print($datos["estado"])
```

Modifying `$copia[0]` also altered `$original[0]`, because both are two different names for the same physical array.

> [!TIP]
> **How ​​to create a standalone copy?**
> To duplicate an array without sharing future changes, use the `merge` function:

```joss
$clon = merge([], $original)
```

---

## 6. Function names that you should know well

Some functions for manipulating arrays have specific contracts in Joss that differ from languages ​​such as PHP or JavaScript:

<!-- joss-run: ["2", "2", "3"] -->
```joss
$numeros = [1, 2]
print(array_pop($numeros))
print(count($numeros))
$numeros = array_push($numeros, 3)
print(count($numeros))
```

Pay attention to these details:
1. `array_pop($arr)`: Returns the last element, but **does not remove it from the original array** (does not mutate the length).
2. `array_shift($arr)`: Returns the first element without deleting it.
3. `array_push($arr, $item)` and `append($arr, $item)`: They take the array, add the new element to it and **return the new resulting array**. That's why you should remap: `$numeros = array_push($numeros, 3)` or use the direct syntax `$numeros[] = 3`.
4. `array_slice($arr, $inicio, $longitud)`: Extracts a portion of the array without modifying the original.

---

## 7. Text Manipulation: Bytes, Runes and Graphemes

Modern digital text is much more complex than the English letters on the ASCII keyboard. When you handle text with accents (`á`, `é`), Asian characters, or emojis (`😀`, `👨‍👩‍👧‍👦`), a single visual letter can be made up of multiple bytes and even multiple Unicode characters combined.

In Joss:

<!-- joss-run: ["3", "2", "é"] -->
```joss
$texto = "é"
print(len($texto))
print(strlen($texto))
print($texto[0])
```

Observe the difference of the three lines for the letter `é` (letter `e` with combined accent):
1. `len($texto)`: Returns **3 bytes** in physical UTF-8 format.
2. `strlen($texto)`: Returns **2 Unicode code points** (the base `e` + the combining accent).
3. `$texto[0]`: Returns the **complete visual grapheme** (`é`).

| Operation | Measuring unit | Recommended use |
|---|---|---|
| `len($texto)` | Physical bytes | File sizes, network transfers, disk buffers. |
| `strlen($texto)` | Code Points (Runes) | Standard text analysis algorithms. |
| `$texto[$i]` | Extended graphemes (visible characters) | **User-oriented text manipulation**: cut names, show avatars, index without breaking emojis in half. |

---

## 7. JSON Serialization and Deserialization (`json_encode` and `json_decode`)

In Joss you can serialize and deserialize JSON using both **maps with braces `{}`** notation and **associative arrays with brackets notation `[]` (`=>`)**, in addition to conventional indexed arrays. Both formats are supported at `json_encode` and `json_decode` (or `JSON::encode` and `JSON::decode`):

### With maps using braces `{}`:
<!-- joss-run: ["Ada", "Joss"] -->
```joss
$perfil = {
    "nombre": "Ada",
    "lenguajes": ["Joss", "Go"]
}
$jsonMapa = json_encode($perfil)
$datosMapa = json_decode($jsonMapa)
print($datosMapa["nombre"])
print($datosMapa["lenguajes"][0])
```

### With associative arrays using square brackets `[]` and `=>`:
<!-- joss-run: ["Carlos", "admin"] -->
```joss
$usuario = [
    "nombre" => "Carlos",
    "roles" => ["admin", "editor"]
]
$jsonArray = json_encode($usuario)
$datosArray = json_decode($jsonArray)
print($datosArray["nombre"])
print($datosArray["roles"][0])
```

- **`json_encode($datos, $pretty = false)`**: Converts arrays, maps or instances into a valid JSON string. Passing `true` in the second parameter formats the text with readable indentation.
- **`json_decode($cadenaJson)`**: Converts a JSON string to Joss collections: `{}` objects are deserialized to associative maps `map`, lists `[]` to arrays, and integers preserve type `int`.
- **`json_verify($cadenaJson)`**: Validates if the text contains a syntactically correct JSON structure.

---

## 8. Summary of useful functions for collections

| Function | Purpose | Example |
|---|---|---|
| `count($arr)` / `len($arr)` | Returns the length of a collection or text. | `count([1, 2])` → `2` |
| `is_array($val)` | Checks if a value is an array or associative map. | `is_array(["a" => 1])` → `true` |
| `in_array($val, $arr)` | Checks if a value exists in the array. | `in_array(2, [1, 2, 3])` → `true` |
| `keys($map)` | Gets the list of keys for a map. | `keys({"a": 1})` → `["a"]` |
| `values($map)` | Gets the list of values ​​for a map. | `values({"a": 1})` → `[1]` |
| `explode($sep, $str)` | Splits text into an array using a separator. | `explode(",", "a,b,c")` → `["a", "b", "c"]` |
| `implode($sep, $arr)` | Joins an array of texts into a single string. | `implode("-", ["2026", "09", "05"])` → `"2026-09-05"` |
| `json_encode($val)` | Converts an array or map to JSON text. | `json_encode(["ok" => true])` → `'{"ok":true}'` |
| `json_decode($str)` | Converts JSON text to Joss array or map. | `json_decode('{"ok":true}')` |
| `array_reverse($arr)` | Reverse the order of the elements. | `array_reverse([1, 2, 3])` → `[3, 2, 1]` |

---

## 9. Practical exercises

1. **Inventory Management**:
   - Create a map called `$producto` with the keys `"nombre"` (`"Laptop"`), `"precio"` (`1200.00m`) and `"stock"` (`5`).
   - Sample in the terminal: `"Producto: Laptop | Precio: $1200.00 | Disponibles: 5"`.
   - Simulates a purchase by subtracting 1 from the stock and shows the new value.
2. **Filter words with `explode` and `implode`**:
   - Create a text `$frase = "manzana,pera,uva,platano"`.
   - Convert it to an array using `explode`.
   - Loop through the array with `foreach` and print each fruit in uppercase using the pipeline operator: `$fruta |> strtoupper`.

---

## Next step

Now that you've mastered in-memory data structures, it's time to dive deeper into how the Joss type parser verifies contracts, how inference works, and how null values ​​are safely handled.

Continue with: [Type system, inference and conversions](SISTEMA_TIPOS.md).

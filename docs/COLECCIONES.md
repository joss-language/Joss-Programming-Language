# Colecciones: Arrays, Maps y Manipulación de Texto Unicode

Antes: [Control de flujo y decisiones](CONTROL_FLUJO.md). Después: [Funciones, ámbito y pipelines](FUNCIONES.md).
Referencia técnica: [Módulos nativos](MODULOS_NATIVOS.md), [Funciones globales](FUNCIONES_GLOBALES.md).

---

## ¿Qué vas a aprender aquí?

Hasta ahora hemos trabajado con variables que almacenan un solo dato a la vez: un número, un nombre o un booleano. Pero en la vida real los datos casi nunca vienen aislados:
- Una lista de productos en una tienda online.
- Los comentarios de una publicación.
- La ficha de un usuario con su nombre, correo, teléfono y dirección.

Para agrupar y organizar múltiples datos en una sola estructura, existen las **colecciones**.

En esta guía aprenderás paso a paso:
1. Qué es un **array**, cómo funciona la numeración desde cero (**índices**) y cómo añadir elementos.
2. Qué es un **map** (diccionario clave → valor) con llaves `{}` o estilo asociativo `["clave" => valor]`.
3. Cómo recorrer colecciones con `foreach` extrayendo tanto la posición o clave como el valor (`$clave => $valor`).
4. Cómo desempaquetar varios valores a la vez mediante **desestructuración** (`[$a, $b] = ...`).
5. Cómo funcionan las colecciones en la memoria (copia vs referencia compartida) y funciones útiles como `count`, `merge`, `explode` e `implode`.
6. Cómo maneja Joss el texto moderno Unicode (letras con tildes y emojis) y cómo convertir colecciones a **JSON**.

---

## 1. Arrays: Listas ordenadas de elementos

Un **array** es una lista ordenada de valores encerrados entre corchetes `[` y `]`. Cada valor ocupa una casilla numerada llamada **índice**.

En Joss (y en la mayoría de lenguajes de programación), **los índices comienzan a contar desde cero (`0`)**, no desde uno:
- El primer elemento está en el índice `0`.
- El segundo elemento está en el índice `1`.
- El tercer elemento está en el índice `2`.

<!-- joss-run: ["pan", "3", "fruta"] -->
```joss
var $compras = ["pan", "leche"]
print($compras[0])
$compras[] = "fruta"
print(count($compras))
print($compras[2])
```

Salida:

```text
pan
3
fruta
```

### Operaciones fundamentales con arrays:

1. **Creación**: Se escriben entre corchetes `[` y `]`, separando los elementos con comas: `["pan", "leche"]`. Una lista vacía se escribe `[]`.
2. **Lectura por índice**: `$compras[0]` obtiene el primer elemento (`"pan"`).
3. **Modificar una posición**: `$compras[1] = "queso"` cambia el segundo elemento.
4. **Añadir al final con `[]`**: Escribir `$compras[] = "fruta"` agrega el nuevo elemento al final de la lista automáticamente.
5. **Contar elementos**: `count($compras)` (o `len($compras)`) devuelve cuántos elementos tiene la lista (`3`).
6. **Protección de límites**: Si intentas leer una casilla que no existe (como `$compras[99]` o `$compras[-1]`), Joss detiene la operación de inmediato con el error `JOSS-INDEX-001` (*Index Out of Range*), evitando lecturas de memoria inválidas.

### Arrays con tipo fijo de elemento (`array<Tipo>`)
Por defecto, un `array` permite guardar distintos tipos de datos. Si quieres exigir que todos los elementos de una lista sean números enteros, puedes declararla con `array<int>`:

```joss
array<int> $edades = [18, 25, 30]
```

### Expandir una lista dentro de otra con `...` (Spread)

Puedes insertar todos los elementos de una lista existente dentro de una lista nueva anteponiendo tres puntos `...`:

<!-- joss-run: ["1", "2", "3", "4"] -->
```joss
var $primeros = [2, 3]
var $todos = [1, ...$primeros, 4]
print($todos[0])
print($todos[1])
print($todos[2])
print($todos[3])
```

Salida:

```text
1
2
3
4
```

---

## 2. Maps: Diccionarios de Clave → Valor

Un array es ideal cuando solo importa el orden (como una fila de turnos). Pero si quieres guardar la ficha de una persona con su nombre, edad y correo, recordar que "el nombre está en el 0 y la edad en el 1" es incómodo.

Para eso existen los **maps** (diccionarios o mapas asociativos). En un map, cada dato se guarda bajo una **etiqueta de texto llamada clave**:

<!-- joss-run: ["Ada", "21", "sin teléfono"] -->
```joss
var $persona = {"nombre": "Ada", "edad": 20}
print($persona["nombre"])
$persona["edad"] = 21
print($persona["edad"])
print($persona["telefono"] ?? "sin teléfono")
```

Salida:

```text
Ada
21
sin teléfono
```

### Características de los Maps en Joss:
1. **Creación con llaves `{}`**: Se asocia cada clave de texto con su valor usando dos puntos: `{"nombre": "Ada", "edad": 20}`. Un mapa vacío se escribe `{}`.
2. **Lectura y modificación**: Se escribe el nombre de la clave entre corchetes y comillas: `$persona["nombre"]`.
3. **Claves que no existen**: Si consultas una clave que aún no ha sido guardada (como `$persona["telefono"]`), Joss devuelve `null` sin fallar. Combinarlo con `??` te permite mostrar un texto de respaldo limpio.
4. **Verificar si una clave existe**: La función integrada `array_key_exists("telefono", $persona)` devuelve `true` o `false`.

### Notación alternativa con corchetes y flecha (`=>`)

Además de las llaves `{}`, Joss también acepta crear mapas usando corchetes `[]` y la flecha doble `=>` (muy familiar si conoces PHP):

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

Salida:

```text
JosSecurity
v1.0
Ada
```

### Combinar mapas con `...` (Spread)

Igual que en los arrays, puedes fusionar las claves de un mapa dentro de otro nuevo usando `...`:

<!-- joss-run: ["localhost", "8080", "true"] -->
```joss
var $base = ["host" => "localhost", "puerto" => "8080"]
var $completo = [...$base, "seguro" => "true"]
print($completo["host"])
print($completo["puerto"])
print($completo["seguro"])
```

Salida:

```text
localhost
8080
true
```

---

## 3. Recorrer Índices y Claves con `foreach ($coleccion as $clave => $valor)`

En el capítulo anterior vimos cómo recorrer los valores de una lista. Si también necesitas saber **en qué posición o bajo qué clave** está cada valor, escribe `$clave => $valor` dentro del `foreach`:

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

Salida:

```text
0: manzana
1: pera
a => alfa
b => beta
```

Además, dispones de dos funciones directas para extraer las partes de un mapa:
- `keys($mapa)`: Devuelve un array con todas las claves.
- `values($mapa)`: Devuelve un array con todos los valores.

---

## 4. Desestructuración de Arrays y Maps

Cuando quieres extraer varios valores de un array o de un map y guardarlos en variables individuales en una sola línea, puedes usar la **desestructuración**, incluyendo valores por defecto por si algún dato falta:

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

Salida:

```text
10
20
30
Ada
cliente
```

> [!NOTE]
> Por compatibilidad, Joss también acepta la forma con paréntesis `let ($x, $y) = [10, 20]`, aunque la forma con corchetes `[$x, $y] = [10, 20]` es más expresiva y soporta valores por defecto.

---

## 5. Métodos directos sobre textos, listas y mapas (`->`)

En Joss, los textos (`string`), listas (`array`) y mapas (`map`) traen **herramientas integradas (métodos)** que puedes invocar escribiendo una flecha delgada **`->`** después de la variable:

<!-- joss-run: ["hola-mundo", "a-b"] -->
```joss
var $txt = "  Hola Mundo  "
print($txt->trim()->lower()->replace(" ", "-"))

var $mapa = {"a": 1, "b": 2}
print($mapa->keys()->join("-"))
```

Salida:

```text
hola-mundo
a-b
```

Observa cómo se lee de izquierda a derecha:
1. `$txt->trim()`: Quita los espacios sobrantes de los extremos (`"Hola Mundo"`).
2. `->lower()`: Convierte las letras a minúsculas (`"hola mundo"`).
3. `->replace(" ", "-")`: Cambia el espacio por un guion (`"hola-mundo"`).

### Procesamiento funcional de listas

Joss también permite filtrar, transformar y sumar listas enteras con funciones como `sum`, `filter`, `map`, `find`, `any` y `all` (que exploraremos en profundidad en el próximo capítulo de [Funciones](FUNCIONES.md)):

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

## 6. Comportamiento en memoria: ¿Copia independiente o lista compartida?

Hay una diferencia muy importante entre los datos simples y las colecciones:

- Cuando asignas un número o un texto a otra variable (`var $b = $a`), se crea una **copia independiente**.
- En cambio, cuando asignas un **array** o un **map** a otra variable (`var $copia = $original`), **ambas variables apuntan a la misma lista en la memoria**:

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

Salida:

```text
9
nuevo
```

Cambiar `$copia[0]` también cambió `$original[0]` porque ambas variables miran a la misma lista.

> [!TIP]
> **¿Cómo crear una copia separada de un array?**
> Si quieres duplicar una lista para modificar la nueva sin afectar la original, usa `merge`:

```joss
var $clon = merge([], $original)
```

### Funciones de extremos en listas (`array_pop`, `array_shift`, `array_push`)

Ten presente cómo funcionan estas tres funciones en Joss:

<!-- joss-run: ["2", "2", "3"] -->
```joss
var $numeros = [1, 2]
print(array_pop($numeros))
print(count($numeros))
$numeros = array_push($numeros, 3)
print(count($numeros))
```

Salida:

```text
2
2
3
```

1. `array_pop($arr)`: Devuelve el último elemento sin borrarlo de la lista original.
2. `array_shift($arr)`: Devuelve el primer elemento sin borrarlo.
3. `array_push($arr, $item)` (o `append($arr, $item)`): Devuelve una nueva lista con el elemento añadido, por lo que puedes reasignarla (`$numeros = array_push($numeros, 3)`) o usar directamente `$numeros[] = 3`.

---

## 7. Texto Unicode: Letras visibles vs Bytes

En los idiomas modernos usamos tildes (`á`, `é`), eñes (`ñ`) y emojis (`🚀`). En la memoria de una computadora, un solo símbolo visible puede ocupar varios bytes.

Mira cómo Joss te permite medir exactamente lo que necesitas:

<!-- joss-run: ["3", "2", "é"] -->
```joss
var $texto = "é"
print(len($texto))
print(strlen($texto))
print($texto[0])
```

Salida:

```text
3
2
é
```

| Operación | Qué mide o devuelve | Cuándo usarla |
|---|---|---|
| `$texto[0]` | El **carácter visible completo** (*grafema*) | Cuando trabajas con nombres de personas o mensajes con tildes y emojis, sin romperlos por la mitad. |
| `strlen($texto)` | Cantidad de **puntos de código Unicode** | Para conteo estándar de caracteres Unicode. |
| `len($texto)` | Cantidad de **bytes en memoria (UTF-8)** | Para tamaños de archivo o envíos por red. |

---

## 8. Convertir colecciones a formato JSON (`json_encode` y `json_decode`)

**JSON** es el formato de texto universal para guardar datos en archivos o enviarlos por internet. Joss convierte cualquier array o map a JSON y viceversa con una sola instrucción:

### Con mapas usando llaves `{}`:
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

Salida:

```text
Ada
Joss
```

### Con mapas usando corchetes `[]` y `=>`:
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

Salida:

```text
Carlos
admin
```

- **`json_encode($datos)`**: Convierte tu array o map en una cadena de texto JSON.
- **`json_decode($textoJson)`**: Reconstruye el texto JSON en arrays y maps de Joss listos para usarse.
- **`json_verify($textoJson)`**: Devuelve `true` si el texto es un JSON válido o `false` si está mal formado.

---

## 9. Resumen de funciones esenciales para colecciones y texto

| Función | Propósito | Ejemplo | Resultado |
|---|---|---|---|
| `count($col)` / `len($col)` | Cantidad de elementos de un array o map | `count([10, 20])` | `2` |
| `in_array($val, $arr)` | ¿Existe el valor dentro del array? | `in_array(2, [1, 2, 3])` | `true` |
| `keys($map)` | Lista con todas las claves del map | `keys({"a": 1})` | `["a"]` |
| `values($map)` | Lista con todos los valores del map | `values({"a": 1})` | `[1]` |
| `explode($sep, $str)` | Parte un texto en un array usando un separador | `explode(",", "a,b,c")` | `["a", "b", "c"]` |
| `implode($sep, $arr)` | Une un array de textos con un separador | `implode("-", ["2026", "10", "17"])` | `"2026-10-17"` |
| `array_reverse($arr)` | Invierte el orden de un array | `array_reverse([1, 2, 3])` | `[3, 2, 1]` |

---

## 10. Práctica del capítulo

### 1. Prueba rápida
En el ejemplo de la sección 1:
1. Agrega un cuarto producto a `$compras` escribiendo `$compras[] = "café"`.
2. Muestra en pantalla el primer elemento (`$compras[0]`) y el total de productos con `count($compras)`.

### 2. Ejercicio guiado: Ficha de inventario con un Map
1. Crea en tu `main.joss` un mapa llamado `$producto` con tres claves: `"nombre": "Laptop"`, `"precio": 1200.00m` y `"stock": 5`.
2. Muestra sus datos: `print("Producto: " . $producto["nombre"] . " | Stock: " . $producto["stock"])`.
3. Simula una venta restando `1` a `$producto["stock"]` (`$producto["stock"] = $producto["stock"] - 1`).
4. Convierte `$producto` a JSON con `print(json_encode($producto))`.

### 3. Reto: Gestor de lista de tareas
1. Crea un array `var $tareas = ["Instalar Joss", "Crear proyecto"]`.
2. Añade dos tareas más usando `$tareas[] = ...`.
3. Recorre la lista con `foreach ($tareas as $indice => $tarea)` e imprime cada tarea numerada desde el `1` (pista: usa `${$indice + 1}: ${tarea}`).

---

## Siguiente paso

Ahora que sabes representar listas, diccionarios y JSON en memoria, aprenderemos a organizar nuestro código en piezas reutilizables, limpias y con nombre propio: **las funciones**.

Continúa con: [Funciones, ámbito de variables (scope), `guard`, `defer`, closures y pipelines](FUNCIONES.md).

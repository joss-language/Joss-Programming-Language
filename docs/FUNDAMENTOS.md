# Valores, variables y operaciones fundamentales

Antes: [Primeros pasos](PRIMEROS_PASOS.md). Después: [Control de flujo y decisiones](CONTROL_FLUJO.md).
Referencia técnica: [Sistema de tipos](SISTEMA_TIPOS.md), [Sintaxis y operadores](SINTAXIS.md).

---

## ¿Qué vas a aprender aquí?

Todo programa de computadora existe para procesar información: calcular el total de una factura, guardar el nombre de un usuario, verificar si una contraseña es correcta o contar cuántos mensajes tienes pendientes.

En esta guía aprenderás desde cero:
1. Qué es un **valor** y qué es un **tipo de dato**.
2. Los tipos primitivos esenciales de Joss: enteros, decimales aproximados, decimales exactos, texto y booleanos.
3. Qué es una **variable**, cómo funciona en la memoria y la diferencia entre **declararla** y **reasignarla**.
4. Las cuatro formas canónicas de declarar variables en Joss: inferida (`var $x = ...`), explícita (`int $x = ...`), dinámica (`mixed $x = ...`) y constante (`const`).
5. Cómo operar números, unir o interpolar textos (`"${...}"`) y formatear la salida en pantalla.
6. Qué errores comunes se cometen y cómo prevenirlos.

---

## 1. Valores y tipos de datos: ¿Qué información manejamos?

Un **dato** o **valor** es cualquier pieza de información que un programa utiliza. Por ejemplo:
- `42` es un número entero.
- `"Ada Lovelace"` es un texto.
- `true` (verdadero) es una respuesta lógica.

En una computadora, no todos los datos se almacenan ni se manipulan de la misma forma. Sumar dos cantidades numéricas (`10 + 5 = 15`) es una operación matemática; en cambio, unir dos nombres (`"Ana" . " Gómez"`) es una operación de texto.

El **tipo de dato** define:
1. Qué clase de información representa el valor.
2. Qué operaciones están permitidas sobre él.
3. Cuánta memoria requiere y cómo se almacena internamente.

### Los tipos canónicos fundamentales de Joss

| Tipo de dato | Qué representa | Ejemplos | Cuándo utilizarlo |
|---|---|---|---|
| `int` | Número entero (sin parte decimal) | `0`, `42`, `-15`, `1000` | Para contadores, edades, identificadores numéricos y cantidades indivisibles. |
| `float` | Número con punto decimal (aproximación binaria) | `3.1416`, `0.5`, `-12.8` | Para cálculos científicos, coordenadas, gráficos o mediciones donde pequeñas aproximaciones son aceptables. |
| `decimal` | Número decimal de alta precisión en base diez | `0.10m`, `19.99m`, `100.00M` | **Esencial en finanzas, precios, impuestos y balances**, donde perder un solo céntimo por aproximación binaria es inaceptable. Lleva el sufijo `m` o `M`. |
| `string` | Secuencia de caracteres (texto) | `"Hola"`, `'Joss'`, `"admin@ejemplo.com"` | Para nombres, correos, descripciones, contenido HTML y mensajes. |
| `bool` | Valor de verdad lógico | `true` (verdadero), `false` (falso) | Para tomar decisiones: ¿está autenticado?, ¿hay existencias?, ¿es mayor de edad? |
| `array` | Lista ordenada de valores | `[1, 2, 3]`, `["pan", "leche"]` | Para secuencias de elementos (lo aprenderás en [Colecciones](COLECCIONES.md)). |
| `map` | Diccionario asociativo (clave → valor) | `{"id": 1, "nombre": "Ada"}` | Para registros de datos con etiquetas (lo aprenderás en [Colecciones](COLECCIONES.md)). |
| `null` | Ausencia de valor | `null` (o `nil` por compatibilidad) | Para indicar que un dato todavía no existe, está vacío o no se ha encontrado. |

---

## 2. Variables: Cajas con nombre en la memoria

Una computadora tiene millones de celdas de memoria. Si guardamos un número y no le damos un nombre, no tendremos forma de encontrarlo un milisegundo después.

Una **variable** es simplemente un nombre humano que le asignamos a un espacio de la memoria para almacenar un dato, leerlo cuando lo necesitemos o cambiarlo por otro valor.

### La regla del `$` en Joss

En Joss, **todos los nombres de variable comienzan obligatoriamente con el signo de dólar (`$`)**:
- `$edad`
- `$nombre_completo`
- `$totalPagar`

> [!TIP]
> **¿Por qué Joss usa `$` para las variables?**
> El prefijo `$` permite que tanto tú como el analizador y el compilador distingan al instante una variable de una palabra clave del lenguaje (`return`, `func`, `class`), de un tipo (`int`, `string`) o de una función integrada (`print`). Además, Joss distingue mayúsculas de minúsculas: `$edad` y `$Edad` son dos variables distintas.

Veamos un ejemplo mínimo:

<!-- joss-run: ["21"] -->
```joss
var $edad = 20
$edad = $edad + 1
print($edad)
```

Salida:

```text
21
```

### ¿Qué sucede paso a paso en este programa?

1. `var $edad = 20` (**Declaración**):
   - Es la **primera vez** que aparece `$edad`, por lo que usamos `var` para crearla.
   - El signo `=` es el **operador de asignación**: toma el `20` de la derecha y lo guarda en `$edad`.
   - Joss **infiere** (deduce) automáticamente que `$edad` es de tipo `int`.
2. `$edad = $edad + 1` (**Reasignación**):
   - Como `$edad` **ya existe**, no volvemos a escribir `var`.
   - La computadora calcula primero el lado derecho (`20 + 1` = `21`) y guarda el nuevo resultado `21` dentro de `$edad`.
3. `print($edad)`:
   - Lee el valor actual de `$edad` (`21`) y lo muestra en la terminal.

---

## 3. Las cuatro formas de declarar una variable

En Joss tienes total control sobre cuán estricto o flexible quieres que sea el tipo de tus variables:

### 1. Inferencia canónica fija: `var $x = valor`
Es la forma más cómoda y recomendada en el día a día. Joss deduce el tipo a partir del valor inicial y, desde ese momento, protege la variable para que no cambie de tipo por accidente:

```joss
var $contador = 0       // Infiere int
var $titulo = "Reporte" // Infiere string
var $peso = 72.5        // Infiere float
```

Si más adelante intentas guardar un texto dentro de una variable que nació como entero, el analizador semántico detendrá el programa antes de ejecutar con el código `JOSS-TYPE-001`:

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
var $cantidad = 2
$cantidad = "muchas"
```

### 2. Declaración con tipo explícito: `int $x = valor`
Cuando quieres que el tipo del dato sea 100% visible al leer el código (y de forma obligatoria en los parámetros de las funciones que veremos más adelante):

```joss
int $puerto = 8080
string $usuario = "admin"
decimal $saldo = 1500.50m
bool $activo = true
```

### 3. Variable dinámica explícita: `mixed $x = valor`
A veces necesitas legítimamente que una misma variable guarde primero un número y después un texto. Para permitir cambios de tipo sin error, indícalo con `mixed`:

<!-- joss-run: ["pendiente"] -->
```joss
mixed $resultado = 2
$resultado = "pendiente"
print($resultado)
```

Salida:

```text
pendiente
```

> [!NOTE]
> **Nota de compatibilidad (`let`)**:
> En código antiguo de Joss puedes encontrar `let $x = 2` (equivalente a `mixed $x = 2`) o `let int $x = 2` (equivalente a `int $x = 2`). El lenguaje sigue aceptando `let` por compatibilidad y la herramienta `joss fix` lo actualiza automáticamente a las formas canónicas modernas.

### 4. Constantes inmutables: `const`
Una **constante** es un valor que se define una sola vez y **jamás puede cambiar** mientras corre el programa. Es ideal para tasas de impuestos, límites máximos o nombres de configuración:

<!-- joss-run: ["3"] -->
```joss
const int $maximo = 3
print($maximo)
```

Salida:

```text
3
```

Si intentas escribir `$maximo = 4`, el analizador emitirá el error `JOSS-SYM-006` impidiendo la modificación.

---

## 4. Texto, comentarios e interpolación

### Delimitadores de texto y caracteres de escape

En Joss puedes escribir cadenas de texto usando comillas dobles (`"..."`) o comillas simples (`'...'`):

<!-- joss-run: ["Hola, Ada", "Primera línea", "Segunda línea"] -->
```joss
// Este comentario explica el código; no se ejecuta.
var $nombre = 'Ada'
print("Hola, " . $nombre)
/* Un comentario también puede
   ocupar varias líneas. */
print("Primera línea\nSegunda línea")
```

Salida:

```text
Hola, Ada
Primera línea
Segunda línea
```

- **Comentarios de una línea**: Empiezan con `//` (o `#`). Todo lo que escribas a su derecha es ignorado por la computadora.
- **Comentarios multilínea**: Empiezan con `/*` y terminan con `*/`.
- **Caracteres de escape** (en comillas dobles o simples):
  - `\n`: Inserta un salto de línea.
  - `\t`: Inserta una tabulación horizontal.
  - `\"` o `\'`: Permite incluir comillas literales dentro del texto.
  - `\\`: Inserta una barra invertida.

### Concatenación con el operador punto (`.`)

En otros lenguajes se usa `+` tanto para sumar como para unir textos, lo cual provoca errores silenciosos cuando se mezclan números y textos. En Joss la regla es inequívoca:
- El signo `+` se reserva **exclusivamente para la suma matemática**.
- El punto `.` se utiliza **exclusivamente para unir texto**.

<!-- joss-run: ["1020"] -->
```joss
var $a = "10"
var $b = "20"
print($a . $b)
```

Salida:

```text
1020
```

### Interpolación de cadenas: `${nombre}` y `${$edad * 2}`

En lugar de unir varios fragmentos con puntos (`"Hola " . $nombre . ", tienes " . $edad . " años"`), dentro de textos con **comillas dobles** puedes incrustar variables o cálculos directamente usando `${...}`:

<!-- joss-run: ["Hola Ada, tienes 21 años", "El doble es 42"] -->
```joss
var $nombre = "Ada"
var $edad = 21
print("Hola ${nombre}, tienes ${edad} años")
print("El doble es ${$edad * 2}")
```

Salida:

```text
Hola Ada, tienes 21 años
El doble es 42
```

- **Cuando incrustas una sola variable**: Puedes escribir `${nombre}` (o `${$nombre}`). Joss reconoce el nombre de tu variable automáticamente.
- **Cuando incrustas una operación o cálculo**: Escribe la expresión completa con el signo `$` en cada variable, por ejemplo `${$edad * 2}` o `${$precio + $impuesto}`.
- **Escape literal**: Si necesitas mostrar literalmente los caracteres `${` en pantalla, escribe `\${`.

### Formato con plantillas posicionales (`printf`)

Cuando quieras formatear textos indicando exactamente dónde va cada texto (`%s`) o número entero (`%d`), también dispones de `printf`:

```joss
var $item = "Teclado"
var $cantidad = 2
printf("Producto: %s | Cantidad: %d\n", $item, $cantidad)
```

---

## 5. Operaciones numéricas y matemáticas

<!-- joss-run: ["5", "2.5", "1", "0.3"] -->
```joss
print(2 + 3)
print(5 / 2)
print(5 % 2)
print(0.10m + 0.20m)
```

Salida:

```text
5
2.5
1
0.3
```

### Operadores aritméticos

| Operador | Operación | Ejemplo | Resultado | Explicación |
|---|---|---|---|---|
| `+` | Suma | `10 + 5` | `15` | Adición matemática. |
| `-` | Resta | `10 - 4` | `6` | Sustracción. |
| `*` | Multiplicación | `3 * 4` | `12` | Producto. |
| `/` | División | `5 / 2` | `2.5` | Dividir enteros **devuelve `float`** para no perder la parte fraccionaria (`2.5`). |
| `%` | Módulo (resto) | `5 % 2` | `1` | El sobrante de una división entera (5 repartido entre 2 toca a 2 y sobra `1`). Muy útil para saber si un número es par (`$n % 2 == 0`). |
| `++` | Post-incremento | `$i++` | Valor anterior | Suma `1` a la variable. |
| `--` | Post-decremento | `$i--` | Valor anterior | Resta `1` a la variable. |

### Operadores de asignación rápida (`+=`, `-=`, `*=`, `/=`)

Cuando quieras sumar, restar o multiplicar sobre el valor que ya tiene una variable (como sumar puntos en un marcador), puedes abreviar `$puntos = $puntos + 5` escribiendo `$puntos += 5`:

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

Salida:

```text
15
12
24
```

| Operador | Equivale a escribir | Qué hace |
|---|---|---|
| `$x += $y` | `$x = $x + $y` | Suma `$y` al valor actual de `$x`. |
| `$x -= $y` | `$x = $x - $y` | Resta `$y` al valor actual de `$x`. |
| `$x *= $y` | `$x = $x * $y` | Multiplica el valor actual de `$x` por `$y`. |
| `$x /= $y` | `$x = $x / $y` | Divide el valor actual de `$x` entre `$y`. |

### Orden de prioridad y uso de paréntesis
Igual que en matemáticas de la escuela, la multiplicación `*` y la división `/` se calculan antes que la suma `+` y la resta `-`. Usa paréntesis `(` y `)` cuando quieras indicar qué operación debe hacerse primero:

<!-- joss-run: ["14", "20"] -->
```joss
print(2 + 3 * 4)
print((2 + 3) * 4)
```

Salida:

```text
14
20
```

### Protección automática en cálculos numéricos

Joss vigila tus operaciones matemáticas para evitar fallos silenciosos:
- **División por cero (`JOSS-ARITH-002`)**: Si intentas dividir entre `0` (`10 / 0`), Joss detiene la operación con un mensaje claro.
- **Desbordamiento de enteros (`JOSS-ARITH-001`)**: Si un cálculo con `int` supera la capacidad máxima de un número entero, Joss avisa en lugar de entregar un número negativo falso.
- **Protección de precisión (`JOSS-ARITH-003`)**: Joss permite combinar directamente `int` con `decimal` porque no se pierde ningún dígito, pero pide que uses una función de conversión explícita si intentas mezclar `float` con `decimal` o `int`.

---

## 6. La diferencia clave para dinero: `float` vs `decimal`

¿Por qué existen dos tipos para números con punto (`float` y `decimal`)?

Las computadoras internamente cuentan en binario (ceros y unos). En binario, fracciones como `0.1` o `0.2` tienen infinitos decimales periódicos, por lo que el tipo estándar `float` guarda una aproximación muy cercana:

```joss
print(0.1 + 0.2) // Muestra 0.30000000000000004
```

Para mover un personaje en un videojuego o medir temperatura, esa diferencia minúscula no importa. Pero para **precios, facturas, sueldos o contabilidad**, necesitamos exactitud total en base diez.

Por eso Joss incluye de fábrica el tipo **`decimal`**, que se escribe añadiendo una `m` (o `M`) al final del número:

<!-- joss-run: ["0.3", "23.2"] -->
```joss
print(0.10m + 0.20m)
var $precio = 11.60m
print($precio * 2)
```

Salida:

```text
0.3
23.2
```

> **Regla práctica**: Usa `int` para cantidades enteras, `float` para medidas físicas o científicas y **`decimal` (con `m`) siempre que representes dinero**.

---

## 7. Salida y entrada por flujos (`cout <<` y `cin >>`)

Además de `print(...)`, Joss ofrece el flujo de salida `cout <<` y el manipulador de salto de línea `endl`:

<!-- joss-run: ["Hola mundo", "Linea 1", "Linea 2"] -->
```joss
// Con print: agrega salto de línea automático al final
print("Hola mundo")

// Con cout: encadena varias piezas con << y cierra la línea con endl
cout << "Linea " << 1 << endl
cout << "Linea " << 2 << endl
```

Salida:

```text
Hola mundo
Linea 1
Linea 2
```

- `print(...)`: Ideal para mostrar una línea completa de forma directa.
- `cout << ...`: Muy útil cuando quieres mostrar un mensaje sin saltar de línea (por ejemplo, antes de pedir un dato con `cin >>`) o encadenar varios valores seguidos.
- `cerr << ...`: Envía mensajes al canal de errores de la terminal (`stderr`).

---

## 8. Convertir un dato de un tipo a otro (Casting)

Si lees un dato que viene como texto (por ejemplo `"42"` o `"19.99"`) y necesitas hacer cálculos matemáticos con él, utiliza las funciones integradas de conversión:

<!-- joss-run: ["52", "29.99"] -->
```joss
var $numero = intval("42") + 10
var $precio = decimal("19.99") + 10.00m
print($numero)
print($precio)
```

Salida:

```text
52
29.99
```

| Función | Convierte el valor a | Ejemplo | Resultado |
|---|---|---|---|
| `intval($v)` | `int` (entero) | `intval("42")` | `42` |
| `floatval($v)` | `float` (decimal aproximado) | `floatval("3.14")` | `3.14` |
| `decimal($v)` | `decimal` (decimal exacto) | `decimal("19.99")` | `19.99m` |
| `strval($v)` | `string` (texto) | `strval(100)` | `"100"` |
| `boolval($v)` | `bool` (`true` o `false`) | `boolval(1)` | `true` |

> [!WARNING]
> Si conviertes un texto que no contiene números con `intval("manzana")`, el resultado será `0`. Cuando quieras comprobar antes si un texto representa un número válido, usa `is_numeric($texto)`.

---

## 9. Errores comunes al trabajar con variables y tipos

| Error común | Por qué ocurre | Cómo solucionarlo |
|---|---|---|
| Usar `+` para unir textos (`"Total: " + $precio`) | En Joss `+` es exclusivo para sumar números (`JOSS-TYPE-004`). | Usa el punto `.` (`"Total: " . $precio`) o interpolación (`"Total: ${precio}"`). |
| Olvidar `var` al crear una variable (`$edad = 20`) | Crear una variable sin declararla emite `JOSS-SYM-001`. | Escribe `var $edad = 20` (o `int $edad = 20`) la primera vez que la creas. |
| Cambiar el tipo de una variable creada con `var` | `var $x = 10` fija el tipo en `int` (`JOSS-TYPE-001`). | Si necesitas que cambie de tipo más adelante, declárala con `mixed $x = 10`. |
| Mezclar `int` y `float` sin convertir (`10 + 2.5`) | Joss evita conversiones implícitas con posible pérdida de precisión (`JOSS-ARITH-003`). | Usa ambos del mismo tipo (`10.0 + 2.5`), convierte con `floatval(10) + 2.5`, o usa `decimal` (`10 + 2.5m`). |
| Intentar cambiar una constante (`const $A = 1; $A = 2`) | Las constantes son inmutables (`JOSS-SYM-006`). | Si el valor debe cambiar durante el programa, decláralo con `var` en vez de `const`. |

---

## 10. Mini-proyecto guiado: Calculadora Financiera de Cuenta y Propinas

Pongamos en práctica variables, el tipo exacto `decimal`, operaciones matemáticas e interpolación de texto en un programa completo que puedes colocar dentro de tu `main.joss`:

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

Salida:

```text
Subtotal: 50
Propina (15%): 7.5
Total a pagar: 57.5
Cada persona paga: 28.75
```

---

## 11. Práctica del capítulo

### 1. Prueba rápida
En el mini-proyecto de la sección 10:
1. Cambia `$subtotal` a `120.00m` y `$personas` a `4`.
2. Ejecuta `joss run main.joss` y verifica cuánto debe pagar cada persona.

### 2. Ejercicio guiado: Convertidor de Temperatura (°C a °F)
1. Declara una variable con los grados Celsius: `float $celsius = 25.0`.
2. Calcula su equivalente en Fahrenheit usando paréntesis: `float $fahrenheit = ($celsius * 9.0 / 5.0) + 32.0`.
3. Muestra el resultado usando interpolación: `print("${celsius} °C equivalen a ${fahrenheit} °F")` (debería mostrar `25 °C equivalen a 77 °F`).

### 3. Reto: Factura con descuento e impuesto
Crea un programa que:
1. Declare una constante `const decimal $IVA = 0.16m`.
2. Declare el precio de un producto `decimal $precio = 200.00m` y aplíquele primero un descuento de `20.00m` usando el operador `-=`.
3. Calcule el impuesto (`$precio * $IVA`) y el total final (`$precio + $impuesto`).
4. Muestre en pantalla el precio con descuento, el impuesto y el total final.

---

## Siguiente paso

Ahora que dominas los valores, los tipos de datos y las operaciones en memoria, es hora de enseñar a tus programas a **tomar decisiones** y a **repetir tareas automáticamente**.

Continúa con: [Control de flujo: decisiones y estructuras de repetición](CONTROL_FLUJO.md).

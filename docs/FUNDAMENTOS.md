# Valores, variables y operaciones fundamentales

Antes: [Primeros pasos](PRIMEROS_PASOS.md). Después: [Control de flujo y decisiones](CONTROL_FLUJO.md).
Referencia técnica: [Sistema de tipos](SISTEMA_TIPOS.md), [Sintaxis y operadores](SINTAXIS.md).

---

## ¿Qué vas a aprender aquí?

Todo programa de computadora existe para procesar información: calcular el total de una factura, guardar el nombre de un usuario, verificar si una contraseña es correcta o contar cuántos mensajes tienes pendientes.

En esta guía aprenderás desde cero:
1. Qué es un **valor** y qué es un **tipo de dato**.
2. Los tipos primitivos esenciales de Joss: enteros, decimales aproximados, decimales exactos, texto y booleanos.
3. Qué es una **variable**, cómo funciona en la memoria y cómo declararla.
4. Las cuatro formas de declarar variables en Joss: inferida (`$x = ...`), explícita (`int $x = ...`), dinámica (`mixed $x = ...`) y constante (`const`).
5. Cómo operar números, unir textos y formatear la salida en pantalla.
6. Qué errores comunes se cometen y cómo prevenirlos.

---

## 1. Valores y tipos de datos: ¿Qué información manejamos?

Un **dato** o **valor** es cualquier pieza de información que un programa utiliza. Por ejemplo:
- `42` es un número.
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
| `array` | Lista ordenada de valores | `[1, 2, 3]`, `["pan", "leche"]` | Para secuencias de elementos (detallado en [Colecciones](COLECCIONES.md)). |
| `map` | Diccionario asociativo (clave → valor) | `{"id": 1, "nombre": "Ada"}` | Para registros de datos con propiedades (detallado en [Colecciones](COLECCIONES.md)). |
| `null` / `nil` | Ausencia absoluta de valor | `null` o `nil` | Para indicar que un dato todavía no existe, está vacío o no se ha encontrado. |

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
> El prefijo `$` permite que tanto tú como el analizador y el compilador distingan al instante una variable de una palabra clave del lenguaje (`return`, `func`, `class`), de un tipo (`int`, `string`) o de una función nativa (`print`). Además, Joss distingue mayúsculas de minúsculas: `$edad` y `$Edad` son dos variables distintas.

Veamos un ejemplo mínimo:

<!-- joss-run: ["21"] -->
```joss
$edad = 20
$edad = $edad + 1
print($edad)
```

### ¿Qué sucede paso a paso en este programa?

1. `$edad = 20`:
   - El signo `=` es el **operador de asignación**. Evalúa lo que está a su derecha (`20`) y lo deposita en la variable `$edad`.
   - Como es la primera vez que `$edad` aparece en el programa, Joss **infiere** automáticamente que `$edad` es de tipo `int`.
2. `$edad = $edad + 1`:
   - La computadora evalúa primero el lado derecho: busca el valor actual de `$edad` (que es `20`), le suma `1`, dando como resultado `21`.
   - Luego, el operador `=` guarda ese nuevo valor `21` en `$edad`, sobreescribiendo el `20` anterior.
3. `print($edad)`:
   - Lee el valor actual de `$edad` y lo muestra en la consola.

---

## 3. Las cuatro formas de declarar una variable

En Joss tienes total control sobre cuán estricto o flexible quieres que sea el tipado de tus variables:

### 1. Inferencia automática fija: `$x = valor` (o `var $x = valor`)
Es la forma más rápida y recomendada para el día a día. Joss deduce el tipo en la primera asignación y a partir de ese momento la variable queda protegida:

```joss
$contador = 0       // Infiere int
$titulo = "Reporte" // Infiere string
var $peso = 72.5    // 'var' solicita inferencia explícita; también fija float
```

Si más adelante intentas meter un texto dentro de un entero, el analizador semántico detendrá el programa con el código `JOSS-TYPE-001`:

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
$cantidad = 2
$cantidad = "muchas"
```

### 2. Declaración con tipo explícito: `int $x = valor`
Cuando quieres que el contrato del dato sea 100% visible para cualquier persona que lea el código, o en parámetros de funciones:

```joss
int $puerto = 8080
string $usuario = "admin"
decimal $saldo = 1500.50m
bool $activo = true
```

### 3. Variable dinámica: `mixed $x = valor` (o `let $x = valor`)
A veces estás creando un algoritmo que legítimamente necesita transformar un número en un texto o un estado pendiente. Para permitir cambios de tipo sin error, indícalo explícitamente con `mixed`:

<!-- joss-run: ["pendiente"] -->
```joss
mixed $resultado = 2
$resultado = "pendiente"
print($resultado)
```

> [!NOTE]
> `let $resultado = 2` equivale exactamente a `mixed $resultado = 2`. En Joss, `let` sin tipo **no significa constante**, sino dinamismo explícito.

### 4. Constantes inmutables: `const`
Una **constante** es un valor que se define una sola vez y jamás puede ser reasignado durante la vida del programa. Sirve para blindar reglas de negocio y parámetros de configuración:

<!-- joss-run: ["3"] -->
```joss
const int $maximo = 3
print($maximo)
```

Si intentas escribir `$maximo = 4`, el analizador emitirá un error `JOSS-SYM-006` indicando que no se puede reasignar una constante.

---

## 4. Texto, comentarios y salida formateada

### Delimitadores de texto y caracteres de escape

En Joss puedes escribir cadenas de texto usando comillas dobles (`"..."`) o comillas simples (`'...'`):

<!-- joss-run: ["Hola, Ada", "Primera línea", "Segunda línea"] -->
```joss
// Este comentario explica el código; no se ejecuta.
$nombre = 'Ada'
print("Hola, " . $nombre)
/* Un comentario también puede
   ocupar varias líneas. */
print("Primera línea\nSegunda línea")
```

- **Comentarios de una línea**: Empiezan con `//` (o `#`). Todo lo que escribas a su derecha es ignorado por la computadora.
- **Comentarios multilínea**: Empiezan con `/*` y terminan con `*/`.
- **Caracteres de escape**:
  - `\n`: Inserta un salto de línea.
  - `\t`: Inserta una tabulación horizontal.
  - `\"` o `\'`: Permite incluir comillas literales dentro del texto.
  - `\\`: Inserta una barra invertida.

### Concatenación con el operador punto (`.`)

En muchos lenguajes se usa `+` para unir texto, lo cual genera errores graves cuando se mezclan números y textos por accidente. En Joss:
- El operador `+` se reserva **exclusivamente para la suma matemática**.
- El operador punto `.` se utiliza **exclusivamente para concatenar texto**.

```joss
$a = "10"
$b = "20"
print($a . $b) // Imprime "1020" (unión de textos)
```

### Interpolación moderna de cadenas: `${variable}` o `${expresión}`

En lugar de encadenar múltiples fragmentos con el operador punto (`"Hola " . $nombre . " tienes " . $edad . " años"`), Joss permite **incrustar variables y cálculos directamente dentro de textos con comillas dobles** usando la sintaxis `${...}` (idéntica a lenguajes modernos como Flutter/Dart o Kotlin):

<!-- joss-run: ["Hola Ada, tienes 21 años", "El doble es 42"] -->
```joss
$nombre = "Ada"
$edad = 21
print("Hola ${nombre}, tienes ${edad} años")
print("El doble es ${$edad * 2}")
```

- **Variables**: Escribe `${variable}` dentro de las comillas dobles para inyectar su valor automáticamente.
- **Cálculos y expresiones**: Puedes colocar operaciones matemáticas o lógicas completas entre las llaves: `${$precio * $cantidad}`.
- **Escape literal**: Si necesitas que el texto muestre literalmente `${`, escribe una barra invertida antes: `\${`.

### Formato avanzado con `printf`

Cuando necesitas armar mensajes con variables numéricas y textos en posiciones exactas sin encadenar muchos puntos, utiliza `printf`:

```joss
$item = "Teclado"
$cantidad = 2
printf("Producto: %s | Cantidad: %d\n", $item, $cantidad)
```
- `%s` se reemplaza por una cadena (`string`).
- `%d` se reemplaza por un número entero (`int`).

---

## 5. Operaciones numéricas y matemáticas

<!-- joss-run: ["5", "2.5", "1", "0.3"] -->
```joss
print(2 + 3)
print(5 / 2)
print(5 % 2)
print(0.10m + 0.20m)
```

### Operadores aritméticos

| Operador | Operación | Ejemplo | Resultado | Explicación |
|---|---|---|---|---|
| `+` | Suma | `10 + 5` | `15` | Adición matemática. |
| `-` | Resta | `10 - 4` | `6` | Sustracción. |
| `*` | Multiplicación | `3 * 4` | `12` | Producto. |
| `/` | División | `5 / 2` | `2.5` | En Joss, dividir enteros **devuelve `float`**, evitando la pérdida accidental de decimales. |
| `%` | Módulo (resto) | `5 % 2` | `1` | El residuo de la división entera (5 entre 2 da 2 con resto 1). Muy útil para saber si un número es par (`$n % 2 == 0`). |
| `++` | Post-incremento | `$i++` | Valor actual | Aumenta la variable en 1 y devuelve su valor anterior. |
| `--` | Post-decremento | `$i--` | Valor actual | Disminuye la variable en 1 y devuelve su valor anterior. |

### Prioridad matemática (precedencia)
Al igual que en álgebra, la multiplicación y el módulo se calculan antes que la suma y la resta. Usa paréntesis `(` `)` para definir claramente qué debe resolverse primero:

```joss
print(2 + 3 * 4)   // Da 14 (3 * 4 = 12, luego + 2)
print((2 + 3) * 4) // Da 20 (2 + 3 = 5, luego * 4)
```

### Seguridad aritmética contra desbordamiento (Overflow)

En sistemas de 64 bits convencionales, si sumas 1 al entero más grande posible, el número se convierte en un valor negativo enorme sin avisarte. Joss previene esto en su núcleo:
- Si una operación entera supera el rango con signo de 64 bits (−9,223,372,036,854,775,808 a 9,223,372,036,854,775,807), Joss detiene la ejecución inmediatamente con el error estructurado `JOSS-ARITH-001` (Arithmetic Overflow).
- Si intentas dividir por cero (`$x / 0`), Joss lo detiene con `JOSS-ARITH-002` (Division by Zero).

---

## 6. La diferencia crítica: `float` vs `decimal`

¿Por qué existen `float` y `decimal`?

Los computadores representan los números de punto flotante (`float`) usando potencias de dos en binario (estándar IEEE-754). Hay fracciones decimales como `0.1` o `0.2` que no tienen representación binaria finita exacta (similar a intentar escribir un tercio `1/3` en decimal como `0.33333...`).

Por eso, en casi todos los lenguajes tradicionales:
```joss
print(0.1 + 0.2) // Imprime aproximadamente 0.30000000000000004
```

Si estás calculando la trayectoria de un proyectil en un videojuego, esa millonésima de diferencia no importa. Pero si estás calculando los intereses bancarios de un millón de transacciones, esa diferencia es un desastre contable.

**La solución de Joss es el tipo `decimal`:**
Al añadir el sufijo `m` o `M`:
```joss
print(0.10m + 0.20m) // Imprime exactamente 0.3
```
Los cálculos se realizan en base diez exacta utilizando aritmética de coma fija.

---

## 7. Entrada interactiva de datos con `cin` y valores nulos

### Entrada desde la consola con `cin >>`

Para crear programas interactivos donde una persona escriba datos en la consola durante la ejecución, Joss proporciona el flujo de entrada `cin` con el operador `>>`:

```joss
print("¿Cómo te llamas? ")
string $nombre = ""
cin >> $nombre

print("¿Cuántos años tienes? ")
int $edad = 0
cin >> $edad

print("Hola ${nombre}, el próximo año tendrás ${$edad + 1} años.")
```

**Características inteligentes de `cin`:**
1. **Conversión automática de tipo**: Si la variable de destino es de tipo numérico (`int` o `float`), `cin` convierte el texto introducido directamente al número correspondiente sin necesidad de llamar a `intval` o `floatval`.
2. **Lectura de texto completo**: Si la variable es `string`, captura la línea completa escrita por el usuario.
3. **Encadenamiento múltiple**: Puedes leer varias variables en una sola instrucción: `cin >> $primerNumero >> $segundoNumero`.

### Asignación nula coalescente (`??=`)

Si una variable tiene valor `null` o aún no ha sido inicializada, puedes asignarle un valor predeterminado solo si está vacía usando `??=`:

<!-- joss-run: ["oscuro", "oscuro"] -->
```joss
$tema = null
$tema ??= "oscuro"
print($tema)
$tema ??= "claro" // No sobreescribe porque ya tiene "oscuro"
print($tema)
```

---

## 8. Funciones de conversión de tipo (Casting)

Si recibes un dato como texto (por ejemplo `"25"`) y necesitas sumarle una cantidad, debes convertirlo explícitamente a número:

| Función | Convierte a | Ejemplo de entrada | Resultado |
|---|---|---|---|
| `intval($v)` | `int` | `intval("42")` | `42` |
| `floatval($v)` | `float` | `floatval("3.14")` | `3.14` |
| `decimal($v)` | `decimal` | `decimal("19.99")` | `19.99m` |
| `strval($v)` | `string` | `strval(100)` | `"100"` |
| `boolval($v)` | `bool` | `boolval(1)` | `true` |

> [!WARNING]
> Convertir no es lo mismo que validar. Si intentas ejecutar `intval("manzana")`, la función devolverá `0` sin fallar. Si necesitas verificar primero si un texto contiene números válidos, utiliza `is_numeric($texto)`.

---

## 9. Errores comunes al empezar

| Error común | Código / Causa | Cómo corregirlo |
|---|---|---|
| Usar `+` para unir textos | `print("Total: " + $precio)` | Usa siempre el punto para texto: `print("Total: " . $precio)`. |
| Olvidar el `$` en una variable | `edad = 20` | Todas las variables deben llevar `$`: `$edad = 20`. |
| Cambiar de tipo una variable inferida | `$x = 10; $x = "hola"` (`JOSS-TYPE-001`) | Si necesitas que cambie de tipo, declárala como `mixed $x = 10`. |
| Olvidar el sufijo `m` en importes financieros | `0.10 + 0.20` | Usa `0.10m + 0.20m` para garantizar exactitud monetaria. |
| Reasignar una constante | `const $A = 1; $A = 2` (`JOSS-SYM-006`) | Si el valor debe cambiar, no uses `const`; usa `$A = 1`. |

---

## 10. Ejercicios prácticos

1. **Calculadora de propinas**:
   - Declara una variable decimal `$cuenta = 45.50m`.
   - Declara un porcentaje `$propina = 0.15m` (15%).
   - Calcula el monto de la propina y el total a pagar (`$cuenta + ($cuenta * $propina)`).
   - Muestra ambos resultados en la consola con mensajes claros.
2. **Convertidor de temperatura**:
   - Declara una variable `$celsius = 25.0`.
   - Aplica la fórmula para convertir a Fahrenheit: `$fahrenheit = ($celsius * 9 / 5) + 32`.
   - Imprime el resultado concatenado: `print($celsius . " °C equivalen a " . $fahrenheit . " °F")`.

---

## Siguiente paso

Ahora que dominas los datos, los tipos y las operaciones matemáticas en memoria, es hora de dotar a tus programas de capacidad de decisión y repetición: cómo ejecutar un bloque solo si se cumple una condición y cómo crear bucles.

Continúa con: [Control de flujo y estructuras de repetición](CONTROL_FLUJO.md).

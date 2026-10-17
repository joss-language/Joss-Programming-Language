# Control de flujo y toma de decisiones

Antes: [Valores y variables](FUNDAMENTOS.md). Después: [Colecciones: Arrays, Maps y Texto](COLECCIONES.md).
Referencia técnica: [Sintaxis y operadores](SINTAXIS.md), [Gramática](GRAMATICA.md).

---

## ¿Qué vas a aprender aquí?

Hasta ahora, nuestros programas se han ejecutado en línea recta: la computadora lee la línea 1, luego la 2, luego la 3 y termina. A este orden natural se le llama **flujo secuencial**.

Sin embargo, el verdadero poder de la programación está en responder a situaciones cambiantes y repetir tareas sin cansarse:
- *Si* el usuario tiene 18 años o más, permitir la entrada; *de lo contrario*, pedirle que espere.
- *Mientras* el contador no llegue a 3, seguir contando.
- *Para cada* producto de una lista, mostrar su nombre en pantalla.

En esta guía aprenderás paso a paso:
1. Qué es una **condición lógica** (`bool`) y cómo comparar datos.
2. Cómo combinar preguntas lógicas con `&&` (Y), `||` (O) y `!` (NO).
3. Cómo tomar decisiones en Joss usando el **operador ternario (`? :`)** con valores y con bloques `{ ... }`.
4. Cómo asignar valores por defecto con el operador Elvis `?:` y la coalescencia nula `??` / `??=`.
5. Cómo elegir entre muchas opciones limpiamente con **`match`**.
6. Cómo repetir instrucciones con los ciclos **`while`**, **`do ... while`** y **`foreach`**.
7. Cómo detener o saltar una vuelta de un ciclo con **`break`** y **`continue`**.

---

## 1. Decisiones lógicas: Hacerle preguntas a la computadora

Una **condición** es una pregunta que la computadora evalúa para responder únicamente con un valor booleano (`bool`): **`true`** (verdadero) o **`false`** (falso).

<!-- joss-run: ["true", "Entrada permitida"] -->
```joss
var $edad = 20
print($edad >= 18)
print(($edad >= 18) ? "Entrada permitida" : "Debes esperar")
```

Salida:

```text
true
Entrada permitida
```

### Operadores de comparación

| Operador | Pregunta que realiza | Ejemplo | Resultado |
|---|---|---|---|
| `==` | ¿Tienen el mismo valor? | `5 == 5` | `true` |
| `!=` | ¿Son diferentes? | `5 != 3` | `true` |
| `===` | ¿Son idénticos en valor **y además en tipo**? | `5 === "5"` | `false` |
| `!==` | ¿No son estrictamente idénticos? | `5 !== "5"` | `true` |
| `<` | ¿El de la izquierda es menor? | `3 < 5` | `true` |
| `<=` | ¿Es menor o igual? | `5 <= 5` | `true` |
| `>` | ¿El de la izquierda es mayor? | `10 > 2` | `true` |
| `>=` | ¿Es mayor o igual? | `20 >= 18` | `true` |
| `<=>` | Comparación triple (*Spaceship*) | `$a <=> $b` | Devuelve `-1` si `$a < $b`, `0` si son iguales y `1` si `$a > $b`. |

### Combinar varias condiciones (`&&`, `||`, `!`)

Cuando una decisión depende de dos o más preguntas a la vez, usamos los **operadores lógicos**:

<!-- joss-run: ["true", "true", "false"] -->
```joss
var $edad = 22
var $tieneEntrada = true

print($edad >= 18 && $tieneEntrada)
print($edad < 18 || $tieneEntrada)
print(!$tieneEntrada)
```

Salida:

```text
true
true
false
```

- **`&&` (Y lógico)**: Devuelve `true` solo si **ambas** condiciones se cumplen.
- **`||` (O lógico)**: Devuelve `true` si **al menos una** de las condiciones se cumple.
- **`!` (Negación)**: Invierte el valor lógico (`!true` es `false`, y `!false` es `true`).

---

## 2. Tomar decisiones en Joss: El operador ternario (`? :`)

A diferencia de otros lenguajes que usan las palabras `if` y `else`, **Joss unifica todas las decisiones bajo una sola construcción clara: el operador ternario**.

Su forma básica se lee como una pregunta en español:

```text
(¿se cumple la condición?) ? resultado_si_es_verdadero : resultado_si_es_falso
```

### 1. Elegir entre dos valores y guardarlos en una variable

Como el ternario produce un valor directamente, puedes guardar la respuesta en una variable en una sola línea:

<!-- joss-run: ["Aprobado"] -->
```joss
var $puntos = 75
var $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
print($estado)
```

Salida:

```text
Aprobado
```

- Primero se evalúa `($puntos >= 60)`. Como `75 >= 60` es `true`, Joss elige el texto después del signo `?` (`"Aprobado"`) y lo guarda en `$estado`.
- Si `$puntos` hubiera sido `40`, habría elegido el texto después de los dos puntos `:` (`"Reprobado"`).

### 2. Ejecutar varias líneas con bloques `{ ... }`

Un **bloque** es un grupo de instrucciones encerradas entre llaves `{` y `}`. Cuando quieres que una rama ejecute varios pasos seguidos, simplemente colocas un bloque `{ ... }`:

<!-- joss-run: ["Hay existencias", "Preparando pedido"] -->
```joss
var $existencias = 4
($existencias > 0) ? {
    print("Hay existencias")
    print("Preparando pedido")
} : {
    print("Producto agotado")
}
```

Salida:

```text
Hay existencias
Preparando pedido
```

### 3. ¿Qué pasa si solo necesito hacer algo cuando es verdadero?

Si no necesitas hacer nada cuando la condición sea falsa, puedes **omitir por completo la parte `: { ... }`**:

<!-- joss-run: ["Bienvenido de nuevo"] -->
```joss
var $usuarioAutenticado = true
($usuarioAutenticado) ? {
    print("Bienvenido de nuevo")
}
```

Salida:

```text
Bienvenido de nuevo
```

*(También es válido escribir `: {}` al final si prefieres dejar explícito el bloque vacío).*

> [!NOTE]
> **¿Y las cláusulas de guarda (`guard`) y limpieza (`defer`)?**
> Joss también cuenta con las sentencias `guard (condicion) : { ... }` (para validar condiciones y salir temprano de una función) y `defer { ... }` (para ejecutar tareas de cierre al terminar una función). Las aprenderás en el capítulo de [Funciones](FUNCIONES.md), en cuanto conozcas cómo funcionan `public func` y `return`.

---

## 3. Valores por defecto rápidos: Elvis (`?:`) y Coalescencia Nula (`??` y `??=`)

Muchas veces quieres decir: *"usa este dato si existe, o usa este valor de respaldo si está vacío"*. Joss tiene tres operadores diseñados para ello:

<!-- joss-run: ["Anónimo", "Invitado", "oscuro"] -->
```joss
var $aliasIngresado = ""
var $apodo = $aliasIngresado ?: "Anónimo"
print($apodo)

var $opcionUsuario = null
var $perfil = $opcionUsuario ?? "Invitado"
print($perfil)

var $tema = null
$tema ??= "oscuro"
print($tema)
```

Salida:

```text
Anónimo
Invitado
oscuro
```

| Operador | Cuándo usa el valor de respaldo (derecha) | Cuándo elegirlo |
|---|---|---|
| **`?:` (Elvis)** | Cuando la izquierda es falsa o vacía (`null`, `false`, `0`, `""`, `[]`). | Cuando un texto vacío `""` o un `0` deben reemplazarse por el valor por defecto. |
| **`??` (Coalescencia nula)** | **Únicamente** cuando la izquierda es `null`. | Cuando `0`, `false` o `""` son datos válidos que quieres conservar y solo deseas reemplazar `null`. |
| **`??=` (Asignación nula)** | Guarda el valor de la derecha dentro de la variable solo si actualmente vale `null`. | Para inicializar variables que podrían estar en `null`. |

---

## 4. Selección múltiple con `match`

Cuando una variable puede tomar muchos valores distintos (como los colores de un semáforo, los días de la semana o el estado de un pedido), usar la expresión **`match`** hace que el código sea limpio y fácil de leer:

<!-- joss-run: ["En camino"] -->
```joss
var $estado = "enviado"
var $mensaje = match ($estado) {
    "nuevo" => "Preparando paquete",
    "enviado", "reparto" => "En camino",
    "entregado" => "Pedido completado",
    default => "Consulta el estado de tu pedido"
}
print($mensaje)
```

Salida:

```text
En camino
```

### Cómo funciona `match`:
1. Joss compara el valor de `$estado` con las opciones de cada línea de arriba hacia abajo.
2. Puedes agrupar varios casos con una coma (por ejemplo `"enviado", "reparto" => "En camino"`).
3. En cuanto encuentra la primera coincidencia, devuelve lo que está a la derecha de la flecha `=>` y termina (no necesitas escribir `break`).
4. **`default`**: Es la opción comodín que se ejecuta si ningún caso anterior coincidió.
5. **Bloques en `match`**: Si en lugar de devolver un valor quieres ejecutar varias instrucciones en cada caso, también puedes usar llaves `{ ... }`:

<!-- joss-run: ["Opción 1 ejecutada"] -->
```joss
var $opcion = 1
match ($opcion) {
    1 => {
        print("Opción 1 ejecutada")
    },
    default => {
        print("Opción por defecto")
    }
}
```

Salida:

```text
Opción 1 ejecutada
```

---

## 5. Bucles: Repetir instrucciones con `while` y `do ... while`

Un **bucle** (o ciclo) le ordena a la computadora repetir un bloque `{ ... }` mientras una condición siga siendo verdadera (`true`).

### El ciclo `while` (repetir mientras se cumpla la condición)

`while` comprueba la condición **antes** de cada vuelta:

<!-- joss-run: ["1", "2", "3"] -->
```joss
var $numero = 1
while ($numero <= 3) {
    print($numero)
    $numero++
}
```

Salida:

```text
1
2
3
```

#### ¿Qué ocurre vuelta por vuelta en la memoria?
1. **Inicio**: `$numero` vale `1`.
2. **Vuelta 1**: ¿Es `1 <= 3`? Sí (`true`). Imprime `1` y `$numero++` lo sube a `2`.
3. **Vuelta 2**: ¿Es `2 <= 3`? Sí (`true`). Imprime `2` y `$numero++` lo sube a `3`.
4. **Vuelta 3**: ¿Es `3 <= 3`? Sí (`true`). Imprime `3` y `$numero++` lo sube a `4`.
5. **Comprobación final**: ¿Es `4 <= 3`? No (`false`). El ciclo termina y el programa continúa abajo.

> [!CAUTION]
> **Evita los bucles infinitos**:
> Dentro de las llaves de un `while` siempre debe cambiar la variable de la condición (por ejemplo con `$numero++`). Si olvidas esa línea, `$numero` valdrá `1` para siempre y el ciclo no terminará nunca (si te ocurre en tu terminal, presiona `Ctrl + C` para detenerlo).

### El ciclo `do ... while` (ejecutar al menos una vez)

A diferencia de `while`, el ciclo `do ... while` ejecuta el bloque **primero** y comprueba la condición **al final**. Así te aseguras de que el bloque corra como mínimo una vez:

<!-- joss-run: ["Intento 1"] -->
```joss
var $intento = 0
do {
    $intento++
    print("Intento " . $intento)
} while ($intento < 1)
```

Salida:

```text
Intento 1
```

---

## 6. Recorrer secuencias y listas con `foreach`

Cuando quieres recorrer una secuencia de números del `1` al `3` o pasar por cada elemento de una lista sin preocuparte de sumar un contador a mano, utilizas **`foreach`** (*para cada uno*).

### 1. Recorrer un rango de números (`inicio..fin`)

El operador de dos puntos seguidos `..` genera los números enteros desde el inicio hasta el final:

<!-- joss-run: ["Paso 1", "Paso 2", "Paso 3"] -->
```joss
foreach (1..3 as $paso) {
    print("Paso ${paso}")
}
```

Salida:

```text
Paso 1
Paso 2
Paso 3
```

### 2. Recorrer una lista sencilla

> **Breve adelanto**: En el próximo capítulo ([Colecciones](COLECCIONES.md)) aprenderemos todo sobre las listas (`array`). Por ahora, basta con saber que escribir varios valores entre corchetes `["pan", "leche"]` crea una lista ordenada de elementos.

Mira qué fácil es recorrer cada elemento de una lista con `foreach`:

<!-- joss-run: ["pan", "leche"] -->
```joss
var $compras = ["pan", "leche"]
foreach ($compras as $producto) {
    print($producto)
}
```

Salida:

```text
pan
leche
```

- **En la primera vuelta**, `$producto` toma el valor `"pan"` y lo imprime.
- **En la segunda vuelta**, `$producto` toma el valor `"leche"` y lo imprime.
- Al no quedar más elementos en `$compras`, el ciclo se detiene solo.

---

## 7. Controlar el interior de un ciclo: `break` y `continue`

Dentro de cualquier bucle (`while`, `do ... while` o `foreach`) puedes usar dos órdenes especiales:

- **`break`**: Detiene el bucle por completo en ese mismo instante y continúa en la línea posterior al cierre del ciclo.
- **`continue`**: Omite el resto de la vuelta actual y pasa inmediatamente a la siguiente iteración del ciclo.

Por ejemplo, si queremos recorrer una secuencia pero detenernos apenas hayamos impreso el primer elemento:

<!-- joss-run: ["Primero: 10"] -->
```joss
var $datos = [10, 20, 30]
foreach ($datos as $valor) {
    print("Primero: " . $valor)
    break
}
```

Salida:

```text
Primero: 10
```

Al ejecutarse `break` al final de la primera vuelta, los valores `20` y `30` ya no se procesan.

De forma análoga, puedes usar un ternario o `break`/`continue` para saltar o detener un ciclo cuando se cumpla una condición:

```joss
while ($intentos < 10) {
    $intentos++
    ($intentos == 5) ? {
        break
    }
}
```

---

## 8. Errores comunes al tomar decisiones y repetir código

| Error común | Por qué ocurre | Cómo solucionarlo |
|---|---|---|
| Escribir `if ($x > 0)` o `for (...)` | `if`, `else`, `switch` y `for` no existen en Joss (`JOSS-PARSE-001`). | Usa el operador ternario `($x > 0) ? { ... }`, `match` y `foreach` / `while`. |
| Confundir `=` con `==` | `=` guarda un valor; `==` compara dos valores. | Dentro de una pregunta lógica usa siempre `==` o `===` (`$edad == 18`). |
| Olvidar actualizar la variable en `while` | La condición nunca cambia y el ciclo se repite sin fin. | Asegúrate de incrementar (`$i++`) o modificar la variable dentro del bloque `{ ... }`. |
| Usar `break` o `continue` fuera de un ciclo | Solo tienen sentido dentro de `while`, `do...while` o `foreach` (`JOSS-FLOW-002`). | Colócalos únicamente dentro del cuerpo de un bucle. |

---

## 9. Práctica del capítulo

### 1. Prueba rápida
En el ejemplo de `match` de la sección 4:
1. Cambia el valor inicial de `var $estado = "enviado"` a `"entregado"`, y luego a `"cancelado"`.
2. Ejecuta `joss run main.joss` en cada caso y observa qué brazo de `match` se activa.

### 2. Ejercicio guiado: Clasificador de calificaciones
Escribe en tu `main.joss` un programa que evalúe una nota siguiendo estos pasos:
1. Declara `int $nota = 85`.
2. Usa un ternario para comprobar si `$nota >= 90` e imprimir `"Sobresaliente"`.
3. En la rama `: { ... }`, comprueba si `$nota >= 60` para imprimir `"Aprobado"` o `"Reprobado"`.
4. Prueba tu programa con las notas `95`, `70` y `45`.

### 3. Reto: Tabla de multiplicar y suma de pares
1. Declara una variable `int $tabla = 7`.
2. Usa `foreach (1..10 as $i)` para imprimir la tabla del 7 completa con el formato `"7 x 1 = 7"`, `"7 x 2 = 14"`, etc.
3. Declara antes del ciclo una variable `int $sumaPares = 0` y, dentro del ciclo, usa un ternario `(($tabla * $i) % 2 == 0) ? { ... }` para sumar en `$sumaPares` únicamente los resultados que sean números pares. Imprime la suma al final.

---

## Siguiente paso

Ya sabes cómo tomar decisiones y recorrer secuencias. El siguiente paso natural es aprender a guardar y organizar colecciones completas de datos en memoria: listas dinámicas (**arrays**), diccionarios con nombre (**maps**) y procesamiento de texto.

Continúa con: [Colecciones: Arrays, Maps y Manipulación de Texto Unicode](COLECCIONES.md).

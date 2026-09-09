# Control de flujo y toma de decisiones

Antes: [Valores y variables](FUNDAMENTOS.md). Después: [Funciones y closures](FUNCIONES.md).
Referencia técnica: [Sintaxis y operadores](SINTAXIS.md), [Gramática](GRAMATICA.md).

---

## ¿Qué vas a aprender aquí?

Hasta ahora, todos nuestros programas se han ejecutado en línea recta: la computadora lee la línea 1, luego la 2, luego la 3 y termina. A este orden natural se le llama **flujo secuencial**.

Sin embargo, el verdadero poder de la programación reside en la capacidad de responder a circunstancias cambiantes:
- *Si* el usuario escribió la contraseña correcta, déjalo entrar; *de lo contrario*, muestra un mensaje de error.
- *Mientras* queden correos por enviar, continúa enviándolos uno por uno.
- *Para cada* producto en el carrito de compras, suma su precio al total.

En esta guía aprenderás:
1. Qué es una **condición lógica** y cómo evaluarla.
2. Cómo tomar decisiones en Joss usando el operador **ternario con bloques** y por qué Joss no utiliza la sintaxis clásica `if/else`.
3. El operador Elvis `?:` y el operador de coalescencia nula `??`.
4. Cómo estructurar selecciones múltiples elegantes con la expresión `match`.
5. Cómo repetir código con bucles `while`, `do...while` y `foreach` (incluyendo su uso con canales de concurrencia).
6. Cómo interrumpir o avanzar un ciclo con `break` y `continue`.

---

## 1. Decisiones lógicas: Preguntarle a la computadora

Una **condición** es cualquier expresión que la computadora evalúa para obtener una respuesta lógica de tipo booleano (`bool`): o es verdadera (`true`) o es falsa (`false`).

<!-- joss-run: ["true", "Entrada permitida"] -->
```joss
$edad = 20
print($edad >= 18)
print(($edad >= 18) ? "Entrada permitida" : "Debes esperar")
```

### Operadores de comparación

| Operador | Pregunta lógica | Ejemplo | Resultado |
|---|---|---|---|
| `==` | ¿Son iguales los valores? | `5 == 5` | `true` |
| `!=` | ¿Son diferentes los valores? | `5 != 3` | `true` |
| `===` | ¿Son estrictamente idénticos en valor **y tipo**? | `5 === "5"` | `false` |
| `!==` | ¿No son estrictamente idénticos? | `5 !== "5"` | `true` |
| `<` | ¿Es menor el de la izquierda? | `3 < 5` | `true` |
| `<=` | ¿Es menor o igual? | `5 <= 5` | `true` |
| `>` | ¿Es estrictamente mayor? | `10 > 2` | `true` |
| `>=` | ¿Es mayor o igual? | `20 >= 18` | `true` |
| `<=>` | Operador nave espacial (Spaceship) | `$a <=> $b` | Retorna `-1` si `$a < $b`, `0` si `$a == $b`, `1` si `$a > $b`. |

---

## 2. La filosofía de Joss: El operador ternario como estructura de control

A diferencia de otros lenguajes que tienen una palabra reservada `if` y otra `else`, **Joss unifica todas las decisiones bajo el operador ternario**.

La estructura básica del ternario es:

```text
(condición) ? resultado_si_es_verdadero : resultado_si_es_falso
```

### ¿Por qué Joss eligió este diseño?

1. **Es una expresión, no una sentencia aislada**: Puedes asignar el resultado de una decisión directamente a una variable sin crear variables vacías intermedias:
   ```joss
   $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
   ```
2. **Estructura visual limpia y sin ambigüedades**: Evita los problemas clásicos de `if` sin llaves o anidamientos confusos.

### Ejecutar múltiples instrucciones con bloques `{ ... }`

Cuando necesitas ejecutar varias líneas de código en una de las ramas, simplemente coloca un bloque entre llaves `{` y `}`:

<!-- joss-run: ["Hay existencias", "Preparando pedido"] -->
```joss
$existencias = 4
($existencias > 0) ? {
    print("Hay existencias")
    print("Preparando pedido")
} : {
    print("Producto agotado")
}
```

### ¿Qué pasa si no necesito la rama falsa?

Si solo quieres hacer algo cuando la condición sea verdadera y no necesitas una alternativa falsa, puedes omitir la rama `: { ... }` completamente:

<!-- joss-run: ["Bienvenido de nuevo"] -->
```joss
$usuarioAutenticado = true
($usuarioAutenticado) ? {
    print("Bienvenido de nuevo")
}
```

También es válido escribir la forma simétrica con bloque vacío `: {}` si prefieres mantener ambos lados explícitos.

### Guard Clauses: Retorno anticipado dentro de funciones

En Joss, si ejecutas una instrucción `return` dentro del bloque de un ternario, el `return` **burbujea de inmediato saliendo de la función contenedora**. Esto permite escribir cláusulas de guarda (*guard clauses*) limpias y evitar anidamientos profundos:

```joss
public func procesarPago(decimal $monto): bool {
    ($monto <= 0.0m) ? {
        print("Error: Monto inválido")
        return false
    }

    // El código continúa en línea recta
    print("Procesando pago de: " . $monto)
    return true
}
```

---

## 3. Operadores Elvis `?:` y Coalescencia Nula `??`

Joss proporciona dos atajos muy potentes para asignar valores por defecto:

### El operador Elvis (`?:`)
Evalúa la expresión de la izquierda. Si es verdadera (o tiene un valor no vacío ni cero), devuelve esa expresión; si es falsa o vacía, devuelve el valor de la derecha:

```joss
$apodo = $aliasIngresado ?: "Anónimo"
```

### El operador de coalescencia nula (`??`)
Se enfoca exclusivamente en la existencia de un valor. Si la variable de la izquierda es `null` (o no está definida), devuelve la alternativa de la derecha:

```joss
$configuracion = $opcionUsuario ?? "valor_predeterminado"
```

---

## 4. Selección múltiple con `match`

Cuando una variable puede tener muchos valores posibles (por ejemplo, el estado de un envío, el rol de un usuario o el código de respuesta de un servidor), encadenar ternarios se vuelve difícil de leer.

Para estos casos, Joss ofrece la expresión **`match`**:

<!-- joss-run: ["En camino"] -->
```joss
$estado = "enviado"
$mensaje = match ($estado) {
    "nuevo" => "Preparando",
    "enviado", "reparto" => "En camino",
    default => "Consulta el pedido"
}
print($mensaje)
```

### Características de `match`:
- **Brazos múltiples**: Cada línea se compone de uno o más patrones, seguidos de una flecha gruesa `=>` y el valor o bloque resultante.
- **Soporte para bloques de sentencias**: Los brazos pueden contener bloques multilínea entre llaves `{ ... }` para ejecutar varias instrucciones consecutivas o actualizar el estado del programa:

<!-- joss-run: ["Opción 1 ejecutada"] -->
```joss
$opcion = 1
match ($opcion) {
    1 => {
        print("Opción 1 ejecutada")
    },
    default => {
        print("Opción por defecto")
    }
}
```
- **Agrupación con comas**: Puedes asociar varios valores al mismo resultado en una sola línea (por ejemplo `"enviado", "reparto"`).
- **Brazo por defecto (`default`)**: Cubre cualquier valor que no haya coincidido con los anteriores. Es una buena práctica incluirlo siempre para evitar resultados indefinidos.
- **Sin caída automática (no fall-through)**: A diferencia de los viejos `switch` de C o Java, `match` solo ejecuta el primer brazo que coincida y termina; no requiere palabras clave como `break`.

---

## 5. Bucles y estructuras de repetición

Un **bucle** (o ciclo) le indica a la computadora que ejecute un bloque de código una y otra vez mientras una condición permanezca verdadera.

### El ciclo `while` (repetir mientras)

El ciclo `while` evalúa la condición **antes** de entrar al cuerpo del ciclo. Si la condición es falsa desde el principio, el cuerpo ni siquiera se ejecuta una vez:

<!-- joss-run: ["1", "2", "3"] -->
```joss
$numero = 1
while ($numero <= 3) {
    print($numero)
    $numero++
}
```

> [!CAUTION]
> **Cuidado con los bucles infinitos**:
> Dentro del cuerpo de un `while`, siempre debe haber una instrucción que modifique las variables de la condición (como `$numero++`). Si la condición nunca se vuelve falsa, el programa se quedará atrapado para siempre consumiendo procesador hasta que lo detengas forzosamente en tu terminal (en Joss, puedes presionar la tecla `q` o `Ctrl + C`).

### El ciclo `do ... while` (hacer al menos una vez)

A diferencia de `while`, el ciclo `do ... while` ejecuta el cuerpo **primero** y comprueba la condición al final. Esto garantiza que las instrucciones se ejecutarán al menos una vez, sin importar la condición inicial:

<!-- joss-run: ["Intento 1"] -->
```joss
$intento = 0
do {
    $intento++
    print("Intento " . $intento)
} while ($intento < 1)
```

---

## 6. Recorrer colecciones y canales con `foreach`

Cuando tienes una lista de datos (como un `array` de nombres o productos), no necesitas gestionar manualmente un contador numérico: utilizas **`foreach`**.

<!-- joss-run: ["pan", "leche"] -->
```joss
$compras = ["pan", "leche"]
foreach ($compras as $producto) {
    print($producto)
}
```

En cada vuelta del ciclo, Joss toma el siguiente elemento de la colección `$compras`, lo deposita en la variable temporal `$producto` y ejecuta el bloque de código.

### Iteración sobre canales de concurrencia (`channel`)
Una característica distintiva de Joss es que `foreach` no solo sirve para recorrer listas estáticas en memoria: también puede consumir **canales de comunicación concurrente** (`channel`). El ciclo leerá mensajes del canal en tiempo real hasta que el canal sea cerrado con `close($canal)`.

---

## 7. Control de bucles: `break` y `continue`

Dentro de cualquier ciclo (`while`, `do...while` o `foreach`), puedes alterar el flujo de repetición con dos instrucciones fundamentales:

### `break`: Terminar el ciclo inmediatamente
Aborta la repetición y salta directamente a la primera línea que esté después del bucle:

<!-- joss-run: ["1"] -->
```joss
foreach ([1, 2, 3, 4] as $numero) {
    print($numero)
    break
}
```
En este ejemplo, solo se imprime el `1` porque la instrucción `break` cancela el ciclo de inmediato.

### `continue`: Saltar a la siguiente vuelta
Omite las instrucciones restantes del cuerpo del ciclo solo para la vuelta actual, avanzando a la siguiente iteración:

```joss
foreach ([1, 2, 3, 4, 5] as $n) {
    ($n % 2 == 0) ? {
        continue // Si es par, sáltatelo
    } : {}
    print("Impar: " . $n)
}
```

---

## 8. Errores comunes y buenas prácticas

| Error | Causa | Solución |
|---|---|---|
| Escribir `if ($x > 0)` | `if` no existe en la gramática de Joss. | Usa la sintaxis ternaria: `($x > 0) ? { ... } : { ... }`. |
| Olvidar el `:` en el ternario | Falta la rama falsa. | Si no tienes nada que hacer en la rama falsa, escribe `{}`: `($x > 0) ? { print("ok") } : {}`. |
| Bucle infinito en `while` | Olvidar incrementar o cambiar la variable de control. | Asegúrate de actualizar el contador o la bandera dentro del cuerpo del bucle. |
| `break` o `continue` fuera de un ciclo | Colocarlos en el nivel superior de un archivo o función. | Solo son válidos dentro de un `while`, `do...while` o `foreach`. |

---

## 9. Ejercicios prácticos

1. **Clasificador de notas**:
   - Declara una variable entera `$nota = 85`.
   - Usando un ternario con bloques, imprime:
     - "Sobresaliente" si `$nota >= 90`.
     - "Aprobado" si `$nota >= 60` y `$nota < 90`.
     - "Reprobado" si `$nota < 60`.
2. **Sumatoria con `foreach`**:
   - Crea un array `$precios = [10, 25, 5, 40]`.
   - Inicializa una variable `$total = 0`.
   - Recorre el array con `foreach`, sumando cada precio a `$total`.
   - Imprime el total final (debe dar `80`).

---

## Siguiente paso

Ahora que puedes tomar decisiones y repetir operaciones, aprenderemos a empaquetar bloques de lógica reutilizables con nombres propios, parámetros y valores de retorno:

Continúa con: [Funciones, ámbito de variables (scope), closures y referencias](FUNCIONES.md).

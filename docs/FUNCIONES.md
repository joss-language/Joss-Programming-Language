# Funciones, ámbito de variables, closures y referencias

Antes: [Control de flujo y decisiones](CONTROL_FLUJO.md). Después: [Colecciones: Arrays, Maps y Texto](COLECCIONES.md).
Referencia técnica: [Sintaxis](SINTAXIS.md), [Recursión](RECURSION.md), [Sistema de tipos](SISTEMA_TIPOS.md).

---

## ¿Qué vas a aprender aquí?

A medida que los programas crecen, escribir cientos de instrucciones seguidas se vuelve inmanejable. Si necesitas calcular el total de una factura con impuestos en diez lugares distintos de tu aplicación, copiar y pegar la misma fórmula diez veces es una receta para el desastre: si la ley tributaria cambia, tendrías que buscar y corregir diez archivos, y tarde o temprano olvidarás uno.

En esta guía aprenderás:
1. Qué es una **función** y por qué es el bloque fundamental de construcción de software.
2. La diferencia crítica entre **parámetros**, **argumentos** e **impresión** versus **retorno**.
3. Cómo declarar funciones con tipos seguros y valores predeterminados.
4. Las reglas de visibilidad obligatoria en Joss (`public` y `private`).
5. Qué es el **ámbito de variables (scope)** y cómo funciona el aislamiento de memoria en cada llamada.
6. Qué son las **funciones anónimas (closures)** y cómo capturan datos de su entorno.
7. Cómo modificar variables externas de forma segura mediante **referencias (`ref`)**.
8. Cómo encadenar transformaciones de datos limpias con el operador **pipeline (`|>`)**.

---

## 1. ¿Qué es una función?

Una **función** es un bloque autónomo de instrucciones al que le asignamos un nombre. Piensa en ella como una pequeña máquina especializada:
1. Recibe materias primas (datos de entrada, llamados **argumentos**).
2. Procesa la información en su interior de forma aislada.
3. Devuelve un producto terminado (el resultado, llamado **valor de retorno**).

<!-- joss-run: ["5", "12"] -->
```joss
public func sumar(int $a, int $b): int {
    return $a + $b
}
print(sumar(2, 3))
print(sumar(5, 7))
```

### Anatomía de una función en Joss:

- `public`: **Modificador de visibilidad**. En Joss, las funciones globales requieren obligatoriamente visibilidad:
  - `public`: La función está disponible en todo el proyecto y otros archivos pueden usarla sin imports.
  - `private`: La función solo puede ser llamada desde el mismo archivo donde fue escrita.
- `func`: Palabra clave que indica el inicio de la declaración (Joss no usa `function`).
- `sumar`: El nombre de la función.
- `(int $a, int $b)`: La lista de **parámetros**. Define qué tipos de datos exige la función para trabajar.
- `: int`: El **tipo de retorno**. Declara qué tipo de valor promete devolver la función a quien la llamó.
- `{ ... }`: El **cuerpo** de la función, donde se escriben las instrucciones.
- `return $a + $b`: La instrucción `return` finaliza la ejecución de la función de inmediato y envía el resultado de vuelta al llamador.

---

## 2. Diferencias conceptuales cruciales

### Parámetros vs Argumentos
- **Parámetro**: Es la variable que declaras en la firma de la función (por ejemplo, `$a` y `$b`). Es la ranura o espacio que espera un valor.
- **Argumento**: Es el valor concreto que entregas al momento de llamar a la función (por ejemplo, `2` y `3`).

### Retornar (`return`) vs Imprimir (`print`)
Este es uno de los tropiezos más comunes para quienes empiezan a programar:
- `print` es una acción física: escribe tinta en la pantalla de la terminal para que un humano lo lea. El resto del programa **no puede reutilizar esa salida**.
- `return` es una acción de memoria interna: le entrega el dato calculado a la línea que llamó a la función para que pueda seguir operando con él (guardarlo en una variable, guardarlo en una base de datos, enviarlo por internet, etc.).

---

## 3. Contratos de tipos y valores por defecto

En Joss, **todo parámetro fuente debe tener un tipo de dato explícito**. Si una función realmente necesita aceptar cualquier valor dinámico, debes indicarlo escribiendo `mixed`.

### Parámetros con valores predeterminados (Defaults)

Puedes hacer que ciertos argumentos sean opcionales asignándoles un valor por defecto con `=`:

<!-- joss-run: ["Hola, visitante", "Hola, Ada"] -->
```joss
public func saludo(string $nombre = "visitante"): string {
    return "Hola, " . $nombre
}
print(saludo())
print(saludo("Ada"))
```

- Si llamas a `saludo()` sin argumentos, Joss utilizará automáticamente `"visitante"`.
- Si llamas a `saludo("Ada")`, el valor entregado sustituye al valor por defecto.

> [!TIP]
> Coloca siempre los parámetros con valor por defecto al final de la lista de parámetros. De lo contrario, Joss no sabría a qué parámetro asignar un argumento si solo pasas uno.

### Argumentos nombrados (Named Arguments)

Puedes pasar argumentos indicando explícitamente el nombre del parámetro seguido de dos puntos (`nombre: valor`). Esto permite omitir parámetros intermedios que tengan valores por defecto o pasar argumentos en cualquier orden:

<!-- joss-run: ["Estimada Ada"] -->
```joss
public func bienvenida(string $nombre, string $titulo = "Estimado/a"): string {
    return $titulo . " " . $nombre
}

print(bienvenida(titulo: "Estimada", nombre: "Ada"))
```

### Operador Spread (`...`) en llamadas

Si tienes una lista y deseas desempaquetar sus elementos como argumentos posicionales independientes para una llamada a función, antepón `...`:

<!-- joss-run: ["10"] -->
```joss
public func sumarTres(int $a, int $b, int $c): int {
    return $a + $b + $c
}

$numeros = [2, 3, 5]
print(sumarTres(...$numeros))
```

---

## 4. Retorno temprano y cláusulas de guarda (Guard Clauses)

La instrucción `return` detiene la función inmediatamente. Si la colocas dentro de un bloque ternario, puedes verificar errores al principio de la función y salir antes de procesar el resto:

<!-- joss-run: ["agotado", "disponible"] -->
```joss
public func disponibilidad(int $cantidad): string {
    ($cantidad <= 0) ? { return "agotado" } : {}
    return "disponible"
}
print(disponibilidad(0))
print(disponibilidad(2))
```

A este patrón se le llama **cláusula de guarda**. Evita crear estructuras condicionales profundamente anidadas (*código espagueti*).

> [!IMPORTANT]
> **Exhaustividad de retorno (`JOSS-TYPE-010`)**:
> Si una función declara un tipo de retorno (como `: string`), el analizador semántico exige que **todas las rutas posibles** de ejecución terminen con un `return` del tipo indicado o con una excepción `throw`. No puedes dejar ramas inconclusas donde la función simplemente termine sin devolver nada.

---

## 5. Ámbito de variables (Scope) y paso por valor

El **ámbito (scope)** es la zona del programa donde una variable existe y es accesible.

En Joss:
1. Cada llamada a una función crea un **marco de memoria aislado (frame)**.
2. Los parámetros y las variables declaradas dentro de la función **solo existen mientras la función se está ejecutando**. En cuanto llega a `return`, desaparecen de la memoria.
3. Las funciones con nombre **no tienen acceso automático a las variables globales del archivo**. Si una función necesita un dato, debes pasárselo como parámetro.
4. **Paso por valor**: Al pasar un valor primitivo (como un `int` o `string`) a una función, Joss le entrega una copia. Modificar esa variable dentro de la función **no altera la variable original que estaba afuera**:

<!-- joss-run: ["20", "10"] -->
```joss
public func duplicar(int $valor): int {
    $valor = $valor * 2
    return $valor
}
$numero = 10
print(duplicar($numero))
print($numero)
```
La variable original `$numero` conserva su valor `10` intacto.

---

## 6. Funciones anónimas y Closures

Una función no siempre necesita un nombre global. Puedes crear una función como si fuera un valor, guardarla en una variable o pasarla como argumento a otra función. A esto se le llama **función anónima** o **función de primera clase**.

Cuando una función anónima utiliza variables que fueron creadas en el bloque exterior que la rodea, se convierte en una **closure**: "captura" y recuerda esas variables para utilizarlas más tarde, incluso si el bloque exterior ya terminó:

<!-- joss-run: ["Hola, Ada"] -->
```joss
$prefijo = "Hola, "
$saludar = func(string $nombre): string {
    return $prefijo . $nombre
}
print($saludar("Ada"))
```

- Las closures son muy utilizadas como **callbacks**: funciones que le entregas a un servicio (por ejemplo, a un servidor HTTP o un WebSocket) para que las ejecute cuando ocurra un evento (como la llegada de un cliente).
- Las closures no llevan modificadores de visibilidad (`public` ni `private`).

---

## 7. Referencias mutables temporales con `ref`

¿Qué pasa si realmente quieres que una función modifique la variable original que le pasaste desde afuera? En lugar de devolver una copia, Joss permite el uso de **referencias (`ref`)**.

Por seguridad, Joss exige que la intención sea **bilateral**: tanto la firma de la función como la llamada deben incluir la palabra reservada `ref`:

<!-- joss-run: ["2"] -->
```joss
public func incrementar(ref int $valor): int {
    $valor = $valor + 1
    return $valor
}
$contador = 1
incrementar(ref $contador)
print($contador)
```

Ahora `$contador` sí cambió su valor original a `2`.

### Reglas de seguridad de las referencias en Joss:
1. **Solo variables simples y mutables**: No puedes pasar constantes, expresiones matemáticas ni literales (por ejemplo, `incrementar(ref 5)` es ilegal con `JOSS-REF-002`).
2. **Invariancia estricta de tipos**: El tipo de la variable debe coincidir exactamente con el tipo del parámetro (no se permite pasar un `int` a un `ref float`).
3. **No pueden escapar**: Una referencia solo vive durante la llamada. No puedes guardar una referencia en una variable global, ni devolverla con `return`, ni enviarla a través de un canal asíncrono.

---

## 8. El operador Pipeline (`|>`)

En programación es muy común tener que pasar un dato a través de una serie de transformaciones sucesivas. En lenguajes clásicos, esto obliga a anidar llamadas de adentro hacia afuera:

```joss
// Difícil de leer: debes leer de derecha a izquierda o de adentro hacia afuera
strtoupper(trim("  ada  "))
```

Joss incluye de forma nativa el operador **Pipeline (`|>`)**, que toma el resultado de la izquierda y lo envía como primer argumento a la función de la derecha:

<!-- joss-run: ["ADA"] -->
```joss
print("  ada  " |> trim |> strtoupper)
```

El flujo se lee de forma natural de izquierda a derecha:
1. Toma el texto `"  ada  "`.
2. Pásalo por `trim` (que quita los espacios, resultando en `"ada"`).
3. Pásalo por `strtoupper` (que convierte a mayúsculas, resultando en `"ADA"`).
4. Pásalo a `print` para mostrarlo en pantalla.

Si la función receptora necesita más de un argumento, escríbelos normalmente entre paréntesis:
```joss
$resultado = $texto |> str_replace("a", "o")
```
Joss colocará el valor de la izquierda como el primer parámetro de `str_replace`.

---

## 9. Funciones recursivas

Una función **recursiva** es aquella que se llama a sí misma para resolver un problema dividiéndolo en versiones más pequeñas del mismo problema.

Toda función recursiva debe tener dos componentes indispensables:
1. **Caso base**: Una condición de parada donde la función retorna un resultado simple sin volverse a llamar.
2. **Paso recursivo**: Donde la función se llama a sí misma con un dato más cercano al caso base.

Joss protege tu memoria limitando la profundidad máxima de llamadas recursivas a 1024 niveles por defecto (consulta [Recursión](RECURSION.md) para más detalles).

---

## 10. Errores comunes con funciones

| Error | Código / Causa | Solución |
|---|---|---|
| Escribir `function f()` | `function` fue eliminada de Joss. | Usa `public func f()` o `private func f()`. |
| Olvidar `public` o `private` en funciones globales | Las funciones con nombre requieren visibilidad obligatoria. | Agrega `public func nombre(...)` o `private func nombre(...)`. |
| Parámetro sin tipo: `func($x)` | Los parámetros deben ser tipados (`JOSS-TYPE-011`). | Declara el tipo: `func(int $x)` o `func(mixed $x)`. |
| Función tipada sin return en todas las rutas | `JOSS-TYPE-010`: El checker detectó un camino que termina sin devolver nada. | Asegúrate de que todas las ramas ternarias retornen o lancen un error. |
| Olvidar `ref` en la llamada | Llamar `f($x)` cuando la función espera `ref T $param` (`JOSS-REF-001`). | Agrega `ref` en la llamada: `f(ref $x)`. |

---

## 11. Ejercicio práctico

1. **Calculadora con funciones**:
   - Escribe una función `public func calcularIVA(decimal $subtotal, decimal $tasa = 0.16m): decimal`.
   - La función debe retornar el monto del impuesto (`$subtotal * $tasa`).
   - Prueba llamarla con un solo argumento (`calcularIVA(100.0m)`) y luego con una tasa personalizada del 8% (`calcularIVA(100.0m, 0.08m)`).
   - Muestra ambos resultados en la consola.

---

## Siguiente paso

Ahora que sabes cómo estructurar código modular con funciones seguras, es momento de aprender a manipular colecciones de datos complejas: listas de elementos y mapas asociativos clave-valor.

Continúa con: [Colecciones: Arrays, Maps y Manipulación de Texto](COLECCIONES.md).

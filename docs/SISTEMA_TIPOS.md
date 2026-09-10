# Sistema de tipos, inferencia y conversiones

Antes: [Colecciones: Arrays, Maps y Texto](COLECCIONES.md). Después: [Clases y objetos](CLASES.md).
Referencia técnica: [Diagnósticos](DIAGNOSTICOS.md), [Sintaxis](SINTAXIS.md).

---

## ¿Qué vas a aprender aquí?

Un **sistema de tipos** es el conjunto de reglas que gobierna cómo una computadora interpreta los unos y ceros en la memoria. Sin tipos, una secuencia de 64 bits en la memoria podría ser un número, una letra, una imagen o una instrucción de procesador; no habría forma de saberlo.

En Joss, el sistema de tipos cumple un doble propósito:
1. **Seguridad y robustez**: Detecta incoherencias (como intentar multiplicar un texto por una lista) antes de que el código se ejecute en producción.
2. **Claridad documental**: Ayuda a cualquier desarrollador a comprender de inmediato qué datos espera una función y qué resultado va a producir.

En esta guía aprenderás:
1. La lista canónica de tipos de datos en Joss y por qué ciertos nombres antiguos (*aliases*) ya no son válidos.
2. Cómo funciona la **inferencia de tipos** y las diferencias entre `$x = ...`, `var`, `mixed` y declaraciones explícitas.
3. El funcionamiento de los **tipos unión** (`T|U`) y valores opcionales anulables (`T?`).
4. Cómo funciona la compatibilidad y la ampliación de números (`int → float → decimal`).
5. Colecciones tipadas con genéricos (`array<T>` y `map<K, V>`).
6. La regla de coerción de cadenas (`typesystem.CoerceString`) y las protecciones aritméticas de seguridad.

---

## 1. Inventario de tipos fuente canónicos

La fuente de verdad canónica del compilador (`pkg/typesystem/types.go`) reconoce los siguientes tipos válidos:

| Tipo fuente | Significado | Representación física en el Runtime | Ejemplo de uso |
|---|---|---|---|
| `int` | Entero con signo de 64 bits | `int64` (−9,223,372,036,854,775,808 a 9,223,372,036,854,775,807) | `int $id = 101` |
| `float` | Punto flotante binario estándar IEEE-754 | `float64` de 64 bits | `float $ratio = 0.75` |
| `decimal` | Número decimal de coma fija en base diez | `shopspring/decimal.Decimal` (alta precisión) | `decimal $precio = 99.99m` |
| `string` | Secuencia de caracteres UTF-8 | `string` de Go con soporte para grafemas | `string $email = "ada@joss.red"` |
| `bool` | Valor de verdad lógico | `bool` (`true` o `false`) | `bool $valido = true` |
| `array` | Secuencia dinámica de elementos | `[]interface{}` (slice de Go) | `array $items = [1, 2, 3]` |
| `map` | Tabla asociativa con claves de texto | `map[string]interface{}` (hashmap de Go) | `map $datos = {"rol": "admin"}` |
| `object` | Instancia genérica de una clase | Instancia de clase de usuario o nativa | `object $instancia = new Persona()` |
| `channel` | Canal de comunicación concurrente | `*core.Channel` (canal Go de mensajes) | `channel $c = make_chan(1)` |
| `mixed` | Dinamismo explícito y polimorfismo | Cualquier valor válido del runtime | `mixed $dato = "dinamico"` |
| `null` / `nil` | Ausencia de valor | Representación `nil` | `null` |
| Nombre de clase | Tipo nominal definido por el usuario o nativo | Instancia de la clase correspondiente | `Persona $p = new Persona()` |

### Los antiguos aliases eliminados

> [!WARNING]
> En versiones antiguas del lenguaje existían nombres alternativos heredados de otros ecosistemas como `integer`, `double`, `boolean`, `dynamic`, `any` o `list`. **Estos nombres fueron completamente eliminados de la gramática**.
>
> Si escribes `integer $x = 10`, el analizador no lo convertirá a `int`; buscará una clase de usuario llamada `integer`. Al no encontrarla, emitirá el error de diagnóstico `JOSS-TYPE-009` (Tipo no resuelto). Utiliza siempre los nombres canónicos: `int`, `float`, `bool`, `mixed` y `array`.

---

## 2. Inferencia y formas de declarar variables

Joss combina la agilidad de los lenguajes dinámicos con la seguridad de los lenguajes fuertemente tipados:

<!-- joss-run: ["30", "Ada", "pendiente"] -->
```joss
var $edad = 20
$edad = 30
string $nombre = "Ada"
mixed $resultado = 10
$resultado = "pendiente"
print($edad)
print($nombre)
print($resultado)
```

### Reglas semánticas de declaración:

1. **Inferencia por asignación simple (`$x = 20`) o con `var` (`var $x = 20`)**:
   - En la primera asignación, Joss inspecciona el valor y fija el tipo concreto (en este caso `int`).
   - Las asignaciones posteriores **deben ser compatibles** con ese tipo. Si intentas meter un texto, el analizador rechazará el programa con `JOSS-TYPE-001`.
2. **Declaración explícita (`string $nombre = "Ada"`)**:
   - Fija el tipo de forma visible y documentada.
3. **Dinamismo voluntario (`mixed $resultado = 10`)**:
   - Le indica al analizador que esta variable cambiará de naturaleza a lo largo del tiempo. Puedes reasignarle un texto, un mapa o una clase sin errores.
   - `let $resultado = 10` es un atajo sintáctico que produce exactamente una variable `mixed`.
4. **Inicialización con `null`**:
   - Si escribes `$x = null` sin tipo, la inferencia se **pospone** hasta la primera asignación que contenga un valor concreto.

---

## 3. Tipos Unión (`T|U`) y tipos anulables (`T?`)

En el desarrollo real es muy común que una operación pueda devolver un dato concreto o bien `null` si no se encontró nada (por ejemplo, buscar un usuario en la base de datos).

Para estos casos, Joss ofrece **tipos unión**:

<!-- joss-run: ["10", "A-10", "sin dato"] -->
```joss
int|string $id = 10
print($id)
$id = "A-10"
print($id)
int? $cantidad = null
print($cantidad ?? "sin dato")
```

### Reglas de los tipos unión:

1. **Sintaxis con barra vertical (`|`)**: `int|string` significa que la variable solo aceptará enteros o textos, pero rechazará booleanos o listas.
2. **El atajo de interrogación (`?`)**: Escribir `int?` es exactamente equivalente a escribir `int|null`. El AST del compilador lo normaliza automáticamente a una unión con `null`.
3. **Refinamiento de tipos (Narrowing) en ternarios**:
   Si tienes una variable `string? $nombre` y preguntas `($nombre != null)`, dentro de la rama verdadera el analizador sabe que `$nombre` ya no puede ser nulo, permitiéndote acceder a sus operaciones de texto de forma segura.

---

## 4. Reglas de compatibilidad y asignación

¿Cuándo puede un valor de tipo origen asignarse a una variable de tipo destino?

```text
       int ──────────► float ──────────► decimal
(Exacto 64 bits)    (Binario IEEE)     (Base 10 exacta)
```

1. **Mismo tipo**: Siempre permitido.
2. **`int → float`**: Permitido automáticamente. Un entero puede promoverse a flotante.
3. **`int → decimal` o `float → decimal`**: Permitido automáticamente. Joss convierte el valor a la representación decimal exacta.
4. **`Clase → object`**: Cualquier instancia de clase es compatible con el tipo universal `object`.
5. **`Subclase → Superclase`**: Una clase derivada que extiende a una clase base es aceptada donde se espere la clase base.
6. **`Clase → Interfaz`**: Una clase que implementa una interfaz (`implements`) es compatible donde se declare dicha interfaz como tipo.
7. **`mixed`**: Es universalmente compatible en ambas direcciones.

Cualquier otra mezcla (como intentar meter un `string` en un `int` o un `bool` en un `array`) será bloqueada por el analizador con `JOSS-TYPE-001` (Type Mismatch).

---

## 5. Colecciones tipadas (Genéricos de primer nivel)

Aunque Joss no tiene plantillas genéricas complejas en funciones de usuario, sí permite parametrizar las dos estructuras de datos principales:

<!-- joss-run: ["6", "2"] -->
```joss
array<int> $cantidades = [2, 4, 6]
map<string, int> $inventario = {"pan": 2}
print($cantidades[2])
print($inventario["pan"])
```

- `array<T>`: Un array donde todos los elementos deben ser de tipo `T`.
- `map<K, V>`: Un mapa con claves de tipo `K` (deben ser `string`) y valores de tipo `V`.

Al indexar una colección parametrizada (por ejemplo `$cantidades[0]`), el analizador infiere de inmediato que el resultado es de tipo `int`, garantizando la seguridad en el resto del código.

---

## 6. Enumeraciones (`enum`)

Las **enumeraciones** permiten definir un tipo cerrado con un conjunto finito de casos posibles, evitando el uso de constantes dispersas o strings mágicos.

En Joss existen dos tipos de enumeraciones:

### Enums puros (Unit Enums)
Cada caso representa un valor simbólico único con la propiedad `->name`:

<!-- joss-run: ["Pendiente", "Aprobado"] -->
```joss
public enum Estado {
    case Pendiente
    case Aprobado
    case Rechazado
}

$e = Estado::Pendiente
print($e->name)
$e2 = Estado::Aprobado
print($e2->name)
```

### Enums respaldados (Backed Enums)
Asocian cada caso a un valor escalar primitivo (`string` o `int`):

<!-- joss-run: ["admin", "admin", "Admin", "3"] -->
```joss
public enum Rol: string {
    case Admin = "admin"
    case Editor = "editor"
    case Lector = "lector"
}

$r = Rol::Admin
print($r->value)

// Instanciar desde valor escalar con from() o tryFrom()
$desdeValor = Rol::from("admin")
print($desdeValor->value)
print($desdeValor->name)

// Obtener todos los casos con cases()
$todos = Rol::cases()
print(count($todos))
```

---

## 7. Operadores de comprobación de tipo: `is` e `instanceof`

El operador **`is`** (y su alias **`instanceof`**) permite consultar en tiempo de ejecución si un valor pertenece a un tipo primitivo (`int`, `string`, `bool`, etc.), una clase o una interfaz:

<!-- joss-run: ["true", "true", "true"] -->
```joss
$numero = 42
$texto = "hola"
print($numero is int)
print($texto is string)
print($numero instanceof int)
```

---

## 8. Coerción textual tipada (`typesystem.CoerceString`)

En aplicaciones web, los datos que llegan desde formularios HTTP o peticiones JSON son cadenas de texto crudas (por ejemplo, `"8080"` o `"true"`).

Joss implementa una política compartida de coerción textual (`CoerceString`) tanto en el analizador estático como en el runtime:

<!-- joss-run: ["9000", "49.99", "true"] -->
```joss
int $puerto = "9000"
decimal $precio = "49.99"
bool $activo = "yes"
print($puerto)
print($precio)
print($activo)
```

### Tabla de entradas textuales aceptadas:

| Tipo destino | Cadenas de texto que Joss convierte automáticamente |
|---|---|
| `int` | Textos con dígitos enteros (`"9000"`, `"-42"`), o números flotantes sin parte fraccionaria (`"100.0"`). |
| `float` | Cualquier texto con notación decimal válida (`"3.1416"`, `"-0.05"`). |
| `decimal` | Textos numéricos limpios (`"49.99"`, `"120.50m"`). |
| `bool` | Acepta sin distinguir mayúsculas: `"true"`, `"1"`, `"yes"` (como `true`); y `"false"`, `"0"`, `"no"`, `""` (como `false`). |

Si el texto no se puede convertir (por ejemplo `int $x = "manzana"`), el analizador emite un error estático `JOSS-TYPE-002` o el runtime lo rechaza de forma defensiva.

---

## 9. Precisión numérica y defensas del runtime

| Regla de seguridad | Comportamiento en Joss | Diagnóstico |
|---|---|---|
| Desbordamiento entero | Las operaciones con enteros de 64 bits (`+`, `-`, `*`) se verifican contra overflow. Si superan los límites de 64 bits, la ejecución se interrumpe de inmediato. | `JOSS-ARITH-001` |
| División por cero | Dividir o calcular el resto de un número entre cero (`$n / 0` o `$n % 0`) produce un error controlado en vez de valores `NaN` o `Infinity` inesperados. | `JOSS-ARITH-002` |
| Índice fuera de rango | Acceder a un índice negativo o superior a la longitud de una lista o cadena detiene el programa de forma estructurada. | `JOSS-INDEX-001` |

---

## 10. Diagnósticos comunes del sistema de tipos

| Código | Significado | Solución habitual |
|---|---|---|
| `JOSS-TYPE-001` | Reasignación con tipo incompatible (ej. `$x = 1; $x = "hola"`). | Mantén el tipo homogéneo o declara la variable explícitamente como `mixed $x`. |
| `JOSS-TYPE-002` | Valor inicial incompatible con la anotación de tipo. | Corrige el valor inicial para que coincida con el tipo declarado. |
| `JOSS-TYPE-008` | El valor retornado por una función no coincide con el tipo prometido en `: Tipo`. | Revisa la expresión de `return` para que entregue el tipo prometido. |
| `JOSS-TYPE-009` | Nombre de tipo o clase inexistente (incluye aliases retirados como `integer`). | Reemplaza `integer`, `double`, `boolean`, `any` por sus nombres canónicos: `int`, `float`, `bool`, `mixed`. |
| `JOSS-TYPE-010` | Una función tipada puede terminar sin ejecutar un `return` o `throw`. | Asegúrate de que todas las ramas ternarias concluyan con un valor devuelto. |
| `JOSS-TYPE-011` | Se declaró un parámetro sin tipo (`func($x)`). | Escribe el tipo del parámetro: `func(int $x)` o `func(mixed $x)`. |

---

## Siguiente paso

Ahora que comprendes el sistema de tipos, las uniones y las conversiones seguras, daremos el siguiente paso hacia el modelado de dominio y la programación orientada a objetos:

Continúa con: [Clases, objetos, métodos y herencia](CLASES.md).

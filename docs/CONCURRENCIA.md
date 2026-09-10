# Concurrencia, operaciones asíncronas, Future y Canales

Antes: [Manejo de errores y excepciones](ERRORES.md). Después: [Proyecto práctico de consola](PROYECTO_CONSOLA.md).
Referencia técnica: [Módulos nativos](MODULOS_NATIVOS.md), [Arquitectura del runtime](ARQUITECTURA.md).

---

## ¿Qué vas a aprender aquí?

En el mundo físico, los seres humanos no hacemos una sola cosa estrictamente después de otra. Mientras la lavadora lava la ropa, tú puedes preparar la comida y escuchar música; no te quedas mirando fijamente la lavadora durante 40 minutos sin hacer nada más.

En la programación tradicional sincrónica, la computadora a menudo se queda "congelada" esperando:
- Espera a que un servidor remoto al otro lado del mundo responda a una consulta (500 milisegundos).
- Espera a que un disco duro lea un archivo grande (200 milisegundos).
- Espera a que se complete una consulta compleja a la base de datos.

Durante esa espera, el procesador está desperdiciando millones de ciclos de cálculo que podrían aprovecharse para atender a otros usuarios o procesar otros datos.

En esta guía aprenderás:
1. La diferencia conceptual entre operaciones **síncronas**, **asíncronas** y **concurrentes**.
2. Qué es un **`Future`** (promesa de un resultado futuro).
3. Cómo delegar tareas a segundo plano con la sintaxis **`async { ... }`**.
4. Qué significa conceptualmente **`await(...)`**, qué devuelve y cómo propaga errores.
5. Qué es un **canal (`channel`)**, cómo enviar y recibir datos entre tareas independientes y cómo consumirlos con `foreach`.
6. Cómo aísla Joss la memoria entre tareas mediante `Runtime.Fork()`.
7. Cuándo utilizar concurrencia y qué errores clásicos evitar.

---

## 1. Los tres conceptos esenciales

Para evitar confusiones habituales, distingamos con precisión tres términos que a menudo se mezclan:

1. **Síncrono (bloqueante)**: Cada instrucción espera obligatoriamente a que la anterior termine. Si la línea 1 tarda 5 segundos, la línea 2 no empieza hasta que pasen esos 5 segundos.
2. **Asíncrono (no bloqueante)**: Inicias una tarea que tomará tiempo, pero en lugar de quedarte esperando con los brazos cruzados, el programa continúa inmediatamente haciendo otras cosas útiles mientras la tarea trabaja en segundo plano.
3. **Concurrente**: Múltiples tareas están en progreso durante el mismo intervalo de tiempo, coordinándose y compartiendo recursos de forma ordenada.

---

## 2. Lanzar tareas en segundo plano: `async` y `*Future`

En Joss, cuando quieres que un bloque de código se ejecute en segundo plano sin detener el flujo principal, utilizas la construcción **`async { ... }`**:

<!-- joss-run: ["Preparando resultado", "42"] -->
```joss
$futuro = async {
    return 20 + 22
}
print("Preparando resultado")
$resultado = await($futuro)
print($resultado)
```

### ¿Qué ocurre paso a paso en este programa?

1. `$futuro = async { ... }`:
   - Joss toma el bloque de código y lo lanza a correr de forma concurrente en un hilo ligero gestionado por el sistema (una *goroutine* de Go).
   - Inmediatamente, la llamada devuelve un objeto especial llamado **`Future`**. Un `Future` no es el número `42` todavía; es un "ticket de reclamo" que representa un resultado que estará listo más adelante.
2. `print("Preparando resultado")`:
   - Esta línea se ejecuta de inmediato, **sin esperar** a que el bloque `async` termine de calcular su suma.
3. `$resultado = await($futuro)`:
   - Aquí interviene la función `await`. Conceptual y prácticamente significa:
     > **"Pausa la ejecución de esta línea hasta que la tarea en segundo plano termine, abre el ticket y deposita su resultado en `$resultado`"**.
   - Si la tarea ya había terminado, `await` entrega el resultado al instante sin demora.
4. `print($resultado)`:
   - Muestra el valor final `42`.

> [!NOTE]
> En Joss, `await` es una función nativa (`await($futuro)`), no una palabra reservada prefija. Puede utilizarse en cualquier parte del código: en el nivel superior de un archivo o dentro de cualquier función; no te exige declarar tus funciones como `async func`.

---

## 3. Ejecución paralela real: Lanzar primero, esperar después

Uno de los errores más comunes al empezar con asincronía es lanzar una tarea y esperar por ella en la línea inmediatamente siguiente:

```joss
// INCORRECTO si buscas paralelismo (se vuelve síncrono):
$a = await(async { return tarea1() })
$b = await(async { return tarea2() })
```
En el código anterior, `tarea2` nunca empieza hasta que `tarea1` haya terminado por completo.

Para lograr un verdadero beneficio de rendimiento cuando tienes tareas independientes (por ejemplo, consultar dos servicios web distintos o procesar dos imágenes), **debes lanzar todas las tareas primero y esperar sus resultados después**:

<!-- joss-run: ["30"] -->
```joss
$uno = async { return 10 }
$dos = async { return 20 }
$a = await($uno)
$b = await($dos)
print($a + $b)
```

Ahora, tanto `$uno` como `$dos` se ejecutan al mismo tiempo en núcleos de procesador separados. El tiempo total de espera será el de la tarea más lenta, no la suma de ambas.

---

## 4. Qué sucede si una tarea asíncrona falla

¿Qué ocurre si el código dentro del bloque `async` sufre un error o lanza una excepción con `throw`?

Joss no permite que tu programa colapse silenciosamente:
1. El `Future` captura internamente la excepción ocurrida.
2. En el momento en que tú invocas `await($futuro)`, el error **se vuelve a lanzar automáticamente** en el hilo principal.
3. Puedes capturar y solucionar ese fallo envolviendo el `await` dentro de un bloque `try / catch`:

```joss
$tarea = async {
    throw "Fallo al conectar con el servidor externo"
}

try {
    $resultado = await($tarea)
} catch ($e) {
    print("Error recuperado con éxito: " . $e)
}
```

---

## 5. Canales (`channel`): Comunicación segura entre tareas

Cuando dos tareas concurrentes necesitan pasarse mensajes continuamente (como una línea de ensamblaje donde un proceso descarga datos y otro los procesa), compartir variables globales mutables es muy peligroso porque pueden sobrescribirse y generar condiciones de carrera (*race conditions*).

La solución canónica y segura de Joss son los **canales (`channel`)**. Un canal es una tubería unidireccional: un extremo introduce datos y el otro extremo los extrae en estricto orden de llegada (FIFO).

<!-- joss-run: ["hola"] -->
```joss
$canal = make_chan(1)
send($canal, "hola")
print(recv($canal))
close($canal)
```

### Operaciones esenciales con canales:

1. `make_chan($capacidad)`:
   - Crea un nuevo canal. El argumento define el tamaño del **búfer** (cuántos mensajes pueden guardarse en la tubería antes de que quien envía tenga que detenerse a esperar a que alguien lea).
   - Si creas `make_chan(1)`, puedes depositar un mensaje sin esperar a que haya un receptor escuchando en ese preciso milisegundo.
   - Si creas `make_chan()` (sin argumentos o con `0`), es un canal sin búfer: el emisor se bloqueará hasta que el receptor esté listo para recibir el dato mano a mano.
2. `send($canal, $valor)` (o el operador `$canal << $valor`):
   - Envía un dato a través de la tubería.
3. `recv($canal)`:
   - Espera a que llegue un mensaje por el canal y lo extrae.
4. `close($canal)`:
   - Cierra el canal, avisando a todos los receptores que ya no se enviarán más datos.

---

## 6. Patrón Productor-Consumidor con `foreach`

Una de las características más elegantes del lenguaje es que puedes utilizar un bucle `foreach` ordinario para consumir todos los mensajes de un canal hasta que sea cerrado:

<!-- joss-run: ["10", "20"] -->
```joss
$canal = make_chan()
$productor = async {
    send($canal, 10)
    send($canal, 20)
    close($canal)
}
foreach ($canal as $valor) {
    print($valor)
}
await($productor)
```

### ¿Por qué funciona esto tan limpiamente?
1. El bloque `async` actúa como **productor**: envía `10`, luego `20` y finalmente avisa que terminó cerrando el canal con `close($canal)`.
2. El bucle `foreach` actúa como **consumidor**: espera pacientemente cada número, lo imprime y, en cuanto detecta que el canal fue cerrado y está vacío, el bucle termina de forma limpia y automática.

---

## 7. Multiplexación de canales con `select`

La sentencia **`select`** permite esperar y reaccionar ante múltiples operaciones de canales simultáneamente, ejecutando el primer caso que esté listo para completarse sin bloquear el hilo si se provee una cláusula `default:`:

- `case send($ch, $valor):` Intenta enviar un valor a un canal.
- `case recv($ch):` Espera recibir de un canal descartando el valor.
- `case $msg = recv($ch):` Recibe de un canal y asigna el valor recibido a una variable.
- `default:` Se ejecuta de inmediato si ninguno de los canales tiene operaciones listas (no bloqueante).

<!-- joss-run: ["recibido: listo"] -->
```joss
$ch = make_chan(1)
send($ch, "listo")

select {
    case $msg = recv($ch):
        print("recibido: " . $msg)
    default:
        print("sin mensajes")
}
```

---

## 8. Funciones generadoras y `yield`

Una **función generadora** permite producir una secuencia de valores perezosamente (*lazy evaluation*) bajo demanda, suspendiendo su ejecución tras cada `yield` y reanudándola exactamente en ese punto cuando se le solicita el siguiente elemento.

Joss soporta:
- `yield $valor`: Emite un valor.
- `yield $clave => $valor`: Emite un par clave-valor.
- Consumo directo mediante un bucle `foreach`.
- Inspección manual mediante métodos de la instancia devuelta: `->current()`, `->next()`, `->key()`, `->valid()`.

<!-- joss-run: ["0: 10", "1: 20", "2: 30"] -->
```joss
public func contar(): mixed {
    yield 10
    yield 20
    yield 30
}

$gen = contar()
foreach ($gen as $k => $v) {
    print($k . ": " . $v)
}
```

---

## 9. El modelo de aislamiento de memoria: `Runtime.Fork()`

Muchos lenguajes sufren de errores oscuros de concurrencia cuando dos tareas modifican las mismas variables al mismo tiempo.

Joss previene esto en su arquitectura interna:
- Cada vez que ejecutas `async { ... }`, el motor realiza una operación de bifurcación controlada (`Runtime.Fork()`).
- Esto **copia las variables locales, tipos y constantes** para la nueva tarea, asegurando que la tarea en segundo plano no corrompa los nombres de quien la lanzó.
- Los recursos que legítimamente deben compartirse (como las conexiones activas a bases de datos y los canales `channel`) se mantienen accesibles para la coordinación.

---

## 10. Tareas periódicas: Cron

Para operaciones que deben repetirse periódicamente en el tiempo (como limpiar sesiones inactivas cada medianoche o generar reportes cada hora), Joss incluye la clase nativa `Cron`:

```joss
Cron::schedule("limpieza_diaria", "0 0 * * *", {
    print("Ejecutando limpieza programada del sistema...")
})
```

`Cron::schedule` acepta expresiones estándar de 5 campos cron o atajos comunes como `hourly`, `daily`, `weekly` o `monthly`.

---

## 11. Buenas prácticas y errores comunes

| Situación | Qué debes hacer | Qué debes evitar |
|---|---|---|
| Múltiples tareas independientes | Lanza todas con `async` primero y haz `await` al final. | Hacer `await` inmediatamente después de cada `async`. |
| Comunicación entre tareas | Usa canales (`make_chan`, `send`, `recv`) o el valor devuelto por `return`. | Modificar variables globales compartidas desde distintos hilos. |
| Canales sin búfer en un solo hilo | Si no usas `async`, dale al menos tamaño 1 (`make_chan(1)`). | Usar `make_chan()` y llamar a `send` antes de `recv` en el mismo hilo (provocará un bloqueo permanente o *deadlock*). |
| Fin de transmisión en canales | El productor debe llamar siempre a `close($canal)` al terminar de emitir datos. | Dejar un canal abierto indefinidamente si un `foreach` está esperando por él. |

---

## 12. Ejercicio práctico

1. **Simulador de descargas paralelas**:
   - Crea una función que simule descargar tres archivos:
     ```joss
     $f1 = async { return "archivo1.png descargado" }
     $f2 = async { return "archivo2.pdf descargado" }
     $f3 = async { return "archivo3.zip descargado" }
     ```
   - Espera los tres resultados con `await` e imprime cada uno.
2. **Cola de tareas con canal**:
   - Crea un canal con búfer para 3 elementos: `$cola = make_chan(3)`.
   - Envía tres tareas: `"enviar_correo"`, `"generar_pdf"`, `"actualizar_stock"`.
   - Cierra el canal.
   - Recorre el canal con `foreach` e imprime `"Procesando: " . $tarea`.

---

## Siguiente paso

¡Felicidades! Has completado el aprendizaje de todos los fundamentos del lenguaje Joss: tipos, estructuras de control, funciones, colecciones, programación orientada a objetos, excepciones y concurrencia.

Ahora pondremos todo este conocimiento en práctica construyendo proyectos reales paso a paso:

Continúa con: [Construir un proyecto de consola completo](PROYECTO_CONSOLA.md).

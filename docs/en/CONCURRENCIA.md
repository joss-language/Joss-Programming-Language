# Concurrency, Asynchronous Operations, Future, and Channels

Previous: [Classes, objects, and inheritance](CLASES.md). Next: [Practical console project](PROYECTO_CONSOLA.md).
Technical reference: [Native modules](MODULOS_NATIVOS.md), [Runtime architecture](ARQUITECTURA.md).

---

## What will you learn here?

In the physical world, human beings do not do one single thing strictly after another. While the washing machine washes clothes, you can prepare food and listen to music; you do not stare at the washing machine for 40 minutes doing nothing else.

In traditional synchronous programming, the computer often stays "frozen" waiting:
- It waits for a remote server on the other side of the world to respond to a query (500 milliseconds).
- It waits for a hard drive to read a large file (200 milliseconds).
- It waits for a complex database query to complete.

During that wait, the processor is wasting millions of calculation cycles that could be used to serve other users or process other data.

In this guide you will learn:
1. The conceptual difference between **synchronous**, **asynchronous**, and **concurrent** operations.
2. What a **`Future`** (promise of a future result) is.
3. How to delegate tasks to the background with the **`async { ... }`** syntax.
4. What **`await`** means conceptually, what it returns, and how it propagates errors.
5. What a **channel (`channel`)** is, how to send and receive data between independent tasks, and how to consume them with `foreach` and `select`.
6. How Joss isolates memory between tasks via `Runtime.Fork()`.
7. When to use concurrency and what classic pitfalls to avoid.

---

## 1. The Three Essential Concepts

To avoid common confusion, let's precisely distinguish three terms that are often mixed up:

1. **Synchronous (blocking)**: Each instruction strictly waits for the previous one to finish. If line 1 takes 5 seconds, line 2 does not start until those 5 seconds have elapsed.
2. **Asynchronous (non-blocking)**: You start a task that will take time, but instead of standing idly by waiting, the program immediately continues doing other useful things while the task works in the background.
3. **Concurrent**: Multiple tasks are in progress during the same time interval, coordinating and sharing resources in an orderly fashion.

---

## 2. Launching Background Tasks: `async` and `Future`

In Joss, when you want a block of code to run in the background without stopping the main flow, you use the **`async { ... }`** construct:

<!-- joss-run: ["Preparando resultado", "42"] -->
```joss
var $futuro = async {
    return 20 + 22
}
print("Preparando resultado")
var $resultado = await($futuro)
print($resultado)
```

Terminal output:

```text
Preparando resultado
42
```

### What happens step by step in this program?

1. `var $futuro = async { ... }`:
   - Joss takes the code block and launches it to run concurrently on a lightweight thread managed by the system (a Go *goroutine*).
   - Immediately, the call returns a special object called a **`Future`**. A `Future` is not the number `42` yet; it is a "claim ticket" representing a result that will be ready later.
2. `print("Preparando resultado")`:
   - This line executes immediately, **without waiting** for the `async` block to finish calculating its sum.
3. `var $resultado = await($futuro)`:
   - Here `await` steps in. Conceptually and practically it means:
     > **"Pause execution at this line until the background task finishes, redeem the ticket, and place its result into `$resultado`"**.
   - If the task had already finished, `await` delivers the result instantly without delay.
4. `print($resultado)`:
   - Displays the final value `42`.

> [!NOTE]
> In Joss, `await` is a language keyword and unary expression: you can write either `await($futuro)` or `await $futuro`. Furthermore, it can be used anywhere in the code (at the top level or inside any function) without forcing you to mark your functions as `async func`.

---

## 3. True Parallel Execution: Launch First, Await Later

One of the most common mistakes when starting with asynchronous code is launching a task and waiting for it on the very next line:

```joss
// INCORRECTO si buscas paralelismo (se vuelve síncrono):
var $a = await(async { return tarea1() })
var $b = await(async { return tarea2() })
```
In the code above, `tarea2` never starts until `tarea1` has completely finished.

To achieve a real performance benefit when you have independent tasks (for example, querying two different web services or processing two images), **you must launch all tasks first and await their results afterward**:

<!-- joss-run: ["30"] -->
```joss
var $uno = async { return 10 }
var $dos = async { return 20 }
var $a = await($uno)
var $b = await($dos)
print($a + $b)
```

Terminal output:

```text
30
```

Now, both `$uno` and `$dos` run at the same time on separate processor cores. The total waiting time will be that of the slowest task, not the sum of both.

---

## 4. What Happens If an Asynchronous Task Fails

What happens if the code inside the `async` block encounters an error or throws an exception with `throw`?

Joss does not allow your program to crash silently:
1. The `Future` internally captures the exception that occurred.
2. The moment you invoke `await($futuro)`, the error **is automatically re-thrown** on the main thread.
3. You can catch and handle that failure by wrapping the `await` inside a `try / catch` block:

<!-- joss-run: ["[ASYNC PANIC] Fallo al conectar con el servidor externo", "Error recuperado con éxito: Fallo al conectar con el servidor externo"] -->
```joss
var $tarea = async {
    throw "Fallo al conectar con el servidor externo"
}

try {
    var $resultado = await($tarea)
} catch ($e) {
    print("Error recuperado con éxito: " . $e)
}
```

Terminal output:

```text
[ASYNC PANIC] Fallo al conectar con el servidor externo
Error recuperado con éxito: Fallo al conectar con el servidor externo
```

---

## 5. Channels (`channel`): Safe Communication Between Tasks

When two concurrent tasks need to pass messages to each other continuously (like an assembly line where one process downloads data and another processes it), sharing mutable global variables is very dangerous because they can overwrite each other and cause *race conditions*.

The canonical and safe solution in Joss is **channels (`channel`)**. A channel is a one-way pipe: one end puts data in and the other end pulls it out in strict first-in, first-out (FIFO) order.

<!-- joss-run: ["hola"] -->
```joss
var $canal = make_chan(1)
send($canal, "hola")
print(recv($canal))
close($canal)
```

Terminal output:

```text
hola
```

### Essential channel operations:

1. `make_chan($capacidad)`:
   - Creates a new channel. The argument defines the **buffer** size (how many messages can be stored in the pipe before the sender has to pause and wait for someone to read).
   - If you create `make_chan(1)`, you can deposit one message without waiting for a receiver to be listening at that exact millisecond.
   - If you create `make_chan()` (with no arguments or with `0`), it is an unbuffered channel: the sender will block until the receiver is ready to take the data hand-to-hand.
2. `send($canal, $valor)` (or the `$canal << $valor` operator):
   - Sends a piece of data through the pipe.
3. `recv($canal)`:
   - Waits for a message to arrive through the channel and extracts it.
4. `close($canal)`:
   - Closes the channel, notifying all receivers that no more data will be sent.

---

## 6. Producer-Consumer Pattern with `foreach`

One of the most elegant features of the language is that you can use an ordinary `foreach` loop to consume all messages from a channel until it is closed:

<!-- joss-run: ["10", "20"] -->
```joss
var $canal = make_chan()
var $productor = async {
    send($canal, 10)
    send($canal, 20)
    close($canal)
}
foreach ($canal as $valor) {
    print($valor)
}
await($productor)
```

Terminal output:

```text
10
20
```

### Why does this work so cleanly?
1. The `async` block acts as the **producer**: it sends `10`, then `20`, and finally signals that it is done by closing the channel with `close($canal)`.
2. The `foreach` loop acts as the **consumer**: it patiently waits for each number, prints it, and as soon as it detects that the channel has been closed and is empty, the loop finishes cleanly and automatically.

---

## 7. Channel Multiplexing with `select`

The **`select`** statement allows waiting on and reacting to multiple channel operations simultaneously, executing the first case that is ready to complete without blocking the thread if a `default:` clause is provided:

- `case send($ch, $valor):` Attempts to send a value to a channel.
- `case recv($ch):` Waits to receive from a channel, discarding the value.
- `case $msg = recv($ch):` Receives from a channel and assigns the received value to a variable.
- `default:` Executes immediately if none of the channels have operations ready (non-blocking).

<!-- joss-run: ["recibido: listo"] -->
```joss
var $ch = make_chan(1)
send($ch, "listo")

select {
    case $msg = recv($ch):
        print("recibido: " . $msg)
    default:
        print("sin mensajes")
}
```

Terminal output:

```text
recibido: listo
```

---

## 8. Generator Functions and `yield`

A **generator function** allows producing a sequence of values lazily (*lazy evaluation*) on demand, suspending its execution after each `yield` and resuming at that exact point when the next element is requested.

Joss supports:
- `yield $valor`: Emits a value.
- `yield $clave => $valor`: Emits a key-value pair.
- Direct consumption via a `foreach` loop.
- Manual inspection via methods on the returned instance: `->current()`, `->next()`, `->key()`, `->valid()`.

<!-- joss-run: ["0: 10", "1: 20", "2: 30"] -->
```joss
public func contar(): mixed {
    yield 10
    yield 20
    yield 30
}

var $gen = contar()
foreach ($gen as $k => $v) {
    print($k . ": " . $v)
}
```

Terminal output:

```text
0: 10
1: 20
2: 30
```

---

## 9. The Memory Isolation Model: `Runtime.Fork()`

Many languages suffer from obscure concurrency bugs when two tasks modify the same variables at the same time.

Joss prevents this in its internal architecture:
- Every time you execute `async { ... }`, the engine performs a controlled fork operation (`Runtime.Fork()`).
- This **copies local variables, types, and constants** for the new task, ensuring that the background task does not corrupt the bindings of the caller that launched it.
- Resources that legitimately need to be shared (such as active database connections and `channel` instances) remain accessible for coordination.

---

## 10. Periodic Tasks: Cron

For operations that must repeat periodically over time (such as cleaning up inactive sessions every midnight or generating reports every hour), Joss includes the native `Cron` class:

```joss
Cron::schedule("limpieza_diaria", "0 0 * * *", {
    print("Ejecutando limpieza programada del sistema...")
})
```

`Cron::schedule` accepts standard 5-field cron expressions or common shortcuts such as `hourly`, `daily`, `weekly`, or `monthly`.

---

## 11. Best Practices and Common Mistakes

| Situation | What you should do | What you should avoid |
|---|---|---|
| Multiple independent tasks | Launch all of them with `async` first and `await` at the end. | Calling `await` immediately after each `async`. |
| Communication between tasks | Use channels (`make_chan`, `send`, `recv`) or the value returned by `return`. | Modifying shared global variables from different threads. |
| Unbuffered channels on a single thread | If you are not using `async`, give it a size of at least 1 (`make_chan(1)`). | Using `make_chan()` and calling `send` before `recv` on the same thread (will cause a permanent freeze or *deadlock*). |
| End of transmission on channels | The producer should always call `close($canal)` when finished emitting data. | Leaving a channel open indefinitely if a `foreach` is waiting on it. |

---

## 12. Practical Exercises

### Level 1 — Quick check
Launch a task with `var $f = async { return "Tarea completada" }`, print `"Esperando..."` first, and then print the result of `await($f)`.

### Level 2 — Guided exercise
1. Simulate downloading three files by launching three `async` blocks in parallel:
   ```joss
   var $f1 = async { return "archivo1.png descargado" }
   var $f2 = async { return "archivo2.pdf descargado" }
   var $f3 = async { return "archivo3.zip descargado" }
   ```
2. Await all three results with `await` and print each one to the console.

### Level 3 — Challenge
1. Create a buffered channel for 3 elements: `var $cola = make_chan(3)`.
2. Launch a block `var $productor = async { ... }` that sends three tasks (`"enviar_correo"`, `"generar_pdf"`, `"actualizar_stock"`) and closes the channel with `close($cola)`.
3. In the main flow, iterate over `$cola` with `foreach ($cola as $tarea)` and print `"Procesando: " . $tarea`.
4. When the loop finishes, wait for the producer with `await($productor)`.

---

## Next step

Congratulations! You have completed learning all the fundamentals of the Joss language: types, control structures, collections, functions, exceptions, object-oriented programming, and concurrency.

Now we will put all this knowledge into practice by building real projects step by step:

Continue with: [Building a complete console project](PROYECTO_CONSOLA.md).

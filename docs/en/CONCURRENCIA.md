# Concurrency, asynchronous operations, Future and Channels

Before: [Error and exception handling](ERRORES.md). After: [Hands-on console project](PROYECTO_CONSOLA.md).
Technical reference: [Native modules](MODULOS_NATIVOS.md), [Runtime architecture](ARQUITECTURA.md).

---

## What are you going to learn here?

In the physical world, human beings do not do one thing strictly after another. While the washing machine washes clothes, you can prepare food and listen to music; You don't stare at the washing machine for 40 minutes without doing anything else.

In traditional synchronous programming, the computer is often "frozen" waiting:
- Wait for a remote server on the other side of the world to respond to a query (500 milliseconds).
- Wait for a hard drive to read a large file (200 milliseconds).
- Waits for a complex database query to complete.

During this wait, the processor is wasting millions of calculation cycles that could be used to serve other users or process other data.

In this guide you will learn:
1. The conceptual difference between **synchronous**, **asynchronous** and **concurrent** operations.
2. What is a **`Future`** (promise of a future result).
3. How to delegate tasks to the background with the syntax **`async { ... }`**.
4. What **`await(...)`** conceptually means, what it returns and how it propagates errors.
5. What is a **channel (`channel`)**, how to send and receive data between independent tasks and how to consume it with `foreach`.
6. How Joss isolates memory between tasks using `Runtime.Fork()`.
7. When to use concurrency and what classic mistakes to avoid.

---

## 1. The three essential concepts

To avoid common confusion, let us precisely distinguish three terms that are often mixed:

1. **Synchronous (blocking)**: Each instruction necessarily waits for the previous one to finish. If line 1 takes 5 seconds, line 2 does not start until those 5 seconds have passed.
2. **Asynchronous (non-blocking)**: You start a task that will take time, but instead of waiting idly, the program immediately continues doing other useful things while the task works in the background.
3. **Concurrent**: Multiple tasks are in progress during the same time interval, coordinating and sharing resources in an orderly manner.

---

## 2. Launch background tasks: `async` and `*Future`

In Joss, when you want a block of code to run in the background without stopping the main flow, you use the **`async { ... }`** construct:

<!-- joss-run: ["Preparando resultado", "42"] -->
```joss
$futuro = async {
    return 20 + 22
}
print("Preparando resultado")
$resultado = await($futuro)
print($resultado)
```

### What happens step by step in this program?

1. `$futuro = async { ... }`:
   - Joss takes the block of code and runs it concurrently in a lightweight thread managed by the system (a Go goroutine).
   - Immediately, the call returns a special object called **`Future`**. A `Future` is not the number `42` yet; is a "claim ticket" that represents a result that will be ready later.
2. `print("Preparando resultado")`:
   - This line is executed immediately, **without waiting** for the `async` block to finish calculating its sum.
3. `$resultado = await($futuro)`:
   - Here the function `await` comes into play. Conceptually and practically it means:
     > **"Pause the execution of this line until the background task finishes, open the ticket and deposit its result at `$resultado`"**.
   - If the task had already finished, `await` delivers the result instantly without delay.
4. `print($resultado)`:
   - Shows the final value `42`.

> [!NOTE]
> In Joss, `await` is a native function (`await($futuro)`), not a prefix reserved word. It can be used anywhere in the code: at the top level of a file or within any function; it does not require you to declare your functions like `async func`.

---

## 3. Real parallel execution: Launch first, wait later

One of the most common mistakes when starting with asynchrony is launching a task and waiting for it on the immediately following line:

```joss
// INCORRECTO si buscas paralelismo (se vuelve síncrono):
$a = await(async { return tarea1() })
$b = await(async { return tarea2() })
```
In the code above, `tarea2` never starts until `tarea1` has completely finished.

To achieve a true performance benefit when you have independent tasks (for example, querying two different web services or processing two images), **you should launch all tasks first and wait for their results afterwards**:

<!-- joss-run: ["30"] -->
```joss
$uno = async { return 10 }
$dos = async { return 20 }
$a = await($uno)
$b = await($dos)
print($a + $b)
```

Now both `$uno` and `$dos` run concurrently on separate processor cores. The total waiting time will be that of the slowest task, not the sum of both.

---

## 4. What happens if an asynchronous task fails

What happens if the code inside the `async` block fails or throws an exception with `throw`?

Joss doesn't let your program crash silently:
1. The `Future` internally captures the exception that occurred.
2. The moment you invoke `await($futuro)`, the error is **automatically re-thrown** in the main thread.
3. You can catch and fix that bug by wrapping the `await` inside a `try / catch` block:

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

## 5. Channels (`channel`): Secure communication between tasks

When two concurrent tasks need to continually pass messages to each other (such as an assembly line where one process downloads data and another processes it), sharing mutable global variables is very dangerous because they can overwrite each other and generate race conditions.

The canonical and secure Joss solution is **channels (`channel`)**. A channel is a unidirectional pipe: one end brings data in and the other end extracts it in strict first-come, first-served (FIFO) fashion.

<!-- joss-run: ["hola"] -->
```joss
$canal = make_chan(1)
send($canal, "hola")
print(recv($canal))
close($canal)
```

### Essential operations with channels:

1. `make_chan($capacidad)`:
   - Create a new channel. The argument defines the size of the **buffer** (how many messages can be stored in the pipeline before the sender has to stop and wait for someone to read).
   - If you create `make_chan(1)`, you can deposit a message without waiting for a receiver to listen at that precise millisecond.
   - If you create `make_chan()` (without arguments or with `0`), it is an unbuffered channel: the sender will be blocked until the receiver is ready to receive the hand-to-hand data.
2. `send($canal, $valor)` (or the operator `$canal << $valor`):
   - Sends a data through the pipe.
3. `recv($canal)`:
   - Waits for a message to arrive through the channel and extracts it.
4. `close($canal)`:
   - Closes the channel, notifying all receivers that no more data will be sent.

---

## 6. Producer-Consumer Pattern with `foreach`

One of the most elegant features of the language is that you can use an ordinary `foreach` loop to consume all messages in a channel until it is closed:

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

### Why does this work so cleanly?
1. The `async` block acts as **producer**: it sends `10`, then `20` and finally warns that it ended up closing the channel with `close($canal)`.
2. The `foreach` loop acts as a **consumer**: it waits patiently for each number, prints it, and as soon as it detects that the channel has been closed and is empty, the loop ends cleanly and automatically.

---

## 7. Channel multiplexing with `select`

The **`select`** statement allows you to wait and react to multiple channel operations simultaneously, executing the first case that is ready to complete without blocking the thread if a `default:` clause is provided:

- `case send($ch, $valor):` Try to send a value to a channel.
- `case recv($ch):` Expect to receive from a channel by discarding the value.
- `case $msg = recv($ch):` Receives from a channel and assigns the received value to a variable.
- `default:` Executes immediately if none of the channels have operations ready (non-blocking).

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

## 8. Generating functions and `yield`

A **generator function** allows a sequence of values ​​to be produced lazily (*lazy evaluation*) on demand, suspending its execution after each `yield` and resuming it exactly at that point when the next element is requested.

Joss supports:
- `yield $valor`: Outputs a value.
- `yield $clave => $valor`: Emits a key-value pair.
- Direct consumption through a loop `foreach`.
- Manual inspection using methods of the returned instance: `->current()`, `->next()`, `->key()`, `->valid()`.

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

## 9. The memory isolation model: `Runtime.Fork()`

Many languages ​​suffer from obscure concurrency bugs when two tasks modify the same variables at the same time.

Joss prevents this in its internal architecture:
- Every time you run `async { ... }`, the engine performs a controlled branch operation (`Runtime.Fork()`).
- This **copies the local variables, types and constants** for the new task, ensuring that the background task does not corrupt the names of whoever launched it.
- Resources that legitimately should be shared (such as active database connections and `channel` channels) are kept accessible for coordination.

---

## 10. Periodic tasks: Cron

For operations that must be repeated periodically over time (such as cleaning inactive sessions every midnight or generating reports every hour), Joss includes the native class `Cron`:

```joss
Cron::schedule("limpieza_diaria", "0 0 * * *", {
    print("Ejecutando limpieza programada del sistema...")
})
```

`Cron::schedule` accepts standard 5-field cron expressions or common shortcuts such as `hourly`, `daily`, `weekly` or `monthly`.

---

## 11. Good practices and common mistakes

| Situation | What you should do | What you should avoid |
|---|---|---|
| Multiple independent tasks | Launch them all with `async` first and do `await` last. | Do `await` immediately after each `async`. |
| Communication between tasks | Use channels (`make_chan`, `send`, `recv`) or the value returned by `return`. | Modify shared global variables from different threads. |
| Unbuffered channels in a single thread | If you don't use `async`, give it at least size 1 (`make_chan(1)`). | Use `make_chan()` and call `send` before `recv` in the same thread (will cause a *deadlock*). |
| End of transmission on channels | The producer must always call `close($canal)` when finishing issuing data. | Leave a channel open indefinitely if a `foreach` is waiting for it. |

---

## 12. Practical exercise

1. **Parallel download simulator**:
   - Create a function that simulates downloading three files:
     ```joss
     $f1 = async { return "archivo1.png descargado" }
     $f2 = async { return "archivo2.pdf descargado" }
     $f3 = async { return "archivo3.zip descargado" }
     ```
   - Wait for the three results with `await` and print each one.
2. **Task queue with channel**:
   - Create a channel with buffer for 3 elements: `$cola = make_chan(3)`.
   - Submit three tasks: `"enviar_correo"`, `"generar_pdf"`, `"actualizar_stock"`.
   - Close the channel.
   - Scan the channel with `foreach` and print `"Procesando: " . $tarea`.

---

## Next step

Congratulations! You have completed learning all the fundamentals of the Joss language: types, control structures, functions, collections, object-oriented programming, exceptions and concurrency.

Now we will put all this knowledge into practice by building real projects step by step:

Continue with: [Build a complete console project](PROYECTO_CONSOLA.md).

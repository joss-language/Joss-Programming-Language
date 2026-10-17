# Classes, Objects, Methods, and Inheritance

Previous: [Error and exception handling](ERRORES.md). Next: [Concurrency, async operations, and channels](CONCURRENCIA.md).
Technical reference: [Type system](SISTEMA_TIPOS.md), [Native catalog](CATALOGO_NATIVO.md).

---

## What will you learn here?

As an application grows, having scattered variables on one side and loose functions on another can become chaotic:
- You might have a variable `$usuario_nombre`, another `$usuario_email`, another `$usuario_rol`.
- If you have 100 users, how do you keep each user's data together with the operations that belong to them (such as authenticating, changing password, or sending a notification)?

**Object-Oriented Programming (OOP)** solves this problem by packaging related data and behaviors into a single conceptual unit.

In this guide you will learn:
1. What a **class** (the design blueprint) is and what an **object** or **instance** (the real entity) is.
2. How to define **properties** (attributes) and **methods** (class functions).
3. How to initialize objects with **constructors** and `Init`.
4. The role of the special **`$this`** variable.
5. Visibility and encapsulation levels: `public`, `protected`, and `private`.
6. Static members and the scope resolution operator (`::`).
7. How to reuse and specialize code through **inheritance (`extends`)**, **interfaces (`implements`)**, and **abstract classes (`abstract`)**.
8. Generic classes (`<T>`), immutable `record` declarations, and the null-safe navigation operator (`?->`).

---

## 1. Classes and Objects: The Blueprint and the House

To understand object orientation, the best analogy is architecture:
- A **class** is the **architectural blueprint**: it describes what rooms the house will have, how many doors, and what features it includes. The blueprint does not occupy physical land and you cannot live in it.
- An **object** (or **instance**) is the **actual physical house built** on a plot of land from that blueprint. You can build ten houses from the same blueprint; painting one house blue does not change the color of the others.

Let's look at a minimal example with a counter:

<!-- joss-run: ["1", "2"] -->
```joss
public class Contador {
    public int $valor = 0

    public func incrementar(): int {
        $this->valor = $this->valor + 1
        return $this->valor
    }
}
var $contador = new Contador()
print($contador->incrementar())
print($contador->incrementar())
```

Terminal output:

```text
1
2
```

### What elements make up this code?

1. `public class Contador`: Declares a public class named `Contador`. In Joss, top-level file classes require a visibility modifier (`public` or `private`).
2. `public int $valor = 0`: Is a **property** (a piece of data that each instance of `Contador` will remember).
3. `public func incrementar(): int`: Is a **method** (a function that belongs to the class and can manipulate its properties).
4. `$this`: Is a reserved word that means "this particular object". When you execute `$this->valor`, you are accessing the `$valor` property of the instance currently running the method.
5. `new Contador()`: The `new` keyword creates a new real instance in memory.
6. `$contador->incrementar()`: The arrow operator `->` is used to access properties and methods of an instance.

---

## 2. Initializing Objects: Constructors

When you create an object, you almost always need to configure it with initial data (for example, a person's name or database credentials).

In Joss you can define a `public func constructor(...)` method or an `Init(...)` block:

<!-- joss-run: ["Hola, Ada"] -->
```joss
public class Persona {
    private string $nombre = ""

    public func constructor(string $nombre) {
        $this->nombre = $nombre
    }

    public func saludar(): string {
        return "Hola, " . $this->nombre
    }
}
var $persona = new Persona("Ada")
print($persona->saludar())
```

Terminal output:

```text
Hola, Ada
```

When writing `new Persona("Ada")`, Joss automatically calls the constructor, passing it the argument `"Ada"`, which is safely stored inside the private property `$this->nombre`.

> [!NOTE]
> Joss also supports the block syntax `Init(string $nombre) { ... }`. `Init` blocks do not take visibility modifiers (`public` or `private`). The historical combined form `Init constructor(...)` is still accepted for compatibility but emits the transition warning `JOSS-DECL-008`; prefer `public func constructor(...)` or `Init(...)`.

### Constructor Property Promotion

To avoid having to declare the property, receive the parameter, and write `$this->prop = $prop` manually, Joss allows declaring visibility (`public`, `protected`, or `private`) and constancy (`const`) directly on the parameters of `Init` or `constructor`. Joss will create and assign the property automatically:

<!-- joss-run: ["Ada", "30"] -->
```joss
public class Usuario {
    Init(
        public string $nombre,
        public int $edad = 30
    ) {}
}

var $u = new Usuario("Ada")
print($u->nombre)
print($u->edad)
```

Terminal output:

```text
Ada
30
```

It is also possible to declare promoted constant properties with `public const Tipo $campo` to protect them against subsequent reassignments.

---

## 3. Encapsulation and Visibility Modifiers

**Encapsulation** is the principle of protecting an object's internal data to prevent external code from modifying it in an incorrect or corrupted way.

Joss offers three explicit visibility modifiers:

| Modifier | Where it can be accessed | Recommended use |
|---|---|---|
| `public` | From **anywhere** in the program (inside the class, in subclasses, and from outside code). | For the public API of the object: methods that users of your class need to invoke. |
| `protected` | Only **inside the class itself** and **inside subclasses** that inherit from it with `extends`. | For internal methods and properties that child classes need to specialize or query. |
| `private` | **Only inside the exact class** where it was declared. Nobody else can see or modify it. | For intimate implementation details (passwords, raw connections, state flags). |

<!-- joss-run: ["150.5"] -->
```joss
public class CuentaBancaria {
    private decimal $saldo = 0.0m

    public func depositar(decimal $monto): void {
        ($monto > 0.0m) ? {
            $this->saldo = $this->saldo + $monto
        }
    }

    public func obtenerSaldo(): decimal {
        return $this->saldo
    }
}

var $cuenta = new CuentaBancaria()
$cuenta->depositar(150.50m)
print($cuenta->obtenerSaldo())
```

Terminal output:

```text
150.5
```

By making `$saldo` private, nobody can write `$cuenta->saldo = -5000.0m` from outside, guaranteeing that money is only modified under the rules of the `depositar` method.

---

## 4. Static Members and the `::` Operator

Not all properties or methods belong to an individual house; some operations belong to the general concept of the class or do not require creating an instance with `new`.

These elements are called **static** and are declared with the word `static` (remember that `static` always accompanies a visibility modifier such as `public static`):

<!-- joss-run: ["hola"] -->
```joss
public class Utilidades {
    public static func limpiarTexto(string $t): string {
        return trim($t)
    }
}

var $limpio = Utilidades::limpiarTexto("  hola  ")
print($limpio)
```

Terminal output:

```text
hola
```

To invoke a static method or read a static property, you do not use `->`, but the double colon operator **`::`**. In Joss, native system classes (such as `Auth::user()`, `GranDB::table()`, `Router::get()`, `Cache::put()`) are facades commonly invoked via `::`.

---

## 5. Inheritance with `extends`

**Inheritance** lets you create a new class based on an existing class, reusing all its public and protected methods and properties without having to rewrite them:

<!-- joss-run: ["hola"] -->
```joss
public class Mensaje {
    public func texto(): string { return "hola" }
}
public class Aviso extends Mensaje {}
var $aviso = new Aviso()
print($aviso->texto())
```

Terminal output:

```text
hola
```

- The `Mensaje` class is the **base class** (or superclass).
- The `Aviso` class is the **derived class** (or subclass).
- `Aviso` automatically inherits the `texto()` method from `Mensaje`.

> [!TIP]
> **When to use inheritance vs when to use composition**:
> Use inheritance only when there is a strict "is-a" relationship (for example, `Gato extends Animal` or `AdminUser extends User`). If you only want to reuse a utility function, do not use inheritance; use functions or inject a service class.

---

## 6. Interfaces and Polymorphism (`interface` and `implements`)

When working on modular applications or clean architecture, you often want to define **what** a component must do without tying yourself to **how** it does it.

An **interface** is a **formal contract**:
- It only declares public method prototypes (name, typed parameters, and return type) without a body.
- An interface cannot be instantiated directly with `new`.
- Any class that declares `implements NombreInterfaz` is **required by the semantic analyzer and runtime** to implement all promised methods with compatible signatures.
- A class can inherit from a base class and at the same time implement **multiple interfaces** separated by commas: `public class MiClase extends Base implements I1, I2`.
- An interface can extend one or more interfaces: `public interface IDerivada extends IBase1, IBase2`.

Let's look at an executable polymorphism example:

<!-- joss-run: ["50", "36"] -->
```joss
public interface IFigura {
    public func calcularArea(): int;
}

public class Rectangulo implements IFigura {
    public int $ancho = 0
    public int $alto = 0

    public func constructor(int $ancho, int $alto) {
        $this->ancho = $ancho
        $this->alto = $alto
    }

    public func calcularArea(): int {
        return $this->ancho * $this->alto
    }
}

public class Cuadrado implements IFigura {
    public int $lado = 0

    public func constructor(int $lado) {
        $this->lado = $lado
    }

    public func calcularArea(): int {
        return $this->lado * $this->lado
    }
}

public func imprimirArea(IFigura $figura): int {
    return $figura->calcularArea()
}

var $r = new Rectangulo(5, 10)
var $c = new Cuadrado(6)
print(imprimirArea($r))
print(imprimirArea($c))
```

Terminal output:

```text
50
36
```

### Advantages of Polymorphism with Interfaces:
1. **Decoupling**: The `imprimirArea(IFigura $figura)` function does not need to know whether it receives a `Rectangulo`, a `Cuadrado`, or any future shape; it only trusts that it fulfills the `IFigura` contract.
2. **Exhaustive static validation**: If you forget to implement an interface method in a class or its signature does not match, the semantic analyzer emits `JOSS-DECL-004` (or `JOSS-SEM-001` if an implemented method is not `public`).

---

## 7. Abstract Classes and Methods (`abstract`)

An **abstract class** (`public abstract class`) serves as a base template for other classes, but **cannot be instantiated directly** with `new` (attempting to do so emits diagnostic `JOSS-DECL-006`).

Abstract classes can contain:
- Complete properties and methods with implementations to be inherited.
- Abstract methods (`public abstract func nombre(...): Tipo`) that have no body and force concrete subclasses to implement them (omitting them emits `JOSS-DECL-005`).

<!-- joss-run: ["Guau!"] -->
```joss
public abstract class Animal {
    public abstract func hablar(): string
}

public class Perro extends Animal {
    public func hablar(): string {
        return "Guau!"
    }
}

var $perro = new Perro()
print($perro->hablar())
```

Terminal output:

```text
Guau!
```

---

## 8. Type and Instance Checking: `is` and `instanceof`

To check at runtime whether an object belongs to a concrete class, inherits from a base class, or implements an interface, use the equivalent **`is`** or **`instanceof`** operators:

<!-- joss-run: ["true", "true", "false"] -->
```joss
public interface IMovible {}
public class Auto implements IMovible {}

var $auto = new Auto()
print($auto is Auto)
print($auto is IMovible)
print($auto instanceof string)
```

Terminal output:

```text
true
true
false
```

You can also use `is` with primitive types such as `int`, `string`, `bool`, etc. (e.g., `$x is int`).

---

## 9. Null-Safe Navigation (`?->`)

If a variable can hold an instance or be `null` (type `Persona?`), trying to access a method with `->` on a null value would cause a failure.

Joss includes the **null-safe (`?->`)** operator:

<!-- joss-run: ["Sin usuario"] -->
```joss
public class Perfil {
    public func saludo(): string {
        return "Hola"
    }
}

Perfil? $usuario = null
var $nombre = $usuario?->saludo()
print($nombre ?? "Sin usuario")
```

Terminal output:

```text
Sin usuario
```

If `$usuario` is `null`, the call is silently and safely skipped, and `$nombre` will simply receive `null` without stopping the program.

---

## 10. Generic Classes (`class Contenedor<T>`) and Immutable Records (`record`)

### Generic classes
Joss supports formal type parameters `<T>` in class declarations:

```joss
public class Caja<T> {
    public T $contenido
    public func constructor(T $valor) {
        $this->contenido = $valor
    }
}
```

The semantic analyzer nominally validates generic parameters, ensuring that `Caja<int>` is not assignable to `Caja<string>`.

### Records (`public record Nombre(campos...)`)
A `record` is a concise declaration of an immutable data-oriented class with an automatic constructor and fields:

<!-- joss-run: ["10", "20"] -->
```joss
public record Coordenada(int $x, int $y)

var $c = new Coordenada(10, 20)
print($c->x)
print($c->y)
```

Terminal output:

```text
10
20
```

Fields declared in a `record` are sealed as constants (`const`) after object initialization, preventing any accidental reassignment (`JOSS-CONST-001`).

---

## 11. Lifecycle and Smart Protective Self-Destruction

In Joss, classes are written in standard style without cumbersome syntax. Internally, the engine attaches a lifecycle finalizer to every instance created with `new`.

When an object exhausts its lifecycle and has no remaining references in the program:
1. **Optional destructor**: If the class defines a `destructor()`, `destroy()`, or `__destruct()` method, the engine executes it automatically in an isolated and safe manner.
2. **Closing native resources**: If the instance held system resources (files, communication channels, streams), they are closed automatically, preventing descriptor leaks.
3. **Memory purge (*Zeroization*)**: The internal fields of the instance are cleared and sanitized to prevent sensitive data (tokens, passwords) from lingering unnecessarily in RAM.
4. **Protection against zombie access**: The instance is marked as destroyed. If any residual pointer tries to read or modify its members, the engine throws a `SecurityError`, protecting system integrity.

<!-- joss-run: ["Conexión activa", "Cerrando sesión de forma segura..."] -->
```joss
public class SesionSegura {
    public string $token = "tok_12345"

    public func constructor() {
        print("Conexión activa")
    }

    public func destructor() {
        print("Cerrando sesión de forma segura...")
    }
}

var $s = new SesionSegura()
$s->destructor()
```

Terminal output:

```text
Conexión activa
Cerrando sesión de forma segura...
```

---

## 12. Common OOP Mistakes in Joss

| Error | Cause | Solution |
|---|---|---|
| Confusing `->` with `::` | Writing `$objeto::metodo()` or `Clase->metodo()`. | Use `->` for real instances created with `new` and `::` for static calls on classes. |
| Trying to access a private member | `$cuenta->saldo` when it is `private`. | Create a public *getter* method (such as `obtenerSaldo()`) to query the value. |
| Forgetting `new` when instantiating | `var $p = Persona()` instead of `var $p = new Persona()`. | Creating instances requires the word `new`. |
| Confusing an instance with a map | Treating an object like an associative array (`$objeto["campo"]`). | Objects use the arrow (`$objeto->campo`), maps use square brackets (`$mapa["campo"]`). |
| Breaking an interface contract | The class declares `implements` but is missing a method or its parameters do not match. | Implement all interface methods with `public` visibility and compatible types (`JOSS-DECL-004`). |
| Instantiating an abstract class | Doing `new ClaseAbstracta()` directly. | Extend the abstract class in a concrete subclass and instantiate the subclass (`JOSS-DECL-006`). |
| Accessing a destroyed object | Trying to read or write an object after its destruction cycle has run. | Create a new valid instance instead of reusing an already invalidated object (`SecurityError`). |

---

## 13. Practical Exercises

### Level 1 — Quick check
Declare a `public record Producto(string $nombre, decimal $precio)`. Instantiate a product `"Teclado"` with price `45.50m` and print its name and price using `->`.

### Level 2 — Guided exercise
1. Create a class `public class Vehiculo` with a protected property `protected string $marca = ""` initialized in `public func constructor(string $marca)` and a method `public func obtenerMarca(): string`.
2. Create a derived class `public class Auto extends Vehiculo` that has a property `public int $puertas = 4`.
3. Instantiate an `Auto("Toyota")`, and print its brand (using `obtenerMarca()`) and its number of doors to the console.

### Level 3 — Challenge
1. Define an interface `public interface INotificador` with the method `public func enviar(string $mensaje): string;`.
2. Create two classes that implement `INotificador`:
   - `NotificadorEmail`: returns `"[EMAIL] " . $mensaje`.
   - `NotificadorSMS`: returns `"[SMS] " . $mensaje`.
3. Write a function `public func despacharAlerta(INotificador $canal, string $texto): void` that prints the result of `$canal->enviar($texto)`.
4. Test it by passing an instance of `NotificadorEmail` and another of `NotificadorSMS`.

---

## Next step

Now that you have mastered classes, interface contracts, and objects, you are ready to discover one of the most powerful capabilities of Joss: running tasks in parallel, delegating operations to the background, and communicating between processes through safe channels.

Continue with: [Concurrency, asynchronous operations, Future, and Channels](CONCURRENCIA.md).

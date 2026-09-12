# Classes, objects, methods and inheritance

Before: [Functions and closures](FUNCIONES.md), [Collections](COLECCIONES.md). After: [Error and exception handling](ERRORES.md).
Technical reference: [Type system](SISTEMA_TIPOS.md), [Native catalog](CATALOGO_NATIVO.md).

---

## What are you going to learn here?

As an application grows, having variables scattered on one side and loose functions on the other can become chaotic:
- You can have one variable `$usuario_nombre`, another `$usuario_email`, another `$usuario_rol`.
- If you have 100 users, how do you keep the data of each one together with the operations that correspond to them (such as authenticating, changing password or sending notification)?

**Object Oriented Programming (OOP)** solves this problem by packaging data and related behaviors into a single conceptual unit.

In this guide you will learn:
1. What is a **class** (the design plane) and what is an **object** or **instance** (the actual entity).
2. How to define **properties** (attributes) and **methods** (class functions).
3. How to initialize objects with **constructors** and `Init`.
4. The role of the special variable **`$this`**.
5. Visibility and encapsulation levels: `public`, `protected` and `private`.
6. Static members and the scope resolution operator (`::`).
7. How to reuse and specialize code through **inheritance** with `extends`.
8. The null-safe navigation operator (`?->`).

---

## 1. Classes and Objects: The plan and the house

To understand object orientation, the best analogy is architecture:
- A **class** is the **architectural plan**: it describes what rooms the house will have, how many doors and what functions it has. The plane does not occupy physical land nor can you live in it.
- An **object** (or **instance**) is the **actual physical house built** on a piece of land from that plan. You can build ten houses from the same plan; Painting one house blue does not change the color of the others.

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
$contador = new Contador()
print($contador->incrementar())
print($contador->incrementar())
```

### What elements make up this code?

1. `public class Contador`: Declare a public class called `Contador`. In Joss, file-level classes require a visibility modifier (`public` or `private`).
2. `public int $valor = 0`: It is a **property** (a piece of information that each instance of `Contador` will remember).
3. `public func incrementar(): int`: It is a **method** (a function that belongs to the class and that can manipulate its properties).
4. `$this`: It is a reserved word that means "this particular object." When you run `$this->valor`, you are accessing the `$valor` property of the instance that is running the method.
5. `new Contador()`: The keyword `new` creates a new real instance in memory.
6. `$contador->incrementar()`: The arrow operator `->` is used to access properties and methods of an instance.

---

## 2. Object initialization: Constructors

When you create an object, you almost always need to configure it with initial data (for example, a person's name or database credentials).

In Joss you can define a method `constructor` or a block `Init`:

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
$persona = new Persona("Ada")
print($persona->saludar())
```

When you type `new Persona("Ada")`, Joss automatically calls the constructor by giving it the argument `"Ada"`, which is stored safely within the private property `$this->nombre`.

> [!NOTE]
> Joss also supports the `Init(string $nombre) { ... }` block syntax. The `Init` blocks do not carry visibility modifiers (`public` or `private`).

### Builder Property Promotion

To avoid having to declare the property, receive the parameter and write `$this->prop = $prop` manually, Joss allows you to declare visibility (`public`, `protected` or `private`) and constancy (`const`) directly in the parameters of `Init` or `constructor`. Joss will create and assign the property automatically:

<!-- joss-run: ["Ada", "30"] -->
```joss
public class Usuario {
    Init (
        public string $nombre,
        public int $edad = 30
    ) {}
}

$u = new Usuario("Ada")
print($u->nombre)
print($u->edad)
```

It is also possible to declare promoted constant properties with `public const Tipo $campo` to protect them from later reallocation.

---

## 3. Encapsulation and visibility modifiers

**Encapsulation** is the principle of protecting the internal data of an object to prevent external code from modifying it incorrectly or corrupting it.

Joss offers three explicit visibility modifiers:

| Modifier | Where can you access | Recommended use |
|---|---|---|
| `public` | From **any part** of the program (within the class, in subclasses and from outside code). | For the object's public API: methods that users of your class need to invoke. |
| `protected` | Only **within the class itself** and **within subclasses** that inherit from it with `extends`. | For internal methods and properties that child classes need to specialize or refer to. |
| `private` | **Only within the exact class** where it was declared. No one else can see or modify it. | For intimate implementation details (passwords, raw connections, status flags). |

```joss
public class CuentaBancaria {
    private decimal $saldo = 0.0m

    public func depositar(decimal $monto) {
        ($monto > 0.0m) ? {
            $this->saldo = $this->saldo + $monto
        }
    }

    public func obtenerSaldo(): decimal {
        return $this->saldo
    }
}
```
By making `$saldo` private, no one can write `$cuenta->saldo = -5000.0m` from the outside, ensuring that money is only modified under the rules of the `depositar` method.

---

## 4. Static members and the operator `::`

Not all properties or methods belong to an individual house; some operations belong to the general concept of the class or do not require creating an instance with `new`.

These elements are called **static** and are declared with the word `static`:

```joss
public class Utilidades {
    public static func limpiarTexto(string $t): string {
        return trim($t)
    }
}
```

To invoke a static method or read a static property, you do not use `->`, but rather the double colon operator **`::`**:

```joss
$limpio = Utilidades::limpiarTexto("  hola  ")
```

In Joss, native system classes (such as `Auth::user()`, `GranDB::table()`, `Route::get()`, `Cache::put()`) are facades that are typically invoked using `::`.

---

## 5. Inheritance with `extends`

**Inheritance** allows you to create a new class based on an existing class, reusing all its public and protected methods and properties without having to rewrite them:

<!-- joss-run: ["hola"] -->
```joss
public class Mensaje {
    public func texto(): string { return "hola" }
}
public class Aviso extends Mensaje {}
$aviso = new Aviso()
print($aviso->texto())
```

- The class `Mensaje` is the **base class** (or superclass).
- The class `Aviso` is the **derived class** (or subclass).
- `Aviso` automatically inherits the method `texto()` from `Mensaje`.

> [!TIP]
> **When to use inheritance vs when to use composition**:
> Use inheritance only when a strict "is a" relationship exists (for example, `Gato extends Animal` or `AdminUser extends User`). If you just want to reuse a utility function, don't use inheritance; use functions or inject a service class.

---

## 6. Interfaces and Polymorphism (`interface` and `implements`)

When you work on modular applications or clean architecture, you often want to define **what** a component should do without being tied to **how** it does it.

An **interface** is a **formal contract**:
- Only declare the prototypes of the public methods (name, typed parameters and return type) without a body.
- An interface cannot be instantiated directly with `new`.
- Any class that declares `implements NombreInterfaz` is **forced by the semantic analyzer and runtime** to implement all promised methods with compatible signatures.
- A class can inherit from a base class and at the same time implement **multiple interfaces** separated by commas: `public class MiClase extends Base implements I1, I2`.
- An interface can extend one or more interfaces: `public interface IDerivada extends IBase1, IBase2`.

Let's look at an example of executable polymorphism:

<!-- joss-run: ["50", "36"] -->
```joss
public interface IFigura {
    public func calcularArea(): int;
}

public class Rectangulo implements IFigura {
    public int $ancho = 0
    public int $alto = 0

    Init constructor(int $ancho, int $alto) {
        $this->ancho = $ancho
        $this->alto = $alto
    }

    public func calcularArea(): int {
        return $this->ancho * $this->alto
    }
}

public class Cuadrado implements IFigura {
    public int $lado = 0

    Init constructor(int $lado) {
        $this->lado = $lado
    }

    public func calcularArea(): int {
        return $this->lado * $this->lado
    }
}

public func imprimirArea(IFigura $figura): int {
    return $figura->calcularArea()
}

$r = new Rectangulo(5, 10)
$c = new Cuadrado(6)
print(imprimirArea($r))
print(imprimirArea($c))
```

### Advantages of Polymorphism with Interfaces:
1. **Decoupling**: The `imprimirArea(IFigura $figura)` function does not need to know whether it receives a `Rectangulo`, a `Cuadrado`, or any future figure; just trust that you comply with the contract `IFigura`.
2. **Exhaustive static validation**: If you forget to implement a method in a class or declare a parameter with a different type, the semantic analyzer immediately outputs `JOSS-DECL-005`.

---

## 7. Abstract classes and methods (`abstract`)

An **abstract class** (`public abstract class`) serves as a base template for other classes but **cannot be directly instantiated** with `new` (it will emit `JOSS-DECL-004`).

Abstract classes can contain:
- Complete properties and methods with implementation to be inherited.
- Abstract methods (`abstract func nombre(...): Tipo`) that lack a body and force subclasses to implement them (`JOSS-DECL-003`).

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

$perro = new Perro()
print($perro->hablar())
```

---

## 8. Type and instance checking: `is` and `instanceof`

To check at run time whether an object belongs to a particular class, inherits from a base class, or implements an interface, use the equivalent operators **`is`** or **`instanceof`**:

<!-- joss-run: ["true", "true", "false"] -->
```joss
public interface IMovible {}
public class Auto implements IMovible {}

$auto = new Auto()
print($auto is Auto)
print($auto is IMovible)
print($auto instanceof string)
```

You can also use `is` with primitive types like `int`, `string`, `bool`, etc. (e.g. `$x is int`).

---

## 9. Null-safe browsing (`?->`)

If a variable can contain an instance or be `null` (type `Persona?`), attempting to access a method with `->` on a null value could cause an error.

Joss includes the **null-safe (`?->`)** operator:

```joss
Persona? $usuario = obtenerUsuario(123)
$nombre = $usuario?->saludar()
```

If `$usuario` is `null`, the call is silently and safely canceled, and `$nombre` will simply receive `null` without stopping the program.

---

## 10. Life cycle and intelligent self-destruction for protection

In Joss, classes are written in a standard way without cumbersome syntax. Internally, the engine attaches a lifecycle finalizer to each instance created with `new`.

When an object exhausts its life cycle and is left without references in the program:
1. **Optional destructor**: If the class defines a method `destructor()`, `destroy()` or `__destruct()`, the engine automatically executes it in isolation and safety.
2. **Closing of native resources**: If the instance retained system resources (files, communication channels, streams), they are automatically closed, preventing descriptor leaks.
3. **Memory purge (*Zeroization*)**: The internal fields of the instance are emptied and sanitized to prevent sensitive data (tokens, passwords) from persisting unnecessarily in RAM memory.
4. **Protection against zombie access**: The instance is marked as destroyed. If any residual pointer attempts to read or modify its members, the engine throws an `SecurityError` protecting the integrity of the system.

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

$s = new SesionSegura()
$s->destructor()
```

---

## 11. Common mistakes in OOP with Joss

| Error | Cause | Solution |
|---|---|---|
| Confusing `->` with `::` | Write `$objeto::metodo()` or `Clase->metodo()`. | Use `->` for real instances created with `new` and `::` for static class calls. |
| Try to access a private member | `$cuenta->saldo` when it is `private`. | Create a public *getter* method (such as `obtenerSaldo()`) to query the value. |
| Forget `new` when instantiating | `$p = Persona()` instead of `$p = new Persona()`. | Instantiation requires the word `new`. |
| Confuse an instance with a map | Treat an object as an associative array (`$objeto["campo"]`). | Objects use arrow (`$objeto->campo`), maps use square brackets (`$mapa["campo"]`). |
| Breach interface contract | The class declares `implements` but is missing a method or its parameters do not match. | Implement all interface methods with visibility `public` and compatible types (`JOSS-DECL-005`). |
| Access to destroyed object | Attempting to read or write an object after its destruction cycle has been executed. | Create a new valid instance instead of reusing an already invalidated object (`SecurityError`). |

---

## 12. Practical exercise

1. **Vehicle hierarchy**:
   - Create a class `public class Vehiculo` with a protected property `protected string $marca` and a method `public func obtenerMarca(): string`.
   - Create a derived class `public class Auto extends Vehiculo` that has a property `public int $puertas = 4`.
   - Instantiate a `Auto`, assign it a brand and display its brand and number of doors in the console.

---

## Next step

Even in the best object-oriented code, things can break: a file may not exist, a database may be offline, or a user may enter invalid data. We will learn how to intercept and solve these problems elegantly:

Continues with: [Error, exception and try/catch handling](ERRORES.md).

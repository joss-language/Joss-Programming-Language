# Clases, objetos, métodos y herencia

Antes: [Funciones y closures](FUNCIONES.md), [Colecciones](COLECCIONES.md). Después: [Manejo de errores y excepciones](ERRORES.md).
Referencia técnica: [Sistema de tipos](SISTEMA_TIPOS.md), [Catálogo nativo](CATALOGO_NATIVO.md).

---

## ¿Qué vas a aprender aquí?

A medida que una aplicación crece, tener variables dispersas por un lado y funciones sueltas por otro puede volverse caótico:
- Puedes tener una variable `$usuario_nombre`, otra `$usuario_email`, otra `$usuario_rol`.
- Si tienes 100 usuarios, ¿cómo mantienes unidos los datos de cada uno con las operaciones que les corresponden (como autenticarse, cambiar contraseña o enviar notificación)?

La **Programación Orientada a Objetos (POO)** resuelve este problema empaquetando datos y comportamientos relacionados en una sola unidad conceptual.

En esta guía aprenderás:
1. Qué es una **clase** (el plano de diseño) y qué es un **objeto** o **instancia** (la entidad real).
2. Cómo definir **propiedades** (atributos) y **métodos** (funciones de la clase).
3. Cómo inicializar objetos con **constructores** e `Init`.
4. El rol de la variable especial **`$this`**.
5. Los niveles de visibilidad y encapsulación: `public`, `protected` y `private`.
6. Miembros estáticos y el operador de resolución de ámbito (`::`).
7. Cómo reutilizar y especializar código mediante **herencia** con `extends`.
8. El operador de navegación segura ante nulos (`?->`).

---

## 1. Clases y Objetos: El plano y la casa

Para entender la orientación a objetos, la mejor analogía es la arquitectura:
- Una **clase** es el **plano arquitectónico**: describe qué habitaciones tendrá la casa, cuántas puertas y qué funciones tiene. El plano no ocupa un terreno físico ni puedes vivir en él.
- Un **objeto** (o **instancia**) es la **casa física real construida** en un terreno a partir de ese plano. Puedes construir diez casas a partir del mismo plano; pintar una casa de azul no cambia el color de las demás.

Veamos un ejemplo mínimo con un contador:

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

### ¿Qué elementos componen este código?

1. `public class Contador`: Declara una clase pública llamada `Contador`. En Joss, las clases a nivel de archivo requieren un modificador de visibilidad (`public` o `private`).
2. `public int $valor = 0`: Es una **propiedad** (un dato que cada instancia de `Contador` recordará).
3. `public func incrementar(): int`: Es un **método** (una función que le pertenece a la clase y que puede manipular sus propiedades).
4. `$this`: Es una palabra reservada que significa "este objeto en particular". Cuando ejecutas `$this->valor`, estás accediendo a la propiedad `$valor` de la instancia que está ejecutando el método.
5. `new Contador()`: La palabra clave `new` crea una nueva instancia real en la memoria.
6. `$contador->incrementar()`: El operador flecha `->` se utiliza para acceder a propiedades y métodos de una instancia.

---

## 2. Inicialización de objetos: Constructores

Cuando creas un objeto, casi siempre necesitas configurarlo con datos iniciales (por ejemplo, el nombre de una persona o las credenciales de una base de datos).

En Joss puedes definir un método `constructor` o un bloque `Init`:

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

Al escribir `new Persona("Ada")`, Joss llama automáticamente al constructor entregándole el argumento `"Ada"`, el cual queda guardado de forma segura dentro de la propiedad privada `$this->nombre`.

> [!NOTE]
> Joss también admite la sintaxis de bloque `Init(string $nombre) { ... }`. Los bloques `Init` no llevan modificadores de visibilidad (`public` ni `private`).

### Promoción de propiedades en el constructor (Constructor Property Promotion)

Para evitar tener que declarar la propiedad, recibir el parámetro y escribir `$this->prop = $prop` manualmente, Joss permite declarar la visibilidad (`public`, `protected` o `private`) directamente en los parámetros de `Init` o del `constructor`. Joss creará y asignará la propiedad automáticamente:

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


---

## 3. Encapsulación y modificadores de visibilidad

La **encapsulación** es el principio de proteger los datos internos de un objeto para evitar que código externo los modifique de forma incorrecta o corrupta.

Joss ofrece tres modificadores de visibilidad explícitos:

| Modificador | Dónde se puede acceder | Uso recomendado |
|---|---|---|
| `public` | Desde **cualquier parte** del programa (dentro de la clase, en subclases y desde código exterior). | Para la API pública del objeto: métodos que los usuarios de tu clase necesitan invocar. |
| `protected` | Solo **dentro de la propia clase** y **dentro de las subclases** que hereden de ella con `extends`. | Para métodos y propiedades internas que las clases hijas necesitan especializar o consultar. |
| `private` | **Únicamente dentro de la clase exacta** donde fue declarada. Nadie más puede verla ni modificarla. | Para detalles de implementación íntimos (contraseñas, conexiones crudas, flags de estado). |

```joss
public class CuentaBancaria {
    private decimal $saldo = 0.0m

    public func depositar(decimal $monto) {
        ($monto > 0.0m) ? {
            $this->saldo = $this->saldo + $monto
        } : {}
    }

    public func obtenerSaldo(): decimal {
        return $this->saldo
    }
}
```
Al hacer `$saldo` privado, nadie puede escribir `$cuenta->saldo = -5000.0m` desde afuera, garantizando que el dinero solo se modifique bajo las reglas del método `depositar`.

---

## 4. Miembros estáticos y el operador `::`

No todas las propiedades o métodos le pertenecen a una casa individual; algunas operaciones pertenecen al concepto general de la clase o no requieren crear una instancia con `new`.

A estos elementos se les llama **estáticos** y se declaran con la palabra `static`:

```joss
public class Utilidades {
    public static func limpiarTexto(string $t): string {
        return trim($t)
    }
}
```

Para invocar un método estático o leer una propiedad estática, no se utiliza `->`, sino el operador de doble dos puntos **`::`**:

```joss
$limpio = Utilidades::limpiarTexto("  hola  ")
```

En Joss, las clases nativas del sistema (como `Auth::user()`, `GranDB::table()`, `Route::get()`, `Cache::put()`) son fachadas que se invocan habitualmente mediante `::`.

---

## 5. Herencia con `extends`

La **herencia** permite crear una clase nueva basada en una clase existente, reutilizando todos sus métodos y propiedades públicas y protegidas sin tener que reescribirlos:

<!-- joss-run: ["hola"] -->
```joss
public class Mensaje {
    public func texto(): string { return "hola" }
}
public class Aviso extends Mensaje {}
$aviso = new Aviso()
print($aviso->texto())
```

- La clase `Mensaje` es la **clase base** (o superclase).
- La clase `Aviso` es la **clase derivada** (o subclase).
- `Aviso` hereda automáticamente el método `texto()` de `Mensaje`.

> [!TIP]
> **Cuándo usar herencia vs cuándo usar composición**:
> Usa herencia solo cuando exista una relación estricta de tipo "es un" (por ejemplo, `Gato extends Animal` o `AdminUser extends User`). Si solo quieres reutilizar una función utilitaria, no uses herencia; usa funciones o inyecta una clase de servicio.

---

## 6. Interfaces y Polimorfismo (`interface` e `implements`)

Cuando trabajas en aplicaciones modulares o arquitectura limpia, muchas veces quieres definir **qué** debe hacer un componente sin atarte a **cómo** lo hace.

Una **interfaz** es un **contrato formal**:
- Solo declara los prototipos de los métodos públicos (nombre, parámetros tipados y tipo de retorno) sin cuerpo.
- Una interfaz no se puede instanciar directamente con `new`.
- Cualquier clase que declare `implements NombreInterfaz` está **obligada por el analizador semántico y el runtime** a implementar todos los métodos prometidos con firmas compatibles.
- Una clase puede heredar de una clase base y a la vez implementar **múltiples interfaces** separadas por comas: `public class MiClase extends Base implements I1, I2`.
- Una interfaz puede extender una o varias interfaces: `public interface IDerivada extends IBase1, IBase2`.

Veamos un ejemplo de polimorfismo ejecutable:

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

### Ventajas del Polimorfismo con Interfaces:
1. **Desacoplamiento**: La función `imprimirArea(IFigura $figura)` no necesita saber si recibe un `Rectangulo`, un `Cuadrado` o cualquier figura futura; solo confía en que cumple con el contrato `IFigura`.
2. **Validación estática exhaustiva**: Si olvidas implementar un método en una clase o declaras un parámetro con un tipo diferente, el analizador semántico emite de inmediato `JOSS-DECL-005`.

---

## 7. Clases y métodos abstractos (`abstract`)

Una **clase abstracta** (`public abstract class`) sirve como plantilla base para otras clases pero **no puede ser instanciada directamente** con `new` (emitirá `JOSS-DECL-004`).

Las clases abstractas pueden contener:
- Propiedades y métodos completos con implementación para ser heredados.
- Métodos abstractos (`abstract func nombre(...): Tipo`) que carecen de cuerpo y obligan a las subclases a implementarlos (`JOSS-DECL-003`).

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

## 8. Comprobación de tipos e instancias: `is` e `instanceof`

Para verificar en tiempo de ejecución si un objeto pertenece a una clase concreta, hereda de una clase base o implementa una interfaz, utiliza los operadores equivalentes **`is`** o **`instanceof`**:

<!-- joss-run: ["true", "true", "false"] -->
```joss
public interface IMovible {}
public class Auto implements IMovible {}

$auto = new Auto()
print($auto is Auto)
print($auto is IMovible)
print($auto instanceof string)
```

También puedes usar `is` con tipos primitivos como `int`, `string`, `bool`, etc. (ej. `$x is int`).

---

## 9. Navegación segura contra nulos (`?->`)

Si una variable puede contener una instancia o ser `null` (tipo `Persona?`), intentar acceder a un método con `->` sobre un valor nulo podría causar un error.

Joss incluye el operador **null-safe (`?->`)**:

```joss
Persona? $usuario = obtenerUsuario(123)
$nombre = $usuario?->saludar()
```

Si `$usuario` es `null`, la llamada se cancela de forma silenciosa y segura, y `$nombre` recibirá simplemente `null` sin detener el programa.

---

## 10. Errores comunes en POO con Joss

| Error | Causa | Solución |
|---|---|---|
| Confundir `->` con `::` | Escribir `$objeto::metodo()` o `Clase->metodo()`. | Usa `->` para instancias reales creadas con `new` y `::` para llamadas estáticas a clases. |
| Intentar acceder a un miembro privado | `$cuenta->saldo` cuando es `private`. | Crea un método público *getter* (como `obtenerSaldo()`) para consultar el valor. |
| Olvidar `new` al instanciar | `$p = Persona()` en vez de `$p = new Persona()`. | La creación de instancias exige la palabra `new`. |
| Confundir una instancia con un map | Tratar un objeto como array asociativo (`$objeto["campo"]`). | Los objetos usan flecha (`$objeto->campo`), los maps usan corchetes (`$mapa["campo"]`). |
| Incumplir contrato de interfaz | La clase declara `implements` pero le falta un método o sus parámetros no coinciden. | Implementar todos los métodos de la interfaz con visibilidad `public` y tipos compatibles (`JOSS-DECL-005`). |

---

## 11. Ejercicio práctico

1. **Jerarquía de vehículos**:
   - Crea una clase `public class Vehiculo` con una propiedad protegida `protected string $marca` y un método `public func obtenerMarca(): string`.
   - Crea una clase derivada `public class Auto extends Vehiculo` que tenga una propiedad `public int $puertas = 4`.
   - Instancia un `Auto`, asígnale marca y muestra su marca y número de puertas en la consola.

---

## Siguiente paso

Incluso en el mejor código orientado a objetos, las cosas pueden fallar: un archivo puede no existir, una base de datos puede estar desconectada o un usuario puede ingresar datos no válidos. Aprenderemos cómo interceptar y solucionar estos problemas con elegancia:

Continúa con: [Manejo de errores, excepciones y try/catch](ERRORES.md).

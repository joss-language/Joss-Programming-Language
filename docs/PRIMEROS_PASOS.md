# Primeros pasos: de un archivo a un programa

Antes: [Índice](README.md). Después: [Valores, variables y operaciones](FUNDAMENTOS.md).
Consulta rápida: [Línea de comandos (CLI)](CLI.md).

---

## ¿Qué vas a aprender aquí?

Si nunca has escrito una sola línea de código en tu vida, este es el lugar correcto para comenzar. No necesitas conocimientos previos de programación ni experiencia en otros lenguajes.

En esta guía aprenderás:
1. Qué es programar y qué es una instrucción para una computadora.
2. Qué es el lenguaje **Joss** y qué componentes forman su ecosistema.
3. Cómo instalar Joss en tu sistema operativo (Windows, Linux o macOS).
4. Cómo escribir tu primer programa ("Hola Mundo"), analizarlo y ejecutarlo.
5. Qué ocurre internamente desde que guardas el archivo de texto hasta que la pantalla muestra el resultado.
6. Cómo solucionar los errores más comunes al dar los primeros pasos.

---

## 1. ¿Qué es programar y qué es un programa?

Una computadora es extraordinariamente rápida realizando cálculos, pero no puede intuir qué quieres hacer. Necesita una secuencia clara, ordenada y sin ambigüedades de órdenes. A esa secuencia de instrucciones se le llama **programa** o **algoritmo**.

Imagina una receta de cocina:
1. Pesa 200 gramos de harina.
2. Añade dos huevos.
3. Mezcla durante cinco minutos.

Un programa de computadora funciona bajo la misma lógica: realiza tareas paso a paso. Al texto exacto que tú escribes para darle esas órdenes a la computadora se le denomina **código fuente**.

En **Joss**, el código fuente se redacta en texto plano y se guarda en archivos cuya extensión finaliza en `.joss` (por ejemplo, `hola.joss` o `app.joss`). No debes usar procesadores de texto enriquecido como Microsoft Word o Google Docs, porque añaden formatos invisibles que la computadora no comprende; se utilizan **editores de código** como Visual Studio Code.

---

## 2. ¿Qué es Joss?

**Joss** es un lenguaje de programación moderno diseñado especialmente para aplicaciones backend, desarrollo web, utilidades de línea de comandos y servicios concurrentes de alto rendimiento.

Su filosofía de diseño se apoya en cuatro pilares:

1. **Cero imports en el código fuente (Zero Imports)**: En muchos lenguajes debes escribir docenas de líneas como `import X from Y` al inicio de cada archivo. Joss descubre y organiza automáticamente los archivos, clases y funciones públicas de tu proyecto y sus plugins, permitiéndote concentrarte en la lógica de negocio.
2. **Seguridad y análisis estático riguroso**: Antes de ejecutar una sola instrucción, el **analizador semántico** de Joss revisa tus variables, tipos de datos, rutas de retorno y visibilidades para detectar errores antes de que lleguen a producción.
3. **Pila completa incluida (Batteries-Included)**: Joss incluye nativamente servidor HTTP de alto rendimiento, sistema de rutas, motor de plantillas HTML, ORM y generador de esquemas para bases de datos (GranDB / Schema), soporte para WebSockets en tiempo real, hashing criptográfico y tareas asíncronas.
4. **Concurrencia limpia y canales**: Puedes delegar tareas pesadas a segundo plano de forma no bloqueante con `async` y comunicar procesos con canales seguros (`channel`).

#### Un solo lenguaje, dos formas de ejecutarlo: `joss run` y `joss build`

El código que escribes en Joss se define una sola vez y tiene exactamente el mismo significado en todos los modos:

```text
┌─────────────────────────────────────────────────────────────────┐
│ 1. Tu código fuente (.joss)                                     │
│    El texto legible que tú escribes con tus instrucciones.      │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. El analizador semántico (Semantic Analyzer)                  │
│    Revisa tipos, nombres, visibilidad y coherencia del proyecto. │
└───────────────┬───────────────────────────────┬─────────────────┘
                │                               │
                ▼                               ▼
┌───────────────────────────────┐ ┌───────────────────────────────┐
│ A. Desarrollo inmediato       │ │ B. Distribución nativa        │
│    `joss run main.joss`       │ │    `joss build main.joss`     │
│    Ejecuta al instante en     │ │    Compila el mismo código a  │
│    tu terminal.               │ │    un ejecutable (.exe / ELF).│
└───────────────────────────────┘ └───────────────────────────────┘
```

- **El lenguaje Joss**: Las reglas de escritura, palabras reservadas y sintaxis que aprenderás en este curso.
- **La herramienta CLI (`joss`)**: El programa de línea de comandos que utilizas desde tu terminal para crear proyectos, analizarlos, ejecutarlos al instante (`joss run`) o compilarlos a un archivo ejecutable independiente (`joss build`).
- **El Runtime / Compilador**: El motor que toma tu programa ya verificado y ejecuta las operaciones reales en el procesador y la memoria de tu equipo.

---

## 3. Instalación paso a paso

Una **terminal** (o consola) es una ventana de texto donde te comunicas con el sistema operativo escribiendo órdenes en lugar de hacer clic con el ratón.
- En **Windows**: Abre **PowerShell** o la aplicación **Terminal de Windows** (búscala en el menú Inicio).
- En **Linux o macOS**: Abre la aplicación llamada **Terminal**.

### Opción A: Instalador automático oficial (Recomendado)

En **Windows** (abre PowerShell):

```powershell
iwr -useb https://astriom.org/install | iex
```

En **Linux, macOS o Android (Termux)** (abre tu Terminal):

```bash
curl -fsSL https://astriom.org/install.sh | bash
```

Este comando descargará el ejecutable de Joss, lo ubicará en una carpeta estándar del sistema y lo registrará en tu `PATH`.

> [!NOTE]
> **¿Qué es el `PATH`?**
> El `PATH` es una lista interna del sistema operativo con las carpetas donde viven los programas que puedes invocar escribiendo solo su nombre. Gracias a ello puedes escribir `joss` desde cualquier carpeta. Si acabas de instalarlo y tu terminal no lo reconoce, **cierra la terminal y ábrela de nuevo**.

### Opción B: Descarga manual desde GitHub Releases

1. Ve a la sección de lanzamientos: [GitHub Releases de Joss](https://github.com/joss-language/Joss-Programming-Language/releases).
2. Descarga el archivo comprimido correspondiente a tu sistema (`windows_amd64`, `linux_amd64`, `darwin_arm64`, etc.).
3. Descomprime el archivo, coloca `joss` (o `joss.exe`) en una carpeta de tu disco y agrégala al `PATH`.

### Opción C: Compilar desde el código fuente con Go

Si ya tienes [Go](https://go.dev) instalado (versión 1.22 o superior):

```bash
git clone https://github.com/joss-language/Joss-Programming-Language.git
cd Joss-Programming-Language
go build -o joss ./cmd/joss
```

### Comprobar que la instalación funciona

Abre tu terminal y escribe:

```bash
joss version
```

Deberías ver en pantalla la versión instalada de Joss:

```text
Joss v3.6.8.0 (powerbuilder Joss 3.6.8.0)
```

Si ves ese mensaje, ¡tu entorno está 100% listo!

---

## 4. Crear y ejecutar tu primer proyecto con `joss new console`

En lugar de crear carpetas y archivos vacíos a mano, Joss incluye un generador oficial que prepara todo tu espacio de trabajo en un segundo.

### Paso 1: Generar el proyecto de consola

En tu terminal, escribe el siguiente comando y presiona **Enter**:

```bash
joss new console mi-primer-joss
```

Verás que Joss te confirma la creación del proyecto:

```text
Creando proyecto Joss Console en: mi-primer-joss

✓ Proyecto de consola creado exitosamente

Estructura creada:
  - main.joss           (Entry point)
  - env.joss            (Variables de entorno)
  - config/reglas.joss  (Constantes globales)
  - app/controllers/    (Lógica de negocio)
  - app/models/         (Acceso a datos)
  - app/libs/           (Librerías)
  - app/database/migrations/

Para ejecutar:
  cd mi-primer-joss
  joss run main.joss
```

### Paso 2: Entrar a la carpeta del proyecto y ejecutarlo

Entra a la carpeta recién creada con el comando `cd` (*change directory*, cambiar de directorio) y ejecuta el archivo principal `main.joss`:

```bash
cd mi-primer-joss
joss run main.joss
```

Salida en tu terminal:

```text
=== Aplicación de Consola Joss ===

¡Hola desde Joss Console!

Este es un proyecto de consola backend-only.
Puedes agregar tu lógica en este archivo main.joss

Aplicación finalizada correctamente.
```

¡Felicidades! Acabas de crear y ejecutar tu primer proyecto completo en Joss.

---

## 5. ¿Qué acaba de crear Joss? Entendiendo `main.joss`

Antes de modificar el programa, entendamos tres conceptos esenciales:

1. **¿Qué es un proyecto?**
   Es una carpeta organizada (`mi-primer-joss`) donde viven tu código fuente, tu configuración (`env.joss`) y las subcarpetas (`app/`) para cuando tu programa crezca.
2. **¿Qué es `main.joss`?**
   Es la **puerta principal** (punto de entrada o *entry point*) de tu programa. Cuando ejecutas `joss run main.joss` o compilas tu proyecto con `joss build`, Joss empieza a leer por este archivo.
3. **¿Qué hay dentro de `main.joss`?**
   Abre la carpeta `mi-primer-joss` en tu editor de código (por ejemplo, escribiendo `code .` si usas Visual Studio Code) y abre el archivo `main.joss`. Verás este contenido:

<!-- joss-run: ["=== Aplicación de Consola Joss ===", "", "¡Hola desde Joss Console!", "", "Aplicación finalizada correctamente."] -->
```joss
public class Main {
    Init main() {
        print("=== Aplicación de Consola Joss ===")
        print("")
        print("¡Hola desde Joss Console!")
        print("")
        print("Aplicación finalizada correctamente.")
    }
}
```

### ¿Qué significan `public class Main` e `Init main()`?

Más adelante (en el capítulo de [Clases y objetos](CLASES.md)) aprenderás en detalle qué es una clase. Por ahora, solo necesitas saber una regla muy sencilla:

- `public class Main { Init main() { ... } }` es el **"botón de encendido"** oficial de tu proyecto.
- Todo lo que escribas **dentro de las llaves `{` y `}` de `Init main()`** son las órdenes que la computadora ejecutará en orden, de arriba hacia abajo, cuando arranques el programa.
- Las líneas que empiezan con `//` son **comentarios**: notas para ti que la computadora ignora por completo.

### Escribir tu propio saludo mínimo

Reemplaza el contenido de `main.joss` por este ejemplo mínimo de una sola instrucción dentro de `Init main()`:

<!-- joss-run: ["Hola, Joss!"] -->
```joss
public class Main {
    Init main() {
        print("Hola, Joss!")
    }
}
```

> [!TIP]
> **¿Sabías que también puedes escribir instrucciones directas?**
> Si para un experimento rápido escribes únicamente `print("Hola, Joss!")` en `main.joss` (sin envolverlo en `public class Main`), Joss también lo ejecutará. A lo largo del curso veremos fragmentos breves así para ir directo al grano, pero en tus proyectos creados con `joss new console` puedes colocar todo tu código dentro de `Init main() { ... }`.

Guarda el archivo y vuelve a ejecutar en tu terminal:

```bash
joss run main.joss
```

Salida:

```text
Hola, Joss!
```

### ¿Qué hace cada parte de `print("Hola, Joss!")`?

1. `print`: Es una **función integrada** en Joss. Su trabajo es recibir un dato y mostrarlo en la pantalla de tu terminal, agregando un salto de línea al final.
2. `(` y `)`: Los paréntesis le indican a Joss que quieres **ejecutar** `print`. Lo que colocas dentro de los paréntesis es el dato que le entregas para trabajar (llamado **argumento**).
3. `"Hola, Joss!"`: Es un valor de tipo **texto** (`string`). Las comillas dobles `"` marcan dónde empieza y dónde termina el texto. Las comillas no se imprimen en pantalla; solo delimitan el mensaje.

### Probar ideas al instante sin editar archivos: `joss repl`

Si en cualquier momento quieres probar una suma o una línea de código en vivo, escribe en tu terminal:

```bash
joss repl
```

Verás la consola interactiva de Joss:

```text
Joss REPL v3.6.8.0
Escribe 'exit' para salir.
joss> var $x = 10
joss> $x * 5
50
joss> print("Hola desde el REPL!")
Hola desde el REPL!
joss> exit
```

---

## 6. El comando `analyze`: Tu red de seguridad antes de ejecutar

En Joss no tienes que esperar a que un programa falle a mitad de camino para descubrir un error. Puedes pedirle al **analizador semántico** que revise tu proyecto escribiendo:

```bash
joss analyze main.joss
```

Si tu código cumple todas las reglas, verás:

```text
[Analyzer] Análisis completado sin errores.
```

¿Qué revisa `joss analyze` por ti?
- **La puntuación (sintaxis)**: Que no hayas olvidado cerrar unas comillas `"`, un paréntesis `)` o una llave `}`.
- **Los nombres**: Que no intentes usar una variable o función que nunca fue creada.
- **Los tipos de datos**: Que no intentes guardar un texto donde prometiste guardar un número entero.

De hecho, cada vez que ejecutas `joss run main.joss` o compilas con `joss build main.joss`, Joss pasa primero este filtro de seguridad automáticamente.

---

## 7. Guardar información en la memoria: Tu primera variable

Un programa que solo imprime texto fijo siempre hace lo mismo. Para recordar un nombre, una edad o un resultado, usamos **variables**.

Imagina una variable como una **caja etiquetada en la memoria** de la computadora donde puedes guardar un dato para usarlo después.

Actualiza tu archivo `main.joss` con el siguiente código:

<!-- joss-run: ["Hola, Ada", "Bienvenida a Joss"] -->
```joss
public class Main {
    Init main() {
        var $nombre = "Ada"
        print("Hola, " . $nombre)
        print("Bienvenida a Joss")
    }
}
```

Guarda y ejecuta `joss run main.joss`:

```text
Hola, Ada
Bienvenida a Joss
```

### Explicación línea por línea:

1. `var $nombre = "Ada"`:
   - `var`: Le dice a Joss *"voy a crear una variable nueva; deduce su tipo a partir del valor inicial"*.
   - `$nombre`: Es el nombre de la variable. **En Joss, todas las variables empiezan obligatoriamente con el signo `$`**.
   - `=`: Es el **operador de asignación**. Toma el valor de la derecha (`"Ada"`) y lo guarda dentro de `$nombre`.
2. `print("Hola, " . $nombre)`:
   - El punto `.` es el **operador de concatenación de texto**. Une el texto `"Hola, "` con el contenido guardado en `$nombre` (`"Ada"`), formando `"Hola, Ada"`.
   - También puedes incrustar la variable directamente dentro de las comillas dobles escribiendo `print("Hola, ${nombre}")`.

> [!IMPORTANT]
> **Crear una variable (`var $nombre = ...`) vs Cambiar su valor (`$nombre = ...`)**
> En Joss, la **primera vez** que creas una variable debes declararla con `var` (o indicando su tipo, como `string $nombre = "Ada"`). Si intentas escribir `$nombre = "Ada"` por primera vez sin haberla declarado antes, Joss te avisará con el error `JOSS-SYM-001` (*Variable is not declared*). Una vez declarada, sí puedes cambiar su valor más adelante escribiendo `$nombre = "Carlos"`.

---

## 8. Dialogar con el usuario: Entrada por teclado con `cin >>`

Ahora hagamos que el programa nos haga preguntas, espere a que escribamos en el teclado y nos responda. Todo programa interactivo sigue tres pasos:

```text
┌─────────────────────────┐       ┌─────────────────────────┐       ┌─────────────────────────┐
│     1. ENTRADA          │  ──>  │     2. PROCESO          │  ──>  │     3. SALIDA           │
│ El usuario escribe con  │       │ El programa calcula o   │       │ Se muestra el resultado │
│ cin >> $variable        │       │ prepara la respuesta    │       │ en pantalla con print() │
└─────────────────────────┘       └─────────────────────────┘       └─────────────────────────┘
```

Escribe en tu `main.joss`:

<!-- joss-check: lectura interactiva de datos por teclado -->
```joss
public class Main {
    Init main() {
        string $nombre = ""
        int $edad = 0

        print("¿Cómo te llamas?")
        cin >> $nombre

        print("¿Cuántos años tienes?")
        cin >> $edad

        print("¡Mucho gusto, " . $nombre . "! El próximo año tendrás " . ($edad + 1) . " años.")
    }
}
```

### ¿Cómo funciona este programa cuando lo ejecutas con `joss run main.joss`?

1. `string $nombre = ""` e `int $edad = 0`: Creamos dos variables listas para recibir datos: una para texto (`string`) inicialmente vacía `""`, y otra para un número entero (`int`) inicialmente en `0`.
2. `cin >> $nombre`: Pausa el programa y espera a que escribas tu nombre en la terminal y presiones **Enter**. Lo que escribas se guarda en `$nombre`.
3. `cin >> $edad`: Espera a que escribas tu edad y la convierte automáticamente en un número entero dentro de `$edad`.
4. `($edad + 1)`: Suma `1` a la edad que escribiste y muestra el saludo completo.

Ejemplo de sesión real en tu terminal:

```text
¿Cómo te llamas?
Ada
¿Cuántos años tienes?
20
¡Mucho gusto, Ada! El próximo año tendrás 21 años.
```

---

## 9. Resumen rápido de tu primer capítulo

Guarda esta tabla como referencia de lo que ya dominas:

| ¿Qué quieres hacer? | Comando o código en Joss | Ejemplo real |
|---|---|---|
| **Crear un proyecto de consola** | `joss new console <nombre>` | `joss new console mi-primer-joss` |
| **Ejecutar tu proyecto** | `joss run main.joss` | `joss run main.joss` |
| **Revisar errores sin ejecutar** | `joss analyze main.joss` | `joss analyze main.joss` |
| **Compilar a ejecutable nativo** | `joss build main.joss` | Genera `build/main.exe` (o `build/main`) |
| **Mostrar un mensaje en pantalla** | `print(...)` | `print("¡Hola mundo!")` |
| **Crear una variable nueva** | `var $variable = valor` | `var $ciudad = "Madrid"` |
| **Cambiar una variable ya creada** | `$variable = nuevo_valor` | `$ciudad = "Lima"` |
| **Unir textos** | `texto1 . texto2` o `"${variable}"` | `"Hola, " . $nombre` |
| **Leer lo que escribe el usuario** | `cin >> $variable` | `cin >> $nombre` |

### Las Tres Reglas de Oro para empezar sin tropiezos

1. **Toda variable empieza con `$`**: Escribe siempre `$nombre`, `$edad`, `$total`.
2. **Declara antes de usar**: Usa `var $x = ...` (o el tipo como `int $x = ...`) la primera vez que crees una variable.
3. **Todo lo que se abre, se cierra**: Comillas `"..."`, paréntesis `(...)` y llaves `{...}` van siempre en pareja.

---

## 10. Si algo no funciona: Diagnóstico rápido

| Síntoma o mensaje | Causa probable | Cómo solucionarlo |
|---|---|---|
| `joss: command not found` o `el término 'joss' no se reconoce` | La terminal aún no recargó el `PATH` tras instalar Joss. | Cierra la ventana de la terminal, ábrela de nuevo y prueba `joss version`. |
| `No se encontró el archivo 'main.joss'` | No has entrado a la carpeta del proyecto que creaste. | Ejecuta `cd mi-primer-joss` antes de correr `joss run main.joss`. |
| `JOSS-PARSE-001` (Error de sintaxis) | Falta cerrar una comilla `"`, un paréntesis `)` o una llave `}`. | Fíjate en el número de línea que te indica Joss y verifica que cada comilla y llave tenga su pareja. |
| `JOSS-SYM-001: Variable '$x' is not declared` | Escribiste `$x = valor` sin haber creado antes la variable, o escribiste mal su nombre. | Agrega `var` al crearla por primera vez (`var $x = valor`) y revisa mayúsculas/minúsculas (`$edad` y `$Edad` son distintas). |

---

## 11. Práctica del capítulo

Pon a prueba lo aprendido directamente en el archivo `main.joss` de tu proyecto `mi-primer-joss`:

### 1. Prueba rápida
En el ejemplo de la sección 7:
1. Cambia `"Ada"` por tu propio nombre.
2. Agrega una tercera línea `print("Estoy aprendiendo a programar")` antes de cerrar la llave `}` de `Init main()`.
3. Ejecuta `joss run main.joss` y comprueba que aparezcan las tres líneas en orden.

### 2. Ejercicio guiado: Tu tarjeta de presentación
Modifica `Init main()` siguiendo estos 4 pasos:
1. Declara una variable con tu nombre: `var $nombre = "Tu Nombre"`.
2. Declara una variable con tu país: `var $pais = "Tu País"`.
3. Declara una variable con tu meta: `var $meta = "Crear aplicaciones en Joss"`.
4. Usa `print` y el operador `.` (o `${...}`) para mostrar un resumen de tres líneas en la terminal.

### 3. Reto: Calculadora del doble de un número
1. Declara una variable entera `int $numero = 0`.
2. Muestra un mensaje con `print("Escribe un número entero:")`.
3. Lee el número del teclado con `cin >> $numero`.
4. Muestra en pantalla: `"El doble de tu número es: " . ($numero * 2)`.
5. *(Opcional)* Ejecuta `joss build main.joss` y observa cómo Joss crea tu propio programa ejecutable dentro de la carpeta `build/`.

---

## Siguiente paso

Ahora que ya sabes crear un proyecto con `joss new console`, entender `main.joss`, mostrar mensajes, crear variables y pedir datos por teclado, es momento de conocer todos los tipos de datos que maneja Joss (números exactos para dinero, decimales, booleanos y texto) y cómo realizar operaciones con ellos.

Continúa con: [Valores, variables y operaciones fundamentales](FUNDAMENTOS.md).


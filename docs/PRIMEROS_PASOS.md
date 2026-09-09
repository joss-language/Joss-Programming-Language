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

### Las capas del ecosistema Joss

Es importante distinguir tres conceptos que a veces se confunden:

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. Tu código fuente (.joss)                                 │
│    El texto legible que tú escribes con tus instrucciones.  │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. El analizador semántico (Semantic Analyzer)              │
│    Revisa tipos, nombres, visibilidad y coherencia.          │
└──────────────────────────────┬──────────────────────────────┘
                               │ Si no hay errores
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. El motor de ejecución (Runtime de Joss en Go)             │
│    Ejecuta las instrucciones reales en tu máquina física.    │
└──────────────────────────────┴──────────────────────────────┘
```

- **El lenguaje Joss**: Las reglas de escritura, palabras reservadas y sintaxis que aprenderás.
- **La herramienta CLI (`joss`)**: El ejecutable de línea de comandos que utilizas desde tu terminal para analizar, dar formato, probar y poner en marcha tus proyectos.
- **El Runtime**: El motor que toma tu programa comprobado y realiza las operaciones reales en el procesador y la memoria de tu equipo.

---

## 3. Instalación paso a paso

Una **terminal** (o consola) es una ventana de texto donde te comunicas con el sistema operativo escribiendo comandos en lugar de hacer clic con el ratón.
- En **Windows**: Puedes abrir **PowerShell** o la aplicación **Terminal de Windows** (búscala en el menú Inicio).
- En **Linux o macOS**: Abre la aplicación llamada **Terminal**.

### Opción A: Instalador automático oficial (Recomendado)

En **Windows** (abre PowerShell como usuario estándar o Administrador):

```powershell
iwr -useb https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.ps1 | iex
```

En **Linux o macOS** (abre tu Terminal):

```bash
curl -fsSL https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.sh | bash
```

Este comando descargará el binario compilado de Joss, lo ubicará en una carpeta estándar del sistema y lo registrará en tu `PATH`.

> [!NOTE]
> **¿Qué es el `PATH`?**
> El `PATH` es una lista interna del sistema operativo con las rutas donde residen los programas que puedes invocar escribiendo solo su nombre. Si agregas Joss al `PATH`, puedes escribir `joss` en cualquier carpeta sin tener que escribir la ruta completa al ejecutable. Si acabas de instalarlo y tu terminal no lo reconoce, **cierra la terminal y ábrela de nuevo**.

### Opción B: Descarga manual desde GitHub Releases

1. Ve a la sección de lanzamientos: [GitHub Releases de Joss](https://github.com/josprox/Joss-language/releases).
2. Descarga el paquete comprimido `.zip` o `.tar.gz` correspondiente a tu arquitectura (`windows_amd64`, `linux_amd64`, `darwin_arm64`, etc.).
3. Descomprime el archivo y coloca el ejecutable `joss` (o `joss.exe`) en una carpeta accesible de tu disco.
4. Agrega dicha carpeta a las variables de entorno de tu sistema (`PATH`).

### Opción C: Compilar desde el código fuente con Go

Si eres desarrollador y tienes [Go](https://go.dev) instalado (versión 1.22 o superior), puedes clonar este repositorio y compilarlo en segundos:

```bash
git clone https://github.com/josprox/Joss-language.git
cd Joss-language
go build -o joss ./cmd/joss
```

En Windows se creará `joss.exe`; en Linux/macOS se creará el binario ejecutable `joss`.

### Comprobar que la instalación funciona

Abre una terminal y escribe:

```bash
joss version
```

Deberías ver en pantalla la versión instalada de Joss (por ejemplo, `Joss version 3.6.7.2`). Si ves ese mensaje, ¡tu entorno está 100% listo para programar!

---

## 4. Escribir y ejecutar tu primer programa

Vamos a crear el clásico programa que todo programador escribe al empezar: mostrar un saludo en la pantalla.

### Paso 1: Crear una carpeta de trabajo

Crea una carpeta limpia para tus experimentos. En tu terminal:

```bash
mkdir mi-primer-joss
cd mi-primer-joss
```

### Paso 2: Crear el archivo `hola.joss`

Abre tu editor de texto preferido (por ejemplo, Visual Studio Code escribiendo `code .` en esa carpeta) y crea un nuevo archivo llamado:

`hola.joss`

> [!CAUTION]
> Asegúrate de que el archivo termine exactamente en `.joss`. En Windows, si las extensiones conocidas están ocultas, el bloc de notas podría guardarlo como `hola.joss.txt`, lo cual impedirá que Joss lo reconozca como código fuente.

Escribe dentro del archivo exactamente la siguiente línea:

<!-- joss-run: ["Hola, Joss!"] -->
```joss
print("Hola, Joss!")
```

Guarda el archivo.

### Paso 3: Entender cada parte de esa línea

Veamos qué significa cada carácter:

1. `print`: Es el nombre de una **función nativa** incorporada en Joss. Su propósito exclusivo es recibir una información y escribirla en la pantalla (salida estándar), agregando al final un salto de línea para que la siguiente instrucción empiece en el renglón de abajo.
2. `(` y `)`: Los paréntesis le indican a Joss que estás **llamando** (ejecutando) la función `print`. Dentro de los paréntesis colocas los datos de entrada que la función necesita para trabajar. A estos datos de entrada se les llama **argumentos**.
3. `"Hola, Joss!"`: Es un valor de tipo **texto** (técnicamente llamado `string` o cadena de caracteres). Las comillas dobles `"` sirven para marcar exactamente dónde empieza y dónde termina el texto. Las comillas no se muestran en pantalla; solo delimitan el contenido.

### Paso 4: Analizar y ejecutar el programa

Vuelve a tu terminal, asegúrate de estar ubicado en la carpeta `mi-primer-joss` y escribe:

```bash
joss run hola.joss
```

Inmediatamente verás la salida en la terminal:

```text
Hola, Joss!
```

¡Felicidades! Acabas de escribir, procesar y ejecutar tu primer programa en Joss.

### Modo interactivo para pruebas rápidas: `joss repl`

Si quieres experimentar con operaciones matemáticas, variables o funciones pequeñas sin necesidad de crear un archivo en el disco, puedes abrir la **consola interactiva (REPL)** de Joss escribiendo en tu terminal:

```bash
joss repl
```

Verás un cursor de bienvenida:
```text
Joss Interactive REPL (v3.7.0)
Escribe expresiones, sentencias o 'exit' / 'quit' para salir.
>>> $x = 10
>>> $x * 5
50
>>> print("Hola desde el REPL!")
Hola desde el REPL!
>>> exit
```

Las variables que crees se recordarán entre línea y línea mientras la sesión esté abierta. Para salir, simplemente escribe `exit` o `quit`.

### Extensión de VS Code y formateo automático

Si utilizas Visual Studio Code con la extensión oficial de Joss, puedes ordenar y alinear automáticamente tu código según las normas de estilo canónicas en cualquier momento con el atajo estándar:
- En **Windows y Linux**: `Shift + Alt + F`
- En **macOS**: `Shift + Option + F` (o clic derecho → *Dar formato al documento*)

---

## 5. El comando `analyze`: Tu red de seguridad

Antes de poner a correr un programa grande o desplegarlo en un servidor, Joss te permite verificarlo estáticamente mediante el comando `analyze`:

```bash
joss analyze hola.joss
```

Si el código es correcto y seguro, el analizador terminará con éxito:

```text
[Analyzer] Análisis completado sin errores.
```

¿Qué hace exactamente el analizador?
- **Revisa la sintaxis**: Comprueba que no hayas olvidado comillas, paréntesis o llaves.
- **Valida los nombres de variables y funciones**: Comprueba que no estés llamando a cosas que no existen.
- **Comprueba los tipos de datos**: Si dijiste que una variable era un número entero, comprueba que no intentes asignarle una lista de usuarios.
- **Verifica las rutas de retorno**: Comprueba que tus funciones devuelvan siempre un valor coherente en cualquier camino posible.

El comando `joss run` realiza internamente este análisis antes de ejecutar. Si hay un error bloqueante (`Severity: error`), Joss se negará a ejecutarlo para proteger tu sistema de comportamientos erráticos.

---

## 6. Modificar el programa: Usar memoria y variables

Un programa que solo muestra texto fijo es poco interactivo. Los programas reales guardan información en la memoria de la computadora para recuperarla o transformarla más adelante. Para hacer esto se usan **variables**.

Modifica tu archivo `hola.joss` para que contenga:

<!-- joss-run: ["Hola, Ada", "Bienvenida a Joss"] -->
```joss
$nombre = "Ada"
print("Hola, " . $nombre)
print("Bienvenida a Joss")
```

Guarda y ejecuta:

```bash
joss run hola.joss
```

Salida:

```text
Hola, Ada
Bienvenida a Joss
```

### ¿Qué ha cambiado aquí?

1. `$nombre = "Ada"`:
   - El símbolo `$` al principio indica que estamos declarando o usando una variable. En Joss, **todas las variables empiezan con `$`**.
   - El signo `=` se llama **operador de asignación**. Toma el valor que está a la derecha (`"Ada"`) y lo guarda en la "caja" de memoria identificada con el nombre `$nombre`.
   - Joss infiere automáticamente que `$nombre` guarda texto (`string`).
2. `"Hola, " . $nombre`:
   - El punto `.` es el **operador de concatenación**. Se utiliza para unir dos piezas de texto en una sola. Aquí une `"Hola, "` con el contenido que hay dentro de `$nombre` (`"Ada"`), produciendo el texto `"Hola, Ada"`.
3. Segunda llamada a `print`:
   - Muestra la siguiente línea de forma independiente.

Prueba a cambiar `"Ada"` por tu propio nombre en la primera línea, guarda el archivo y vuelve a ejecutar `joss run hola.joss`. Verás cómo el saludo cambia automáticamente.

---

## 7. Si algo no funciona: Diagnóstico rápido

Cuando estás aprendiendo a programar, equivocarse no solo es normal, ¡es la mejor forma de entender cómo piensa la computadora!

Aquí tienes una tabla con los problemas más frecuentes y cómo resolverlos:

| Síntoma o mensaje | Causa probable | Cómo solucionarlo |
|---|---|---|
| `joss: command not found` o `el término 'joss' no se reconoce` | La terminal no sabe dónde está el ejecutable `joss`. | Cierra y vuelve a abrir tu terminal. Si persiste, revisa que la carpeta de Joss esté incluida en tu variable de entorno `PATH`. |
| `Error: No se pudo leer el archivo 'hola.joss'` (`JOSS-IO-001`) | La terminal no encuentra el archivo en la carpeta actual. | Escribe `ls` (en Linux/macOS o PowerShell) o `dir` (en Windows CMD) para ver los archivos de la carpeta. Verifica si el nombre está bien escrito o si estás en el directorio correcto con `cd`. |
| `JOSS-PARSE-001: Error de sintaxis` | Falta cerrar una comilla `"`, un paréntesis `)` o hay un carácter inesperado. | Lee la línea y columna señaladas por el mensaje de error. Revisa que todas las comillas y paréntesis estén debidamente emparejados. |
| `JOSS-SYM-001: Variable no definida` | Intentaste leer una variable que nunca fue creada o su nombre tiene una errata. | Recuerda incluir siempre el `$`. Verifica las mayúsculas y minúsculas (`$nombre` y `$Nombre` son dos variables completamente distintas para Joss). |
| Decenas de errores sobre archivos que no reconoces | Estás ejecutando `joss analyze` o `joss run` dentro de una carpeta que contiene otros proyectos o archivos `.joss` previos. | Trabaja siempre dentro de una carpeta vacía dedicada a tu proyecto. Joss descubre automáticamente todos los archivos `.joss` de la carpeta actual y sus subdirectorios. |

---

## 8. Ejercicios prácticos

Para afianzar lo que acabas de aprender antes de continuar:

1. **Tu tarjeta de presentación**: Escribe un programa `tarjeta.joss` que declare tres variables: `$miNombre`, `$miPais` y `$miProfesion`. Luego, usando `print` y concatenación con `.`, muestra un mensaje en pantalla que arme un párrafo presentándote.
2. **Experimentar con errores**: Quita a propósito la comilla final del texto en `print("Hola)` y ejecuta `joss analyze`. Observa cómo Joss te indica exactamente el número de línea donde detectó el problema. Vuelve a colocar la comilla para que quede limpio.

---

## Siguiente paso

Ahora que ya sabes qué es Joss, cómo crear un archivo y cómo imprimir datos en pantalla, es momento de aprender en profundidad los tipos de datos que puede manejar una computadora, cómo operar números y cómo decidir qué tipo de variable utilizar.

Continúa con: [Valores, variables y operaciones fundamentales](FUNDAMENTOS.md).

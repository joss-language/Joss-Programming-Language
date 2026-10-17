# Glosario

[Índice](README.md) · [Primeros pasos](PRIMEROS_PASOS.md) · [Referencia](SINTAXIS.md)

Estas palabras se explican también donde se introducen. Puedes volver aquí
sin interrumpir el recorrido de aprendizaje.

| Término | Significado en esta documentación |
|---|---|
| Programa | Instrucciones guardadas en archivos para que una computadora realice una tarea. |
| Proyecto | Carpeta organizada creada con `joss new` o `joss new console` que contiene tu código, configuración y estructura lista para trabajar. |
| `main.joss` | Archivo principal de entrada de todo proyecto Joss. Es el primer archivo que Joss busca al ejecutar o compilar tu aplicación. |
| `Init main()` | Bloque de arranque dentro de `public class Main` en `main.joss`. Todo lo que escribas dentro de sus llaves `{ ... }` se ejecuta al iniciar el programa. |
| Valor | Un dato concreto: `12`, `"Ana"`, `true`. |
| Variable | Nombre con prefijo `$` que permite guardar, consultar o cambiar un valor en memoria; por ejemplo `$edad`. |
| Declaración | Acción de crear una variable por primera vez indicando `var` (`var $x = 1`), un tipo explícito (`int $x = 1`), `mixed` o `const`. |
| Reasignación | Acción de cambiar el valor de una variable que ya fue declarada previamente (`$x = 2`). |
| Binding | Asociación entre un nombre y su valor. Una constante (`const`) protege esa asociación para que no pueda reasignarse. |
| Tipo | Categoría de datos que una operación o variable admite. Por ejemplo, `int` representa números enteros y `decimal` importes monetarios exactos. |
| Inferencia | Deducción automática del tipo de una variable a partir de su valor inicial (como en `var $edad = 20`, donde Joss deduce `int`). |
| Conversión / casting | Obtención de un valor en otro tipo mediante funciones como `intval(...)`, `decimal(...)` o `strval(...)`. |
| Interpolación | Incrustar variables o cálculos dentro de un texto con comillas dobles usando `${nombre}` o `${$precio * 2}`. |
| `mixed` | Decisión explícita de permitir que una variable o parámetro acepte valores de distintos tipos a lo largo del tiempo. |
| `void` | Tipo de retorno que indica explícitamente que una función realiza acciones pero no devuelve ningún valor. |
| `unknown` | Estado interno del analizador cuando aún no conoce un tipo; no es una palabra para declarar variables. |
| `null` / `nil` | Ausencia de valor; `null` es la forma canónica recomendada y `nil` se acepta como sinónimo de compatibilidad. |
| Nullable | Tipo que permite un valor concreto o `null`, escrito como `string|null` o con el atajo `string?`. |
| Expresión | Fragmento de código que se evalúa y produce un valor: `2 + 3`, `"Hola " . $nombre` o `($edad >= 18) ? "Mayor" : "Menor"`. |
| Sentencia | Instrucción que realiza un paso dentro del programa: declarar una variable, mostrar un mensaje, repetir un ciclo o retornar. |
| Bloque | Grupo de instrucciones delimitadas entre llaves `{` y `}`. |
| Función | Máquina reutilizable de código (`public func` o `private func`) que recibe datos de entrada y puede devolver un resultado. |
| Parámetro | Variable tipada declarada entre los paréntesis de una función (`int $a`) que define qué dato espera recibir. |
| Argumento | Valor real entregado entre los paréntesis cuando llamas a una función (`sumar(5, 10)`). |
| Retorno | Resultado que una función devuelve a quien la llamó mediante la palabra `return`. |
| `guard` | Cláusula de guarda (`guard (condicion) : { return ... }`) que exige que se cumpla una condición o sale inmediatamente de la función. |
| `defer` | Instrucción que agenda un bloque para ejecutarse justo antes de que termine la función o archivo actual, útil para cerrar recursos. |
| Callable | Cualquier valor que puede invocarse con paréntesis `()`: una función, un método o una closure. |
| Ámbito / scope | Zona del programa donde una variable existe y puede utilizarse. Cada llamada a función tiene su propio ámbito aislado. |
| Closure | Función anónima (`func(...) { ... }`) que puede guardarse en una variable y recordar variables del entorno donde fue creada. |
| Recursión | Técnica en la que una función se llama a sí misma para resolver un problema en pasos más pequeños. |
| Clase | Plano o molde (`public class Nombre`) que agrupa datos (propiedades) y comportamientos (métodos). |
| `record` | Forma concisa (`public record Nombre(...)`) de declarar una clase cuyos datos quedan protegidos como constantes tras crearse. |
| Instancia / Objeto | Entidad concreta creada en memoria a partir de una clase usando `new Nombre(...)`. |
| Propiedad | Variable que pertenece a una clase o instancia (accedida con `$this->propiedad` o `$objeto->propiedad`). |
| Método | Función que pertenece a una clase; se invoca con `->` sobre una instancia o con `::` si es un método estático de la clase. |
| Constructor | Método especial (`public func constructor(...)` o `Init(...)`) que prepara los datos iniciales al crear un objeto con `new`. |
| Herencia | Capacidad de una clase para reutilizar y ampliar otra clase base mediante `extends`. |
| Interfaz | Contrato formal (`public interface`) que enumera qué métodos públicos debe implementar una clase mediante `implements`. |
| Encapsulación | Protección de los miembros de una clase usando `public` (abierto), `protected` (clase e hijas) o `private` (solo la misma clase). |
| Array | Lista ordenada de elementos entre corchetes `[10, 20]`, cuyas posiciones (índices) empiezan a contar desde `0`. |
| Map | Diccionario de pares clave → valor escrito como `{"clave": valor}` o `["clave" => valor]`. |
| Referencia | Acceso compartido al mismo dato. Con `ref` (`incrementar(ref $x)`), permite que una función modifique temporalmente una variable del llamador. |
| Mutabilidad | Posibilidad de modificar el valor de una variable o los elementos internos de una colección. |
| Copia superficial | Duplicado del contenedor exterior de una colección cuyos elementos internos complejos siguen compartiéndose. |
| Error / Diagnóstico | Aviso estructurado (`JOSS-...`) con archivo, línea, explicación y sugerencia que señala un problema en el código. |
| Excepción | Fallo en tiempo de ejecución que se lanza con `throw` y puede interceptarse con `try { ... } catch ($e) { ... }`. |
| Pila de llamadas / stack | Cadena de funciones en espera mientras se ejecuta una llamada interna. No confundir con la clase de estructura de datos `Stack`. |
| Heap | Memoria administrada automáticamente donde viven los objetos, arrays y maps creados durante la ejecución. |
| Sincronía | Ejecución paso a paso en la que cada instrucción espera a que termine la anterior. |
| Concurrencia | Avance coordinado de varias tareas al mismo tiempo. |
| Asincronía | Delegar una tarea con `async { ... }` para que se ejecute sin bloquear el programa principal. |
| Future | Objeto devuelto por un bloque `async` que representa el resultado futuro de esa tarea. |
| `await` | Palabra clave (`await $tarea` o `await($tarea)`) que espera y obtiene el valor final de un `Future`. |
| Channel | Canal tipado (`channel<T>` / `make_chan()`) para enviar (`<<`) y recibir (`<-`) mensajes seguros entre tareas concurrentes. |
| Runtime | Motor de ejecución de Joss que gestiona la memoria, la concurrencia y las clases nativas integradas. |
| Lexer | Primer paso del procesador de Joss: convierte el texto del archivo `.joss` en piezas indivisibles llamadas tokens. |
| Token | Unidad léxica reconocida: una palabra clave, una variable, un número o un operador. |
| Parser | Segundo paso del procesador: organiza los tokens en un árbol sintáctico comprobando las reglas de escritura. |
| AST | Árbol de Sintaxis Abstracta que representa la estructura lógica de tu código. |
| Analizador / analyzer | Red de seguridad estática que verifica tipos, nombres, contratos y retornos antes de ejecutar o compilar, generando un `PreparedProgram`. |
| Interpretación (`joss run`) | Ejecución inmediata del programa validado directamente desde la terminal. |
| Compilación nativa (`joss build`) | Transformación del mismo programa Joss a un archivo ejecutable nativo independiente (`.exe` en Windows o binario en Linux/macOS/Android). |
| Bytecode (`.jp`) | Representación binaria firmada criptográficamente utilizada para distribuir paquetes y plugins de Joss. |
| Módulo | Organización automática de archivos dentro del proyecto (`app/`); en Joss las declaraciones públicas se descubren sin escribir `import`. |
| Paquete | Librería reutilizable distribuida y gestionada con `joss pub`. |
| Plugin | Extensión compilada en formato `.jp` que añade capacidades o comandos al proyecto. |
| CLI | Herramienta de línea de comandos (`joss`) que controlas escribiendo órdenes en la terminal. |
| Terminal / Consola | Ventana del sistema operativo (como PowerShell o Terminal) donde escribes comandos de texto y ves los resultados. |
| Ruta HTTP | Regla en `routes.joss` o `api.joss` que conecta una dirección web (URL) con el controlador encargado de responderla. |
| Controlador | Clase ubicada en `app/controllers/` que recibe la petición del usuario, procesa la lógica y devuelve una vista o JSON. |
| Middleware | Filtro intermedio que revisa o protege una petición web antes de que llegue al controlador (por ejemplo, verificar si inició sesión). |
| Vista | Plantilla HTML dinámica (`.joss.html`) en `app/views/` que muestra datos al usuario escapando código malicioso automáticamente. |
| Migración | Archivo en `app/database/migrations/` que crea o actualiza tablas de la base de datos de forma ordenada con `joss migrate`. |
| Query builder / ORM | Sistema integrado (`Model` y `GranDB`) para consultar y guardar datos en bases de datos sin concatenar SQL inseguro. |
| API | Conjunto de funciones, métodos o rutas web que un programa ofrece para comunicarse con otro. |

# Manejo de errores, excepciones y diagnóstico

Antes: [Funciones, closures y referencias](FUNCIONES.md). Después: [Clases, objetos y herencia](CLASES.md).
Referencia técnica: [Diagnósticos estructurados](DIAGNOSTICOS.md), [Analizador semántico](ANALIZADOR.md).

---

## ¿Qué vas a aprender aquí?

En un mundo ideal, los programas siempre recibirían los datos correctos, los discos duros nunca se llenarían y las conexiones de red jamás fallarían. En el mundo real, los errores son inevitables:
- Un usuario escribe letras donde se esperaba un número de tarjeta.
- Un archivo de configuración que el programa necesita fue borrado por accidente.
- El servidor de base de datos se reinicia en medio de una transacción.

Un programa profesional no es aquel que nunca encuentra problemas, sino aquel que **sabe anticiparlos, contenerlos y recuperarse sin colapsar**.

En esta guía aprenderás:
1. Las tres fases temporales donde se detectan los errores: sintaxis, análisis estático y tiempo de ejecución (*runtime*).
2. Cómo leer e interpretar un mensaje de diagnóstico estructurado de Joss.
3. El mecanismo de control de excepciones con **`try`**, **`catch`** y **`throw`**.
4. Qué información contiene la variable de error capturada (el mapa `$error`).
5. Por qué las instrucciones `return`, `break` y `continue` dentro de un bloque `try` no son interferidas por `catch`.
6. Cuándo manejar errores mediante excepciones vs cuándo comprobar valores de retorno (`null`).

---

## 1. Las tres fases de un error

Para solucionar un problema rápidamente, lo primero es identificar en qué fase del ciclo de vida del programa ocurrió:

```text
Código fuente (.joss)
         │
         ▼
[ 1. PARSER ] ────────► ¿Error de Sintaxis?
         │              (Falta cerrar paréntesis, comilla o llave)
         ▼
[ 2. ANALYZER ] ──────► ¿Error Semántico o de Tipo?
         │              (Variable no definida, tipo incompatible, retorno ausente)
         ▼
[ 3. RUNTIME ] ───────► ¿Excepción en Ejecución?
                        (Archivo no encontrado, división por cero, throw explícito)
```

| Fase | Cuándo ocurre | Ejemplo | Cómo se resuelve |
|---|---|---|---|
| **Sintaxis** | Durante la lectura inicial del texto | `print("Hola)` (comilla sin cerrar) | Corrige la puntuación señalada por el parser. |
| **Análisis** | Durante la verificación estática previa | `int $x = "texto"` (`JOSS-TYPE-001`) | Corrige la discrepancia de tipos o nombres antes de ejecutar. |
| **Ejecución (Runtime)** | Mientras el programa está corriendo en memoria | `$n / 0` o un fallo de red | Se intercepta y gestiona con bloques `try / catch`. |

---

## 2. Anatomía de un mensaje de diagnóstico

Cuando Joss detecta un problema antes de ejecutar, imprime un diagnóstico estructurado. Por ejemplo:

```text
error[JOSS-TYPE-001] app.joss:15:5: Asignación incompatible: se esperaba 'int', se obtuvo 'string'.
  --> app.joss:15:5
   |
15 |     $edad = "veinte"
   |     ^^^^^
Sugerencia: Modifica el valor asignado o declara la variable como 'mixed'.
```

Cada parte del mensaje tiene un propósito:
1. **Severidad (`error` o `warning`)**:
   - `error`: Problema crítico. Joss se negará a ejecutar el programa para evitar comportamientos impredecibles.
   - `warning`: Aviso informativo (como una variable declarada que nunca se usó). El programa puede ejecutarse, pero se recomienda limpiarlo.
2. **Código estable (`JOSS-TYPE-001`)**: Un identificador único que te permite buscar la causa exacta y ejemplos en la [Referencia de diagnósticos](DIAGNOSTICOS.md).
3. **Ubicación (`app.joss:15:5`)**: El archivo exacto, número de línea (`15`) y columna (`5`) donde se detectó la discrepancia.
4. **Explicación y Sugerencia**: Una descripción en lenguaje natural de qué regla se rompió y cómo repararla.

---

## 3. Manejo de excepciones: `try`, `catch` y `throw`

Una **excepción** es una señal de alarma que interrumpe el flujo normal del programa cuando ocurre una situación imprevista que el código actual no puede resolver por sí mismo.

- **`throw`**: Lanza la alarma (la excepción).
- **`try`**: Delimita una zona protegida de código donde sospechamos que algo podría fallar.
- **`catch ($error)`**: Es la brigada de emergencia. Si algo explota dentro del bloque `try`, la ejecución salta de inmediato al bloque `catch` para mitigar el problema:

<!-- joss-run: ["No se pudo continuar: faltan datos"] -->
```joss
try {
    throw "faltan datos"
} catch ($error) {
    print("No se pudo continuar: " . $error)
}
```

Salida en la terminal:

```text
No se pudo continuar: faltan datos
```

### ¿Qué sucede en este ejemplo?
1. La computadora entra al bloque `try`.
2. Ejecuta `throw "faltan datos"`. En ese instante, la ejecución normal se detiene.
3. El control salta directamente al bloque `catch`.
4. El mensaje `"faltan datos"` queda depositado en la variable `$error`.
5. El bloque `catch` imprime el aviso.
6. El programa no se estrella; continúa ejecutando las líneas posteriores al `catch`.

---

## 4. Inspección avanzada del objeto de error en `catch`

Cuando el propio motor de ejecución de Joss genera un error interno (llamado `JossError`, como un desbordamiento aritmético o un índice fuera de rango), la variable `$e` del `catch` recibe en tiempo de ejecución un **mapa asociativo** con información técnica detallada:

```joss
try {
    var $arr = [1, 2]
    print($arr[99]) // Índice fuera de rango
} catch ($e) {
    print("Mensaje: " . $e["message"])
    print("Tipo: " . $e["type"])
    print("Archivo: " . $e["file"])
    print("Línea: " . $e["line"])
}
```

### Campos disponibles en el mapa de error interno:
- `$e["message"]`: El mensaje descriptivo del fallo.
- `$e["type"]`: La categoría interna del error (por ejemplo, `"IndexOutOfRange"` o `"ArithmeticFault"`).
- `$e["file"]`: La ruta al archivo fuente donde se originó el fallo.
- `$e["line"]`: El número de línea exacto.
- `$e["error"]`: La representación textual completa del error.

### Excepciones estructuradas con mapas o instancias de clase

También puedes lanzar un mapa asociativo con tus propios campos de contexto, o bien una instancia de una clase personalizada (veremos cómo construir clases en detalle en el siguiente capítulo, [Clases, objetos y herencia](CLASES.md)):

<!-- joss-run: ["Campo: email", "Motivo: Formato inválido"] -->
```joss
public class ErrorValidacion {
    Init(public string $campo, public string $motivo) {}
}

try {
    throw new ErrorValidacion("email", "Formato inválido")
} catch ($e) {
    print("Campo: " . $e->campo)
    print("Motivo: " . $e->motivo)
}
```

Salida en la terminal:

```text
Campo: email
Motivo: Formato inválido
```

---

## 5. Garantía de flujo: `return` y bucles dentro de `try`

En muchos lenguajes, meter sentencias de control dentro de un bloque protegido puede causar comportamientos inesperados. En Joss:

- Si ejecutas `return $valor` dentro de un bloque `try`, la función retornará inmediatamente y el bloque `catch` **no intervendrá**.
- Si ejecutas `break` o `continue` dentro de un `try` que está en un bucle, el ciclo se interrumpirá o avanzará normalmente sin que `catch` confunda el salto con una excepción.

Joss distingue internamente las señales de control de flujo de los errores de usuario reales, garantizando que tus cláusulas de guarda funcionen de forma 100% predecible.

---

## 6. Excepciones vs Comprobación de retornos

No todos los problemas deben manejarse con `try / catch`. En la biblioteca estándar de Joss, muchas operaciones devuelven un valor especial (`null` o `false`) cuando una consulta simplemente no encuentra resultados.

Por ejemplo, leer un archivo que no existe:

<!-- joss-run: ["No se pudo leer el archivo"] -->
```joss
var $contenido = file_get_contents("archivo-que-no-existe.txt")
($contenido == null) ? {
    print("No se pudo leer el archivo")
} : {
    print($contenido)
}
```

Salida en la terminal:

```text
No se pudo leer el archivo
```

- `file_get_contents(...)`: Devuelve el texto del archivo si existe, o `null` si no se pudo leer. No lanza una excepción destructiva; te permite comprobar el resultado con un ternario simple.
- `file_put_contents(...)`: Devuelve `true` si el archivo se escribió en disco o `false` si hubo un fallo de permisos.

### ¿Cuándo usar cada enfoque?
- **Usa comprobación de retorno (`$res == null`)**: Cuando la ausencia del dato sea una posibilidad normal de la aplicación (un usuario que busca un producto que no existe en el catálogo).
- **Usa `throw` y `try / catch`**: Cuando el fallo represente una condición crítica o anormal de la que el código local no puede recuperarse (se perdió la conexión con el servidor de pagos en medio del cobro, o faltan variables de entorno esenciales para arrancar).

---

## 7. Antipatrones: Qué NO debes hacer

> [!CAUTION]
> **Nunca silencies errores con un `catch` vacío**:

```joss
// MALO: Esconde bugs catastróficos
try {
    iniciarBaseDeDatos()
} catch ($e) {}
```

> Si la base de datos no pudo iniciar, el programa continuará corriendo a ciegas y fallará de forma incomprensible diez líneas más adelante. Como mínimo, registra el error en consola con `print($e)` o cancela la ejecución.

---

## 8. Ejercicios prácticos

### Nivel 1 — Prueba rápida
Escribe un bloque `try / catch ($err)` donde dentro del `try` lances la excepción `throw "Conexión rechazada"`. En el `catch`, imprime `"Alerta: " . $err`.

### Nivel 2 — Ejercicio guiado
1. Escribe una función `public func registrarEdad(int $edad): string`.
2. Si `$edad < 0`, lanza una excepción: `throw "La edad no puede ser negativa"`.
3. Si `$edad < 18`, lanza: `throw "Debe ser mayor de edad para registrarse"`.
4. Si es válida, retorna `"Registro exitoso"`.
5. Llama a la función dentro de un bloque `try / catch`, probando con `-5`, `15` y `25`, e imprime el resultado o el mensaje de error capturado.

### Nivel 3 — Reto
Crea una función `public func dividirSeguro(int $numerador, int $denominador): int` que:
1. Use un bloque `try / catch ($e)` para realizar la operación `$numerador / $denominador`.
2. Si ocurre un error aritmético (como dividir entre `0`), capture el mapa `$e`, imprima `"Fallo detectado: " . $e["type"]` y retorne `0`.
3. Prueba llamarla con `dividirSeguro(20, 4)` y `dividirSeguro(20, 0)`.

---

## Siguiente paso

Ahora que sabes cómo organizar funciones, manipular colecciones y proteger tu programa frente a fallos, estás listo para empaquetar datos y comportamientos juntos mediante la Programación Orientada a Objetos.

Continúa con: [Clases, objetos, métodos y herencia](CLASES.md).

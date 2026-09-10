# Catálogo y referencia de diagnósticos del lenguaje

Antes: [Manejo de errores y excepciones](ERRORES.md). Después: [Analizador semántico AST](ANALIZADOR.md).
Índice general: [Documentación de Joss](README.md).

---

## ¿Qué es un diagnóstico en Joss?

Un **diagnóstico** es un informe técnico estructurado emitido por las herramientas de Joss (el compilador, el analizador semántico `joss analyze`, el linter o el runtime defensivo) cuando se detecta una violación a las reglas sintácticas, de tipos o de seguridad del lenguaje.

A diferencia de los mensajes de error genéricos de herramientas antiguas, cada diagnóstico de Joss está diseñado siguiendo estos principios:
1. **Identificador único y estable**: Cada regla tiene un código inmutable (por ejemplo `JOSS-TYPE-001`).
2. **Ubicación exacta**: Archivo, línea y columna precisa donde se originó el conflicto.
3. **Explicación pedagógica**: Describe claramente qué regla se incumplió y por qué.
4. **Sugerencia accionable**: Propone la corrección canónica recomendada.

### Niveles de severidad

- **`error`**: Impide la ejecución del programa (`joss run` y el compilador se detienen). Representa un fallo estructural o de seguridad.
- **`warning`**: Aviso informativo (como una variable declarada que nunca se usó o una convención de nombres desalineada). `joss analyze` finaliza con código de salida `0` si solo hay advertencias, permitiendo continuar la ejecución.

---

## 1. Parser, carga de proyectos y tabla de símbolos

| Código | Categoría | Significado y Causa | Ejemplo incorrecto | Solución y Caso correcto |
|---|---|---|---|---|
| `JOSS-IO-001` | Entrada / Salida | No se puede leer el archivo fuente en disco (permisos o ruta inexistente). | `joss run fantasma.joss` | Verificar que el archivo exista en la ruta indicada con permisos de lectura. |
| `JOSS-PARSE-001` | Sintaxis | Token o estructura gramatical inválida: falta una comilla, llave o visibilidad obligatoria. | `class MiClase {}`<br>`func prueba() {}` | Usar sintaxis canónica:<br>`public class MiClase {}`<br>`public func prueba() {}` |
| `JOSS-SYM-001` | Símbolos | Variable no definida: se intenta leer una variable antes de ser creada. | `print($variable)` | Declarar e inicializar la variable antes de usarla:<br>`$variable = "valor"` |
| `JOSS-SYM-002` | Símbolos | Redeclaración de variable en el mismo ámbito léxico. | `int $x = 1`<br>`int $x = 2` | Reasignar sin volver a declarar:<br>`$x = 2` |
| `JOSS-SYM-003` | Símbolos | Función no resuelta: se invoca una función que no existe en el proyecto ni en built-ins. | `calcularTotal()` | Definir la función con `public func` o verificar la ortografía del nombre. |
| `JOSS-SYM-004` | Símbolos | Clase no resuelta: intento de instanciar (`new`) una clase no declarada ni registrada. | `$p = new Persona()` | Declarar `public class Persona {}` o cargar el plugin que la exponga. |
| `JOSS-SYM-005` | Símbolos | Herencia inválida: la clase especificada en `extends` no existe. | `public class A extends B {}` | Asegurarse de que la superclase `B` esté declarada en el proyecto. |
| `JOSS-SYM-006` | Símbolos | Intento de reasignar una constante inmutable. | `const int $MAX = 5`<br>`$MAX = 10` | Si el valor debe cambiar, declararla como variable mutable: `$MAX = 5`. |
| `JOSS-SYM-007` | Símbolos | Interfaz no resuelta: la interfaz especificada en `implements` o `extends` no existe. | `public class A implements IDesconocida {}` | Asegurarse de declarar la interfaz o verificar su ortografía. |
| `JOSS-SYM-008` | Símbolos | Herencia cíclica en interfaces: una interfaz se extiende a sí misma directa o indirectamente. | `public interface A extends B {}`<br>`public interface B extends A {}` | Romper el ciclo de herencia entre las interfaces. |
| `JOSS-DECL-001` | Declaraciones | Conflicto de nombres: dos funciones globales tienen exactamente el mismo identificador. | Dos archivos con:<br>`public func procesar() {}` | Renombrar una de las dos funciones. En Joss no hay namespaces fuente por carpeta. |
| `JOSS-DECL-002` | Declaraciones | Conflicto de clases: dos clases globales tienen el mismo nombre en el proyecto. | Dos archivos con:<br>`public class Usuario {}` | Mantener una única declaración canónica de la clase en todo el proyecto. |
| `JOSS-DECL-003` | Declaraciones | Métodos duplicados: una misma clase declara dos métodos con el mismo nombre. | `public func id() {}`<br>`public func id(int $x) {}` | Joss no admite sobrecarga de métodos por firma; usa nombres descriptivos distintos. |
| `JOSS-DECL-004` | Declaraciones | Conflicto de nombres entre interfaz y clase o interfaces duplicadas. | `public class Repo {}`<br>`public interface Repo {}` | Usar nombres únicos para cada tipo; convención sugerida: prefijo `I` para interfaces (`IRepo`). |
| `JOSS-DECL-005` | Declaraciones | Incumplimiento de contrato de interfaz: falta un método, o discrepa en cantidad/tipo de parámetros, retorno o visibilidad. | `public class MiClase implements IFigura {}` (sin `calcularArea()`) | Implementar todos los métodos de la interfaz con visibilidad `public` y tipos compatibles. |

---

## 2. Sistema de tipos, llamadas y accesibilidad

| Código | Categoría | Significado y Causa | Ejemplo incorrecto | Solución y Caso correcto |
|---|---|---|---|---|
| `JOSS-TYPE-001` | Tipos | Reasignación incompatible: se asigna un dato de tipo distinto a una variable tipada o inferida. | `$edad = 20`<br>`$edad = "veinte"` | Mantener el tipo homogéneo, o usar dinamismo voluntario:<br>`mixed $edad = 20`<br>`$edad = "veinte"` |
| `JOSS-TYPE-002` | Tipos | Valor inicial incompatible con la anotación de tipo explícita. | `int $x = "texto"` | Proveer un valor del tipo esperado o una cadena convertible según `CoerceString`. |
| `JOSS-TYPE-003` | Tipos | Argumento incompatible con el parámetro tipado de una función o método. | `public func f(int $n) {}`<br>`f("hola")` | Pasar el tipo correcto o convertir el argumento antes de la llamada. |
| `JOSS-TYPE-004` | Tipos | Operador aplicado a operandos no admitidos (ej. sumar texto con `+`). | `"hola" + " mundo"` | Para aritmética usa números; para unir texto usa el operador punto (`.`): `"hola" . " mundo"`. |
| `JOSS-TYPE-005` | Tipos | Tipo de clave no admitido en un mapa asociativo. | `{[1, 2]: "valor"}` | Las claves de un `map` deben ser de tipo `string`. |
| `JOSS-TYPE-006` | Tipos | Tipo de índice incorrecto para una colección conocida. | `$arr["clave"]` (en un array)<br>`$map[true]` (en un map) | Los arrays se indexan con enteros (`$arr[0]`); los maps con cadenas (`$map["clave"]`). |
| `JOSS-TYPE-007` | Tipos | Intento de indexar con `[...]` un valor que no es indexable (ej. un entero o booleano). | `$n = 42`<br>`print($n[0])` | Indexar únicamente arrays, maps, cadenas o instancias compatibles. |
| `JOSS-TYPE-008` | Tipos | El valor retornado por `return` no coincide con el tipo prometido en la firma `: Tipo`. | `public func f(): int {`<br>`    return "no es int"`<br>`}` | Devolver un valor compatible con la firma declarada. |
| `JOSS-TYPE-009` | Tipos | Tipo de dato o clase inexistente (incluye aliases eliminados como `integer`, `double`, `boolean`, `any`, `list`). | `integer $x = 10`<br>`boolean $flag = true` | Usar los tipos canónicos de Joss:<br>`int $x = 10`<br>`bool $flag = true` |
| `JOSS-TYPE-010` | Tipos | Función con tipo de retorno anotado puede terminar sin ejecutar un `return` o `throw`. | `public func f(int $n): string {`<br>`    $n > 0 ? { return "si" } : {}`<br>`}` | Garantizar que todas las rutas posibles retornen un valor del tipo prometido. |
| `JOSS-TYPE-011` | Tipos | Se declaró un parámetro sin tipo explícito. | `public func f($x) {}` | En Joss todos los parámetros deben declarar su tipo:<br>`public func f(int $x) {}` o `public func f(mixed $x) {}` |
| `JOSS-CALL-001` | Llamadas | Cantidad incorrecta de argumentos respecto a los parámetros de la firma conocida. | `public func f(int $a, int $b) {}`<br>`f(1)` | Proporcionar todos los argumentos obligatorios requeridos por la función. |
| `JOSS-MEMBER-001` | Miembros | Se intenta invocar un método que no existe en la clase receptora resuelta. | `$usuario->metodoInexistente()` | Comprobar el nombre del método en la definición de la clase o en el catálogo nativo. |
| `JOSS-ACCESS-001` | Visibilidad | Se intenta usar una clase o función declarada como `private` desde otro archivo. | Llamar a una función privada de otro archivo. | Declarar la función o clase como `public` si debe ser compartida en el proyecto. |
| `JOSS-ACCESS-002` | Visibilidad | Intento de acceder a una propiedad o método `private` o `protected` fuera de su ámbito autorizado. | `$cuenta->saldo` (siendo privado) | Acceder a través de métodos públicos autorizados (getters/setters). |

---

## 3. Referencias temporales (`ref`)

| Código | Categoría | Significado y Causa | Solución |
|---|---|---|---|
| `JOSS-REF-001` | Referencias | Falta el marcador bilateral `ref` en la llamada o en la definición, o cruce ilegal a nativo/async. | Si la función espera `ref int $x`, la llamada debe ser `f(ref $x)`. |
| `JOSS-REF-002` | Referencias | Se pasa una expresión, un literal, un campo de objeto o un índice de array como referencia. | Pasar únicamente variables locales mutables directas (`ref $miVariable`). |
| `JOSS-REF-003` | Referencias | La variable pasada como referencia es una constante inmutable (`const`). | Una referencia muta el valor original; no puedes pasar constantes a un parámetro `ref`. |
| `JOSS-REF-004` | Referencias | Discrepancia de tipo en la referencia (el tipo de la variable no coincide exactamente con el parámetro). | Las referencias son estrictamente invariantes: un `ref float` exige exactamente una variable `float`. |
| `JOSS-REF-005` | Referencias | Intento de almacenar, capturar en closure, retornar o hacer escapar una referencia. | Una referencia solo vive durante la llamada; para devolver datos usa `return`. |
| `JOSS-REF-006` | Referencias | Parámetro `ref` declarado con un valor por defecto. | Los parámetros por referencia no admiten valores por defecto; quita el `= valor`. |

---

## 4. Control de flujo, linter y plantillas

| Código | Severidad | Significado y Causa | Solución |
|---|---|---|---|
| `JOSS-FLOW-001` | Warning | Código inalcanzable (*dead code*): instrucciones escritas después de un `return` incondicional. | Mover las instrucciones antes del `return` o eliminarlas. |
| `JOSS-LINT-001` | Warning | Variable local declarada pero nunca leída en el cuerpo. | Utilizar la variable o retirarla para mantener el código limpio. |
| `JOSS-SYNTAX-001` | Error | Error sintáctico capturado durante la fase de análisis del linter. | Corregir la puntuación o estructura señalada por el parser. |
| `JOSS-LINT-002` | Error | Parámetro sin tipo explícito reportado por el linter. | Añadir la anotación de tipo correspondiente (`int`, `string`, `mixed`). |
| `JOSS-LINT-007` | Warning | Desviación de convenciones de nombres del proyecto (clases en PascalCase, funciones en camelCase). | Ajustar el nombre al estándar canónico. |
| `JOSS-SEC-001` | Warning | Detección heurística de un posible secreto sensible escrito en texto plano (claves API, tokens). | Mover el secreto al archivo de configuración de entorno (`env.joss`). |
| `JOSS-VIEW-001` | Error | Fallo crítico al compilar o parsear una plantilla de vista HTML. | Verificar el cierre de directivas (`@foreach`, `@endforeach`) y archivos incluidos. |
| `JOSS-VIEW-SYNTAX` | Error | Expresión de Joss mal formada dentro de una etiqueta de vista `{{ ... }}`. | Corregir la sintaxis de la expresión embebida. |
| `JOSS-VIEW-UNDEF` | Warning | Variable de vista accedida sin protección ante valores indefinidos o nulos. | Proteger con el operador de coalescencia nula: `{{ $variable ?? 'default' }}`. |

---

## 5. Errores aritméticos e indexación en tiempo de ejecución

| Código | Categoría | Operación inválida en Runtime | Cómo prevenirlo |
|---|---|---|---|
| `JOSS-ARITH-001` | Runtime | Desbordamiento de entero con signo de 64 bits (−2⁶³ a 2⁶³−1). | Validar los límites antes de operar o utilizar el tipo `decimal` para cálculos de gran escala. |
| `JOSS-ARITH-002` | Runtime | División o módulo por cero (`$x / 0` o `$x % 0`). | Comprobar que el divisor sea distinto de cero antes de ejecutar la división: `($divisor != 0) ? ($x / $divisor) : 0.0`. |
| `JOSS-INDEX-001` | Runtime | Índice fuera de rango: acceso a posición negativa o mayor/igual a la longitud. | Verificar `count($arr)` antes de indexar o comprobar la existencia con `array_key_exists`. |
| `JOSS-INDEX-002` | Runtime | Tipo de índice no compatible con la estructura de datos. | Indexar arrays con números enteros y maps con cadenas de texto. |

---

## 6. Casos completos de prueba verificados

A continuación se presentan ejemplos ejecutables que validan formalmente la emisión de los diagnósticos y su contraparte correcta:

### Tipo inexistente o alias eliminado (`JOSS-TYPE-009`)

<!-- joss-error: JOSS-TYPE-009 -->
```joss-invalid
integer $edad = 20
```

Caso corregido con el tipo canónico `int`:

<!-- joss-run: ["20"] -->
```joss
int $edad = 20
print($edad)
```

---

### Parámetro sin tipo explícito (`JOSS-TYPE-011`)

<!-- joss-error: JOSS-TYPE-011 -->
```joss-invalid
public func duplicar($valor) { return $valor * 2 }
```

Caso corregido tipando el parámetro y el retorno:

<!-- joss-run: ["6"] -->
```joss
public func duplicar(int $valor): int { return $valor * 2 }
print(duplicar(3))
```

---

### Reasignación con tipo incompatible (`JOSS-TYPE-001`)

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
$edad = 20
$edad = "veinte"
```

Caso corregido usando dinamismo voluntario (`mixed`):

<!-- joss-run: ["veinte"] -->
```joss
mixed $dato = 20
$dato = "veinte"
print($dato)
```

---

### Retorno ausente en rutas de flujo (`JOSS-TYPE-010`)

<!-- joss-error: JOSS-TYPE-010 -->
```joss-invalid
public func signo(int $n): string {
    $n > 0 ? { return "positivo" } : {}
}
```

Caso corregido garantizando retorno en todas las rutas posibles:

<!-- joss-run: ["no positivo"] -->
```joss
public func signo(int $n): string {
    return $n > 0 ? "positivo" : "no positivo"
}
print(signo(0))
```

---

## Siguiente paso

Ahora que conoces todos los diagnósticos y cómo resolverlos, puedes profundizar en cómo el analizador semántico examina el árbol de sintaxis abstracta para emitir estos códigos:

Continúa con: [Analizador estático AST](ANALIZADOR.md).

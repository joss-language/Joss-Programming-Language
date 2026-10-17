# Referencia de sintaxis y operadores

Para aprender: [fundamentos](FUNDAMENTOS.md), [flujo](CONTROL_FLUJO.md),
[colecciones](COLECCIONES.md), [funciones](FUNCIONES.md), [clases](CLASES.md).
Complementos: [gramática](GRAMATICA.md), [tipos](SISTEMA_TIPOS.md).

Esta referencia describe el parser y evaluator actuales de Joss. Las
limitaciones observadas se indican como tales; no son propuestas de diseño.

## Léxico

Las palabras reservadas se obtienen de `parser.KeywordNames()` (`pkg/parser/token.go`):

```text
Init abstract as async await break case catch class const continue default defer do echo empty
enum extends false foreach func guard implements instanceof interface is isset let match new nil null print private
protected public record ref return select static this throw true try while yield
```

`var`, `int`, `float`, `decimal`, `string`, `bool`, `array`, `map`, `object`, `channel`, `mixed`, `void` y `make_chan` son identificadores de tipo o funciones integradas interpretados por contexto, no keywords cerradas del lexer.
Las constantes internas `IF` y `ELSE` no significan que esas construcciones estén disponibles: `if`, `else`, `elif`, `for` y `switch` no son estructuras fuente de Joss.

| Forma | Regla |
|---|---|
| Identificador | ASCII: letras, `_` o `@` al inicio; también dígitos después. Evita `@` fuera de APIs concretas; no es un sistema de anotaciones. |
| Variable | `$nombre`; el token `$` se separa del nombre. `$this` tiene tratamiento propio. |
| Comentarios | `//` y `#` hasta fin de línea; `/* ... */` sin anidamiento. |
| Strings | Comillas simples o dobles; escapes `\n`, `\t`, `\r`, `\'`, `\"`, `\\`; interpolación estilo Dart/Flutter en comillas dobles: `"${var}"` o `"${$expr}"` (escape `\${`); comillas simples sin interpolación. |
| Enteros | Secuencia de dígitos; el parser usa base automática: `010` se lee como octal, `08` falla. Evita ceros iniciales. |
| Float | Dígitos, punto y más dígitos: `0.5`. No hay literal exponencial, hexadecimal ni separador `_` en el lexer. |
| Decimal | Entero o fracción con sufijo `m`/`M`: `100m`, `1.25M`. |
| Ausencia | `null` es el literal canónico (`nil` produce el mismo valor pero emite la advertencia de transición `JOSS-DECL-010`). |
| Separación | Nueva línea o `;`. Los espacios y tabs no delimitan bloques. |

El lexer elimina BOM UTF-8 inicial. Fuera de strings, omite bytes no ASCII:
no confíes en identificadores acentuados.

Además de nombres, literales y keywords, se tokenizan:

```text
= += -= *= /= ??= + - ! * / % < > == != === !== <=> <= >= << >> && || ++ --
, ; : ? ( ) { } [ ] . .. ... -> ?-> :: | |> ?? =>
NEWLINE EOF ILLEGAL
```

No hay exponenciación, AND binario `&` ni OR binario `|`;
este último separa tipos de una unión (`int|string`).

## Precedencia: de menor a mayor

La tabla reproduce `pkg/parser/parser.go`. A igual nivel, los infijos
ordinarios agrupan a la izquierda; la asignación analiza toda su derecha.

| Nivel | Operadores / construcciones |
|---|---|
| 1 | `=`, `+=`, `-=`, `*=`, `/=`, `??=` (derecha) |
| 2 | `? :`, `?:` |
| 3 | `??` |
| 4 | `\|>` |
| 5 | `\|\|` |
| 6 | `&&` |
| 7 | `==`, `!=`, `===`, `!==`, `<=>` |
| 8 | `<`, `>`, `<=`, `>=`, `is`, `instanceof` |
| 9 | `<<`, `>>` |
| 10 | `..` |
| 11 | `+`, `-`, `.` |
| 12 | `*`, `/`, `%` |
| 13 | Prefijos `-`, `!`, `await`, `ref`, `...` |
| 14 | Llamada `()` |
| 15 | Índice `[]`, miembros `->`, `?->`, `::`, postfix `++`, `--` |

Los operadores aritméticos y lógicos siguen su jerarquía convencional: `%`,
`*` y `/` comparten nivel, y `&&` liga más fuerte que `||`.
Los brazos del ternario se parsean como expresiones completas; parentetiza
ternarios anidados en vez de trasladar la asociatividad de otro lenguaje.

<!-- joss-run: ["1", "true", "true"] -->
```joss
print(8 * 5 % 3)
print(true || false && false)
print(true || (false && false))
```

Da `1`, `true` y `true`. La primera expresión es `(8 * 5) % 3`.

## Evaluación

- `+`, `-`, `*`: enteros exactos con overflow comprobado; operaciones entre `int` y `decimal` promueven a `decimal`; mezclar `int` con `float` o `float` con `decimal` requiere conversión explícita (`JOSS-ARITH-003`).
- `/`: resultado `float` para enteros; `decimal` si interviene `decimal`.
- `+=`, `-=`, `*=`, `/=`, `??=`: asignación compuesta y asignación nula
  coalescente (`$a ??= $b` equivale a `$a = $a ?? $b`) con validación
  estricta de tipos.
- `%`: resto entero; con float trunca operandos a entero; decimal usa `Mod`.
- `.`: representa operandos como texto y concatena; `null` contribuye texto vacío.
- `..`: operador de rango numérico (`$a..$b`); genera una secuencia entera entre
  ambos límites, útil en expresiones y bucles `foreach (1..10 as $i)`.
- `"${...}"`: interpolación de cadenas estilo Flutter / Dart dentro de comillas
  dobles. Permite incrustar variables simples (`"${indice}"`) o expresiones (`"${$a + $b}"`),
  desazucarándose automáticamente a concatenaciones de tipo string. El escape `\${`
  imprime `${` de forma literal.
- `++`, `--`: incrementa o decrementa la variable numérica y retorna el valor anterior.
- `&&`, `||`: cortocircuito y resultado bool. `!`: negación de truthiness.
- `==`/`!=`: comparación numérica cuando corresponde; fallback mediante
  representación textual para otros valores. Para colecciones prefiere
  `===`/`!==`, basados en comparación estructural.
- `===`: distingue entero, float, string y decimal; normaliza variantes
  numéricas internas de Go antes de comparar.
- `<=>`: retorna `-1`, `0` o `1`; compara números, strings, nulos y
  finalmente representaciones textuales.
- `??`: evalúa la derecha sólo si la izquierda produce `null`. Los errores al
  evaluar la izquierda se propagan y deben manejarse con `try`/`catch`.
- `?:` (Elvis): conserva la izquierda si es verdadera según truthiness.
- Ternario completo y de una sola rama: `cond ? expr : expr` y `(cond) ? { cuerpo }`
  (permite omitir la rama `: {}` cuando no se requiere alternativa falsa).
- `guard (cond) : { cuerpo }`: asegura que `cond` sea verdadera; si es falsa ejecuta el bloque, el cual debe terminar el flujo con `return` o `throw` (`JOSS-FLOW-005`).
- `match`: selección múltiple por valor estricta (`===`); admite expresiones o bloques `{ ... }`.
- `?->`: devuelve nulo ante receptor nulo; no valida ni corrige otros accesos.
- `|>`: antepone el valor izquierdo a los argumentos de una función, llamada
  o closure.
- `cout << valor`: imprime sin agregar salto y retorna `cout`.
  `canal << valor`: envía al canal. `cin >> $variable`: lee de forma adaptativa
  desde la entrada estándar (convirtiendo automáticamente a número si la variable
  o entrada es numérica y leyendo líneas completas con espacios si es texto).

## Verdad de valores

Joss conserva una truthiness pequeña y uniforme. Son falsos: `null`, `false`,
cero de cualquier tipo numérico, string vacío, array vacío y map vacío. Todo
valor restante es verdadero; en particular, `"0"` es verdadero porque es un
string no vacío. `empty` usa esta misma interpretación después de comprobar la
existencia del valor.

## Declaraciones y ámbitos

| Forma | Clasificación y efecto |
|---|---|
| `var $x = valor` | **Canónica**: Declaración con inferencia estricta y fija. |
| `T $x = valor` | **Canónica**: Declaración con tipo explícito (`int`, `string`, `Persona`, etc.). |
| `mixed $x = valor` | **Canónica**: Dinamismo explícito voluntario; permite cambiar de tipo en reasignaciones. |
| `const [T] $x = valor` | **Canónica**: Binding constante inmutable; requiere inicializador. |
| `int $a = 1, $b = 2` | **Canónica**: Declaración múltiple del mismo tipo. |
| `$x = valor` (sin declaración previa) | **Reasignación** si `$x` ya existe; si no fue declarada previamente, emite error `JOSS-SYM-001` (y aviso histórico `JOSS-DECL-007`). |
| `let [T] $x = valor` | **Transición**: Soportada por compatibilidad, emite advertencia `JOSS-DECL-006`. Usa `var`, `T` o `mixed`. |
| `public/private func f(T $p): R { ... }` | **Canónica**: Función global; parámetros tipados y retorno `: R` o `: void`. |
| `func(T $p): R { ... }` | **Canónica**: Closure sin modificador de visibilidad. |
| `public/private [abstract] class C [<T>] [extends B] [implements I1, I2] { ... }` | **Canónica**: Clase nominal (con soporte para genéricos `<T>`, herencia e interfaces). |
| `public/private record R(T $campo, ...)` | **Canónica**: Clase de datos inmutable con campos `public const` y constructor automático. |
| `public/private interface I [extends I1, I2] { ... }` | **Canónica**: Interfaz de contratos de métodos públicos sin cuerpo. |
| `public/private enum E [: int\|string] { case ... }` | **Canónica**: Enumeración pura o respaldada (`int`/`string`). |
| `public func constructor(...) { ... }` o `Init(...) { ... }` | **Canónica**: Inicializador de instancia (`Init constructor(...)` es forma de transición `JOSS-DECL-008`). |

Las funciones y clases globales se registran antes de analizar cuerpos.
Las variables de nivel superior no son globals implícitos de funciones con
nombre. Cada llamada usa su frame; una closure captura su entorno léxico.
La visibilidad de métodos y propiedades es obligatoria. No hay sobrecargas
por firma.

## Bloques, colecciones y sentencias

`[]` crea un array, `{}` crea un map vacío en contexto de expresión y
`{"clave": valor}` crea un map no vacío. Una llave en un lugar que exige un
cuerpo (función/ciclo) delimita un bloque.

- Ternario: elige un valor o ejecuta el bloque seleccionado; `return` puede
  salir del callable desde ese bloque.
- `guard (condición) : { ... }`: cláusula de guarda con salida obligatoria (`return` o `throw`).
- `while (condición) { ... }`: comprueba antes de cada vuelta.
- `do { ... } while (condición)`: comprueba después.
- `foreach (array_o_canal as $valor)` o `foreach ($coleccion as $clave => $valor)`:
  recorre secuencias, rangos `1..$n`, mapas asociativos, generadores y canales de concurrencia.
- `break` y `continue`: salir/saltar vuelta.
- `defer { ... }` o `defer expresión`: pospone la ejecución hasta que
  el frame actual finalice (en orden LIFO).
- `[$a, $b] = $expr`: destructuración secuencial de arreglos.
- `match (valor) { clave, clave => resultado, default => resultado }`: compara
  estrictamente, primer brazo coincidente; sin coincidencia/default retorna nulo.
- `try { ... } catch ($error) { ... }`, `throw expresión`: recuperación runtime.
- `return [expresión]`: sale del callable.
- `async { ... }`: crea un `Future`; se recoge con `await($futuro)` o `await $futuro`.
- `select { case ... default: ... }`: multiplexación sobre canales.
- `yield [clave =>] valor`: emisión perezosa en funciones generadoras.
- `Console::*`: módulo nativo para colores ANSI (`Console::green`, `Console::red`, `Console::bold`, etc.).
- `joss repl`: entorno interactivo por terminal (Read-Eval-Print Loop).

## Ausencias y compatibilidad

No hay imports fuente, namespaces, exports de archivos, traits,
protocolos, ownership, punteros manuales, `switch`, `for` clásico ni syntax sugar de funciones flecha.
`ref` sólo sirve como parámetro/argumento temporal y no es un puntero almacenable.

`function`, `import`, `@import`, `use`, `Use`, `Import`,
`namespace` y `Namespace` generan errores de sintaxis eliminada (`JOSS-PARSE-001`).
Los nombres heredados de APIs no son necesariamente tipos fuente válidos:
`is_integer` sigue registrado como función built-in, pero `integer $x` no es alias de `int` (`JOSS-TYPE-009`).

[Índice](README.md)

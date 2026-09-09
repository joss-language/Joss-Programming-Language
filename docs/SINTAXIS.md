# Referencia de sintaxis y operadores

Para aprender: [fundamentos](FUNDAMENTOS.md), [flujo](CONTROL_FLUJO.md),
[funciones](FUNCIONES.md), [clases](CLASES.md).
Complementos: [gramática](GRAMATICA.md), [tipos](SISTEMA_TIPOS.md).

Esta referencia describe el parser y evaluator de esta revisión. Las
limitaciones observadas se indican como tales; no son propuestas de diseño.

## Léxico

Las palabras reservadas se obtienen de `parser.KeywordNames()`:

```text
Init as async break catch class const continue default do echo empty
extends false foreach func isset let match new nil null print private
protected public ref return static this throw true try while
```

`var`, `int`, `mixed`, `await` y `make_chan` son identificadores
interpretados por contexto o nombres de funciones, no keywords del lexer.
Las constantes internas `IF` y `ELSE` no significan que esas construcciones
estén disponibles. `if`, `else`, `elif`, `for` y `switch` no son
estructuras fuente de Joss.

| Forma | Regla |
|---|---|
| Identificador | ASCII: letras, `_` o `@` al inicio; también dígitos después. Evita `@` fuera de APIs concretas; no es un sistema de anotaciones. |
| Variable | `$nombre`; el token `$` se separa del nombre. `$this` tiene tratamiento propio. |
| Comentarios | `//` y `#` hasta fin de línea; `/* ... */` sin anidamiento. |
| Strings | Comillas simples o dobles; escapes `\\n`, `\\t`, `\\r`, `\\'`, `\\"`, `\\\\`; escapes desconocidos conservan la barra. Sin interpolación. |
| Enteros | Secuencia de dígitos; el parser usa base automática: `010` se lee como octal, `08` falla. Evita ceros iniciales. |
| Float | Dígitos, punto y más dígitos: `0.5`. No hay literal exponencial, hexadecimal ni separador `_` en el lexer. |
| Decimal | Entero o fracción con sufijo `m`/`M`: `100m`, `1.25M`. |
| Ausencia | `null` y `nil` producen el mismo valor. |
| Separación | Nueva línea o `;`. Los espacios y tabs no delimitan bloques. |

El lexer elimina BOM UTF-8 inicial. Fuera de strings, omite bytes no ASCII:
no confíes en identificadores acentuados. Los diagnósticos de strings multilínea
y texto no ASCII aún tienen limitaciones de posición.

Además de nombres, literales y keywords, se tokenizan:

```text
= + - ! * / % < > == != === !== <=> <= >= << >> && || ++
, ; : ? ( ) { } [ ] . -> ?-> :: | |> ?? =>
NEWLINE EOF ILLEGAL
```

No hay `--`, `+=`, exponenciación, AND binario `&` ni OR binario `|`;
este último separa tipos de una unión.

## Precedencia: de menor a mayor

La tabla reproduce `pkg/parser/parser.go`. A igual nivel, los infijos
ordinarios agrupan a la izquierda; la asignación analiza toda su derecha.

| Nivel | Operadores / construcciones |
|---|---|
| 1 | `=` (derecha) |
| 2 | `? :`, `?:` |
| 3 | `??` |
| 4 | `&&`, `\|\|` |
| 5 | `==`, `!=`, `===`, `!==`, `<=>` |
| 6 | `<`, `>`, `<=`, `>=` |
| 7 | `\|>` |
| 8 | `+`, `-`, `.` |
| 9 | `<<`, `>>` |
| 10 | `*`, `/` |
| 11 | `%` |
| 12 | Prefijos `-`, `!`, `ref` |
| 13 | Llamada `()` |
| 14 | Índice `[]`, miembros `->`, `?->`, `::`, postfix `++` |

Consecuencias: `%` liga más fuerte que multiplicación y división, mientras
`&&` y `||` comparten nivel. Usa paréntesis para expresar tu intención.
Los brazos del ternario se parsean como expresiones completas; parentetiza
ternarios anidados en vez de trasladar la asociatividad de otro lenguaje.

<!-- joss-run: ["16", "false", "true"] -->
```joss
print(8 * 5 % 3)
print(true || false && false)
print(true || (false && false))
```

Da `16`, `false` y `true`. La primera expresión es `8 * (5 % 3)`.
No es una tabla de precedencia de PHP o Go.

## Evaluación

- `+`, `-`, `*`: enteros exactos con overflow comprobado; promoción si hay
  float, operaciones decimales si interviene decimal.
- `/`: resultado float para enteros; decimal si interviene decimal.
- `+=`, `-=`, `*=`, `/=`: asignación compuesta; se desugarean a su operación
  aritmética correspondiente (`$a += $b` equivale a `$a = $a + $b`) con
  validación estricta de tipos.
- `%`: resto entero; con float trunca operandos a entero; decimal usa `Mod`.
- `.`: representa operandos como texto y concatena; `null` contribuye texto vacío.
- `++`: incrementa y retorna el valor anterior. No se define decremento.
- `&&`, `||`: cortocircuito y resultado bool. `!`: negación de truthiness.
- `==`/`!=`: comparación numérica cuando corresponde; fallback mediante
  representación textual para otros valores. Para colecciones prefiere
  `===`/`!==`, basados en comparación estructural.
- `===`: distingue entero, float, string y decimal; normaliza variantes
  numéricas internas de Go antes de comparar.
- `<=>`: retorna `-1`, `0` o `1`; compara números, strings, nulos y
  finalmente representaciones textuales.
- `??`: evalúa la derecha sólo si la izquierda es nula. **Actualmente recupera
  cualquier panic de la izquierda** y lo trata como nulo.
- `?:` (Elvis): conserva la izquierda si es verdadera según truthiness.
- Ternario completo y de una sola rama: `cond ? expr : expr` y `(cond) ? { cuerpo }`
  (permite omitir la rama `: {}` cuando no se requiere alternativa falsa).
- `match`: selección múltiple por valor; admite expresiones o bloques multilínea `{ ... }`.
- `?->`: devuelve nulo ante receptor nulo; no valida ni corrige otros accesos.
- `|>`: antepone el valor izquierdo a los argumentos de una función, llamada
  o closure. No es concurrencia.
- `cout << valor`: imprime sin agregar salto y retorna `cout`.
  `canal << valor`: envía al canal. `cin >> $variable`: lee una palabra.
  Aunque los tokens se llaman SHIFT, no hay un operador entero de desplazamiento
  implementado por estas rutas.

## Verdad de valores

`isFalsy` considera falsos: `null`, `false`, `int64(0)`, decimal cero,
`""`, `"0"` y array vacío. Considera verdaderas las instancias.
**El caso float cero y el mapa vacío no se comprueban y resultan verdaderos**.
Por eso conviene escribir condiciones bool explícitas. `empty` usa esa misma
interpretación tras comprobar existencia, no una regla universal de «sin datos».

## Declaraciones y ámbitos

| Forma | Efecto |
|---|---|
| `$x = valor` | Primera asignación declara/infiere; después reasigna. |
| `var $x = valor` | Declaración con inferencia fija. |
| `T $x = valor`, `let T $x = valor` | Tipo explícito. |
| `let $x = valor`, `mixed $x = valor` | Binding dinámico explícito. |
| `const [T] $x = valor` | Binding constante; requiere inicializador. |
| `int $a = 1, $b = 2` | Declaración múltiple del mismo tipo. |
| `public/private func f(T $p): R { ... }` | Función global; retorno opcional, parámetros tipados. |
| `func(T $p): R { ... }` | Closure sin modificador de visibilidad. |
| `public/private class C extends B { ... }` | Clase; una superclase opcional. |
| `Init nombre(...) { ... }` | Inicializador sin modificador. |

Las funciones/clases globales se registran antes de analizar cuerpos.
Las variables de nivel superior no son globals implícitos de funciones con
nombre. Cada llamada usa su frame; una closure captura un entorno.
La visibilidad de métodos y propiedades es obligatoria. No hay sobrecargas
por firma ni sintaxis general de parámetros de tipo.

## Bloques, colecciones y sentencias

`[]` crea un array, `{}` crea un map vacío en contexto de expresión y
`{"clave": valor}` crea un map no vacío. Una llave en un lugar que exige un
cuerpo (función/ciclo) delimita un bloque. Los bloques como expresiones se
representan internamente como AST y no se ejecutan automáticamente en todo contexto.

- Ternario: elige un valor o ejecuta el bloque seleccionado; `return` puede
  salir del callable desde ese bloque.
- `while (condición) { ... }`: comprueba antes de cada vuelta.
- `do { ... } while (condición)`: comprueba después.
- `foreach (array_o_canal as $valor) { ... }`: sólo una variable; mapas mediante
  `keys`. Sin sintaxis `$clave => $valor`.
- `break` y `continue`: salir/saltar vuelta. La detección de control dentro
  de ternarios/match tiene una limitación descrita en [flujo](CONTROL_FLUJO.md).
- `match (valor) { clave, clave => resultado, default => resultado }`: compara
  estrictamente, primer brazo coincidente; sin coincidencia/default retorna
  nulo. **Un brazo que es bloque retorna el bloque AST, no ejecuta su cuerpo**,
  aunque el analizador lo considere para cobertura de retorno. Usa brazos
  con valores o llamadas hasta corregir esa discrepancia.
- `try { ... } catch ($error) { ... }`, `throw expresión`: recuperación
  runtime. No hay `finally`, `defer` ni captura tipada.
- `return [expresión]`: sale del callable. No hay retorno múltiple especial;
  retorna un array o mapa si necesitas varios resultados.
- `async { ... }`: crea un Future; se recoge con `await(futuro)`.

## Ausencias y compatibilidad

No hay imports fuente, namespaces, exports de archivos, interfaces, traits,
protocolos, ownership, punteros manuales, `switch`, `for`, destructuring,
comprehensions ni syntax sugar de funciones flecha. `=>` pertenece a `match`.
`ref` sólo sirve como parámetro/argumento temporal y no es un puntero almacenable.

`function`, `import`, `@import`, `use`, `Use`, `Import`,
`namespace` y `Namespace` generan errores de sintaxis eliminada.
Los nombres heredados de APIs no son necesariamente tipos fuente válidos:
`is_integer` sigue registrado, `integer $x` no es alias de `int`.


[Índice](README.md)

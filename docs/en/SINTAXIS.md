#

Syntax and Operators Reference To learn: [fundamentals](FUNDAMENTOS.md) , [flow](CONTROL_FLUJO.md) ,
[functions](FUNCIONES.md) , [classes](CLASES.md) .
Plugins: [grammar](GRAMATICA.md) , [types](SISTEMA_TIPOS.md) .

This reference describes the parser and evaluator for this patch.
observed limitations are noted as such;They are not design proposals.

## Lexicon

Reserved words are obtained from `parser.KeywordNames()` :

```text
Init abstract as async break case catch class const continue default defer do echo empty
enum extends false foreach func implements instanceof interface is isset let match new nil null print private
protected public ref return select static this throw true try while yield
```

`var` , `int` , `mixed` , `await` and `make_chan` are
identifiers interpreted by context or function names, not lexer keywords.
The internal constants `IF` and `ELSE` do not mean that those
constructs are available.`if` , `else` , `elif` , `for` and `switch` are not
Joss source structures.

|Shape |Rule |
|---|---|
|Identifier |ASCII: letters, `_` or `@` at the beginning;also digits after.Avoid `@` outside of specific APIs;It is not a scoring system.|
|Variable |`$nombre` ;the `$` token is separated from the name.`$this` has its own treatment.|
|Comments |`//` and `#` until end of line;`/* ... */` without nesting.|
|Strings |Single or double quotes;exhausts `\n` , `\t` , `\r` , `\'` , `\"` , `\\` ;Dart/Flutter style interpolation in double quotes: `"${var}"` or `"${expr}"` (escape `\${` );single quotes without interpolation.|
|Integers |Digit sequence;the parser uses automatic base: `010` is read as octal, `08` fails.Avoid leading zeros.|
|Floats |Digits, period and more digits: `0.5` .There is no exponential literal, hexadecimal literal, or `_` separator in the lexer.|
|Decimal |Integer or fraction with suffix `m` / `M` : `100m` , `1.25M` .|
|Absence |`null` and `nil` produce the same value.|
|Separation |New line or `;` .Spaces and tabs do not delimit blocks.|

The lexer removes leading UTF-8 BOM.Outside of strings, omit non-ASCII bytes:
do not trust accented identifiers.Diagnostics for multiline strings
and non-ASCII text still have position limitations.

In addition to names, literals and keywords, they are tokenized:

```text
= += -= *= /= ??= + - ! * / % < > == != === !== <=> <= >= << >> && || ++ --
, ; : ? ( ) { } [ ] . .. ... -> ?-> :: | |> ?? =>
NEWLINE EOF ILLEGAL
```

There is no exponentiation, binary AND `&` or binary OR `|` ;
the latter separates types of a union.

## Precedence: lowest to highest

The table reproduces `pkg/parser/parser.go` .At the same level, ordinary
infixes group to the left;the assignment analyzes all your right.

|Level |Operators/constructions |
|---|---|
|1 |`=` , `+=` , `-=` , `*=` , `/=` , `??=` (right) |
|2 |`? :` , `?:` |
|3 |`??` |
|4 |`\|>` |
|5 |`\|\|` |
|6 |`&&` |
|7 |`==` , `!=` , `===` , `!==` , `<=>` |
|8 |`<` , `>` , `<=` , `>=` , `is` , `instanceof` |
|9 |`<<` , `>>` |
|10 |`..` |
|11 |`+` , `-` , `.` |
|12 |`*` , `/` , `%` |
|13 |Prefixes `-` , `!` , `ref` , `...` |
|14 |Call `()` |
|15 |Index `[]` , members `->` , `?->` , `::` , postfix `++` , `--` |

Arithmetic and logical operators follow their conventional hierarchy: `%` ,
`*` and `/` share level, and `&&` binds stronger than `||` .
The arms of the ternary are parsed as complete expressions;parenthetizes
nested ternaries instead of transferring the associativity from another language.

<!-- joss-run: ["1", "true", "true"] -->
```joss
print(8 * 5 % 3)
print(true || false && false)
print(true || (false && false))
```

Gives `1` , `true` and `true` .The first expression is `(8 * 5) % 3` .

## Evaluation

- `+` , `-` , `*` : exact integers with checked overflow;promotion if there is
float, decimal operations if decimal is involved.
- `/` : float result for integers;decimal if decimal is involved.
- `+=` , `-=` , `*=` , `/=` , `??=` : composite assignment and coalescent
null assignment (`$a ??= $b` is equivalent to `$a = $a ?? $b` ) with strict type validation
.
- `%` : remainder integer;with float truncates operands to integer;decimal uses `Mod` .
- `.` - represents operands as text and concatenates;`null` contributes empty text.
- `..` : numeric range operator ( `$a..$b` );generates an integer sequence between
both limits, useful in `foreach (1..10 as $i)` expressions and loops.
- `"${...}"` - Flutter/Dart-style string interpolation inside double
quotes.It allows embedding variables or expressions (e.g. `"${indice}"` , `"${a + b}"` ),
, automatically desugaring string type concatenations.The `\${`
escape prints `${` literally.
- `++` , `--` : increments or decrements the numeric variable and returns the previous value.
- `&&` , `||` : short circuit and bool result.`!`: denial of truthiness.
- `==` / `!=` : numerical comparison where applicable;fallback using
textual representation for other values.For collections prefer
`===` / `!==`, based on structural comparison.
- `===` : distinguishes integer, float, string and decimal;normalizes internal Go numeric
variants before comparing.
- `<=>` : returns `-1` , `0` or `1` ;compares numbers, strings, nulls and
finally textual representations.
- `??` : evaluates the right only if the left produces `null` .Errors when
evaluate left are propagated and should be handled with `try` / `catch` .
- `?:` (Elvis): preserve left if true according to truthiness.
- Full, single-branch ternary: `cond ? expr : expr` and `(cond) ? { cuerpo }`
(allows branch `: {}` to be skipped when false alternative is not required).
- `match` : multiple selection by value;supports multiline expressions or blocks `{ ... }` .
- `?->` : returns null before null receiver;It does not validate or correct other accesses.
- `|>` : prepends the left value to the arguments of a function, called
or closure.
- `cout << valor` : prints without adding a break and returns `cout` .
`canal << valor` : Send to channel.`cin >> $variable` - Adaptively reads
from standard input (automatically converting to number if the
variable or input is numeric and reading full lines with spaces if it is text).

## Truthiness of values ​​

Joss retains a small and uniform truthiness.They are fake: `null` , `false` ,
zero of any numeric type, empty string, empty array and empty map.All remaining
value is true;in particular, `"0"` is true because it is a non-empty
string.`empty` uses this same interpretation after checking for the existence of the
value.

## Declarations and scopes

|Shape |Effect |
|---|---|
|`$x = valor` |First assignment declares/infers;then reassign.|
|`var $x = valor` |Statement with fixed inference.|
|`T $x = valor` , `let T $x = valor` |Explicit type.|
|`let $x = valor` , `mixed $x = valor` |Explicit dynamic binding.|
|`const [T] $x = valor` |Binding constant;requires initializer.|
|`int $a = 1, $b = 2` |Multiple declaration of the same type.|
|`public/private func f(T $p): R { ... }` |global function;optional return, typed parameters.|
|`func(T $p): R { ... }` |Closure without visibility modifier.|
|`public/private class C [extends B] [implements I1, I2] { ... }` |Class;Optional superclass and implemented interfaces.|
|`public/private interface I [extends I1, I2] { ... }` |Interface;Bodyless public method contracts.|
|`Init nombre(...) { ... }` |Initializer without modifier.|

Global functions/classes are registered before parsing bodies.
Top-level variables are not implicit globals of functions with
name.Each call uses its frame;A closure captures an environment.
Visibility of methods and properties is mandatory.There are no signature
overloads or general type parameter syntax.

## Blocks, collections and statements

`[]` creates an array, `{}` creates an empty map in expression context and
`{"clave": valor}` creates a non-empty map.A key in a place that requires a
body (function/cycle) delimits a block.Blocks such as expressions are
represented internally as ASTs and are not automatically executed in every context.

- Ternary: choose a value or execute the selected block;`return` can
exit the callable from that block.
- `while (condición) { ... }` : check before each lap.
- `do { ... } while (condición)` : check later.
- `foreach (array_o_canal as $valor)` or `foreach ($coleccion as $clave => $valor)` :
traverses sequences, `1..$n` ranges, associative maps, and concurrency channels.
- `break` and `continue`: exit/skip lap.
- `defer { ... }` or `defer expresión;` : postpones the execution of the statement until
the current frame ends (in LIFO order).
- `[$a, $b] = $expr` : sequential destructuring of arrays in direct assignment.
- `match (valor) { clave, clave => resultado, default => resultado }` : compare
strictly, first matching arm;no match/default returns null.
- `try { ... } catch ($error) { ... }` , `throw expresión` : runtime recovery.
- `return [expresión]` : exits the callable.
- `async { ... }` : creates a Future;is collected with `await(futuro)` .
- `Console::*`: native module for ANSI colors (`Console::green`, `Console::red`, `Console::bold`, etc.).
- `joss repl`: interactive environment per terminal (Read-Eval-Print Loop).

## Absences and compatibility

There are no source imports, namespaces, file exports, interfaces, traits,
protocols, ownership, manual pointers, classic `switch`, `for` or sugar syntax of arrow functions.
`ref` only serves as a temporary parameter/argument and is not a storable pointer.

`function` , `import` , `@import` , `use` , `Use` , `Import` ,
`namespace` and `Namespace` generate deleted syntax errors.
Names inherited from APIs are not necessarily valid source types:
`is_integer` is still registered, `integer $x` is not an alias of `int` .


[Index](README.md)

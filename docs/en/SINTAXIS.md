# Syntax and operators reference

To learn: [fundamentals](FUNDAMENTOS.md), [flow](CONTROL_FLUJO.md),
[functions](FUNCIONES.md), [classes](CLASES.md).
Complements: [grammar](GRAMATICA.md), [types](SISTEMA_TIPOS.md).

This reference describes the parser and evaluator for this patch. The
Observed limitations are indicated as such; They are not design proposals.

## Lexicon

Reserved words are obtained from `parser.KeywordNames()`:```text
Init abstract as async break case catch class const continue default defer do echo empty
enum extends false foreach func implements instanceof interface is isset let match new nil null print private
protected public ref return select static this throw true try while yield
```
`var`, `int`, `mixed`, `await` and `make_chan` are identifiers
interpreted by context or function names, not lexer keywords.
The internal constants `IF` and `ELSE` do not mean that those constructs
are available. `if`, `else`, `elif`, `for` and `switch` are not
Joss source structures.

| Shape | Rule |
|---|---|
| Identifier | ASCII: letters, `_` or `@` at the beginning; also digits after. Avoid `@` outside of specific APIs; It is not a scoring system. |
| Variable | `$nombre`; the token `$` is separated from the name. `$this` has its own treatment. |
| Comments | `//` and `#` until end of line; `/* ... */` without nesting. |
| Strings | Single or double quotes; escapes `\n`, `\t`, `\r`, `\'`, `\"`, `\\`; Dart/Flutter style interpolation in double quotes: `"${var}"` or `"${expr}"` (escaping `\${`); single quotes without interpolation. |
| Integers | Digit sequence; the parser uses automatic base: `010` is read as octal, `08` fails. Avoid leading zeros. |
| Floats | Digits, period and more digits: `0.5`. There is no exponential literal, hexadecimal literal, or `_` separator in the lexer. |
| Decimal | Integer or fraction with `m`/`M` suffix: `100m`, `1.25M`. |
| Absence | `null` and `nil` produce the same value. |
| Separation | New line or `;`. Spaces and tabs do not delimit blocks. |

The lexer removes leading UTF-8 BOM. Outside of strings, skip non-ASCII bytes:
Don't trust accented identifiers. Multiline string diagnostics
and non-ASCII text still have position limitations.

In addition to names, literals and keywords, the following are tokenized:```text
= += -= *= /= ??= + - ! * / % < > == != === !== <=> <= >= << >> && || ++ --
, ; : ? ( ) { } [ ] . .. ... -> ?-> :: | |> ?? =>
NEWLINE EOF ILLEGAL
```
There is no exponentiation, binary AND `&` or binary OR `|`;
the latter separates types of a union.

## Precedence: from lowest to highest

The table reproduces `pkg/parser/parser.go`. At the same level, the infixes
ordinary group on the left; the assignment analyzes all your right.

| Level | Operators/constructions |
|---|---|
| 1 | `=`, `+=`, `-=`, `*=`, `/=`, `??=` (right) |
| 2 | `? :`, `?:` |
| 3 | `??` |
| 4 | `&&`, `\|\|` |
| 5 | `==`, `!=`, `===`, `!==`, `<=>` |
| 6 | `<`, `>`, `<=`, `>=`, `..`, `is`, `instanceof` |
| 7 | `\|>` |
| 8 | `+`, `-`, `.` |
| 9 | `<<`, `>>` |
| 10 | `*`, `/` |
| 11 | `%` |
| 12 | Prefixes `-`, `!`, `ref`, `...` |
| 13 | Call `()` |
| 14 | Index `[]`, members `->`, `?->`, `::`, postfix `++`, `--` |

Consequences: `%` league stronger than multiplication and division, while
`&&` and `||` share level. Use parentheses to express your intention.
The arms of the ternary are parsed as complete expressions; parenthesis
nested ternaries instead of transferring the associativity of another language.<!-- joss-run: ["16", "false", "true"] -->
```joss
print(8 * 5 % 3)
print(true || false && false)
print(true || (false && false))
```
Returns `16`, `false` and `true`. The first expression is `8 * (5 % 3)`.
It is not a PHP or Go precedence table.

## Evaluation

- `+`, `-`, `*`: exact integers with checked overflow; promotion if there is
  float, decimal operations if decimal is involved.
- `/`: float result for integers; decimal if decimal is involved.
- `+=`, `-=`, `*=`, `/=`, `??=`: compound assignment and null assignment
  coalescent (`$a ??= $b` equals `$a = $a ?? $b`) with validation
  strict type.
- `%`: remainder integer; with float truncates operands to integer; decimal uses `Mod`.
- `.`: represents operands as text and concatenates; `null` contributes empty text.
- `..`: numeric range operator (`$a..$b`); generates an entire sequence between
  both limits, useful in expressions and `foreach (1..10 as $i)` loops.
- `"${...}"`: Flutter/Dart style string interpolation inside quotes
  doubles. Allows embedding variables or expressions (e.g. `"${indice}"`, `"${a + b}"`),
  automatically desugaring to string type concatenations. The `\${` escape
  prints `${` literally.
- `++`, `--`: increments or decrements the numeric variable and returns the previous value.
- `&&`, `||`: short circuit and bool result. `!`: denial of truthiness.
- `==`/`!=`: numerical comparison when applicable; fallback via
  textual representation for other values. For collections you prefer
  `===`/`!==`, based on structural comparison.
- `===`: distinguishes integer, float, string and decimal; normalizes variants
  Go's internal numeric numbers before comparing.
- `<=>`: return `-1`, `0` or `1`; compares numbers, strings, nulls and
  finally textual representations.
- `??`: evaluates right only if left is null. **Currently recovering
  any panic on the left** and treats it as null.
- `?:` (Elvis): preserve left if true according to truthiness.
- Complete, single-branch ternary: `cond ? expr : expr` and `(cond) ? { cuerpo }`
  (allows skipping branch `: {}` when no false alternative is required).
- `match`: multiple selection by value; supports multiline `{ ... }` expressions or blocks.
- `?->`: return null before null receiver; It does not validate or correct other accesses.
- `|>`: prepends the left value to the arguments of a function, called
  or closure.
- `cout << valor`: prints without adding a break and returns `cout`.
  `canal << valor`: send to channel. `cin >> $variable`: read adaptively
  from standard input (automatically converting to number if the variable
  or input is numerical and reading full lines with spaces if it is text).

## Truth of values

`isFalsy` considers false: `null`, `false`, `int64(0)`, decimal zero,
`""`, `"0"` and empty array. Consider the instances true.
**The zero float case and empty map are not checked and return true**.
That is why it is convenient to write explicit bool conditions. `empty` uses that same one
interpretation after checking existence, not a universal “no data” rule.

## Declarations and scopes

| Shape | Effect |
|---|---|
| `$x = valor` | First assignment declares/infers; then reassign. || `var $x = valor` | Statement with fixed inference. |
| `T $x = valor`, `let T $x = valor` | Explicit type. |
| `let $x = valor`, `mixed $x = valor` | Explicit dynamic binding. |
| `const [T] $x = valor` | Binding constant; requires initializer. |
| `int $a = 1, $b = 2` | Multiple declaration of the same type. |
| `public/private func f(T $p): R { ... }` | global function; optional return, typed parameters. |
| `func(T $p): R { ... }` | Closure without visibility modifier. |
| `public/private class C [extends B] [implements I1, I2] { ... }` | Class; Optional superclass and implemented interfaces. |
| `public/private interface I [extends I1, I2] { ... }` | Interface; Bodyless public method contracts. |
| `Init nombre(...) { ... }` | Initializer without modifier. |

Global functions/classes are registered before parsing bodies.
Top-level variables are not implicit globals of functions with
name. Each call uses its frame; A closure captures an environment.
Visibility of methods and properties is mandatory. There are no overloads
by signature or general type parameter syntax.

## Blocks, collections and statements

`[]` creates an array, `{}` creates an empty map in expression context and
`{"clave": valor}` creates a non-empty map. A key in a place that demands a
body (function/cycle) delimits a block. Blocks as expressions are
They are represented internally as ASTs and are not automatically executed in every context.

- Ternary: choose a value or execute the selected block; `return` can
  exit the callable from that block.
- `while (condición) { ... }`: check before each return.
- `do { ... } while (condición)`: check after.
- `foreach (array_o_canal as $valor)` or `foreach ($coleccion as $clave => $valor)`:
  iterates through sequences, `1..$n` ranges, associative maps, and concurrency channels.
- `break` and `continue`: leave/skip lap.
- `defer { ... }` or `defer expresión;`: postpones the execution of the statement until
  the current frame ends (in LIFO order).
- `[$a, $b] = $expr`: sequential destructuring of arrays in direct assignment.
- `match (valor) { clave, clave => resultado, default => resultado }`: compare
  strictly, first matching arm; no match/default returns null.
- `try { ... } catch ($error) { ... }`, `throw expresión`: runtime recovery.
- `return [expresión]`: exit the callable.
- `async { ... }`: create a Future; is collected with `await(futuro)`.
- `Console::*`: native module for ANSI colors (`Console::green`, `Console::red`, `Console::bold`, etc.).
- `joss repl`: interactive environment per terminal (Read-Eval-Print Loop).

## Absences and compatibility

There are no source imports, namespaces, file exports, interfaces, traits,
protocols, ownership, manual pointers, `switch`, classic `for` or sugar syntax of arrow functions.
`ref` only serves as a temporary parameter/argument and is not a storable pointer.

`function`, `import`, `@import`, `use`, `Use`, `Import`,
`namespace` and `Namespace` generate deleted syntax errors.
Names inherited from APIs are not necessarily valid source types:`is_integer` is still registered, `integer $x` is not aliased to `int`.


[Index](README.md)
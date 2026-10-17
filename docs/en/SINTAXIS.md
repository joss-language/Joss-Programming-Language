# Syntax and Operators Reference

To learn: [basics](FUNDAMENTOS.md), [control flow](CONTROL_FLUJO.md),
[collections](COLECCIONES.md), [functions](FUNCIONES.md), [classes](CLASES.md).
Supplements: [grammar](GRAMATICA.md), [types](SISTEMA_TIPOS.md).

This reference describes the current Joss parser and evaluator. Observed
limitations are stated as such; they are not design proposals.

## Lexicon

Reserved words are obtained from `parser.KeywordNames()` (`pkg/parser/token.go`):

```text
Init abstract as async await break case catch class const continue default defer do echo empty
enum extends false foreach func guard implements instanceof interface is isset let match new nil null print private
protected public record ref return select static this throw true try while yield
```

`var`, `int`, `float`, `decimal`, `string`, `bool`, `array`, `map`, `object`, `channel`, `mixed`, `void`, and `make_chan` are contextual type identifiers or built-in functions, not closed lexer keywords.
The internal constants `IF` and `ELSE` do not mean those constructs are available: `if`, `else`, `elif`, `for`, and `switch` are not source constructs in Joss.

| Form | Rule |
|---|---|
| Identifier | ASCII: letters, `_`, or `@` at the start; digits also allowed afterwards. Avoid `@` outside concrete APIs; it is not an annotation system. |
| Variable | `$nombre`; the `$` token is separated from the name. `$this` has special handling. |
| Comments | `//` and `#` to end of line; `/* ... */` without nesting. |
| Strings | Single or double quotes; escapes `\n`, `\t`, `\r`, `\'`, `\"`, `\\`; Dart/Flutter-style interpolation in double quotes: `"${var}"` or `"${$expr}"` (escape `\${`); single quotes without interpolation. |
| Integers | Sequence of digits; the parser uses automatic base detection: `010` is read as octal, `08` fails. Avoid leading zeros. |
| Float | Digits, dot, and more digits: `0.5`. There is no exponential, hexadecimal, or `_` separator literal in the lexer. |
| Decimal | Integer or fraction with `m`/`M` suffix: `100m`, `1.25M`. |
| Absence | `null` is the canonical literal (`nil` produces the same value but emits the transition warning `JOSS-DECL-010`). |
| Separation | Newline or `;`. Spaces and tabs do not delimit blocks. |

The lexer strips an initial UTF-8 BOM. Outside strings, it skips non-ASCII bytes:
do not rely on accented identifiers.

In addition to names, literals, and keywords, the following are tokenized:

```text
= += -= *= /= ??= + - ! * / % < > == != === !== <=> <= >= << >> && || ++ --
, ; : ? ( ) { } [ ] . .. ... -> ?-> :: | |> ?? =>
NEWLINE EOF ILLEGAL
```

There is no exponentiation, bitwise AND `&`, or bitwise OR `|`;
the latter separates types in a union (`int|string`).

## Precedence: From Lowest to Highest

The table reproduces `pkg/parser/parser.go`. At the same level, ordinary infix
operators associate to the left; assignment parses its entire right-hand side.

| Level | Operators / constructs |
|---|---|
| 1 | `=`, `+=`, `-=`, `*=`, `/=`, `??=` (right) |
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
| 13 | Prefixes `-`, `!`, `await`, `ref`, `...` |
| 14 | Call `()` |
| 15 | Index `[]`, members `->`, `?->`, `::`, postfix `++`, `--` |

Arithmetic and logical operators follow their conventional hierarchy: `%`,
`*`, and `/` share the same level, and `&&` binds tighter than `||`.
Ternary branches are parsed as full expressions; parenthesize
nested ternaries instead of carrying over associativity assumptions from another language.

<!-- joss-run: ["1", "true", "true"] -->
```joss
print(8 * 5 % 3)
print(true || false && false)
print(true || (false && false))
```

Produces `1`, `true`, and `true`. The first expression is `(8 * 5) % 3`.

## Evaluation

- `+`, `-`, `*`: exact integers with checked overflow; operations between `int` and `decimal` promote to `decimal`; mixing `int` with `float` or `float` with `decimal` requires explicit conversion (`JOSS-ARITH-003`).
- `/`: `float` result for integers; `decimal` if `decimal` is involved.
- `+=`, `-=`, `*=`, `/=`, `??=`: compound assignment and null-coalescing
  assignment (`$a ??= $b` is equivalent to `$a = $a ?? $b`) with strict
  type validation.
- `%`: integer remainder; with float it truncates operands to integer; decimal uses `Mod`.
- `.`: represents operands as text and concatenates; `null` contributes empty text.
- `..`: numeric range operator (`$a..$b`); generates an integer sequence between
  both bounds, useful in expressions and `foreach (1..10 as $i)` loops.
- `"${...}"`: Flutter / Dart-style string interpolation inside double
  quotes. Allows embedding simple variables (`"${indice}"`) or expressions (`"${$a + $b}"`),
  automatically desugaring into string concatenations. The escape `\${`
  prints `${` literally.
- `++`, `--`: increments or decrements the numeric variable and returns the previous value.
- `&&`, `||`: short-circuit evaluation and bool result. `!`: truthiness negation.
- `==`/`!=`: numeric comparison when applicable; fallback via
  textual representation for other values. For collections prefer
  `===`/`!==`, based on structural comparison.
- `===`: distinguishes integer, float, string, and decimal; normalizes internal
  Go numeric variants before comparing.
- `<=>`: returns `-1`, `0`, or `1`; compares numbers, strings, nulls, and
  finally textual representations.
- `??`: evaluates the right side only if the left side produces `null`. Errors when
  evaluating the left side propagate and must be handled with `try`/`catch`.
- `?:` (Elvis): keeps the left side if it is truthy according to truthiness rules.
- Full and single-branch ternary: `cond ? expr : expr` and `(cond) ? { cuerpo }`
  (allows omitting the `: {}` branch when no false alternative is needed).
- `guard (cond) : { cuerpo }`: ensures `cond` is truthy; if falsy it executes the block, which must terminate control flow with `return` or `throw` (`JOSS-FLOW-005`).
- `match`: strict multi-way value selection (`===`); accepts expressions or `{ ... }` blocks.
- `?->`: returns null on a null receiver; does not validate or fix other accesses.
- `|>`: prepends the left value to the arguments of a function, call,
  or closure.
- `cout << valor`: prints without adding a newline and returns `cout`.
  `canal << valor`: sends to the channel. `cin >> $variable`: adaptively reads
  from standard input (automatically converting to a number if the variable
  or input is numeric and reading full lines with spaces if it is text).

## Value Truthiness

Joss maintains a small, uniform truthiness model. The following are falsy: `null`, `false`,
zero of any numeric type, empty string, empty array, and empty map. Every
remaining value is truthy; in particular, `"0"` is truthy because it is a
non-empty string. `empty` uses this same interpretation after checking that the
value exists.

## Declarations and Scopes

| Form | Classification and effect |
|---|---|
| `var $x = valor` | **Canonical**: Declaration with strict, fixed type inference. |
| `T $x = valor` | **Canonical**: Declaration with explicit type (`int`, `string`, `Persona`, etc.). |
| `mixed $x = valor` | **Canonical**: Voluntary explicit dynamism; allows changing type on reassignments. |
| `const [T] $x = valor` | **Canonical**: Immutable constant binding; requires initializer. |
| `int $a = 1, $b = 2` | **Canonical**: Multiple declaration of the same type. |
| `$x = valor` (without prior declaration) | **Reassignment** if `$x` already exists; if not declared previously, emits error `JOSS-SYM-001` (and historical notice `JOSS-DECL-007`). |
| `let [T] $x = valor` | **Transition**: Supported for compatibility, emits warning `JOSS-DECL-006`. Use `var`, `T`, or `mixed`. |
| `public/private func f(T $p): R { ... }` | **Canonical**: Global function; typed parameters and `: R` or `: void` return. |
| `func(T $p): R { ... }` | **Canonical**: Closure without visibility modifier. |
| `public/private [abstract] class C [<T>] [extends B] [implements I1, I2] { ... }` | **Canonical**: Nominal class (with support for generics `<T>`, inheritance, and interfaces). |
| `public/private record R(T $campo, ...)` | **Canonical**: Immutable data class with `public const` fields and automatic constructor. |
| `public/private interface I [extends I1, I2] { ... }` | **Canonical**: Interface of bodiless public method contracts. |
| `public/private enum E [: int\|string] { case ... }` | **Canonical**: Pure or backed enumeration (`int`/`string`). |
| `public func constructor(...) { ... }` or `Init(...) { ... }` | **Canonical**: Instance initializer (`Init constructor(...)` is transition form `JOSS-DECL-008`). |

Global functions and classes are registered before analyzing bodies.
Top-level variables are not implicit globals inside named
functions. Each call uses its own frame; a closure captures its lexical environment.
Visibility on methods and properties is mandatory. There is no signature
overloading.

## Blocks, Collections, and Statements

`[]` creates an array, `{}` creates an empty map in expression context, and
`{"clave": valor}` creates a non-empty map. A brace in a position that requires a
body (function/loop) delimits a block.

- Ternary: chooses a value or executes the selected block; `return` can
  exit the callable from inside that block.
- `guard (condición) : { ... }`: guard clause with mandatory exit (`return` or `throw`).
- `while (condición) { ... }`: checks before each iteration.
- `do { ... } while (condición)`: checks after each iteration.
- `foreach (array_o_canal as $valor)` or `foreach ($coleccion as $clave => $valor)`:
  iterates over sequences, `1..$n` ranges, associative maps, generators, and concurrency channels.
- `break` and `continue`: exit/skip iteration.
- `defer { ... }` or `defer expresión`: postpones execution until
  the current frame finishes (in LIFO order).
- `[$a, $b] = $expr`: sequential array destructuring.
- `match (valor) { clave, clave => resultado, default => resultado }`: compares
  strictly, first matching arm; without match/default returns null.
- `try { ... } catch ($error) { ... }`, `throw expresión`: runtime recovery.
- `return [expresión]`: exits the callable.
- `async { ... }`: creates a `Future`; collected with `await($futuro)` or `await $futuro`.
- `select { case ... default: ... }`: multiplexing over channels.
- `yield [clave =>] valor`: lazy emission in generator functions.
- `Console::*`: native module for ANSI colors (`Console::green`, `Console::red`, `Console::bold`, etc.).
- `joss repl`: interactive terminal environment (Read-Eval-Print Loop).

## Omissions and Compatibility

There are no source imports, namespaces, file exports, traits,
protocols, ownership, manual pointers, `switch`, classic `for`, or arrow function syntax sugar.
`ref` only serves as a temporary parameter/argument and is not a storable pointer.

`function`, `import`, `@import`, `use`, `Use`, `Import`,
`namespace`, and `Namespace` generate removed-syntax errors (`JOSS-PARSE-001`).
Legacy API names are not necessarily valid source types:
`is_integer` remains registered as a built-in function, but `integer $x` is not an alias for `int` (`JOSS-TYPE-009`).

[Index](README.md)

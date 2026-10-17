# Reading grammar and correspondence with the parser

[Index](README.md)

Before: [syntax](SINTAXIS.md). Go deeper: [architecture](ARQUITECTURA.md).
This grammar summarizes the implemented productions. It does not replace the Pratt
parser nor promise to accept all programs that an isolated EBNF would produce:
visibility, types, and valid locations of `ref` are validated afterwards.

## Notation

Quotation marks indicate literal text. `[...]` indicates optional; `{...}` repetition;
`|` separates alternatives. `expr` uses the reference precedence table.
`sep` is end of line or `;`; lists allow newlines and trailing commas.

```ebnf
programa      = { sentencia [sep] } ;
sentencia     = declaracion | destructuracion | funcion | clase | record | interfaz | enum
              | inicializador | guard | defer | select
              | "return" [expr] | "throw" expr
              | "break" | "continue" | ciclo | captura
              | ("print" | "echo") ["("] expr [")"] | expr ;
variable      = "$" identificador ;
declaracion   = [visibilidad] ["static"] ["const"]
                ("var" | "let" [tipo] | tipo) variable ["=" expr]
                {"," variable ["=" expr]} ;
destructuracion = "[" [variable] {"," [variable]} "]" "=" expr ;
funcion       = [visibilidad] ["abstract"] ["static"] "func" identificador firma (bloque | [sep]) ;
firma         = "(" [parametros] ")" [":" tipo] ;
parametros    = parametro {"," parametro} [","] ;
parametro     = [visibilidad] ["const"] ["ref"] tipo variable ["=" expr] ;
closure       = "func" firma bloque ;
clase         = visibilidad ["abstract"] ["static"] "class" identificador ["<" identificador {"," identificador} ">"]
                ["extends" identificador]
                ["implements" identificador {"," identificador}] "{" {miembro} "}" ;
record        = visibilidad "record" identificador "(" [parametrosRecord] ")" ;
parametrosRecord = [visibilidad] ["const"] tipo variable ["=" expr] {"," [visibilidad] ["const"] tipo variable ["=" expr]} ;
interfaz      = visibilidad "interface" identificador
                ["extends" identificador {"," identificador}] "{" {protoMetodo} "}" ;
enum          = visibilidad "enum" identificador [":" tipo] "{" {casoEnum} "}" ;
casoEnum      = "case" identificador ["=" expr] [sep] ;
protoMetodo   = [visibilidad] "func" identificador "(" [parametros] ")" [":" tipo] [sep] ;
miembro       = declaracion | funcion | inicializador ;
inicializador = "Init" [identificador] "(" [parametros] ")" bloque ;
guard         = "guard" "(" expr ")" ":" bloque ;
defer         = "defer" (bloque | sentencia) ;
visibilidad   = "public" | "private" | "protected" ;
tipo          = simple {"|" simple} ["?"] ;
simple        = nombreTipo ["<" tipo {"," tipo} ">"] ;
bloque        = "{" {sentencia [sep]} "}" ;
ciclo         = "while" "(" expr ")" bloque
              | "do" bloque "while" "(" expr ")"
              | "foreach" "(" expr "as" [variable "=>"] variable ")" bloque ;
captura       = "try" bloque "catch" "(" variable ")" bloque ;
select        = "select" "{" {casoSelect} "}" ;
casoSelect    = ("case" (("send" "(" expr "," expr ")") | ("recv" "(" expr ")") | (variable "=" "recv" "(" expr ")")) | "default") ":" {sentencia [sep]} ;
array         = "[" [elementoArray {"," elementoArray} [","]] "]" ;
elementoArray = expr | "..." expr ;
map           = "{" [elementoMap {"," elementoMap} [","]] "}" ;
elementoMap   = expr ":" expr | "..." expr ;
match         = "match" "(" expr ")" "{" {brazo} "}" ;
brazo         = ("default" | expr {"," expr}) "=>" (expr | bloque) [","] ;
llamada       = expr "(" [argumento {"," argumento} [","]] ")" ;
argumento     = expr | "ref" variable | identificador ":" expr | "..." expr ;
yield         = "yield" [expr ["=>" expr]] ;
await         = "await" ( "(" expr ")" | expr ) ;
comprobacion  = expr ("is" | "instanceof") tipo ;
acceso        = expr ("->" | "?->" | "::") nombreMiembro ;
indice        = expr "[" [expr] "]" ;
nuevo         = "new" identificador ["<" tipo {"," tipo} ">"] "(" [argumentos] ")" ;
asincrono     = "async" bloque ;
```

## Restrictions that the notation does not express

- `protected` only applies to class members, not global classes/functions.
- `static` requires explicit visibility. `Init` and closures do not carry it.
- The parser preserves untyped parameters to be able to emit `JOSS-TYPE-011`;
  valid code grammar requires the type.
- `const` requires an initializer and has its own parsing path.
- Canonical type names are described in [types](SISTEMA_TIPOS.md);
  accepting the name in the parser does not prove that a class exists.
- An empty index `[]` only works for append as an assignment target (`$arr[] = $v`).
- In an expression, `{}` is interpreted as an empty map. A mandatory body is
  parsed as a block. `{ "a": 1 }` is distinguished from a block by `:`.
- Ternary uses `cond ? expr : expr`, `cond ?: expr`, or single-branch conditional
  `(cond) ? bloque`; its branches accept blocks or expressions.
- `guard (cond) : { ... }` requires its block to exit the callable via `return` or `throw` (`JOSS-FLOW-005`).
- Double-quoted strings support expression interpolation `${expr}`
  desugared to `.` concatenation. Numeric range operators
  `..`, postfix decrement `--`, and null-coalescing assignment `??=` are incorporated.
- `async expresión` is still a parser route, with early evaluation
  of the argument; `async(...)` is rejected. The recommended form is the block `async { ... }`.
- Calls, arrays, and signatures allow trailing commas. Continuation of an
  expression after a newline is decided by `isExpressionContinuation`.

## From text to node

| Production | Parser | AST Node |
|---|---|---|
| Program | `ParseProgram` | `Program` |
| Expression by precedence | `parseExpression`, prefix/infix registries | `Expression` and concrete types |
| Assignment | `parseAssignExpression` | `AssignExpression` |
| Declared variable | Routes of `parseStatement` | `LetStatement`, `MultiLetStatement` |
| Destructuring | `parseDestructureStatement` | `DestructureStatement` |
| Global function/method | `parseMethodStatement` | `MethodStatement` |
| Closure | `parseFunctionLiteral` | `FunctionLiteral` |
| Type | `parseTypeReference` | Normalized token preserved in declaration/signature |
| Class / Record / Init | `parseClassStatement`, `parseRecordStatement`, `parseInitStatement` | `ClassStatement`, `InitStatement` |
| Interface | `parseInterfaceStatement` | `InterfaceStatement` |
| Enum | `parseEnumStatement` | `EnumStatement` |
| Guard | `parseGuardStatement` | `GuardStatement` |
| Defer | `parseDeferStatement` | `DeferStatement` |
| Select | `parseSelectStatement` | `SelectStatement` |
| Yield | `parseYieldExpression` | `YieldExpression` |
| Spread | `parseSpreadExpression` | `SpreadExpression` |
| Type check | `parseIsInfix` | `IsExpression` |
| Loops | `parseForeachStatement`, `parseWhileStatement`, `parseDoWhileStatement` | Their statement nodes |
| Error | `parseTryCatchStatement`, `parseThrowStatement` | `TryCatchStatement`, `ThrowStatement` |
| Async / Await | `parseAsyncExpression`, `parseAwaitExpression` | `CallExpression` to `async`, `PrefixExpression` / `CallExpression` |

The files are `pkg/parser/parser*.go`, `ast*.go`, `lexer.go`, and `token.go`.
To add syntax, update positive and negative tests before changing
this reference. Do not turn residual token names into public features.

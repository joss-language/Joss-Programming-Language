# Reading grammar and correspondence with the parser

[Index](README.md)

Before: [syntax](SINTAXIS.md). Go deeper: [architecture](ARQUITECTURA.md).
This grammar summarizes the implemented productions. Does not replace the parser
Pratt does not promise to accept all the programs that an isolated EBNF would produce:
the visibility, types, and valid locations of `ref` are then validated.

## Notation

Quotation marks indicate literal text. `[...]` indicates optional; `{...}` repeat;
`|` separates alternatives. `expr` uses the reference precedence table.
`sep` is end of line or `;`; lists allow new lines and trailing commas.

```ebnf
programa      = { sentencia [sep] } ;
sentencia     = declaracion | funcion | clase | interfaz | enum | inicializador | select
              | "return" [expr] | "throw" expr
              | "break" | "continue" | ciclo | captura
              | ("print" | "echo") ["("] expr [")"] | expr ;
variable      = "$" identificador ;
declaracion   = [visibilidad] ["static"] ["const"]
                ("let" [tipo] | tipo) variable ["=" expr]
                {"," variable ["=" expr]} ;
funcion       = [visibilidad] ["abstract"] ["static"] "func" identificador firma (bloque | [sep]) ;
firma         = "(" [parametros] ")" [":" tipo] ;
parametros    = parametro {"," parametro} [","] ;
parametro     = [visibilidad] ["ref"] tipo variable ["=" expr] ;
closure       = "func" firma bloque ;
clase         = visibilidad ["abstract"] ["static"] "class" identificador
                ["extends" identificador]
                ["implements" identificador {"," identificador}] "{" {miembro} "}" ;
interfaz      = visibilidad "interface" identificador
                ["extends" identificador {"," identificador}] "{" {protoMetodo} "}" ;
enum          = visibilidad "enum" identificador [":" tipo] "{" {casoEnum} "}" ;
casoEnum      = "case" identificador ["=" expr] [sep] ;
protoMetodo   = [visibilidad] "func" identificador "(" [parametros] ")" [":" tipo] [sep] ;
miembro       = declaracion | funcion | inicializador ;
inicializador = "Init" [identificador] "(" [parametros] ")" bloque ;
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
comprobacion  = expr ("is" | "instanceof") tipo ;
acceso        = expr ("->" | "?->" | "::") nombreMiembro ;
indice        = expr "[" [expr] "]" ;
nuevo         = "new" identificador "(" [argumentos] ")" ;
asincrono     = "async" bloque ;
```

## Restrictions that the notation does not express

- `protected` only applies to members, not global classes/functions.
- `static` requires explicit visibility. `Init` and closures do not have it.
- The parser preserves untyped parameters to be able to emit `JOSS-TYPE-011`;
  valid code grammar requires the type.
- `const` requires an initializer and has its own parsing path.
- Canonical type names are described in [types](SISTEMA_TIPOS.md);
  accepting the name in the parser does not prove that a class exists.
- The only semantic genericity implemented is that of arrays/maps.
- An empty index only works for append as an allocation destination.
- In expression, `{}` is interpreted as empty map. A mandatory body
  parse as a block. `{ "a": 1 }` is distinguished from a block by `:`.
- Ternary uses `cond ? expr : expr`, `cond ?: expr` or single branch conditional
  `(cond) ? bloque`; its branches support blocks or expressions. `match` evaluates and
  executes both expressions and blocks of multiline statements.
- Strings with double quotes support expression interpolation `${expr}`
  desugaring to concatenation `.`. Numerical range operators are incorporated
  `..`, postfix decrement `--` and coalescent null assignment `??=`.
- `async expresión` is still a parser route, with early evaluation
  of the argument; `async(...)` is rejected. The recommended shape is the block.
- Calls, arrays and signatures allow trailing commas. The continuity of a
  expression after a jump is decided by `isExpressionContinuation`; not all
  postfix tokens have identical treatment (for example `?->`).

## From text to node

| Production | Parser | AST Node |
|---|---|---|
| Program | `ParseProgram` | `Program` |
| Expression by precedence | `parseExpression`, prefix/infix logs | `Expression` and concrete types |
| Assignment | `parseAssignExpression` | `AssignExpression` |
| Declared variable | Routes from `parseStatement` | `LetStatement`, `MultiLetStatement` |
| Global function/method | `parseMethodStatement` | `MethodStatement` |
| Closure | `parseFunctionLiteral` | `FunctionLiteral` |
| Type | `parseTypeReference` | Normalized token preserved in declaration/signature |
| Class/Init | `parseClassStatement`, `parseInitStatement` | `ClassStatement`, `InitStatement` |
| Interface | `parseInterfaceStatement` | `InterfaceStatement` |
| Enum | `parseEnumStatement` | `EnumStatement` |
| Select | `parseSelectStatement` | `SelectStatement` |
| Yield | `parseYieldExpression` | `YieldExpression` |
| Spread | `parseSpreadExpression` | `SpreadExpression` |
| Type checking | `parseIsInfix` | `IsExpression` |
| Cycles | `parseForeachStatement`, `parseWhileStatement`, `parseDoWhileStatement` | Your statement nodes |
| Error | `parseTryCatchStatement`, `parseThrowStatement` | `TryCatchStatement`, `ThrowStatement` |
| Defer | `parseDeferStatement` | `DeferStatement` |
| Async | `parseAsyncExpression` | `CallExpression` to `async` with captured function |

The files are `pkg/parser/parser*.go`, `ast*.go`, `lexer.go` and `token.go`.
To add syntax, update positive and negative tests before changing
this reference. Don't convert residual token names into public features.

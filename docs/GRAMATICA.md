# Gramática de lectura y correspondencia con el parser

[Índice](README.md)

Antes: [sintaxis](SINTAXIS.md). Profundizar: [arquitectura](ARQUITECTURA.md).
Esta gramática resume las producciones implementadas. No reemplaza al parser
Pratt ni promete aceptar todos los programas que una EBNF aislada produciría:
la visibilidad, los tipos y los lugares válidos de `ref` se validan después.

## Notación

Comillas indican texto literal. `[...]` indica opcional; `{...}` repetición;
`|` separa alternativas. `expr` usa la tabla de precedencia de la referencia.
`sep` es fin de línea o `;`; las listas permiten nuevas líneas y coma final.

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

## Restricciones que la notación no expresa

- `protected` sólo corresponde a miembros de clase, no a clases/funciones globales.
- `static` requiere visibilidad explícita. `Init` y closures no la llevan.
- El parser conserva parámetros sin tipo para poder emitir `JOSS-TYPE-011`;
  la gramática de código válido exige el tipo.
- `const` exige inicializador y tiene su propia ruta de parseo.
- Los nombres de tipos canónicos se describen en [tipos](SISTEMA_TIPOS.md);
  aceptar el nombre en el parser no prueba que exista una clase.
- Un índice vacío `[]` sólo sirve para append como destino de asignación (`$arr[] = $v`).
- En expresión, `{}` se interpreta como mapa vacío. Un cuerpo obligatorio se
  parsea como bloque. `{ "a": 1 }` se distingue de un bloque por `:`.
- El ternario usa `cond ? expr : expr`, `cond ?: expr` o condicional de una sola rama
  `(cond) ? bloque`; sus ramas admiten bloques o expresiones.
- `guard (cond) : { ... }` exige que su bloque salga del callable mediante `return` o `throw` (`JOSS-FLOW-005`).
- Las cadenas con comillas dobles admiten interpolación de expresiones `${expr}`
  desazucarada a concatenación `.`. Se incorporan los operadores de rango numérico
  `..`, decremento postfix `--` y asignación nula coalescente `??=`.
- `async expresión` todavía es una ruta del parser, con evaluación anticipada
  del argumento; `async(...)` se rechaza. La forma recomendada es el bloque `async { ... }`.
- Llamadas, arrays y firmas permiten comas finales. La continuidad de una
  expresión tras un salto se decide por `isExpressionContinuation`.

## Del texto al nodo

| Producción | Parser | Nodo AST |
|---|---|---|
| Programa | `ParseProgram` | `Program` |
| Expresión por precedencia | `parseExpression`, registros prefix/infix | `Expression` y tipos concretos |
| Asignación | `parseAssignExpression` | `AssignExpression` |
| Variable declarada | Rutas de `parseStatement` | `LetStatement`, `MultiLetStatement` |
| Destructuración | `parseDestructureStatement` | `DestructureStatement` |
| Función global/método | `parseMethodStatement` | `MethodStatement` |
| Closure | `parseFunctionLiteral` | `FunctionLiteral` |
| Tipo | `parseTypeReference` | Token normalizado conservado en declaración/firma |
| Clase / Record / Init | `parseClassStatement`, `parseRecordStatement`, `parseInitStatement` | `ClassStatement`, `InitStatement` |
| Interfaz | `parseInterfaceStatement` | `InterfaceStatement` |
| Enum | `parseEnumStatement` | `EnumStatement` |
| Guard | `parseGuardStatement` | `GuardStatement` |
| Defer | `parseDeferStatement` | `DeferStatement` |
| Select | `parseSelectStatement` | `SelectStatement` |
| Yield | `parseYieldExpression` | `YieldExpression` |
| Spread | `parseSpreadExpression` | `SpreadExpression` |
| Comprobación de tipo | `parseIsInfix` | `IsExpression` |
| Ciclos | `parseForeachStatement`, `parseWhileStatement`, `parseDoWhileStatement` | Sus nodos de sentencia |
| Error | `parseTryCatchStatement`, `parseThrowStatement` | `TryCatchStatement`, `ThrowStatement` |
| Async / Await | `parseAsyncExpression`, `parseAwaitExpression` | `CallExpression` a `async`, `PrefixExpression` / `CallExpression` |

Los archivos son `pkg/parser/parser*.go`, `ast*.go`, `lexer.go` y `token.go`.
Para añadir sintaxis, actualiza pruebas positivas y negativas antes de cambiar
esta referencia. No conviertas nombres de tokens residuales en features públicas.

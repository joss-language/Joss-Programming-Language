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
sentencia     = declaracion | funcion | clase | inicializador
              | "return" [expr] | "throw" expr
              | "break" | "continue" | ciclo | captura
              | ("print" | "echo") ["("] expr [")"] | expr ;
variable      = "$" identificador ;
declaracion   = [visibilidad] ["static"] ["const"]
                ("let" [tipo] | tipo) variable ["=" expr]
                {"," variable ["=" expr]} ;
funcion       = visibilidad ["static"] "func" identificador firma bloque ;
firma         = "(" [parametros] ")" [":" tipo] ;
parametros    = parametro {"," parametro} [","] ;
parametro     = ["ref"] tipo variable ["=" expr] ;
closure       = "func" firma bloque ;
clase         = visibilidad ["static"] "class" identificador
                ["extends" identificador] "{" {miembro} "}" ;
miembro       = declaracion | funcion | inicializador ;
inicializador = "Init" identificador "(" [parametros] ")" bloque ;
visibilidad   = "public" | "private" | "protected" ;
tipo          = simple {"|" simple} ["?"] ;
simple        = nombreTipo ["<" tipo {"," tipo} ">"] ;
bloque        = "{" {sentencia [sep]} "}" ;
ciclo         = "while" "(" expr ")" bloque
              | "do" bloque "while" "(" expr ")"
              | "foreach" "(" expr "as" variable ")" bloque ;
captura       = "try" bloque "catch" "(" variable ")" bloque ;
array         = "[" [expr {"," expr} [","]] "]" ;
map           = "{" [expr ":" expr {"," expr ":" expr} [","]] "}" ;
match         = "match" "(" expr ")" "{" {brazo} "}" ;
brazo         = ("default" | expr {"," expr}) "=>" (expr | bloque) [","] ;
llamada       = expr "(" [argumento {"," argumento} [","]] ")" ;
argumento     = expr | "ref" variable ;
acceso        = expr ("->" | "?->" | "::") nombreMiembro ;
indice        = expr "[" [expr] "]" ;
nuevo         = "new" identificador "(" [argumentos] ")" ;
asincrono     = "async" bloque ;
```

## Restricciones que la notación no expresa

- `protected` sólo corresponde a miembros, no a clases/funciones globales.
- `static` requiere visibilidad explícita. `Init` y closures no la llevan.
- El parser conserva parámetros sin tipo para poder emitir `JOSS-TYPE-011`;
  la gramática de código válido exige el tipo.
- `const` exige inicializador y tiene su propia ruta de parseo.
- Los nombres de tipos canónicos se describen en [tipos](SISTEMA_TIPOS.md);
  aceptar el nombre en el parser no prueba que exista una clase.
- La única genericidad semántica implementada es la de arrays/maps.
- Un índice vacío sólo sirve para append como destino de asignación.
- En expresión, `{}` se interpreta como mapa vacío. Un cuerpo obligatorio se
  parsea como bloque. `{ "a": 1 }` se distingue de un bloque por `:`.
- El ternario usa `cond ? expr : expr`, `cond ?: expr` o condicional de una sola rama
  `(cond) ? bloque`; sus ramas admiten bloques o expresiones. `match` evalúa y
  ejecuta tanto expresiones como bloques de sentencias multilínea.
- Las cadenas con comillas dobles admiten interpolación de expresiones `${expr}`
  desazucarada a concatenación `.`. Se incorporan los operadores de rango numérico
  `..`, decremento postfix `--` y asignación nula coalescente `??=`.
- `async expresión` todavía es una ruta del parser, con evaluación anticipada
  del argumento; `async(...)` se rechaza. La forma recomendada es el bloque.
- Llamadas, arrays y firmas permiten comas finales. La continuidad de una
  expresión tras un salto se decide por `isExpressionContinuation`; no todos
  los tokens postfix tienen idéntico tratamiento (por ejemplo `?->`).

## Del texto al nodo

| Producción | Parser | Nodo AST |
|---|---|---|
| Programa | `ParseProgram` | `Program` |
| Expresión por precedencia | `parseExpression`, registros prefix/infix | `Expression` y tipos concretos |
| Asignación | `parseAssignExpression` | `AssignExpression` |
| Variable declarada | Rutas de `parseStatement` | `LetStatement`, `MultiLetStatement` |
| Función global/método | `parseMethodStatement` | `MethodStatement` |
| Closure | `parseFunctionLiteral` | `FunctionLiteral` |
| Tipo | `parseTypeReference` | Token normalizado conservado en declaración/firma |
| Clase / Init | `parseClassStatement`, `parseInitStatement` | `ClassStatement`, `InitStatement` |
| Ciclos | `parseForeachStatement`, `parseWhileStatement`, `parseDoWhileStatement` | Sus nodos de sentencia |
| Error | `parseTryCatchStatement`, `parseThrowStatement` | `TryCatchStatement`, `ThrowStatement` |
| Async | `parseAsyncExpression` | `CallExpression` a `async` con función capturada |

Los archivos son `pkg/parser/parser*.go`, `ast*.go`, `lexer.go` y `token.go`.
Para añadir sintaxis, actualiza pruebas positivas y negativas antes de cambiar
esta referencia. No conviertas nombres de tokens residuales en features públicas.

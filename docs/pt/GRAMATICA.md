# Gramática de leitura e correspondência com o parser

[Índice](README.md)

Antes: [sintaxe](SINTAXIS.md). Aprofunde-se: [arquitetura](ARQUITECTURA.md).
Esta gramática resume as produções implementadas. Não substitui o parser
Pratt nem promete aceitar todos os programas que uma EBNF isolada produziria:
a visibilidade, os tipos e os locais válidos de `ref` são validados depois.

## Notação

Aspas indicam texto literal. `[...]` indica opcional; `{...}` repetição;
`|` separa alternativas. `expr` usa a tabela de precedência da referência.
`sep` é fim de linha ou `;`; as listas permitem novas linhas e vírgula final.

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

## Restrições que a notação não expressa

- `protected` aplica-se apenas a membros de classe, não a classes/funções globais.
- `static` requer visibilidade explícita. `Init` e closures não a possuem.
- O parser preserva parâmetros sem tipo para poder emitir `JOSS-TYPE-011`;
  a gramática de código válido exige o tipo.
- `const` exige inicializador e tem sua própria rota de análise.
- Os nomes de tipos canônicos são descritos em [tipos](SISTEMA_TIPOS.md);
  aceitar o nome no parser não prova que exista uma classe.
- Um índice vazio `[]` só serve para append como destino de atribuição (`$arr[] = $v`).
- Em expressão, `{}` é interpretado como mapa vazio. Um corpo obrigatório é
  analisado como bloco. `{ "a": 1 }` distingue-se de um bloco por `:`.
- O ternário usa `cond ? expr : expr`, `cond ?: expr` ou condicional de uma única ramificação
  `(cond) ? bloque`; suas ramificações admitem blocos ou expressões.
- `guard (cond) : { ... }` exige que seu bloco saia do callable mediante `return` ou `throw` (`JOSS-FLOW-005`).
- As cadeias com aspas duplas admitem interpolação de expressões `${expr}`
  desaçucarada para concatenação `.`. Incorporam-se os operadores de intervalo numérico
  `..`, decremento postfix `--` e atribuição nula coalescente `??=`.
- `async expresión` ainda é uma rota do parser, com avaliação antecipada
  do argumento; `async(...)` é rejeitado. A forma recomendada é o bloco `async { ... }`.
- Chamadas, arrays e assinaturas permitem vírgulas finais. A continuidade de uma
  expressão após uma quebra de linha é decidida por `isExpressionContinuation`.

## Do texto ao nó

| Produção | Parser | Nó AST |
|---|---|---|
| Programa | `ParseProgram` | `Program` |
| Expressão por precedência | `parseExpression`, registros prefix/infix | `Expression` e tipos concretos |
| Atribuição | `parseAssignExpression` | `AssignExpression` |
| Variável declarada | Rotas de `parseStatement` | `LetStatement`, `MultiLetStatement` |
| Desestruturação | `parseDestructureStatement` | `DestructureStatement` |
| Função global/método | `parseMethodStatement` | `MethodStatement` |
| Closure | `parseFunctionLiteral` | `FunctionLiteral` |
| Tipo | `parseTypeReference` | Token normalizado preservado na declaração/assinatura |
| Classe / Record / Init | `parseClassStatement`, `parseRecordStatement`, `parseInitStatement` | `ClassStatement`, `InitStatement` |
| Interface | `parseInterfaceStatement` | `InterfaceStatement` |
| Enum | `parseEnumStatement` | `EnumStatement` |
| Guard | `parseGuardStatement` | `GuardStatement` |
| Defer | `parseDeferStatement` | `DeferStatement` |
| Select | `parseSelectStatement` | `SelectStatement` |
| Yield | `parseYieldExpression` | `YieldExpression` |
| Spread | `parseSpreadExpression` | `SpreadExpression` |
| Verificação de tipo | `parseIsInfix` | `IsExpression` |
| Laços | `parseForeachStatement`, `parseWhileStatement`, `parseDoWhileStatement` | Seus nós de instrução |
| Erro | `parseTryCatchStatement`, `parseThrowStatement` | `TryCatchStatement`, `ThrowStatement` |
| Async / Await | `parseAsyncExpression`, `parseAwaitExpression` | `CallExpression` a `async`, `PrefixExpression` / `CallExpression` |

Os arquivos são `pkg/parser/parser*.go`, `ast*.go`, `lexer.go` e `token.go`.
Para adicionar sintaxe, atualize testes positivos e negativos antes de alterar
esta referência. Não converta nomes de tokens residuais em funcionalidades públicas.
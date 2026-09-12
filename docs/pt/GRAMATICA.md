# Lendo gramática e correspondência com o analisador

[Índice](README.md)

Antes: [sintaxe](SINTAXIS.md). Aprofunde-se: [arquitetura](ARQUITECTURA.md).
Esta gramática resume as produções implementadas. Não substitui o analisador
A Pratt não promete aceitar todos os programas que uma EBNF isolada produziria:
a visibilidade, tipos e locais válidos de `ref` são então validados.

## Notação

As aspas indicam texto literal. `[...]` indica opcional; `{...}` repetição;
`|` separa alternativas. `expr` usa a tabela de precedência de referência.
`sep` é fim de linha ou `;`; listas permitem novas linhas e vírgulas finais.```ebnf
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
## Restrições que a notação não expressa

- `protected` aplica-se apenas a membros, não a classes/funções globais.
- `static` requer visibilidade explícita. `Init` e encerramentos não o possuem.
- O analisador preserva parâmetros não digitados para emitir `JOSS-TYPE-011`;
  a gramática de código válida requer o tipo.
- `const` requer um inicializador e possui seu próprio caminho de análise.
- Os nomes dos tipos canônicos são descritos em [types](SISTEMA_TIPOS.md);
  aceitar o nome no analisador não prova que existe uma classe.
- A única genericidade semântica implementada é a de arrays/mapas.
- Um índice vazio só funciona para acréscimo como destino de alocação.
- Na expressão, `{}` é interpretado como mapa vazio. Um órgão obrigatório
  analisar como um bloco. `{ "a": 1 }` é diferenciado de um bloco por `:`.
- O ternário usa `cond ? expr : expr`, `cond ?: expr` ou condicional de ramificação única
  `(cond) ? bloque`; suas ramificações suportam blocos ou expressões. `match` avalia e
  executa expressões e blocos de instruções multilinhas.
- Strings entre aspas duplas suportam interpolação de expressão `${expr}`
  desaçucarado para concatenação `.`. Operadores de intervalo numérico são incorporados
  `..`, decremento de postfix `--` e atribuição nula coalescente `??=`.
- `async expresión` ainda é um caminho de analisador, com avaliação antecipada
  do argumento; `async(...)` é rejeitado. A forma recomendada é o bloco.
- Chamadas, matrizes e assinaturas permitem vírgulas finais. A continuidade de um
  a expressão após um salto é decidida por `isExpressionContinuation`; nem todos
  tokens postfix têm tratamento idêntico (por exemplo `?->`).

## Do texto ao nó

| Produção | Analisador | Nó AST |
|---|---|---|
| Programa | `ParseProgram` | `Program` |
| Expressão por precedência | `parseExpression`, registros de prefixo/infixo | `Expression` e tipos concretos |
| Atribuição | `parseAssignExpression` | `AssignExpression` |
| Variável declarada | Rotas `parseStatement` | `LetStatement`, `MultiLetStatement` |
| Função/método global | `parseMethodStatement` | `MethodStatement` |
| Encerramento | `parseFunctionLiteral` | `FunctionLiteral` |
| Tipo | `parseTypeReference` | Token normalizado preservado na declaração/assinatura |
| Classe/Início | `parseClassStatement`, `parseInitStatement` | `ClassStatement`, `InitStatement` |
| Interface | `parseInterfaceStatement` | `InterfaceStatement` |
| Enum | `parseEnumStatement` | `EnumStatement` |
| Selecione | `parseSelectStatement` | `SelectStatement` |
| Rendimento | `parseYieldExpression` | `YieldExpression` |
| Espalhar | `parseSpreadExpression` | `SpreadExpression` |
| Verificação de tipo | `parseIsInfix` | `IsExpression` |
| Ciclos | `parseForeachStatement`, `parseWhileStatement`, `parseDoWhileStatement` | Seus nós de instrução |
| Erro | `parseTryCatchStatement`, `parseThrowStatement` | `TryCatchStatement`, `ThrowStatement` |
| Adiar | `parseDeferStatement` | `DeferStatement` |
| Assíncrono | `parseAsyncExpression` | `CallExpression` para `async` com função capturada |

Os arquivos são `pkg/parser/parser*.go`, `ast*.go`, `lexer.go` e `token.go`.Para adicionar sintaxe, atualize os testes positivos e negativos antes de alterar
esta referência. Não converta nomes de tokens residuais em recursos públicos.
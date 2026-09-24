#

Referência de sintaxe e operadores Para aprender: [fundamentos](FUNDAMENTOS.md) , [fluxo](CONTROL_FLUJO.md) ,
[funções](FUNCIONES.md) , classes [CLASES.md](CLASES.md) . Plug-ins
: gramática [GRAMATICA.md](GRAMATICA.md) , tipos [SISTEMA_TIPOS.md](SISTEMA_TIPOS.md) .

Esta referência descreve o analisador e o avaliador para este patch. As limitações observadas de
são indicadas como tal; Não são propostas de design.

## Lexicon

As palavras reservadas são obtidas de `parser.KeywordNames()` :

```text
Init abstract as async break case catch class const continue default defer do echo empty
enum extends false foreach func implements instanceof interface is isset let match new nil null print private
protected public ref return select static this throw true try while yield
```

`var` , `int` , `mixed` , `await` e `make_chan` são identificadores
interpretados por nomes de contexto ou função, não por palavras-chave lexer.
As constantes internas `IF` e `ELSE` não significam que essas construções
estejam disponíveis. `if` , `else` , `elif` , `for` e `switch` não são
estruturas de origem Joss.

| Forma | Regra |
|---|---|
| Identificador | ASCII: letras, `_` ou `@` no início; também dígitos depois. Evite `@` fora de APIs específicas; Não é um sistema de pontuação. |
| Variável | `$nombre` ; o token `$` é separado do nome. `$this` tem tratamento próprio. |
| Comentários | `//` e `#` até o final da linha; `/* ... */` sem aninhamento. |
| Cordas | Aspas simples ou duplas; escapa `\n` , `\t` , `\r` , `\'` , `\"` , `\\` ; Interpolação estilo Dart/Flutter entre aspas duplas: `"${var}"` ou `"${expr}"` (escape `\${` ); aspas simples sem interpolação. |
| Inteiros | Sequência de dígitos; o analisador usa base automática: `010` é lido como octal, `08` falha. Evite zeros à esquerda. |
| Flutuadores | Dígitos, ponto final e mais dígitos: `0.5` . Não há literal exponencial, literal hexadecimal ou separador `_` no lexer. |
| Decimais | Inteiro ou fração com sufixo `m` / `M` : `100m` , `1.25M` . |
| Ausência | `null` e `nil` produzem o mesmo valor. |
| Separação | Nova linha ou `;` . Espaços e tabulações não delimitam blocos. |

O lexer remove o BOM UTF-8 inicial. Fora das strings, omita bytes não ASCII:
não confie em identificadores acentuados. O diagnóstico para sequências multilinhas
e texto não ASCII ainda tem limitações de posição.

Além de nomes, literais e palavras-chave, eles são tokenizados:

```text
= += -= *= /= ??= + - ! * / % < > == != === !== <=> <= >= << >> && || ++ --
, ; : ? ( ) { } [ ] . .. ... -> ?-> :: | |> ?? =>
NEWLINE EOF ILLEGAL
```

Não há exponenciação, binário AND `&` ou binário OR `|` ;
o último separa os tipos de união.

## Precedência: do menor para o maior

A tabela reproduz `pkg/parser/parser.go` . No mesmo nível,
comum infixa o grupo à esquerda; a tarefa analisa todos os seus direitos.

| Nível | Operadores/construções |
|---|---|
| 1 | `=` , `+=` , `-=` , `*=` , `/=` , `??=` (direita) |
| 2 | `? :` , `?:` |
| 3 | `??` |
| 4 | `\|>` |
| 5 | `\|\|` |
| 6 | `&&` |
| 7 | `==` , `!=` , `===` , `!==` , `<=>` |
| 8 | `<` , `>` , `<=` , `>=` , `is` , `instanceof` |
| 9 | `<<` , `>>` |
| 10 | `..` |
| 11 | `+` , `-` , `.` |
| 12 | `*` , `/` , `%` |
| 13 | Prefixos `-` , `!` , `ref` , `...` |
| 14 | Ligue para `()` |
| 15 | Índice `[]` , membros `->` , `?->` , `::` , postfix `++` , `--` |

Os operadores aritméticos e lógicos seguem sua hierarquia convencional: `%` ,
`*` e `/` nível de compartilhamento e `&&` se liga mais forte que `||` .
Os braços do ternário são analisados ​​como expressões completas; coloca entre parênteses
ternários aninhados em vez de transferir a associatividade de outro idioma.

<!-- joss-run: ["1", "true", "true"] -->
```joss
print(8 * 5 % 3)
print(true || false && false)
print(true || (false && false))
```

Fornece `1` , `true` e `true` . A primeira expressão é `(8 * 5) % 3` .

## Avaliação

- `+` , `-` , `*` : inteiros exatos com overflow verificado; promoção se houver
float, operações decimais se decimal estiver envolvido.
- `/` : resultado flutuante para inteiros; decimal se decimal estiver envolvido.
- `+=` , `-=` , `*=` , `/=` , `??=` : atribuição composta e atribuição nula coalescente
( `$a ??= $b` é equivalente a `$a = $a ?? $b` ) com validação de tipo estrita
.
- `%` : número inteiro restante; com float trunca operandos para número inteiro; decimal usa `Mod` .
- `.` - representa operandos como texto e concatena; `null` contribui com texto vazio.
- `..` : operador de intervalo numérico ( `$a..$b` ); gera uma sequência inteira entre
ambos os limites, úteis em expressões e loops `foreach (1..10 as $i)` .
- `"${...}"` - Interpolação de string estilo Flutter/Dart dentro de aspas duplas
. Ele permite incorporar variáveis ​​​​ou expressões (por exemplo, `"${indice}"` , `"${a + b}"` ),
, simplificando automaticamente as concatenações de tipo de string. O escape `\${`
imprime `${` literalmente.
- `++` , `--` : incrementa ou decrementa a variável numérica e retorna o valor anterior.
- `&&` , `||` : curto-circuito e resultado bool. `!` : negação da veracidade.
- `==` / `!=` : comparação numérica quando aplicável; fallback usando representação textual
para outros valores. Para coleções prefira
`===` / `!==` , com base na comparação estrutural.
- `===` : distingue inteiro, flutuante, string e decimal; normaliza variantes numéricas
internas do Go antes de comparar.
- `<=>` : retorna `-1` , `0` ou `1` ; compara números, strings, nulos e
finalmente representações textuais.
- `??` : avalia a direita somente se a esquerda produzir `null` . Erros quando
avalia left são propagados e devem ser tratados com `try` / `catch` .
- `?:` (Elvis): preserva à esquerda se for verdadeiro de acordo com a veracidade.
- Ternário completo de ramificação única: `cond ? expr : expr` e `(cond) ? { cuerpo }`
(permite que a ramificação `: {}` seja ignorada quando a alternativa falsa não for necessária).
- `match` : seleção múltipla por valor; suporta expressões ou blocos multilinha `{ ... }` .
- `?->` : retorna nulo antes do receptor nulo; Não valida nem corrige outros acessos.
- `|>` : acrescenta o valor esquerdo aos argumentos de uma função, chamada
ou encerramento.
- `cout << valor` : imprime sem adicionar quebra e retorna `cout` .
`canal << valor` : Enviar para o canal. `cin >> $variable` - Lê
de forma adaptativa da entrada padrão (convertendo automaticamente em número se a variável ou entrada
for numérica e lendo linhas completas com espaços se for texto).

## Verdade dos valores

Joss mantém uma veracidade pequena e uniforme. Eles são falsos: `null` , `false` ,
zero de qualquer tipo numérico, string vazia, array vazio e mapa vazio. Todos os valores
restantes são verdadeiros; em particular, `"0"` é verdadeiro porque é uma string
não vazia. `empty` usa essa mesma interpretação após verificar a existência do valor
.

## Declarações e escopos

| Forma | Efeito |
|---|---|
| `$x = valor` | A primeira atribuição declara/infere; então reatribuir. |
| `var $x = valor` | Declaração com inferência fixa. |
| `T $x = valor` , `let T $x = valor` | Tipo explícito. |
| `let $x = valor` , `mixed $x = valor` | Vinculação dinâmica explícita. |
| `const [T] $x = valor` | Constante de ligação; requer inicializador. |
| `int $a = 1, $b = 2` | Declaração múltipla do mesmo tipo. |
| `public/private func f(T $p): R { ... }` | função global; retorno opcional, parâmetros digitados. |
| `func(T $p): R { ... }` | Fechamento sem modificador de visibilidade. |
| `public/private class C [extends B] [implements I1, I2] { ... }` | Aula; Superclasse opcional e interfaces implementadas. |
| `public/private interface I [extends I1, I2] { ... }` | Interface; Contratos de método público sem corpo. |
| `Init nombre(...) { ... }` | Inicializador sem modificador. |

Funções/classes globais são registradas antes da análise dos corpos.
Variáveis ​​de nível superior não são globais implícitas de funções com
nome. Cada chamada utiliza seu próprio quadro; Um fechamento captura um ambiente.
A visibilidade dos métodos e propriedades é obrigatória. Não há sobrecargas de assinatura
ou sintaxe de parâmetro de tipo geral.

## Blocos, coleções e instruções

`[]` cria um array, `{}` cria um mapa vazio no contexto de expressão e
`{"clave": valor}` cria um mapa não vazio. Uma chave em um local que requer um corpo
(função/ciclo) delimita um bloco. Blocos como expressões são
representados internamente como ASTs e não são executados automaticamente em todos os contextos.

- Ternário: escolha um valor ou execute o bloco selecionado; `return` pode
sair do chamável desse bloco.
- `while (condición) { ... }` : verifique antes de cada volta.
- `do { ... } while (condición)` : verifique mais tarde.
- `foreach (array_o_canal as $valor)` ou `foreach ($coleccion as $clave => $valor)` :
percorre sequências, intervalos `1..$n` , mapas associativos e canais de simultaneidade.
- `break` e `continue` : sair/pular volta.
- `defer { ... }` ou `defer expresión;` : adia a execução da instrução até
o quadro atual terminar (na ordem LIFO).
- `[$a, $b] = $expr` : desestruturação sequencial de arrays em atribuição direta.
- `match (valor) { clave, clave => resultado, default => resultado }` : compare
estritamente, primeiro braço correspondente; nenhuma correspondência/padrão retorna nulo.
- `try { ... } catch ($error) { ... }` , `throw expresión` : recuperação em tempo de execução.
- `return [expresión]` : sai do chamável.
- `async { ... }` : cria um Futuro; é coletado com `await(futuro)` .
- `Console::*` : módulo nativo para cores ANSI ( `Console::green` , `Console::red` , `Console::bold` , etc.).
- `joss repl` : ambiente interativo por terminal (Read-Eval-Print Loop).

## Ausências e compatibilidade

Não há importações de origem, namespaces, exportações de arquivos, interfaces, características, protocolos
, propriedade, ponteiros manuais, `switch` clássico, `for` ou sintaxe de açúcar de funções de seta.
`ref` serve apenas como parâmetro/argumento temporário e não é um ponteiro armazenável.

`function` , `import` , `@import` , `use` , `Use` , `Import` ,
`namespace` e `Namespace` geram erros de sintaxe excluídos.
Os nomes herdados de APIs não são necessariamente tipos de origem válidos:
`is_integer` ainda está registrado, `integer $x` não é um alias de `int` .


[Índice](README.md)

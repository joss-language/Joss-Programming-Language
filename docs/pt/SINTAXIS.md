# Referência de sintaxe e operadores

Para aprender: [fundamentos](FUNDAMENTOS.md), [fluxo](CONTROL_FLUJO.md),
[coleções](COLECCIONES.md), [funções](FUNCIONES.md), [classes](CLASES.md).
Complementos: [gramática](GRAMATICA.md), [tipos](SISTEMA_TIPOS.md).

Esta referência descreve o parser e o avaliador atuais do Joss. As
limitações observadas são indicadas como tais; não são propostas de design.

## Léxico

As palavras reservadas são obtidas de `parser.KeywordNames()` (`pkg/parser/token.go`):

```text
Init abstract as async await break case catch class const continue default defer do echo empty
enum extends false foreach func guard implements instanceof interface is isset let match new nil null print private
protected public record ref return select static this throw true try while yield
```

`var`, `int`, `float`, `decimal`, `string`, `bool`, `array`, `map`, `object`, `channel`, `mixed`, `void` e `make_chan` são identificadores de tipo ou funções integradas interpretados por contexto, não keywords fechadas do lexer.
As constantes internas `IF` e `ELSE` não significam que essas construções estejam disponíveis: `if`, `else`, `elif`, `for` e `switch` não são estruturas fonte do Joss.

| Forma | Regra |
|---|---|
| Identificador | ASCII: letras, `_` ou `@` no início; também dígitos depois. Evite `@` fora de APIs concretas; não é um sistema de anotações. |
| Variável | `$nombre`; o token `$` se separa do nome. `$this` tem tratamento próprio. |
| Comentários | `//` e `#` até o fim da linha; `/* ... */` sem aninhamento. |
| Strings | Aspas simples ou duplas; escapes `\n`, `\t`, `\r`, `\'`, `\"`, `\\`; interpolação estilo Dart/Flutter em aspas duplas: `"${var}"` ou `"${$expr}"` (escape `\${`); aspas simples sem interpolação. |
| Inteiros | Sequência de dígitos; o parser usa base automática: `010` é lido como octal, `08` falha. Evite zeros à esquerda. |
| Float | Dígitos, ponto e mais dígitos: `0.5`. Não há literal exponencial, hexadecimal nem separador `_` no lexer. |
| Decimal | Inteiro ou fração com sufixo `m`/`M`: `100m`, `1.25M`. |
| Ausência | `null` é o literal canônico (`nil` produz o mesmo valor, mas emite o aviso de transição `JOSS-DECL-010`). |
| Separação | Nova linha ou `;`. Os espaços e tabs não delimitam blocos. |

O lexer remove o BOM UTF-8 inicial. Fora de strings, omite bytes não ASCII:
não confie em identificadores acentuados.

Além de nomes, literais e keywords, são tokenizados:

```text
= += -= *= /= ??= + - ! * / % < > == != === !== <=> <= >= << >> && || ++ --
, ; : ? ( ) { } [ ] . .. ... -> ?-> :: | |> ?? =>
NEWLINE EOF ILLEGAL
```

Não há exponenciação, AND binário `&` nem OR binário `|`;
este último separa tipos de uma união (`int|string`).

## Precedência: da menor para a maior

A tabela reproduz `pkg/parser/parser.go`. No mesmo nível, os infixos
ordinários agrupam à esquerda; a atribuição analisa toda a sua direita.

| Nível | Operadores / construções |
|---|---|
| 1 | `=`, `+=`, `-=`, `*=`, `/=`, `??=` (direita) |
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
| 13 | Prefixos `-`, `!`, `await`, `ref`, `...` |
| 14 | Chamada `()` |
| 15 | Índice `[]`, membros `->`, `?->`, `::`, pós-fixos `++`, `--` |

Os operadores aritméticos e lógicos seguem sua hierarquia convencional: `%`,
`*` e `/` compartilham o mesmo nível, e `&&` tem precedência maior que `||`.
Os braços do ternário são analisados como expressões completas; coloque parênteses
em ternários aninhados em vez de transferir a associatividade de outra linguagem.

<!-- joss-run: ["1", "true", "true"] -->
```joss
print(8 * 5 % 3)
print(true || false && false)
print(true || (false && false))
```

Resulta em `1`, `true` e `true`. A primeira expressão é `(8 * 5) % 3`.

## Avaliação

- `+`, `-`, `*`: inteiros exatos com verificação de overflow; operações entre `int` e `decimal` promovem para `decimal`; misturar `int` com `float` ou `float` com `decimal` requer conversão explícita (`JOSS-ARITH-003`).
- `/`: resultado `float` para inteiros; `decimal` se envolver `decimal`.
- `+=`, `-=`, `*=`, `/=`, `??=`: atribuição composta e atribuição de coalescência
  nula (`$a ??= $b` equivale a `$a = $a ?? $b`) com validação
  estrita de tipos.
- `%`: resto inteiro; com float trunca operandos para inteiro; decimal usa `Mod`.
- `.`: representa operandos como texto e concatena; `null` contribui com texto vazio.
- `..`: operador de intervalo numérico (`$a..$b`); gera uma sequência inteira entre
  ambos os limites, útil em expressões e laços `foreach (1..10 as $i)`.
- `"${...}"`: interpolação de strings estilo Flutter / Dart dentro de aspas
  duplas. Permite embutir variáveis simples (`"${indice}"`) ou expressões (`"${$a + $b}"`),
  convertendo-se automaticamente em concatenações do tipo string. O escape `\${`
  imprime `${` de forma literal.
- `++`, `--`: incrementa ou decrementa a variável numérica e retorna o valor anterior.
- `&&`, `||`: curto-circuito e resultado bool. `!`: negação de truthiness.
- `==`/`!=`: comparação numérica quando aplicável; fallback mediante
  representação textual para outros valores. Para coleções prefira
  `===`/`!==`, baseados em comparação estrutural.
- `===`: distingue inteiro, float, string e decimal; normaliza variantes
  numéricas internas do Go antes de comparar.
- `<=>`: retorna `-1`, `0` ou `1`; compara números, strings, nulos e
  finalmente representações textuais.
- `??`: avalia a direita somente se a esquerda produzir `null`. Os erros ao
  avaliar a esquerda se propagam e devem ser tratados com `try`/`catch`.
- `?:` (Elvis): conserva a esquerda se for verdadeira segundo truthiness.
- Ternário completo e de ramo único: `cond ? expr : expr` e `(cond) ? { cuerpo }`
  (permite omitir o ramo `: {}` quando não é necessária alternativa falsa).
- `guard (cond) : { cuerpo }`: garante que `cond` seja verdadeira; se for falsa executa o bloco, o qual deve encerrar o fluxo com `return` ou `throw` (`JOSS-FLOW-005`).
- `match`: seleção múltipla por valor estrita (`===`); admite expressões ou blocos `{ ... }`.
- `?->`: retorna nulo diante de receptor nulo; não valida nem corrige outros acessos.
- `|>`: antepõe o valor esquerdo aos argumentos de uma função, chamada
  ou closure.
- `cout << valor`: imprime sem adicionar quebra de linha e retorna `cout`.
  `canal << valor`: envia ao canal. `cin >> $variable`: lê de forma adaptativa
  da entrada padrão (convertendo automaticamente para número se a variável
  ou entrada for numérica e lendo linhas completas com espaços se for texto).

## Valor verdade (Truthiness)

O Joss conserva um modelo de truthiness pequeno e uniforme. São falsos: `null`, `false`,
zero de qualquer tipo numérico, string vazia, array vazio e map vazio. Todo
valor restante é verdadeiro; em particular, `"0"` é verdadeiro porque é uma
string não vazia. `empty` usa esta mesma interpretação depois de verificar a
existência do valor.

## Declarações e escopos

| Forma | Classificação e efeito |
|---|---|
| `var $x = valor` | **Canônica**: Declaração com inferência estrita e fixa. |
| `T $x = valor` | **Canônica**: Declaração com tipo explícito (`int`, `string`, `Persona`, etc.). |
| `mixed $x = valor` | **Canônica**: Dinamismo explícito voluntário; permite mudar de tipo em reatribuições. |
| `const [T] $x = valor` | **Canônica**: Binding constante imutável; requer inicializador. |
| `int $a = 1, $b = 2` | **Canônica**: Declaração múltipla do mesmo tipo. |
| `$x = valor` (sem declaração prévia) | **Reatribuição** se `$x` já existir; se não foi declarada previamente, emite erro `JOSS-SYM-001` (e aviso histórico `JOSS-DECL-007`). |
| `let [T] $x = valor` | **Transição**: Suportada por compatibilidade, emite aviso `JOSS-DECL-006`. Use `var`, `T` ou `mixed`. |
| `public/private func f(T $p): R { ... }` | **Canônica**: Função global; parâmetros tipados e retorno `: R` ou `: void`. |
| `func(T $p): R { ... }` | **Canônica**: Closure sem modificador de visibilidade. |
| `public/private [abstract] class C [<T>] [extends B] [implements I1, I2] { ... }` | **Canônica**: Classe nominal (com suporte para genéricos `<T>`, herança e interfaces). |
| `public/private record R(T $campo, ...)` | **Canônica**: Classe de dados imutável com campos `public const` e construtor automático. |
| `public/private interface I [extends I1, I2] { ... }` | **Canônica**: Interface de contratos de métodos públicos sem corpo. |
| `public/private enum E [: int\|string] { case ... }` | **Canônica**: Enumeração pura ou apoiada (`int`/`string`). |
| `public func constructor(...) { ... }` ou `Init(...) { ... }` | **Canônica**: Inicializador de instância (`Init constructor(...)` é forma de transição `JOSS-DECL-008`). |

As funções e classes globais são registradas antes de analisar corpos.
As variáveis de nível superior não são globais implícitas de funções com
nome. Cada chamada usa seu próprio frame; uma closure captura seu ambiente léxico.
A visibilidade de métodos e propriedades é obrigatória. Não há sobrecarga
por assinatura.

## Blocos, coleções e instruções

`[]` cria um array, `{}` cria um map vazio em contexto de expressão e
`{"clave": valor}` cria um map não vazio. Uma chave em um lugar que exige um
corpo (função/laço) delimita um bloco.

- Ternário: escolhe um valor ou executa o bloco selecionado; `return` pode
  sair do callable a partir desse bloco.
- `guard (condición) : { ... }`: cláusula de guarda com saída obrigatória (`return` ou `throw`).
- `while (condición) { ... }`: verifica antes de cada iteração.
- `do { ... } while (condición)`: verifica depois.
- `foreach (array_o_canal as $valor)` ou `foreach ($coleccion as $clave => $valor)`:
  percorre sequências, intervalos `1..$n`, mapas associativos, geradores e canais de concorrência.
- `break` e `continue`: sair/pular iteração.
- `defer { ... }` ou `defer expresión`: adia a execução até que
  o frame atual finalize (em ordem LIFO).
- `[$a, $b] = $expr`: desestruturação sequencial de arrays.
- `match (valor) { clave, clave => resultado, default => resultado }`: compara
  estritamente, primeiro braço correspondente; sem correspondência/default retorna nulo.
- `try { ... } catch ($error) { ... }`, `throw expresión`: recuperação em runtime.
- `return [expresión]`: sai do callable.
- `async { ... }`: cria um `Future`; é recolhido com `await($futuro)` ou `await $futuro`.
- `select { case ... default: ... }`: multiplexação sobre canais.
- `yield [clave =>] valor`: emissão preguiçosa em funções geradoras.
- `Console::*`: módulo nativo para cores ANSI (`Console::green`, `Console::red`, `Console::bold`, etc.).
- `joss repl`: ambiente interativo de terminal (Read-Eval-Print Loop).

## Ausências e compatibilidade

Não há imports fonte, namespaces, exports de arquivos, traits,
protocolos, ownership, ponteiros manuais, `switch`, `for` clássico nem syntax sugar de arrow functions.
`ref` serve apenas como parâmetro/argumento temporário e não é um ponteiro armazenável.

`function`, `import`, `@import`, `use`, `Use`, `Import`,
`namespace` e `Namespace` geram erros de sintaxe removida (`JOSS-PARSE-001`).
Os nomes herdados de APIs não são necessariamente tipos fonte válidos:
`is_integer` continua registrado como função built-in, mas `integer $x` não é alias de `int` (`JOSS-TYPE-009`).

[Índice](README.md)

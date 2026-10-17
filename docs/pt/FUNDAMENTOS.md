# Valores, variáveis e operações fundamentais

Antes: [Primeiros passos](PRIMEROS_PASOS.md). Depois: [Controle de fluxo e decisões](CONTROL_FLUJO.md).
Referência técnica: [Sistema de tipos](SISTEMA_TIPOS.md), [Sintaxe e operadores](SINTAXIS.md).

---

## O que você vai aprender aqui?

Todo programa de computador existe para processar informações: calcular o total de uma fatura, guardar o nome de um usuário, verificar se uma senha está correta ou contar quantas mensagens pendentes você tem.

Neste guia você aprenderá do zero:
1. O que é um **valor** e o que é um **tipo de dado**.
2. Os tipos primitivos essenciais do Joss: inteiros, decimais aproximados, decimais exatos, texto e booleanos.
3. O que é uma **variável**, como ela funciona na memória e a diferença entre **declará-la** e **reatribuí-la**.
4. As quatro formas canônicas de declarar variáveis no Joss: inferida (`var $x = ...`), explícita (`int $x = ...`), dinâmica (`mixed $x = ...`) e constante (`const`).
5. Como operar números, unir ou interpolar textos (`"${...}"`) e formatar a saída na tela.
6. Quais erros comuns são cometidos e como preveni-los.

---

## 1. Valores e tipos de dados: Que informações manipulamos?

Um **dado** ou **valor** é qualquer informação que um programa utiliza. Por exemplo:
- `42` é um número inteiro.
- `"Ada Lovelace"` é um texto.
- `true` (verdadeiro) é uma resposta lógica.

Em um computador, nem todos os dados são armazenados ou manipulados da mesma forma. Somar duas quantidades numéricas (`10 + 5 = 15`) é uma operação matemática; em compensação, unir dois nomes (`"Ana" . " Gómez"`) é uma operação de texto.

O **tipo de dado** define:
1. Que tipo de informação o valor representa.
2. Quais operações são permitidas sobre ele.
3. Quanta memória requer e como é armazenado internamente.

### Os tipos canônicos fundamentais do Joss

| Tipo de dado | O que representa | Exemplos | Quando utilizá-lo |
|---|---|---|---|
| `int` | Número inteiro (sem parte decimal) | `0`, `42`, `-15`, `1000` | Para contadores, idades, identificadores numéricos e quantidades indivisíveis. |
| `float` | Número com ponto decimal (aproximação binária) | `3.1416`, `0.5`, `-12.8` | Para cálculos científicos, coordenadas, gráficos ou medições onde pequenas aproximações são aceitáveis. |
| `decimal` | Número decimal de alta precisão na base dez | `0.10m`, `19.99m`, `100.00M` | **Essencial em finanças, preços, impostos e balanços**, onde perder um único centavo por aproximação binária é inaceitável. Leva o sufixo `m` ou `M`. |
| `string` | Sequência de caracteres (texto) | `"Hola"`, `'Joss'`, `"admin@ejemplo.com"` | Para nomes, e-mails, descrições, conteúdo HTML e mensagens. |
| `bool` | Valor lógico de verdade | `true` (verdadeiro), `false` (falso) | Para tomar decisões: está autenticado?, há estoque?, é maior de idade? |
| `array` | Lista ordenada de valores | `[1, 2, 3]`, `["pan", "leche"]` | Para sequências de elementos (você aprenderá em [Coleções](COLECCIONES.md)). |
| `map` | Dicionário associativo (chave → valor) | `{"id": 1, "nombre": "Ada"}` | Para registros de dados com etiquetas (você aprenderá em [Coleções](COLECCIONES.md)). |
| `null` | Ausência de valor | `null` (ou `nil` por compatibilidade) | Para indicar que um dado ainda não existe, está vazio ou não foi encontrado. |

---

## 2. Variáveis: Caixas com nome na memória

Um computador tem milhões de células de memória. Se guardarmos um número e não lhe dermos um nome, não teremos como encontrá-lo um milissegundo depois.

Uma **variável** é simplesmente um nome humano que atribuímos a um espaço da memória para armazenar um dado, lê-lo quando precisarmos ou trocá-lo por outro valor.

### A regra do `$` no Joss

No Joss, **todos os nomes de variáveis começam obrigatoriamente com o cifrão (`$`)**:
- `$edad`
- `$nombre_completo`
- `$totalPagar`

> [!TIP]
> **Por que o Joss usa `$` para as variáveis?**
> O prefixo `$` permite que tanto você quanto o analisador e o compilador distingam instantaneamente uma variável de uma palavra-chave da linguagem (`return`, `func`, `class`), de um tipo (`int`, `string`) ou de uma função integrada (`print`). Além disso, o Joss diferencia maiúsculas de minúsculas: `$edad` e `$Edad` são duas variáveis distintas.

Vejamos um exemplo mínimo:

<!-- joss-run: ["21"] -->
```joss
var $edad = 20
$edad = $edad + 1
print($edad)
```

Saída:

```text
21
```

### O que acontece passo a passo neste programa?

1. `var $edad = 20` (**Declaração**):
   - É a **primeira vez** que aparece `$edad`, por isso usamos `var` para criá-la.
   - O sinal `=` é o **operador de atribuição**: pega o `20` da direita e o guarda em `$edad`.
   - O Joss **infere** (deduz) automaticamente que `$edad` é do tipo `int`.
2. `$edad = $edad + 1` (**Reatribuição**):
   - Como `$edad` **já existe**, não voltamos a escrever `var`.
   - O computador calcula primeiro o lado direito (`20 + 1` = `21`) e guarda o novo resultado `21` dentro de `$edad`.
3. `print($edad)`:
   - Lê o valor atual de `$edad` (`21`) e o mostra no terminal.

---

## 3. As quatro formas de declarar uma variável

No Joss você tem controle total sobre quão estrito ou flexível quer que seja o tipo das suas variáveis:

### 1. Inferência canônica fixa: `var $x = valor`
É a forma mais cômoda e recomendada no dia a dia. O Joss deduz o tipo a partir do valor inicial e, a partir desse momento, protege a variável para que não mude de tipo por acidente:

```joss
var $contador = 0       // Infiere int
var $titulo = "Reporte" // Infiere string
var $peso = 72.5        // Infiere float
```

Se mais adiante você tentar guardar um texto dentro de uma variável que nasceu como inteiro, o analisador semântico deterá o programa antes de executar com o código `JOSS-TYPE-001`:

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
var $cantidad = 2
$cantidad = "muchas"
```

### 2. Declaração com tipo explícito: `int $x = valor`
Quando você quer que o tipo do dado seja 100% visível ao ler o código (e de forma obrigatória nos parâmetros das funções que veremos mais adiante):

```joss
int $puerto = 8080
string $usuario = "admin"
decimal $saldo = 1500.50m
bool $activo = true
```

### 3. Variável dinâmica explícita: `mixed $x = valor`
Às vezes você precisa legitimamente que uma mesma variável guarde primeiro um número e depois um texto. Para permitir mudanças de tipo sem erro, indique isso com `mixed`:

<!-- joss-run: ["pendiente"] -->
```joss
mixed $resultado = 2
$resultado = "pendiente"
print($resultado)
```

Saída:

```text
pendiente
```

> [!NOTE]
> **Nota de compatibilidade (`let`)**:
> Em código antigo do Joss você pode encontrar `let $x = 2` (equivalente a `mixed $x = 2`) ou `let int $x = 2` (equivalente a `int $x = 2`). A linguagem continua aceitando `let` por compatibilidade e a ferramenta `joss fix` o atualiza automaticamente para as formas canônicas modernas.

### 4. Constantes imutáveis: `const`
Uma **constante** é um valor que é definido uma única vez e **jamais pode mudar** enquanto o programa roda. É ideal para taxas de impostos, limites máximos ou nomes de configuração:

<!-- joss-run: ["3"] -->
```joss
const int $maximo = 3
print($maximo)
```

Saída:

```text
3
```

Se você tentar escrever `$maximo = 4`, o analisador emitirá o erro `JOSS-SYM-006` impedindo a modificação.

---

## 4. Texto, comentários e interpolação

### Delimitadores de texto e caracteres de escape

No Joss você pode escrever cadeias de texto usando aspas duplas (`"..."`) ou aspas simples (`'...'`):

<!-- joss-run: ["Hola, Ada", "Primera línea", "Segunda línea"] -->
```joss
// Este comentario explica el código; no se ejecuta.
var $nombre = 'Ada'
print("Hola, " . $nombre)
/* Un comentario también puede
   ocupar varias líneas. */
print("Primera línea\nSegunda línea")
```

Saída:

```text
Hola, Ada
Primera línea
Segunda línea
```

- **Comentários de uma linha**: Começam com `//` (ou `#`). Tudo o que você escrever à direita deles é ignorado pelo computador.
- **Comentários de várias linhas**: Começam com `/*` e terminam com `*/`.
- **Caracteres de escape** (em aspas duplas ou simples):
  - `\n`: Insere uma quebra de linha.
  - `\t`: Insere uma tabulação horizontal.
  - `\"` ou `\'`: Permite incluir aspas literais dentro do texto.
  - `\\`: Insere uma barra invertida.

### Concatenação com o operador ponto (`.`)

Em outras linguagens usa-se `+` tanto para somar quanto para unir textos, o que provoca erros silenciosos quando se misturam números e textos. No Joss a regra é inequívoca:
- O sinal `+` é reservado **exclusivamente para a soma matemática**.
- O ponto `.` é utilizado **exclusivamente para unir texto**.

<!-- joss-run: ["1020"] -->
```joss
var $a = "10"
var $b = "20"
print($a . $b)
```

Saída:

```text
1020
```

### Interpolação de strings: `${nombre}` e `${$edad * 2}`

Em vez de unir vários fragmentos com pontos (`"Hola " . $nombre . ", tienes " . $edad . " años"`), dentro de textos com **aspas duplas** você pode incorporar variáveis ou cálculos diretamente usando `${...}`:

<!-- joss-run: ["Hola Ada, tienes 21 años", "El doble es 42"] -->
```joss
var $nombre = "Ada"
var $edad = 21
print("Hola ${nombre}, tienes ${edad} años")
print("El doble es ${$edad * 2}")
```

Saída:

```text
Hola Ada, tienes 21 años
El doble es 42
```

- **Quando você incorpora uma única variável**: Pode escrever `${nombre}` (ou `${$nombre}`). O Joss reconhece o nome da sua variável automaticamente.
- **Quando você incorpora uma operação ou cálculo**: Escreva a expressão completa com o sinal `$` em cada variável, por exemplo `${$edad * 2}` ou `${$precio + $impuesto}`.
- **Escape literal**: Se precisar mostrar literalmente os caracteres `${` na tela, escreva `\${`.

### Formatação com modelos posicionais (`printf`)

Quando quiser formatar textos indicando exatamente onde vai cada texto (`%s`) ou número inteiro (`%d`), você também dispõe de `printf`:

```joss
var $item = "Teclado"
var $cantidad = 2
printf("Producto: %s | Cantidad: %d\n", $item, $cantidad)
```

---

## 5. Operações numéricas e matemáticas

<!-- joss-run: ["5", "2.5", "1", "0.3"] -->
```joss
print(2 + 3)
print(5 / 2)
print(5 % 2)
print(0.10m + 0.20m)
```

Saída:

```text
5
2.5
1
0.3
```

### Operadores aritméticos

| Operador | Operação | Exemplo | Resultado | Explicação |
|---|---|---|---|---|
| `+` | Soma | `10 + 5` | `15` | Adição matemática. |
| `-` | Subtração | `10 - 4` | `6` | Subtração. |
| `*` | Multiplicação | `3 * 4` | `12` | Produto. |
| `/` | Divisão | `5 / 2` | `2.5` | Dividir inteiros **retorna `float`** para não perder a parte fracionária (`2.5`). |
| `%` | Módulo (resto) | `5 % 2` | `1` | O que sobra de uma divisão inteira (5 dividido por 2 dá 2 e sobra `1`). Muito útil para saber se um número é par (`$n % 2 == 0`). |
| `++` | Pós-incremento | `$i++` | Valor anterior | Soma `1` à variável. |
| `--` | Pós-decremento | `$i--` | Valor anterior | Subtrai `1` da variável. |

### Operadores de atribuição rápida (`+=`, `-=`, `*=`, `/=`)

Quando quiser somar, subtrair ou multiplicar sobre o valor que uma variável já tem (como somar pontos em um placar), você pode abreviar `$puntos = $puntos + 5` escrevendo `$puntos += 5`:

<!-- joss-run: ["15", "12", "24"] -->
```joss
var $puntos = 10
$puntos += 5
print($puntos)

$puntos -= 3
print($puntos)

$puntos *= 2
print($puntos)
```

Saída:

```text
15
12
24
```

| Operador | Equivale a escrever | O que faz |
|---|---|---|
| `$x += $y` | `$x = $x + $y` | Soma `$y` ao valor atual de `$x`. |
| `$x -= $y` | `$x = $x - $y` | Subtrai `$y` do valor atual de `$x`. |
| `$x *= $y` | `$x = $x * $y` | Multiplica o valor atual de `$x` por `$y`. |
| `$x /= $y` | `$x = $x / $y` | Divide o valor atual de `$x` por `$y`. |

### Ordem de prioridade e uso de parênteses
Assim como na matemática da escola, a multiplicação `*` e a divisão `/` são calculadas antes da soma `+` e da subtração `-`. Use parênteses `(` e `)` quando quiser indicar qual operação deve ser feita primeiro:

<!-- joss-run: ["14", "20"] -->
```joss
print(2 + 3 * 4)
print((2 + 3) * 4)
```

Saída:

```text
14
20
```

### Proteção automática em cálculos numéricos

O Joss vigia suas operações matemáticas para evitar falhas silenciosas:
- **Divisão por zero (`JOSS-ARITH-002`)**: Se você tentar dividir por `0` (`10 / 0`), o Joss interrompe a operação com uma mensagem clara.
- **Estouro de inteiros (`JOSS-ARITH-001`)**: Se um cálculo com `int` superar a capacidade máxima de um número inteiro, o Joss avisa em vez de entregar um número negativo falso.
- **Proteção de precisão (`JOSS-ARITH-003`)**: O Joss permite combinar diretamente `int` com `decimal` porque nenhum dígito é perdido, mas pede que você use uma função de conversão explícita se tentar misturar `float` com `decimal` ou `int`.

---

## 6. A diferença fundamental para dinheiro: `float` vs `decimal`

Por que existem dois tipos para números com ponto (`float` e `decimal`)?

Os computadores internamente contam em binário (zeros e uns). Em binário, frações como `0.1` ou `0.2` têm infinitas casas decimais periódicas, de modo que o tipo padrão `float` guarda uma aproximação muito próxima:

```joss
print(0.1 + 0.2) // Muestra 0.30000000000000004
```

Para mover um personagem em um videogame ou medir temperatura, essa diferença minúscula não importa. Mas para **preços, faturas, salários ou contabilidade**, precisamos de exatidão total na base dez.

Por isso o Joss inclui de fábrica o tipo **`decimal`**, que é escrito adicionando um `m` (ou `M`) ao final do número:

<!-- joss-run: ["0.3", "23.2"] -->
```joss
print(0.10m + 0.20m)
var $precio = 11.60m
print($precio * 2)
```

Saída:

```text
0.3
23.2
```

> **Regra prática**: Use `int` para quantidades inteiras, `float` para medidas físicas ou científicas e **`decimal` (com `m`) sempre que representar dinheiro**.

---

## 7. Saída e entrada por fluxos (`cout <<` e `cin >>`)

Além de `print(...)`, o Joss oferece o fluxo de saída `cout <<` e o manipulador de quebra de linha `endl`:

<!-- joss-run: ["Hola mundo", "Linea 1", "Linea 2"] -->
```joss
// Con print: agrega salto de línea automático al final
print("Hola mundo")

// Con cout: encadena varias piezas con << y cierra la línea con endl
cout << "Linea " << 1 << endl
cout << "Linea " << 2 << endl
```

Saída:

```text
Hola mundo
Linea 1
Linea 2
```

- `print(...)`: Ideal para mostrar uma linha completa de forma direta.
- `cout << ...`: Muito útil quando você quer mostrar uma mensagem sem pular de linha (por exemplo, antes de pedir um dado com `cin >>`) ou encadear vários valores seguidos.
- `cerr << ...`: Envia mensagens ao canal de erros do terminal (`stderr`).

---

## 8. Converter um dado de um tipo para outro (Casting)

Se você ler um dado que vem como texto (por exemplo `"42"` ou `"19.99"`) e precisar fazer cálculos matemáticos com ele, utilize as funções integradas de conversão:

<!-- joss-run: ["52", "29.99"] -->
```joss
var $numero = intval("42") + 10
var $precio = decimal("19.99") + 10.00m
print($numero)
print($precio)
```

Saída:

```text
52
29.99
```

| Função | Converte o valor para | Exemplo | Resultado |
|---|---|---|---|
| `intval($v)` | `int` (inteiro) | `intval("42")` | `42` |
| `floatval($v)` | `float` (decimal aproximado) | `floatval("3.14")` | `3.14` |
| `decimal($v)` | `decimal` (decimal exato) | `decimal("19.99")` | `19.99m` |
| `strval($v)` | `string` (texto) | `strval(100)` | `"100"` |
| `boolval($v)` | `bool` (`true` ou `false`) | `boolval(1)` | `true` |

> [!WARNING]
> Se você converter um texto que não contém números com `intval("manzana")`, o resultado será `0`. Quando quiser verificar antes se um texto representa um número válido, use `is_numeric($texto)`.

---

## 9. Erros comuns ao trabalhar com variáveis e tipos

| Erro comum | Por que ocorre | Como solucionar |
|---|---|---|
| Usar `+` para unir textos (`"Total: " + $precio`) | No Joss `+` é exclusivo para somar números (`JOSS-TYPE-004`). | Use o ponto `.` (`"Total: " . $precio`) ou interpolação (`"Total: ${precio}"`). |
| Esquecer `var` ao criar uma variável (`$edad = 20`) | Criar uma variável sem declará-la emite `JOSS-SYM-001`. | Escreva `var $edad = 20` (ou `int $edad = 20`) na primeira vez que a criar. |
| Mudar o tipo de uma variável criada com `var` | `var $x = 10` fixa o tipo em `int` (`JOSS-TYPE-001`). | Se precisar que ela mude de tipo mais adiante, declare-a com `mixed $x = 10`. |
| Misturar `int` e `float` sem converter (`10 + 2.5`) | O Joss evita conversões implícitas com possível perda de precisão (`JOSS-ARITH-003`). | Use ambos do mesmo tipo (`10.0 + 2.5`), converta com `floatval(10) + 2.5`, ou use `decimal` (`10 + 2.5m`). |
| Tentar mudar uma constante (`const $A = 1; $A = 2`) | As constantes são imutáveis (`JOSS-SYM-006`). | Se o valor deve mudar durante o programa, declare-o com `var` em vez de `const`. |

---

## 10. Mini-projeto guiado: Calculadora Financeira de Conta e Gorjetas

Vamos colocar em prática variáveis, o tipo exato `decimal`, operações matemáticas e interpolação de texto em um programa completo que você pode colocar dentro do seu `main.joss`:

<!-- joss-run: ["Subtotal: 50", "Propina (15%): 7.5", "Total a pagar: 57.5", "Cada persona paga: 28.75"] -->
```joss
// 1. Datos iniciales con precisión monetaria decimal (sufijo m)
decimal $subtotal = 50.00m
decimal $tasaPropina = 0.15m
int $personas = 2

// 2. Cálculos matemáticos
decimal $propina = $subtotal * $tasaPropina
decimal $total = $subtotal + $propina
decimal $porPersona = $total / $personas

// 3. Mostrar resumen en pantalla
print("Subtotal: ${subtotal}")
print("Propina (15%): ${propina}")
print("Total a pagar: ${total}")
print("Cada persona paga: ${porPersona}")
```

Saída:

```text
Subtotal: 50
Propina (15%): 7.5
Total a pagar: 57.5
Cada persona paga: 28.75
```

---

## 11. Prática do capítulo

### 1. Teste rápido
No mini-projeto da seção 10:
1. Mude `$subtotal` para `120.00m` e `$personas` para `4`.
2. Execute `joss run main.joss` e verifique quanto cada pessoa deve pagar.

### 2. Exercício guiado: Conversor de Temperatura (°C para °F)
1. Declare uma variável com os graus Celsius: `float $celsius = 25.0`.
2. Calcule seu equivalente em Fahrenheit usando parênteses: `float $fahrenheit = ($celsius * 9.0 / 5.0) + 32.0`.
3. Mostre o resultado usando interpolação: `print("${celsius} °C equivalen a ${fahrenheit} °F")` (deverá mostrar `25 °C equivalen a 77 °F`).

### 3. Desafio: Fatura com desconto e imposto
Crie um programa que:
1. Declare uma constante `const decimal $IVA = 0.16m`.
2. Declare o preço de um produto `decimal $precio = 200.00m` e aplique primeiro um desconto de `20.00m` usando o operador `-=`.
3. Calcule o imposto (`$precio * $IVA`) e o total final (`$precio + $impuesto`).
4. Mostre na tela o preço com desconto, o imposto e o total final.

---

## Próximo passo

Agora que você domina os valores, os tipos de dados e as operações na memória, é hora de ensinar seus programas a **tomar decisões** e a **repetir tarefas automaticamente**.

Continue com: [Controle de fluxo: decisões e estruturas de repetição](CONTROL_FLUJO.md).

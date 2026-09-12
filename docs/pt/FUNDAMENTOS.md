# Valores fundamentais, variáveis ​​e operações

Antes: [Primeiros passos](PRIMEROS_PASOS.md). Depois: [Controle de fluxo e decisões](CONTROL_FLUJO.md).
Referência técnica: [Sistema de tipo](SISTEMA_TIPOS.md), [Sintaxe e operadores](SINTAXIS.md).

---

## O que você vai aprender aqui?

Todo programa de computador existe para processar informações: calcular o total de uma fatura, salvar o nome de um usuário, verificar se uma senha está correta ou contar quantas mensagens você tem pendentes.

Neste guia você aprenderá do zero:
1. O que é um **valor** e o que é um **tipo de dados**.
2. Os tipos primitivos essenciais de Joss: inteiros, decimais aproximados, decimais exatos, texto e booleanos.
3. O que é uma **variável**, como ela funciona na memória e como declará-la.
4. As quatro maneiras de declarar variáveis em Joss: inferida (`$x = ...`), explícita (`int $x = ...`), dinâmica (`mixed $x = ...`) e constante (`const`).
5. Como operar números, juntar textos e formatar a saída da tela.
6. Quais erros comuns são cometidos e como evitá-los.

---

## 1. Valores e tipos de dados: quais informações tratamos?

Um **dado** ou **valor** é qualquer informação que um programa usa. Por exemplo:
- `42` é um número.
- `"Ada Lovelace"` é um texto.
- `true` (true) é uma resposta lógica.

Em um computador, nem todos os dados são armazenados ou manipulados da mesma maneira. Adicionar duas quantidades numéricas (`10 + 5 = 15`) é uma operação matemática; por outro lado, juntar dois nomes (`"Ana" . " Gómez"`) é uma operação de texto.

O **tipo de dados** define:
1. Que tipo de informação o valor representa.
2. Quais operações são permitidas nele.
3. Quanta memória requer e como é armazenada internamente.

### Tipos canônicos fundamentais de Joss

| Tipo de dados | O que isso representa | Exemplos | Quando usar |
|---|---|---|---|
| `int` | Número inteiro (sem parte decimal) | `0`, `42`, `-15`, `1000` | Para contadores, idades, identificadores numéricos e quantidades indivisíveis. |
| `float` | Número com vírgula decimal (aproximação binária) | `3.1416`, `0.5`, `-12.8` | Para cálculos científicos, coordenadas, gráficos ou medições onde pequenas aproximações são aceitáveis. |
| `decimal` | Número decimal de alta precisão na base dez | `0.10m`, `19.99m`, `100.00M` | **Essencial em finanças, preços, impostos e balanços**, onde perder um único centavo devido à aproximação binária é inaceitável. Tem o sufixo `m` ou `M`. |
| `string` | Sequência de caracteres (texto) | `"Hola"`, `'Joss'`, `"admin@ejemplo.com"` | Para nomes, e-mails, descrições, conteúdo HTML e mensagens. |
| `bool` | Valor de verdade lógica | `true` (verdadeiro), `false` (falso) | Para tomar decisões: é autenticado? Existe estoque? É maior de idade? |
| `array` | Lista ordenada de valores | `[1, 2, 3]`, `["pan", "leche"]` | Para sequências de elementos (detalhadas em [Coleções](COLECCIONES.md)). |
| `map` | Dicionário associativo (chave → valor) | `{"id": 1, "nombre": "Ada"}` | Para registros de dados com propriedades (detalhadas em [Coleções](COLECCIONES.md)). |
| `null` / `nil` | Ausência absoluta de valor | `null` ou `nil` | Para indicar que um dado ainda não existe, está vazio ou não foi encontrado. |

---

## 2. Variáveis: caixas nomeadas na memória

Um computador possui milhões de células de memória. Se salvarmos um número e não lhe dermos um nome, não teremos como encontrá-lo um milissegundo depois.

Uma **variável** é simplesmente um nome humano que atribuímos a um espaço de memória para armazenar dados, lê-los quando precisamos ou alterá-los para outro valor.

### A regra `$` em Joss

Em Joss, **todos os nomes de variáveis começar obrigatoriamente com o cifrão (`$`)**:
- `$edad`
- `$nombre_completo`
- `$totalPagar`

> [!TIP]
> **Por que Joss usa `$` para variáveis?**
> O prefixo `$` permite que você, o analisador e o compilador distingam instantaneamente uma variável de uma palavra-chave de linguagem (`return`, `func`, `class`), de tipo (`int`, `string`) ou de função nativa (`print`). Além disso, Joss diferencia maiúsculas de minúsculas: `$edad` e `$Edad` são duas variáveis ​​diferentes.

Vejamos um exemplo mínimo:

<!-- joss-run: ["21"] -->
```joss
$edad = 20
$edad = $edad + 1
print($edad)
```

### O que acontece passo a passo neste programa?

1. `$edad = 20`:
   - O sinal `=` é o **operador de atribuição**. Ele avalia o que está à sua direita (`20`) e deposita na variável `$edad`.
   - Como esta é a primeira vez que `$edad` aparece no programa, Joss automaticamente **infere** que `$edad` é do tipo `int`.
2. `$edad = $edad + 1`:
   - O computador avalia primeiro o lado direito: procura o valor atual de `$edad` (que é `20`), adiciona `1`, resultando em `21`.
   - Em seguida, o operador `=` salva esse novo valor `21` em `$edad`, substituindo o `20` acima.
3. `print($edad)`:
   - Leia o valor atual de `$edad` e exibe-o no console.

---

## 3. As quatro maneiras de declarar uma variável

No Joss você tem controle total sobre o quão estrita ou flexível você deseja que a digitação de suas variáveis seja:

### 1. Inferência automática corrigida: `$x = valor` (ou `var $x = valor`)
É a forma mais rápida e recomendada para o dia a dia. Joss deduz o tipo na primeira atribuição e a partir desse momento a variável fica protegida:

```joss
$contador = 0       // Infiere int
$titulo = "Reporte" // Infiere string
var $peso = 72.5    // 'var' solicita inferencia explícita; también fija float
```

Se mais tarde você tentar colocar texto dentro de um número inteiro, o analisador semântico irá parar o programa com o código `JOSS-TYPE-001`:

<!-- joss-error: JOSS-TYPE-001 -->
```joss-invalid
$cantidad = 2
$cantidad = "muchas"
```

### 2. Declaração com tipo explícito: `int $x = valor`
Quando você deseja que o contrato de dados fique 100% visível para qualquer pessoa que esteja lendo o código, ou nos parâmetros da função:

```joss
int $puerto = 8080
string $usuario = "admin"
decimal $saldo = 1500.50m
bool $activo = true
```

### 3. Variável dinâmica: `mixed $x = valor` (ou `let $x = valor`)
Às vezes, você está criando um algoritmo que precisa legitimamente transformar um número em um texto ou em um status pendente. Para permitir alterações de tipo sem erros, indique isso explicitamente com `mixed`:

<!-- joss-run: ["pendiente"] -->
```joss
mixed $resultado = 2
$resultado = "pendiente"
print($resultado)
```

> [!NOTE]
> `let $resultado = 2` é exatamente equivalente a `mixed $resultado = 2`. Em Joss, `let` typeless **não significa constante**, mas dinamismo explícito.

### 4. Constantes imutáveis: `const`
A **constant** is a value that is defined only once and can never be reassigned during the life of the program. It serves to shield business rules and configuration parameters:

<!-- joss-run: ["3"] -->
```joss
const int $maximo = 3
print($maximo)
```

Se você tentar digitar `$maximo = 4`, o analisador emitirá um erro `JOSS-SYM-006` indicando que uma constante não pode ser reatribuída.

---

## 4. Texto, comentários e saída formatada

### Delimitadores de texto e caracteres de escape

No Joss você pode escrever strings de texto usando aspas duplas (`"..."`) ou aspas simples (`'...'`):

<!-- joss-run: ["Hola, Ada", "Primera línea", "Segunda línea"] -->
```joss
// Este comentario explica el código; no se ejecuta.
$nombre = 'Ada'
print("Hola, " . $nombre)
/* Un comentario también puede
   ocupar varias líneas. */
print("Primera línea\nSegunda línea")
```

- **Comentários de uma linha**: Eles começam com `//` (ou `#`). Tudo o que você digita à direita é ignorado pelo computador.
- **Comentários multilinhas**: começam com `/*` e terminam com `*/`.
- **Caracteres de escape**:
  - `\n`: Insere uma quebra de linha.
  - `\t`: Insere uma parada de tabulação horizontal.
  - `\"` ou `\'`: Permite incluir aspas literais no texto.
  - `\\`: Insere uma barra invertida.

### Concatenação com o operador ponto (`.`)

Em muitos idiomas, é usado `+` para juntar texto, o que causa erros graves quando números e texto são acidentalmente misturados. Em Joss:
- O operador `+` é reservado **exclusivamente para adição matemática**.
- O operador ponto `.` é usado **exclusivamente para concatenar texto**.

```joss
$a = "10"
$b = "20"
print($a . $b) // Imprime "1020" (unión de textos)
```

### Interpolação de string moderna: `${variable}` ou `${expresión}`

Em vez de encadear vários fragmentos com o operador ponto (`"Hola " . $nombre . " tienes " . $edad . " años"`), Joss permite **incorporar variáveis e cálculos diretamente em textos com aspas duplas** usando a sintaxe `${...}` (idêntica a linguagens modernas como Flutter/Dart ou Kotlin):

<!-- joss-run: ["Hola Ada, tienes 21 años", "El doble es 42"] -->
```joss
$nombre = "Ada"
$edad = 21
print("Hola ${nombre}, tienes ${edad} años")
print("El doble es ${$edad * 2}")
```

- **Variáveis**: Escreva `${variable}` entre aspas duplas para injetar automaticamente seu valor.
- **Cálculos e expressões**: Você pode colocar operações matemáticas ou lógicas completas entre os colchetes: `${$precio * $cantidad}`.
- **Escape literal**: se você precisar que o texto exiba literalmente `${`, digite uma barra invertida antes dele: `\${`.

### Formatação avançada com `printf`

Quando você precisa montar mensagens com variáveis numéricas e textos em posições exatas sem encadeamento de muitos pontos, use `printf`:

```joss
$item = "Teclado"
$cantidad = 2
printf("Producto: %s | Cantidad: %d\n", $item, $cantidad)
```
- `%s` é substituído por uma string (`string`).
- `%d` é substituído por um número inteiro (`int`).

---

## 5. Operações numéricas e matemáticas

<!-- joss-run: ["5", "2.5", "1", "0.3"] -->
```joss
print(2 + 3)
print(5 / 2)
print(5 % 2)
print(0.10m + 0.20m)
```

### Operadores aritméticos

| Operador | Operação | Exemplo | Resultado | Explicação |
|---|---|---|---|---|
| `+` | Soma | `10 + 5` | `15` | Adição matemática. |
| `-` | Subtração | `10 - 4` | `6` | Subtração. |
| `*` | Multiplicação | `3 * 4` | `12` | Produto. |
| `/` | Divisão | `5 / 2` | `2.5` | Em Joss, dividir inteiros **retorna `float`**, evitando perda acidental de decimais. |
| `%` | Módulo (descanso) | `5 % 2` | `1` | O resto da divisão inteira (5 dividido por 2 dá 2 com resto 1). Muito útil para saber se um número é par (`$n % 2 == 0`). |
| `++` | Pós-incremento | `$i++` | Valor atual | Aumente a variável em 1 e retorne seu valor anterior. |
| `--` | Pós-decremento | `$i--` | Valor atual | Diminui a variável em 1 e retorna seu valor anterior. |

### Operadores de atribuição compostos (`+=`, `-=`, `*=`, `??=`)

Quando você deseja modificar o valor que uma variável já possui (por exemplo, adicionando pontos em um jogo ou deduzindo vidas), você não precisa repetir o nome da variável (`$puntos = $puntos + 5`). Você pode usar os operadores de atribuição rápida:

<!-- joss-run: ["15", "12", "24", "Invitado"] -->
```joss
$puntos = 10
$puntos += 5
print($puntos)

$puntos -= 3
print($puntos)

$puntos *= 2
print($puntos)

$nombre = null
$nombre = $nombre ?? "Invitado"
print($nombre)
```

| Operador | Equivalência | Descrição |
|---|---|---|
| `$x += $y` | `$x = $x + $y` | Adiciona `$y` ao valor atual de `$x`. |
| `$x -= $y` | `$x = $x - $y` | Subtraia `$y` do valor atual de `$x`. |
| `$x *= $y` | `$x = $x * $y` | Multiplica o valor atual de `$x` por `$y`. |
| `$x ??= $y` | `$x = $x ?? $y` | Atribui `$y` somente se `$x` for atualmente `null`. |

### Prioridade matemática (precedência)
Como na álgebra, a multiplicação e o módulo são calculados antes da adição e subtração. Use parênteses `(` `)` para definir claramente o que precisa ser resolvido primeiro:

```joss
print(2 + 3 * 4)   // Da 14 (3 * 4 = 12, luego + 2)
print((2 + 3) * 4) // Da 20 (2 + 3 = 5, luego * 4)
```

### Segurança de estouro aritmético

Em sistemas convencionais de 64 bits, se você adicionar 1 ao maior número inteiro possível, o número se torna um valor negativo enorme sem avisar. Joss evita isso em seu kernel:
- Se uma operação inteira exceder o intervalo assinado de 64 bits (-9.223.372.036.854.775.808 para 9.223.372.036.854.775.807), Joss interrompe a execução imediatamente com o erro estruturado `JOSS-ARITH-001` (Arithmetic Overflow).
- Se você tentar dividir por zero (`$x / 0`), Joss interrompe com `JOSS-ARITH-002` (Divisão por Zero).

---

## 6. A diferença crítica: `float` vs `decimal`

Por que `float` e `decimal` existem?

Os computadores representam números de ponto flutuante (`float`) usando potências de dois em binário (padrão IEEE-754). Existem frações decimais como `0.1` ou `0.2` que não possuem representação binária finita exata (semelhante a tentar escrever um terço `1/3` em decimal como `0.33333...`).

Portanto, em quase todos os idiomas tradicionais:
```joss
print(0.1 + 0.2) // Imprime aproximadamente 0.30000000000000004
```

Se você está calculando a trajetória de um projétil em um videogame, esse milionésimo de diferença não importa. Mas se você estiver calculando juros bancários sobre um milhão de transações, essa diferença será um desastre contábil.

**A solução Joss é do tipo `decimal`:**
Ao adicionar o sufixo `m` ou `M`:
```joss
print(0.10m + 0.20m) // Imprime exactamente 0.3
```
Os cálculos são realizados na base dez exata usando aritmética de ponto fixo.

---

## 7. Entrada e saída do console (`print`, `cout`, `cin`)

### Saída do console: `print` e `cout`

Joss oferece duas formas modernas e complementares de exibir informações no terminal:

1. **`print(...)`**: A função padrão para emissão de mensagens. Cada argumento é impresso na tela adicionando uma quebra de linha automática no final.
2. **`cout << ...`**: O fluxo de saída padrão (estilo C++). Permite encadear expressões com o operador `<<`, não adiciona quebras de linha automáticas (você pode usar o manipulador `endl` ou `"\n"`) e também pode ser chamado diretamente como uma função `cout(...)`.
3. **`cerr << ...`**: O fluxo de saída de erros padrão (`stderr`).
4. **`endl`**: Constante nativa que representa a quebra de linha (`"\n"`).

<!-- joss-run: ["Hola mundo", "Linea 1", "Linea 2"] -->
```joss
// Con print: genera salto de línea automático
print("Hola mundo")

// Con cout: encadenamiento con << y manipulador endl
cout << "Linea " << 1 << endl
cout << "Linea " << 2 << endl
```

### Entrada do console com `cin >>`

Para criar programas interativos onde uma pessoa grava dados no console durante a execução, Joss fornece o fluxo de entrada `cin` com o operador `>>`:

```joss
cout << "¿Cómo te llamas? "
string $nombre = ""
cin >> $nombre

cout << "¿Cuántos años tienes? "
int $edad = 0
cin >> $edad

print("Hola ${nombre}, el próximo año tendrás ${$edad + 1} años.")
```

**Recursos de `cin` e `cout`:**
1. **Conversão automática em `cin`**: se a variável de destino for numérica (`int` ou `float`), `cin` converte o texto inserido diretamente no tipo esperado.
2. **Leitura de texto completo**: Se a variável for `string`, captura toda a linha digitada pelo usuário.
3. **Encadeamento múltiplo**: você pode encadear entradas e saídas: `cin >> $a >> $b` ou `cout << "A: " << $a << " B: " << $b << endl`.

### Coalesce atribuição nula (`??=`)

Se uma variável tiver um valor `null` ou ainda não foi inicializado, você pode atribuir a ele um valor padrão somente se estiver vazio usando `??=`:

<!-- joss-run: ["oscuro", "oscuro"] -->
```joss
$tema = null
$tema ??= "oscuro"
print($tema)
$tema ??= "claro" // No sobreescribe porque ya tiene "oscuro"
print($tema)
```

---

## 8. Funções de conversão de tipo (casting)

If você recebe um dado como texto (por exemplo `"25"`) e precisa adicionar um valor a ele, deve convertê-lo explicitamente em um número:

| Função | Converter para | Exemplo de entrada | Resultado |
|---|---|---|---|
| `intval($v)` | `int` | `intval("42")` | `42` |
| `floatval($v)` | `float` | `floatval("3.14")` | `3.14` |
| `decimal($v)` | `decimal` | `decimal("19.99")` | `19.99m` |
| `strval($v)` | `string` | `strval(100)` | `"100"` |
| `boolval($v)` | `bool` | `boolval(1)` | `true` |

> [!WARNING]
> Converter não é o mesmo que validar. Se você tentar executar `intval("manzana")`, a função retornará `0` sem falhar. Se você precisar primeiro verificar se um texto contém números válidos, use `is_numeric($texto)`.

---

## 9. Erros comuns ao iniciar

| Erro comum | Código/Causa | Como consertar |
|---|---|---|
| Use `+` para juntar textos | `print("Total: " + $precio)` | Sempre use o ponto final para o texto: `print("Total: " . $precio)`. |
| Esqueça o `$` em uma variável | `edad = 20` | Todas as variáveis ​​devem ter `$`: `$edad = 20`. |
| Alterar tipo de uma variável inferida | `$x = 10; $x = "hola"` (`JOSS-TYPE-001`) | Se precisar alterar o tipo, declare-o como `mixed $x = 10`. |
| Esqueça o sufixo `m` em valores financeiros | `0.10 + 0.20` | Use `0.10m + 0.20m` para garantir a precisão monetária. |
| Reatribuir uma constante | `const $A = 1; $A = 2` (`JOSS-SYM-006`) | Se o valor precisar ser alterado, não use `const`; use `$A = 1`. |

---

## 10. Miniprojeto guiado: Calculadora de contas e gorjetas

Para colocar em prática tudo o que você aprendeu neste guia (variáveis, tipos, operações matemáticas e texto concatenado), aqui está um programa completo que calcula a gorjeta e divide o total entre amigos:

<!-- joss-run: ["Subtotal: 50", "Propina: 7.5", "Total: 57.5", "Por persona: 28.75"] -->
```joss
// 1. Datos iniciales
$subtotal = 50.0
$propina = $subtotal * 0.15
$total = $subtotal + $propina
$porPersona = $total / 2

// 2. Mostrar resumen en pantalla
print("Subtotal: " . $subtotal)
print("Propina: " . $propina)
print("Total: " . $total)
print("Por persona: " . $porPersona)
```

### Explicação passo a passo:
1. `$subtotal = 50.0`: Salvamos o valor da conta como um número decimal (`float`).
2. `$propina = $subtotal * 0.15`: Calculamos 15% multiplicando por `0.15`.
3. `$total = $subtotal + $propina`: Adicionamos o custo do consumo e a gorjeta.
4. `$porPersona = $total / 2`: Dividimos a conta igualmente entre duas pessoas.
5. Chamadas para `print(...)`: Junte o texto longo com o valor numérico usando o operador de concatenação de ponto `.`.

---

## 11. Exercícios práticos para iniciantes

1. **Conversor de temperatura**:
   - Declare uma variável `$celsius = 25.0`.
   - Aplique a fórmula para converter para Fahrenheit: `$fahrenheit = ($celsius * 9 / 5) + 32`.
   - Imprima o resultado concatenado: `print($celsius . " °C equivalen a " . $fahrenheit . " °F")`.
2. **Placar de jogo de atribuição rápida**:
   - Começa com `$puntos = 0`.
   - Adicione 100 pontos com `+=`, subtraia 20 com `-=` para penalidade e dobre a pontuação com `*= 2` para um bônus.
   - Exibe a pontuação final no console.

---

## Próximo passo

Agora que você domina dados, tipos e matemática na memória, é hora de dar decisão e repetibilidade aos seus programas: como executar um bloco somente se uma condição for atendida e como criar loops.

Continua com: [Controle de fluxo e estruturas de repetição](CONTROL_FLUJO.md).

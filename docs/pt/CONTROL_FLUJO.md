# Controle de fluxo e tomada de decisões

Antes: [Valores e variáveis](FUNDAMENTOS.md). Depois: [Coleções: Arrays, Maps e Texto](COLECCIONES.md).
Referência técnica: [Sintaxe e operadores](SINTAXIS.md), [Gramática](GRAMATICA.md).

---

## O que você vai aprender aqui?

Até agora, nossos programas foram executados em linha reta: o computador lê a linha 1, depois a 2, depois a 3 e termina. A essa ordem natural dá-se o nome de **fluxo sequencial**.

No entanto, o verdadeiro poder da programação está em responder a situações variáveis e repetir tarefas sem se cansar:
- *Se* o usuário tiver 18 anos ou mais, permitir a entrada; *caso contrário*, pedir que aguarde.
- *Enquanto* o contador não chegar a 3, continuar contando.
- *Para cada* produto de uma lista, mostrar seu nome na tela.

Neste guia você aprenderá passo a passo:
1. O que é uma **condição lógica** (`bool`) e como comparar dados.
2. Como combinar perguntas lógicas com `&&` (E), `||` (OU) e `!` (NÃO).
3. Como tomar decisões no Joss usando o **operador ternário (`? :`)** com valores e com blocos `{ ... }`.
4. Como atribuir valores padrão com o operador Elvis `?:` e a coalescência nula `??` / `??=`.
5. Como escolher entre muitas opções de forma limpa com **`match`**.
6. Como repetir instruções com os laços **`while`**, **`do ... while`** e **`foreach`**.
7. Como interromper ou pular uma volta de um laço com **`break`** e **`continue`**.

---

## 1. Decisões lógicas: Fazendo perguntas ao computador

Uma **condição** é uma pergunta que o computador avalia para responder unicamente com um valor booleano (`bool`): **`true`** (verdadeiro) ou **`false`** (falso).

<!-- joss-run: ["true", "Entrada permitida"] -->
```joss
var $edad = 20
print($edad >= 18)
print(($edad >= 18) ? "Entrada permitida" : "Debes esperar")
```

Saída:

```text
true
Entrada permitida
```

### Operadores de comparação

| Operador | Pergunta que realiza | Exemplo | Resultado |
|---|---|---|---|
| `==` | Têm o mesmo valor? | `5 == 5` | `true` |
| `!=` | São diferentes? | `5 != 3` | `true` |
| `===` | São idênticos em valor **e também no tipo**? | `5 === "5"` | `false` |
| `!==` | Não são estritamente idênticos? | `5 !== "5"` | `true` |
| `<` | O da esquerda é menor? | `3 < 5` | `true` |
| `<=` | É menor ou igual? | `5 <= 5` | `true` |
| `>` | O da esquerda é maior? | `10 > 2` | `true` |
| `>=` | É maior ou igual? | `20 >= 18` | `true` |
| `<=>` | Comparação tripla (*Spaceship*) | `$a <=> $b` | Retorna `-1` se `$a < $b`, `0` se forem iguais e `1` se `$a > $b`. |

### Combinar várias condições (`&&`, `||`, `!`)

Quando uma decisão depende de duas ou mais perguntas ao mesmo tempo, usamos os **operadores lógicos**:

<!-- joss-run: ["true", "true", "false"] -->
```joss
var $edad = 22
var $tieneEntrada = true

print($edad >= 18 && $tieneEntrada)
print($edad < 18 || $tieneEntrada)
print(!$tieneEntrada)
```

Saída:

```text
true
true
false
```

- **`&&` (E lógico)**: Retorna `true` somente se **ambas** as condições forem cumpridas.
- **`||` (OU lógico)**: Retorna `true` se **pelo menos uma** das condições for cumprida.
- **`!` (Negação)**: Inverte o valor lógico (`!true` é `false`, e `!false` é `true`).

---

## 2. Tomar decisões no Joss: O operador ternário (`? :`)

Diferentemente de outras linguagens que usam as palavras `if` e `else`, **o Joss unifica todas as decisões sob uma única construção clara: o operador ternário**.

Sua forma básica lê-se como uma pergunta natural:

```text
(¿se cumple la condición?) ? resultado_si_es_verdadero : resultado_si_es_falso
```

### 1. Escolher entre dois valores e guardá-los em uma variável

Como o ternário produz um valor diretamente, você pode guardar a resposta em uma variável em uma única linha:

<!-- joss-run: ["Aprobado"] -->
```joss
var $puntos = 75
var $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
print($estado)
```

Saída:

```text
Aprobado
```

- Primeiro avalia-se `($puntos >= 60)`. Como `75 >= 60` é `true`, o Joss escolhe o texto após o sinal `?` (`"Aprobado"`) e o guarda em `$estado`.
- Se `$puntos` fosse `40`, teria escolhido o texto após os dois pontos `:` (`"Reprobado"`).

### 2. Executar várias linhas com blocos `{ ... }`

Um **bloco** é um grupo de instruções delimitadas entre chaves `{` e `}`. Quando você quer que um ramo execute vários passos seguidos, basta colocar um bloco `{ ... }`:

<!-- joss-run: ["Hay existencias", "Preparando pedido"] -->
```joss
var $existencias = 4
($existencias > 0) ? {
    print("Hay existencias")
    print("Preparando pedido")
} : {
    print("Producto agotado")
}
```

Saída:

```text
Hay existencias
Preparando pedido
```

### 3. O que acontece se eu só precisar fazer algo quando for verdadeiro?

Se você não precisar fazer nada quando a condição for falsa, pode **omitir por completo a parte `: { ... }`**:

<!-- joss-run: ["Bienvenido de nuevo"] -->
```joss
var $usuarioAutenticado = true
($usuarioAutenticado) ? {
    print("Bienvenido de nuevo")
}
```

Saída:

```text
Bienvenido de nuevo
```

*(Também é válido escrever `: {}` no final se preferir deixar o bloco vazio explícito).*

> [!NOTE]
> **E as cláusulas de guarda (`guard`) e limpeza (`defer`)?**
> O Joss também conta com as instruções `guard (condicion) : { ... }` (para validar condições e sair cedo de uma função) e `defer { ... }` (para executar tarefas de fechamento ao terminar uma função). Você as aprenderá no capítulo de [Funções](FUNCIONES.md), assim que conhecer como funcionam `public func` e `return`.

---

## 3. Valores padrão rápidos: Elvis (`?:`) e Coalescência Nula (`??` e `??=`)

Muitas vezes você quer dizer: *"use este dado se existir, ou use este valor reserva se estiver vazio"*. O Joss tem três operadores projetados para isso:

<!-- joss-run: ["Anónimo", "Invitado", "oscuro"] -->
```joss
var $aliasIngresado = ""
var $apodo = $aliasIngresado ?: "Anónimo"
print($apodo)

var $opcionUsuario = null
var $perfil = $opcionUsuario ?? "Invitado"
print($perfil)

var $tema = null
$tema ??= "oscuro"
print($tema)
```

Saída:

```text
Anónimo
Invitado
oscuro
```

| Operador | Quando usa o valor reserva (direita) | Quando escolhê-lo |
|---|---|---|
| **`?:` (Elvis)** | Quando a esquerda é falsa ou vazia (`null`, `false`, `0`, `""`, `[]`). | Quando um texto vazio `""` ou um `0` devem ser substituídos pelo valor padrão. |
| **`??` (Coalescência nula)** | **Unicamente** quando a esquerda é `null`. | Quando `0`, `false` ou `""` são dados válidos que você quer preservar e só deseja substituir `null`. |
| **`??=` (Atribuição nula)** | Guarda o valor da direita dentro da variável somente se atualmente ela valer `null`. | Para inicializar variáveis que poderiam estar em `null`. |

---

## 4. Seleção múltipla com `match`

Quando uma variável pode assumir muitos valores diferentes (como as cores de um semáforo, os dias da semana ou o estado de um pedido), usar a expressão **`match`** torna o código limpo e fácil de ler:

<!-- joss-run: ["En camino"] -->
```joss
var $estado = "enviado"
var $mensaje = match ($estado) {
    "nuevo" => "Preparando paquete",
    "enviado", "reparto" => "En camino",
    "entregado" => "Pedido completado",
    default => "Consulta el estado de tu pedido"
}
print($mensaje)
```

Saída:

```text
En camino
```

### Como funciona o `match`:
1. O Joss compara o valor de `$estado` com as opções de cada linha de cima para baixo.
2. Você pode agrupar vários casos com uma vírgula (por exemplo `"enviado", "reparto" => "En camino"`).
3. Assim que encontra a primeira coincidência, retorna o que está à direita da seta `=>` e termina (você não precisa escrever `break`).
4. **`default`**: É a opção curinga executada se nenhum caso anterior coincidir.
5. **Blocos em `match`**: Se em vez de retornar um valor você quiser executar várias instruções em cada caso, também pode usar chaves `{ ... }`:

<!-- joss-run: ["Opción 1 ejecutada"] -->
```joss
var $opcion = 1
match ($opcion) {
    1 => {
        print("Opción 1 ejecutada")
    },
    default => {
        print("Opción por defecto")
    }
}
```

Saída:

```text
Opción 1 ejecutada
```

---

## 5. Laços: Repetir instruções com `while` e `do ... while`

Um **laço** (ou loop) ordena ao computador repetir um bloco `{ ... }` enquanto uma condição continuar sendo verdadeira (`true`).

### O laço `while` (repetir enquanto a condição for cumprida)

`while` verifica a condição **antes** de cada volta:

<!-- joss-run: ["1", "2", "3"] -->
```joss
var $numero = 1
while ($numero <= 3) {
    print($numero)
    $numero++
}
```

Saída:

```text
1
2
3
```

#### O que acontece volta por volta na memória?
1. **Início**: `$numero` vale `1`.
2. **Volta 1**: `1 <= 3`? Sim (`true`). Imprime `1` e `$numero++` o sobe para `2`.
3. **Volta 2**: `2 <= 3`? Sim (`true`). Imprime `2` e `$numero++` o sobe para `3`.
4. **Volta 3**: `3 <= 3`? Sim (`true`). Imprime `3` e `$numero++` o sobe para `4`.
5. **Verificação final**: `4 <= 3`? Não (`false`). O laço termina e o programa continua abaixo.

> [!CAUTION]
> **Evite os laços infinitos**:
> Dentro das chaves de um `while` a variável da condição deve sempre mudar (por exemplo com `$numero++`). Se você esquecer essa linha, `$numero` valerá `1` para sempre e o laço nunca terminará (se isso acontecer no seu terminal, pressione `Ctrl + C` para interrompê-lo).

### O laço `do ... while` (executar pelo menos uma vez)

Diferentemente de `while`, o laço `do ... while` executa o bloco **primeiro** e verifica a condição **no final**. Assim você garante que o bloco rode no mínimo uma vez:

<!-- joss-run: ["Intento 1"] -->
```joss
var $intento = 0
do {
    $intento++
    print("Intento " . $intento)
} while ($intento < 1)
```

Saída:

```text
Intento 1
```

---

## 6. Percorrer sequências e listas com `foreach`

Quando você quer percorrer uma sequência de números de `1` a `3` ou passar por cada elemento de uma lista sem se preocupar em somar um contador manualmente, utiliza **`foreach`** (*para cada um*).

### 1. Percorrer um intervalo de números (`inicio..fin`)

O operador de dois pontos seguidos `..` gera os números inteiros do início até o fim:

<!-- joss-run: ["Paso 1", "Paso 2", "Paso 3"] -->
```joss
foreach (1..3 as $paso) {
    print("Paso ${paso}")
}
```

Saída:

```text
Paso 1
Paso 2
Paso 3
```

### 2. Percorrer uma lista simples

> **Breve prévia**: No próximo capítulo ([Coleções](COLECCIONES.md)) aprenderemos tudo sobre as listas (`array`). Por enquanto, basta saber que escrever vários valores entre colchetes `["pan", "leche"]` cria uma lista ordenada de elementos.

Veja como é fácil percorrer cada elemento de uma lista com `foreach`:

<!-- joss-run: ["pan", "leche"] -->
```joss
var $compras = ["pan", "leche"]
foreach ($compras as $producto) {
    print($producto)
}
```

Saída:

```text
pan
leche
```

- **Na primeira volta**, `$producto` assume o valor `"pan"` e o imprime.
- **Na segunda volta**, `$producto` assume o valor `"leche"` e o imprime.
- Não restando mais elementos em `$compras`, o laço para sozinho.

---

## 7. Controlar o interior de um laço: `break` e `continue`

Dentro de qualquer laço (`while`, `do ... while` ou `foreach`) você pode usar duas ordens especiais:

- **`break`**: Interrompe o laço por completo naquele exato instante e continua na linha seguinte ao fechamento do laço.
- **`continue`**: Pula o restante da volta atual e passa imediatamente para a próxima iteração do laço.

Por exemplo, se quisermos percorrer uma sequência, mas parar assim que tivermos impresso o primeiro elemento:

<!-- joss-run: ["Primero: 10"] -->
```joss
var $datos = [10, 20, 30]
foreach ($datos as $valor) {
    print("Primero: " . $valor)
    break
}
```

Saída:

```text
Primero: 10
```

Ao executar `break` no final da primeira volta, os valores `20` e `30` já não são processados.

De forma análoga, você pode usar um ternário ou `break`/`continue` para pular ou interromper um laço quando uma condição for cumprida:

```joss
while ($intentos < 10) {
    $intentos++
    ($intentos == 5) ? {
        break
    }
}
```

---

## 8. Erros comuns ao tomar decisões e repetir código

| Erro comum | Por que ocorre | Como solucionar |
|---|---|---|
| Escrever `if ($x > 0)` ou `for (...)` | `if`, `else`, `switch` e `for` não existem no Joss (`JOSS-PARSE-001`). | Use o operador ternário `($x > 0) ? { ... }`, `match` e `foreach` / `while`. |
| Confundir `=` com `==` | `=` guarda um valor; `==` compara dois valores. | Dentro de uma pergunta lógica use sempre `==` ou `===` (`$edad == 18`). |
| Esquecer de atualizar a variável em `while` | A condição nunca muda e o laço se repete sem fim. | Certifique-se de incrementar (`$i++`) ou modificar a variável dentro do bloco `{ ... }`. |
| Usar `break` ou `continue` fora de um laço | Só fazem sentido dentro de `while`, `do...while` ou `foreach` (`JOSS-FLOW-002`). | Coloque-os unicamente dentro do corpo de um laço. |

---

## 9. Prática do capítulo

### 1. Teste rápido
No exemplo de `match` da seção 4:
1. Mude o valor inicial de `var $estado = "enviado"` para `"entregado"`, e depois para `"cancelado"`.
2. Execute `joss run main.joss` em cada caso e observe qual ramo de `match` é ativado.

### 2. Exercício guiado: Classificador de notas
Escreva no seu `main.joss` um programa que avalie uma nota seguindo estes passos:
1. Declare `int $nota = 85`.
2. Use um ternário para verificar se `$nota >= 90` e imprimir `"Sobresaliente"`.
3. No ramo `: { ... }`, verifique se `$nota >= 60` para imprimir `"Aprobado"` ou `"Reprobado"`.
4. Teste seu programa com as notas `95`, `70` e `45`.

### 3. Desafio: Tabuada de multiplicação e soma de pares
1. Declare uma variável `int $tabla = 7`.
2. Use `foreach (1..10 as $i)` para imprimir a tabuada completa do 7 com o formato `"7 x 1 = 7"`, `"7 x 2 = 14"`, etc.
3. Declare antes do laço uma variável `int $sumaPares = 0` e, dentro do laço, use um ternário `(($tabla * $i) % 2 == 0) ? { ... }` para somar em `$sumaPares` unicamente os resultados que forem números pares. Imprima a soma no final.

---

## Próximo passo

Você já sabe como tomar decisões e percorrer sequências. O próximo passo natural é aprender a guardar e organizar coleções completas de dados na memória: listas dinâmicas (**arrays**), dicionários com nome (**maps**) e processamento de texto.

Continue com: [Coleções: Arrays, Maps e Manipulação de Texto Unicode](COLECCIONES.md).
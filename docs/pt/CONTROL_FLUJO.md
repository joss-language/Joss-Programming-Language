# Controle de fluxo e tomada de decisão

Antes: [Valores e variáveis](FUNDAMENTOS.md). Depois: [Funções e fechamentos](FUNCIONES.md).
Referência técnica: [Sintaxe e operadores](SINTAXIS.md), [Gramática](GRAMATICA.md).

---

## O que você vai aprender aqui?

Até agora, todos os nossos programas foram executados em linha reta: o computador lê a linha 1, depois a 2, depois a 3, e pronto. Essa ordem natural é chamada de **fluxo sequencial**.

No entanto, o verdadeiro poder da programação reside na capacidade de responder às novas circunstâncias:
- *Se* o usuário digitou a senha correta, deixe-o entrar; *caso contrário* será exibida uma mensagem de erro.
- *Enquanto* ainda houver e-mails para enviar, continue enviando-os um por um.
- *Para cada* produto do carrinho de compras, adicione seu preço ao total.

Neste guia você aprenderá:
1. O que é uma **condição lógica** e como avaliá-la.
2. Como tomar decisões em Joss usando o **operador de bloco ternário** e por que Joss não usa a sintaxe clássica `if/else`.
3. O operador Elvis `?:` e o operador de coalescência nula `??`.
4. Como estruturar seleções múltiplas elegantes com a expressão `match`.
5. Como iterar código com loops `while`, `do...while` e `foreach` (incluindo seu uso com canais de simultaneidade e valores-chave).
6. Como interromper ou avançar um loop com `break` e `continue`.
7. Como garantir a limpeza de recursos e a conclusão de tarefas com `defer`.

---

## 1. Decisões lógicas: pergunte ao computador

Uma **condição** é qualquer expressão que o computador avalia para obter uma resposta lógica do tipo booleano (`bool`): é verdadeira (`true`) ou falsa (`false`).<!-- joss-run: ["true", "Entrada permitida"] -->
```joss
$edad = 20
print($edad >= 18)
print(($edad >= 18) ? "Entrada permitida" : "Debes esperar")
```
### Operadores de comparação

| Operador | Pergunta lógica | Exemplo | Resultado |
|---|---|---|---|
| `==` | Os valores são iguais? | `5 == 5` | `true` |
| `!=` | Os valores são diferentes? | `5 != 3` | `true` |
| `===` | Eles são estritamente idênticos em valor **e tipo**? | `5 === "5"` | `false` |
| `!==` | Eles não são estritamente idênticos? | `5 !== "5"` | `true` |
| `<` | O da esquerda é menor? | `3 < 5` | `true` |
| `<=` | É menor ou igual? | `5 <= 5` | `true` |
| `>` | É estritamente mais antigo? | `10 > 2` | `true` |
| `>=` | É maior ou igual? | `20 >= 18` | `true` |
| `<=>` | Operador de nave espacial (nave espacial) | `$a <=> $b` | Retorna `-1` se `$a < $b`, `0` se `$a == $b`, `1` se `$a > $b`. |

---

## 2. Filosofia de Joss: O operador ternário como estrutura de controle

Ao contrário de outras linguagens que possuem uma palavra reservada `if` e outra `else`, **Joss unifica todas as decisões sob o operador ternário**.

A estrutura básica do ternário é:```text
(condición) ? resultado_si_es_verdadero : resultado_si_es_falso
```
### Por que Joss escolheu esse design?

1. **É uma expressão, não uma instrução isolada**: Você pode atribuir o resultado de uma decisão diretamente a uma variável sem criar variáveis ​​intermediárias vazias:   ```joss
   $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
   ```
2. **Estrutura visual limpa e inequívoca**: Evite problemas clássicos de `if` sem colchetes ou aninhamentos confusos.

### Execute múltiplas instruções com blocos `{ ... }`

Quando você precisar executar várias linhas de código em uma das ramificações, basta colocar um bloco entre chaves `{` e `}`:<!-- joss-run: ["Hay existencias", "Preparando pedido"] -->
```joss
$existencias = 4
($existencias > 0) ? {
    print("Hay existencias")
    print("Preparando pedido")
} : {
    print("Producto agotado")
}
```
### E se eu não precisar do branch falso?

Se você quiser fazer algo apenas quando a condição for verdadeira e não precisar de uma alternativa falsa, você pode pular o ramo `: { ... }` completamente:<!-- joss-run: ["Bienvenido de nuevo"] -->
```joss
$usuarioAutenticado = true
($usuarioAutenticado) ? {
    print("Bienvenido de nuevo")
}
```
Também é válido escrever a forma simétrica com bloco vazio `: {}` se preferir manter ambos os lados explícitos.

### Cláusulas de guarda com ternários e a instrução `guard`

Em Joss, se você executar uma instrução `return` dentro de um bloco ternário, o `return` **imediatamente sai da função que o contém**. Isso permite que você escreva *cláusulas de proteção* limpas e evite aninhamentos profundos:```joss
public func procesarPago(decimal $monto): bool {
    ($monto <= 0.0m) ? {
        print("Error: Monto inválido")
        return false
    }

    // El código continúa en línea recta
    print("Procesando pago de: " . $monto)
    return true
}
```
### Instrução nativa `guard ... :` com rescisão obrigatória e Smart Casts

Consistente com a filosofia de Joss, onde `if` e `else` não existem, a instrução **`guard`** adota os dois pontos `:` do operador ternário para definir seu bloco de escape:

- Expressa na condição o estado **desejado** para continuar a execução em linha reta.
- Se a condição não for atendida, o bloco após os dois pontos `:` é executado obrigatoriamente.
- O bloco de escape **deve encerrar a função** usando `return` ou `throw`; caso contrário, o analisador emitirá o erro de diagnóstico `JOSS-FLOW-005`.
- Ao sair do branch de escape, o sistema de tipos executa **Smart Cast** (*Type Narrowing*), reduzindo tipos como `T|null` diretamente para `T` no fluxo principal subsequente:<!-- joss-run: ["Procesando: 50"] -->
```joss
public func procesar(int $monto): string {
    guard ($monto > 0) : {
        return "Monto inválido"
    }
    return "Procesando: " . $monto
}
print(procesar(50))
```
---

## 3. Operadores Elvis `?:` e Coalescência Nula `??`

Joss fornece dois atalhos muito poderosos para atribuir valores padrão:

### O operador Elvis (`?:`)
Avalie a expressão à esquerda. Se for verdadeiro (ou tiver um valor não vazio e diferente de zero), ele retornará essa expressão; se for falso ou vazio, retorna o valor à direita:```joss
$apodo = $aliasIngresado ?: "Anónimo"
```
### O operador de coalescência nula (`??`)
Centra-se exclusivamente na existência de um valor. Se a variável à esquerda for `null` (ou não definida), ela retorna a alternativa à direita:```joss
$configuracion = $opcionUsuario ?? "valor_predeterminado"
```
---

## 4. Seleção múltipla com `match`

Quando uma variável pode ter muitos valores possíveis (por exemplo, o status de um envio, a função de um usuário ou o código de resposta de um servidor), o encadeamento de ternários torna-se difícil de ler.

Para estes casos, Joss oferece a expressão **`match`**:<!-- joss-run: ["En camino"] -->
```joss
$estado = "enviado"
$mensaje = match ($estado) {
    "nuevo" => "Preparando",
    "enviado", "reparto" => "En camino",
    default => "Consulta el pedido"
}
print($mensaje)
```
### Recursos do `match`:
- **Múltiplos braços**: Cada linha é composta por um ou mais padrões, seguidos por uma seta grossa `=>` e o valor ou bloco resultante.
- **Suporte para blocos de instruções**: os braços podem conter blocos multilinhas entre colchetes `{ ... }` para executar várias instruções consecutivas ou atualizar o estado do programa:<!-- joss-run: ["Opción 1 ejecutada"] -->
```joss
$opcion = 1
match ($opcion) {
    1 => {
        print("Opción 1 ejecutada")
    },
    default => {
        print("Opción por defecto")
    }
}
```
- **Agrupamento com vírgulas**: Você pode associar vários valores ao mesmo resultado em uma única linha (por exemplo `"enviado", "reparto"`).
- **Arm padrão (`default`)**: Cobre qualquer valor que não corresponda aos anteriores. É uma boa prática incluí-lo sempre para evitar resultados indefinidos.
- **Sem falhas**: Ao contrário do antigo `switch` C ou Java, `match` apenas executa o primeiro braço correspondente e termina; não requer palavras-chave como `break`.

---

## 5. Loops e estruturas de repetição

Um **loop** (ou ciclo) diz ao computador para executar um bloco de código repetidamente, desde que uma condição permaneça verdadeira.

### O loop `while` (repetir while)

O loop `while` avalia a condição **antes** de entrar no corpo do loop. Se a condição for falsa desde o início, o corpo não será executado nem uma vez:<!-- joss-run: ["1", "2", "3"] -->
```joss
$numero = 1
while ($numero <= 3) {
    print($numero)
    $numero++
}
```
> [!CUIDADO]
> **Cuidado com loops infinitos**:
> Dentro do corpo de um `while`, sempre deve haver uma instrução que modifique as variáveis de condição (como `$numero++`). Se a condição nunca se tornar falsa, o programa ficará preso para sempre consumindo o processador até que você o force a parar em seu terminal (em Joss, você pode pressionar a tecla `q` ou `Ctrl + C`).

### O loop `do ... while` (faça pelo menos uma vez)

Ao contrário de `while`, o loop `do ... while` executa o corpo **primeiro** e verifica a condição no final. Isso garante que as instruções serão executadas pelo menos uma vez, independente da condição inicial:<!-- joss-run: ["Intento 1"] -->
```joss
$intento = 0
do {
    $intento++
    print("Intento " . $intento)
} while ($intento < 1)
```
---

## 6. Percorra coleções e canais com `foreach`

Quando você tem uma lista de dados (como um `array` de nomes ou produtos), você não precisa gerenciar manualmente um contador numérico: você usa **`foreach`**.<!-- joss-run: ["pan", "leche"] -->
```joss
$compras = ["pan", "leche"]
foreach ($compras as $producto) {
    print($producto)
}
```
A cada volta do loop, Joss pega o próximo elemento da coleção `$compras`, deposita-o na variável temporária `$producto` e executa o bloco de código.

### Iteração em intervalos numéricos (`..`)

Você também pode iterar sequências numéricas contínuas sem criar matrizes manualmente usando o operador de intervalo `..`:<!-- joss-run: ["Paso 1", "Paso 2", "Paso 3"] -->
```joss
foreach (1..3 as $paso) {
    print("Paso ${paso}")
}
```
### Iteração sobre canais de simultaneidade (`channel`)
Uma característica distintiva do Joss é que `foreach` não é usado apenas para percorrer listas estáticas na memória: ele também pode consumir **canais de comunicação simultâneos** (`channel`). O loop lerá mensagens do canal em tempo real até que o canal seja fechado com `close($canal)`.

---

## 7. Controle de loop: `break` e `continue`

Dentro de qualquer loop (`while`, `do...while` ou `foreach`), você pode alterar o fluxo de repetição com duas instruções fundamentais:

### `break`: Finaliza o loop imediatamente
Aborte o loop e pule diretamente para a primeira linha após o loop:<!-- joss-run: ["1"] -->
```joss
foreach ([1, 2, 3, 4] as $numero) {
    print($numero)
    break
}
```
Neste exemplo, apenas `1` é impresso porque a instrução `break` cancela o loop imediatamente.

### `continue`: Pular para a próxima volta
Ignora as instruções restantes do corpo do loop apenas para o loop atual, avançando para a próxima iteração:```joss
foreach ([1, 2, 3, 4, 5] as $n) {
    ($n % 2 == 0) ? {
        continue // Si es par, sáltatelo
    } : {}
    print("Impar: " . $n)
}
```
---

## 8. Limpeza de recursos garantida: `defer`

A instrução `defer` adia a execução de um bloco ou expressão até o exato momento em que a função ou arquivo atual termina, garantindo a liberação de memória, fechamento de arquivos ou conexões. Múltiplas instruções `defer` são executadas na ordem **LIFO** (Last In, First Out):<!-- joss-run: ["inicio", "fin", "limpieza 2", "limpieza 1"] -->
```joss
public func tarea() {
    print("inicio")
    defer {
        print("limpieza 1")
    }
    defer {
        print("limpieza 2")
    }
    print("fin")
}
tarea()
```
---

## 9. Erros comuns e boas práticas

| Erro | Causa | Solução |
|---|---|---|
| Escreva `if ($x > 0)` | `if` não existe na gramática de Joss. | Use a sintaxe ternária: `($x > 0) ? { ... } : { ... }`. |
| Esqueça o `:` no ternário | O ramo falso está faltando. | Se você não tem nada para fazer no branch falso, escreva `{}`: `($x > 0) ? { print("ok") } : {}`. |
| Loop infinito em `while` | Esquecer de aumentar ou alterar a variável de controle. | Certifique-se de atualizar o contador ou sinalizador dentro do corpo do loop. |
| `break` ou `continue` fora de um loop | Coloque-os no nível superior de um arquivo ou função. | Eles só são válidos dentro de um `while`, `do...while` ou `foreach`. |

---

## 10. Exercícios práticos

1. **Classificador de notas**:
   - Declare uma variável inteira `$nota = 85`.
   - Usando um ternário com blocos, imprima:
     - "Excelente" se `$nota >= 90`.
     - "Aprovado" se `$nota >= 60` e `$nota < 90`.
     - "Falha" se `$nota < 60`.
2. **Soma com `foreach`**:
   - Crie uma matriz `$precios = [10, 25, 5, 40]`.
   - Inicializa uma variável `$total = 0`.
   - Percorra o array com `foreach`, adicionando cada preço a `$total`.
   - Imprima o total final (deve ser `80`).

---

## Próxima etapa

Agora que você pode tomar decisões e repetir operações, aprenderemos como empacotar blocos reutilizáveis de lógica com nomes próprios, parâmetros e valores de retorno:

Continue com: [Funções, escopo de variável (escopo), fechamentos e referências](FUNCIONES.md).
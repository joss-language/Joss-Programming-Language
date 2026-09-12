# Funções, escopo de variáveis, fechamentos e referências

Antes: [Controle de fluxo e decisões](CONTROL_FLUJO.md). Depois: [Coleções: Arrays, Mapas e Texto](COLECCIONES.md).
Referência técnica: [Sintaxe](SINTAXIS.md), [Recursão](RECURSION.md), [Sistema de tipos](SISTEMA_TIPOS.md).

---

## O que você vai aprender aqui?

À medida que os programas crescem, escrever centenas de instruções seguidas torna-se incontrolável. Se você precisa calcular o total de uma nota fiscal em dez lugares diferentes do seu aplicativo, copiar e colar a mesma fórmula dez vezes é uma receita para o desastre: se a legislação tributária mudar, você terá que encontrar e corrigir dez arquivos e, mais cedo ou mais tarde, esquecerá um.

Neste guia você aprenderá:
1. O que é uma **função** e por que ela é o alicerce fundamental do software.
2. A diferença crítica entre **parâmetros**, **argumentos** e **impressão** versus **retorno**.
3. Como declarar funções com tipos seguros e valores padrão.
4. Regras de visibilidade obrigatórias em Joss (`public` e `private`).
5. O que é escopo variável e como funciona o isolamento de memória em cada chamada.
6. O que são **funções anônimas (fechamentos)** e como elas capturam dados de seu ambiente.
7. Como modificar variáveis ​​externas com segurança usando **referências (`ref`)**.
8. Como encadear transformações limpas de dados com o operador **pipeline (`|>`)**.

---

## 1. O que é uma função?

Uma **função** é um bloco autônomo de instruções ao qual atribuímos um nome. Pense nisso como uma pequena máquina especializada:
1. Recebe matéria-prima (dados de entrada, chamados **argumentos**).
2. Ele processa as informações internas isoladamente.
3. Retorna um produto acabado (o resultado, denominado **valor de retorno**).<!-- joss-run: ["5", "12"] -->
```joss
public func sumar(int $a, int $b): int {
    return $a + $b
}
print(sumar(2, 3))
print(sumar(5, 7))
```
### Anatomia de uma função em Joss:

- `public`: **Modificador de visibilidade**. Em Joss, as funções globais requerem visibilidade:
  - `public`: A função está disponível em todo o projeto e outros arquivos podem utilizá-la sem importações.
  - `private`: A função só pode ser chamada a partir do mesmo arquivo onde foi escrita.
- `func`: Palavra-chave que indica o início da declaração (Joss não utiliza `function`).
- `sumar`: O nome da função.
- `(int $a, int $b)`: A lista de **parâmetros**. Define quais tipos de dados a função requer para funcionar.
- `: int`: O **tipo de retorno**. Declara que tipo de valor a função promete retornar ao chamador.
- `{ ... }`: O **corpo** da função, onde as instruções são escritas.
- `return $a + $b`: A instrução `return` encerra a execução da função imediatamente e envia o resultado de volta ao chamador.

---

## 2. Diferenças conceituais cruciais

### Parâmetros vs Argumentos
- **Parâmetro**: É a variável que você declara na assinatura da função (por exemplo, `$a` e `$b`). É o slot ou espaço que espera um valor.
- **Argumento**: É o valor específico que você fornece ao chamar a função (por exemplo, `2` e `3`).

### Return (`return`) vs Imprimir (`print`)
Essa é uma das armadilhas mais comuns para quem começa a programar:
- `print` é uma ação física: escreve com tinta na tela do terminal para um ser humano ler. O resto do programa **não pode reutilizar essa saída**.
- `return` é uma ação de memória interna: entrega os dados calculados para a linha que chamou a função para que ela possa continuar operando com ela (salvar em uma variável, salvar em um banco de dados, enviar pela internet, etc.).

---

## 3. Tipos de contrato e valores padrão

Em Joss, **cada parâmetro de origem deve ter um tipo de dados explícito**. Se uma função realmente precisa aceitar qualquer valor dinâmico, você deve indicar isso escrevendo `mixed`.

### Parâmetros com valores padrão (Padrões)

Você pode tornar certos argumentos opcionais atribuindo-lhes um valor padrão com `=`:<!-- joss-run: ["Hola, visitante", "Hola, Ada"] -->
```joss
public func saludo(string $nombre = "visitante"): string {
    return "Hola, " . $nombre
}
print(saludo())
print(saludo("Ada"))
```
- Se você chamar `saludo()` sem argumentos, Joss usará automaticamente `"visitante"`.
- Se você chamar `saludo("Ada")`, o valor entregue substitui o valor padrão.

> [!TIP]
> Sempre coloque parâmetros com valores padrão no final da lista de parâmetros. Caso contrário, Joss não saberia a qual parâmetro atribuir um argumento se você passar apenas um.

### Argumentos Nomeados

Você pode passar argumentos declarando explicitamente o nome do parâmetro seguido por dois pontos (`nombre: valor`). Isso permite pular parâmetros intermediários que possuem valores padrão ou passar argumentos em qualquer ordem:<!-- joss-run: ["Estimada Ada"] -->
```joss
public func bienvenida(string $nombre, string $titulo = "Estimado/a"): string {
    return $titulo . " " . $nombre
}

print(bienvenida(titulo: "Estimada", nombre: "Ada"))
```
### Operador Spread (`...`) em chamadas

Se você possui uma lista e deseja descompactar seus elementos como argumentos posicionais independentes para uma chamada de função, acrescente `...`:<!-- joss-run: ["10"] -->
```joss
public func sumarTres(int $a, int $b, int $c): int {
    return $a + $b + $c
}

$numeros = [2, 3, 5]
print(sumarTres(...$numeros))
```
---

## 4. Retorno antecipado e cláusulas de guarda

A instrução `return` interrompe a função imediatamente. Se você colocá-lo dentro de um bloco ternário, poderá verificar se há erros no início da função e sair antes de processar o restante:<!-- joss-run: ["agotado", "disponible"] -->
```joss
public func disponibilidad(int $cantidad): string {
    ($cantidad <= 0) ? { return "agotado" } : {}
    return "disponible"
}
print(disponibilidad(0))
print(disponibilidad(2))
```
Esse padrão é chamado de **cláusula de guarda**. Evite criar estruturas condicionais profundamente aninhadas (*código espaguete*).

> [!IMPORTANTE]
> **Completude do retorno (`JOSS-TYPE-010`)**:
> Se uma função declara um tipo de retorno (como `: string`), o analisador semântico requer que **todos os caminhos de execução possíveis** terminem com um `return` do tipo indicado ou com uma exceção `throw`. Você não pode deixar ramificações inacabadas onde a função simplesmente termina sem retornar nada.

---

## 5. Escopo das variáveis (Escopo) e passagem por valor

O **escopo** é a área do programa onde uma variável existe e é acessível.

Em Joss:
1. Cada chamada de função cria um **quadro de memória isolado (quadro)**.
2. Parâmetros e variáveis ​​declarados dentro da função **existem apenas enquanto a função está em execução**. Assim que atinge `return`, eles desaparecem da memória.
3. Funções nomeadas **não têm acesso automático às variáveis ​​globais do arquivo**. Se uma função precisar de dados, você deverá passá-los como parâmetro.
4. **Passagem por valor**: Ao passar um valor primitivo (como um `int` ou `string`) para uma função, Joss fornece uma cópia dele. Modificar essa variável dentro da função **não altera a variável original que estava fora**:<!-- joss-run: ["20", "10"] -->
```joss
public func duplicar(int $valor): int {
    $valor = $valor * 2
    return $valor
}
$numero = 10
print(duplicar($numero))
print($numero)
```
A variável original `$numero` mantém seu valor `10` intacto.

---

## 6. Funções e encerramentos anônimos

Uma função nem sempre precisa de um nome global. Você pode criar uma função como um valor, armazená-la em uma variável ou passá-la como argumento para outra função. Isso é chamado de **função anônima** ou **função de primeira classe**.

Quando uma função anônima usa variáveis ​​que foram criadas no bloco externo ao seu redor, ela se torna um **fechamento**: "capture" e lembra dessas variáveis ​​para uso posterior, mesmo que o bloco externo já tenha terminado:<!-- joss-run: ["Hola, Ada"] -->
```joss
$prefijo = "Hola, "
$saludar = func(string $nombre): string {
    return $prefijo . $nombre
}
print($saludar("Ada"))
```
- Closures são amplamente usados ​​como **callbacks**: funções que você fornece a um serviço (por exemplo, a um servidor HTTP ou a um WebSocket) para ser executado quando ocorre um evento (como a chegada de um cliente).
- Os fechamentos não possuem modificadores de visibilidade (`public` nem `private`).

---

## 7. Referências mutáveis ​​temporárias com `ref`

E se você realmente quiser que uma função modifique a variável original que você passou de fora? Em vez de retornar uma cópia, Joss permite o uso de **referências (`ref`)**.

Por segurança, Joss exige que a intenção seja **bilateral**: tanto a assinatura da função quanto a chamada devem incluir a palavra reservada `ref`:<!-- joss-run: ["2"] -->
```joss
public func incrementar(ref int $valor): int {
    $valor = $valor + 1
    return $valor
}
$contador = 1
incrementar(ref $contador)
print($contador)
```
Agora `$contador` mudou seu valor original para `2`.

### Referência de regras de segurança em Joss:
1. **Somente variáveis simples e mutáveis**: Você não pode passar constantes, expressões matemáticas ou literais (por exemplo, `incrementar(ref 5)` é ilegal com `JOSS-REF-002`).
2. **Invariância estrita de tipo**: O tipo da variável deve corresponder exatamente ao tipo do parâmetro (não é permitido passar um `int` para um `ref float`).
3. **Eles não podem escapar**: Uma referência só vive durante a chamada. Você não pode salvar uma referência a uma variável global, retorná-la com `return` ou enviá-la por um canal assíncrono.

---

## 8. O operador Pipeline (`|>`)

Na programação é muito comum ter que passar um dado por uma série de transformações sucessivas. Nas linguagens clássicas, isso força as chamadas a serem aninhadas de dentro para fora:```joss
// Difícil de leer: debes leer de derecha a izquierda o de adentro hacia afuera
strtoupper(trim("  ada  "))
```
Joss inclui nativamente o operador **Pipeline (`|>`)**, que pega o resultado à esquerda e o envia como primeiro argumento para a função à direita:<!-- joss-run: ["ADA"] -->
```joss
print("  ada  " |> trim |> strtoupper)
```
O fluxo lê naturalmente da esquerda para a direita:
1. Pegue o texto `"  ada  "`.
2. Passe por `trim` (que remove os espaços, resultando em `"ada"`).
3. Passe por `strtoupper` (que converte para maiúsculas, resultando em `"ADA"`).
4. Passe para `print` para exibi-lo na tela.

Se a função receptora precisar de mais de um argumento, escreva-os normalmente entre parênteses:```joss
$resultado = $texto |> str_replace("a", "o")
```
Joss colocará o valor à esquerda como o primeiro parâmetro de `str_replace`.

---

## 9. Funções recursivas

Uma função **recursiva** é aquela que se chama para resolver um problema dividindo-o em versões menores do mesmo problema.

Toda função recursiva deve ter dois componentes essenciais:
1. **Caso base**: Uma condição de parada em que a função retorna um resultado simples sem ser chamada novamente.
2. **Etapa recursiva**: onde a função chama a si mesma com um dado mais próximo do caso base.

Joss protege sua memória limitando a profundidade máxima de chamadas recursivas a 1024 níveis por padrão (consulte [Recursão](RECURSION.md) para obter detalhes).

---

## 10. Erros comuns com funções

| Erro | Código/Causa | Solução |
|---|---|---|
| Escreva `function f()` | `function` foi removido de Joss. | Use `public func f()` ou `private func f()`. |
| Esqueça `public` ou `private` em funções globais | Funções nomeadas requerem visibilidade obrigatória. | Adicione `public func nombre(...)` ou `private func nombre(...)`. |
| Parâmetro não digitado: `func($x)` | Os parâmetros devem ser digitados (`JOSS-TYPE-011`). | Declare o tipo: `func(int $x)` ou `func(mixed $x)`. |
| Função digitada sem retorno em todas as rotas | `JOSS-TYPE-010`: O verificador detectou um caminho que termina sem retornar nada. | Certifique-se de que todas as ramificações ternárias retornem ou gerem um erro. |
| Esqueça `ref` na chamada | Chame `f($x)` quando a função espera `ref T $param` (`JOSS-REF-001`). | Adicione `ref` na chamada: `f(ref $x)`. |

---

## 11. Exercício prático

1. **Calculadora com funções**:
   - Escreva uma função `public func calcularIVA(decimal $subtotal, decimal $tasa = 0.16m): decimal`.
   - A função deve retornar o valor do imposto (`$subtotal * $tasa`).
   - Tente chamá-lo com um único argumento (`calcularIVA(100.0m)`) e depois com uma taxa personalizada de 8% (`calcularIVA(100.0m, 0.08m)`).
   - Mostrar ambos os resultados no console.

---

## Próxima etapa

Agora que você sabe como estruturar código modular com funções seguras, é hora de aprender como manipular coleções de dados complexas: listas de elementos e mapas associativos de valores-chave.

Continue com: [Coleções: Arrays, Mapas e Manipulação de Texto](COLECCIONES.md).
# Funções, escopo de variáveis, closures e referências

Antes: [Coleções: Arrays, Maps e Texto](COLECCIONES.md). Depois: [Tratamento de erros e exceções](ERRORES.md).
Referência técnica: [Sintaxe](SINTAXIS.md), [Recursão](RECURSION.md), [Sistema de tipos](SISTEMA_TIPOS.md).

---

## O que você vai aprender aqui?

À medida que os programas crescem, escrever centenas de instruções seguidas torna-se inadministrável. Se você precisa calcular o total de uma fatura com impostos em dez lugares diferentes da sua aplicação, copiar e colar a mesma fórmula dez vezes é uma receita para o desastre: se a legislação tributária mudar, você teria que procurar e corrigir dez arquivos e, mais cedo ou mais tarde, esquecerá um.

Neste guia você aprenderá:
1. O que é uma **função** e por que ela é o bloco fundamental de construção de software.
2. A diferença crítica entre **parâmetros**, **argumentos** e **impressão (`print`)** versus **retorno (`return`)**, incluindo funções `: void`.
3. Como declarar funções com tipos seguros, valores padrão, argumentos nomeados e desempacotamento (`...`).
4. As regras de visibilidade obrigatória no Joss (`public` e `private`).
5. Como escrever retornos antecipados limpos com **`guard`** e programar ações de limpeza garantidas com **`defer`**.
6. O que é o **escopo de variáveis (scope)** e como funciona o isolamento de memória em cada chamada.
7. O que são as **funções anônimas (closures)** e como usá-las para transformar listas com `map` e `filter`.
8. Como modificar variáveis externas de forma segura mediante **referências (`ref`)**.
9. Como encadear transformações de dados limpas com o operador **pipeline (`|>`)**.

---

## 1. O que é uma função?

Uma **função** é um bloco autônomo de instruções ao qual atribuímos um nome. Pense nela como uma pequena máquina especializada:
1. Recebe matérias-primas (dados de entrada, chamados **argumentos**).
2. Processa a informação em seu interior de forma isolada.
3. Devolve um produto acabado (o resultado, chamado **valor de retorno**).

<!-- joss-run: ["5", "12"] -->
```joss
public func sumar(int $a, int $b): int {
    return $a + $b
}
print(sumar(2, 3))
print(sumar(5, 7))
```

Saída no terminal:

```text
5
12
```

### Anatomia de uma função no Joss:

- `public`: **Modificador de visibilidade**. No Joss, as funções globais exigem obrigatoriamente visibilidade:
  - `public`: A função fica disponível em todo o projeto e outros arquivos podem usá-la sem imports.
  - `private`: A função só pode ser chamada a partir do mesmo arquivo onde foi escrita.
- `func`: Palavra-chave que indica o início da declaração (o Joss não usa `function`).
- `sumar`: O nome da função.
- `(int $a, int $b)`: A lista de **parâmetros**. Define quais tipos de dados a função exige para trabalhar.
- `: int`: O **tipo de retorno**. Declara que tipo de valor a função promete devolver a quem a chamou.
- `{ ... }`: O **corpo** da função, onde são escritas as instruções.
- `return $a + $b`: A instrução `return` finaliza a execução da função imediatamente e envia o resultado de volta ao chamador.

---

## 2. Diferenças conceituais cruciais

### Parâmetros vs Argumentos
- **Parâmetro**: É a variável que você declara na assinatura da função (por exemplo, `$a` e `$b`). É o espaço que espera um valor.
- **Argumento**: É o valor concreto que você entrega no momento de chamar a função (por exemplo, `2` e `3`).

### Retornar (`return`) vs Imprimir (`print`) e o tipo `: void`
Este é um dos tropeços mais comuns para quem está começando a programar:
- `print` é uma ação física: escreve tinta na tela do terminal para que um humano leia. O restante do programa **não pode reutilizar essa saída**.
- `return` é uma ação de memória interna: entrega o dado calculado para a linha que chamou a função para que ela possa continuar operando com ele (guardá-lo em uma variável, salvá-lo em um banco de dados, enviá-lo pela internet, etc.).

E se a sua função apenas realiza uma ação (como mostrar um cabeçalho na tela ou salvar um registro) e não precisa devolver nenhum dado? Nesse caso, você declara seu tipo de retorno como **`: void`** (vazio):

<!-- joss-run: ["=== REPORTE DE VENTAS ==="] -->
```joss
public func mostrarEncabezado(string $titulo): void {
    print("=== " . $titulo . " ===")
}

mostrarEncabezado("REPORTE DE VENTAS")
```

Saída no terminal:

```text
=== REPORTE DE VENTAS ===
```

Uma função `: void` não devolve valor e pode terminar sem `return` ou com um `return` vazio para sair antecipadamente.

---

## 3. Contratos de tipos e valores padrão

No Joss, **todo parâmetro fonte deve ter um tipo de dado explícito** (`JOSS-TYPE-011`). Se uma função realmente precisar aceitar qualquer valor dinâmico, você deve indicar isso escrevendo `mixed`.

### Parâmetros com valores padrão (Defaults)

Você pode tornar certos argumentos opcionais atribuindo a eles um valor padrão com `=`:

<!-- joss-run: ["Hola, visitante", "Hola, Ada"] -->
```joss
public func saludo(string $nombre = "visitante"): string {
    return "Hola, " . $nombre
}
print(saludo())
print(saludo("Ada"))
```

Saída no terminal:

```text
Hola, visitante
Hola, Ada
```

- Se você chamar `saludo()` sem argumentos, o Joss utilizará automaticamente `"visitante"`.
- Se você chamar `saludo("Ada")`, o valor entregue substitui o valor padrão.

> [!TIP]
> Coloque sempre os parâmetros com valor padrão no final da lista de parâmetros. Caso contrário, o Joss não saberia a qual parâmetro atribuir um argumento se você passar apenas um.

### Argumentos nomeados (Named Arguments)

Você pode passar argumentos indicando explicitamente o nome do parâmetro seguido de dois pontos (`nombre: valor`). Isso permite omitir parâmetros intermediários que tenham valores padrão ou passar argumentos em qualquer ordem:

<!-- joss-run: ["Estimada Ada"] -->
```joss
public func bienvenida(string $nombre, string $titulo = "Estimado/a"): string {
    return $titulo . " " . $nombre
}

print(bienvenida(titulo: "Estimada", nombre: "Ada"))
```

Saída no terminal:

```text
Estimada Ada
```

### Operador Spread (`...`) em chamadas

Se você tem uma lista (que conheceu no capítulo anterior) e deseja desempacotar seus elementos como argumentos posicionais independentes para uma chamada de função, anteponha `...`:

<!-- joss-run: ["10"] -->
```joss
public func sumarTres(int $a, int $b, int $c): int {
    return $a + $b + $c
}

var $numeros = [2, 3, 5]
print(sumarTres(...$numeros))
```

Saída no terminal:

```text
10
```

---

## 4. Retorno antecipado (`guard`) e limpeza adiada (`defer`)

### Cláusulas de guarda com `guard` ou ternário

A instrução `return` interrompe a função imediatamente. Para validar que os dados de entrada atendem a uma condição antes de continuar, o Joss oferece a instrução dedicada **`guard (condición) : { ... }`** (além do ternário tradicional):

<!-- joss-run: ["agotado", "disponible (2 unidades)"] -->
```joss
public func disponibilidad(int $cantidad): string {
    guard ($cantidad > 0) : {
        return "agotado"
    }
    return "disponible (${cantidad} unidades)"
}
print(disponibilidad(0))
print(disponibilidad(2))
```

Saída no terminal:

```text
agotado
disponible (2 unidades)
```

Como se lê `guard ($cantidad > 0) : { return "agotado" }`?
- **"Certifique-se de que `$cantidad > 0`. Caso contrário (`:`), execute este bloco de saída imediata"**.
- Você também pode expressá-lo com um ternário: `($cantidad <= 0) ? { return "agotado" } : {}`.
- Esse padrão é chamado de **cláusula de guarda**. Ele evita criar estruturas condicionais profundamente aninhadas (*código espaguete*).

> [!IMPORTANT]
> **Exaustividade de retorno (`JOSS-TYPE-010`)**:
> Se uma função declara um tipo de retorno diferente de `void` (como `: string`), o analisador semântico exige que **todas as rotas possíveis** de execução terminem com um `return` do tipo indicado ou com uma exceção `throw`.

### Execução adiada ao sair com `defer`

Às vezes uma função abre um recurso ou inicia uma operação e você quer garantir que, aconteça o que acontecer, uma ação de fechamento seja executada pouco antes de a função terminar. Para isso existe **`defer { ... }`**:

<!-- joss-run: ["1. Procesando pago", "2. Cerrando sesión de auditoría", "OK"] -->
```joss
public func procesarPago(): string {
    defer {
        print("2. Cerrando sesión de auditoría")
    }
    print("1. Procesando pago")
    return "OK"
}

print(procesarPago())
```

Saída no terminal:

```text
1. Procesando pago
2. Cerrando sesión de auditoría
OK
```

O bloco `defer` é registrado no início, mas o Joss **adia sua execução** até o instante exato em que a função está prestes a retornar (mesmo que haja vários `return` em diferentes ramificações). Se você declarar vários `defer`, eles serão executados em ordem inversa (o último registrado é executado primeiro, ordem LIFO).

---

## 5. Escopo de variáveis (Scope) e passagem por valor

O **escopo (scope)** é a região do programa onde uma variável existe e é acessível.

No Joss:
1. Cada chamada a uma função cria um **quadro de memória isolado (frame)**.
2. Os parâmetros e as variáveis declaradas dentro da função **só existem enquanto a função está sendo executada**. Assim que chega ao `return`, desaparecem da memória.
3. As funções nomeadas **não têm acesso automático às variáveis de nível superior do arquivo**. Se uma função precisa de um dado, você deve passá-lo como parâmetro.
4. **Passagem por valor**: Ao passar um valor primitivo (como um `int` ou `string`) para uma função, o Joss entrega uma cópia. Modificar essa variável dentro da função **não altera a variável original que estava fora**:

<!-- joss-run: ["20", "10"] -->
```joss
public func duplicar(int $valor): int {
    $valor = $valor * 2
    return $valor
}
var $numero = 10
print(duplicar($numero))
print($numero)
```

Saída no terminal:

```text
20
10
```

A variável original `$numero` conserva seu valor `10` intacto.

---

## 6. Funções anônimas e Closures

Uma função nem sempre precisa de um nome global. Você pode criar uma função como se fosse um valor, guardá-la em uma variável ou passá-la como argumento para outra função. Isso é chamado de **função anônima** ou **função de primeira classe**.

Quando uma função anônima utiliza variáveis que foram criadas no bloco externo ao seu redor, ela se torna uma **closure**: "captura" e lembra dessas variáveis para utilizá-las mais tarde:

<!-- joss-run: ["Hola, Ada"] -->
```joss
var $prefijo = "Hola, "
var $saludar = func(string $nombre): string {
    return $prefijo . $nombre
}
print($saludar("Ada"))
```

Saída no terminal:

```text
Hola, Ada
```

- As closures não levam modificadores de visibilidade (`public` nem `private`).
- São ideais para transformar ou filtrar coleções com as funções nativas `map` e `filter`:

<!-- joss-run: ["[20 40 60]"] -->
```joss
var $precios = [10, 20, 30]
var $dobles = map($precios, func(int $p): int {
    return $p * 2
})
print($dobles)
```

Saída no terminal:

```text
[20 40 60]
```

---

## 7. Referências mutáveis temporárias com `ref`

O que acontece se você realmente quiser que uma função modifique a variável original que você passou de fora? Em vez de devolver uma cópia, o Joss permite o uso de **referências (`ref`)**.

Por segurança, o Joss exige que a intenção seja **bilateral**: tanto a assinatura da função quanto a chamada devem incluir a palavra reservada `ref`:

<!-- joss-run: ["2"] -->
```joss
public func incrementar(ref int $valor): int {
    $valor = $valor + 1
    return $valor
}
var $contador = 1
incrementar(ref $contador)
print($contador)
```

Saída no terminal:

```text
2
```

Agora `$contador` realmente mudou seu valor original para `2`.

### Regras de segurança das referências no Joss:
1. **Apenas variáveis simples e mutáveis**: Você não pode passar constantes, expressões matemáticas nem literais (por exemplo, `incrementar(ref 5)` é ilegal com `JOSS-REF-002`).
2. **Invariância estrita de tipos**: O tipo da variável deve coincidir exatamente com o tipo do parâmetro (não é permitido passar um `int` para um `ref float`).
3. **Não podem escapar**: Uma referência só vive durante a chamada. Você não pode guardar uma referência em uma variável global, nem devolvê-la com `return`, nem enviá-la para uma função `async`.

---

## 8. O operador Pipeline (`|>`)

Na programação é muito comum ter que passar um dado por uma série de transformações sucessivas. Em linguagens clássicas, isso obriga a aninhar chamadas de dentro para fora:

```joss
// Difícil de leer: debes leer de derecha a izquierda o de adentro hacia afuera
strtoupper(trim("  ada  "))
```

O Joss inclui de forma nativa o operador **Pipeline (`|>`)**, que pega o resultado da esquerda e o envia como primeiro argumento para a função da direita:

<!-- joss-run: ["ADA"] -->
```joss
print("  ada  " |> trim |> strtoupper)
```

Saída no terminal:

```text
ADA
```

O fluxo é lido de forma natural da esquerda para a direita:
1. Pegue o texto `"  ada  "`.
2. Passe-o por `trim` (que remove os espaços, resultando em `"ada"`).
3. Passe-o por `strtoupper` (que converte para maiúsculas, resultando em `"ADA"`).
4. Passe-o para `print` para exibi-lo na tela.

Você também pode combinar pipelines com funções personalizadas, coleções e closures para criar fluxos de processamento de dados muito expressivos:

<!-- joss-run: ["Precio: 25 USD"] -->
```joss
public func formatearMoneda(int $monto, string $moneda = "USD"): string {
    return "Precio: ${monto} ${moneda}"
}

var $resultado = 25 |> formatearMoneda
print($resultado)
```

Saída no terminal:

```text
Precio: 25 USD
```

E com listas usando `filter`:

<!-- joss-run: ["60"] -->
```joss
var $ventas = [5, 10, 20, 30]
var $altas = $ventas |> filter(func(int $x): bool { return $x >= 10 })
print(sum($altas))
```

Saída no terminal:

```text
60
```

---

## 9. Funções recursivas

Uma função **recursiva** é aquela que chama a si mesma para resolver um problema dividindo-o em versões menores do mesmo problema.

Toda função recursiva deve ter dois componentes indispensáveis:
1. **Caso base**: Uma condição de parada onde a função retorna um resultado simples sem voltar a se chamar.
2. **Passo recursivo**: Onde a função chama a si mesma com um dado mais próximo do caso base.

O Joss protege sua memória limitando a profundidade máxima de chamadas recursivas a 1024 níveis por padrão (consulte [Recursão](RECURSION.md) para mais detalhes).

---

## 10. Erros comuns com funções

| Erro | Código / Causa | Solução |
|---|---|---|
| Escrever `function f()` | `function` foi removida do Joss. | Use `public func f()` ou `private func f()`. |
| Esquecer `public` ou `private` em funções globais | As funções nomeadas exigem visibilidade obrigatória. | Adicione `public func nombre(...)` ou `private func nombre(...)`. |
| Parâmetro sem tipo: `func($x)` | Os parâmetros devem ser tipados (`JOSS-TYPE-011`). | Declare o tipo: `func(int $x)` ou `func(mixed $x)`. |
| Função tipada sem return em todas as rotas | `JOSS-TYPE-010`: O verificador detectou um caminho que termina sem devolver nada. | Certifique-se de que todas as ramificações ternárias retornem ou lancem um erro. |
| Esquecer `ref` na chamada | Chamar `f($x)` quando a função espera `ref T $param` (`JOSS-REF-001`). | Adicione `ref` na chamada: `f(ref $x)`. |

---

## 11. Exercícios práticos

### Nível 1 — Teste rápido
Escreva uma função `public func presentar(string $nombre, string $rol = "Estudiante"): string` que retorne `"${nombre} (${rol})"`. Chame-a primeiro passando apenas `"Ada"` e depois passando `"Grace"` e `"Arquitecta"`. Imprima ambos os resultados.

### Nível 2 — Exercício guiado
1. Escreva uma função `public func calcularIVA(decimal $subtotal, decimal $tasa = 0.16m): decimal`.
2. Use uma cláusula `guard ($subtotal > 0.0m) : { return 0.0m }` para evitar calcular impostos sobre valores zerados ou negativos.
3. Se o subtotal for válido, retorne `$subtotal * $tasa`.
4. Teste chamá-la com `0.0m`, `100.0m` (usando a taxa padrão) e `100.0m` com taxa `0.08m`.

### Nível 3 — Desafio
Dada a lista `var $precios = [50, 120, 80, 200, 30]`:
1. Utilize o operador pipeline (`|>`) junto com `filter` para ficar apenas com os preços maiores ou iguais a `100`.
2. Encadeie outro `|>` com `map` para aplicar um desconto subtraindo `15` de cada preço filtrado.
3. Encadeie `|> sum` no final e imprima a soma total resultante (deve imprimir `290`).

---

## Próximo passo

Agora que você sabe como organizar sua lógica em funções limpas e reutilizáveis, aprenderá como detectar, lançar e se recuperar de situações imprevistas sem que seu programa entre em colapso.

Continue com: [Tratamento de erros, exceções e diagnóstico](ERRORES.md).
# Coleções: Arrays, Maps e Manipulação de Texto Unicode

Antes: [Controle de fluxo e decisões](CONTROL_FLUJO.md). Depois: [Funções, escopo e pipelines](FUNCIONES.md).
Referência técnica: [Módulos nativos](MODULOS_NATIVOS.md), [Funções globais](FUNCIONES_GLOBALES.md).

---

## O que você vai aprender aqui?

Até agora trabalhamos com variáveis que armazenam um único dado por vez: um número, um nome ou um booleano. Mas na vida real os dados quase nunca vêm isolados:
- Uma lista de produtos em uma loja online.
- Os comentários de uma publicação.
- O cadastro de um usuário com seu nome, e-mail, telefone e endereço.

Para agrupar e organizar múltiplos dados em uma única estrutura, existem as **coleções**.

Neste guia você aprenderá passo a passo:
1. O que é um **array**, como funciona a numeração a partir do zero (**índices**) e como adicionar elementos.
2. O que é um **map** (dicionário chave → valor) com chaves `{}` ou estilo associativo `["clave" => valor]`.
3. Como percorrer coleções com `foreach` extraindo tanto a posição ou chave quanto o valor (`$clave => $valor`).
4. Como desempacotar vários valores de uma só vez mediante **desestruturação** (`[$a, $b] = ...`).
5. Como funcionam as coleções na memória (cópia vs referência compartilhada) e funções úteis como `count`, `merge`, `explode` e `implode`.
6. Como o Joss lida com o texto moderno Unicode (letras acentuadas e emojis) e como converter coleções para **JSON**.

---

## 1. Arrays: Listas ordenadas de elementos

Um **array** é uma lista ordenada de valores delimitados por colchetes `[` e `]`. Cada valor ocupa uma posição numerada chamada **índice**.

No Joss (e na maioria das linguagens de programação), **os índices começam a contar a partir do zero (`0`)**, e não do um:
- O primeiro elemento está no índice `0`.
- O segundo elemento está no índice `1`.
- O terceiro elemento está no índice `2`.

<!-- joss-run: ["pan", "3", "fruta"] -->
```joss
var $compras = ["pan", "leche"]
print($compras[0])
$compras[] = "fruta"
print(count($compras))
print($compras[2])
```

Saída:

```text
pan
3
fruta
```

### Operações fundamentais com arrays:

1. **Criação**: São escritos entre colchetes `[` e `]`, separando os elementos com vírgulas: `["pan", "leche"]`. Uma lista vazia é escrita `[]`.
2. **Leitura por índice**: `$compras[0]` obtém o primeiro elemento (`"pan"`).
3. **Modificar uma posição**: `$compras[1] = "queso"` altera o segundo elemento.
4. **Adicionar ao final com `[]`**: Escrever `$compras[] = "fruta"` adiciona o novo elemento ao final da lista automaticamente.
5. **Contar elementos**: `count($compras)` (ou `len($compras)`) retorna quantos elementos a lista possui (`3`).
6. **Proteção de limites**: Se você tentar ler uma posição que não existe (como `$compras[99]` ou `$compras[-1]`), o Joss interrompe a operação imediatamente com o erro `JOSS-INDEX-001` (*Index Out of Range*), evitando leituras inválidas de memória.

### Arrays com tipo fixo de elemento (`array<Tipo>`)
Por padrão, um `array` permite guardar diferentes tipos de dados. Se você quiser exigir que todos os elementos de uma lista sejam números inteiros, pode declará-la com `array<int>`:

```joss
array<int> $edades = [18, 25, 30]
```

### Expandir uma lista dentro de outra com `...` (Spread)

Você pode inserir todos os elementos de uma lista existente dentro de uma lista nova antepondo três pontos `...`:

<!-- joss-run: ["1", "2", "3", "4"] -->
```joss
var $primeros = [2, 3]
var $todos = [1, ...$primeros, 4]
print($todos[0])
print($todos[1])
print($todos[2])
print($todos[3])
```

Saída:

```text
1
2
3
4
```

---

## 2. Maps: Dicionários de Chave → Valor

Um array é ideal quando apenas a ordem importa (como uma fila de atendimento). Mas se você quiser guardar a ficha de uma pessoa com seu nome, idade e e-mail, lembrar que "o nome está no 0 e a idade no 1" é incômodo.

Para isso existem os **maps** (dicionários ou mapas associativos). Em um map, cada dado é guardado sob um **rótulo de texto chamado chave**:

<!-- joss-run: ["Ada", "21", "sin teléfono"] -->
```joss
var $persona = {"nombre": "Ada", "edad": 20}
print($persona["nombre"])
$persona["edad"] = 21
print($persona["edad"])
print($persona["telefono"] ?? "sin teléfono")
```

Saída:

```text
Ada
21
sin teléfono
```

### Características dos Maps no Joss:
1. **Criação com chaves `{}`**: Associa-se cada chave de texto ao seu valor usando dois pontos: `{"nombre": "Ada", "edad": 20}`. Um mapa vazio é escrito `{}`.
2. **Leitura e modificação**: Escreve-se o nome da chave entre colchetes e aspas: `$persona["nombre"]`.
3. **Chaves que não existem**: Se você consultar uma chave que ainda não foi salva (como `$persona["telefono"]`), o Joss retorna `null` sem falhar. Combiná-lo com `??` permite exibir um texto alternativo limpo.
4. **Verificar se uma chave existe**: A função integrada `array_key_exists("telefono", $persona)` retorna `true` ou `false`.

### Notação alternativa com colchetes e seta (`=>`)

Além das chaves `{}`, o Joss também aceita criar mapas usando colchetes `[]` e a seta dupla `=>` (muito familiar se você conhece PHP):

<!-- joss-run: ["JosSecurity", "v1.0", "Ada"] -->
```joss
var $config = [
    "app" => "JosSecurity",
    "version" => "v1.0",
    "autor" => ["nombre" => "Ada"]
]
print($config["app"])
print($config["version"])
print($config["autor"]["nombre"])
```

Saída:

```text
JosSecurity
v1.0
Ada
```

### Combinar mapas com `...` (Spread)

Assim como nos arrays, você pode mesclar as chaves de um mapa dentro de outro novo usando `...`:

<!-- joss-run: ["localhost", "8080", "true"] -->
```joss
var $base = ["host" => "localhost", "puerto" => "8080"]
var $completo = [...$base, "seguro" => "true"]
print($completo["host"])
print($completo["puerto"])
print($completo["seguro"])
```

Saída:

```text
localhost
8080
true
```

---

## 3. Percorrer Índices e Chaves com `foreach ($coleccion as $clave => $valor)`

No capítulo anterior vimos como percorrer os valores de uma lista. Se você também precisa saber **em qual posição ou sob qual chave** está cada valor, escreva `$clave => $valor` dentro do `foreach`:

<!-- joss-run: ["0: manzana", "1: pera", "a => alfa", "b => beta"] -->
```joss
var $frutas = ["manzana", "pera"]
foreach ($frutas as $indice => $fruta) {
    print($indice . ": " . $fruta)
}

var $letras = ["a" => "alfa", "b" => "beta"]
foreach ($letras as $k => $v) {
    print($k . " => " . $v)
}
```

Saída:

```text
0: manzana
1: pera
a => alfa
b => beta
```

Além disso, você dispõe de duas funções diretas para extrair as partes de um mapa:
- `keys($mapa)`: Retorna um array com todas as chaves.
- `values($mapa)`: Retorna um array com todos os valores.

---

## 4. Desestruturação de Arrays e Maps

Quando você quer extrair vários valores de um array ou de um map e guardá-los em variáveis individuais em uma única linha, pode usar a **desestruturação**, incluindo valores padrão caso algum dado esteja faltando:

<!-- joss-run: ["10", "20", "30", "Ada", "cliente"] -->
```joss
[$x, $y, $z = 30] = [10, 20]
print($x)
print($y)
print($z)

{"nombre": $nombre, "rol": $rol = "cliente"} = {"nombre": "Ada"}
print($nombre)
print($rol)
```

Saída:

```text
10
20
30
Ada
cliente
```

> [!NOTE]
> Por compatibilidade, o Joss também aceita a forma com parênteses `let ($x, $y) = [10, 20]`, embora a forma com colchetes `[$x, $y] = [10, 20]` seja mais expressiva e suporte valores padrão.

---

## 5. Métodos diretos sobre textos, listas e mapas (`->`)

No Joss, os textos (`string`), listas (`array`) e mapas (`map`) trazem **ferramentas integradas (métodos)** que você pode invocar escrevendo uma seta fina **`->`** após a variável:

<!-- joss-run: ["hola-mundo", "a-b"] -->
```joss
var $txt = "  Hola Mundo  "
print($txt->trim()->lower()->replace(" ", "-"))

var $mapa = {"a": 1, "b": 2}
print($mapa->keys()->join("-"))
```

Saída:

```text
hola-mundo
a-b
```

Observe como se lê da esquerda para a direita:
1. `$txt->trim()`: Remove os espaços excedentes das extremidades (`"Hola Mundo"`).
2. `->lower()`: Converte as letras para minúsculas (`"hola mundo"`).
3. `->replace(" ", "-")`: Troca o espaço por um hífen (`"hola-mundo"`).

### Processamento funcional de listas

O Joss também permite filtrar, transformar e somar listas inteiras com funções como `sum`, `filter`, `map`, `find`, `any` e `all` (que exploraremos em profundidade no próximo capítulo de [Funções](FUNCIONES.md)):

<!-- joss-run: ["60", "20", "true"] -->
```joss
var $numeros = [1, 2, 3, 4, 5]
var $pares = $numeros |> filter(func(int $x): bool { return $x % 2 == 0; })
var $escalados = $pares |> map(func(int $x): int { return $x * 10; })
print(sum($escalados))
var $encontrado = $numeros |> find(func(int $x): bool { return $x == 2; })
print($encontrado * 10)
print($numeros |> any(func(int $x): bool { return $x == 3; }))
```

<!-- joss-run: ["hola-mundo", "6, 8", "a-b"] -->
```joss
var $txt = "  Hola Mundo  "
print($txt->trim()->lower()->replace(" ", "-"))

var $nums = [1, 2, 3, 4]
var $filtrados = $nums->map(func(int $n, int $i): int { return $n * 2; })->filter(func(int $n, int $i): bool { return $n > 4; })
print($filtrados->join(", "))

var $mapa = {"a": 1, "b": 2}
print($mapa->keys()->join("-"))
```

---

## 6. Comportamento na memória: Cópia independente ou lista compartilhada?

Há uma diferença muito importante entre os dados simples e as coleções:

- Quando você atribui um número ou um texto a outra variável (`var $b = $a`), cria-se uma **cópia independente**.
- Em contrapartida, quando você atribui um **array** ou um **map** a outra variável (`var $copia = $original`), **ambas as variáveis apontam para a mesma lista na memória**:

<!-- joss-run: ["9", "nuevo"] -->
```joss
var $original = [1, 2]
var $copia = $original
$copia[0] = 9
print($original[0])

var $datos = {"estado": "inicial"}
var $alias = $datos
$alias["estado"] = "nuevo"
print($datos["estado"])
```

Saída:

```text
9
nuevo
```

Alterar `$copia[0]` também alterou `$original[0]` porque ambas as variáveis olham para a mesma lista.

> [!TIP]
> **Como criar uma cópia separada de um array?**
> Se você quiser duplicar uma lista para modificar a nova sem afetar a original, use `merge`:

```joss
var $clon = merge([], $original)
```

### Funções de extremidades em listas (`array_pop`, `array_shift`, `array_push`)

Tenha em mente como funcionam estas três funções no Joss:

<!-- joss-run: ["2", "2", "3"] -->
```joss
var $numeros = [1, 2]
print(array_pop($numeros))
print(count($numeros))
$numeros = array_push($numeros, 3)
print(count($numeros))
```

Saída:

```text
2
2
3
```

1. `array_pop($arr)`: Retorna o último elemento sem apagá-lo da lista original.
2. `array_shift($arr)`: Retorna o primeiro elemento sem apagá-lo.
3. `array_push($arr, $item)` (ou `append($arr, $item)`): Retorna uma nova lista com o elemento adicionado, pelo que você pode reatribuí-la (`$numeros = array_push($numeros, 3)`) ou usar diretamente `$numeros[] = 3`.

---

## 7. Texto Unicode: Letras visíveis vs Bytes

Nos idiomas modernos usamos acentos (`á`, `é`), caracteres especiais (`ñ`) e emojis (`🚀`). Na memória de um computador, um único símbolo visível pode ocupar vários bytes.

Veja como o Joss permite medir exatamente o que você precisa:

<!-- joss-run: ["3", "2", "é"] -->
```joss
var $texto = "é"
print(len($texto))
print(strlen($texto))
print($texto[0])
```

Saída:

```text
3
2
é
```

| Operação | O que mede ou retorna | Quando usá-la |
|---|---|---|
| `$texto[0]` | O **caractere visível completo** (*grafema*) | Quando você trabalha com nomes de pessoas ou mensagens com acentos e emojis, sem parti-los ao meio. |
| `strlen($texto)` | Quantidade de **pontos de código Unicode** | Para contagem padrão de caracteres Unicode. |
| `len($texto)` | Quantidade de **bytes na memória (UTF-8)** | Para tamanhos de arquivo ou envios pela rede. |

---

## 8. Converter coleções para formato JSON (`json_encode` e `json_decode`)

**JSON** é o formato de texto universal para salvar dados em arquivos ou enviá-los pela internet. O Joss converte qualquer array ou map para JSON e vice-versa com uma única instrução:

### Com mapas usando chaves `{}`:
<!-- joss-run: ["Ada", "Joss"] -->
```joss
var $perfil = {
    "nombre": "Ada",
    "lenguajes": ["Joss", "Go"]
}
var $jsonMapa = json_encode($perfil)
var $datosMapa = json_decode($jsonMapa)
print($datosMapa["nombre"])
print($datosMapa["lenguajes"][0])
```

Saída:

```text
Ada
Joss
```

### Com mapas usando colchetes `[]` e `=>`:
<!-- joss-run: ["Carlos", "admin"] -->
```joss
var $usuario = [
    "nombre" => "Carlos",
    "roles" => ["admin", "editor"]
]
var $jsonArray = json_encode($usuario)
var $datosArray = json_decode($jsonArray)
print($datosArray["nombre"])
print($datosArray["roles"][0])
```

Saída:

```text
Carlos
admin
```

- **`json_encode($datos)`**: Converte seu array ou map em uma string de texto JSON.
- **`json_decode($textoJson)`**: Reconstrói o texto JSON em arrays e maps do Joss prontos para uso.
- **`json_verify($textoJson)`**: Retorna `true` se o texto for um JSON válido ou `false` se estiver malformado.

---

## 9. Resumo de funções essenciais para coleções e texto

| Função | Propósito | Exemplo | Resultado |
|---|---|---|---|
| `count($col)` / `len($col)` | Quantidade de elementos de um array ou map | `count([10, 20])` | `2` |
| `in_array($val, $arr)` | O valor existe dentro do array? | `in_array(2, [1, 2, 3])` | `true` |
| `keys($map)` | Lista com todas as chaves do map | `keys({"a": 1})` | `["a"]` |
| `values($map)` | Lista com todos os valores do map | `values({"a": 1})` | `[1]` |
| `explode($sep, $str)` | Divide um texto em um array usando um separador | `explode(",", "a,b,c")` | `["a", "b", "c"]` |
| `implode($sep, $arr)` | Une um array de textos com um separador | `implode("-", ["2026", "10", "17"])` | `"2026-10-17"` |
| `array_reverse($arr)` | Inverte a ordem de um array | `array_reverse([1, 2, 3])` | `[3, 2, 1]` |

---

## 10. Prática do capítulo

### 1. Teste rápido
No exemplo da seção 1:
1. Adicione um quarto produto a `$compras` escrevendo `$compras[] = "café"`.
2. Exiba na tela o primeiro elemento (`$compras[0]`) e o total de produtos com `count($compras)`.

### 2. Exercício guiado: Ficha de estoque com um Map
1. Crie em seu `main.joss` um mapa chamado `$producto` com três chaves: `"nombre": "Laptop"`, `"precio": 1200.00m` e `"stock": 5`.
2. Exiba seus dados: `print("Producto: " . $producto["nombre"] . " | Stock: " . $producto["stock"])`.
3. Simule uma venda subtraindo `1` de `$producto["stock"]` (`$producto["stock"] = $producto["stock"] - 1`).
4. Converta `$producto` para JSON com `print(json_encode($producto))`.

### 3. Desafio: Gerenciador de lista de tarefas
1. Crie um array `var $tareas = ["Instalar Joss", "Crear proyecto"]`.
2. Adicione mais duas tarefas usando `$tareas[] = ...`.
3. Percorra a lista com `foreach ($tareas as $indice => $tarea)` e imprima cada tarefa numerada a partir de `1` (dica: use `${$indice + 1}: ${tarea}`).

---

## Próximo passo

Agora que você sabe representar listas, dicionários e JSON na memória, aprenderemos a organizar nosso código em peças reutilizáveis, limpas e com nome próprio: **as funções**.

Continue com: [Funções, escopo de variáveis (scope), `guard`, `defer`, closures e pipelines](FUNCIONES.md).
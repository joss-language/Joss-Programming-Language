# Coleções: Arrays, Mapas e Manipulação de Texto Unicode Antes: [Funções e fechamentos](FUNCIONES.md). Depois: [Digite sistema e inferência](SISTEMA_TIPOS.md). Referência técnica: [Módulos nativos](MODULOS_NATIVOS.md), [Funções globais](FUNCIONES_GLOBALES.md). --- ## O que você vai aprender aqui? Até agora trabalhamos com variáveis ​​que armazenam um único dado por vez: um número, um nome ou um booleano. Mas na vida real os dados quase nunca vêm isolados: - Uma lista de produtos numa loja online. - Comentários sobre uma publicação. - Um arquivo do usuário com nome, e-mail, telefone e endereço. Para agrupar e organizar vários dados em uma única estrutura, existem **coleções**. Neste guia você aprenderá: 1. O que é um **array**, como funciona a numeração do zero (índices) e como adicionar elementos. 2. O que é um **mapa** (dicionário de valores-chave) com sintaxe de chaves `{}` ou estilo associativo PHP `["clave" => valor]`. 3. Como percorrer coleções com `foreach` extraindo pares `$clave => $valor`. 4. Como processar listas com funções funcionais e pipelines: `map`, `filter`, `reduce`, `find`, `any`, `all`, `sum`. 5. Como as coleções se comportam na memória: cópia de referências vs duplicação de dados. 6. As peculiaridades de funções como `array_pop`, `array_push` e `array_shift` em Joss. 7. Texto como coleção: a diferença fundamental entre **bytes**, **pontos de código Unicode** e **grafemas (caracteres visíveis)**. --- ## 1. Matrizes: sequências ordenadas de elementos Uma **matriz** é uma lista ordenada de valores. Cada valor ocupa uma caixa numerada chamada **índice**. Em Joss (e na grande maioria das linguagens modernas), **os índices começam a contar do zero (`0`)**, e não de um: - O primeiro elemento está no índice `0`. - O segundo elemento está no índice `1`. - O terceiro elemento está no índice `2`.<!-- joss-run: ["pan", "3", "fruta"] -->
```joss
$compras = ["pan", "leche"]
print($compras[0])
$compras[] = "fruta"
print(count($compras))
print($compras[2])
```
### Operações básicas com arrays: 1. **Criação**: São delimitados por colchetes `[` e `]`, separando os elementos por vírgulas: `["pan", "leche"]`. 2. **Ler por índice**: `$compras[0]` acessa o primeiro elemento (`"pan"`). 3. **Adicionar ao final com `[]`**: Digitar `$compras[] = "fruta"` adiciona automaticamente o novo elemento ao final da lista. 4. **Contar Elementos**: `count($compras)` (ou `len($compras)`) retorna o número total de elementos (neste caso, `3`). 5. **Proteção de limite**: Se você tentar acessar um índice que não existe (por exemplo `$compras[99]` ou um índice negativo `$compras[-1]`), Joss interromperá a execução imediatamente com o erro de segurança `JOSS-INDEX-001` (Índice fora do intervalo), protegendo seu programa contra a leitura de memória indesejada. ### Digitação de coleção: matrizes homogêneas Por padrão, uma matriz `array` pode conter tipos mistos. Se quiser garantir que todos os elementos sejam inteiros, você pode usar a sintaxe parametrizada:```joss
array<int> $edades = [18, 25, 30]
```
### Desestruturação Declarativa de Arrays e Mapas Quando você tem um array ou um mapa e precisa extrair seus elementos em variáveis ​​separadas, você não precisa escrever atribuições individuais repetitivas. Você pode descompactá-los diretamente em uma única linha usando **desestruturação declarativa**, incluindo valores padrão opcionais:<!-- joss-run: ["10", "20", "30", "Ada", "cliente"] -->
```joss
[$x, $y, $z = 30] = [10, 20]
print($x)
print($y)
print($z)

{"nombre": $nombre, "rol": $rol = "cliente"} = {"nombre": "Ada"}
print($nombre)
print($rol)
```
Isto é muito conveniente para descompactar parâmetros de solicitação da web, pares de coordenadas ou resultados retornados por funções sem código cerimonial. ### Operador Spread (`...`) em arrays Você pode expandir os elementos de um array existente em um novo array acrescentando `...`:<!-- joss-run: ["1", "2", "3", "4"] -->
```joss
$primeros = [2, 3]
$todos = [1, ...$primeros, 4]
print($todos[0])
print($todos[1])
print($todos[2])
print($todos[3])
```
--- ## 2. Mapas: Chave → Dicionários de Valores Um array é perfeito quando a ordem dos elementos é importante (como uma lista de espera). Mas se você deseja representar uma entidade com propriedades marcadas (como um usuário), lembrar que "o nome está no índice 0 e o correio está no índice 1" é frágil e confuso. É por isso que existem **mapas** (também conhecidos como dicionários, tabelas hash ou mapas associativos). Em um mapa, cada valor é salvo e recuperado usando uma **chave de texto**:<!-- joss-run: ["Ada", "21", "sin teléfono"] -->
```joss
$persona = {"nombre": "Ada", "edad": 20}
print($persona["nombre"])
$persona["edad"] = 21
print($persona["edad"])
print($persona["telefono"] ?? "sin teléfono")
```
### Características dos Mapas no Joss: 1. **Criação**: São delimitados com colchetes `{}` associando com `:` (`{"clave": valor}`) ou com colchetes `[]` associando com `=>` (`["clave" => valor]`). 2. **Mapa vazio**: `{}` cria um mapa vazio. 3. **Acesso e modificação**: Os colchetes são usados ​​com o nome da chave entre aspas: `$persona["nombre"]`. 4. **Chaves inexistentes**: Se você tentar ler uma chave que não existe (como `$persona["telefono"]`), Joss retornará `null` com segurança em vez de falhar. Você pode usar o operador `??` para fornecer um valor padrão elegante. 5. **Verificação de existência**: Você pode usar `array_key_exists("telefono", $persona)` para saber com certeza se uma chave foi definida, mesmo que seu valor associado seja `null`. ### Notação associativa no estilo PHP (`=>`) Joss suporta a notação clássica de chaves `{}` com dois pontos `:` e os colchetes e a notação de seta grossa `=>`, incluindo mapas multidimensionais aninhados:<!-- joss-run: ["JosSecurity", "v1.0", "Ada"] -->
```joss
$config = [
    "app" => "JosSecurity",
    "version" => "v1.0",
    "autor" => ["nombre" => "Ada"]
]
print($config["app"])
print($config["version"])
print($config["autor"]["nombre"])
```
### Operador Spread (`...`) em mapas Assim como em arrays, você pode espalhar e mesclar pares de valores-chave de um mapa em outro usando o operador `...`:<!-- joss-run: ["localhost", "8080", "true"] -->
```joss
$base = ["host" => "localhost", "puerto" => "8080"]
$completo = [...$base, "seguro" => "true"]
print($completo["host"])
print($completo["puerto"])
print($completo["seguro"])
```
--- ## 3. Percorrer coleções com `foreach ($coleccion as $clave => $valor)` Você pode percorrer arrays e mapas associativos acessando diretamente a chave (ou índice numérico em arrays) e o valor usando a sintaxe `$clave => $valor`:<!-- joss-run: ["0: manzana", "1: pera", "a => alfa", "b => beta"] -->
```joss
$frutas = ["manzana", "pera"]
foreach ($frutas as $indice => $fruta) {
    print($indice . ": " . $fruta)
}

$letras = ["a" => "alfa", "b" => "beta"]
foreach ($letras as $k => $v) {
    print($k . " => " . $v)
}
```
- `keys($persona)`: Retorna um array com todos os nomes das chaves do mapa. - `values($persona)`: Retorna um array apenas com os valores contidos no mapa. --- ## 4. Funções e pipelines de ordem superior (`map`, `filter`, `reduce`, `find`, `sum`) Joss inclui funções integradas para transformar e filtrar coleções em estilo funcional, projetadas para serem encadeadas com o operador de pipeline `|>`:<!-- joss-run: ["60", "20", "true"] -->
```joss
$numeros = [1, 2, 3, 4, 5]
$pares = $numeros |> filter(func(int $x): bool { return $x % 2 == 0; })
$escalados = $pares |> map(func(int $x): int { return $x * 10; })
print(sum($escalados))
$encontrado = $numeros |> find(func(int $x): bool { return $x == 2; })
print($encontrado * 10)
print($numeros |> any(func(int $x): bool { return $x == 3; }))
```
### Métodos de instância fluentes em coleções e strings Além do operador de pipeline, Joss permite invocar métodos fluidos diretamente em valores primitivos (`string`, `array`, `map`):<!-- joss-run: ["hola-mundo", "6, 8", "a-b"] -->
```joss
$txt = "  Hola Mundo  "
print($txt->trim()->lower()->replace(" ", "-"))

$nums = [1, 2, 3, 4]
$filtrados = $nums->map(func(int $n, int $i): int { return $n * 2; })->filter(func(int $n, int $i): bool { return $n > 4; })
print($filtrados->join(", "))

$mapa = {"a": 1, "b": 2}
print($mapa->keys()->join("-"))
```
--- ## 5. Comportamento da memória: cópia ou referência? Este é um conceito fundamental na arquitetura Joss: - Quando você atribui um número ou texto a outra variável (`$b = $a`), o valor é copiado de forma independente. - Por outro lado, **arrays** e **maps** são gerenciados internamente usando ponteiros e estruturas compartilhadas (fatias e mapas Go). Se você atribuir um array ou mapa existente a uma nova variável, **ambas as variáveis ​​apontam para a mesma estrutura de dados na memória**:<!-- joss-run: ["9", "nuevo"] -->
```joss
$original = [1, 2]
$copia = $original
$copia[0] = 9
print($original[0])
$datos = {"estado": "inicial"}
$alias = $datos
$alias["estado"] = "nuevo"
print($datos["estado"])
```
A modificação de `$copia[0]` também alterou `$original[0]`, porque ambos são dois nomes diferentes para o mesmo array físico. > [!TIP] > **Como criar uma cópia independente?** > Para duplicar um array sem compartilhar alterações futuras, use a função `merge`:```joss
$clon = merge([], $original)
```
--- ## 6. Nomes de funções que você deve conhecer bem Algumas funções para manipulação de arrays possuem contratos específicos no Joss que diferem de linguagens como PHP ou JavaScript:<!-- joss-run: ["2", "2", "3"] -->
```joss
$numeros = [1, 2]
print(array_pop($numeros))
print(count($numeros))
$numeros = array_push($numeros, 3)
print(count($numeros))
```
Preste atenção a estes detalhes: 1. `array_pop($arr)`: Retorna o último elemento, mas **não o remove do array original** (não altera o comprimento). 2. `array_shift($arr)`: Retorna o primeiro elemento sem excluí-lo. 3. `array_push($arr, $item)` e `append($arr, $item)`: Eles pegam o array, adicionam o novo elemento a ele e **retornam o novo array resultante**. É por isso que você deve remapear: `$numeros = array_push($numeros, 3)` ou usar a sintaxe direta `$numeros[] = 3`. 4. `array_slice($arr, $inicio, $longitud)`: Extrai uma parte do array sem modificar o original. --- ## 7. Manipulação de Texto: Bytes, Runas e Grafemas O texto digital moderno é muito mais complexo do que as letras inglesas do teclado ASCII. Ao lidar com texto com acentos (`á`, `é`), caracteres asiáticos ou emojis (`😀`, `👨‍👩‍👧‍👦`), uma única letra visual pode ser composta de vários bytes e até mesmo de vários caracteres Unicode combinados. Em Joss:<!-- joss-run: ["3", "2", "é"] -->
```joss
$texto = "é"
print(len($texto))
print(strlen($texto))
print($texto[0])
```
Observe a diferença das três linhas para a letra `é` (letra `e` com til combinado): 1. `len($texto)`: Retorna **3 bytes** no formato físico UTF-8. 2. `strlen($texto)`: Retorna **2 pontos de código Unicode** (a base `e` + o acento de combinação). 3. `$texto[0]`: Retorna o **grafema visual completo** (`é`). | Operação | Unidade de medição | Uso recomendado | |---|---|---| | `len($texto)` | Bytes físicos | Tamanhos de arquivos, transferências de rede, buffers de disco. | | `strlen($texto)` | Pontos de Código (Runas) | Algoritmos de análise de texto padrão. | | `$texto[$i]` | Grafemas estendidos (caracteres visíveis) | **Manipulação de texto orientada ao usuário**: cortar nomes, mostrar avatares, indexar sem quebrar emojis ao meio. | --- ## 7. Serialização e desserialização JSON (`json_encode` e `json_decode`) No Joss você pode serializar e desserializar JSON usando a **notação de mapas com colchetes `{}`** e a **notação de matrizes associativas com quadrado colchetes `[]` (`=>`)**, além de matrizes indexadas convencionais. Ambos os formatos são suportados em `json_encode` e `json_decode` (ou `JSON::encode` e `JSON::decode`): ### Com mapas usando colchetes `{}`:<!-- joss-run: ["Ada", "Joss"] -->
```joss
$perfil = {
    "nombre": "Ada",
    "lenguajes": ["Joss", "Go"]
}
$jsonMapa = json_encode($perfil)
$datosMapa = json_decode($jsonMapa)
print($datosMapa["nombre"])
print($datosMapa["lenguajes"][0])
```
### Com matrizes associativas usando colchetes `[]` e `=>`:<!-- joss-run: ["Carlos", "admin"] -->
```joss
$usuario = [
    "nombre" => "Carlos",
    "roles" => ["admin", "editor"]
]
$jsonArray = json_encode($usuario)
$datosArray = json_decode($jsonArray)
print($datosArray["nombre"])
print($datosArray["roles"][0])
```
- **`json_encode($datos, $pretty = false)`**: Converte arrays, mapas ou instâncias em uma string JSON válida. Passar `true` no segundo parâmetro formata o texto com recuo legível. - **`json_decode($cadenaJson)`**: Converte uma string JSON em coleções Joss: `{}` objetos são desserializados em mapas associativos `map`, listas `[]` para matrizes e tipo inteiro preservado `int`. - **`json_verify($cadenaJson)`**: Valida se o texto contém uma estrutura JSON sintaticamente correta. --- ## 8. Resumo de funções úteis para coleções | Função | Finalidade | Exemplo | |---|---|---| | `count($arr)` / `len($arr)` | Retorna o comprimento de uma coleção ou texto. | `count([1, 2])` → `2` | | `is_array($val)` | Verifica se um valor é uma matriz ou mapa associativo. | `is_array(["a" => 1])` → `true` | | `in_array($val, $arr)` | Verifica se existe um valor na matriz. | `in_array(2, [1, 2, 3])` → `true` | | `keys($map)` | Obtém a lista de chaves de um mapa. | `keys({"a": 1})` → `["a"]` | | `values($map)` | Obtém a lista de valores de um mapa. | `values({"a": 1})` → `[1]` | | `explode($sep, $str)` | Divide o texto em uma matriz usando um separador. | `explode(",", "a,b,c")` → `["a", "b", "c"]` | | `implode($sep, $arr)` | Une uma série de textos em uma única string. | `implode("-", ["2026", "09", "05"])` → `"2026-09-05"` | | `json_encode($val)` | Converte uma matriz ou mapa em texto JSON. | `json_encode(["ok" => true])` → `'{"ok":true}'` | | `json_decode($str)` | Converte texto JSON em array ou mapa Joss. | `json_decode('{"ok":true}')` | | `array_reverse($arr)` | Inverta a ordem dos elementos. | `array_reverse([1, 2, 3])` → `[3, 2, 1]` | --- ## 9. Exercícios práticos 1. **Gerenciamento de estoque**: - Crie um mapa chamado `$producto` com as chaves `"nombre"` (`"Laptop"`), `"precio"` (`1200.00m`) e `"stock"` (`5`). - Amostra no terminal: `"Producto: Laptop | Precio: $1200.00 | Disponibles: 5"`. - Simula uma compra subtraindo 1 do estoque e mostra o novo valor. 2. **Filtre palavras com `explode` e `implode`**: - Crie um texto `$frase = "manzana,pera,uva,platano"`. - Converta-o em um array usando `explode`. - Percorra o array com `foreach` e imprima cada fruta em maiúsculas usando o operador de pipeline: `$fruta |> strtoupper`. --- ## Próxima etapa Agora que você domina as estruturas de dados na memória, é hora de se aprofundar em como o analisador do tipo Joss verifica contratos, como a inferência funciona e como os valores nulos são tratados com segurança. Continue com: [Sistema de tipos, inferência e conversões](SISTEMA_TIPOS.md).
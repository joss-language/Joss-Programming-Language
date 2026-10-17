# Projeto prático: Aplicação de console com persistência JSON

Antes: [Concorrência e canais](CONCURRENCIA.md). Depois: [Projeto web completo](PROYECTO_WEB.md).
Referência técnica: [Linha de comando (CLI)](CLI.md), [Estrutura de projetos](ESTRUCTURA_PROYECTO.md).

---

## O que você vai construir aqui?

Uma **aplicação de console** (CLI) é um programa que é executado diretamente no terminal sem interface gráfica. É o tipo de software utilizado em automações, processamento de dados em lote (*batch processing*), scripts de manutenção de servidores e utilitários para desenvolvedores.

Neste tutorial guiado construiremos um **gerenciador de compras e estoque**:
1. Criar a estrutura oficial do projeto com `joss new console`.
2. Estruturar uma lista de itens com quantidades e preços decimais exatos.
3. Salvar os dados em um arquivo físico no disco (`compras.json`) em formato JSON estruturado.
4. Ler o arquivo do disco, validar sua integridade sintática (`json_verify`) e reconstruir as estruturas de dados na memória (`json_decode`).
5. Processar e totalizar os valores com precisão decimal financeira usando funções modulares.
6. Compilar o projeto em um executável nativo independente com `joss build`.

---

## 1. Preparar o ambiente de trabalho com `joss new console`

Abra seu terminal e gere um projeto de console oficial com o CLI do Joss:

```bash
joss new console proyecto-compras
cd proyecto-compras
```

O Joss terá criado a estrutura base do projeto (incluindo `main.joss`, `env.joss`, `joss.yaml` e a pasta `app/`). Abra `main.joss` em seu editor para escrever a lógica da nossa aplicação.

---

## 2. O código completo do programa

Substitua o conteúdo de `main.joss` pelo seguinte programa completo:

<!-- joss-run: ["Articulos: 2", "Total: 42.5", "Archivo guardado"] -->
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}

var $compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]

var $guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }

var $texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }

var $leidas = json_decode($texto)
print("Articulos: " . count($leidas))
print("Total: " . totalizar($leidas))
print("Archivo guardado")
```

---

## 3. Explicação passo a passo da arquitetura

Vamos analisar como interagem os diferentes subsistemas da linguagem neste programa:

### 1. A função de cálculo (`totalizar`)
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}
```
- Recebe um `array` de compras e promete retornar um valor do tipo `decimal`.
- Inicializa um acumulador exato: `decimal $total = 0m`.
- Percorre cada elemento com `foreach ($compras as $compra)`.
- Extrai `"precio"` e `"cantidad"` usando chaves de mapa. Observe como convertemos explicitamente:
  - `decimal($compra["precio"])`: Converte o texto numérico em decimal de ponto fixo.
  - `intval($compra["cantidad"])`: Converte a quantidade em inteiro de 64 bits.
- Multiplica ambos os valores e os soma a `$total`.

### 2. Estrutura de dados em memória
```joss
var $compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]
```
- Definimos um array cujos elementos são mapas associativos (`{"clave": valor}`).
- Salvar os preços como texto (`"12.50"`) dentro do JSON é uma boa prática contábil: evita que decodificadores JSON padrão introduzam imprecisões binárias ao ler números de ponto flutuante.

### 3. Persistência em disco com JSON
```joss
var $guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }
```
- `json_encode($compras)`: Transforma a estrutura em memória do Joss em um texto padrão JSON.
- `file_put_contents("compras.json", ...)`: Escreve esse texto no arquivo físico no disco rígido. Retorna `true` se teve êxito ou `false` se falhou por permissões ou falta de espaço.
- A expressão ternária atua como salvaguarda: se `$guardado` for falso, lança um erro com `throw`.

### 4. Leitura e validação de segurança
```joss
var $texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }
```
- `file_get_contents(...)`: Recupera os bytes do arquivo em uma cadeia de texto. Se o arquivo não existir, retorna `null`.
- `json_verify($texto)`: Função nativa do Joss que verifica se uma cadeia cumpre a especificação JSON válida sem chegar a analisar toda a árvore na memória. Se o arquivo estiver corrompido ou tiver sido editado incorretamente por um usuário, detecta imediatamente.
- `json_decode($texto)`: Reconstrói os dados JSON em arrays e maps nativos do Joss prontos para serem processados.

---

## 4. Análise, Execução e Compilação Nativa

Primeiro, vamos executar a análise estática para garantir que não haja inconsistências:

```bash
joss analyze main.joss
```

Se tudo estiver correto, execute o programa com o interpretador:

```bash
joss run main.joss
```

Saída produzida no console:

```text
Articulos: 2
Total: 42.5
Archivo guardado
```

Se você verificar sua pasta com `ls` (ou `dir` no Windows) ou pelo explorador de arquivos, verá que foi criado o arquivo físico `compras.json`:

```json
[{"cantidad":2,"nombre":"Cuaderno","precio":"12.50"},{"cantidad":5,"nombre":"Lapiz","precio":"3.50"}]
```

### Compilar para um binário nativo independente (`joss build`)

Quando quiser distribuir sua ferramenta de console para outras máquinas sem que precisem ter o Joss instalado nem entregar seu código-fonte, compile-a com:

```bash
joss build main.joss
```

O Joss gerará um executável nativo dentro da pasta `build/` (`build/main.exe` no Windows ou `./build/main` no Linux/macOS) pronto para ser executado diretamente.

---

## 5. Saída visual com cores: O módulo nativo `Console`

Para que sua aplicação de console ofereça uma experiência visual atraente e profissional, você pode colorir e destacar as mensagens no terminal por meio da classe nativa **`Console`**:

```joss
print(Console::green("✓ Archivo guardado con éxito"))
print(Console::yellow("⚠ Advertencia: El stock es bajo"))
print(Console::red("✗ Error al procesar datos"))
print(Console::bold("Total a pagar: S/ 42.50"))
```

### Métodos disponíveis de `Console`:

| Método | Propósito e Cor | Uso típico |
|---|---|---|
| `Console::green($t)` | Verde | Operações bem-sucedidas, confirmações (`✓ OK`). |
| `Console::red($t)` | Vermelho | Erros, falhas de validação, exceções. |
| `Console::yellow($t)` | Amarelo | Advertências, avisos que requerem atenção. |
| `Console::blue($t)` / `Console::cyan($t)` | Azul / Ciano | Títulos, links, informações descritivas. |
| `Console::bold($t)` | Negrito | Totais numéricos, nomes destacados. |
| `Console::clear()` | Limpar tela | Reinicia o terminal antes de mostrar um menu. |

---

## 6. Exercícios práticos

### Nível 1 — Teste rápido
Adicione um terceiro item ao array `$compras` (por exemplo, `{"nombre": "Mochila", "precio": "35.00", "cantidad": 1}`), execute novamente `joss run main.joss` e verifique se o total é atualizado para `77.5` e se `compras.json` reflete os 3 itens.

### Nível 2 — Exercício guiado
1. Crie uma função `public func articulosCaros(array $compras, decimal $umbral): array` que percorra `$compras` com `foreach` e retorne um novo array apenas com os produtos cujo `decimal($compra["precio"])` supere `$umbral`.
2. Imprima quantos itens superam `10.00m` usando `count(articulosCaros($leidas, 10.00m))`.
3. Colora a mensagem final `"Archivo guardado"` usando `Console::green("Archivo guardado")`.

### Nível 3 — Desafio
Modifique o programa para que antes de salvar o arquivo peça ao usuário um item adicional pelo teclado:
1. Declare `string $nuevoNombre = ""`, `string $nuevoPrecio = ""` e `int $nuevaCantidad = 0`.
2. Peça cada dado usando `cin >> $nuevoNombre`, `cin >> $nuevoPrecio` e `cin >> $nuevaCantidad`.
3. Adicione-o a `$compras` com `$compras[] = {"nombre": $nuevoNombre, "precio": $nuevoPrecio, "cantidad": $nuevaCantidad}`, salve o JSON e compile sua aplicação com `joss build main.joss`.

---

## Próximo passo

Agora que você dominou a persistência de dados em disco e o desenvolvimento de utilitários de console, é hora de explorar a área mais forte do Joss: o desenvolvimento de aplicações web de alto desempenho com rotas, controladores, bancos de dados e views HTML.

Continue com: [Construir uma aplicação web completa com a stack nativa](PROYECTO_WEB.md).
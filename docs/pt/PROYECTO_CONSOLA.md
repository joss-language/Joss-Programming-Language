# Projeto prático: Aplicação console com persistência JSON

Antes: [Simultaneidade e canais](CONCURRENCIA.md). Depois: [Projeto web completo](PROYECTO_WEB.md).
Referência técnica: [Linha de Comando (CLI)](CLI.md), [Estrutura do Projeto](ESTRUCTURA_PROYECTO.md).

---

## O que você vai construir aqui?

Um **aplicativo de console** (CLI) é um programa executado diretamente no terminal sem uma interface gráfica. É o tipo de software usado em automação, processamento em lote, scripts de manutenção de servidor e utilitários de desenvolvedor.

Neste tutorial guiado, construiremos um **gerenciador de compras e estoque**:
1. Estruture uma lista de itens com quantidades e preços decimais exatos.
2. Salve os dados em um arquivo físico em disco (`compras.json`) em formato JSON estruturado.
3. Lê o arquivo do disco, valida sua integridade sintática (`json_verify`) e reconstrói as estruturas de dados na memória (`json_decode`).
4. Processar valores totais com precisão decimal financeira usando funções modulares.
5. Gerencia possíveis falhas de leitura ou gravação de forma robusta.

---

## 1. Prepare o ambiente de trabalho

Abra seu terminal e crie um diretório limpo para este projeto:```bash
mkdir proyecto-compras
cd proyecto-compras
```
Abra seu editor e crie um arquivo chamado `main.joss`.

---

## 2. O código completo do programa

Escreva ou copie o seguinte programa completo em `main.joss`:<!-- joss-run: ["Articulos: 2", "Total: 42.5", "Archivo guardado"] -->
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}

$compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]

$guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }

$texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }

$leidas = json_decode($texto)
print("Articulos: " . count($leidas))
print("Total: " . totalizar($leidas))
print("Archivo guardado")
```
---

## 3. Explicação passo a passo da arquitetura

Vamos analisar como os diferentes subsistemas de linguagem interagem neste programa:

### 1. A função de cálculo (`totalizar`)```joss
public func totalizar(array $compras): decimal {
    return 0m
}
```
- Recebe um `array` de compras e promete retornar um valor do tipo `decimal`.
- Inicializa um acumulador exato: `decimal $total = 0m`.
- Percorra cada elemento com `foreach ($compras as $compra)`.
- Extraia `"precio"` e `"cantidad"` usando chaves de mapa. Observe como convertemos explicitamente:
  - `decimal($compra["precio"])`: Converte texto numérico em decimal de ponto fixo.
  - `intval($compra["cantidad"])`: Converte a quantidade em um número inteiro de 64 bits.
- Multiplica os dois valores e os soma ao `$total`.

### 2. Estrutura de dados na memória```joss
$compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]
```
- Definimos um array cujos elementos são mapas associativos (`{"clave": valor}`).
- Salvar preços como texto (`"12.50"`) dentro do JSON é uma boa prática contábil: evita que decodificadores JSON padrão introduzam imprecisões binárias ao ler números flutuantes.

### 3. Persistência em disco com JSON```joss
$guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }
```
- `json_encode($compras)`: Transforma a estrutura na memória do Joss em um texto JSON padrão.
- `file_put_contents("compras.json", ...)`: Grave esse texto no arquivo físico no disco rígido. Retorna `true` se tiver sucesso ou `false` se falhar devido a permissões ou falta de espaço.
- A expressão ternária atua como uma salvaguarda: se `$guardado` for falso, gera um erro com `throw`.

### 4. Leitura e validação de segurança```joss
$texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }
```
- `file_get_contents(...)`: Recupera os bytes do arquivo em uma string de texto. Se o arquivo não existir, retorna `null`.
- `json_verify($texto)`: Função Joss nativa que verifica se uma string atende à especificação JSON válida sem analisar a árvore inteira na memória. Se o arquivo estiver corrompido ou tiver sido editado incorretamente por um usuário, ele o detectará imediatamente.
- `json_decode($texto)`: Reconstrói dados JSON em arrays Joss nativos e mapas prontos para serem processados.

---

## 4. Análise e Execução

Primeiro, vamos executar uma análise estática para garantir que não haja inconsistências:```bash
joss analyze main.joss
```
Se tudo estiver correto, execute o programa:```bash
joss run main.joss
```
Saída produzida no console:```text
Articulos: 2
Total: 42.5
Archivo guardado
```
Se você verificar sua pasta com `ls` ou explorador de arquivos, verá que o arquivo físico `compras.json` foi criado. Se você abri-lo, você verá:```json
[{"cantidad":2,"nombre":"Cuaderno","precio":"12.50"},{"cantidad":5,"nombre":"Lapiz","precio":"3.50"}]
```
---

## 5. Saída visual com cores: O módulo nativo `Console`

Para fazer com que seu aplicativo de console forneça uma experiência visual atraente e profissional, você pode colorir e destacar mensagens no terminal usando a classe nativa **`Console`**:```joss
print(Console::green("✓ Archivo guardado con éxito"))
print(Console::yellow("⚠ Advertencia: El stock es bajo"))
print(Console::red("✗ Error al procesar datos"))
print(Console::bold("Total a pagar: S/ 42.50"))
```
### Métodos `Console` disponíveis:

| Método | Finalidade e Cor | Uso típico |
|---|---|---|
| `Console::green($t)` | Verde | Operações bem-sucedidas, confirmações (`✓ OK`). |
| `Console::red($t)` | Vermelho | Erros, falhas de validação, exceções. |
| `Console::yellow($t)` | Amarelo | Avisos, avisos que requerem atenção. |
| `Console::blue($t)` / `Console::cyan($t)` | Azul / Ciano | Títulos, links, informações descritivas. |
| `Console::bold($t)` | Ousado | Totais numéricos, nomes notáveis. |
| `Console::clear()` | Limpar tela | Reinicie o terminal antes de exibir um menu. |

---

## 6. Exercícios para expandir o projeto

1. **Adicione itens dinamicamente**:
   - Modifique o programa para solicitar ao usuário o nome, preço e quantidade do seguinte produto usando `cin >> $nombre`.
   - Adicione-o ao array com `$compras[] = ...` antes de salvar o arquivo.
2. **Filtrar itens caros**:
   - Crie uma função `public func articulosCaros(array $compras, decimal $umbral): array` que retorne um novo array apenas com os produtos cujo preço ultrapassa o limite.
3. **Colorir o relatório final**:
   - Use `Console::green(...)` para mostrar `"Archivo guardado"` e `Console::bold(...)` para o total.

---

## Próxima etapa

Agora que você domina a persistência de dados em disco e o desenvolvimento de utilitários de console, é hora de explorar a área mais forte de Joss: desenvolver aplicativos Web de alto desempenho com rotas, controladores, bancos de dados e visualizações HTML.

Continue com: [Construa um aplicativo Web completo com a pilha nativa](PROYECTO_WEB.md).
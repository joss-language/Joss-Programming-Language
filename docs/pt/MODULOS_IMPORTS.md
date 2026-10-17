# Módulos, arquivos e plugins

[Índice](README.md)

## Estado atual: Descoberta automática sem `import`

No Joss, um projeto é organizado como um conjunto de arquivos `.joss` que compartilham um espaço global de declarações públicas sem necessidade de escrever instruções `import` manuais:

- O carregador canônico `semanticanalyzer.LoadProject("main.joss", "app")` constrói a unidade de projeto descobrindo automaticamente o arquivo de entrada (`main.joss`), os arquivos de rotas (`routes.joss`, `api.joss`) e todos os arquivos `.joss` dentro de `app/` (incluindo `controllers`, `models`, `middleware`, `services`, `libs`, `database`, etc.).
- Esse conjunto validado produz um `PreparedProgram` único que é consumido diretamente por `joss run` (`Runtime.ExecutePrepared`), `joss server` e `joss build`.
- Visibilidade entre arquivos:
  - As classes, interfaces, enums e funções declaradas como `public` são visíveis em todos os arquivos do projeto.
  - As declaradas como `private` estão restritas exclusivamente ao arquivo `.joss` onde foram definidas (`JOSS-ACCESS-001`).

As formas históricas `import`, `use`, `@import` e `Namespace` foram eliminadas do conjunto de tokens, AST, executor e compilador de plugins. O parser as rejeita com uma mensagem de migração (`JOSS-PARSE-001`). Esta ausência é uma decisão permanente da linguagem: não haverá exports, namespaces fonte nem grafo de imports manuais.

## Plugins

A extensibilidade modular externa vigente usa pacotes `.jp`:

- São declarados em `joss.yaml` ou colocados em `plugins/`.
- O runtime os verifica criptograficamente e os carrega automaticamente.
- Cada pacote publica `META-INF/joss-symbols.json`.
- O analyzer consome esse índice para resolver classes, métodos e funções exportadas sem executar o plugin.

Um símbolo externo que não conste no índice não é inventado nem adicionado a uma lista manual do analyzer: deve-se corrigir o pacote ou sua geração de símbolos.

## Arquivos incluídos por infraestrutura

Rotas, controladores, modelos, serviços, bibliotecas e middleware sob `app/` são descobertos automaticamente segundo o layout de projeto. Esta inclusão física não cria um namespace por arquivo; as funções e classes `public` top-level compartilham o espaço de declarações do projeto e suas duplicatas são diagnosticadas (`JOSS-SYM-002`, `JOSS-DECL-004`).

## Decisão em relação à tese

O capítulo 11 da tese descreve módulos fonte com interfaces, exports e um DAG. A implementação adota uma decisão diferente: “modular” no ALIM significa capacidades integradas desacopladas e plugins `.jp` isolados, não imports escritos pela aplicação. Esta discrepância é deliberada e o design de módulos fonte da tese não faz parte do roadmap do Joss.

As funções e classes top-level formam um único espaço de declarações do projeto. As variáveis fonte top-level não são herdadas dinamicamente dentro de funções nomeadas; devem ser passadas como parâmetros. Uma closure conserva o ambiente léxico que captura.
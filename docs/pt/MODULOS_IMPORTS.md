# Módulos, arquivos e plugins

[Índice](README.md)

## Status atual

O projeto é organizado como um conjunto de arquivos `.joss`, mas cada comando
Escolha uma superfície específica. `joss analyze` carrega a entrada indicada e tudo mais
`app/`; ele não adiciona automaticamente `routes.joss` ou `api.joss`. O servidor carrega
suas rotas através de sua própria infraestrutura. O tempo de execução `joss run` pré-carrega apenas o
domínios padrão em `app/` e hoje omite `app/libs`, embora o analisador não
vá. Esta assimetria é registada como dívida.

As formas históricas `import`, `use`, `@import` e `Namespace` foram removidas do conjunto de tokens, AST, executor e compilador de plugins. O analisador os rejeita com uma mensagem de migração. Esta ausência é uma decisão permanente da linguagem: não haverá exportações, namespaces de origem ou gráfico de importações.

##Plugins

A extensibilidade modular atual usa pacotes `.jp`:

- Eles são declarados em `joss.yaml` ou colocados em `plugins/`.
- O tempo de execução verifica e carrega-os automaticamente.
- Cada pacote publica `META-INF/joss-symbols.json`.
- O analisador consome esse índice para resolver classes, métodos e funções exportados sem executar o plugin.

Um símbolo externo que não aparece no índice não é inventado ou adicionado a uma lista manual do analisador: o pacote ou a geração de seu símbolo devem ser corrigidos.

## Arquivos incluídos por infraestrutura

Rotas, controladores, modelos e middleware são descobertos com base no layout do projeto e CLI/tempo de execução. Essa inclusão física não cria um namespace por arquivo; Funções e classes de nível superior compartilham o espaço de declaração do projeto e suas duplicatas são diagnosticadas.

## Decisão quanto à tese

O capítulo 11 da tese descreve módulos de origem com interfaces, exportações e um DAG. A implementação toma uma decisão diferente: “modular” no ALIM significa recursos integrados desacoplados e plug-ins isolados, não importações escritas pela aplicação. Esta discrepância é deliberada e o design dos módulos fonte da tese não faz parte do roteiro de Joss.

As funções e classes de nível superior formam um único espaço de declaração de projeto. Variáveis ​​de origem de nível superior não são herdadas dinamicamente em funções nomeadas; Eles devem ser passados ​​como parâmetros. Um encerramento preserva o ambiente lexical que captura.
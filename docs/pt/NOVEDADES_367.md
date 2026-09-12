# O que há de novo no Joss v3.6.7.2

[Índice](README.md)

Esta versão consolida o mecanismo em torno de garantias verificáveis: análise
antes de executar, tipos estáveis ​​após inferi-los, carregamento automático sem
importa fonte e ferramentas que validam os projetos gerados.

## Idioma e sistema de tipos

- `$x = valor` e `var $x = valor` inferem um tipo fixo. Uma reatribuição deve
  guarde.
- `let $x = valor` declara dinamismo explícito usando `mixed`.
- Constantes implementadas, tipos anuláveis/de união e tipos de retorno.
- Funções e métodos utilizam frames lexicais isolados; recursão direta e
  A Mutual funciona com um limite configurável de 1.024 chamadas por padrão.
- `func` é a única palavra-chave de função. `function`, `import`, `@import`,
  `use` e namespaces de origem foram removidos.

## Análise estática

- `joss analyze` carrega o mesmo projeto e superfície nativa do tempo de execução.
- Resolve classes e funções de nível superior em duas passagens, incluindo referências
  avanços necessários para a recursão mútua.
- Verifique símbolos, escopos, aridade conhecida, operadores, atribuições,
  argumentos, retornos e fluxo alcançável.
- Os diagnósticos estruturados incluem código, gravidade, arquivo, intervalo,
  explicação e sugestão; informação desconhecida não se torna um
  erro sem evidência.

## Tempo de execução, bytecode e plug-ins

- Quadros de execução evitam escopo dinâmico e tempo de execução acidentais
  protege o limite de recursão.
- O formato principal `JOSSBC2Z` contém AST serializado e compactado. O
  o corredor continua a jogá-lo; Não é apresentado como código de máquina.
- Os pacotes JP v2 contêm manifesto, tabela de símbolos, bytecode e assinatura
  Ed25519 verificável.
- Os back-ends Python, Java, PHP e WASM dependem de seus hosts/protocolos reais
  quando aplicável; nenhuma promessa é feita para remover tempos de execução externos.
- Plugins declarados em `joss.yaml` ou instalados em `plugins/` são carregados
  automaticamente, sem instruções de importação no código Joss.

## CLI e projetos gerados

- `joss new web`, `console`, `package` e `plugin` produzem projetos que passam
  analisador, análise e seus fluxos representativos de execução/compilação.
- `make:migration` normaliza nomes como `create_products_table`, aplica o
  migração e só relata sucesso após registrar o lote.
- `make:crud` valida o esquema, distingue relacionamentos existentes, limita
  campos graváveis, evita exclusões por GET e não duplica rotas ou navegação.

## Validação

O repositório valida construção, testes, análise, catálogo de idiomas e exemplos
representante através da CI. Para o status exato e seus limites consulte
[Status de implementação](ESTADO_IMPLEMENTACION.md) e o
[Auditoria técnica](AUDITORIA_TECNICA_2026.md).
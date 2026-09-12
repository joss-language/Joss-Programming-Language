# Auditoria de núcleo técnico e alinhamento com tese (agosto de 2026)

[Índice](README.md)

## Linha de base

Antes das mudanças, `go test ./...` e `go build ./...` foram aprovados; `go vet ./...` falhou devido à construção manual de um endereço SMTP incompatível com IPv6. `joss analyze` no JosSecurity produziu 10 erros falsos e 14 avisos sem arquivo: dois nomes integrados implementados, mas ausentes do catálogo (`html_escape`, `unlink`) e sete usos de classes exportadas por plugins que o analisador não consultou (`BrevoClient`, `Notify`).

## Causas raiz encontradas

- Analisador global baseado em `map[string]int`, sem escopos, tipos ou unidade de origem.
- CLI concatenou ASTs e removeu a identidade do arquivo.
- Catálogo de built-ins divergentes do despachante: incluía funções inexistentes e omitia funções reais.
- Classes e plugins nativos foram resolvidos de diferentes maneiras.
- `let $name` construiu erroneamente um símbolo chamado `$`.
- `VarTypes` apenas declarações digitadas protegidas; as primeiras atribuições e parâmetros perderam seu tipo.
- `CallMethod` e `CallMethodEvaluated` ligação e validação duplicadas.
- O editor manteve listas manuais separadas de palavras-chave e classes/métodos nativos.
- CI só existia para distribuição manual; não havia fluxo de trabalho push/PR.
- `go vet` revelou o uso de `fmt.Sprintf("%s:%s")` para SMTP em vez de `net.JoinHostPort`.
- O pool preservou `PluginRegistry` ao excluir símbolos expostos, portanto, uma reutilização poderia ignorar o recarregamento do plugin.

## Decisões aplicadas

- Camadas novas e consumidas: `pkg/typesystem`, `pkg/diagnostics`, `pkg/analyzer`.
- Escopos de chamada e resolução de declarações em nível de projeto.
- Inferência fixa na primeira tarefa; `var` inferido; `let $x` dinâmico explícito.
- Diagnósticos estruturados e determinísticos por arquivo/linha/coluna.
- Ambiente analisador adaptado de registros de tempo de execução real e símbolos JP v2.
- Catálogo gerado para VS Code, validado por CI.
- As ricas assinaturas da editora são filtradas nesse catálogo; Os metadados desatualizados não podem mais publicar símbolos que o tempo de execução não registra.
- Vinculação de método unificado em `CallMethodEvaluated`.
- Frames de invocação isolados, recursão direta/mútua/método, limite de profundidade e contratos de retorno opcionais.
- Quadros lexicais sem escopo dinâmico: funções nomeadas não herdam localidades do chamador; Os fechamentos mantêm sua captura.
- Uniões `T|U`, `T?` anuláveis, retornos prováveis ​​exaustivos e assinaturas de retorno explícitas para todo o kernel nativo.
- Propriedades `const` e digitadas/constantes validadas pelo analisador e pelo tempo de execução.
- Erros do analisador armazenados diretamente como diagnósticos estruturados; a remoção de linhas de strings foi removida.
- Eliminação física de `ImportStatement`, importação de tabelas/tokens, plugin de linker textual e formato de bytecode descompactado.
- Remoção de APIs de compatibilidade sem consumidores canônicos: rotas brutas, inserções para arrays, Schema para mapas e `where(..., "json")`.
- Redefinição completa do registro do plugin ao retornar um tempo de execução ao pool.- Teste de integração especialmente JosSecurity.
- Fixação versionada do projeto em `testdata/analyzer-project`; JosSecurity continua sendo um repositório externo ignorado e seu teste é ignorado somente quando não está disponível.

## Resultado em JosSecurity

Ao eliminar as causas dos falsos positivos, surgiram seis problemas reais que antes estavam ocultos:

- `Math::length` não existe; foi substituído por `count`.
- `Str::endsWith` não existe; substituído por `str_ends_with`.
- `Str::upper` não existe; substituído por `strtoupper`.
- Três modelos herdados da classe removida `GranMySQL`; eles agora herdam de `GranDB`.

Uma segunda revisão detectou cinco avisos falsos: variáveis ​​de modelo com o mesmo nome de sua classe foram confundidas com acesso estático e não foram marcadas como usadas porque eram destinatárias de `->`. Precedência corrigida para que o símbolo lexical sombreie a classe e adicione uma regressão.

O resultado final é zero erros e cinco avisos, todos verificados em relação ao código: `$id`, `$licenseData`, `$offset`, `$user` e `$domain` são inicializados, mas não lidos de volta em seus respectivos callables. Eles são uma dívida real com a JosSecurity, não bloqueiam a execução e não foram modificados apenas para obter resultados vazios.

## Comparação com a tese

A implementação corresponde à visão ALIM no tempo de execução integrado, analisador Pratt, AST compartilhado, plug-ins isolados e conjunto de ferramentas único. A tese, no entanto, mistura estado real, sintaxe conceitual e roteiro:

| Declaração de tese | Status do repositório verificado |
|---|---|
| Pipeline inclui verificador de tipo | Existe um verificador semântico inicial com inferência fixa, constantes, assinaturas/retornos e chamadas recursivas; ainda não cobre esquemas contaminados, de escape ou de banco de dados. |
| AOT/LLVM/Cranelift e código de máquina | Os principais pacotes de construção serializaram o AST e o interpretador Go. LLVM/Cranelift não estão implementados. |
| Lexer/analisador em Rust | A implementação atual está em Go. |
| Imutabilidade por padrão/propriedade | Não há semântica de propriedade ou imutabilidade por padrão. |
| Importações/módulos com rede e ciclos | A sintaxe histórica foi completamente removida e não retornará. Plugins e arquivos convencionais são carregados automaticamente; Joss adota deliberadamente um projeto de importação zero. |
| Rotas/BDs como nós AST de primeira ordem | Hoje são chamadas para classes nativas (`Router`, `GranDB`), não para nós específicos. |
| 1.420 exames e cobertura de 91,4% | O repositório contém um conjunto Go muito menor. Medição focada atual: analisador 52,3%, sistema de tipos 44,9%, analisador 47,7% e núcleo 14,8%; O CI valida a execução e não afirma cobertura inexistente. |
| Análise de contaminação e 83% de vulnerabilidades | Existe um analisador heurístico de segurança no LSP, não um mecanismo de contaminação formal no compilador. |

Essas diferenças não foram “corrigidas” pela invenção de recursos. A proposta de módulos fonte no capítulo 11 está expressamente descartada para Joss; A modularidade do ALIM é preservada através de componentes de tempo de execução e plugins isolados. O resto deverá ser resolvido na tese, distinguindo implementação validada, sintaxe conceitual e trabalhos futuros.## Auditoria arquitetônica incremental — 11 de setembro de 2026

A revisão do commit 4b41742 usou o tamanho apenas como um sinal e comparou responsabilidades, dependências, estado compartilhado e conhecimento duplicado. A linha de base foi aprovada, exceto pkg/pluginpkg/TestLoadOrCreateSigningKey, que tentou gravar fora da sandbox para C:\Users\Asus\.joss\keys.

### Mapa priorizado

| Prioridade | Arquivo | Linhas aprox. | Diagnóstico | Ação |
|---|---|---:|---|---|
| P0 | cmd/joss/pub_cli.go | 1169 | credenciais, HTTP, resolução, cache, ZIP, YAML, lockfile e publicação | Extraia cliente, resolvedor, instalador e armazenamento |
| P0 | pkg/analyzer/infer.go | 1075 | inferência, chamadas, membros, acesso, hierarquia e estreitamento | Extraia o resolvedor nominal e valide a chamada |
| P0 | pkg/analyzer/analyzer.go | 927 | símbolos, contratos nominais, declarações e diagnósticos | Coletores separados, contratos e entidades |
| P0 | pkg/servidor/handler.go | 1042 | runtime, CORS, WebSocket, solicitação, sessão, CSRF e resposta | Função de Deus; extração progressiva iniciada |
| P0 | pkg/core/avaliador_infix.go | 772 | ternário, fluxos, pipeline, aritmética, postfix e correspondência | Fluxo separado, aritmética e incrementos |
| P0 | pkg/core/runtime.go | 691 | pool, ciclo de vida, fork, env, DB, instâncias e pré-carga | Extraia ciclo de vida, configuração e carregador |
| P0 | pkg/core/evaluator_call.go | 550 | vinculação, frames, referências, chamadas de host e built-ins | Separar pasta/moldura da expedição |
| P1 | pkg/parser/parser_statements.go | 1027 | declarações, tipos nominais, controle, exceções e seleção | Extrair por domínios gramaticais |
| P1 | pkg/parser/parser_expressions.go | 1088 | Pratt, literais, interpolação, coleções, chamadas e correspondência | Interpolação extraída; continuar para cobranças/solicitações |
| P1 | pkg/core/builtins_array.go | 681 | conversões, coleções e ordem superior | Conversões e algoritmos separados |
| P1 | pkg/core/native_seo.go | 588 | SEO e Sitemap | Agregados separados |
| P1 | pkg/core/view.go | 614 | avaliação, diretivas, seções e estado compartilhado | Extrair compilador de modelo |
| Manter | pkg/parser/ast_statements.go | 460 | modelo AST declarativo | Grande, mas coeso |
| Manter | pkg/typesystem/types.go | 402 | tipos, análise, atribuibilidade, inferência e coerção | Fonte canônica coesa |
| Manter | pkg/runtime/plan/callable.go | 443 | planos exigíveis | Especializado |

### Fontes da verdade

| Conceito | Antes | Resultado/proposta |
|---|---|---|
| Palavras-chave | Já canônico em parser/token.go | Manter nomes de palavras-chave |
| Símbolos | mudar lexer + lista multichar + mudar formatador | Símbolo CanônicoDefinição |
| Tipos numéricos | typesystem + analisador.isNumeric | Tipo.IsNumeric |
| Integrados | lista + seis conjuntos de retorno + cinco despachantes | nome do descritor/domínio/retorno |
| JSON global | duplicado em string e IO | builtins_serialization.go |
| Métodos nativos | nomes separados e retornos | Pendente: NativeMethodDefinition |
| Métodos primitivos | analisador e núcleo/primitivos | Pendente: catálogo mínimo de assinaturas |
| Diretiva @json | regex em linter e visualização | Pendente: compilador compartilhado || VM | semântica experimental paralela | Exigir conjunto diferencial antes de integrá-lo |

### Arquitetura e refator aplicado

O léxico permanece no analisador, os tipos no sistema de tipos, a semântica no analisador e a execução/superfície nativa no núcleo. Não é criado um pacote linguístico global: a centralização de todos os domínios aumentaria o acoplamento e o risco de ciclos.

1. parser/token.go define símbolos por mastigação máxima; lexer e formatador consomem a mesma projeção.
2. parser_interpolation.go encapsula segmentação e composição AST sem nova API pública.
3. Type.IsNumeric substitui a classificação local do analisador.
4. core/builtins.go declara uma vez nome, domínio e retorno; o analisador e o tempo de execução consomem o descritor.
5. builtins_serialization.go remove as duas implementações JSON.
6. server/rate_limiter.go e request_data.go isolam o estado e a fronteira HTTP→Joss.

| Área | Antes | Depois |
|---|---|---|
| Símbolos | 3 representações | 1 inscrição + exibições |
| Lexer | 445 linhas | 218 |
| Expressões do analisador | 1088 com interpolação | 958 + módulo coeso de 129 |
| Integrados | lista + 6 conjuntos + cascata | descritor único + envio direto |
| JSON | 2 implementações | 1 |
| Manipulador HTTP | 1042 | 925 + módulos coesos de 51 e 100 |
| Guardas | parcial | testes exaustivos de símbolos e manipuladores |

O número total de linhas não era o objetivo: foram somados testes e contratos. Agora, um símbolo se transforma em uma definição e um símbolo integrado em um descritor, além de seu manipulador de domínio.

### Riscos restantes

- MainHandler ainda contém sessão/CSRF, WebSocket e escrita de resposta; requer testes de caracterização HTTP antes da extração.
- Classes nativas ainda separam nomes de métodos e retornos precisos; devem ser migrados classe por classe.
- infer.go e analyzer.go permanecem riscos semânticos e exigem resolução nominal, referências e testes de estreitamento durante sua divisão.
- Formatador precisa de curiosidades; o compartilhamento de símbolos é bom, substituí-lo pelo lexer que descarta curiosidades não seria.
- pub_cli.go precisa de caracterização de sistema de arquivos, rede e arquivo de bloqueio antes da divisão.
- Aliases integrados são suportados publicamente; Eles não devem ser eliminados por duplicação superficial.
- VM e JPBC não devem ser promovidos sem testes diferenciais.

## Segunda fase de refatoração — setembro de 2026

### Revalidação e pedido

A auditoria foi revalidada na árvore atual antes de modificá-la. Os sete P0s ainda estavam ativos: `pub_cli.go` 1312 linhas, `infer.go` 1101, `analyzer.go` 964, `handler.go` 1024, `evaluator_infix.go` 835, `runtime.go` 762 e `evaluator_call.go` 601. A cobertura de referência foi analisador 57,7%, núcleo 47,9%, servidor 8,0% e cmd/joss 16,2%.

A ordem escolhida foi chamadas → operadores → inferência. As chamadas apresentavam boa caracterização e percurso semântico já unificado; os operadores poderiam ser congelados com testes diferenciais por domínio; a inferência exigia a preservação do limite do sistema de tipos/analisador. O ciclo de vida do tempo de execução, HTTP e publicação permanecem depois porque seu estado externo requer acessórios específicos antes da extração.

### Alterações aplicadas| Área | Situação anterior | Arquitetura resultante |
|---|---|---|
| Invocação | `evaluator_call.go` avaliação de argumentos mistos, ligação, frames, tipos chamáveis ​​e embutidos | `call_arguments.go` possui avaliação/binding/ref; `call_method.go` possui frame, retorno e recursão; `callable_dispatch.go` possui tipos que podem ser chamados; `evaluator_call.go` coordena a resolução da fonte e os recursos integrados |
| Operadores | `evaluateInfix` controle misto, pipeline, números, fluxos, intervalos, prefixo/postfix e correspondência; Também continha um segundo pipeline inacessível | `evaluator_control.go`, `evaluator_pipeline.go`, `evaluator_numeric.go`, `evaluator_stream.go` e `evaluator_update.go`; `evaluator_infix.go` preserva a ordem de avaliação e curto-circuito |
| Inferência nominal | `infer.go` continha atribuibilidade nominal, hierarquia e estreitamento junto com inferência de base | `infer_nominal.go` possui hierarquia/atribuição de projeto; `infer_narrowing.go` possui refinamento de escopo; `infer.go` retém a entrada de inferência e ainda contém resolução de chamada/membro pendente |
| Métodos primitivos | nomes duplicados e retornos no analisador; implementação listada separadamente no núcleo | `typesystem.PrimitiveMethodDefinition` são os metadados canônicos; analisador e tempo de execução consultam-no e um teste requer implementação em tempo de execução para cada definição |
| Extensão | `js-yaml` 4.3.1 (alto) e `qs` 6.15.3 (moderado), ambos devtools transitivos | substituições compatíveis com 4.3.2 e 6.16.0; `npm audit` é definido como zero sem `--force` |

A divergência `array.pop` era real: o analisador postou o retorno do elemento, mas o tempo de execução do array não implementou o método. Foi removido dos metadados compartilhados sem inventar uma semântica mutável. Se for projetado posteriormente, deverá ser adicionado como uma decisão explícita com implementação e testes.

### Caracterização e regressões

- `call_characterization_test.go` congela ligações nomeadas/posicionais/padrão, erros ausentes/desconhecidos e contratos de retorno estruturados. Os pacotes existentes cobrem referências multiníveis, fechamentos, recursão, herança e reciclagem de quadros.
- `infix_characterization_test.go` congela pipeline, Elvis/ternário, correspondência estrita, comparações, decimal exato, intervalos e postfix.
- `primitive_methods_architecture_test.go` evita metadados de publicidade sem implementação em tempo de execução; `primitive_methods_test.go` valida retornos dependentes do receptor.
- A suíte detectou capacidade negativa em níveis ascendentes durante a extração. Foi uma regressão do refatorador, foi corrigida antes de continuar e os contratos de documentação foram aprovados novamente.

### Comparação e alteração da superfície

| Métrica | Antes | Depois |
|---|---:|---:|
| `evaluator_call.go` | 601 linhas / 6 responsabilidades | 91 linhas coordenadoras + 3 módulos de 143/216/158 linhas |
| `evaluator_infix.go` | 835 linhas / 7 domínios | 120 linhas de coordenação + 5 módulos por domínio de 58–269 linhas |
| `infer.go` | 1101 linhas/inferência + nominal + estreitamento | 907 linhas; nominal 86 e estreitamento 65; convocação/resolução de membro ainda pendente || Pipeline `|>` | 2 implantações, uma morta | 1 implementação canônica |
| Assinaturas primitivas | analisador + interruptores de tempo de execução sem proteção comum | 1 metadados + implementações de tempo de execução protegidas por teste |

Área aproximada de mudança: a adição de um método primitivo vai desde a edição do analisador e tempo de execução sem verificação (2 locais propensos a divergência) até a edição de metadados e implementação (2 locais necessários) com projeção automática para o analisador e teste de paridade. Adicionar um operador ainda requer analisador/analisador/tempo de execução e, se aplicável, VM; O comportamento que pertence a diferentes camadas não foi centralizado. Adicionar um integrado continua exigindo um descritor e manipulador de domínio. A adição de uma regra de analisador nominal está localizada em `infer_nominal.go`.

### Dívida reorganizada

- **P0:** `analyzer.go`; resolução de chamada/membro restante em `infer.go`; ciclo de vida/pool em `runtime.go`; `MainHandler`; `pub_cli.go`. Nenhum é considerado resolvido por tamanho.
- **P1:** metadados `NativeMethodDefinition` classe por classe; compilador de diretiva de visão compartilhada; Caracterização e pooling HTTP; declarações/expressões do analisador restantes.
- **P2:** conjunto diferencial de VM/intérprete antes de promover a VM; publicar uma série de métodos primitivos quando existir um contrato confiável.

Regra para a próxima iteração: não extraia ciclo de vida, HTTP ou log/publicar até que seus testes congelem redefinição/reutilização/fork, CORS/sessão/CSRF/WebSocket/status e cache/archive/lockfile respectivamente.

Cobertura posteriormente focada: analisador 59,1%, núcleo 48,2%, servidor 8,0% e cmd/joss 16,2%. A melhoria é pequena porque os novos testes priorizam contratos críticos; a baixa cobertura de servidor/CLI confirma que eles ainda não devem ser refatorados sem caracterização adicional.

## Riscos pendentes

- A recuperação do analisador pode produzir diagnósticos derivados após o primeiro token inválido, embora agora todos usem o modelo estruturado e preservem a coluna sem extraí-la das mensagens.
- Os retornos nativos são explícitos, mas muitas APIs preservam parâmetros variados até publicar contratos de aridade confiáveis.
- Não há refinamento sensível para filiais, contratos de infraestrutura, contaminação ou fuga formal. Os ciclos de módulo não se aplicam porque não há módulos de origem.
- `pkg/core` permanece amplo; A divisão de subsistemas de infraestrutura requer testes específicos e não foi feita apenas por questões estéticas.
- O rico catálogo de assinaturas do LSP consiste em metadados manuais; A existência de nomes vem do catálogo gerado.

## Terceira fase da arquitetura — setembro de 2026

Esta fase priorizou a estabilização do analisador e a caracterização do edifício antes da extração da infraestrutura.

###Analisador`Analyzer.Analyze` agora expressa o pipeline `collectDeclarations → projectScope → validateNominalContracts → analyzeSourceBodies → diagnostics`. A fachada mantém o seu estatuto de projeto; coleta de declarações, contratos nominais, análise corporal, resolução de chamadas e resolução de membros residem em arquivos no mesmo pacote (`declaration_collection.go`, `nominal_contracts.go`, `body_analysis.go`, `call_resolution.go`, `member_resolution.go`). Não foram criados gestores públicos nem introduzida dependência do `core`. O cursor arquivo/classe/retorno é restaurado por callable, evitando contaminação entre unidades.

As chamadas passam por uma assinatura semântica `Callable` e uma validação única de aridade, nomes, referências e compatibilidade; Métodos globais, nativos, primitivos e de plug-in são projetados para essa representação sem compartilhar uma implementação de tempo de execução. A resolução dos membros distingue receptor, visibilidade, hierarquia, campos e métodos. A representação comum são os metadados: o analisador e o tempo de execução não compartilham o estado de execução.

### Caracterização pré-infraestrutura

Adicionados testes de ciclo de vida e pool (`runtime_lifecycle_characterization_test.go`) que corrigem a criação, bifurcação, redefinição e reutilização. O teste revelou e corrigiu a contaminação real de `Env`, caches de metadados, fonte atual, geradores e deferimentos em `Runtime.Free`; `DB` permanece como compartilhamento externo e não é fechado em `Free`. O estado é classificado como configuração persistente, por tempo de execução, por execução ou cache reconfigurável.

A borda HTTP não possui caracterização de tempo de execução, 503, CORS, sessões/CSRF e mapeamento de uma resposta Joss (`handler_characterization_test.go`). WebSocket e casos de resposta não suportados ainda estão explicitamente pendentes para não inventar a semântica. CLI/pub incorpora dispositivos temporários e servidores de registro na memória para extração segura de ZIP, rejeição de travessia, arquivo de bloqueio determinístico, manifestos e resolução de erros (`cmd/joss/pub_cli_test.go`). Isso permite refatorar posteriormente sem confundir alterações de infraestrutura com alterações de linguagem.

### Diretivas e metadados

O inventário confirma que `@json` é uma transformação de visão isolada junto com `@extends`/`@section`; linter e view ainda interpretam de maneiras diferentes. Permanece como dívida P1: primeiro deve ser definida uma representação política, não uma regex compartilhada. `NativeMethodDefinition` também permanece P1: as classes retêm nomes e retornos separados e aridades inventadas para APIs variadas não são publicadas.

### Alterar superfície e métricas

| Alterar | Antes | Situação da fase 3 | Proteção |
|---|---:|---:|---|
| regra do analisador | `infer.go` + estado implícito | fase explícita e arquivos por conceito | `semantic_pipeline_test.go` |
| ciclo de vida/redefinir tempo de execução | sem contrato de pool completo | redefinição de estado por execução centralizada | `runtime_lifecycle_characterization_test.go` |
| Resposta HTTP | manipulador monolítico sem placa | fronteira caracterizada, extração diferida | `handler_characterization_test.go` || publicação/ZIP/bloqueio | efeitos externos sem luminárias | luminárias temporárias e registro `httptest` | `pub_cli_test.go` |

Fan-in/out ainda é observado no nível do pacote: `analyzer` consome analisador/typesystem/diagnostics mas não núcleo; `core` mantém o maior espalhamento para integração de idioma e host; server e cmd dependem de núcleo/adaptadores. Esses números não são usados ​​como portas. A duplicação semântica restante é intencional (validação estática versus execução) e compartilha tipos/metadados, não código de efeitos.

### Dívida restante

- **P0:** `MainHandler` e `Runtime` ainda coordenam múltiplos domínios; Eles devem ser extraídos somente após extensão do WebSocket, mapeamento de resposta e contaminação simultânea.
- **P1:** `NativeMethodDefinition`, assinaturas primitivas completas, compilador de diretiva de modelo, normalização de argumento nomeado/ref exclusivo e métricas automatizadas de superfície de fan-in/alteração.
- **P2:** Suíte de intérprete/VM diferencial, difusão seletiva de ZIP/lock/modelo e benchmarks comparáveis ​​de inferência/chamadas.

## Quarta fase da arquitetura — setembro de 2026

### Revalidação e ambiente reproduzível

Os P0s atuais eram ciclo de vida/bifurcação de `Runtime`, `MainHandler` e `pub_cli.go`; Os P1s eram metadados, argumentos e diretivas nativos. A linha de base focada foi analisador 60,3%, núcleo 48,3%, servidor 29,7% e cmd/joss 20,4% após esta fase. A falha do `pluginpkg` não foi com o produto: seu teste foi escrito no HOME real; agora redefine `USERPROFILE`/`HOME` para `t.TempDir()`. O cache global Go tem ACLs inválidas neste host; A solução reproduzível é atribuir `GOCACHE` a um diretório temporário. `npm ci` dentro do checkout colide com arquivos abertos pelo VS Code; uma cópia temporária sem `node_modules`, com cache/rede habilitado, executou `npm ci`, `npm run compile` e `npm audit` (0 vulnerabilidades).

### Modelo de estado de tempo de execução

| Campo | Responsabilidade | Vitalício | Garfos | Grátis | Compartilhado/proprietário |
|---|---|---|---|---|---|
| `Env` | configuração eficaz | configuração de tempo de execução | copiar | limpar | tempo de execução |
| `Variables` | ligações e globais | por execução | cópia com clones conhecidos | encadernações limpas + padrão | tempo de execução/quadro |
| `VarTypes` | tipos de tempo de execução | por execução | copiar | limpar | tempo de execução |
| `Constants` | constância | por execução | copiar | limpar | tempo de execução |
| `HostGlobals` | visibilidade do host | ciclo de vida de execução | copiar | reconstrói | tempo de execução |
| `Classes` | aulas/projetos nativos | configuração de tempo de execução | cópia do mapa, AST imutável compartilhado | limpar | tempo de execução/projeto |
| `Interfaces` | contratos de projecto | configuração de tempo de execução | cópia do mapa | limpar | tempo de execução/projeto |
| `Enums` | enumerações do projeto | configuração de tempo de execução | cópia do mapa | limpar | tempo de execução/projeto |
| `Functions` | funções do projeto | configuração de tempo de execução | cópia do mapa, compartilhada AST | limpar | tempo de execução/projeto |
| `DB` | conjunto SQL | recurso externo | compartilhar | sobrevive, não fecha | aplicação/`database/sql` thread-safe |
| `Routes` | Tabela HTTP | configuração de tempo de execução | cópia em dois níveis | limpar | tempo de execução || `CurrentMiddleware` | empilhar ao registrar rotas | por execução | reiniciar | limpar | tempo de execução |
| `CustomMiddlewares` | middleware chamável | configuração de tempo de execução | copiar | limpar | tempo de execução |
| `NativeHandlers` | implementação nativa | global imutável após registro | copiar | limpar/reconstruir na compra | núcleo |
| `NativePlugins` | plug-in de cargas úteis | configuração de tempo de execução | cópia do mapa; definições tratadas como imutáveis ​​| limpar | carregador de tempo de execução/plugin |
| `NativeDrivers` | alças nativas | recurso externo | compartilhar definições/nomes | referência limpa, sem identificador de download | carregador |
| `PluginRegistry` | plugin de símbolos/instâncias | ciclo de vida de execução | compartilha explicitamente com o pai | remover referência | tempo de execução pai/pluginruntime |
| `ProjectRoot` | raiz do projeto | configuração de tempo de execução | copiar | limpar | tempo de execução |
| `SEO` | acumulador de respostas | por solicitação | reiniciar | limpar | solicitação de tempo de execução |
| `SitemapEntries` | configuração do mapa do site | configuração de tempo de execução | copiar fatia | limpar | tempo de execução |
| `SitemapProviders` | fechamentos de mapa do site | configuração de tempo de execução | copiar fatia; fechamentos compartilhados | limpar | tempo de execução |
| `SitemapExclusions` | configuração do mapa do site | configuração de tempo de execução | copiar fatia | limpar | tempo de execução |
| `CurrentSource` | cursor de origem | por execução | reiniciar | limpar | avaliador |
| `CurrentFile` | arquivo de cursor | por execução | reiniciar | limpar | avaliador |
| `MaxCallDepth` | limite | configuração de tempo de execução | copiar | padrão | tempo de execução |
| `callDepth` | profundidade atual | por execução | reiniciar | limpar | avaliador |
| `currentClass` | cursor nominal | por execução | reiniciar | limpar | avaliador |
| `callStack` | pilha de diagnóstico | por execução | reiniciar | limpar | avaliador |
| `callablePlans` | planos de método | esconderijo | cópia do mapa; planos imutáveis ​​| limpar | tempo de execução |
| `functionPlans` | planos de encerramento | esconderijo | cópia do mapa | limpar | tempo de execução |
| `classMetadataCache` | metadados derivados | esconderijo | reiniciar | limpar | tempo de execução; protegido por `planMu` |
| `currentFrame` | quadro ativo | por execução | reiniciar | limpar | avaliador |
| `planMu` | proteção de cache | ciclo de vida de execução | novo mutex | permanece | tempo de execução |
| `captureEnvironment` | captura temporária | por execução | reiniciar | limpar | avaliador |
| `cinReader`, `cinTokens` | entrada tokenizada | por execução | reiniciar | limpar | Tempo de execução de E/S |
| `currentGenerator`, `generatorIndex` | gerador de cursor | por execução | reiniciar | limpar | avaliador |
| `topDefers` | adiamentos de nível superior | por execução | reiniciar | limpar | avaliador |

Invariantes: após `Free` nenhuma solicitação, execução, cursores ou caches sobrevivem; as ligações de host são reconstruídas; `DB` não está fechado porque `Runtime` não é seu proprietário. `Fork` compartilha apenas AST/planos/configuração tratados como imutáveis, registro de plugins e recursos externos; mapas de solicitação mutáveis ​​são isolados. Os testes descobriram que `Fork` omitiu `Enums`; foi corrigido. O pool suporta aquisição simultânea/liberação sob detector de corrida.`runtime_lifecycle.go` contém compilação, pool, aquisição, redefinição e invariantes. `runtime.go` preserva ambiente/carregamento e instruções de log. `Runtime` ainda é coordenador; nenhum gerente foi apresentado.

### HTTP do ciclo de vida e mapeamento de resposta

O tempo de execução da solicitação é liberado por um único `defer` imediatamente após `Fork`, incluindo limitação de taxa, CORS, sessão/armazenamento, pânico e WebSocket. `response_writer.go` é a fronteira Joss→HTTP.

| Resultado | Status/Tipo de conteúdo | Conduta |
|---|---|---|
| `string` | 200, HTML padrão | escrever texto e recarregar HTML a quente |
| mapa/instância `JSON` | `status_code` ou 200, JSON | codificação `data` |
| mapa/instância `RAW` | `status_code` ou 200, configurável | string/bytes/representação textual + cabeçalhos |
| Instância `FILE` | 200, MIME detectado | anexo ou 500 se não for lido |
| Instância `STREAM` | 200, SE | liberar e retornar chamada com Stream |
| mapa/Instância `REDIRECT` | 302/mapa ou `status_code`/Instância | Localização; A instância aplica cookies/flash |
| int/float/bool/null/array/map sem `_type` | nenhuma representação publicada | continuar para arquivo público/404 |

A caracterização corrigiu um bug: `Response::json(..., status)` armazena `status_code`, mas o manipulador lê `status`, retornando 200. WebSocket cobre atualização inválida e atualização/fechamento válido sem caminho; retornos de chamada de mensagem completa continuam P1.

### Metadados e assinaturas

`NativeMethodDefinition` separa nome, retorno, parâmetros opcionais, `ArityKnown` e variedade da implementação `NativeHandler`. Stack, Queue e Math são a primeira migração; seus nomes/retornos são projetados no tempo de execução do AST, analisador, catálogo LSP e documentação. A aridade permanece desconhecida quando o manipulador tolera entradas dinâmicas. Os testes de paridade evitam o anúncio de métodos sem um manipulador. As demais classes retêm o adaptador legado até a migração classe por classe.

A normalização estática cria um único relacionamento parâmetro→argumento para posicional, nomeado, padrões, ref, desconhecido, duplicado e ausente. O tempo de execução mantém seu fichário, mas ambos consomem os mesmos parâmetros semânticos e rejeitam duplicatas; analisador não executa código de tempo de execução.

### Templates, CLI e semântica diferencial

O inventário de visualização inclui `@extends`, `@section`/`@endsection`, `@yield`, `@include`, `@json` e `@foreach`/`@endforeach`. `pkg/viewtemplate` fornece scanner mínimo com intervalos, aspas e parênteses aninhados. O tempo de execução e o linter consomem `RewriteJSON`; Eles não mantêm mais regex distintos. Herança/seções/inclui/foreach retêm seus consumidores atuais até a migração progressiva. Existe um alvo fuzz para determinismo/sem pânico/intervalos.

CLI/pub usa um cliente HTTP interno com timeout, cache configurável para testes, registro `httptest`, ZIP seguro e temps. Adicionado 401/403/429/500, JSON truncado/inválido, tempo limite, acerto/obsoleto de cache e parciais ausentes. O destino ZIP fuzz verifica se não está gravado fora do destino.O primeiro corpus diferencial declara apenas aritmética/comparação inteira, atribuição local e prefixo inteiro. O analisador deve aceitar, o intérprete e a VM devem produzir o mesmo valor. A VM permanece experimental e quaisquer novos recursos devem ser explicitamente incorporados ao inventário, e não inferidos como suportados.

| Recursos | Analisador | Intérprete | PSL | VM | Estado |
|---|---|---|---|---|---|
| inteiros/aritmética básica | tipos canônicos | referência publicada | catálogo de sintaxe | diferencial verde | suportado em corpus |
| chamadas/métodos/fechamentos | assinaturas semânticas | referência publicada | projeção parcial | não comparável | VM pendente |
| nativos | projeção de metadados | manipuladores | catálogo gerado | não suportado | metadados progressivos |
| modelos | validar script compilado | renderizar | trechos/diagnósticos | não aplicável | scanner comum parcial |
| correspondência/pipeline/referências | analisador ativo | referência publicada | sintaxe | não paridade total | Diferencial P2 |

As proteções negativas verificam se o analisador/typesystem/diagnostics/analyzer não importa o núcleo. O catálogo JSON gerado contém `_generated: DO NOT EDIT` e ainda é validado por `cataloggen --check`.

### Linha de base de desempenho

Windows/amd64 i5-10300H, `-benchtime=100ms`: chamada simples 794 ns/op, 2936 ns/op aninhado, ref 1496 ns/op, fechamento 973 ns/op; reciclagem de quadros 1213 ns/op, 24 B/op, 2 alocações/op. Analisador de inicialização do aparelho: 10095 ns/op, 7315 B/op, 69 alocações/op. Ciclo de vida: construção 235536 ns/op, aquisição em pool/liberação 289634 ns/op e bifurcação/liberação 26039 ns/op. A piscina inclui recarga/carregamento automático; Não foi otimizado sem delinear essa responsabilidade.

### Dívida reorganizada (Quarta fase)

- **P0:** isolamento/propriedade completo de registros e drivers de plugins antes da simultaneidade mutável; retornos de chamada WebSocket completos e erros de sessão de back-end.
- **P1:** migrar NativeMethodDefinition classe por classe; extrair sessão/CSRF do MainHandler; registro/cache/manifesto/editor separado da CLI do pub; migre o restante das diretivas para o scanner.
- **P2:** expanda o corpus Analyzer↔Interpreter↔VM, catálogo de assinaturas LSP, tipo fuzz do analisador e lockfile de ida e volta.
- **P3:** automatizar a superfície de fan-in/change e estabelecer benchmarks históricos comparáveis ​​em CI.

---

## Quinta fase da arquitetura — setembro de 2026

A quinta fase aborda e resolve todas as dívidas P0 críticas pendentes da quarta fase, com foco na consistência de propriedade, simultaneidade segura, contratos de sessão e ciclo de vida de plug-ins e WebSockets, além de avançar metadados canônicos e ferramentas de modelo.

### 1. Propriedade e simultaneidade de plug-ins e drivers nativos (P0-A)

- **Isolamento por tempo de execução (`PluginAwareHost`)**: Introduziu a interface `PluginAwareHost` em `pkg/pluginruntime` para desacoplar o registro global de pacotes da execução concreta. `Runtime` implementa esta interface mantendo seus próprios motores AST (`pluginASTEngines map[string]*PluginASTEngine`) e namespaces (`PluginNamespace`).- **Semântica de Fork**: Ao executar `Runtime.Fork()`, as fachadas do motor são duplicadas vinculadas ao novo tempo de execução filho, compartilhando com segurança o AST imutável do plugin enquanto isolam totalmente o estado avaliado e os frames locais. Dois plug-ins diferentes com funções idênticas (por exemplo, `run()`) não colidem mais entre si ou cruzam escopos de solicitação.
- **Descarregamento de driver atômico (`NativeDriverDefinition.Unload()`)**: Drivers nativos dinâmicos (`.dll`/`.so`/`.dylib`) incorporam um método seguro `Unload()` protegido por `driverMu`. Através de `unloadNativeDriverHandle` (`FreeLibrary` no Windows, `dlclose` no Unix), é garantida a liberação idempotente de recursos e a prevenção de falhas de segmentação durante chamadas simultâneas ou pós-descarregamento.

| Componente | Nível de compartilhamento | Ciclo de Vida | Política Simultânea |
|---|---|---|---|
| Plugin AST (`*parser.Program`) | Imutável/Compartilhado | Processo | Thread-safe somente leitura |
| `PluginASTEngine` | Por `Runtime` / Instância | Solicitação/Fork | Nenhuma contenção entre threads |
| `PluginNamespace` | Por `Runtime` / Instância | Solicitação/Fork | Instâncias clonadas em `Fork()` |
| `NativeDriverDefinition.handle` | Ponteiro para sistema operacional | Carregar até `Unload()` | Protegido por `driverMu` |

### 2. Ciclo de vida completo e retornos de chamada WebSocket (P0-B)

- **Execução isolada de retornos de chamada**: conexões WebSocket executam retornos de chamada definidos no código Joss (`onConnect`, `onMessage`, `onClose`, `onError`) em um tempo de execução bifurcado separado.
- **Propagação de Parâmetros**: Parâmetros de rota (`$params`) extraídos durante o handshake HTTP são injetados corretamente no contexto de retorno de chamada.
- **Isolamento de múltiplas conexões**: conexões simultâneas operam em soquetes e tempos de execução independentes, sem filtragem de quadros ou estado léxico entre clientes.

### 3. Contratos de redirecionamento de Flash e armazenamento de sessão (P0-C)

- **Persistência unificada**: A serialização Flash em redirecionamentos HTTP (`persistRedirectFlash` em `response_writer.go`) foi unificada sob o contrato canônico `saveSession(sessionID, store)`.
- **Tratamento rigoroso de erros**: após uma falha de back-end da sessão durante um redirecionamento, a resposta é imediatamente abortada com HTTP 500 e uma mensagem de erro explícita, suprimindo o cabeçalho `Location` para evitar que o cliente siga um redirecionamento com flash corrompido ou não persistente ou dados de sessão.

### 4. Proteção de liberação dupla (`sync.Pool`)

- **Defesa contra double-free**: Uma potencial condição de corrida em testes e solicitações devido a chamadas duplicadas para `Free()` no mesmo `*Runtime` foi identificada e resolvida. Um campo booleano `freed` protegido por `poolMu` garante que o retorno ao pool seja idempotente e que um ponteiro não entre novamente no pool simultâneo várias vezes.

### 5. Expansão de metadados nativos (`NativeMethodDefinition`)

Concluída a migração de um lote estendido de 10 classes nativas canônicas para `NativeMethodDefinition`, distinguindo assinaturas canônicas, nomes digitados e aridade exata:- `Stack`: `push`, `pop`, `peek`, `isEmpty`, `clear`, `count`, `toArray`
- `Queue`: `push`, `pop`, `peek`, `isEmpty`, `clear`, `count`, `toArray`
- `Math`: `abs`, `sqrt`, `pow`, `round`, `floor`, `ceil`, `min`, `max`, `random`, `sin`, `cos`, `tan`, `log`, `exp`
- `JSON`: `encode`, `decode`, `valid`, `prettify`
- `Markdown`: `toHtml`, `toHtmlSafe`, `toc`, `meta`
- `Str`: `length`, `lower`, `upper`, `contains`, `startsWith`, `endsWith`, `replace`, `split`, `trim`, `substr`, `indexOf`, `pad`, `repeat`
- `UUID`: `v4`, `v7`, `isValid`
- `Lang`: `type`, `isNumeric`, `isCallable`, `isIterable`, `methods`, `properties`, `clone`
- `Console`: `log`, `info`, `warn`, `error`, `debug`, `table`, `trace`, `clear`, `time`, `timeEnd`, `assert`
- `Zip`: `extract`

As definições são consumidas pelo analisador, pelo gerador de catálogo (`tools/cataloggen`), pelo gerador de documentação (`tools/docgen`) e pelo servidor de linguagem (LSP).

### 6. Visualização de modelos e contagem de colunas por runas UTF-8

- **Posições de origem com reconhecimento de runas**: Em `pkg/viewtemplate/directives.go`, o cálculo da coluna para diagnóstico foi ajustado para iterar sobre runas UTF-8 (`utf8.RuneCountInString`), corrigindo o deslocamento em caracteres acentuados ou multibyte e garantindo o alinhamento exato do intervalo com o analisador e LSP.
- **Teste Fuzz**: Fuzzing contínuo no extrator ZIP (`FuzzExtractPluginZip`) e no scanner de diretiva (`FuzzDirectiveScanner`).

### Dívida reorganizada (quinta fase)

- **P0:** Nenhum. Todas as inconsistências de propriedade, drivers nativos, sessões e WebSocket foram resolvidas e verificadas em testes de caracterização e detector de corrida.
- **P1:** Continuar a migração progressiva das classes nativas restantes (`GranDB`, `Crypto`, `File`, `Http`, `Router`, etc.) para `NativeMethodDefinition`; extrair sessão/CSRF de `MainHandler` para submódulos dedicados; migre as diretivas restantes (`@extends`, `@section`, `@yield`, `@include`, `@foreach`) para o scanner unificado de `pkg/viewtemplate`.
- **P2:** Estender o corpus diferencial do Analisador↔Interpretador↔VM para tipos complexos e fechamentos; expanda o tipo de difusão do analisador e ida e volta do arquivo de bloqueio.
- **P3:** Automatizar fan-in/fan-out e alterar métricas de superfície em CI; estabelecer benchmarks de desempenho comparáveis.
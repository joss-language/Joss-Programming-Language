# Auditoria abrangente da linguagem Joss — setembro de 2026

[Índice](README.md) · [Arquitetura](ARQUITECTURA.md) · [Tipos](SISTEMA_TIPOS.md) · [Diagnósticos](DIAGNOSTICOS.md)

**Escopo e método.** Revisão da árvore atual do repositório, não da tese nem de auditorias históricas como autoridade. Foram comparados parser, AST, analyzer, typesystem, runtime, servidor, CLI, VM, bytecode, plugins, mobile, ferramentas, testes, benchmarks, documentação e o projeto web de referência. `go test ./...` concluiu com código 0 durante esta auditoria. As observações que dependem de um caminho de código são indicadas como tal; não são atribuídas medições de desempenho nem explorações de segurança sem um teste específico. O relatório preserva a avaliação original e registra abaixo o progresso verificado do roadmap.

**Estado de implementação em 22 de setembro de 2026.** Os problemas P0 reproduzíveis de transações, ownership de forks, operações de canais e timeout mobile já possuem correções e testes de regressão. `PreparedProgram` conserva a AST validada e fatos semânticos reais; CLI e mobile executam seu entrypoint, enquanto server/runner ainda precisam convergir. O analyzer adicionou definite assignment conservador em loops e o aviso `JOSS-FLOW-003` para acesso direto por receptores nullable. As principais APIs nativas de filesystem, rede e processo respeitam capacidades do host, incluindo funções globais, streams, armazenamento de usuário e `run`. Este estado não declara concluídos typed IR, IDs semânticos universais, CFG completo, soundness final de coleções, cancelamento rígido de nativos bloqueantes nem expansão da VM.

**Encerramento do roadmap executável.** CLI, mobile, runner empacotado e hot reload agora passam por preparação semântica antes da execução; o hot reload constrói um runtime candidato e preserva o anterior em caso de falha. `AnalysisFacts` publica IDs estáveis, tipos por nó, chamadas resolvidas e efeitos nativos declarativos. Coleções mutáveis parametrizadas são invariantes e SQL usa o contexto cancelável do runtime. Typed IR, otimizações de dispatch e expansão da VM permanecem linhas experimentais condicionadas à equivalência diferencial e benchmarks; não são garantias publicadas nem são apresentadas como artificialmente concluídas.

## 1. Executive Summary

Joss já cumpre uma parte importante do objetivo «analisar → validar → executar»: a CLI analisa o projeto antes de executá-lo, o analyzer possui símbolos e escopos, tipos de parâmetros obrigatórios, contratos nominais, verificações de chamadas e membros conhecidos, retorno declarado, diagnósticos estruturados e detecção de certas operações aritméticas constantes. A execução publicada continua sendo um interpretador de AST em Go com planos de slots por callable. `JOSSBC2Z` empacota AST compactado e a VM é experimental; nenhum é uma fase geral de compilação semântica.

As maiores lacunas atuais estão em **ciclo de vida concorrente e recursos**, **contratos de APIs nativas e coleções**, **fluxo sensível a estados**, e **discrepância entre rotas de entrada**. A rota móvel implementa um timeout que relata falha sem interromper a goroutine; as rotas `async`, `Task` e finalização de instâncias criam forks sem liberar seu ownership; `GranDB::transaction` abre um `sql.Tx` que as consultas normais do callback não usam. Os canais delegam operações inválidas a panics de Go. Esses são achados do código atual, não propostas estéticas.

**Julgamento:** a base da linguagem é promissora e modular nas camadas de análise, mas ainda não oferece garantias equivalentes a uma linguagem compilada para programas com `mixed`, nativos, recursos externos ou concorrência. P0 deve priorizar falhas de ciclo de vida/atomicidade e testes de regressão; depois ampliar contratos estáticos. A melhoria de desempenho só deve avançar após perfilar cargas representativas.

## 2. Current Architecture

```text
.joss → parser.Lexer → parser Pratt → parser.Program / AST
                                  ├→ analyzer.LoadProject (entrypoint + app/**/*.joss)
                                  │    → collectDeclarations → projectScope
                                  │    → validateNominalContracts → analyzeSourceBodies
                                  │    → diagnostics.Diagnostic
                                  ├→ bytecode.Encode → JOSSBC2Z → runner Go → intérprete
                                  └→ core.Runtime.Execute → registro de clases/funciones
                                       → runtime/plan.Callable → runtime/frame.Slot
                                       → evaluator/executor → nativos/host/servidor
                       VM experimental ← subconjunto AST (sin ruta CLI principal)
             plugins JP ← SymbolIndex + AST/JPBC propios
```

**Fronteiras e dependências.** `pkg/parser` define tokens, lexer, precedências, AST e parser; `pkg/typesystem` define nomes, atribuibilidade e verificações de inteiros; `pkg/analyzer` importa essas camadas e `pkg/diagnostics`, mas não `pkg/core`. `pkg/core/analyzer.go` projeta built-ins, classes nativas e plugins para o ambiente semântico. `pkg/core` executa AST e integra SQL, arquivos, rede, visualizações, auth, WebSocket e plugins. `pkg/server` adapta HTTP para runtimes forkeados. `cmd/joss` orquestra o projeto; `cmd/runner` consome builds. `pkg/mobile` oferece API embutida e exportação C; `sdk/dart` adapta essa superfície e baixa binários de release. Não há um pacote separado chamado `libjoss` nesta árvore. `vscode-joss` oferece LSP; `pkg/formatter`, `pkg/linter`, `pkg/fixer` e `pkg/tester` são ferramentas separadas. `pkg/plugincompiler`, `pkg/pluginpkg`, `pkg/pluginruntime` e `pkg/vfs` formam a fronteira de plugins/pacotes. `pkg/i18n`, `pkg/template`, `pkg/viewtemplate` e `pkg/crypto` apoiam superfícies de aplicação.

**Decisões acertadas já aplicadas.** Catálogos canônicos de tokens, built-ins e métodos primitivos; assinaturas semânticas separadas da execução; frames isolados para funções nomeadas; closures com captura lexical; `ref` temporário e invariante; `Free` idempotente com `atomic.Bool`; ownership contado para drivers; snapshots de sessão; scanner compartilhado de diretivas; metadados de classe em cache. Veja [Arquitetura](ARQUITECTURA.md) e `pkg/core/runtime_lifecycle.go`, `pkg/core/call_arguments.go`, `pkg/analyzer/analyzer.go`.

**Acoplamento restante.** `core.Runtime` reúne linguagem e serviços do host (`pkg/core/types.go`); `cmd/joss/main.go` e `pkg/server/handler.go` orquestram muitas rotas; `core/evaluator_member.go` conserva buscas dinâmicas por nomes e scans de plugins. É acoplamento funcional real, mas dividir arquivos por tamanho não seria uma solução. Os contratos duplicados intencionalmente (analyzer vs runtime) devem compartilhar metadados, não estado. A fronteira de análise termina ao produzir diagnósticos: o AST não se converte em um typed IR reutilizado por todas as rotas. `runtime/plan` é calculado na execução de callables, não certificando o programa inteiro.

## 3. Language Pipeline

A sintaxe fonte real exige `$` para variáveis e visibilidade explícita em funções/classes globais. Portanto `func suma(int a, int b)` e `class Usuario` são exemplos conceituais, não Joss válido: seriam `public func suma(int $a, int $b): int { return $a + $b }` e `public class Usuario { public string $nombre }`. O laço é `foreach ($items as $item) { ... }`; `await($futuro)` é chamada nativa, não operador prefixo; `async { ... }` cria o futuro.

| Fonte | Tokens / AST principal | Analyzer e tipo conhecido | Execução e resolução pendente |
|---|---|---|---|
| `int $edad = 20` | tipo, variável, atribuição, inteiro → `LetStatement(IntegerLiteral)` | tipo declarado `int`, compatibilidade do inicializador | armazena binding tipado, `int64`; runtime revalida |
| `var $nombre = "Joss"` | `VAR`, literal string → `LetStatement` | infere e fixa `string` | slot/map com tipo inferido; revalidação ao reatribuir |
| `mixed $valor = obtenerValor()` | declaração + `CallExpression` | `mixed` explícito; assinatura de chamada se resolúvel | busca função/callable e valor efetivo em runtime |
| `public func suma(int $a, int $b): int` | `MethodStatement`, `ReturnStatement(InfixExpression)` | parâmetros, aridade, `+`, tipo de retorno e rotas de saída | planeja slots; resolve chamada, frame e operação |
| `public class Usuario { public string $nombre }` | `ClassStatement` + propriedade | contratos nominais, membros e visibilidade conhecida | metadados em cache; instância usa `Fields map[string]interface{}` |
| `await($f)` | `CallExpression` | built-in conhecido; resultado frequentemente `mixed` | `Future.Wait()`, pode bloquear ou propagar erro |
| `foreach ($items as $item)` | `ForeachStatement` | iterável é inferido; elemento se torna `unknown` no binding | executa iteração de array/map/channel/generator, conforme valor |

Os tokens precisos e a precedência vêm de `pkg/parser/token.go`, `lexer.go` e `parser.go`; a tabela resume categorias, não um rastreamento de `NextToken` instrumentado. O AST retém tokens/posições e tipos escritos; não retém um `SymbolID` nem uma referência semântica universal a cada chamada. `runtime/plan/callable.go` adiciona `IdentifierSlots` e `NameSlots` para parâmetros/locais, mas globais, propriedades, métodos, plugins e nativos continuam com buscas por nome.

Em classes/métodos: o analyzer coleta declarações antes dos corpos, valida herança/interfaces e visibilidade, e tipifica chamadas quando conhece receiver/assinatura. O runtime ainda verifica construtor, propriedade, acesso e tipo do valor efetivo. Em closures: `FunctionLiteral` captura mapas e slots atuais; ao invocar usa um frame planejado e um mutex do ambiente capturado. Em `try/catch`, `throw`, `defer`, geradores, `select`, `match` e `async`, a sintaxe existe, mas a análise de estados/efeitos é parcial. Para `null`, `T?` normaliza para união; narrowing se aplica em `is`, comparações com null e alguns `guard`/ternários (`pkg/analyzer/infer_narrowing.go`).

## 4. Type System

| Tipo/conceito | Garantia real | Limite |
|---|---|---|
| `int` | `int64`; somas/subtrações/produtos e negação protegidos contra overflow; divisão por zero protegida | valores de nativos ou `mixed` testados apenas na execução |
| `float` | `float64`; aceita atribuição a partir de `int` | inteiros maiores que 2^53 não são todos representados exatamente; revisar NaN/Inf por operação específica |
| `decimal` | `shopspring/decimal`; aceita promoção int/float; literal decimal | converter `float` pode importar seu arredondamento anterior |
| `string`, `bool` | tipos canônicos; strings UTF-8 e operações Unicode específicas | coerção de string para int/float/decimal/bool existe ao atribuir a tipo explícito; a regra deve permanecer explícita em diagnósticos |
| `array<T>`, `map<K,V>` | tipo de elemento/valor verificável ao declarar e validar valores completos; maps runtime usam chave string | não são genéricos universais; funções nativas e mutações podem retornar `unknown/mixed`; não há prova de aliasing/variância segura |
| `object`, classes, interfaces | tipo nominal, herança e interfaces do projeto; membros conhecidos são checados | campos são maps, não offsets; o receiver `mixed` requer lookup dinâmico |
| `channel` | identidade de canal | sem tipo de mensagem nem estado aberto/fechado estático |
| `mixed` | dinamismo deliberado | aceita qualquer origem/destino em `Assignable`; desloca erros para o runtime |
| `var` / primeira atribuição | infere o primeiro tipo concreto e o fixa; null adia a inferência | fluxo condicional e chamadas de retorno desconhecido reduzem precisão |
| `const` | impede reatribuição; propriedades constantes protegidas | não converte estruturas referenciadas em profundamente imutáveis |
| `T|null`, `T?` | união nullable normalizada; verificação de atribuição e narrowing local | não há análise geral de null-state interprocedural |

`typesystem.Assignable` aceita `Unknown` de maneira permissiva para evitar falsos positivos (`pkg/typesystem/types.go`); portanto «análise limpa» não significa «sem erro de tipos possível». `int $edad = "hola"` produz incompatibilidade se a string não for coercível; `int $edad = "20"` pode ser aceito mediante `CoerceString`. `var $contador = 10; $contador = "texto"` é rejeitado; `mixed` é a opção dinâmica. `ref T` é invariante e não escapa. A compatibilidade nominal é complementada em `pkg/analyzer/infer_nominal.go` e em `core.checkParsedType`; há duas defesas necessárias, mas convém testar sua concordância com um corpus comum.

**Achado de consistência:** `Assignable` trata `array<T>` e `map<K,V>` de forma covariante e aceita coleções sem argumento de tipo como destino/origem. Com contêineres mutáveis/com alias isso pode admitir uma atribuição que posteriormente permite introduzir elementos incompatíveis. A extensão real da exposição depende de cada rota de mutação: é um risco de soundness que exige regressões de aliasing antes de alterar a regra. `runtimeTypeOf` de um array/map perde os argumentos genéricos; `checkParsedType` percorre elementos para uma variável tipada, mas uma operação nativa posterior pode perder essa verificação.

**Precisão numérica:** `typesystem.Assignable` documenta `int → float` como «losslessly», mas um `float64` não pode representar cada `int64` (por exemplo, 2^53+1). A compatibilidade está implementada, não a garantia de precisão do comentário. Isso merece um teste de valor e uma política explícita: warning por conversão potencialmente inexata, cast expresso ou conservação do comportamento documentando perda. `decimal` preserva precisão decimal ao receber um inteiro; converter a partir de `float` não restaura dígitos já perdidos.

## 5. Static Safety

O analyzer detecta símbolos e classes inexistentes, duplicados, visibilidade, tipos fonte desconhecidos, inicializadores/reatribuições/argumentos/retornos incompatíveis, aridade e nomes de argumentos conhecidos, referências ilegais, membros conhecidos inexistentes, índices com tipo inválido, operações conhecidas inválidas, overflow e divisão por zero constantes, retornos demonstravelmente ausentes e código após uma saída incondicional. Emite códigos `JOSS-...` a partir de `pkg/analyzer`, apoiado por `pkg/diagnostics`. `pkg/analyzer/flow.go` é deliberadamente conservador: não constrói um CFG geral e `hasYield` considera gerador uma saída válida.

| Erro / decisão | Fase atual | Movimento razoável | Fallback |
|---|---|---|---|
| sintaxe, visibilidade obrigatória | parser | já inicial | não executar |
| variável/função/classe desconhecida | analyzer quando resolúvel | fechar diferenças de entrypoint e plugin | runtime para carga dinâmica |
| tipo incompatível / retorno / `ref` | analyzer | aprofundar aliasing e fluxo | runtime obrigatório |
| chamada/membro com `mixed` ou nativo sem assinatura | runtime | publicar assinaturas verificadas e narrowing | runtime |
| uso antes de inicialização | parcial; runtime slot `Initialized` | definite assignment via CFG | runtime |
| null dereference | narrowing parcial / runtime | análise de estado e `T?` | runtime |
| divisão por zero / índice fora de intervalo | constantes: analyzer; variáveis: runtime | propagação de constantes de baixo custo | runtime |
| canal fechado / send/recv bloqueante | runtime/Go panic | estado apenas se local e demonstrável | runtime estruturado |
| transação não vinculada / recurso não fechado | runtime/host | efeitos e ownership de APIs | runtime e testes de integração |

A regra fundamental é preservar defesas runtime: cargas dinâmicas, `mixed`, I/O e concorrência não são decidíveis em geral antes de executar.

## 6. Runtime Safety

Os erros do Joss usam `pkg/runtime/errors` e `pkg/core/errors.go`; aritmética e índices têm códigos estáveis. `MaxCallDepth` limita a recursão a 1024 frames por padrão. O runtime valida tipos de slots, campos, parâmetros, retornos, visibilidade e `ref`; o pool é limpo em `Free`. As falhas do Go originadas em APIs nativas nem sempre se convertem em `JossError`: `close(ch.Ch)` duplicado e `send` para canal fechado podem gerar panic; `make_chan` com capacidade negativa também. Isso produz uma experiência diferente das falhas aritméticas/indexação estruturadas. `await` e `recv` podem bloquear indefinidamente se não houver produtor; não existe cancelamento estruturado geral.

Classificação de controles: parser para construção impossível; analyzer/typesystem para incompatibilidade, fluxo e null demonstráveis; planner para referências e metadados estáveis; runtime para estado de canal, limites, recursos e valores externos; stdlib para contratos de rede/FS/SQL; tooling para diagnósticos, rastreamento e auditoria de dependências. FFI e plugins com permissão de host não constituem sandbox do SO: a assinatura do pacote verifica procedência/integridade, não isola efeitos. Qualquer proposta de permissões deve distinguir capacidades do host e isolamento real.

## 7. Memory Model

Os valores são armazenados em `interface{}` e estruturas Go: arrays `[]interface{}`, maps `map[string]interface{}`, instâncias com `Fields map`, strings Go e decimal. O GC do Go gerencia a memória ordinária. `executionFrame` usa slots tipados etiquetados e `sync.Pool`; `Runtime` também usa pool e `Free` limpa caches, globais, estado de request, geradores, defers e plugins. `Fork` copia tabelas, compartilha AST/planos e `*sql.DB`; clona certas instâncias/mapas/fatias apenas superficialmente (`pkg/core/runtime.go`). Uma estrutura aninhada mutável pode permanecer com alias entre forks se introduzida por um caminho não coberto; exigir teste específico antes de afirmar isolamento profundo.

`CapturedFunction` armazena um snapshot lexical em mapas protegidos por mutex, com a semântica de aliasing própria de valores internos. `ref` conserva binding durante a chamada, não ponteiro geral nem valor que escapa. O ownership de `DB` é externo ao runtime; o de drivers nativos é contado. O ownership de forks assíncronos e finalizadores não está fechado em todas as rotas. `evaluateNew` cria um `Fork()` por instância para um finalizer, e `AutoDestroy` pode criar outro fork para destruidor sem `Free` visível (`pkg/core/evaluator_member.go`, `instance_lifecycle.go`). Além disso, finalizadores do Go não garantem execução pontual; não devem ser a única estratégia para recursos críticos.

## 8. Concurrency

`async { ... }` projeta-se para o built-in `async`: cria `Future`, realiza `Fork` e executa uma goroutine; `await` aguarda seu resultado/erro. `channel`, `send`, `recv`, `close`, `foreach` sobre canal e `select` existem. `pkg/server` usa um fork por request e possui caracterizações de WebSocket, sessões e cleanup; sua segurança não deve ser extrapolada para outras goroutines. `ClosureEnvironment.mu` serializa o uso compartilhado de uma captura.

Riscos comprovados por inspeção: forks sem `Free` em `builtins_async.go` e `task.go`; tarefas sem cancelamento nem join obrigatório; `Future` pode não ser observado; operações de canal sem estado e com panics do Go; `mobile.RunDirect` retorna timeout enquanto a goroutine pode continuar usando o runtime que seu caller libera. A última rota pode ocasionar condição de corrida/uso após reciclagem, não apenas atraso. Priorizar contexto/cancelamento cooperativo e ownership explícito antes de prometer «timeout de execução» no SDK móvel. `go test -race` da suíte existente é necessário, mas não demonstra ausência de condições de corrida em intercalações não cobertas.

## 9. Performance

Há trabalho prévio útil: `runtime/plan` atribui slots por identificador AST, `runtime/frame` reduz maps para locais e `classMetadataCache` evita reconstruir hierarquias por acesso. `pkg/core/runtime_benchmark_test.go` cobre startup, operadores, laços, funções, objetos, coleções e cenários de aplicação; `pkg/vm/vm_benchmark_test.go` cobre apenas um subconjunto. Não há nesta auditoria perfis de CPU/heap que permitam declarar um hot path dominante nem prometer porcentagens.

Candidatos para **medição**: `evaluateNew` busca classe de plugin percorrendo registro/classes e cria fork/finalizer por instância; `evaluateMember` e despacho nativo resolvem nomes; classes guardam campos em mapas; `typesystem.Parse` de uniões refaz parse de nomes; coerção e validação de coleções percorrem valores; closures copiam mapas; `Fork` copia vários registros. Um cache de método monomórfico/polimórfico ou IDs só tem valor para classes estáveis e com invalidação clara. Antes de otimizar: bench de chamadas/propriedades em frio/quente, `benchmem`, perfis pprof e comparação semântica de caminho rápido/geral.

## 10. Diagnostics

`diagnostics.Diagnostic` contém código, severidade, arquivo, intervalo, mensagem, explicação e sugestão; o analyzer ordena por arquivo/linha/coluna. Parser e LSP consomem diagnósticos. Arith/index runtime possuem códigos; vários nativos ainda lançam strings ou panics do Go sem código nem span preciso. Um `JossError` suporta stack, mas a tradução em CLI/móvel/servidor não possui apresentação idêntica. O parser recupera erros parcialmente e pode emitir derivados após o primeiro. A prioridade é alinhar erros nativos frequentes e adicionar localização/intervalo completo à falha runtime, sem inventar um novo código se já existir um canônico. Os exemplos de `docs/DIAGNOSTICOS.md` devem acompanhar cada código novo.

## 11. Tooling

A CLI implementa `run`, `build`, `check`, `analyze`, `test`, `format`, `lint`, `fix`, `eval` e **`repl`** (`cmd/joss/main.go`), além de comandos de aplicação/pacotes. `format` preserva trivia com scanner próprio que consome símbolos canônicos; `fix` usa regras/regex de transformação e necessita de testes contra strings/comentários. `vscode-joss` declara completion, hover, definition, references, signature help, diagnostics, document symbols e formatting; não consta provedor de rename em `server.ts`. Há testes de parser fuzzing, typesystem fuzzing, valor Unicode, diferencial de VM e benchmarks. Faltam debugger/profiler de Joss integrados e um inspetor de efeitos/dependências de projeto; «ausente» não implica prioridade alta.

**Divergências documentais concretas:** [Estado de implementação](ESTADO_IMPLEMENTACION.md) afirma que não há REPL, interfaces, `defer` nem `select`; CLI, AST, analyzer e executor de fato os possuem. O relatório histórico que ocupava esta página também afirmava ausência de `guard`/narrowing posterior, mas `body_analysis.go` e `infer_narrowing.go` já implementam ambos. `sdk/dart/README.md` descreve distribuição mobile que deve ser verificada contra artefatos de release reais antes de prometer instalação automática. Corrigir essas páginas na fase documental posterior, com contratos executáveis; não usar suas afirmações antigas como base de projeto.

## 12. Technical Debt

1. **HIGH:** contratos de coleções mutáveis e `Unknown/Mixed` deixam brechas de soundness; `typesystem.Assignable` e `runtimeTypeOf` necessitam de testes de aliasing e política explícita.
2. **HIGH:** rotas de entrada não compartilham um `PreparedProgram` com análise e metadados persistidos; CLI, mobile, runner e server orquestram análise/registro de forma distinta.
3. **HIGH:** ownership de forks fora do HTTP não está expresso em tipos/API e permite esquecimentos.
4. **MEDIUM:** metadados nativos publicam retornos confiáveis, mas `ArityKnown=false` em muitas APIs; o analyzer não pode antecipar aridade/tipos.
5. **MEDIUM:** metadados de métodos/classes e resolução de plugin ainda usam strings/mapas e scans em rotas potencialmente frequentes.
6. **MEDIUM:** `pkg/core` integra semântica e infraestrutura; separar unicamente os pontos onde contratos/testes de fronteira justifiquem.
7. **MEDIUM:** documentação de estado e auditorias históricas contradizem funcionalidades atuais; a auditoria da linguagem não pode basear-se nessas páginas sem revalidar.
8. **LOW:** símbolos LSP gerados convivem com assinaturas ricas mantidas manualmente; risco de metadados incompletos.

## 13. Bugs Found

| Severidade | Evidência e causa raiz | Efeito / verificação pendente |
|---|---|---|
| **CRITICAL** | `pkg/core/database.go` ramo `transaction` abre `tx := db.Begin()`, chama callback mediante `r.CallFunction` e confirma `tx`; as consultas ordinárias usam `r.GetDB()`, sem contexto `tx`. Já consta como limite em [Estado](ESTADO_IMPLEMENTACION.md). | Uma operação do callback pode persistir mesmo que este falhe e ocorra rollback do `tx`. Teste SQLite em `t.TempDir`: insert + throw + verificar tabela vazia. |
| **HIGH** | `pkg/mobile/mobile.go` seleciona `time.After` mas não cancela nem aguarda a goroutine; restaura `os.Stdout/Stderr` e `defer rt.Free()` pode reciclar o runtime ainda ativo. | Timeout não é interrupção de execução; risco de goroutine persistente, saída tardia e condição de corrida. Teste de timeout com sinalização/join e `-race`. |
| **HIGH** | `pkg/core/builtins_async.go` e `pkg/core/task.go` fazem `r.Fork()` sem `Free()` na goroutine; `evaluator_member.go` e `instance_lifecycle.go` repetem padrão em finalizadores/destruidor. | Estado/handles retidos e ownership contado não liberado. Teste de driver retido e ciclos de fork; definir encerramento do owner. |
| **HIGH** | `builtins_async.go` executa `close(ch.Ch)`, `ch.Ch <- value` e `make(chan, size)` sem controlar fechamento duplo, envio em canal fechado ou capacidade negativa. | Panic do Go ou bloqueio sem erro estruturado Joss. Testes dos três casos e concorrente com `-race`. |
| **MEDIUM** | `typesystem.Assignable` permite `int → float` e descreve como sem perda, embora `float64` perca precisão acima de 2^53. | Resultado numérico surpreendente em IDs/contadores grandes. Teste de 2^53+1 e decisão de compatibilidade. |
| **MEDIUM** | `pkg/analyzer/project.go` carrega entrypoint e `app/**/*.joss`; `routes.joss` é carregado pelo servidor conforme [auditoria documental](DOCUMENTATION_AUDIT.md). | `joss analyze` pode omitir erros de rotas; verificar fixture web e unificar manifesto de fontes de execução. |
| **MEDIUM** | `docs/ESTADO_IMPLEMENTACION.md` nega REPL/interfaces/defer/select; implementação presente em `cmd/joss/main.go`, AST e `core/executor.go`. | Decisões de usuários e roadmap baseados em informação errônea. Corrigir com contratos de documentação. |

Não são atribuídos como bugs atuais o antigo `match` de blocos, o escape de `break` em ternário ou o reset do pool: há rotas/guardas posteriores e requerem nova reprodução antes de reabrir. Os achados de código anteriores ainda necessitam de testes de regressão ao serem corrigidos; a inspeção não substitui um teste end-to-end.

## 14. Missing Language Features

Problemas reais: falta análise geral de definite assignment e null-state; não há tipo de mensagem para canais nem cancelamento de tarefas; não há tipo de resultado para falhas esperadas de I/O; não há contrato de efeito/capacidade para chamadas nativas; a precisão de coleções se perde com APIs `mixed`. `match` de valores existe, mas não é exaustivo por enum/tipo união. Interfaces e enums **de fato existem**; não devem ser propostos como ausentes. Imports fonte não existem por decisão explícita de projeto zero-imports; adicioná-los agora quebraria a arquitetura sem solucionar o problema prioritário. Tampouco se justifica ownership geral no estilo Rust, genéricos universais, traits, AOT/LLVM ou uma VM completa nesta fase.

## 15. Proposed Language Features

Cada sintaxe é uma **proposta**, não um contrato atual.

| Proposta e exemplo | Problema | Parser/AST | Analyzer | Runtime/tooling | Compatibilidade; complexidade |
|---|---|---|---|---|---|
| **Definite assignment e null-state**: `T? $x`, guard e acesso posterior | uso antes de inicializar / dereference nullable | sem sintaxe nova; CFG sobre AST atual | estados por símbolo e joins; narrowing invalidado por mutação/alias | defesas conservadas; hover mostra estado | compatível com warning → erro opt-in; média |
| **`Result<T,E>` leve**: `Result<Usuario, DbError>` com `match` | falhas esperadas de DB/HTTP hoje misturam nil, false, panic | gramática de tipo parametrizado já existe para coleções; ampliar AST de type refs e construção | variantes e exaustividade; propagação explícita apenas se projetada | valor tagged e SDK; LSP completion | experimental; alta |
| **Canal tipado e fechamento seguro**: `channel<string>` | enviar valor incorreto/fechar duas vezes | estender TypeReference/AST sem alterar `send` | verificar tipo de mensagem e diagnósticos locais de estado | wrapper de estado, erro Joss, hover; testes race | compatível opt-in; média |
| **`match` exaustivo para enum/união** | ramo faltante deriva para null | AST atual possui braços/default; possível padrão simples novo | cobertura de casos e duplicados | runtime conserva fallback; quick fix LSP | warning primeiro; média |
| **Atributo de efeito para nativos confiáveis**, p. ex. metadata `effects: io, blocking` (sem sintaxe fonte inicialmente) | analyzer desconhece bloqueio/recursos | nenhuma alteração parser/AST no início | validar `await`, recursos e rotas críticas usando assinaturas | catálogo e LSP; wrapper host | compatível; média |

Não recomendar `readonly` superficial enquanto mapas/fatias com alias permanecerem mutáveis; uma garantia parcial deve ser chamada explicitamente de «binding imutável». Safe casts só trariam valor após definir falhas como `Result`/nullable e testar a interação com `mixed`. Records/sealed classes podem ser reavaliados após exaustividade nominal; extension methods/traits e overloads adicionam resolução complexa sem bug demonstrado que os exija.

**Comparação seletiva:** Kotlin/Dart/Swift inspiram null-state e promoção local, mas o Joss precisa invalidar narrowing ao escrever `mixed` ou aliases; TypeScript demonstra a utilidade e os limites do controle de fluxo com dinamismo; Rust traz `Result` e recursos explícitos, sem tornar necessário borrow checking completo; Go fornece canais e contextos de cancelamento, enquanto o Joss deve evitar panics visíveis do Go; Java/C# mostram metadados estáveis e despachos antecipados, úteis apenas para receptores nominais; Lua/JVM demonstram VM viável após equivalência semântica; Python/PHP lembram o valor de `mixed` e iteração rápida, junto ao custo de falhas tardias. Copiar sintaxe externa sem resolver contratos internos não oferece garantias.

## 16. Compiled-Like Experience

Arquitetura objetivo incremental, derivada das camadas atuais:

```text
parser.Program + SourceUnits
  → analyzer: símbolos, tipos, contratos, CFG/estados
  → diagnostics + AnalysisFacts (ID de símbolo, tipo, efectos, spans)
  → PreparedProgram (AST inmutable + planes de callable/clase + referencias resueltas)
  → intérprete core (ruta publicada) / VM sólo para subset diferencial
  → runtime host (recursos, IO, requests, plugins)
```

Primeiro adicionar fatos semânticos **sidecar** indexados por nós AST, evitando mutar AST compartilhado por forks. IDs imutáveis por projeto/versão podem estabilizar símbolos; `SlotID` já existe para locais. Resolver antecipadamente funções e métodos apenas quando classe e tabela forem estáveis; `mixed`, plugins dinâmicos e host globals mantêm guardas/fallback. Um typed IR completo teria alto custo de duplicação semântica: exigir protótipo com comparação diferencial e métricas antes de adotá-lo. CFG para retornos, atribuição e null-state tem valor antes de otimizações. Constant folding/propagation apenas para operações puras com `typesystem.CheckedIntBinary`; nunca antecipar efeitos de nativos. Field offsets e inline caches necessitam de versão/invalidação de classe; não são primeira fase.

## 17. Architecture Improvements

1. Um `PreparedProgram` por projeto com origem de arquivos, diagnósticos, `AnalysisFacts`, planos e catálogo de assinaturas; CLI/mobile/runner/server consomem a mesma validação. Manter `analyzer` sem importar `core`.
2. Uma API de ownership para forks/tarefas com `defer Free()` obrigatório na goroutine e finalização explícita de instâncias com recursos. Não transferir ownership de `DB` para o pool.
3. Um contexto de transação no adaptador de DB, propagado a todas as operações do callback; não basta envolver `Begin/Commit`.
4. Metadados `NativeMethodDefinition` com parâmetros e efeitos apenas onde o contrato for comprovado; `ArityKnown=false` deve permanecer para casos desconhecidos.
5. Manifesto único de arquivos fonte do projeto que cubra rotas web realmente executadas; preservar política zero-imports.
6. Contextos de erro com código/span/stack em fronteiras nativas e SDK, sem managers públicos cerimoniais.

## 18. Security Improvements

Prioridade: transações atômicas e gerenciamento de recursos; depois, cancelamento cooperativo de requests/tarefas; capacidades explícitas para filesystem/rede/processo/FFI em host/plugin; limites de tamanho, tempo e profundidade em nível de operação. Verificar rotas e VFS contra traversal, e não confundir assinatura JP com sandbox. Para `null`, índice, divisão e overflow, o analyzer detecta constantes/estados demonstráveis e o runtime conserva verificações. A propagação de erros async deve ser observável mesmo se o Future não for awaited; uma política de tarefas órfãs (log/propagação para o request ou cancelamento) necessita de especificação antes de ser implementada. `panic` de um nativo deve se converter em erro estruturado sem silenciar falhas internas não previstas.

## 19. Performance Improvements

Hipóteses com benchmark requerido: pré-indexar classes exportadas de plugins para `new`; reduzir forks/finalizers por instância; compartilhar metadados de classe imutáveis entre forks; resolver receiver nominal para método pré-computado; cache de acesso a propriedade com invalidação; especializar operadores de tipos conhecidos em planos. A primeira melhoria pode ser de **segurança e memória** além de tempo. Métricas mínimas: ns/op, B/op, allocs/op, p50/p95 de request, heap retido por 10k objetos/futures, custo de cold start e pprof. Rejeitar uma otimização se quebrar equivalência, aumentar retenção ou beneficiar apenas microbenchmarks irrelevantes.

## 20. Roadmap

As prioridades são P0 (falha de segurança/corretude), P1 (garantia central), P2 (evolução condicionada), P3 (opcional). Cada tarefa parte de problema e evidência anteriores; ao executá-la: reproduzir → caracterizar → comparar alternativas → corrigir → testes → benchmark se aplicável.

### Phase 1 — Correctness

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| Transação real | P0 | Callback opera fora de `sql.Tx`; introduzir executor transacional em toda consulta do callback, rollback em erro | `core/database*.go`, testes SQL | compatível, correção de bug | risco médio por nesting; atomicidade alta | SQLite rollback/commit/nested/error |
| Ownership de forks | P0 | forks async/task/finalizer sem encerramento; `defer Free`, política de destruidor e recursos | `core/builtins_async.go`, `task.go`, `evaluator_member.go`, `instance_lifecycle.go`, lifecycle | compatível | risco médio; memória/handles alta | owner count, GC, panic, race |
| Timeout móvel | P0 | goroutine sobrevive e runtime é liberado; cancelamento cooperativo + join ou isolar processo se prometer hard timeout | `mobile/mobile.go`, `core/executor.go` | depreciação do timeout «duro» se não alcançado | risco alto; isolamento alto | loop infinito, IO bloqueante, race, stdout |
| Canais seguros | P0 | panics do Go por close/send/capacidade; wrapper de estado e erros Joss | `core/builtins_async.go`, channel tests | compatível exceto tipo de erro | risco médio; estabilidade alta | fechado/duplicado/negativo/concorrente |

### Phase 2 — Type Safety

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| Soundness de coleções | P1 | covariância/aliasing e perda de tipo; decidir invariância ou wrapper revalidado e documentar | `typesystem/types.go`, analyzer, core collections | warning → depreciação se quebrar | risco alto; segurança alta | alias, aninhado, mutação, nativos |
| Assinaturas nativas | P1 | aridade/tipos desconhecidos; completar apenas contratos confiáveis | `core/native_signatures.go`, analyzer, catálogo | compatível com warning | risco baixo/médio; DX alta | paridade assinatura-handler, negativos |

### Phase 3 — Static Analysis

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| CFG e definite assignment | P1 | inicialização/retorno parcialmente resolvidos; CFG mínimo por callable com joins | `analyzer/flow.go`, `body_analysis.go` | warning → erro opt-in | risco médio; segurança alta | ramos, laços, try/catch, defer, yield |
| Null-state/exaustividade | P1 | dereference e match incompleto; narrowing com invalidação e cobertura de enums | `analyzer/infer_narrowing.go`, `flow.go`, LSP | warning → erro opt-in | risco médio; DX alta | alias, mutação, união, defaults |
| Fonte web completa | P1 | `routes.joss` fora da análise de projeto; manifesto de execução compartilhado | `analyzer/project.go`, CLI/server | compatível | risco baixo; erros iniciais | projeto web fixture |

### Phase 4 — Runtime Safety

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| Cancelamento e erros nativos | P1 | bloqueio/panics sem código; contexto por execução e adaptadores de erro | `core`, server, mobile, runtime/errors | compatível exceto mensagens | risco alto; estabilidade alta | timeout/IO/canal/panic/stack |
| Capability host | P2 | FFI/FS/rede sem contrato declarativo universal; permissões de host comprovadas | pluginruntime/core/native | experimental | risco alto; segurança alta | rejeição e autorização por recurso |

### Phase 5 — Execution Architecture

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| AnalysisFacts/PreparedProgram | P2 | análise e planos não persistidos uniformemente; sidecar imutável, IDs, fontes | analyzer, runtime/plan, core, CLI/mobile | compatível internamente | risco alto; previsibilidade alta | diferencial rota prévia/nova, invalidação |
| VM seletiva | P3 | VM cobre subconjunto; ampliar apenas após corpus diferencial | `pkg/vm` | experimental | risco alto; potencial desempenho | diferencial, fuzz, bench |

### Phase 6 — Language Features

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| Canal tipado + Result | P2 | erros de mensagem/I-O tardios; protótipos independentes, sem impor adoção | parser, typesystem, analyzer, core, docs | experimental | risco alto; API clara | sintaxe +/-; analyzer; runtime; LSP |
| Match exaustivo | P2 | faltam casos nominais; warning antes de erro | analyzer, docs, LSP | compatível com warning | risco médio; robustez | enums/uniões/default |

### Phase 7 — Tooling and Documentation

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| Corrigir estado/documentação | P1 | páginas negam funcionalidades existentes; atualizar a partir do código, espelho e traduções | `docs/*.md`, espelho, traduções | compatível | risco baixo; confiança alta | contratos, navegação, docsi18n |
| Rename e rastros | P2 | LSP sem rename, depuração limitada; usar IDs semânticos antes de editar referências | `vscode-joss`, analyzer | compatível | risco médio; DX alta | workspace edits, shadowing |

### Phase 8 — Performance

| Tarefa | Prioridade | Problema / solução | Arquivos afetados | Compatibilidade | Risco / benefício | Testes necessários |
|---|---|---|---|---|---|---|
| Perfilar e otimizar hot paths | P2 | scans/mapas/forks potencialmente caros; pprof primeiro, cache com invalidação depois | `core/evaluator_member.go`, runtime/plan, class metadata, plugin registry | compatível internamente | risco médio/alto; desempenho a medir | benchmem, pprof, diferencial, race |

### Matriz de recomendações

| Proposta | Segurança | Estabilidade | Desempenho | DX | Complexidade | Prioridade |
|---|---|---|---|---|---|---|
| Transação vinculada a Tx | alta: evita rollback falso | alta | neutra | média | média | P0 |
| Encerramento de forks e timeout móvel | alta: evita estado concorrente reciclado | alta | média por retenção | média | alta | P0 |
| Canais com erros Joss | alta: evita panic do host | alta | neutra | alta | média | P0 |
| CFG/null-state | alta: adianta falhas | média | neutra | alta | média | P1 |
| Coleções sound | alta: protege aliasing | alta | possível custo de verificação | média | alta | P1 |
| Assinaturas nativas verificadas | média | média | neutra | alta | média | P1 |
| PreparedProgram/IDs | média | média | potencial alta | média | alta | P2 |
| VM ampliada | baixa até equivalência | incerta | potencial alta | baixa | alta | P3 |

As categorias descrevem causalidade e custos demonstráveis no código; «potencial» significa que falta benchmark, não pontuação inventada. A ordenação preserva semântica antes de otimizar. Não se recomenda iniciar a implementação desta folha de rota até revisar e aceitar seus contratos de comportamento e testes de regressão.

**Verificação desta edição.** `go test ./...`, `go vet ./...`, `go build ./...`, `go run ./tools/cataloggen --check`, `go run ./tools/docgen --check`, `go test ./pkg/core -run TestDocumentation -v`, `npm run compile` e `git diff --check` concluíram com código 0. Uma cópia temporária do VS Code completou `npm ci --ignore-scripts` e `npm run compile`; o `npm ci` normal falhou com `EPERM spawn` tanto no checkout quanto em uma cópia limpa, de modo que seus scripts de instalação não ficaram validados. O Go imprimiu advertências do host por ACL de telemetria/cache sem afetar os comandos aprovados. `go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core` não pôde compilar `runtime/race` neste host (`package testmain: cannot find package`); não é apresentado como aprovado. `go run ./tools/docsi18n -check` relatou traduções e espelhos desatualizados pré-existentes e, após esta edição, versões en/pt do relatório pendentes de tradução. O espelho espanhol do JosSecurity foi sincronizado byte a byte; a tradução completa permanece como tarefa documental, sem alterações semânticas.

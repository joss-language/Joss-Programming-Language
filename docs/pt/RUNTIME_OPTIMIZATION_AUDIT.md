# Auditoria de tempo de execução antes de otimizar [Índice](README.md) Status auditado: commit `9d27239`, Go 1.26.2, Windows/amd64. Este documento descreve o tempo de execução antes das otimizações. As medições reproduzíveis estão em `benchmarks/BASELINE_9D27239.md`. ## Fluxo real```text
.joss
  -> lexer (Token por valor)
  -> parser Pratt (nodos AST enlazados por interfaces)
  -> analyzer (scopes y símbolos por map[string]*symbol)
  -> diagnostics
  -> Runtime.Execute
  -> executeStatement (type switch)
  -> evaluateExpression (type switch)
  -> helper especializado
  -> valor Go en interface{}
```
`pkg/bytecode` não altera esse fluxo: ele restaura via gob+flate o mesmo AST pelo qual o avaliador retorna. Não há IR digitado ou resolução semântica persistente entre o analisador e o tempo de execução. ## Camadas percorridas por uma expressão Uma expressão normal de nível superior atravessa pelo menos quatro decisões: 1. `Execute` registra instruções e seleciona script/modo principal. 2. `executeStatement` faz uma troca de tipo de instruções. 3. `evaluateExpression` faz outro tipo de troca de expressões. 4. O auxiliar (`evaluateInfix`, `evaluateMember`, `executeCall`, etc.) discrimina novamente os tipos Go, procura por nomes ou percorre ASTs. Uma chamada adiciona avaliação e materialização de argumentos, resolução de nomes, classificação chamável, criação de quadros, ligação e verificação de parâmetros, execução de corpo, retorno `panic/recover` e validação de tipo de retorno. Um acesso de método adiciona pesquisa de classe, travessia de herança, travessia linear das instruções de cada classe e construção de um `BoundMethod`. ## Representações e estruturas criadas | Conceito | Representação anterior | Custo relevante | |---|---|---| | Valores | `interface{}` | boxe de primitivos quando eles escapam; interruptores de tipo repetido | | Variáveis ​​| `Runtime.Variables map[string]interface{}` | hash por leitura/gravação local e global | | Tipos de tempo de execução | `VarTypes map[string]string` | análise repetida do nome do tipo | | Constantes | `Constants map[string]bool` | pesquisa separada | | Quadro | substituição temporária de três mapas do `Runtime` | três novos mapas por chamada e cópia dos globais | | Recursos | `map[string]*parser.MethodStatement` | resolução por nome e execução do AST original | | Fechamentos | AST + cópia de três mapas | cópia completa na captura; mutex por ambiente capturado | | `ref` | três mapas + nome em `VariableReference` | pesquisa por nome e possível referência aninhada | | Objetos | `Instance{Class, Fields map, Constants map}` | propriedade por hash; metadados/método por travessia AST | | Matrizes | `[]interface{}` | elementos em caixa; literais crescem com `append` sem capacidade | | Mapas | `map[string]interface{}` | chaves avaliadas dinamicamente; literais sem capacidade inicial | | Cordas | `string` Vá para UTF-8 | indexação anterior por byte, não por caractere | | Futuros | goroutine + tempo de execução clonado + `chan bool` | fork copia mapas mesmo para um valor trivial | | Canais | `chan interface{}` | fronteira dinâmica sem contrato de elemento | O pacote `pkg/core` continha 104 arquivos Go e 17.623 linhas. A pesquisa estática encontrou 651 menções de `interface{}`, 243 de `map[string]interface{}`, 17 usos de reflexão, 100 chamadas para `panic` e 29 para `recover`. Esses números incluem stdlib/framework e testes; Nem todos estão no caminho certo, mas quantificam a concentração de responsabilidades. ## Mapas de execução ### Caminho ativo```text
while
  -> evaluate condition
     -> identifier -> Variables[name]
     -> infix -> conversión numérica + operador
  -> execute block
     -> postfix
        -> identifier -> Variables[name]
        -> box int64
        -> Variables[name] = value
```
O perfil de CPU do loop de 10.000 iterações coloca `evaluateExpression` em 82,13% cumulativamente, `evaluateInfix` em 30,88%, `evaluatePostfix` em 46,71% e primitivos de mapa/hash entre os custos fixos mais altos. ### Caminho de alocação```text
call
  -> []interface{} de argumentos
  -> frameVariables map
  -> frameVarTypes map
  -> frameConstants map
  -> parameterNames map
  -> ReturnPanic heap object
```
Em chamadas aninhadas, `callMethodEvaluated` representa 93,12% do espaço alocado. No loop, `evaluatePostfix` representa 86,18% do espaço alocado: o novo `int64` escapa sendo salvo como `interface{}`. ### Caminho de envio```text
CallExpression
  -> evaluar argumentos
  -> IsBuiltin map lookup
  -> hasta cinco switches de familias builtin
  -> Functions[name]
  -> Variables[name]
  -> applyFunction
     -> PluginCallable / CapturedFunction / BoundMethod /
        MethodStatement / FunctionLiteral / NativeHandler /
        func([]interface{}) / reflect.Func
```
A resolução do método do usuário percorre instruções de classe em cada acesso. A resolução de classe/método de plug-in pode percorrer todos os plug-ins e seus símbolos. Os métodos estáticos também criam uma instância fictícia. ### Caminho do erro Quatro políticas coexistem: - `panic(*JossError)` para alguns erros de idioma; - `panic(string)` ou `panic(error)` para outros; - `fmt.Print*` seguido de `nil` para falhas recuperáveis; - digitou panics (`ReturnPanic`, `BreakPanic`, `ContinuePanic`) para fluxo normal. `try/catch`, chamadas, loops, assíncronos e vários componentes da estrutura recuperam pânicos com diferentes critérios. O erro estruturado anterior não tinha código estável, pilha Joss, contexto, dica ou causa. ### Caminho de verificação de tipo```text
binding/assignment/call/return/property write
  -> typesystem.Parse(typeName string)
  -> runtimeTypeOf(interface{})
  -> typesystem.Assignable
  -> si es instancia: recorrido de herencia
```
O analisador já conhece tipos de símbolos, parâmetros, retornos e membros, mas essa informação não fica registrada no AST. O tempo de execução analisa novamente cadeias de tipos, resolve ligações, localiza membros e valida contratos. `mixed`, plugins e bytecode externo forçam a preservação de um caminho lento e dinâmico; Eles não exigem penalizar sites de chamadas verificados. ## Semântica auditada - `mixed` é representado como uma string de tipo e valor `interface{}`; desativa intencionalmente a incompatibilidade estática/dinâmica. - `ref` não expõe ponteiros Go; preserva os mapas de ligação e o nome. Não pode cruzar plugins, manipuladores assíncronos ou nativos. - Exceções e o controle `return/break/continue` usam panic/recover. - `async` clona o tempo de execução antes de iniciar a goroutine. Registros imutáveis ​​e de banco de dados são compartilhados; variáveis, tipos, constantes, mapas, fatias e instâncias são parcialmente copiados. - Os fechamentos retidos copiam o ambiente e serializam seu uso com um mutex. - Matrizes e mapas são heterogêneos. O analisador conhecia apenas `array`/`map`, portanto, um índice retornou `mixed`/`unknown`. - A reflexão Go aparece principalmente em builds de array e substitutos que podem ser chamados de host. - A aritmética acima converteu inteiros para `float64` e depois para `int64`. Isso perde precisão acima de 2 ^ 53 e o overflow de `int64` ficou silencioso. - `string[index]` usado `str[idx]`: retornou um byte UTF-8 convertido em uma string. ## Hotspots classificados | Prioridade | Ponto de acesso | Evidência | |---|---|---| | P0 | Variáveis ​​locais em mapas + boxe postfix | loop: ~2,10 ms, 9.752 alocações; hash/mapa domina a CPU; postfix 86,18% de alloc_space | | P0 | Construção de caixilhos por chamada | simples: 760 B/7 alocações; aninhados: 2.312 B/23; `callMethodEvaluated` 93,12% de alloc_space | | P1 | Resolução repetida de métodos/campos | perfil do objeto: `evaluateMember` 16,18% cumulativo e `lookupInstanceFieldOwner` 6,08% | | P1 | Retorno por pânico/recuperação | cada chamada de retorno cria `ReturnPanic`; aparece em 65,54% do acumulado do perfil de alocações de chamadas aninhadas | | P1 | Carregamento AST serializado | ~255 us, 80.208 B e 857 alocações para o programa de startups | | P1 | Aritmética inteira via float64 | perda de precisão demonstrável e trabalho duplicado por operação | | P2 | Despacho integrado por cascata de switches | pesquisa inicial seguida por até cinco famílias | | P2 | Literais sem capacidade e metadados por string | pequenos arrays/mapas alocam mais do que o necessário; tipos são analisados ​​novamente | | P2 | Fork completo por assíncrono | ~ 4,17 us, 1.680 B e 18 alocações para async+await trivial | | P3 | Reflexão sobre compatibilidade de host | não apareceu como hotspot em programas Joss puros; conservar como caminho lento | | P3 | Estrutura DB/HTTP/IO | arquitetonicamente importante, mas não dominou o perfil do avaliador puro | ## Simultaneidade e GC O conjunto pré-existente passa:```text
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
```
O perfil de bloqueio assíncrono atribui o bloqueio esperado a `Future.Wait` e `runtime.chanrecv1`; não mostrou nenhuma contenção de mutex do aplicativo. O perfil mutex era quase inteiramente runtime/GC. Isso não prova ausência de riscos em plugins/framework: apenas estabelece a linha de base coberta pelos testes. ## Conclusão antes das mudanças A primeira otimização deve eliminar alocações e hashing dentro do loop e nas chamadas antes de introduzir uma nova representação `Value`. As evidências ainda não justificam a substituição global de `interface{}` ou a construção de uma VM completa. Isso justifica manter um caminho AST e adicionar metadados pré-analisados, quadros compactos/caches simples e caminhos rápidos de números inteiros seguros.
# Contribua para Joss

[Índice](README.md) · Antes: [arquitetura](ARQUITECTURA.md) · [Auditoria de documento](DOCUMENTATION_AUDIT.md)

Comece com [AGENTS.md](../../AGENTS.md), a arquitetura, tipos, diagnósticos e os testes
do subsistema. O código e os testes executados são a fonte da verdade;
uma tese, comentário ou documento histórico pode descrever um objetivo diferente.

## Fluxo de uma contribuição

1. Escreva o contrato observável e seus limites.
2. Localize a fonte canônica, sem criar outra lista paralela.
3. Adicione o caso válido e o vizinho inválido quando apropriado.
4. Implemente a mesma regra nas camadas afetadas.
5. Atualizar tutorial, referência, catálogo e diagnóstico relacionados.
6. Execute as validações nesta página.

## Alterar idioma

A nova sintaxe normalmente percorre token/lexer, analisador Pratt, AST, analisador e avaliador
. Define precedência e associatividade, recuperação de erros e exemplos.
Um operador requer regras de tipo e defesa de tempo de execução. Não adicione `if`, importações, namespaces
ou aliases retirados como um atalho de compatibilidade: essas são decisões explícitas de linguagem
.

Para um tipo, comece em `pkg/typesystem`: Tipo, nome canônico, atribuibilidade, inferência e coerção
. Em seguida, integre-o ao analisador/tempo de execução e toque no analisador apenas se
houver uma nova sintaxe. Atualiza [tipos](SISTEMA_TIPOS.md) e regenera o catálogo.

Um diagnóstico público usa código estável `JOSS-...`, gravidade, arquivo/intervalo, explicação e sugestão de
. Adiciona evidências suficientes e protege contra vizinho falso positivo
. Documente o código em [diagnóstico](DIAGNOSTICOS.md).

## Adicionar APIs nativas

Para um integrado global, o nome está em `pkg/core/builtins.go`; Deve haver
um caso alcançável em um dos despachantes e um retorno em
`native_signatures.go`. Para uma aula use o registro executado por
`Runtime.RegisterNativeClasses()` e `GetNativeClassMethods()`. Publicar apenas um nome
sem um manipulador cria uma API fantasma; implementar apenas um caso sem registrá-lo
cria código inacessível.

Adicione parâmetros aos metadados quando suportado; não invente aridade no analisador
. Atualize os contratos [MODULOS_NATIVOS.md](MODULOS_NATIVOS.md), execute
`go run ./tools/docgen` e verifique o exemplo em contexto real.

Para plug-ins, mantenha JOSSBC2Z, JPBC e a VM experimental separados. Todos novos
O estado mutável do tempo de execução precisa de uma decisão de copiar/compartilhar, limpeza em
`Free` e um teste simultâneo. Uma operação de host confidencial deve cruzar um limite de permissões
real; Declará-lo em metadados não é suficiente.

## Visualizações, editor e publicação

As palavras-chave são projetadas com `parser.KeywordNames()`. O Código VS consome
`vscode-joss/src/server/generated/languageCatalog.json`; Nunca é editado à mão.
Os guias canônicos em espanhol estão disponíveis em `docs/*.md`; Inglês e Português mantêm
o mesmo nome de arquivo em `docs/en` e `docs/pt`. A versão
JosSecurity com versão usa `assets/docs/{es,en,pt}` e deve corresponder byte por byte com cada idioma de origem
. Após modificar o espanhol, execute
`go run ./tools/docsi18n -translate -sync`; O manifesto hash evita trabalho desnecessário de
e `go run ./tools/docsi18n -check` detecta traduções obsoletas,
ausentes, links locais quebrados e diferenças de espelho público.

Exemplos verificáveis ​​completos usam marcadores `joss-run`, `joss-check` ou
`joss-error` imediatamente antes de sua cerca. `documentation_test.go` analisa
todos os três e executa `joss-run`, comparando as linhas de saída. Use `joss-check` para fragmentos
que dependem de servidor, banco de dados ou plugins e explique esse contexto.

## Validação

```sh
gofmt -w archivos_go_modificados
go run ./tools/cataloggen --check
go run ./tools/docgen --check
go run ./tools/docsi18n -check
go vet ./...
go test ./...
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
go build ./...
```

Em `vscode-joss`: `npm ci` e `npm run compile`. Crie um binário temporário de
`./cmd/joss` e analise o projeto de integração real. Alterações de modelo, migrações
ou CRUD devem passar as matrizes nomeadas em AGENTS.md.

Uma revisão documental deve procurar links locais, barreiras Joss, termos legados, índices
e diferenças entre registro e despachante. Evite reivindicar portabilidade, atomicidade
, segurança ou compatibilidade total sem provas que comprovem isso.

# Contribua para Joss

[Índice](README.md) · Antes: [arquitetura](ARQUITECTURA.md) · [Auditoria documental](DOCUMENTATION_AUDIT.md)

Comece com [AGENTS.md](../../AGENTS.md), a arquitetura, tipos, diagnósticos e o
testes de subsistema. O código e os testes executados são a fonte da verdade;
uma tese, comentário ou documento histórico pode descrever um objetivo diferente.

## Fluxo de uma contribuição

1. Escreva o contrato observável e seus limites.
2. Localize a fonte canônica, sem criar outra lista paralela.
3. Adicione o caso válido e o vizinho inválido quando apropriado.
4. Implemente a mesma regra nas camadas afetadas.
5. Atualizar tutorial, referência, catálogo e diagnóstico relacionados.
6. Execute as validações nesta página.

## Alterar o idioma

A nova sintaxe normalmente percorre token/lexer, analisador Pratt, AST, analisador e
avaliador. Define precedência e associatividade, recuperação de erros e exemplos.
Um operador requer regras de tipo e defesa em tempo de execução. Não adicione `if`, importações,
namespaces ou aliases removidos como um atalho de compatibilidade: são decisões
linguagem explícita.

Para um tipo, comece em `pkg/typesystem`: Tipo, nome canônico, atribuibilidade,
inferência e coerção. Em seguida, integre-o ao analisador/tempo de execução e toque apenas no analisador se
há uma nova sintaxe. Atualize [types](SISTEMA_TIPOS.md) e gere novamente o catálogo.

Um diagnóstico público usa código estável `JOSS-...`, gravidade, arquivo/intervalo,
explicação e sugestão. Adicione evidências suficientes e proteja-se contra falsas
vizinho positivo. Documente o código em [diagnóstico](DIAGNOSTICOS.md).

## Adicione APIs nativas

Para um integrado global, o nome está em `pkg/core/builtins.go`; deve existir
um caso alcançável em um dos despachantes e um retorno em
`native_signatures.go`. Para uma classe use o registro executado por
`Runtime.RegisterNativeClasses()` e `GetNativeClassMethods()`. Poste apenas um
nome sem manipulador cria uma API fantasma; implementar apenas um caso sem registrá-lo
cria código inacessível.

Adicione parâmetros aos metadados quando houver suporte; não inventem a aridez no
analisador. Atualize [contratos](MODULOS_NATIVOS.md), execute
`go run ./tools/docgen` e verifique o exemplo em contexto real.

Para plug-ins, mantenha JOSSBC2Z, JPBC e a VM experimental separados. Tudo novo
O estado mutável do tempo de execução precisa de uma decisão de copiar/compartilhar, limpeza em
`Free` e um teste simultâneo. Uma operação de host sensível deve cruzar um
fronteira de autorização real; Declará-lo em metadados não é suficiente.

## Visualizações, editor e publicação

Palavras-chave são projetadas com `parser.KeywordNames()`. Código VS consome
`vscode-joss/src/server/generated/languageCatalog.json`; Nunca é editado à mão.
Os guias canônicos em espanhol estão em `docs/*.md`; Inglês e Português mantêm
o mesmo nome de arquivo em `docs/en` e `docs/pt`. A publicação versionada de
JosSecurity usa `assets/docs/{es,en,pt}` e deve corresponder byte por byte com cada
idioma de origem. Depois de modificar o espanhol, execute
`go run ./tools/docsi18n -translate -sync`; manifesto hash evita trabalhodesnecessário e `go run ./tools/docsi18n -check` detecta traduções obsoletas,
links perdidos, links locais quebrados e diferenças de espelho público.

Exemplos verificáveis completos usam `joss-run`, `joss-check` ou
`joss-error` imediatamente antes de sua cerca. Análises `documentation_test.go`
todos os três e execute `joss-run`, comparando as linhas de saída. Use `joss-check` para
fragmentos que dependem de servidor, banco de dados ou plugins e explica esse contexto.

## Validação```sh
gofmt -w archivos_go_modificados
go run ./tools/cataloggen --check
go run ./tools/docgen --check
go run ./tools/docsi18n -check
go vet ./...
go test ./...
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
go build ./...
```
Em `vscode-joss`: `npm ci` e `npm run compile`. Construa um binário temporário
`./cmd/joss` e analise o projeto de integração real. Mudanças de modelo,
Migrações ou CRUD devem passar pelos arrays nomeados em AGENTS.md.

Uma revisão documental deve procurar links locais, barreiras de Joss, termos legados,
índices e diferenças entre registro e despachante. Evite reivindicar portabilidade,
atomicidade, segurança ou compatibilidade completa sem provas que comprovem isso.
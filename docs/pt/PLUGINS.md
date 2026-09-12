# Plugins e pacotes

[Índice](README.md) · Antes: [estrutura](ESTRUCTURA_PROYECTO.md) · Depois: [contribuir](CONTRIBUIR.md)

Um **pacote** reúne arquivos e metadados para distribuir um recurso.
Um **plugin** incorpora funções, classes ou comandos a um aplicativo. Josh
carrega seus pacotes `.jp` automaticamente; uma importação não está escrita no programa.

## Crie um plugin Joss

Com a CLI instalada, em uma pasta de trabalho:```sh
joss new plugin calculadora
cd calculadora
joss plugin compile .
```
O modelo contém `joss.yaml`, `src/plugin.joss` e um fluxo de trabalho de
publicação. Leia o manifesto gerado antes de alterar nomes ou exportar.
A variante `joss new package calculadora` produz um pacote menor;
é construído com `joss build package .`. Os testes
`TestNewPackageAndPluginTemplatesCompileEndToEnd` cria ambas as variantes,
Eles verificam a assinatura e decodificam o conteúdo.```sh
joss plugin inspect calculadora.jp
joss plugin verify calculadora.jp
```
`inspect` permite descobrir nomes, exportações, permissões e símbolos.
`verify` verifica a integridade e a assinatura Ed25519. A chave pública incluída em
um arquivo demonstra consistência com sua assinatura; por si só **não estabelece
que o editor é alguém em quem você confia**.

## Instale e use```sh
joss pub add nombre_del_paquete ^1.0.0
joss pub install
```
Substitui o nome e a versão por um pacote existente no registro
configurado. `pub` mantém dependências; não adiciona sintaxe de origem. Consulta
[CLI](CLI.md) e a documentação do pacote para sua API específica.

Pacotes declarados em `joss.yaml` ou presentes em `plugins/` são descobertos
ao preparar o tempo de execução. Seu `SymbolIndex` publica parâmetros e retorna para
o analisador. Um pacote antigo sem retorno publicado fornece `unknown`:
significa falta de informação, não uma garantia de compatibilidade.

Exemplo de integração **dependente de um plugin que exporta estes símbolos**:```joss
$resultado = calculadora::sumar(2, 3)
```
Não copie esse nome sem verificá-lo com `inspect`. Uma função exportada
também pode estar disponível por nome direto; as classes exportadas
são instanciados com `new`. Não há namespaces ou módulos de origem com importações.

## O que um JP realmente contém?

O contêiner assinado armazena metadados, arquivos, índice de símbolos e um
entrada de bytecode. O tempo de execução detecta dois formatos diferentes:

| Conteúdo | Executor | Escopo |
|---|---|---|
| `JOSSBC2Z` | Adaptador AST `pkg/core` | Árvore Joss serializada e compactada, interpretada. |
| `JPBC` | `pkg/pluginruntime.JPBCVM` | Máquina de instruções específicas do plug-in. |

Nenhum converte o programa principal em código de máquina LLVM/Cranelift.
A VM experimental em `pkg/vm` é um terceiro componente e não é o executor
padrão de `joss run` ou `joss build native`.

## Compilação de outras linguagens: estado e limites

`joss plugin compile archivo --lang=... --name=... --exports=...` selecione
um backend `pkg/plugincompiler`. Tenha um nome aceito pela CLI
Isso não significa que toda essa linguagem seja implementada.

| Entrada | Implementação atual |
|---|---|
| Joss/projeto gerado | Embalagem Joss AST. |
| Pitão | Tradutor de um subconjunto de expressões e funções para IR/JPBC; ele não executa CPython nem incorpora todo o seu ecossistema. |
| PHP | Tradutor parcial para IR/JPBC; Não é equivalente a um tempo de execução PHP. |
| Java/Kotlin | Leitura de `.class` ou `.jar` e tradução parcial; ele não oferece a JVM inteira. |
| Ferrugem, C, C++, Dardo, Flutter, Wasm | O backend verifica o cabeçalho `\\0asm` e gera uma função por exportação que retorna um texto de demonstração. **Não interpreta nem traduz instruções do Wasm.** |

Portanto, não use o caminho Wasm para criptografar, transformar arquivos ou executar
uma verdadeira biblioteca. Um pacote assinado produzido por essa rota pode ser
estruturalmente válido e ainda não implementa a operação solicitada.

A otimização do plug-in remove funções não acessíveis nas exportações
de acordo com o IR construído. Não demonstra equivalência com nenhum programa
do idioma de origem. O limite de tamanho configurado gera um aviso,
não é uma garantia de tamanho máximo.

## Permissões e isolamento

`PermissionGuard` verifica chamadas para o host que estão mapeadas:

| Operação de host | Permissão |
|---|---|
| `http_get`, `http_post`, `fetch` | `network.http` |
| `file_read`, `file_write` | `filesystem.read`, `filesystem.write` |
| `env_read`, `env_write` | `env.read`, `env.write` |
| `db_query`, `db_exec` | `database.query`, `database.exec` |

Verifique a tabela `pkg/pluginruntime/jpbc_vm.go` ao estender o host.
As permissões declaradas são concedidas durante a construção da guarda; não há diálogo
aprovação do usuário. Os curingas expandem as permissões por prefixo e um
A revogação exata não anula um curinga concedido.

Este **não é um sandbox WASI nem isolamento de processo ou sistema operacional**.
A verificação limita-se às operações integradas nessa fronteira. O
Os drivers ABI C, os processos externos e o executor AST possuem mecanismos diferentes.O orçamento da demonstração JPBC é aplicado por invocação de função;
Não deve ser anunciado como um limite global de recursos para um aplicativo inteiro.

## Pontes e ciclo de vida

`Plugin::call`, `stream`, `path` e `platform` conectam formatos de plugins;
`System::load_driver` e `driver_call` carregam bibliotecas ABI C v1 específicas
de plataforma. Seus contratos estão no [catálogo](CATALOGO_NATIVO.md) e
a [referência nativa](MODULOS_NATIVOS.md).

Uma bifurcação de tempo de execução compartilha o registro do plugin e outros recursos.
Ao liberar uma instância do pool, `Runtime.Free()` também limpa
`PluginRegistry`; mantendo o registro e excluindo apenas quebras de classes/símbolos
o próximo pedido. Plugins não devem assumir que variáveis de um
solicitação anterior ainda estão disponíveis.

Fontes: [compilador](../../pkg/plugincompiler/plugincompiler.go),
[Backend Wasm](../../pkg/plugincompiler/backends/nativewasm/nativewasm_backend.go),
[tempo de execução](../../pkg/pluginruntime/), [contêiner](../../pkg/pluginpkg/).
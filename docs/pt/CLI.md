# Joss CLI â ReferÃªncia completa de comandos

[Ãndice](README.md) Â·

Antes: [primeiros passos](PRIMEROS_PASOS.md) Â·

Depois: [analisador](ANALIZADOR.md)

A fonte do comando canÃ´nico Ã© `cmd/joss/main.go`. `joss help` mostra a ajuda interativa instalada e `joss version` a versÃ£o atual do tempo de execuÃ§Ã£o.

A CLI Ã© o programa que recebe comandos no terminal. Incorpora um ambiente REPL
interativo (`joss repl`), execuÃ§Ã£o direta de script (`joss run`), servidor web de Ãºltima geraÃ§Ã£o
desempenho (`joss server start`) e um conjunto completo de ferramentas de qualidade de cÃ³digo e gerenciamento de pacotes (`pub`).

---

## 1. ExecuÃ§Ã£o, REPL e Build

```bash
joss run archivo.joss
joss repl
joss server start
joss program start
joss build [archivo.joss] [opciones]
joss emit-ir [archivo.joss] [-o salida.ir]
joss analyze [archivo.joss]
joss update [-f|--canary|--stable]
```

- `run [archivo]`: Execute um script `.joss` após analisar o projeto. Erros semânticos bloqueiam a execução; os avisos não.
- `repl`: Inicia o console interativo (Read-Eval-Print Loop) para avaliar expressões, testar funções e experimentar código em tempo real. Digite `exit` ou pressione `Ctrl+C` para sair.
- `server start`: Requer o ponto de entrada `main.joss` e inicia o servidor HTTP multinível de alto desempenho. Pressione `q` para pará-lo com segurança.
- `program start`: Inicia o aplicativo no modo desktop.
- `build [archivo.joss]`: **Compilador nativo oficial de Joss**. Compila o programa em um executável binário nativo real independente (PE `.exe` no Windows, ELF no Linux, Mach-O no macOS). Realiza análise semântica exaustiva (`PreparedProgram`), análise de alcance (*Reachability Analysis*), lowering para Joss Native IR (`pkg/ir`), verificação de IR e geração de código nativo através de LLVM ou do backend de compilação direta (`pkg/backend/native`), sem empacotar AST nem depender do runtime de Go.
  - `-o <salida>`: Nome ou caminho do binário executável final.
  - `--target=<os>-<arch>`: Alvo de compilação cruzada (ex. `windows-amd64`, `linux-amd64`, `darwin-arm64`).
  - `--release`: Compilação otimizada para produção sem símbolos de depuração.
  - `--debug`: Compilação com informações de depuração.
  - `--trace`: Emite e preserva artefatos intermediários do pipeline nativo (`.ir`, `.ll`, `.standalone.go`).
  - `build package [ruta]`: Empacota extensões e plugins em pacotes `.jp` (v2) assinados com Ed25519 contendo bytecode puro.
  - `build web`: Prepara assets e distribuição estática para implantação web.
- `emit-ir [archivo.joss] [-o salida.ir]`: Emite a representação intermediária canônica (Joss Native IR) em formato textual determinístico após a análise semântica e lowering.
- `analyze [archivo]`: Analisa a entrada (por padrão `main.joss`) e `app/**/*.joss`. Não inclui automaticamente `routes.joss`, `api.joss` ou otros irmãos. Retorna código diferente de zero se existirem erros e preserva arquivo/linha/coluna. Consulte [ANALIZADOR.md](ANALIZADOR.md).
- `update`: Usa o atualizador implementado pela CLI e pode exigir permissões de rede/sistema. Verifique seus canais e artefatos reais antes de prometer que uma distribuição contém um SDK ou editor.

---

## 2. CriaÃ§Ã£o de Projeto (`new`)

```bash
joss new mi_proyecto
joss new web mi_proyecto
joss new console mi_cli
joss new package mi_paquete
joss new plugin mi_plugin
```

- `new web` / `new`: Gera a estrutura MVC completa de uma aplicaÃ§Ã£o web com motor de visualizaÃ§Ãµes, rotas, middleware e ORM.
- `new console`: Gere um modelo leve para ferramentas de linha de comando.
- `new package`: Crie uma estrutura de pacote declarativa para o gerenciador `pub`.
- `new plugin`: Crie um projeto oficial de plugin multilÃ­ngue traduzÃ­vel para bytecode binÃ¡rio `.jp`.

---

## 3. Geradores de cÃ³digo (`make:*` e `remove:*`)

```bash
joss make:controller Users
joss make:middleware AuthGuard
joss make:model User
joss make:view users/index
joss make:mvc Product
joss make:crud products
joss remove:crud products
joss make:migration create_products
```

### ð ï¸ `make:crud [Tabla]` (Gerador Relacional Inteligente)
Conecta-se ao banco de dados configurado em `env.joss`, inspeciona o esquema da tabela e gera automaticamente um mÃ³dulo administrativo completo:
1. **InspeÃ§Ã£o de chave estrangeira (`_id`)**: Detecta relacionamentos com outras tabelas, infere nomes de modelos relacionais e detecta automaticamente colunas visÃ­veis (`username`, `name`, `title`).
2. **Modelo e modelos relacionados**: Gere `app/models/Model.joss` e quaisquer modelos relacionais ausentes.
3. **Controlador CRUD completo**: Gere `app/controllers/ModelController.joss` com mÃ©todos `index`, `create`, `store`, `edit`, `update` e `delete` que incluem `joins` e `selects` automÃ¡ticos.
4. **VisualizaÃ§Ãµes Tailwind CSS**: Gera `app/views/model/index.joss.html`, `create.joss.html` e `edit.joss.html` com formulÃ¡rios dinÃ¢micos e menus suspensos `<select>` para relacionamentos.
5. **InjeÃ§Ã£o de barra de navegaÃ§Ã£o e rotas**: Injete a opÃ§Ã£o em `app/views/layouts/master.joss.html` e insira as rotas protegidas no grupo `Router::middleware("auth")` em `routes.joss`.

O comando sÃ³ Ã© suportado em projetos web e requer que a tabela jÃ¡ exista.
Os nomes de tabelas/colunas sÃ£o validados como identificadores antes da consulta
o esquema. O controlador gerado aceita apenas colunas editÃ¡veis
descoberto (nÃ£o faz alocaÃ§Ã£o em massa), a exclusÃ£o usa `POST` com CSRF e retorna
executar o gerador nÃ£o duplica rotas ou links de navegaÃ§Ã£o.

### ðï¸ `remove:crud [Tabla]`
Desfaz a compilaÃ§Ã£o de maneira limpa: remove o controlador, o modelo, a pasta de visualizaÃ§Ãµes e remove as rotas injetadas em `routes.joss` e o link da barra de navegaÃ§Ã£o.

---

## 4. Banco de dados e migraÃ§Ãµes

```bash
joss make:migration create_users_table
joss migrate
joss migrate:fresh
joss db:seed
joss change db mysql
joss change db sqlite
joss change db prefix app_
joss change db migrate --host=HOST --port=3306 --database=DB --user=USER --password=PASS
```

- `make:migration`: Gere uma nova migraÃ§Ã£o com timestamp em `app/database/migrations/`. `create_users`, `create_users_table` e `user` sÃ£o normalizados para a tabela lÃ³gica `users`; `make:miggrate` nÃ£o Ã© um alias e mostra a correÃ§Ã£o sugerida.
- `migrate`: Execute migraÃ§Ãµes pendentes em ordem cronolÃ³gica.
- `migrate:fresh`: Exclua todas as tabelas do banco de dados e execute novamente todas as migraÃ§Ãµes do zero.
- `db:seed`: Execute os seeders definidos em `app/database/seeders/`.
- `change db`: Altere o motor configurado (`mysql` ou `sqlite`) ou modifique o prefixo global da tabela (`DB_PREFIX`).
- `change db migrate`: Migre a quente os dados e a estrutura da conexÃ£o atual para um novo servidor MySQL remoto.

---

## 5. CompilaÃ§Ã£o e gerenciamento de plugins (`.jp`)

```bash
joss plugin compile .
joss plugin compile script.py --lang=python --name=mi_plugin --exports=calcular
joss plugin inspect mi_plugin.jp
joss plugin verify mi_plugin.jp
```

- `plugin compile`: Produz JPBC assinado a partir de backends parciais. Subconjuntos de traduÃ§Ã£o Python/PHP/Java; A rota Wasm valida apenas o cabeÃ§alho e gera stubs de demonstraÃ§Ã£o. Consulte [Plugins](PLUGINS.md).
- `plugin inspect`: Mostra metadados, permissÃµes declaradas e tabela de sÃ­mbolos do pacote `.jp`.
- `plugin verify`: Verifica a assinatura digital Ed25519 e a integridade estrutural do container `.jp`.

---

## 6. Gerenciador de pacotes (`pub`)

```bash
joss pub add paquete ^1.2.0
joss pub remove paquete
joss pub install
joss pub install --offline
joss pub update
joss pub search termino
joss pub info paquete
joss pub publish
joss pub login
joss pub logout
joss pub cache clean
```

Se `PUB_REGISTRY_URL` nÃ£o for especificado, o Pub resolve as dependÃªncias usando o registro oficial em `https://joss.red`.

---

## 7. Qualidade de cÃ³digo e ferramentas (`check`, `format`, `lint`, `fix`, `test`)

```bash
joss check [ruta]
joss format [ruta] [--write|--check]
joss lint [ruta] [--json]
joss fix [ruta] [--dry-run]
joss test [--filter=nombre] [ruta]
```

- `check`: Executa formato, sintaxe, anÃ¡lise e linter. Verifique a saÃ­da: um problema de formataÃ§Ã£o Ã© relatado como um aviso neste pipeline.
- `format`: Em um arquivo, modifique o padrÃ£o a menos que vocÃª use `--check`; em um diretÃ³rio vocÃª sÃ³ escreve com `--write`. Sempre use `joss format ruta --check` no CI.
- `lint`: Executa anÃ¡lise estÃ¡tica com regras de consistÃªncia de tipo, estilo e detecÃ§Ã£o de segredos ou credenciais em cÃ³digo rÃ­gido (`--json` para integraÃ§Ã£o estruturada).
- `fix`: Aplica correÃ§Ãµes automÃ¡ticas seguras (visibilidade necessÃ¡ria, formataÃ§Ã£o) com suporte para `--dry-run`.
- `test`: Execute arquivos `*_test.joss` com `test`/`it`, `assert`, `assertTrue`, `assertFalse`, `assertEqual`, `assertNotEqual`, `assertNull`, `assertNotNull` e `assertThrows`. Coloque `--filter` antes do caminho: o analisador de flags para de ler as opÃ§Ãµes apÃ³s o primeiro argumento posicional.

---

## 8. Comandos de plug-ins e ajuda dinÃ¢mica

```bash
joss help plugins
joss help plugins [nombre_plugin]
joss ai:activate
joss brevo:config [--enable|--disable] [--api-key=CLAVE]
joss backup:create [destino]
joss backup:restore <archivo.zip>
joss bg:remove <input.jpg> [output.png]
joss notify:send <canal> <mensaje>
```

- `help plugins`: Mostra todos os plugins instalados e disponÃ­veis junto com seus comandos CLI expostos e seu status (`[protegido]`).
- `help plugins [nombre_plugin]`: Mostra a ficha tÃ©cnica, repositÃ³rio, opÃ§Ãµes e comandos especÃ­ficos do plugin selecionado.
- **Comandos de plug-ins**: Plugins instalados em `plugins/` ou declarados em `joss.yaml` podem registrar e despachar comandos CLI independentes e protegidos.

---

## 9. Armazenamento e serviÃ§os em nuvem (`userstorage`)

```bash
joss userstorage local
joss userstorage oci
joss userstorage sync-oci
joss userstorage sync-local
```

- `userstorage`: alterna o provedor de armazenamento entre o disco local e o **Oracle Cloud Infrastructure (OCI)**, permitindo a sincronizaÃ§Ã£o bidirecional via `sync-oci` e `sync-local`.

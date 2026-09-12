# Joss CLI – Referência completa de comandos

[Índice](README.md) · Antes: [primeiros passos](PRIMEROS_PASOS.md) · Depois: [analisador](ANALIZADOR.md)

A fonte do comando canônico é `cmd/joss/main.go`. `joss help` mostra a ajuda interativa instalada e `joss version` a versão atual do tempo de execução.

A CLI é o programa que recebe comandos no terminal. Incorpora um ambiente REPL
interativo (`joss repl`), execução direta de script (`joss run`), servidor web de última geração
desempenho (`joss server start`) e um conjunto completo de ferramentas de qualidade de código e gerenciamento de pacotes (`pub`).

---

## 1. Execução, REPL e Build```bash
joss run archivo.joss
joss repl
joss server start
joss program start
joss analyze [archivo.joss]
joss update [-f|--canary|--stable]
joss build [web|program|native|package]
joss build native [os] [arch] [--gui]
```
- `run [archivo]`: Execute um script `.joss` após analisar o projeto. Erros semânticos bloqueiam a execução; os avisos não.
- `repl`: Inicia o console interativo (Read-Eval-Print Loop) para avaliar expressões, testar funções e experimentar código em tempo real. Digite `exit` ou pressione `Ctrl+C` para sair.
- `server start`: Requer o ponto de entrada `main.joss` e inicia o servidor HTTP multinível de alto desempenho. Pressione `q` para pará-lo com segurança.
- `program start`: Inicia o aplicativo no modo desktop.
- `analyze [archivo]`: Analisa a entrada (por padrão `main.joss`) e `app/**/*.joss`. Não inclui automaticamente `routes.joss`, `api.joss` ou outros irmãos. Retorna código diferente de zero se existirem erros e preserva arquivo/linha/coluna. Consulte [ANALYZER.md](ANALIZADOR.md).
- `build native [os] [arch]`: Construa um binário independente para `windows`, `linux` ou `darwin`; empacota o AST serializado e o executor Go. Não é um backend LLVM/AOT do programa Joss. Use `--gui` para aplicativos com interface de desktop.
- `update`: Usa o atualizador implementado pela CLI e pode exigir permissões de rede/sistema. Verifique seus canais e artefatos reais antes de prometer que uma distribuição contém um SDK ou editor.

---

## 2. Criação de Projeto (`new`)```bash
joss new mi_proyecto
joss new web mi_proyecto
joss new console mi_cli
joss new package mi_paquete
joss new plugin mi_plugin
```
- `new web` / `new`: Gera a estrutura MVC completa de uma aplicação web com motor de visualizações, rotas, middleware e ORM.
- `new console`: Gere um modelo leve para ferramentas de linha de comando.
- `new package`: Crie uma estrutura de pacote declarativa para o gerenciador `pub`.
- `new plugin`: Crie um projeto oficial de plugin multilíngue traduzível para bytecode binário `.jp`.

---

## 3. Geradores de código (`make:*` e `remove:*`)```bash
joss make:controller Users
joss make:middleware AuthGuard
joss make:model User
joss make:view users/index
joss make:mvc Product
joss make:crud products
joss remove:crud products
joss make:migration create_products
```
### 🛠️ `make:crud [Tabla]` (Gerador Relacional Inteligente)
Conecta-se ao banco de dados configurado em `env.joss`, inspeciona o esquema da tabela e gera automaticamente um módulo administrativo completo:
1. **Inspeção de chave estrangeira (`_id`)**: Detecta relacionamentos com outras tabelas, infere nomes de modelos relacionais e detecta automaticamente colunas visíveis (`username`, `name`, `title`).
2. **Modelo e modelos relacionados**: Gere `app/models/Model.joss` e quaisquer modelos relacionais ausentes.
3. **Controlador CRUD completo**: Gere `app/controllers/ModelController.joss` com métodos `index`, `create`, `store`, `edit`, `update` e `delete` que incluem `joins` e `selects` automáticos.
4. **Visualizações CSS do Tailwind`: Genera `app/views/model/index.joss.html`, `create.joss.html` y `edit.joss.html` con formularios dinámicos y menús desplegables `<select>` para relacionamentos.
5. **Injeção de barra de navegação e rotas**: Injete a opção em `app/views/layouts/master.joss.html` e insira as rotas protegidas no grupo `Router::middleware("auth")` em `routes.joss`.

O comando só é suportado em projetos web e requer que a tabela já exista.
Os nomes de tabelas/colunas são validados como identificadores antes da consulta
o esquema. O controlador gerado aceita apenas colunas editáveis
descoberto (não faz alocação em massa), a exclusão usa `POST` com CSRF e retorna
executar o gerador não duplica rotas ou links de navegação.

### 🗑️ `remove:crud [Tabla]`
Desfaz a compilação de maneira limpa: remove o controlador, o modelo, a pasta de visualizações e remove as rotas injetadas em `routes.joss` e o link da barra de navegação.

---

## 4. Banco de dados e migrações```bash
joss make:migration create_users_table
joss migrate
joss migrate:fresh
joss db:seed
joss change db mysql
joss change db sqlite
joss change db prefix app_
joss change db migrate --host=HOST --port=3306 --database=DB --user=USER --password=PASS
```
- `make:migration`: Gere uma nova migração com timestamp em `app/database/migrations/`. `create_users`, `create_users_table` e `user` são normalizados para a tabela lógica `users`; `make:miggrate` não é um alias e mostra a correção sugerida.
- `migrate`: Execute migrações pendentes em ordem cronológica.
- `migrate:fresh`: Exclua todas as tabelas do banco de dados e execute novamente todas as migrações do zero.
- `db:seed`: Execute os seeders definidos em `app/database/seeders/`.
- `change db`: Altere o motor configurado (`mysql` ou `sqlite`) ou modifique o prefixo global da tabela (`DB_PREFIX`).
- `change db migrate`: Migre a quente os dados e a estrutura da conexão atual para um novo servidor MySQL remoto.

---

## 5. Compilação e gerenciamento de plugins (`.jp`)```bash
joss plugin compile .
joss plugin compile script.py --lang=python --name=mi_plugin --exports=calcular
joss plugin inspect mi_plugin.jp
joss plugin verify mi_plugin.jp
```
- `plugin compile`: Produz JPBC assinado a partir de backends parciais. Subconjuntos de tradução Python/PHP/Java; A rota Wasm valida apenas o cabeçalho e gera stubs de demonstração. Consulte [Plugins](PLUGINS.md).
- `plugin inspect`: Mostra metadados, permissões declaradas e tabela de símbolos do pacote `.jp`.
- `plugin verify`: Verifica a assinatura digital Ed25519 e a integridade estrutural do container `.jp`.

---

## 6. Gerenciador de pacotes (`pub`)```bash
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
Se `PUB_REGISTRY_URL` não for especificado, o Pub resolve as dependências usando o registro oficial em `https://joss.red`.

---

## 7. Qualidade de código e ferramentas (`check`, `format`, `lint`, `fix`, `test`)```bash
joss check [ruta]
joss format [ruta] [--write|--check]
joss lint [ruta] [--json]
joss fix [ruta] [--dry-run]
joss test [--filter=nombre] [ruta]
```
- `check`: Executa formato, sintaxe, análise e linter. Verifique a saída: um problema de formatação é relatado como um aviso neste pipeline.
- `format`: Em um arquivo, modifique o padrão a menos que você use `--check`; em um diretório você só escreve com `--write`. Sempre use `joss format ruta --check` no CI.
- `lint`: Executa análise estática com regras de consistência de tipo, estilo e detecção de segredos ou credenciais em código rígido (`--json` para integração estruturada).
- `fix`: Aplica correções automáticas seguras (visibilidade necessária, formatação) com suporte para `--dry-run`.
- `test`: Execute arquivos `*_test.joss` com `test`/`it`, `assert`, `assertTrue`, `assertFalse`, `assertEqual`, `assertNotEqual`, `assertNull`, `assertNotNull` e `assertThrows`. Coloque `--filter` antes do caminho: o analisador de flags para de ler as opções após o primeiro argumento posicional.

---

## 8. Comandos de plug-ins e ajuda dinâmica```bash
joss help plugins
joss help plugins [nombre_plugin]
joss ai:activate
joss brevo:config [--enable|--disable] [--api-key=CLAVE]
joss backup:create [destino]
joss backup:restore <archivo.zip>
joss bg:remove <input.jpg> [output.png]
joss notify:send <canal> <mensaje>
```
- `help plugins`: Mostra todos os plugins instalados e disponíveis junto com seus comandos CLI expostos e seu status (`[protegido]`).
- `help plugins [nombre_plugin]`: Mostra a ficha técnica, repositório, opções e comandos específicos do plugin selecionado.
- **Comandos de plug-ins**: Plugins instalados em `plugins/` ou declarados em `joss.yaml` podem registrar e despachar comandos CLI independentes e protegidos.

---

## 9. Armazenamento e serviços em nuvem (`userstorage`)```bash
joss userstorage local
joss userstorage oci
joss userstorage sync-oci
joss userstorage sync-local
```
- `userstorage`: alterna o provedor de armazenamento entre o disco local e o **Oracle Cloud Infrastructure (OCI)**, permitindo a sincronização bidirecional via `sync-oci` e `sync-local`.
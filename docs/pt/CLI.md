# CLI de Joss — Referência Completa de Comandos

[Índice](README.md) · Antes: [primeiros passos](PRIMEROS_PASOS.md) · Depois: [analisador](ANALIZADOR.md)

A fonte canônica de comandos é `cmd/joss/main.go`. `joss help` mostra a ajuda interativa instalada e `joss version` a versão atual do runtime (`3.6.8.0`).

A CLI é o programa que recebe comandos no terminal. Incorpora um ambiente REPL
interativo (`joss repl`), execução direta de scripts (`joss run`), servidor web de alto
desempenho (`joss server start` / `joss server`), compilação nativa AOT (`joss build`) e um conjunto completo de ferramentas de qualidade de código e gerenciamento de pacotes (`pub`).

---

## 1. Execução, REPL e Build Nativo

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

- `run [archivo]`: Executa um script `.joss` após analisar o projeto (`LoadProject`) e validar seu `PreparedProgram` com o interpretador de Joss. Os erros semânticos bloqueiam a execução; os warnings não.
- `repl`: Inicia o console interativo (Read-Eval-Print Loop) com prompt `joss> ` para avaliar expressões, testar funções e experimentar código em tempo real. Digite `exit` ou pressione `Ctrl+C` para sair.
- `server start` (ou `joss server`): Requer o ponto de entrada `main.joss` e inicia o servidor HTTP multinível de alto desempenho interpretado pelo runtime de servidor. Pressione `q` ou `Ctrl+C` para pará-lo com segurança.
- `program start`: Inicia a aplicação em modo desktop.
- `build [archivo.joss]`: **Compilador nativo oficial de Joss**. Compila o programa para um executável binário nativo real autocontido (PE `.exe` no Windows, ELF no Linux, Mach-O no macOS), depositando-o por padrão dentro da pasta `build/`. Realiza análise semântica exaustiva (`PreparedProgram`), análise de alcançabilidade (*Reachability Analysis*), lowering para Joss Native IR (`pkg/ir`), verificação de IR (`ir.Verifier`) e geração de código nativo (`pkg/backend/native`).
  - **Binários CLI puros**: São compilados para código de máquina puro ultracompacto (~5.5 KB - 32 KB) sem `joss_host.dll` nem trailer VFS.
  - **Aplicações completas com serviços web/DB**: Vinculam seletivamente `_joss_host_bridge` (podando motores não utilizados mediante tags `no<driver>`) e selam o sistema de arquivos virtual criptografado (`env.enc` com AES-256) dentro do executável.
  - `-o <salida>`: Nome ou caminho personalizado do binário executável final.
  - `--target=<os>-<arch>` (ou atalhos diretos `joss build linux`, `joss build macos`, `joss build android`): Compilação cruzada integrada para `windows-amd64`, `windows-arm64`, `linux-amd64`, `linux-arm64`, `darwin-arm64` (Apple Silicon), `darwin-amd64` e `android-arm64` (Termux).
  - `--release`: Compilação otimizada para produção sem símbolos de depuração.
  - `--debug`: Compilação com informações de depuração.
  - `--trace`: Emite e preserva artefatos intermediários do pipeline nativo (`.ir`, `.ll`, `.standalone.go`).
  - `build package [ruta]`: Empacota extensões e plugins em pacotes `.jp` (v2) assinados com Ed25519 contendo bytecode puro.
  - `build web`: Prepara assets e distribuição estática para implantação web.
- `emit-ir [archivo.joss] [-o salida.ir]`: Emite a representação intermediária canônica (Joss Native IR) em formato textual determinístico após a análise semântica e o lowering.
- `analyze [archivo]`: Analisa o projeto através de `semanticanalyzer.LoadProject` (incluindo a entrada, `routes.joss`, `api.joss` e `app/**/*.joss`). Retorna código diferente de zero se existirem erros e preserva arquivo/linha/coluna. Consulte [ANALIZADOR.md](ANALIZADOR.md).
- `update`: Usa o atualizador implementado pela CLI (`--canary`, `--stable`, `-f`).

---

## 2. Criação de Projetos (`new`)

```bash
joss new mi_proyecto
joss new web mi_proyecto
joss new console mi_cli
joss new package mi_paquete
joss new plugin mi_plugin
```

- `new web` / `new`: Gera a estrutura MVC completa de uma aplicação web com motor de views, rotas, middleware e ORM.
- `new console`: Gera um template leve para ferramentas de linha de comando (`main.joss`, `env.joss`, `joss.yaml` e `app/`).
- `new package`: Cria uma estrutura declarativa de pacote para o gerenciador `pub`.
- `new plugin`: Cria um projeto de plugin multilíngue oficial traduzível para bytecode binário `.jp`.

---

## 3. Geradores de Código (`make:*` e `remove:*`)

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

### 🛠️ `make:crud [Tabla]` (Gerador Relacional Inteligente)
Conecta ao banco de dados configurado em `env.joss`, inspeciona o esquema da tabela e gera automaticamente um módulo administrativo completo:
1. **Inspeção de Chaves Estrangeiras (`_id`)**: Detecta relações com outras tabelas, infere nomes de modelos relacionais e autodetecta colunas visíveis (`username`, `name`, `title`).
2. **Modelo e Modelos Relacionados**: Gera `app/models/Model.joss` e qualquer modelo relacional faltante.
3. **Controlador CRUD completo**: Gera `app/controllers/ModelController.joss` com métodos `index`, `create`, `store`, `edit`, `update` e `delete` que incluem `joins` e `selects` automáticos.
4. **Views Tailwind CSS**: Gera `app/views/model/index.joss.html`, `create.joss.html` e `edit.joss.html` com formulários dinâmicos e menus suspensos `<select>` para relações.
5. **Injeção na Navbar e Rotas**: Injeta a opção em `app/views/layouts/master.joss.html` e insere as rotas protegidas dentro do grupo `Router::middleware("auth")` em `routes.joss`.

O comando só é suportado em projetos web e requer que a tabela já exista.
Os nomes de tabela/colunas são validados como identificadores antes de consultar
o esquema. O controlador gerado aceita unicamente as colunas editáveis
descobertas (não faz atribuição em massa), a exclusão usa `POST` com CSRF e executar
novamente o gerador não duplica rotas nem links de navegação.

### 🗑️ `remove:crud [Tabla]`
Desfaz de forma limpa a geração: remove o controlador, modelo, pasta de views e remove as rotas injetadas em `routes.joss` e o link da navbar.

---

## 4. Banco de Dados e Migrações

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

- `make:migration`: Gera uma nova migração com timestamp em `app/database/migrations/`. `create_users`, `create_users_table` e `user` são normalizados para a tabela lógica `users`; `make:miggrate` não é um alias e mostra a correção sugerida.
- `migrate`: Executa as migrações pendentes em ordem cronológica.
- `migrate:fresh`: Remove todas as tabelas do banco de dados e executa novamente todas as migrações do zero.
- `db:seed`: Executa os seeders populadores definidos em `app/database/seeders/`.
- `change db`: Altera o motor configurado (`mysql`, `postgres`, `sqlserver` ou `sqlite`) ou modifica o prefixo global de tabelas (`DB_PREFIX`).
- `change db migrate`: Migra a quente os dados e a estrutura da conexão atual para um novo servidor remoto.

---

## 5. Compilação e Gerenciamento de Plugins (`.jp`)

```bash
joss plugin compile .
joss plugin compile script.py --lang=python --name=mi_plugin --exports=calcular
joss plugin inspect mi_plugin.jp
joss plugin verify mi_plugin.jp
```

- `plugin compile`: Produz JPBC assinado a partir de backends parciais. Python/PHP/Java traduzem subconjuntos; a rota Wasm apenas valida o cabeçalho e gera stubs de demonstração. Consulte [Plugins](PLUGINS.md).
- `plugin inspect`: Mostra metadados, permissões declaradas e tabela de símbolos do pacote `.jp`.
- `plugin verify`: Verifica a assinatura digital Ed25519 e a integridade estrutural do contêiner `.jp`.

---

## 6. Gerenciador de Pacotes (`pub`)

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

Se `PUB_REGISTRY_URL` não for especificado, o Pub resolve as dependências utilizando o registro oficial em `https://astriom.org`.

---

## 7. Qualidade de Código e Tooling (`check`, `format`, `lint`, `fix`, `test`)

```bash
joss check [ruta]
joss format [ruta] [--write|--check]
joss lint [ruta] [--json]
joss fix [ruta] [--dry-run]
joss test [--filter=nombre] [ruta]
```

- `check`: Executa formatação, sintaxe, análise e linter. Revise sua saída: um problema de formatação é comunicado como warning neste pipeline.
- `format`: Em um arquivo, modifica por padrão salvo se você usar `--check`; em um diretório só escreve com `--write`. Use sempre `joss format ruta --check` no CI.
- `lint`: Executa análise estática com regras de consistência de tipos, estilo e detecção de segredos ou credenciais fixas no código (`--json` para integração estruturada).
- `fix`: Aplica correções automáticas lexicamente seguras (migrações de `let`/`nil`, visibilidade exigida, formatação) sem tocar em strings nem comentários, com suporte para `--dry-run`.
- `test`: Executa arquivos `*_test.joss` com `test`/`it`, `assert`, `assertTrue`, `assertFalse`, `assertEqual`, `assertNotEqual`, `assertNull`, `assertNotNull` e `assertThrows`. Coloque `--filter` antes do caminho: o parser de flags deixa de ler opções após o primeiro argumento posicional.

---

## 8. Comandos de Plugins e Ajuda Dinâmica

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

- `help plugins`: Mostra todos os plugins instalados e disponíveis junto com seus comandos CLI expostos e seu estado (`[protegido]`).
- `help plugins [nombre_plugin]`: Mostra a ficha técnica, repositório, opções e comandos específicos do plugin selecionado.
- **Comandos de plugins**: Os plugins instalados em `plugins/` ou declarados em `joss.yaml` podem registrar e despachar comandos CLI autônomos e protegidos.

---

## 9. Armazenamento e Serviços em Nuvem (`userstorage` e `indexnow`)

```bash
joss userstorage local
joss userstorage oci
joss userstorage sync-oci
joss userstorage sync-local
joss indexnow generate
```

- `userstorage`: Alterna o provedor de armazenamento entre disco local e **Oracle Cloud Infrastructure (OCI)**, permitindo sincronização bidirecional mediante `sync-oci` e `sync-local`.
- `indexnow generate`: Gera e configura a chave para indexação instantânea IndexNow no ambiente do projeto.

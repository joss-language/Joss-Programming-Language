# Joss CLI — Complete Command Reference

[Index](README.md) · Before: [first steps](PRIMEROS_PASOS.md) · After: [parser](ANALIZADOR.md)

The canonical command source is `cmd/joss/main.go`. `joss help` shows the installed interactive help and `joss version` the current runtime version.

The CLI is the program that receives commands in the terminal. Incorporates a REPL environment
interactive (`joss repl`), direct script execution (`joss run`), high-performance web server
performance (`joss server start`) and a complete set of code quality and package management tools (`pub`).

---

## 1. Execution, REPL and Build

```bash
joss run archivo.joss
joss repl
joss server start
joss program start
joss analyze [archivo.joss]
joss update [-f|--canary|--stable]
joss build [web|program|native|package]
joss build native [os] [arch] [--gui]
```

- `run [archivo]`: Run a script `.joss` after analyzing the project. Semantic errors block execution; the warnings do not.
- `repl`: Launches the interactive console (Read-Eval-Print Loop) to evaluate expressions, test functions, and experiment with code in real time. Type `exit` or press `Ctrl+C` to exit.
- `server start`: Requires the entry point `main.joss` and starts the high-performance multi-level HTTP server. Press `q` to stop it safely.
- `program start`: Start the application in desktop mode.
- `analyze [archivo]`: Parse the entry (by default `main.joss`) and `app/**/*.joss`. Does not automatically include `routes.joss`, `api.joss` or other siblings. Returns non-zero code if errors exist and preserves file/line/column. See [ANALYZER.md](ANALIZADOR.md).
- `build native [os] [arch]`: Generate a standalone binary for `windows`, `linux` or `darwin`; packages the serialized AST and the Go runner. It is not an LLVM/AOT backend of the Joss program. Use `--gui` for applications with a desktop interface.
- `update`: Uses the updater implemented by the CLI and may require network/system permissions. Check their actual channels and artifacts before promising that a distribution contains an SDK or editor.

---

## 2. Project Creation (`new`)

```bash
joss new mi_proyecto
joss new web mi_proyecto
joss new console mi_cli
joss new package mi_paquete
joss new plugin mi_plugin
```

- `new web` / `new`: Generates the complete MVC structure of a web application with views engine, routes, middleware and ORM.
- `new console`: Generate a lightweight template for command line tools.
- `new package`: Creates a declarative package structure for the manager `pub`.
- `new plugin`: Create an official multilanguage plugin project translatable to binary bytecode `.jp`.

---

## 3. Code Generators (`make:*` and `remove:*`)

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

### 🛠️ `make:crud [Tabla]` (Intelligent Relational Generator)
Connect to the database configured at `env.joss`, inspect the table schema, and automatically generate a complete administrative module:
1. **Foreign Key Inspection (`_id`)**: Detects relationships with other tables, infers names of relational models and auto-detects visible columns (`username`, `name`, `title`).
2. **Model and Related Models**: Generate `app/models/Model.joss` and any missing relational models.
3. **Full CRUD Controller**: Generate `app/controllers/ModelController.joss` with methods `index`, `create`, `store`, `edit`, `update` and `delete` including automatic `joins` and `selects`.
4. **Tailwind CSS views`: Genera `app/views/model/index.joss.html`, `create.joss.html` y `edit.joss.html` con formularios dinámicos y menús desplegables `<select>` for relationships.
5. **Injection in Navbar and Routes**: Inject the option in `app/views/layouts/master.joss.html` and insert the protected routes within the group `Router::middleware("auth")` in `routes.joss`.

The command is only supported in web projects and requires that the table already exist.
Table/column names are validated as identifiers before querying
the scheme. The generated controller accepts only editable columns
discovered (does not do bulk allocation), deletion uses `POST` with CSRF and returns
running the generator does not duplicate routes or navigation links.

### 🗑️ `remove:crud [Tabla]`
Cleanly undoes the build: deletes the controller, model, views folder and removes the injected paths at `routes.joss` and the navbar link.

---

## 4. Database and Migrations

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

- `make:migration`: Generate a new migration with timestamp at `app/database/migrations/`. `create_users`, `create_users_table` and `user` are normalized to the logical table `users`; `make:miggrate` is not an alias and shows the suggested fix.
- `migrate`: Run pending migrations in chronological order.
- `migrate:fresh`: Delete all database tables and rerun all migrations from scratch.
- `db:seed`: Runs the settler seeders defined in `app/database/seeders/`.
- `change db`: Change the configured engine (`mysql` or `sqlite`) or modify the global table prefix (`DB_PREFIX`).
- `change db migrate`: Hot migrate the data and structure of the current connection to a new remote MySQL server.

---

## 5. Compilation and Plugin Management (`.jp`)

```bash
joss plugin compile .
joss plugin compile script.py --lang=python --name=mi_plugin --exports=calcular
joss plugin inspect mi_plugin.jp
joss plugin verify mi_plugin.jp
```

- `plugin compile`: Produce signed JPBC from partial backends. Python/PHP/Java translate subsets; The Wasm route only validates the header and generates demo stubs. See [Plugins](PLUGINS.md).
- `plugin inspect`: Shows metadata, declared permissions and symbol table of the package `.jp`.
- `plugin verify`: Checks the Ed25519 digital signature and the structural integrity of the container `.jp`.

---

## 6. Package Manager (`pub`)

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

If `PUB_REGISTRY_URL` is not specified, Pub resolves dependencies using the official registry at `https://joss.red`.

---

## 7. Code Quality and Tooling (`check`, `format`, `lint`, `fix`, `test`)

```bash
joss check [ruta]
joss format [ruta] [--write|--check]
joss lint [ruta] [--json]
joss fix [ruta] [--dry-run]
joss test [--filter=nombre] [ruta]
```

- `check`: Runs format, syntax, parsing and linter. Check its output: a formatting problem is reported as a warning in this pipeline.
- `format`: In a file, modify the default unless you use `--check`; in a directory just write with `--write`. Always use `joss format ruta --check` in CI.
- `lint`: Runs static analysis with type consistency rules, style and detection of secrets or credentials in hard code (`--json` for structured integration).
- `fix`: Applies secure automatic fixes (visibility required, formatting) with support for `--dry-run`.
- `test`: Run files `*_test.joss` with `test`/`it`, `assert`, `assertTrue`, `assertFalse`, `assertEqual`, `assertNotEqual`, `assertNull`, `assertNotNull` and `assertThrows`. Place `--filter` before the path: the flags parser stops reading options after the first positional argument.

---

## 8. Plugins Commands and Dynamic Help

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

- `help plugins`: Shows all installed and available plugins along with their exposed CLI commands and status (`[protegido]`).
- `help plugins [nombre_plugin]`: Shows the technical sheet, repository, options and specific commands of the selected plugin.
- **Plugin Commands**: Plugins installed at `plugins/` or declared at `joss.yaml` can register and dispatch standalone and protected CLI commands.

---

## 9. Storage and Cloud Services (`userstorage`)

```bash
joss userstorage local
joss userstorage oci
joss userstorage sync-oci
joss userstorage sync-local
```

- `userstorage`: Switches the storage provider between local disk and **Oracle Cloud Infrastructure (OCI)**, allowing two-way synchronization through `sync-oci` and `sync-local`.

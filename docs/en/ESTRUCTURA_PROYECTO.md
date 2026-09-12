# Project structure

[Index](README.md)

##Web

`joss new web mi_app` and `joss new mi_app` create the actual web template from the `pkg/template/files` package.

All four variants of `joss new` are validated in CI: web and console should parse and parse without errors; console must also run; package and plugin must compile to a signed, verifiable JP v2, with decodable bytecode and symbol index.

```text
mi_app/
├── main.joss
├── env.joss
├── routes.joss
├── api.joss
├── joss.yaml
├── AGENTS.md                 # Guía y sintaxis para asistentes de IA
├── config/
├── app/
│   ├── controllers/          # Subcarpetas por dominio (web/, auth/, api/)
│   │   ├── web/
│   │   ├── auth/
│   │   └── api/
│   ├── models/               # Modelos GranDB (auth/, etc.)
│   │   └── auth/
│   ├── services/             # Servicios en segundo plano e integraciones
│   ├── middleware/           # Middleware de peticiones
│   ├── libs/                # Creado por la plantilla, no precargado actualmente por run
│   └── database/migrations/
├── assets/
├── public/
├── storage/
├── package.json
└── README.md
```

The template can also include routes, authentication files, frontend resources, and API collections. The exact list may grow; the generator and its tests are the executable reference.

`main.joss` is required for `joss server start`. `env.joss` is not Joss code and should not be run with `joss run`.

`joss analyze` recursively discovers the entry and `app/`. When executing, the
runtime preload is limited to `controllers`, `models`, `middleware`, `services`,
`database`, `jobs`, `tasks` and `providers`. Although the template creates `app/libs`,
that folder is not preloaded today. Placing a statement there can pass the
analysis and lack in execution; move the code to a loaded domain while
both lists are unified.

## Console

`joss new console mi_cli` creates `main.joss`, configuration, drivers, models, libraries and migrations; it does not create routes, views, `public/` or web assets.

```bash
cd mi_cli
joss run main.joss
```

## Package

`joss new package mi_plugin` creates:

```text
mi_plugin/
├── joss.yaml
├── README.md
└── src/plugin.joss
```

Compile it with `joss build package .`. See [Plugins](PLUGINS.md).

## Effective conventions

- Drivers: `app/controllers/NameController.joss`.
- Models: `app/models/Name.joss` and normally `extends GranDB`.
- Views: `app/views/name.joss.html` or `.html`.
- Migrations: timestamp, friendly name and extension `.joss`.
- Generated syntax: `func`, `::` for statics and `->` for instances.

The generators are validated with a test that creates web and console projects and passes all their `.joss` files through the parser.

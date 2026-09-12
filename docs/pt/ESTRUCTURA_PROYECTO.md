# Estrutura do projeto

[Índice](README.md)

##Web

`joss new web mi_app` e `joss new mi_app` criam o modelo web real a partir do pacote `pkg/template/files`.

Todas as quatro variantes de `joss new` são validadas no CI: web e console devem analisar e analisar sem erros; console também deve ser executado; o pacote e o plugin devem ser compilados em um JP v2 assinado e verificável, com bytecode decodificável e índice de símbolos.```text
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
O modelo também pode incluir rotas, arquivos de autenticação, recursos de front-end e coleções de API. A lista exata pode aumentar; o gerador e seus testes são a referência executável.

`main.joss` é necessário para `joss server start`. `env.joss` não é código Joss e não deve ser executado com `joss run`.

`joss analyze` descobre recursivamente a entrada e `app/`. Ao executar, o
o pré-carregamento de tempo de execução é limitado a `controllers`, `models`, `middleware`, `services`,
`database`, `jobs`, `tasks` e `providers`. Embora o modelo crie `app/libs`,
essa pasta não está pré-carregada hoje. Colocar uma declaração lá pode passar o
análise e falta de execução; mova o código para um domínio carregado enquanto
ambas as listas são unificadas.

##Console

`joss new console mi_cli` cria `main.joss`, configuração, controladores, modelos, bibliotecas e migrações; ele não cria rotas, visualizações, `public/` ou ativos da web.```bash
cd mi_cli
joss run main.joss
```
## Pacote

`joss new package mi_plugin` cria:```text
mi_plugin/
├── joss.yaml
├── README.md
└── src/plugin.joss
```
Construa-o com `joss build package .`. Consulte [Plugins](PLUGINS.md).

## Convenções eficazes

- Controladores: `app/controllers/NameController.joss`.
- Modelos: `app/models/Name.joss` e geralmente `extends GranDB`.
- Visualizações: `app/views/name.joss.html` ou `.html`.
- Migrações: timestamp, nome amigável e extensão `.joss`.
- Sintaxe gerada: `func`, `::` para estática e `->` para instâncias.

Os geradores são validados com um teste que cria projetos web e de console e passa todos os seus arquivos `.joss` através do analisador.
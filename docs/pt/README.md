# Joss Documentação

Esta documentação descreve o comportamento do código atual. Está separado
na aprendizagem, guias, referências e internos para que uma nova pessoa possa
avança em ordem e uma pessoa experiente encontra regras concretas.

## Idiomas

- [Español (fonte canônica)](../README.md)
- [English](../en/README.md)
- Português: este diretório.

Todos os 46 documentos usam o mesmo nome de arquivo em cada idioma, para que
os links relativos funcionem de forma consistente. Depois de editar a fonte em espanhol, execute
`go run ./tools/docsi18n -translate -sync` para atualizar traduções e espelhar
a cópia web; `go run ./tools/docsi18n -check` detecta arquivos ausentes,
traduções desatualizadas, links quebrados ou diferenças com JosSecurity.

## Aprenda Joss (rota guiada para iniciantes)

Se você nunca programou ou está aprendendo Joss pela primeira vez, leia este passo a passo na ordem. Cada página apresenta conceitos com explicações pedagógicas, exemplos visuais passo a passo e folhas de dicas rápidas antes de se aprofundar em aspectos técnicos avançados.

0. [Primeiros passos: de um arquivo ao seu primeiro programa interativo com cin](PRIMEROS_PASOS.md)
1. [Valores, variáveis e operações fundamentais](FUNDAMENTOS.md)
2. [Controle de fluxo e tomada de decisão](CONTROL_FLUJO.md)
3. [Funções, escopo, fechamentos e referências](FUNCIONES.md)
4. [Matrizes, mapas e texto](COLECCIONES.md)
5. [Tipos, inferência e conversões](SISTEMA_TIPOS.md)
6. [Classes, objetos e herança](CLASES.md)
7. [Erros e exceções](ERRORES.md)
8. [Assíncrono, Futuro, canais e simultaneidade](CONCURRENCIA.md)
9. [Projeto de console completo com persistência JSON](PROYECTO_CONSOLA.md)
10. [Projeto web completo com MVC nativo](PROYECTO_WEB.md)

O [glossário](GLOSARIO.md) explica a programação e os termos Joss sem exigir
que você já conhece outro idioma.

## Referência de linguagem e ferramentas

- [Sintaxe, tokens e precedência](SINTAXIS.md)
- [Gramática EBNF e nós AST](GRAMATICA.md)
- [Funções](FUNCIONES.md), [recursão](RECURSION.md) e [tipos](SISTEMA_TIPOS.md)
- [Diagnóstico](DIAGNOSTICOS.md) e [analisador](ANALIZADOR.md)
- [Funções globais](FUNCIONES_GLOBALES.md)
- [Classes e serviços nativos](MODULOS_NATIVOS.md)
- [Catálogo nativo gerado](CATALOGO_NATIVO.md)
- [CLI, formatador, linter e testes](CLI.md)
- [Extensão de código VS](VSCODE_EXTENSION.md)
- [Estado e limites](ESTADO_IMPLEMENTACION.md)

## Criar aplicativos

- [Estrutura do projeto](ESTRUCTURA_PROYECTO.md) e [configuração](CONFIGURACION.md)
- [Organização sem importações](MODULOS_IMPORTS.md) e [plugins](PLUGINS.md)
- Web: [servidor](SERVIDOR.md), [HTTP/controladores](CONTROLADORES.md),
  [middleware](MIDDLEWARE.md), [visualizações](VISTAS.md), [ativos](ASSETS.md) e
  [WebSockets](WEBSOCKETS.md)
- Dados: [models/GranDB](MODELOS.md), [Schema Builder](SCHEMA_BUILDER.md) e
  [migrações](MIGRACIONES.md)
- Aplicação: [autenticação](AUTENTICACION.md) e [SEO/sitemap](SEO_SITEMAP.md)

## Internos e manutenção

- [Arquitetura de linguagem](ARQUITECTURA.md)
- [Guia de Contribuição](CONTRIBUIR.md)- [Auditoria de documentação](DOCUMENTATION_AUDIT.md)
- [Auditoria de linguagem Joss 2026](JOSS_LANGUAGE_AUDIT_2026.md)
- [Auditoria técnica de 2026](AUDITORIA_TECNICA_2026.md)
- [Auditoria de otimização de tempo de execução](RUNTIME_OPTIMIZATION_AUDIT.md)
- [Notícias históricas 3.6.7](NOVEDADES_367.md)

Documentos históricos preservam o contexto e podem descrever o estado do seu
data. No caso de uma diferença, implementação, teste e
[Status de implementação](ESTADO_IMPLEMENTACION.md).

# Joss Documentation

This documentation describes the behavior of the current code. It is separated
in learning, guides, reference and internals so that a new person can
move forward in order and an experienced person finds concrete rules.

## Languages

- [Español (canonical source)](../README.md)
- English: this directory.
- [Português](../pt/README.md)

All 46 documents use the same filename in every language, so relative links
work consistently. After editing the Spanish source, run
`go run ./tools/docsi18n -translate -sync` to update translations and mirror
the web copy; `go run ./tools/docsi18n -check` detects missing files, stale
translations, broken links or differences with JosSecurity.

## Learn Joss (Guided route for beginners)

If you've never programmed or are learning Joss for the first time, read this walkthrough in order. Each page introduces concepts with pedagogical explanations, step-by-step visual examples, and quick cheat sheets before delving into advanced technical aspects.

0. [First steps: from a file to your first interactive program with cin](PRIMEROS_PASOS.md)
1. [Fundamental values, variables and operations](FUNDAMENTOS.md)
2. [Flow control and decision making](CONTROL_FLUJO.md)
3. [Functions, scope, closures and references](FUNCIONES.md)
4. [Arrays, maps and text](COLECCIONES.md)
5. [Types, inference and conversions](SISTEMA_TIPOS.md)
6. [Classes, objects and inheritance](CLASES.md)
7. [Errors and exceptions](ERRORES.md)
8. [Async, Future, channels and concurrency](CONCURRENCIA.md)
9. [Full console project with JSON persistence](PROYECTO_CONSOLA.md)
10. [Complete web project with native MVC](PROYECTO_WEB.md)

The [glossary](GLOSARIO.md) explains programming and Joss terms without requiring
that you already know another language.

## Language and tools reference

- [Syntax, tokens and precedence](SINTAXIS.md)
- [EBNF grammar and AST nodes](GRAMATICA.md)
- [Functions](FUNCIONES.md), [recursion](RECURSION.md) and [types](SISTEMA_TIPOS.md)
- [Diagnostics](DIAGNOSTICOS.md) and [analyzer](ANALIZADOR.md)
- [Global functions](FUNCIONES_GLOBALES.md)
- [Native classes and services](MODULOS_NATIVOS.md)
- [Generated native catalog](CATALOGO_NATIVO.md)
- [CLI, formatter, linter and tests](CLI.md)
- [VS Code Extension](VSCODE_EXTENSION.md)
- [State and limits](ESTADO_IMPLEMENTACION.md)

## Create applications

- [Project structure](ESTRUCTURA_PROYECTO.md) and [configuration](CONFIGURACION.md)
- [Organization without imports](MODULOS_IMPORTS.md) and [plugins](PLUGINS.md)
- Web: [server](SERVIDOR.md), [HTTP/controllers](CONTROLADORES.md),
  [middleware](MIDDLEWARE.md), [views](VISTAS.md), [assets](ASSETS.md) and
  [WebSockets](WEBSOCKETS.md)
- Data: [models/GranDB](MODELOS.md), [Schema Builder](SCHEMA_BUILDER.md) and
  [migrations](MIGRACIONES.md)
- Application: [authentication](AUTENTICACION.md) and [SEO/sitemap](SEO_SITEMAP.md)

## Internals and maintenance

- [Language architecture](ARQUITECTURA.md)
- [Contribution Guide](CONTRIBUIR.md)- [Documentation audit](DOCUMENTATION_AUDIT.md)
- [Joss Language Audit 2026](JOSS_LANGUAGE_AUDIT_2026.md)
- [Technical audit of 2026](AUDITORIA_TECNICA_2026.md)
- [Runtime Optimization Audit](RUNTIME_OPTIMIZATION_AUDIT.md)
- [Historical news 3.6.7](NOVEDADES_367.md)

Historical documents preserve context and can describe the state of your
date. In the event of a difference, implementation, testing and
[Implementation status](ESTADO_IMPLEMENTACION.md).

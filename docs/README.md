# Documentación de Joss

Esta documentación describe el comportamiento del código actual. Está separada
en aprendizaje, guías, referencia e internos para que una persona nueva pueda
avanzar en orden y una persona experimentada encuentre reglas concretas.

## Idiomas

- Español (fuente canónica): este directorio.
- [English](en/README.md)
- [Português](pt/README.md)

Los 46 documentos mantienen el mismo nombre en cada idioma para que los enlaces
relativos funcionen igual. Después de editar el español, ejecuta
`go run ./tools/docsi18n -translate -sync` para actualizar traducciones y espejo
web; `go run ./tools/docsi18n -check` detecta archivos ausentes, traducciones
desactualizadas, enlaces rotos o diferencias con JosSecurity.

## Aprender Joss (Ruta guiada para principiantes)

Si nunca has programado o estás aprendiendo Joss por primera vez, lee este recorrido en orden. Cada página introduce los conceptos con explicaciones pedagógicas, ejemplos visuales paso a paso y chuletas rápidas antes de profundizar en los aspectos técnicos avanzados.

0. [Primeros pasos: de un archivo a tu primer programa interactivo con cin](PRIMEROS_PASOS.md)
1. [Valores, variables y operaciones fundamentales](FUNDAMENTOS.md)
2. [Control de flujo y toma de decisiones](CONTROL_FLUJO.md)
3. [Funciones, scope, closures y referencias](FUNCIONES.md)
4. [Arrays, maps y texto](COLECCIONES.md)
5. [Tipos, inferencia y conversiones](SISTEMA_TIPOS.md)
6. [Clases, objetos y herencia](CLASES.md)
7. [Errores y excepciones](ERRORES.md)
8. [Async, Future, channels y concurrencia](CONCURRENCIA.md)
9. [Proyecto de consola completo con persistencia JSON](PROYECTO_CONSOLA.md)
10. [Proyecto web completo con MVC nativo](PROYECTO_WEB.md)

El [glosario](GLOSARIO.md) explica términos de programación y de Joss sin exigir
que ya conozcas otro lenguaje.

## Referencia del lenguaje y herramientas

- [Sintaxis, tokens y precedencia](SINTAXIS.md)
- [Gramática EBNF y nodos AST](GRAMATICA.md)
- [Funciones](FUNCIONES.md), [recursión](RECURSION.md) y [tipos](SISTEMA_TIPOS.md)
- [Diagnósticos](DIAGNOSTICOS.md) y [analizador](ANALIZADOR.md)
- [Funciones globales](FUNCIONES_GLOBALES.md)
- [Clases nativas y servicios](MODULOS_NATIVOS.md)
- [Catálogo nativo generado](CATALOGO_NATIVO.md)
- [CLI, formatter, linter y tests](CLI.md)
- [Extensión de VS Code](VSCODE_EXTENSION.md)
- [Estado y límites](ESTADO_IMPLEMENTACION.md)

## Crear aplicaciones

- [Estructura de proyectos](ESTRUCTURA_PROYECTO.md) y [configuración](CONFIGURACION.md)
- [Organización sin imports](MODULOS_IMPORTS.md) y [plugins](PLUGINS.md)
- Web: [servidor](SERVIDOR.md), [HTTP/controladores](CONTROLADORES.md),
  [middleware](MIDDLEWARE.md), [vistas](VISTAS.md), [assets](ASSETS.md) y
  [WebSockets](WEBSOCKETS.md)
- Datos: [modelos/GranDB](MODELOS.md), [Schema Builder](SCHEMA_BUILDER.md) y
  [migraciones](MIGRACIONES.md)
- Aplicación: [autenticación](AUTENTICACION.md) y [SEO/sitemap](SEO_SITEMAP.md)

## Internos y mantenimiento

- [Arquitectura del lenguaje](ARQUITECTURA.md)
- [Guía de contribución](CONTRIBUIR.md)
- [Auditoría de documentación](DOCUMENTATION_AUDIT.md)
- [Auditoría del lenguaje Joss 2026](JOSS_LANGUAGE_AUDIT_2026.md)
- [Auditoría técnica de 2026](AUDITORIA_TECNICA_2026.md)
- [Auditoría de optimización runtime](RUNTIME_OPTIMIZATION_AUDIT.md)
- [Novedades históricas 3.6.7](NOVEDADES_367.md)

Los documentos históricos conservan contexto y pueden describir el estado de su
fecha. Ante una diferencia mandan la implementación, las pruebas y
[Estado de implementación](ESTADO_IMPLEMENTACION.md).

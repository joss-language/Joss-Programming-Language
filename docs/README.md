# Documentación de Joss

Esta documentación describe el comportamiento del código actual. Está separada
en aprendizaje, guías, referencia e internos para que una persona nueva pueda
avanzar en orden y una persona experimentada encuentre reglas concretas.

## Idiomas

- Español (fuente canónica): este directorio.
- [English](en/README.md)
- [Português](pt/README.md)

Los 50 documentos mantienen el mismo nombre en cada idioma para que los enlaces
relativos funcionen igual. Después de editar el español, ejecuta
`go run ./tools/docsi18n -translate -sync` para actualizar traducciones y espejo
web; `go run ./tools/docsi18n -check` detecta archivos ausentes, traducciones
desactualizadas, enlaces rotos o diferencias con JosSecurity.

## Aprender Joss (Ruta guiada desde cero)

Si nunca has programado o estás aprendiendo Joss por primera vez, sigue este curso en orden. Cada capítulo introduce un concepto a la vez, partiendo desde cómo crear y ejecutar tu proyecto oficial con `joss new console`, con explicaciones línea por línea, resultados visibles y tres niveles de práctica (*Prueba rápida*, *Ejercicio guiado* y *Reto*).

0. [Primeros pasos: crear tu proyecto con `joss new console`, entender `main.joss` y dialogar con `cin`](PRIMEROS_PASOS.md)
1. [Valores, variables y operaciones fundamentales](FUNDAMENTOS.md)
2. [Control de flujo: decisiones y ciclos de repetición](CONTROL_FLUJO.md)
3. [Colecciones: Arrays, Maps y texto Unicode](COLECCIONES.md)
4. [Funciones, ámbito (scope), `guard`, `defer`, closures, `ref` y pipelines](FUNCIONES.md)
5. [Manejo de errores, diagnósticos `JOSS-...` y excepciones `try/catch`](ERRORES.md)
6. [Clases, objetos, herencia, interfaces, `record` y genéricos](CLASES.md)
7. [Concurrencia, `async/await`, `Future` y canales (`channel`)](CONCURRENCIA.md)
8. [Proyecto práctico de consola con persistencia JSON y `joss build`](PROYECTO_CONSOLA.md)
9. [Proyecto web completo con `joss new` y arquitectura MVC nativa](PROYECTO_WEB.md)

El [glosario](GLOSARIO.md) explica todos los términos de programación y de Joss en lenguaje cotidiano, sin exigir que conozcas otro lenguaje. Una vez completado el curso básico, profundiza en las reglas formales de tipos en [Sistema de tipos, inferencia y conversiones](SISTEMA_TIPOS.md).

## Referencia del lenguaje y herramientas

- [Sintaxis, tokens y precedencia](SINTAXIS.md)
- [Gramática EBNF y nodos AST](GRAMATICA.md)
- [Funciones](FUNCIONES.md), [recursión](RECURSION.md) y [tipos](SISTEMA_TIPOS.md)
- [Diagnósticos](DIAGNOSTICOS.md) y [analizador](ANALIZADOR.md)
- [Funciones globales](FUNCIONES_GLOBALES.md)
- [Clases nativas y servicios](MODULOS_NATIVOS.md)
- [Catálogo nativo generado](CATALOGO_NATIVO.md)
- [CLI, formatter, linter y tests](CLI.md)
- [Soporte móvil: Android e iOS](MOBILE.md)
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
- [Evaluación crítica y comparativa de Joss 2026](JOSS_LANGUAGE_REVIEW_2026.md)
- [Evolución de Joss 2026: fases 0 y 1](JOSS_EVOLUTION_PHASE_0_1_2026.md)
- [Auditoría del ORM GranDB 2026](GRANDB_ORM_AUDIT_2026.md)
- [Auditoría técnica de 2026](AUDITORIA_TECNICA_2026.md)
- [Auditoría de optimización runtime](RUNTIME_OPTIMIZATION_AUDIT.md)
- [Novedades históricas 3.6.7](NOVEDADES_367.md)

Los documentos históricos conservan contexto y pueden describir el estado de su
fecha. Ante una diferencia mandan la implementación, las pruebas y
[Estado de implementación](ESTADO_IMPLEMENTACION.md).

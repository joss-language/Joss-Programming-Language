# Estado y límites de implementación

[Índice](README.md) · [Arquitectura](ARQUITECTURA.md) · [Auditoría](DOCUMENTATION_AUDIT.md)

Esta página separa capacidades disponibles, implementaciones parciales y
objetivos de diseño. Corresponde al código auditado, no a garantías de una
versión descargada previamente.

| Área | Estado real | Referencia |
|---|---|---|
| Lenguaje | Parser Pratt, variables de tipo fijo o mixed explícito, clases/herencia/interfaces/enums/visibilidad, funciones/closures/ref, ternarios, guard, loops, match de valores, try/catch, defer y select de canales. | [Sintaxis](SINTAXIS.md) |
| Tipos | int64, float64, decimal, strings, arrays, maps, object, channel, clases, uniones y nullable. Análisis y defensa runtime con diferencias registradas. | [Tipos](SISTEMA_TIPOS.md) |
| Concurrencia | Goroutines mediante async, Future, espera bloqueante y channels. La ejecución móvil admite cancelación cooperativa y captura segura de salida tardía; una llamada nativa bloqueante puede seguir tras el timeout. No existe cancelación estructurada general ni aislamiento profundo. | [Concurrencia](CONCURRENCIA.md) |
| Analizador | Declaraciones, scopes, asignabilidad, miembros conocidos, retornos y diagnósticos; narrowing local en ternarios, guard y comparaciones con null. No existe CFG general ni prueba de terminación completa. | [Analizador](ANALIZADOR.md) |
| Ejecución principal | AST interpretado con planes de callable, frames y caches. Build nativo empaqueta AST comprimido JOSSBC2Z con runner Go. | [Arquitectura](ARQUITECTURA.md) |
| VM experimental | pkg/vm contiene compilador y VM independientes; no es backend por defecto de CLI/core. | [Internos](ARQUITECTURA.md) |
| Web | Router HTTP/WS, vistas, Request/Response, sesión, CSRF, CORS, TLS y límites configurables. | [Proyecto web](PROYECTO_WEB.md) |
| SQL | Adaptadores SQLite/MySQL/PostgreSQL/SQL Server, builder, Schema y migraciones. Portabilidad parcial por operación; las consultas SQL ordinarias del runtime durante GranDB::transaction usan el Tx activo. Transacciones anidadas no están soportadas. | [Modelos](MODELOS.md) |
| Plugins | Contenedor firmado e índice, AST y JPBC; compiladores parciales. Ruta Wasm genera stubs de texto, no ejecuta Wasm. | [Plugins](PLUGINS.md) |
| Permisos de plugins | Guard para llamadas host mapeadas; no sandbox WASI/OS ni consentimiento por paquete. | [Plugins](PLUGINS.md) |
| Herramientas | CLI con REPL, formatter, linter/fix, runner de tests y extensión VS Code con catálogo generado. No hay debugger integrado. | [CLI](CLI.md) |

No existen ownership general, inmutabilidad por defecto, punteros generales,
traits, protocolos, generics de funciones/clases, finally,
ni backend LLVM/Cranelift. Las anotaciones de colecciones no equivalen a generics
universales ni garantizan que cada mutación revalide elementos.

La ausencia de imports/exports/namespaces fuente es una decisión permanente,
no una función pendiente. La modularidad de ALIM utiliza capacidades integradas,
organización física y plugins con carga automática.

La tesis combina arquitectura objetivo e implementación. Sus afirmaciones de
backend, aislamiento y módulos deben contrastarse con este estado y con la
[auditoría técnica histórica](AUDITORIA_TECNICA_2026.md). No presentes objetivos
como prestaciones ya terminadas.

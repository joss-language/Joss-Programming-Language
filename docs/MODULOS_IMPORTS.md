# Módulos, archivos y plugins

[Índice](README.md)

## Estado actual: Descubrimiento automático sin `import`

En Joss, un proyecto se organiza como un conjunto de archivos `.joss` que comparten un espacio global de declaraciones públicas sin necesidad de escribir sentencias `import` manuales:

- El cargador canónico `semanticanalyzer.LoadProject("main.joss", "app")` construye la unidad de proyecto descubriendo automáticamente el archivo de entrada (`main.joss`), los archivos de rutas (`routes.joss`, `api.joss`) y todos los archivos `.joss` dentro de `app/` (incluyendo `controllers`, `models`, `middleware`, `services`, `libs`, `database`, etc.).
- Ese conjunto validado produce un `PreparedProgram` único que es consumido directamente por `joss run` (`Runtime.ExecutePrepared`), `joss server` y `joss build`.
- Visibilidad entre archivos:
  - Las clases, interfaces, enums y funciones declaradas como `public` son visibles en todos los archivos del proyecto.
  - Las declaradas como `private` están restringidas exclusivamente al archivo `.joss` donde fueron definidas (`JOSS-ACCESS-001`).

Las formas históricas `import`, `use`, `@import` y `Namespace` fueron eliminadas del conjunto de tokens, AST, ejecutor y compilador de plugins. El parser las rechaza con un mensaje de migración (`JOSS-PARSE-001`). Esta ausencia es una decisión permanente del lenguaje: no habrá exports, namespaces fuente ni grafo de imports manuales.

## Plugins

La extensibilidad modular externa vigente usa paquetes `.jp`:

- Se declaran en `joss.yaml` o se colocan en `plugins/`.
- El runtime los verifica criptográficamente y los carga automáticamente.
- Cada paquete publica `META-INF/joss-symbols.json`.
- El analyzer consume ese índice para resolver clases, métodos y funciones exportadas sin ejecutar el plugin.

Un símbolo externo que no figure en el índice no se inventa ni se añade a una lista manual del analyzer: debe corregirse el paquete o su generación de símbolos.

## Archivos incluidos por infraestructura

Rutas, controladores, modelos, servicios, librerías y middleware bajo `app/` se descubren automáticamente según el layout de proyecto. Esta inclusión física no crea un namespace por archivo; las funciones y clases `public` top-level comparten el espacio de declaraciones del proyecto y sus duplicados son diagnosticados (`JOSS-SYM-002`, `JOSS-DECL-004`).

## Decisión frente a la tesis

El capítulo 11 de la tesis describe módulos fuente con interfaces, exports y un DAG. La implementación adopta una decisión distinta: “modular” en ALIM significa capacidades integradas desacopladas y plugins `.jp` aislados, no imports escritos por la aplicación. Esta discrepancia es deliberada y el diseño de módulos fuente de la tesis no forma parte de la hoja de ruta de Joss.

Las funciones y clases top-level forman un único espacio de declaraciones del proyecto. Las variables fuente top-level no se heredan dinámicamente dentro de funciones con nombre; deben pasar como parámetros. Una closure sí conserva el entorno léxico que captura.

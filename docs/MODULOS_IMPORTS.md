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

## Biblioteca Estándar (`stdlib/`)

Joss incluye una biblioteca estándar escrita en el propio lenguaje Joss alojada en la carpeta `stdlib/`. Todos los archivos y submódulos dentro de `stdlib/` se cargan y analizan automáticamente en cada proyecto sin necesidad de importar archivos a mano ni configurar rutas de dependencias.

### Módulos disponibles y capacidades

1. **`MathUtils` (`stdlib/math/math.joss`):**
   - `MathUtils::clamp(int $val, int $min, int $max): int`: Restringe un valor dentro de un rango inclusivo mínimo y máximo.
   - `MathUtils::sign(int $n): int`: Retorna `1` si el número es positivo, `-1` si es negativo y `0` si es cero.
   - `MathUtils::gcd(int $a, int $b): int`: Calcula el máximo común divisor entre dos números enteros mediante el algoritmo de Euclides.

2. **`StringUtils` (`stdlib/strings/strings.joss`):**
   - `StringUtils::isEmpty(string $s): bool`: Verifica de forma rápida si una cadena tiene longitud 0.
   - `StringUtils::isNotEmpty(string $s): bool`: Verifica si una cadena contiene al menos un carácter.
   - `StringUtils::padZero(int $num, int $width): string`: Rellena un número con ceros a la izquierda hasta alcanzar la longitud especificada.

### Ejemplo de uso

```joss
<!-- joss-check: uso directo de la biblioteca estándar -->
int $minimo = MathUtils::clamp(150, 0, 100);
echo $minimo; // 100

int $signo = MathUtils::sign(-45);
echo $signo; // -1

string $codigo = StringUtils::padZero(42, 5);
echo $codigo; // "00042"
```

## Interfaz de funciones foráneas (FFI con C)

Para interactuar con bibliotecas dinámicas nativas (`.dll`, `.so`, `.dylib`), Joss proporciona la sintaxis de bloque `extern`:

```joss
<!-- joss-check: declaración FFI externa -->
extern "C" library "user32.dll" {
    public func MessageBoxA(int $hWnd, string $lpText, string $lpCaption, int $uType): int;
}
```

Las funciones declaradas dentro de un bloque `extern` se registran en la tabla global de símbolos y comparten la misma convención directa de código máquina nativo de la CPU.

## Decisión frente a la tesis

El capítulo 11 de la tesis describe módulos fuente con interfaces, exports y un DAG. La implementación adopta una decisión distinta: “modular” en ALIM significa capacidades integradas desacopladas y plugins `.jp` aislados, no imports escritos por la aplicación. Esta discrepancia es deliberada y el diseño de módulos fuente de la tesis no forma parte de la hoja de ruta de Joss.

Las funciones y clases top-level forman un único espacio de declaraciones del proyecto. Las variables fuente top-level no se heredan dinámicamente dentro de funciones con nombre; deben pasar como parámetros. Una closure sí conserva el entorno léxico que captura.

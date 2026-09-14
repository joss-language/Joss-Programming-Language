# Informe de auditoría integral y reconstrucción documental de Joss

Antes: [Contribuir a Joss](CONTRIBUIR.md). Después: [Auditoría técnica de 2026](AUDITORIA_TECNICA_2026.md).
Índice general: [Documentación de Joss](README.md).

---

## 1. Introducción y propósito de la auditoría

El presente informe documenta la auditoría exhaustiva y la reconstrucción profunda de la documentación del lenguaje de programación **Joss**, llevada a cabo el 5 de septiembre de 2026.

El principio rector absoluto de este trabajo ha sido:

> **El código fuente es la única fuente de verdad.**

Toda la documentación anterior fue contrastada contra el comportamiento real del analizador léxico (`pkg/parser/lexer.go`), el parser Pratt (`pkg/parser`), el árbol de sintaxis abstracta AST (`pkg/parser/ast*.go`), el sistema de tipos e inferencia (`pkg/typesystem`), el analizador semántico (`pkg/analyzer`), el evaluador y motor de ejecución en Go (`pkg/core`), las clases y funciones nativas registradas, y la suite de pruebas automatizadas.

---

## 2. Diagnóstico del estado inicial y deuda documental

Al iniciar la auditoría, se detectó que la documentación existente presentaba una brecha significativa respecto a las capacidades reales del código:

### A. Brecha pedagógica para principiantes
- La documentación estaba redactada casi exclusivamente como una referencia técnica compacta para personas que ya dominaban otros lenguajes o conocían la jerga de compiladores.
- No existían explicaciones sobre conceptos fundacionales como qué es una variable, por qué se utiliza `$`, la diferencia entre imprimir en pantalla (`print`) y retornar un valor (`return`), o qué representa la memoria y el ámbito (*scope*).
- Conceptos avanzados como `async`, `await` o `channel` se presentaban mediante fragmentos de código sin explicar previamente la diferencia entre sincronía, asincronía y concurrencia.

### B. Características implementadas en el código que estaban sin documentar o subdocumentadas
Durante la inspección línea por línea del código fuente se descubrieron comportamientos y sintaxis que estaban implementados pero ausentes en las guías:
1. **El operador Pipeline (`|>`)**:
   - Implementado en `pkg/parser/parser.go` (precedencia `PIPE_OP`), `pkg/parser/parser_expressions.go` y evaluado en `pkg/core/evaluator_infix.go`.
   - Permite componer funciones de izquierda a derecha: `" ada " |> trim |> strtoupper`. Si la función requiere más argumentos, el valor izquierdo se inyecta como primer parámetro. Estaba ausente en los tutoriales.
2. **El operador Elvis (`?:`)**:
   - Implementado en el analizador de ternarios (`parseTernaryExpression`) y evaluado en `evaluateTernary`. Permite evaluar valores por defecto cuando la condición es truthy: `$val = $input ?: "default"`.
3. **El operador de coalescencia nula (`??`)**:
   - Implementado con protección ante errores nulos para fallbacks limpios: `$val = $input ?? "fallback"`.
4. **El operador de navegación segura ante nulos (`?->`)**:
   - Permite acceder a propiedades y métodos de instancias potencialmente nulas (`$user?->nombre`) sin estrellar el programa.
5. **Sintaxis de auto-append en arrays (`$arr[] = $val`)**:
   - Soportado en el evaluador de asignaciones (`IndexExpression` con índice nulo), permitiendo agregar elementos al final de forma natural.
6. **Consumo de canales concurrentes mediante `foreach`**:
   - `executeForeach` en `pkg/core/executor.go` comprueba si el iterable es un `*Channel` (`for item := range ch.Ch`), permitiendo crear patrones productor-consumidor puros sin llamadas manuales repetitivas a `recv`.
7. **Estructura del mapa de error capturado en `catch ($e)`**:
   - Cuando ocurre un `JossError` en tiempo de ejecución, el bloque `catch` recibe un mapa asociativo estructurado con las claves `"message"`, `"type"`, `"file"`, `"line"` y `"error"`.
8. **Invarianza bilateral de referencias (`ref`)**:
   - El código impone que tanto la declaración de la función como la llamada utilicen `ref`, exige tipos idénticos (sin ampliación `int` a `float`) y prohíbe el escape de la referencia fuera del frame.

### C. Documentación obsoleta y desinformación eliminada
- **Aliases retirados**: Se aclaró categóricamente que `integer`, `double`, `boolean`, `dynamic`, `any` y `list` ya no son normalizados; usarlos dispara el error `JOSS-TYPE-009`.
- **Sintaxis async obsoleta**: Se documentó que `async(func() ...)` fue eliminado y que la única sintaxis válida es el bloque `async { ... }`.
- **APIs fantasma o mal catalogadas**: Se eliminaron menciones a métodos no registrados en el dispatcher como `Http::query` o `System::change_db`.
- **Comportamiento no mutante de `array_pop` y `array_shift`**: Se alertó explícitamente a los desarrolladores de que en Joss estas funciones devuelven el elemento sin reducir la longitud del array original.

---

## 3. Nueva arquitectura documental del proyecto

Se reestructuró la documentación en cuatro niveles complementarios para satisfacer a todas las audiencias sin degradar el rigor técnico:

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. APRENDER JOSS (Niveles 0 al 10 - Progresivo y pedagógico) │
│    - PRIMEROS_PASOS: De cero a un programa ejecutable.      │
│    - FUNDAMENTOS: Memoria, tipos primitivos y variables.    │
│    - CONTROL_FLUJO: Ternarios con bloques, match y bucles.  │
│    - FUNCIONES: Scope, closures, ref y pipelines.           │
│    - COLECCIONES: Arrays, maps y texto Unicode.             │
│    - SISTEMA_TIPOS: Inferencia, uniones y conversiones.     │
│    - CLASES: POO, Init, encapsulación y herencia.           │
│    - ERRORES: Fases, diagnósticos y try/catch.              │
│    - CONCURRENCIA: Async, Future y canales.                 │
│    - PROYECTO_CONSOLA: Proyecto real con persistencia JSON. │
│    - PROYECTO_WEB: Aplicación MVC con el stack nativo.      │
│    - GLOSARIO: Diccionario conceptual para principiantes.   │
├─────────────────────────────────────────────────────────────┤
│ 2. REFERENCIA TÉCNICA DEL LENGUAJE Y HERRAMIENTAS           │
│    - SINTAXIS: Tokens, precedencias y operadores.           │
│    - GRAMATICA: EBNF formal y correspondencia con el AST.   │
│    - DIAGNOSTICOS: Catálogo completo de códigos JOSS-*.     │
│    - FUNCIONES_GLOBALES: Las 121 funciones built-in.        │
│    - MODULOS_NATIVOS: Clases integradas en Go.              │
│    - CATALOGO_NATIVO: Catálogo sincronizado por docgen.     │
│    - CLI: Referencia de comandos joss.                      │
│    - VSCODE_EXTENSION: LSP y tooling del editor.            │
│    - ESTADO_IMPLEMENTACION: Estado real vs límites.         │
├─────────────────────────────────────────────────────────────┤
│ 3. DESARROLLO DE APLICACIONES                               │
│    - ESTRUCTURA_PROYECTO, CONFIGURACION, MODULOS_IMPORTS.   │
│    - SERVIDOR, CONTROLADORES, MIDDLEWARE, VISTAS, ASSETS.   │
│    - MODELOS, SCHEMA_BUILDER, MIGRACIONES, AUTENTICACION.   │
│    - WEBSOCKETS, PLUGINS, SEO_SITEMAP.                      │
├─────────────────────────────────────────────────────────────┤
│ 4. INTERNALS Y GUÍA PARA CONTRIBUIDORES                     │
│    - ARQUITECTURA: Pipeline del compilador y runtime Go.    │
│    - CONTRIBUIR: Cómo extender sintaxis, tipos y built-ins. │
│    - AUDITORIAS Y NOVEDADES: Historial y optimizaciones.    │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. Problemas de experiencia del desarrollador (UX) detectados en el lenguaje

Durante la auditoría del código fuente y la ejecución de pruebas, se identificaron los siguientes puntos donde el comportamiento actual del lenguaje puede inducir a confusión o requerir atención en el futuro:

1. **Detección de `break` / `continue` en ternarios anidados**:
   - El optimizador de planes de bucle (`pkg/runtime/plan`) busca saltos directos en el AST. Si un `break` o `continue` se encuentra dentro del bloque de un ternario en el bucle, en ciertas situaciones el plan no lo detecta y escapa como un panic interno en lugar de ser absorbido por el ciclo.
2. **Brazos de `match` que son bloques**:
   - En la implementación actual de `pkg/core`, un brazo `match` cuyo valor derecho es un bloque `{ ... }` puede retornar el nodo sintáctico del bloque como valor en lugar de ejecutar sus declaraciones internas, a pesar de que el analizador semántico lo contabiliza como retorno válido. Se recomienda usar expresiones directas en los brazos de `match`.
3. **Comportamiento no atómico de `GranDB::transaction`**:
   - `GranDB::transaction` abre una transacción SQL en Go (`tx, err := db.Begin()`), pero las llamadas normales ejecutadas dentro de la closure de usuario utilizan la conexión de base de datos estándar de la instancia y no el puntero `*sql.Tx`, por lo que no garantizan atomicidad completa ante errores dentro del callback.
4. **Discrepancia en `array_pop` y `array_shift`**:
   - Los desarrolladores que provienen de PHP o JavaScript esperan que `array_pop` elimine el elemento del array original. En Joss, retorna el valor pero el array subyacente conserva su tamaño.
5. **Asimetría de análisis estático en proyectos web**:
   - El comando `joss analyze main.joss` escanea `main.joss` y los archivos dentro de `app/**/*.joss`. Sin embargo, `routes.joss` se procesa únicamente cuando arranca el servidor HTTP mediante su propio cargador, por lo que un error sintáctico en `routes.joss` solo se descubre al ejecutar `joss server start`.
6. **Seguridad en generación de números pseudoaleatorios**:
   - Ciertas funciones utilitarias nativas (como `Str::random` y el módulo TOTP) utilizan el paquete `math/rand` con semillas predecibles en lugar del generador criptográficamente seguro `crypto/rand`.

---

## 5. Verificación formal y pruebas ejecutadas

Para garantizar que ningún documento presente enlaces rotos, ejemplos desactualizados o inconsistencias con el runtime, se ejecutó la suite de validación completa:

1. **Verificación de catálogo y documentación nativa**:
   ```bash
   go run ./tools/cataloggen --check
   go run ./tools/docgen --check
   ```
2. **Suite de pruebas unitarias y de integración**:
   ```bash
   go test ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
   ```
3. **Verificación de contratos y navegación (`TestDocumentationNavigationAndPublicMirror` y `TestDocumentationContracts`)**:
   - Comprobación de todos los enlaces Markdown locales entre documentos.
   - Verificación de paridad byte por byte entre `docs/*.md` y `ejemplos/Joss-Red-JosSecurity/assets/docs/*.md`.
   - Ejecución y análisis de todos los bloques etiquetados con `<!-- joss-run: ... -->`, `<!-- joss-check: ... -->` y `<!-- joss-error: ... -->`.
4. **Construcción general del repositorio**:
   ```bash
   go build ./...
   ```

---

## 6. Conclusión

Con esta reconstrucción, la documentación de Joss deja de ser un simple catálogo de sintaxis para convertirse en un **sistema pedagógico y técnico integral**. Permite que cualquier persona sin experiencia previa aprenda a programar desde cero paso a paso, al tiempo que proporciona a desarrolladores experimentados y contribuidores del compilador una referencia exhaustiva, honesta y verificable basada al 100% en el código fuente.

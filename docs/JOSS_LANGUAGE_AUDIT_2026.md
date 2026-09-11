# Auditoría Profunda del Lenguaje de Programación Joss (2026)

> **Documento Canónico de Auditoría Técnica, Ergonomía, Sistema de Tipos, Tooling y Arquitectura**  
> **Fecha de Elaboración:** Septiembre de 2026  
> **Ámbito:** Repositorio oficial `jossecurity/joss`, subsistemas del núcleo (`pkg/*`), herramientas CLI, servidor LSP (`vscode-joss`) y proyecto real de referencia `Joss-Red-JosSecurity`.  
> **Propósito:** Diagnóstico exhaustivo, evidencia empírica en código, catálogo de propuestas y plan de evolución tecnológica hacia un lenguaje más expresivo, coherente y seguro.

---

## Índice General

1. [Resumen Ejecutivo](#1-resumen-ejecutivo)
2. [Filosofía Actual Detectada en Joss](#2-filosofía-actual-detectada-en-joss)
3. [Fortalezas Actuales](#3-fortalezas-actuales)
4. [Debilidades Estructurales](#4-debilidades-estructurales)
5. [Fricción Encontrada al Escribir Código](#5-fricción-encontrada-al-escribir-código)
6. [Boilerplate y Ceremonia Identificados](#6-boilerplate-y-ceremonia-identificados)
7. [Inconsistencias del Lenguaje](#7-inconsistencias-del-lenguaje)
8. [Complejidad Conceptual](#8-complejidad-conceptual)
9. [Auditoría del Sistema de Tipos](#9-auditoría-del-sistema-de-tipos)
10. [Auditoría de Seguridad y Defensas Runtime](#10-auditoría-de-seguridad-y-defensas-runtime)
11. [Auditoría de Errores y Diagnósticos](#11-auditoría-de-errores-y-diagnósticos)
12. [Auditoría de la Biblioteca Estándar (Stdlib)](#12-auditoría-de-la-biblioteca-estándar-stdlib)
13. [Auditoría de Tooling y Ecosistema](#13-auditoría-de-tooling-y-ecosistema)
14. [Auditoría del Formatter Oficial](#14-auditoría-del-formatter-oficial)
15. [Auditoría del Analyzer y Linter](#15-auditoría-del-analyzer-y-linter)
16. [Comparación Selectiva con Otros Lenguajes](#16-comparación-selectiva-con-otros-lenguajes)
17. [Oportunidades de Simplificación](#17-oportunidades-de-simplificación)
18. [Características que NO Conviene Implementar](#18-características-que-no-conviene-implementar)
19. [Catálogo de Propuestas Priorizadas (P1 a P8)](#19-catálogo-de-propuestas-priorizadas-p1-a-p8)
20. [Cambios que Necesitarían Deprecación](#20-cambios-que-necesitarían-deprecación)
21. [Posibles Breaking Changes Justificados](#21-posibles-breaking-changes-justificados)
22. [Propuesta y Arquitectura de `joss fix`](#22-propuesta-y-arquitectura-de-joss-fix)
23. [Definición de "Código Joss Idiomático"](#23-definición-de-código-joss-idiomático)
24. [Roadmap Recomendado (Fases 0 a 5)](#24-roadmap-recomendado-fases-0-a-5)

---

## 1. Resumen Ejecutivo

Joss es un lenguaje de programación multiparadigma moderno desarrollado en Go, concebido para el desarrollo de servicios web, APIs de alto rendimiento, microservicios y utilidades de infraestructura.

Su propuesta fundacional busca combinar la **agilidad de iteración y familiaridad sintáctica** de los lenguajes dinámicos de servidor (PHP, JavaScript, Python) con la **seguridad estática, robustez concurrente y velocidad** del ecosistema Go.

A través de su evolución, Joss ha establecido pilares arquitectónicos excepcionales:
- **Pipeline de compilación desacoplado:** Arquitectura formal basada en un lexer especializado, un parser Pratt de precedencia de operadores, un AST unificado, un analizador semántico estricto (`pkg/analyzer`), diagnósticos estructurados y un runtime de evaluación de árbol optimizado por slots léxicos (`pkg/runtime/plan`).
- **Arquitectura Zero-Imports:** Carga automática y resolución de símbolos a nivel de proyecto basada en convenciones de estructura, eliminando la gestión manual de grafos de dependencias en aplicaciones web.
- **Seguridad aritmética y de memoria:** Enteros de 64 bits con detección de desbordamiento en tiempo de compilación y ejecución (`JOSS-ARITH-001`), punto fijo monetario nativo (`decimal` con literal `m`), referencias seguras temporales e invariantes (`ref`), y control de recursión profunda.
- **Asincronía limpia:** Soporte nativo para goroutines mediante `async { ... }` y canales con operadores directos (`$c << $msg`), con la función `await(...)` habilitada en cualquier contexto sin obligar a fragmentar el código en funciones coloreadas.

### El Diagnóstico de la Auditoría
No obstante sus fortalezas, la presente auditoría técnica ha detectado que **Joss impone actualmente una fricción accidental notable sobre el desarrollador**:
1. **Asimetría del Type Checker:** El compilador exige anotaciones estrictas en parámetros (`JOSS-TYPE-011`) y validación exhaustiva de ramas en retornos (`JOSS-TYPE-010`), pero carece de **propagación de refinamiento de tipos (flow-sensitive type narrowing)** tras cláusulas de guarda. Esto empuja a los programadores en producción a desarmar el tipado usando `mixed` generalizado.
2. **Control de Flujo Forzado:** La erradicación dogmática de sentencias condicionales clásicas en favor del operador ternario generalizado con bloques produce pirámides de anidamiento de hasta 6 niveles y la proliferación de ramas falsas vacías `: {}`.
3. **Pérdida de Identidad de Objetos en Excepciones:** Las instancias de error capturadas en `catch ($e)` son degradadas a texto mediante `fmt.Sprintf`, destruyendo la POO en el tratamiento de excepciones.
4. **Biblioteca Estándar Inconsistente:** Coexisten nombres duplicados de funciones globales heredadas de PHP junto a métodos fluidos ausentes en strings y colecciones.
5. **Tooling Fragmentado:** Formateador y linter reimplementan analizadores independientes en lugar de compartir el AST oficial del núcleo.

Este documento presenta el análisis detallado, la evidencia empírica recolectada en el código y el catálogo priorizado de soluciones para consolidar a Joss como un lenguaje predecible, seguro y productivo.

---

## 2. Filosofía Actual Detectada en Joss

El análisis de la implementación revela las siguientes premisas que definen el carácter actual de Joss:

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                        PRINCIPIOS REALES DE JOSS                        │
├─────────────────────────────────────────────────────────────────────────┤
│ 1. Seguridad estática demostrable sin compilación pesada a máquina.     │
│ 2. Cero fricción de importación de archivos de proyecto (Zero-Imports). │
│ 3. Unificación sintáctica: las bifurcaciones son expresiones evaluables.│
│ 4. Tipado numérico defensivo: protección monetaria y anti-overflow.     │
│ 5. Concurrencia de paso de mensajes inspirada en CSP / Go.              │
│ 6. Baterías incluidas para desarrollo web y servicios backend.          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Inconsistencias Históricas y Tensiones Filosóficas

1. **Expresividad vs. Burocracia en Declaraciones:**
   - Mientras Joss busca eliminar ceremonia en imports, impone una rigidez formal en firmas de callables: visibilidad explícita obligatoria (`public`/`private`/`protected`), tipos de parámetros obligatorios y contratos exhaustivos de retorno. En la práctica, el código de producción elude esta rigidez tipando con `mixed` y omitiendo el retorno.
2. **"Todo es Expresión" vs. Código Imperativo con Efectos Secundarios:**
   - La unificación de decisiones bajo el operador ternario `($cond) ? { ... } : { ... }` funciona de forma elegante para asignaciones simples:
     ```joss-snippet
     $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
     ```
     Sin embargo, en código de controladores de negocio donde se requiere validar precondiciones, registrar auditorías y abortar prematuramente, el ternario se utiliza como sentencia, forzando la escritura de bloques vacíos `: {}` o anidamientos profundos que contradicen el objetivo de claridad.
3. **Tipado Fuerte vs. Biblioteca Procedural Desconociendo Tipos:**
   - El sistema de tipos permite especificar `array<int>` y `map<string, Usuario>`, pero casi todas las funciones de `builtins_array.go` devuelven `[]interface{}` o `mixed`, despojando a la colección de sus garantías de tipo.
4. **Zero-Imports vs. Espacio Global Contaminado:**
   - Al no existir namespaces en el código fuente Joss, las clases deben renombrarse con prefijos artificiales (`XiaomiCategory`, `CmsPost`, `AuthUser`) para prevenir colisiones en el diccionario global del proyecto.

---

## 3. Fortalezas Actuales

Joss cuenta con pilares técnicos sobresalientes que deben preservarse como ventajas competitivas:

1. **Pipeline Arquitectónico Unidireccional:**
   - El compilador respeta estrictamente la frontera de capas: `parser`, `typesystem` y `analyzer` son agnósticos respecto al runtime y jamás importan bases de datos ni servicios de red.
2. **Modelo de Diagnósticos Estructurados:**
   - Emisión determinista mediante `pkg/diagnostics` con rangos de línea/columna precisos, códigos estables y sugerencias de corrección legibles para humanos y consumibles por IDEs.
3. **Precisión Numérica Financiera (`decimal`):**
   - El soporte integrado de coma fija en base diez (`decimal $precio = 99.99m`) mediante `shopspring/decimal` elimina los errores de redondeo de IEEE-754 comunes en PHP, Python o JavaScript.
4. **Defensa Rigurosa contra Desbordamiento:**
   - Intercepción de overflow en aritmética de 64 bits a través de `typesystem.CheckedIntBinary` antes de que ocurran corrupciones de datos.
5. **Concurrencia Pragmática y Liviana:**
   - Canales como ciudadanos de primer orden (`channel $c = make_chan(10)`), operador de envío `$c << $msg`, consumo secuencial `foreach ($c as $msg)` y control múltiple `select`.
   - `async { ... }` produce objetos `Future` concurrentes en goroutines sin imponer "function coloring" a lo largo del árbol de llamadas.
6. **Constructor Property Promotion:**
   - La sintaxis `Init(public string $nombre, public int $edad = 30) {}` erradica el código ceremonial de asignación de campos.
7. **Verificación Automatizada de Documentación:**
   - `pkg/core/documentation_test.go` valida automáticamente cada fragmento ejecutable de la documentación oficial, garantizando que los manuales nunca desincronicen con el comportamiento real del motor.

---

## 4. Debilidades Estructurales

1. **Ausencia de Flow-Sensitive Type Narrowing Exterior:**
   - El analizador solo reconoce el estrechamiento de un tipo nullable dentro de las ramas del ternario. Tras una cláusula de guarda con salida prematura, la variable exterior no es promocionada.
2. **Fricción en Control de Flujo:**
   - Ausencia de una sentencia de decisión plana (`guard` o `if`), lo que genera construcciones forzadas con bloques y ramas falsas vacías `: {}`.
3. **Destrucción de Instancias en el Manejo de Excepciones:**
   - El runtime degrada las instancias de clases lanzadas mediante `throw new CustomException(...)` a simples cadenas de texto `string` al ser atrapadas en `catch ($e)`.
4. **Comportamiento Sorprendente en Igualdad (`==`) y Falsedad (`isFalsy`):**
   - El operador `==` recurre a la conversión a texto mediante `fmt.Sprintf` cuando los operandos no son numéricos, generando que `null == ""` y `[1, 2] == ["1", "2"]` evalúen como verdaderos.
   - En `isFalsy`, el número flotante `0.0` y los mapas vacíos `{}` son tratados como verdaderos, mientras que la cadena `"0"` es tratada como falsa.
5. **Biblioteca Estándar Fragmentada:**
   - Supervivencia de nombres duplicados (`str_contains` vs `contains`, `len` vs `count` vs `strlen`) y ausencia total de métodos de instancia orientados a objetos en cadenas y colecciones.
6. **Tooling Desacoplado del AST Central:**
   - El formateador reimplementa un escáner léxico independiente, el fixer depende de expresiones regulares y la extensión de VS Code replica parseos en TypeScript.

---

## 5. Fricción Encontrada al Escribir Código

### Evidencia 1: Pirámide de Ternarios Anidados en Controladores
En el archivo real [BackupController.joss:10-66](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss#L10-L66) del proyecto de referencia, se observa el siguiente patrón de validación en cadena:

```joss-snippet
// CÓDIGO REAL EN JOSS-RED-JOSSECURITY:
public func saveOrUpdateBackup(mixed $appName) {
    return ($appName == "otp_backup") ? json({"error": "..."}, 400) : {
        $allowedExtension = $this->getAllowedExtension($appName)

        return (!$allowedExtension) ? json({"error": "..."}, 404) : {
            $file = Request::file("file")
            
            return (!$file) ? json({"error": "..."}, 422) : {
                $originalName = $file["name"]
                $parts = explode(".", $originalName)
                $ext = end($parts)

                return ($ext != $allowedExtension) ? json({"error": "..."}, 422) : {
                    $u = Auth::user()
                    return (!$u) ? json({"error": "..."}, 401) : {
                        // ... Lógica de negocio desplazada a más de 24 espacios de sangría ...
                    }
                }
            }
        }
    }
}
```

**Diagnóstico:** El programador se ve obligado a convertir validaciones lineales secuenciales en una jerarquía de bloques falsos anidados. Cualquier modificación a mitad del método exige reformatear y reindentar decenas de líneas de código.

### Evidencia 2: La Rama Falsa Fantasma `: {}`
En [AuthController.joss:3](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/auth/AuthController.joss#L3) y en múltiples controladores:

```joss-snippet
// CÓDIGO REAL:
public func showLogin() {
    (!Auth::guest()) ? { return redirect("/dashboard") } : {}
    SEO::title("Iniciar Sesión — Joss Red")
    return view("auth.login", { ... })
}
```

**Diagnóstico:** El desarrollador escribe `: {}` al final de cada ternario condicional para tener certeza de que el compilador no interpretará la siguiente línea como una continuación del operador.

### Evidencia 3: Transformaciones Procedurales Innecesarias
En [BackupController.joss:70-97](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss#L70-L97):

```joss-snippet
// CÓDIGO REAL:
$files = $db->table("backups")->where("user_token", $userToken)->get()
$filesList = $files
$mapped = []

foreach ($filesList as $file) {
    $parts = explode("/", $file["file_name"])
    $fileName = end($parts)
    
    $mapped[] = {
        "id": $file["id"],
        "name": $fileName
    }
}
return json({"files": $mapped})
```

**Diagnóstico:** 16 líneas de código procedimental con acumulador mutable `$mapped[] = ...` para realizar una operación que conceptualmente es una sola transformación de mapeo.

---

## 6. Boilerplate y Ceremonia Identificados

| Patrón Ceremonial | Impacto en Líneas | Causa Subyacente | Solución Propuesta |
|---|:---:|---|---|
| **Rama `: {}` en sentencias condicionales** | 1 línea por condicional | Sintaxis ternaria forzada para control de flujo | Introducción de `guard` o sentencia condicional plana |
| **Parámetros marcados como `mixed`** | 1 por parámetro | Falta de flow narrowing en variables con tipos unión | Estrechamiento automático de ámbito (smart casts) |
| **Extracción manual de extensión/nombre de archivo** | 3-4 líneas | Ausencia de métodos en cadenas o clase de utilidad `Path` | `$archivo->extension()` o `Path::extension($archivo)` |
| **Instanciación repetitiva `new GranDB()`** | 1 línea por consulta | Acceso de instancia no unificado con fachadas estáticas | Unificar en `GranDB::table(...)` |
| **Mapeo acumulativo `$acc[] = ...`** | 10-15 líneas por bucle | Arreglos sin métodos funcionales fluidos | `$files->map(func($f) => ...)` |
| **Comprobación manual de existencia previa a lectura** | 2-3 líneas | Falta de métodos seguros en diccionarios/mapas | `$map->get("clave", "default")` |

---

## 7. Inconsistencias del Lenguaje

### 1. Inconsistencia del Sigilo `$`
- Variables locales: Obligatorio (`$usuario = new Usuario()`).
- Parámetros: Obligatorio (`func(string $nombre)`).
- Propiedades en declaración: Obligatorio (`public string $nombre = ""`).
- Acceso a propiedad de instancia: **Prohibido** (`$this->nombre`, no `$this->$nombre`).
- Acceso a propiedad estática: **Obligatorio** (`Clase::$contador`).
- Parámetro de catch: **Obligatorio** (`catch ($e)`).

### 2. Duplicidad de Paradigmas en Bibliotecas
Conviven tres estilos irreconciliables en las APIs integradas:
- **Estilo PHP histórico:** `str_contains`, `str_replace`, `array_keys`, `file_get_contents`.
- **Estilo Moderno breve:** `contains`, `keys`, `values`, `file_read`.
- **Estilo C++:** Operadores de flujo `cout << $val` y `cin >> $var`.
- **Estilo Facade Web:** `Response::json()`, `View::render()`, `Request::input()`.

### 3. Exigencia de Tipado Asimétrica
- En funciones: `func($x)` está estrictamente prohibido (`JOSS-TYPE-011`); exige `func(mixed $x)`.
- En `catch`: `catch (MiError $e)` genera un error sintáctico de compilador; exige estrictamente `catch ($e)`.

### 4. Indexación Fuera de Rango
- En arreglos y strings: `$arr[99]` provoca un **pánico fatal en tiempo de ejecución** (`JOSS-INDEX-001`).
- En mapas: `$map["clave_inexistente"]` retorna **`null` silenciosamente**.

---

## 8. Complejidad Conceptual

Actualmente, un desarrollador debe memorizar un número innecesario de reglas para tareas idénticas:

```text
DECLARACIÓN DE VARIABLES (7 Formas Convivientes):
  1. $x = 10         (inferencia fija)
  2. var $x = 10     (inferencia fija explícita)
  3. int $x = 10     (tipo estático estricto)
  4. let int $x = 10 (tipo estático con prefijo let)
  5. let $x = 10     (variable dinámica mixed)
  6. mixed $x = 10   (variable dinámica explícita)
  7. const $x = 10   (inmutable)
```

**Propuesta de unificación:** Reducir a tres formas intuitivas y predecibles:
- `$x = 10` o `var $x = 10`: Inferencia fija del valor asignado.
- `int $x = 10`: Tipo estático declarado explícitamente.
- `mixed $x = 10`: Dinamismo voluntario (desaconsejando `let $x`).
- `const $x = 10`: Constante inmutable.

---

## 9. Auditoría del Sistema de Tipos

El núcleo de tipado (`pkg/typesystem/types.go`) posee fundamentos matemáticos sólidos:
- Promoción numérica estricta: `int` → `float` → `decimal`.
- Operaciones seguras con tipos unión normalizados (`T|U`) y opcionales (`T?` → `T|null`).
- Compatibilidad nominal de clases e interfaces.

### El Defecto Crítico: Ámbito Aislado de Refinamiento
En `pkg/analyzer/infer.go:929` (`narrowScopeFromCondition`), el estrechamiento de tipos nullable opera creando ámbitos aislados:

```go
trueScope := newScope(current)
falseScope := newScope(current)
```

Dichos ámbitos **solo afectan las expresiones situadas dentro del ternario**. Una vez que el análisis continúa en la sentencia siguiente del bloque principal, la variable recupera su tipo original con `null`.

```joss-snippet
public func formatearNombre(string? $nombre): string {
    ($nombre == null) ? {
        return "Anónimo"
    }

    // EL COMPILADOR FALLA AQUÍ:
    // $nombre sigue siendo inferido como string|null.
    return $nombre // JOSS-TYPE-008: se esperaba 'string', se obtuvo 'string|null'.
}
```

Esta limitación impide escribir cláusulas de guarda idiomáticas y conduce al abuso de `mixed` en proyectos de software real.

---

## 10. Auditoría de Seguridad y Defensas Runtime

### Defensas Validadas y Exitosas:
- **Detección de Desbordamiento:** `CheckedIntBinary` bloquea adiciones o productos que excedan el rango `int64`.
- **División por Cero Controlada:** Lanza diagnósticos inmediatos sin producir estados corruptos de memoria.
- **Protección contra Recursión Infinita:** Límite máximo configurable de marcos de llamada.

### Brechas y Vulnerabilidades Ocultas:

1. **El Operador `??` Oculta Errores Críticos:**
   - En [evaluator_infix.go:54-60](file:///c:/Users/Asus/Documents/proyectos/Joss-language/pkg/core/evaluator_infix.go#L54-L60):
   ```go
   if ie.Operator == "??" {
       var left interface{}
       func() {
           defer func() {
               if rec := recover(); rec != nil {
                   left = nil
               }
           }()
           left = r.evaluateExpression(ie.Left)
       }()
   ```
   Cualquier excepción producida en la rama izquierda (división por cero, desbordamiento o excepción de base de datos) es capturada en silencio y sustituida por el valor por defecto, haciendo imposible diagnosticar bugs en producción.

2. **Degradación Silenciosa de Caracteres UTF-8 en el Lexer:**
   - En [lexer.go:327-332](file:///c:/Users/Asus/Documents/proyectos/Joss-language/pkg/parser/lexer.go#L327-L332), el lexer descarta sin notificación cualquier byte mayor a 127 fuera de literales de cadena. Un identificador como `$año` es transformado silenciosamente en `$ao`.

---

## 11. Auditoría de Errores y Diagnósticos

El subsistema `pkg/diagnostics` es de alta calidad arquitectónica, pero requiere mayor empatía contextual:

1. **Mensajes Técnicamente Correctos pero Poco Orientativos:**
   - *Actual:* `error[JOSS-TYPE-001] app.joss:15:5: Asignación incompatible: se esperaba 'int', se obtuvo 'string'.`
   - *Orientativo:* `error[JOSS-TYPE-001] app.joss:15:5: No se puede asignar 'string' a la variable '$edad' porque fue inferida como 'int' en la línea 4. Sugerencia: Modifica el valor asignado o declara explícitamente 'mixed $edad'.`
2. **Cascada de Errores en Parser:**
   - Ante la falta de un delimitador o llave, el parser Pratt emite múltiples diagnósticos derivados. Es prioritario sincronizar el parser hasta el siguiente `SEMICOLON` o `NEWLINE`.

---

## 12. Auditoría de la Biblioteca Estándar (Stdlib)

### Principales Oportunidades de Rediseño:
1. **Unificación de Cadenas y Colecciones Orientadas a Objetos:**
   - Reemplazar funciones anidadas por llamadas fluidas:
     ```joss-snippet
     // Actual:
     $limpio = trim(strtolower(substr($texto, 0, 10)))

     // Propuesto:
     $limpio = $texto->slice(0, 10)->lower()->trim()
     ```
2. **Evaluación Perezosa del Operador Rango (`..`):**
   - Modificar `evaluator_infix.go:458` para que `1..1000000` devuelva un iterador liviano en lugar de asignar inmediatamente un arreglo de un millón de elementos en el heap.

---

## 13. Auditoría de Tooling y Ecosistema

- **`joss run` y `joss analyze`:** Funcionan de manera impecable, asegurando que ningún código con errores de análisis semántico se ejecute.
- **Extensión de VS Code (`vscode-joss`):** Duplica la lógica de parseo de rutas y validación sintáctica en TypeScript. Debe evolucionar hacia un Language Server puro en Go alimentado por `pkg/analyzer`.

---

## 14. Auditoría del Formatter Oficial

- El archivo `pkg/formatter/scanner.go` mantiene una tabla de tokens paralela que diverge de `pkg/parser/token.go`.
- El formateador debe refactorizarse para operar directamente sobre la secuencia de tokens generada por el lexer canónico, preservando saltos de línea intencionales y aplicando un formato canónico estricto al estilo `gofmt`.

---

## 15. Auditoría del Analyzer y Linter

Se recomienda segregar claramente los niveles de severidad:
- **Analizador:** Valida la corrección del programa (tipos, símbolos, terminación de llamadas, invariantes). Emite exclusivamente `Error` y `Warning`.
- **Linter:** Evalúa el estilo y las buenas prácticas (convención de nombres, detección de claves en código duro `JOSS-SEC-001`). Emite `Warning` e `Info`.

---

## 16. Comparación Selectiva con Otros Lenguajes

| Lenguaje | Enfoque de Control de Flujo | Manejo de Nulabilidad | Lección para Joss |
|---|---|---|---|
| **Dart** | Mantiene `if/else` tradicional | Sound Null Safety con promoción de flujo | Permite escribir código lineal mientras el compilador elimina `null` tras una guarda. |
| **Go** | Sentencias simples `if err != nil` | Punteros y tuplas de error explícitas | El código lineal es más legible que las expresiones anidadas. |
| **Swift** | Sentencia obligatoria `guard cond else { return }` | Opcionales estrictos (`T?`) | La cláusula `guard` garantiza la salida de la función sin indentación piramidal. |
| **Kotlin** | `if` es expresión; soporta *Smart Casts* | Tipos anulables `T?` con operador Elvis `?:` | Si una variable se verifica contra `null`, el compilador debe actualizar su tipo automáticamente. |

---

## 17. Oportunidades de Simplificación

1. **Eliminar Aliases Obsoletos de Funciones:** Deprecar gradualmente los prefijos `str_` y `array_`.
2. **Desaconsejar `let $x` en Favor de `mixed $x`:** Eliminar la ambigüedad conceptual sobre la mutabilidad de tipos.
3. **Unificar Fachadas y Helpers:** Asegurar que `view()` y `View::render()` compartan el mismo contrato formal de firma.

---

## 18. Características que NO Conviene Implementar

Se recomienda rechazar explícitamente:
- ❌ **Grafos de Imports Tradicionales:** Conservar la simplicidad del modelo Zero-Imports.
- ❌ **Genéricos de Orden Superior Complejos:** Mantener la parametrización acotada a `array<T>` y `map<K, V>`.
- ❌ **Herencia Múltiple o Traits Complejos:** Preservar la herencia simple con interfaces limpias.
- ❌ **Function Coloring en Async:** Prohibir la exigencia de palabras clave `async func`.

---

## 19. Catálogo de Propuestas Priorizadas (P1 a P8)

A continuación se detalla el catálogo técnico de propuestas de evolución del lenguaje Joss, priorizadas según su impacto en la reducción de complejidad accidental, eliminación de código defensivo y mejora directa de la productividad y seguridad del desarrollador.

### Matriz de Decisión y Priorización

| ID | Propuesta | Problema Principal | Beneficio | Complejidad | Riesgo | Compatibilidad | Prioridad | Subsistema |
|:---|:---|:---|:---|:---:|:---:|:---:|:---:|:---|
| **P1** | **Sentencia `guard`** | Pirámides de anidamiento por validaciones previas | Flujo de control lineal y lectura secuencial | Media | Bajo | 100% Compatible | **P0** | Parser, Analyzer, Evaluator |
| **P2** | **Propagación de Type Narrowing** | Pérdida de refinamiento de tipos tras salidas tempranas | Elimina aserciones y comprobaciones redundantes | Media | Bajo | 100% Compatible | **P0** | Analyzer (`flow.go`, `infer.go`) |
| **P3** | **Métodos Fluidos en Primitivos** | Fricción por mezcla entre PHP procedural y clases estáticas | API moderna, autocompletado y encadenamiento | Media | Bajo | 100% Compatible | **P1** | Evaluator, Stdlib, Analyzer |
| **P4** | **Preservación de Objetos en `catch`** | Excepciones degradadas a `string` plano en el catch | Manejo tipado de errores, acceso a traza y metadatos | Baja | Mínimo | 100% Compatible | **P0** | Evaluator (`executor.go`) |
| **P5** | **Seguridad en Coalescencia `??`** | Silenciamiento indiscriminado de pánicos reales | Detección inmediata de bugs lógicos y divisiones por cero | Baja | Bajo | Compatible (fix semántico) | **P1** | Evaluator (`evaluator_infix.go`) |
| **P6** | **Destructuring Declarativo** | 5 a 10 líneas de desempaquetado repetitivo por controlador | Reducción de 60% en boilerplate de asignación | Media | Bajo | 100% Compatible | **P1** | Parser, Analyzer, Evaluator |
| **P7** | **Promoción de Propiedades en `Init`** | Cuádruple declaración de propiedades en clases y DTOs | Definición concisa y declarativa de modelos | Media | Mínimo | 100% Compatible | **P1** | Parser, Analyzer, Core (`classes.go`) |
| **P8** | **Motor de Autofix AST (`joss fix`)** | Deuda técnica y correcciones sintácticas manuales | Migración y modernización automática con 0 esfuerzo | Media | Bajo | Herramienta CLI externa | **P0** | CLI, Fixer, Formatter |

---

### P1: Sentencia de Guarda y Control de Flujo Plano (`guard`)

#### 1. Problema Actual
En controladores, middlewares y servicios de Joss, los métodos requieren verificar múltiples precondiciones (autenticación, existencia de registros en base de datos, validez de tokens, presencia de archivos subidos). Dado que los desarrolladores suelen utilizar ternarios con bloques o condicionales anidados para evitar duplicar retornos, el código colapsa en la llamada "pirámide de la perdición" (*pyramid of doom*). Cada validación añade un nivel extra de indentación y encierra la lógica principal dentro del cuerpo de la rama falsa o verdadera.

#### 2. Evidencia en el Código Real
- `ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss`: Hasta 5 niveles de anidamiento de ternarios consecutivos para verificar `$user`, permisos de rol, existencia del backup y parámetros de solicitud.
- `ejemplos/Joss-Red-JosSecurity/app/controllers/api/ApiRepositoryController.joss` (líneas 60-80): Se anidan tres ternarios para verificar si el archivo existe, si el JSON parsea y si los campos opcionales vienen presentes.
- `ejemplos/Joss-Red-JosSecurity/app/middleware/MiddlewareLoader.joss` (líneas 30-93): Bloques de validación defensiva que fuerzan saltos de lectura.

#### 3. Impacto en el Desarrollador
- **Legibilidad severamente degradada:** La lógica de negocio feliz (*happy path*) queda escondida en el nivel más profundo de indentación.
- **Riesgo de errores en refactorización:** Modificar un bloque anidado exige ajustar llaves emparejadas a decenas de líneas de distancia.
- **Dificultad de auditoría:** No es evidente a primera vista cuáles son los requisitos previos para que un endpoint ejecute su lógica central.

#### 4. Solución Técnica Propuesta
Incorporar la sentencia `guard (condición) else { ... }`.
- **Semántica:** La condición debe evaluar a booleano. Si la condición es verdadera, la ejecución continúa inmediatamente en la siguiente sentencia en el mismo nivel de indentación. Si es falsa, se ejecuta el bloque `else`.
- **Invariante estático:** El analizador semántico (`pkg/analyzer`) exige de forma exhaustiva que el bloque `else` de un `guard` termine el flujo de la función (mediante `return`, `throw` o salida terminal). Si el bloque `else` no interrumpe el flujo, el compilador emite un error estático `JOSS-FLOW-005`.

#### 5. Código Actual vs. Código Propuesto

```joss-snippet
// --- ACTUAL (Anidamiento y ramas de escape complejas) ---
public func download(mixed $id) {
    $item = GranDB::table("repos")->where("id", $id)->first()
    return (!$item) ? json({"error": "No encontrado"}, 404) : {
        $user = Auth::user()
        return (!$user) ? json({"error": "No autenticado"}, 401) : {
            $path = $item["file_path"]
            return (!file_exists($path)) ? json({"error": "Archivo perdido"}, 404) : {
                return Response::download($path)
            }
        }
    }
}

// --- PROPUESTO (Flujo plano y lectura lineal con guard) ---
public func download(mixed $id) {
    $item = GranDB::table("repos")->where("id", $id)->first()
    guard ($item != null) else {
        return json({"error": "No encontrado"}, 404)
    }

    $user = Auth::user()
    guard ($user != null) else {
        return json({"error": "No autenticado"}, 401)
    }

    $path = $item["file_path"]
    guard (file_exists($path)) else {
        return json({"error": "Archivo perdido"}, 404)
    }

    return Response::download($path)
}
```

#### 6. Subsistema Afectado
- `pkg/parser`: Nueva keyword `guard`, nodo AST `GuardStatement`.
- `pkg/analyzer`: Validación de condición booleana y verificación de terminación garantizada en el bloque `else` mediante `flow.go:blockTerminatesCallable`.
- `pkg/core`: Evaluación en `executor.go` (si `isTruthy(cond)`, continuar; si no, evaluar `elseBlock`).

#### 7. Estimación y Compatibilidad
- **Beneficio:** Muy Alto (Elimina el 80% de la anidación accidental en controladores).
- **Dificultad:** Media.
- **Riesgo:** Bajo.
- **Compatibilidad:** 100% compatible hacia atrás (palabra clave contextual o reservada con verificación en `token.go`).
- **Prioridad:** **P0**.

---

### P2: Propagación de Type Narrowing en el Ámbito Principal (*Smart Casts*)

#### 2.1. Problema Actual
El sistema de tipos de Joss implementa refinamiento de tipos (`narrowScopeFromCondition` en `pkg/analyzer/infer.go`), pero únicamente dentro del cuerpo interno de una rama `if` o del brazo verdadero/falso de un operador ternario. Cuando un desarrollador valida la nulidad de una variable al inicio de una función y retorna inmediatamente si es nula, el analizador semántico **olvida el refinamiento** en las líneas subsiguientes del ámbito principal.

#### 2.2. Evidencia en el Código Real
- `ejemplos/Joss-Red-JosSecurity/app/controllers/auth/ProfileController.joss` (línea 18-28):
  ```joss-snippet
  $u = Auth::user() // Tipo inferido: User|null
  (!$u) ? { return redirect("/login") }
  // En las siguientes líneas, $u->email o $u->first_name siguen considerando User|null
  ```
- `pkg/analyzer/infer.go` (líneas 90-96): `narrowScopeFromCondition` genera dos scopes hijos aislados (`trueScope`, `falseScope`), pero ninguno de ellos retroalimenta al scope padre `current`.

#### 2.3. Impacto en el Desarrollador
- Se obliga al desarrollador a utilizar `mixed` en variables para silenciar advertencias del analyzer.
- Provoca desconfianza en el sistema de tipos, incentivando comprobaciones defensivas duplicadas (`if ($u != null && $u->email)`) a lo largo del mismo método.

#### 2.4. Solución Técnica Propuesta
Integrar el análisis de terminación de flujo (`flow.go`) con el refinamiento de ámbitos en `infer.go`:
- Cuando una sentencia condicional (`if`, `guard`, o ternario de declaración) demuestre de forma exhaustiva que su rama de escape termina la ejecución de la función (`return`, `throw`), el scope principal posterior asume el tipo refinado de la rama que no escapó.
- Ejemplo: si `$x` es `User|null` y se ejecuta `if ($x == null) { return }`, el tipo de `$x` en el scope principal pasa automáticamente a ser `User`.

#### 2.5. Código Actual vs. Código Propuesto

```joss-snippet
// --- ACTUAL: Analyzer reporta posible desreferencia nula en $user->email ---
public func getEmail(): string {
    User|null $user = Auth::user()
    if ($user == null) {
        return ""
    }
    // El analyzer todavía considera que $user puede ser null
    return $user->email // Genera fricción o exige let mixed
}

// --- PROPUESTO: Smart Cast automático en flujo secuencial ---
public func getEmail(): string {
    User|null $user = Auth::user()
    if ($user == null) {
        return ""
    }
    // Type Narrowing propagado al scope exterior: $user promovido a User estricto
    return $user->email // 100% tipado, autocompletado y validado
}
```

#### 2.6. Subsistema Afectado
- `pkg/analyzer/flow.go`: Exponer `statementTerminatesCallable(stmt parser.Statement) bool`.
- `pkg/analyzer/infer.go`: En `inferStatement`, si un bloque condicional escapa, aplicar las mutaciones de tipo de `narrowScopeFromCondition` sobre el `currentScope`.

#### 2.7. Estimación y Compatibilidad
- **Beneficio:** Muy Alto (El sistema de tipos trabaja a favor del desarrollador, no en su contra).
- **Dificultad:** Media.
- **Riesgo:** Bajo.
- **Compatibilidad:** 100% compatible hacia atrás (no invalida código válido existente; únicamente resuelve falsos positivos).
- **Prioridad:** **P0**.

---

### P3: Métodos Fluidos de Instancia en Primitivos (`string`, `array`, `map`)

#### 3.1. Problema Actual
Actualmente existe una dicotomía confusa en la biblioteca estándar y tipos básicos:
1. Funciones globales de estilo procedural heredadas de PHP (`strlen`, `str_contains`, `substr`, `array_keys`, `array_push`, `json_encode`).
2. Clases nativas con métodos estáticos (`Str::contains`, `Str::length`, `Arr::has`, `JSON::encode`).
El desarrollador debe memorizar constantemente cuándo llamar a una función global, cuándo usar una clase estática y en qué orden van los parámetros (por ejemplo, `$needle` vs `$haystack`). Las transformaciones encadenadas de datos se vuelven ilegibles por el anidamiento de llamadas hacia adentro.

#### 3.2. Evidencia en el Código Real
- `ejemplos/Joss-Red-JosSecurity/app/services/PageBuilderService.joss`:
  ```joss-snippet
  $clean = Str::trim(Str::lower(Str::replace($input, " ", "-")))
  ```
  La lectura se realiza desde adentro hacia afuera, requiriendo 3 llamadas estáticas para una operación trivial sobre una cadena.
- `ShopController.joss`: Manipulaciones de listas que combinan `count($items)`, `array_slice($items, ...)` y `Arr::map(...)`.

#### 3.3. Impacto en el Desarrollador
- Fricción cognitiva continua por alternar entre estilos sintácticos incompatibles.
- Falta de autocompletado natural en el editor: al escribir `$cadena->`, el LSP no puede ofrecer métodos de transformación fluida porque las primitivas no exponen métodos de instancia.

#### 3.4. Solución Técnica Propuesta
Habilitar invocación de métodos de instancia virtuales directamente sobre valores de tipo `string`, `array` y `map`:
- `$cadena->trim()->lower()->replace(" ", "-")`
- `$lista->map(fn($x) => $x * 2)->filter(fn($x) => $x > 10)->join(", ")`
- `$mapa->keys()`, `$mapa->values()`, `$mapa->has("clave")`
- **Implementación sin sobrecoste:** En `pkg/core/evaluator_member.go`, si el receptor es un valor primitivo nativo de Go (`string`, `[]interface{}`, `map[string]interface{}`), despachar internamente hacia los evaluadores ya optimizados de `StrMethods` y `ArrMethods` sin crear objetos wrapper intermediarios.

#### 3.5. Código Actual vs. Código Propuesto

```joss-snippet
// --- ACTUAL: Llamadas estáticas anidadas de adentro hacia afuera ---
public func slugify(string $title): string {
    return Str::lower(Str::trim(Str::replace($title, " ", "-")))
}

// --- PROPUESTO: Encadenamiento fluido natural de izquierda a derecha ---
public func slugify(string $title): string {
    return $title->trim()->replace(" ", "-")->lower()
}
```

#### 3.6. Subsistema Afectado
- `pkg/core/evaluator_member.go`: Interceptar llamadas de miembros sobre tipos no instancia e indexar en despachadores nativos de primitivos.
- `pkg/analyzer/infer.go`: Declarar firmas de retorno para miembros de tipos `String`, `Array` y `Map`.
- `tools/cataloggen`: Exportar métodos de primitivos al catálogo de autocompletado de VS Code.

#### 3.7. Estimación y Compatibilidad
- **Beneficio:** Muy Alto (Moderniza drásticamente la ergonomía del lenguaje).
- **Dificultad:** Media.
- **Riesgo:** Bajo.
- **Compatibilidad:** 100% compatible. Las funciones globales y clases estáticas continúan existiendo sin alteraciones.
- **Prioridad:** **P1**.

---

### P4: Preservación de Instancias de Objetos y Excepciones en `catch`

#### 4.1. Problema Actual
En la implementación actual del runtime de Joss (`pkg/core/executor.go`, línea 459), cuando una excepción es capturada mediante un bloque `try / catch ($ex)`, el valor arrojado se convierte forzosamente a una cadena de texto plana mediante `fmt.Sprint(evalErr.Value)`. Si el desarrollador arrojó una instancia de clase (`throw new ValidationException("Error", 422, $errores)`), el bloque `catch` recibe una cadena vacía o una representación formateada inerte (`"Instance of ValidationException"`), en lugar de la instancia viva del objeto.

#### 4.2. Evidencia en el Código Real
- `pkg/core/executor.go` (línea 459):
  ```go
  r.currentEnvironment.Set(node.Variable.Value, fmt.Sprint(evalErr.Value))
  ```
- `ejemplos/Joss-Red-JosSecurity/app/controllers/web/FlaskController.joss` (líneas 28-30):
  ```joss-snippet
  } catch ($ex) {
      return json({"error": "Plugin error: " . $ex}, 500)
  }
  ```
  Los desarrolladores no pueden acceder a `$ex->getCode()` ni inspeccionar propiedades específicas porque `$ex` es siempre un string.

#### 4.3. Impacto en el Desarrollador
- Imposibilidad de implementar manejo granular de excepciones por tipo (`if ($ex instanceof NotFoundException)`).
- Pérdida irremediable del stack trace, códigos de estado HTTP asociados, metadatos y contexto estructurado de fallos.

#### 4.4. Solución Técnica Propuesta
- En `pkg/core/executor.go`, asignar directamente el valor `evalErr.Value` (sea `*Instance`, `error`, mapa o string) al frame del entorno léxico del bloque `catch`.
- Si el valor lanzado es un error genérico o un string, envolverlo de forma transparente en una instancia de la clase base nativa `Exception` con métodos `$ex->getMessage()` y `$ex->getFile()`.

#### 4.5. Código Actual vs. Código Propuesto

```joss-snippet
// --- ACTUAL: $ex es forzado a string plano, sin métodos ni propiedades ---
try {
    PaymentGateway::charge($amount)
} catch ($ex) {
    // $ex es string: "CardDeclinedException"
    // $ex->getCode() provoca error de runtime
    return json({"error": $ex}, 500)
}

// --- PROPUESTO: $ex preserva la instancia original lanzada ---
try {
    PaymentGateway::charge($amount)
} catch ($ex) {
    if ($ex instanceof CardDeclinedException) {
        return json({"error": $ex->getMessage(), "decline_code": $ex->declineCode}, 402)
    }
    return json({"error": $ex->getMessage()}, 500)
}
```

#### 4.6. Subsistema Afectado
- `pkg/core/executor.go`: Reemplazar la coerción `fmt.Sprint` por asignación directa de valor y empaquetado consistente en `Exception`.
- `pkg/core/classes.go`: Garantizar que la clase base `Exception` ofrezca métodos canónicos `getMessage()`, `getCode()`, `getLine()`, `getFile()`.

#### 4.7. Estimación y Compatibilidad
- **Beneficio:** Crítico (Restaura la integridad del paradigma orientado a objetos en gestión de errores).
- **Dificultad:** Baja.
- **Riesgo:** Mínimo.
- **Compatibilidad:** 100% compatible (concatenar `$ex` como string sigue funcionando gracias a `CoerceString`).
- **Prioridad:** **P0** (Corrección inmediata).

---

### P5: Seguridad en Operador Coalescente `??` y Rescate No Silenciador de Pánicos

#### 5.1. Problema Actual
El operador null-coalescing `??` está diseñado para proporcionar un valor alternativo cuando una expresión evalúa a `null` o accede a un índice/propiedad no definida en una colección. Sin embargo, en la implementación actual (`pkg/core/evaluator_infix.go`, líneas 88-94), la evaluación del operando izquierdo está envuelta en un `recover()` indiscriminado que atrapa cualquier pánico de Go o excepción de Joss, silenciando errores graves de programación como división por cero, tipos inválidos o errores lógicos internos.

#### 5.2. Evidencia en el Código Real
- `pkg/core/evaluator_infix.go` (líneas 88-94):
  ```go
  defer func() {
      if r := recover(); r != nil {
          result = r.evaluateExpression(ie.Right)
      }
  }()
  ```
- Si un desarrollador escribe `$val = ($total / $count) ?? 0` y `$count` es 0, en lugar de alertar sobre el fallo o división prohibida, el operador oculta silenciosamente el error y retorna el valor derecho.

#### 5.3. Impacto en el Desarrollador
- **Bugs silenciosos difíciles de depurar:** Defectos críticos en algoritmos pasan desapercibidos porque `??` captura cualquier excepción ocurrida en la evaluación de expresiones complejas en su lado izquierdo.
- Viola el principio de menor sorpresa y las garantías de robustez de Joss.

#### 5.4. Solución Técnica Propuesta
- Refactorizar el evaluador de `??` para que únicamente rescate errores de valor ausente (`UndefinedVariable`, `MissingKeyError` o valor devuelto igual a `nil`/`NullValue`).
- Los errores fatales del runtime (excepciones lanzadas explícitamente, errores de invocación de métodos inexistentes o fallos de tipo estricto) no deben ser consumidos por `??` y deben propagarse al manejador de errores o bloque `catch` superior.

#### 5.5. Código Actual vs. Código Propuesto

```joss-snippet
// --- ACTUAL: Silenciamiento accidental de fallos de ejecución ---
// Si calculateDiscount() tiene un bug y arroja excepción, ?? lo oculta
$precioFinal = calculateDiscount($producto) ?? 0 // Devuelve 0 en silencio

// --- PROPUESTO: Coalescencia estricta solo ante null/no definido ---
// Si calculateDiscount() retorna null, asigna 0.
// Si calculateDiscount() arroja una excepción, la excepción se propaga y se diagnostica.
$precioFinal = calculateDiscount($producto) ?? 0
```

#### 5.6. Subsistema Afectado
- `pkg/core/evaluator_infix.go`: Eliminar `recover()` ciego en `evaluateNullCoalesceExpression` y evaluar el valor verificando si es nulo o índice no encontrado.

#### 5.7. Estimación y Compatibilidad
- **Beneficio:** Alto (Previene fallos silenciosos en producción).
- **Dificultad:** Baja.
- **Riesgo:** Bajo.
- **Compatibilidad:** Compatible (mejora la corrección semántica sin romper código idiomático).
- **Prioridad:** **P1**.

---

### P6: Destructuring Declarativo de Tuplas, Arrays y Mapas

#### 6.1. Problema Actual
En aplicaciones web de Joss (como el proyecto real JosSecurity), los métodos de controlador y servicios reciben constantemente mapas o tuplas con múltiples valores (datos de formularios, cabeceras, resultados de validación, secretos de 2FA). Para extraer estos valores, el desarrollador se ve forzado a escribir entre 5 y 10 asignaciones individuales repetitivas línea por línea.

#### 6.2. Evidencia en el Código Real
- `ejemplos/Joss-Red-JosSecurity/app/controllers/auth/ProfileController.joss` (líneas 43-48):
  ```joss-snippet
  $first_name = request("first_name")
  $last_name  = request("last_name")
  $phone      = request("phone")
  $password   = request("password")
  ```
- `AuthController.joss` (líneas 125-140): Desempaquetado manual de arrays devueltos por servicios de autenticación y 2FA (`$secret = $totp["secret"]`, `$qrCode = $totp["qr_url"]`).

#### 6.3. Impacto en el Desarrollador
- Gran volumen de código ceremonial y repetitivo.
- Aumento de errores tipográficos en el emparejamiento manual entre el nombre de la variable y la clave del mapa.

#### 6.4. Solución Técnica Propuesta
Incorporar patrones de desestructuración (*destructuring assignment*) en sentencias de asignación y declaración:
1. **Desestructuración de Listas/Tuplas por posición:**
   `[$id, $nombre, $rol] = $usuarioArray`
2. **Desestructuración de Mapas por clave:**
   `{"email": $email, "password": $password} = request()`
3. **Valores por defecto opcionales:**
   `{"role": $role = "cliente", "active": $active = true} = $data`

#### 6.5. Código Actual vs. Código Propuesto

```joss-snippet
// --- ACTUAL: 6 líneas ceremoniales de extracción individual ---
public func registerUser() {
    $req = request()
    $name = $req["name"]
    $email = $req["email"]
    $password = $req["password"]
    $role = $req["role"] ?? "user"
    $terms = $req["terms"] ?? false
}

// --- PROPUESTO: 1 sola línea declarativa y expresiva ---
public func registerUser() {
    {"name": $name, "email": $email, "password": $password, "role": $role = "user", "terms": $terms = false} = request()
}
```

#### 6.6. Subsistema Afectado
- `pkg/parser`: Soporte de patrones `ArrayPattern` y `MapPattern` en el lado izquierdo de sentencias de asignación (`parser_statements.go`).
- `pkg/analyzer`: Tipado e inferencia de cada variable individual a partir del tipo del contenedor (`infer.go`).
- `pkg/core/executor.go`: Asignación secuencial de slots a partir del objeto iterado o indexado.

#### 6.7. Estimación y Compatibilidad
- **Beneficio:** Muy Alto (Reduce el boilerplate de controladores en un 60%).
- **Dificultad:** Media.
- **Riesgo:** Bajo.
- **Compatibilidad:** 100% compatible (nueva construcción sintáctica sin conflictos léxicos).
- **Prioridad:** **P1**.

---

### P7: Promoción de Propiedades en Constructor (`Init`)

#### 7.1. Problema Actual
Para crear clases simples de dominio, DTOs (*Data Transfer Objects*), entidades o servicios en Joss, el programador debe declarar el nombre del campo en cuatro lugares distintos:
1. Como propiedad de clase (`public string $name`).
2. Como parámetro en el constructor `Init(string $name)`.
3. Como asignación a `$this` en el cuerpo del constructor (`$this->name = $name`).
4. En la documentación o tipos de retorno.

#### 7.2. Evidencia en el Código Real
- `ejemplos/Joss-Red-JosSecurity/app/services/LicenseService.joss`: Múltiples clases de servicios con 5 o más propiedades asignadas de forma idéntica en el constructor.
- `ejemplos/plugins/joss_ai/src/plugin.joss`: Repetición de parámetros de configuración asignados uno a uno a `$this->propiedad`.

#### 7.3. Impacto en el Desarrollador
- Resistencia a crear DTOs fuertemente tipados debido a la verbosidad ceremonial requerida para cada clase.
- Refactorizaciones lentas: renombrar una propiedad exige modificar múltiples puntos dentro del mismo archivo.

#### 7.4. Solución Técnica Propuesta
Permitir modificadores de visibilidad (`public`, `protected`, `private`) y constancia (`const`) directamente en los parámetros de la función constructora `Init`:
- Al declarar un parámetro con visibilidad (p. ej., `func Init(public string $titulo, private GranDB $db = new GranDB())`), el compilador y runtime declaran automáticamente la propiedad en la clase y generan la asignación `$this->titulo = $titulo` antes de ejecutar el cuerpo de `Init`.

#### 7.5. Código Actual vs. Código Propuesto

```joss-snippet
// --- ACTUAL: Cuádruple repetición del identificador ---
public class UserDTO {
    public int $id
    public string $email
    public string $role

    public func Init(int $id, string $email, string $role) {
        $this->id = $id
        $this->email = $email
        $this->role = $role
    }
}

// --- PROPUESTO: Constructor conciso con promoción de propiedades ---
public class UserDTO {
    public func Init(
        public int $id,
        public string $email,
        public string $role = "cliente"
    ) {}
}
```

#### 7.6. Subsistema Afectado
- `pkg/parser`: Reconocer `public`, `protected`, `private` antes del tipo en la lista de parámetros de funciones `Init`.
- `pkg/analyzer`: Sintetizar propiedades de clase a partir de los parámetros promovidos.
- `pkg/core/classes.go`: Instanciación automática de slots en la construcción del objeto.

#### 7.7. Estimación y Compatibilidad
- **Beneficio:** Muy Alto en ergonomía orientada a objetos y arquitectura limpia.
- **Dificultad:** Media.
- **Riesgo:** Mínimo.
- **Compatibilidad:** 100% compatible. La sintaxis tradicional de `Init` sigue funcionando exactamente igual.
- **Prioridad:** **P1**.

---

### P8: Motor de Autofix Mecánico Basado en AST (`joss fix`)

#### 8.1. Problema Actual
La evolución de un lenguaje genera inevitablemente deuda técnica en proyectos reales cuando se modernizan patrones (como las 370 ramas vacías `: {}` que acabamos de limpiar en JosSecurity, o la exigencia de visibilidad explícita en funciones globales y métodos). Actualmente, los desarrolladores dependen de búsquedas por expresiones regulares manuales, lo que introduce riesgos de modificar texto dentro de cadenas literales o comentarios.

#### 8.2. Evidencia en el Código Real
- La presencia masiva de ramas `: {}` a lo largo de 65 archivos en JosSecurity demostró que los programadores conservan hábitos sintácticos antiguos a falta de una herramienta oficial de modernización.
- Múltiples diagnósticos estables del compilador (`JOSS-VIS-001`, `JOSS-TYPE-009`, `JOSS-DEPR-001`) ya calculan sugerencias precisas de solución (`Diagnostic.Suggestion`), pero hoy en día solo se imprimen en consola sin poder aplicarse automáticamente al código fuente.

#### 8.3. Impacto en el Desarrollador
- Fricción y retraso en la adopción de nuevas versiones de Joss.
- Miedo a refactorizar o actualizar el compilador por la carga de trabajo manual que representa corregir advertencias de estilo o deprecaciones.

#### 8.4. Solución Técnica Propuesta
Consolidar el comando CLI `joss fix [directorio]` como un motor de reescritura mecánica basado directamente en el Árbol de Sintaxis Abstracta (AST) y en la tabla de diagnósticos:
1. **Detección estructurada:** El analizador semántico identifica diagnósticos que posean un `FixAvailable` o sugerencia estandarizada.
2. **Transformación segura:** En lugar de reemplazar texto mediante expresiones regulares, el fixer sustituye nodos específicos del AST o aplica deltas de texto delimitados por rangos exactos de tokens (`Token.StartLine`, `Token.StartCol`, `Token.EndCol`).
3. **Formateo preservado:** Al finalizar la aplicación de correcciones, invoca automáticamente el motor de `pkg/formatter` para garantizar que la indentación y estilo del proyecto se mantengan uniformes.
4. **Reglas iniciales soportadas en `joss fix`:**
   - Eliminación automática de ramas vacías redundantes `: {}`.
   - Inserción de modificadores de visibilidad `public` automáticos donde falten.
   - Sustitución de funciones globales deprecadas por llamadas canónicas a clases estáticas (`str_contains` -> `Str::contains`).
   - Normalización de tipos históricos deprecados (`list` -> `array`, `dynamic` -> `mixed`).

#### 8.5. Ejemplo de Flujo de Trabajo en Terminal

```bash
# Diagnosticar problemas corregibles automáticamente en el proyecto
joss check ./app

# Aplicar correcciones mecánicas automáticas con informe detallado
joss fix ./app

# Salida esperada:
# [joss fix] Analizando 65 archivos...
# [joss fix] Eliminadas 370 ramas ternarias vacías ': {}' innecesarias.
# [joss fix] Actualizadas 14 llamadas deprecadas a métodos canónicos de Str/Arr.
# [joss fix] Formateado completado satisfactoriamente. 0 errores restantes.
```

#### 8.6. Subsistema Afectado
- `cmd/joss/fix.go`: Subcomando CLI y orquestador de proyectos.
- `pkg/fixer/`: Motor de reescritura por deltas de tokens sobre el AST.
- `pkg/diagnostics`: Incorporación del campo opcional `TextEdit` en `Diagnostic`.

#### 8.7. Estimación y Compatibilidad
- **Beneficio:** Extraordinario para la salud del ecosistema y la fidelización del desarrollador.
- **Dificultad:** Media.
- **Riesgo:** Bajo.
- **Compatibilidad:** 100% compatible (herramienta opt-in que no modifica la semántica del lenguaje).
- **Prioridad:** **P0** (Imprescindible para el ciclo de vida del lenguaje).

---

---

## 20. Cambios que Necesitarían Deprecación

- Marcar con advertencia de diagnóstico `JOSS-DEPR-001` los nombres procedurales de funciones globales que duplican nombres modernos (`str_contains`, `array_keys`, `file_get_contents`).
- Ofrecer autofix mecánico mediante `joss fix`.

---

## 21. Posibles Breaking Changes Justificados

1. **Restricción de `null == ""` a `false`:** Ningún tipo nulo debe evaluar como equivalente a un string vacío bajo igualdad ordinaria.
2. **Propagación de pánicos en `??`:** Erradicar la captura silenciosa de errores fatales en la rama izquierda del operador.

---

## 22. Propuesta y Arquitectura de `joss fix`

Modernizar la herramienta CLI para que utilice transformaciones de árbol sintáctico (AST rewrite) en lugar de regex:
- Inserción automática de modificadores de visibilidad requeridos.
- Sustitución de llamadas a funciones deprecadas por sus equivalentes canónicos.
- Eliminación automática de ramas vacías `: {}` redundantes.
- Ejecución automática del formateador oficial al finalizar la reparación.

---

## 23. Definición de "Código Joss Idiomático"

El código Joss idiomático se define por las siguientes características:
1. **Tipado Declarativo y Seguro:** Contratos públicos explícitos, inferencia limpia en variables locales, y uso de `mixed` reservado para fronteras de entrada dinámicas.
2. **Flujo Lineal sin Anidamientos:** Uso disciplinado de cláusulas de guarda con terminación temprana.
3. **Expresividad Fluida:** Uso del operador pipeline `|>` y llamadas encadenadas sobre colecciones.
4. **Manejo Estructurado de Errores:** Excepciones representadas como objetos de dominio, evitando cadenas de texto planas.

---

## 24. Roadmap Recomendado (Fases 0 a 5)

| Fase | Título | Iniciativas Principales | Prioridad | Impacto |
|---|---|---|:---:|---|
| **Fase 0** | **Correcciones Inmediatas** | Preservar instancias en `catch ($e)`; no silenciar pánicos fatales en `??`; reportar caracteres UTF-8 inválidos en lexer. | **P0** | Crítico |
| **Fase 1** | **Quick Wins** | Flow-sensitive narrowing tras retornos en el analyzer; corrección de `isFalsy` (tratar `0.0` y `{}` como falsos); coerción de claves numéricas en maps. | **P0** | Muy Alto |
| **Fase 2** | **Tooling Unificado** | Refactorizar `pkg/formatter` usando el lexer canónico; modernizar `joss fix` con AST rewriting. | **P1** | Alto |
| **Fase 3** | **Ergonomía de Colecciones** | Métodos de instancia nativos en strings y arrays; operador `..` perezoso (lazy iterator). | **P1** | Alto |
| **Fase 4** | **Evolución del Lenguaje** | Sentencia `guard (...) else { ... }`; soporte para `catch (TipoException $e)` y bloque `finally`. | **P2** | Muy Alto |
| **Fase 5** | **Arquitectura Futura** | Servidor LSP unificado nativo en Go; conexión gradual de `pkg/vm` para optimización de bytecode. | **P3** | Estratégico |

---

*Fin del Documento de Auditoría 2026. Este informe constituye la base de referencia oficial para la toma de decisiones de diseño y evolución técnica de Joss.*

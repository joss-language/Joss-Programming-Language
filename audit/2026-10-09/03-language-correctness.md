# 03. Corrección del Lenguaje, Sistema de Tipos, Semántica y Runtime

**Proyecto:** Joss Programming Language  
**Componentes Evaluados:** `pkg/parser`, `pkg/typesystem`, `pkg/analyzer`, `pkg/core`  
**Metodología:** Batería experimental de 32 programas controlados ejecutados contra `joss analyze` y `joss run`.  

---

## 1. Diseño y Sintaxis del Lenguaje

Joss presenta una gramática singular y altamente consistente orientada a la minimización de ambigüedades:
- **Identificadores de variables:** Obligatoriedad de prefijo `$` (`var $x = 10`).
- **Supresión de sentencias de control imperativas tradicionales:**  
  La prueba `p10_missing_ret` y `p11_dead_branch` arrojaron que la palabra clave `if` fue deliberadamente eliminada de la gramática. Los condicionales se expresan mediante ternarios funcionales con soporte de bloques:
  `($condicion) ? { return $a } : { return $b }` o mediante `match ($val) { ... }`.
- **Eliminación de bucles tipo `for` clásico:**  
  La prueba `p26_loop` confirmó que el bucle `for` tradicional estilo C no existe en el lenguaje; el analizador rechaza la sintaxis con el diagnóstico claro:
  `error[JOSS-PARSE-001]: El bucle 'for' no existe en Joss. Utiliza 'foreach ($array as $item)' o 'while ($cond) { ... }' en su lugar.`
- **Punto y coma opcional:** Delimitación por salto de línea mediante tratamiento canónico de tokens en el parser Pratt.

---

## 2. Robustez del Analizador Semántico y Sistema de Tipos

El pipeline del analizador estático (`pkg/analyzer`) demostró un nivel excepcional de rigor y consistencia diagnóstica, capturando estáticamente todas las siguientes condiciones anómalas:

1. **Variables no declaradas (`JOSS-SYM-001`):** Capturada en `p02_undef_var` y rechazada antes de la ejecución.
2. **Funciones inexistentes (`JOSS-SYM-003`):** Capturada en `p03_undef_func` estáticamente.
3. **Clases inexistentes (`JOSS-SYM-004`):** Capturada en `p04_undef_class` estáticamente.
4. **Aridad incorrecta de llamadas (`JOSS-CALL-001`):** Detectada estáticamente en `p05_argcount`.
5. **Incompatibilidad de tipos de argumentos (`JOSS-TYPE-003`):** Detectada en `p06_argtype`.
6. **Incompatibilidad de retorno (`JOSS-TYPE-008`):** Detectada en `p07_ret`.
7. **Reasignación con cambio de tipo no autorizado (`JOSS-TYPE-001`):** Detectada en `p08_reassign`.
8. **Mutación de constantes (`JOSS-SYM-006`):** Rechazada estáticamente en `p09_const`.
9. **Operaciones mixtas `int + float` sin coerción explícita (`JOSS-ARITH-003`):**  
   El analizador bloquea sumas entre enteros y flotantes para evitar pérdida oculta de precisión.
10. **Doble declaración de funciones (`JOSS-DECL-001`):** Detectada en `p21_dup`.
11. **Parámetros sin tipar (`JOSS-TYPE-011`):** Exige tipado explícito o `mixed` explícito en `p25_untyped_param`.

---

## 3. Comportamiento en Runtime y Detección de Anomalías

Durante la ejecución dinámica (`joss run`), el runtime (`pkg/core`) exhibe las siguientes protecciones comprobadas:

- **División entre cero:** Interceptada con código estructurado `Error[JOSS-ARITH-002]: División entre cero`.
- **Desbordamiento entero (Integer Overflow):** Interceptado con código `Error[JOSS-ARITH-001]: Overflow entero en 9223372036854775807 + 1`.
- **Acceso fuera de límites (Index Out of Bounds):** Interceptado con `Error[JOSS-INDEX-001]: Índice 10 fuera de rango`.
- **Límite de recursión:** Respeta `Runtime.MaxCallDepth = 1024` emitiendo stack trace limpio sin colapsar el proceso Go con desbordamiento de pila del sistema operativo.
- **Manejo de excepciones:** Bloques `try ... catch ($e)` funcionales tanto para strings como para instancias de excepción.
- **Cierres léxicos (Closures):** Captura correcta de variables del entorno circundante (`p20_closure`).

---

## 4. Defectos y Desviaciones Semánticas Identificadas

1. **Vulnerabilidad del Lexer a Cadenas sin Cerrar (HAL-02):**  
   En la prueba `p19_unterminated.joss` (`print("hola)`), el lexer aceptó la cadena no terminada absorbiendo el paréntesis de cierre como parte del literal de texto. El programa no fue rechazado por el parser, ejecutándose e imprimiendo `hola)`. Esto representa una debilidad en el escaneo léxico de caracteres de terminación de línea para strings.
2. **Evaluación de Expresiones de Miembro Inexistentes en Tiempo de Compilación:**  
   En `p12_member.joss` (`$a->zzz`), el analizador estático no emitió error si el tipo no estaba completamente inferido en la frontera, delegando el fallo a tiempo de ejecución: `[Error de Ejecución JOSS] Propiedad o método 'zzz' no encontrado en clase 'A'`.
3. **Ausencia de Timeout en Bucles Infinitos (`while (true)`):**  
   La prueba `p29_infinite.joss` demostró que una goroutine con un bucle infinito se ejecuta indefinidamente sin cuota de instrucciones en el intérprete principal, dependiendo exclusivamente de la cancelación externa del proceso.

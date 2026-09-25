# Evolución de Joss 2026: fases 0 y 1

Este documento registra decisiones ejecutables posteriores a la
[evaluación crítica](JOSS_LANGUAGE_REVIEW_2026.md). El código y las pruebas son
la fuente de verdad.

## 1. Estado inicial

Baseline de `main` en `362c26a`, antes de cambiar semántica:

- `go test ./...`: correcto.
- `go vet ./...`: correcto.
- `go build ./...`: correcto.
- `cataloggen --check` y `docgen --check`: correctos.
- El parser daba mayor precedencia a `%` que a `*`/`/`, y trataba `&&` y `||`
  como equivalentes.
- `??` recuperaba panics de lookup, tipos y otros fallos y los convertía en null.
- `0.0` y `{}` eran verdaderos, aunque `0`, decimal cero y `[]` eran falsos.
- `int → float` y `float → decimal` eran asignables implícitamente.

## 2. Decisiones semánticas

### Precedencia

La tabla canónica vive en `pkg/parser/parser.go` y se publica en
[Sintaxis](SINTAXIS.md). De menor a mayor: asignación, ternario, coalescencia,
pipeline, OR, AND, igualdad, comparación, shifts, rango, suma, producto,
prefijos, llamada y acceso. `*`, `/` y `%` comparten nivel; `&&` supera a `||`.

### Null coalescing

`izquierda ?? derecha` evalúa `derecha` únicamente cuando `izquierda` termina
normalmente con `null`. No captura excepciones, panics ni errores de programa.
La recuperación explícita pertenece a `try`/`catch`.

### Truthiness

Se conserva truthiness porque ya forma parte de ternarios, guard, filtros y
APIs existentes. La tabla es cerrada y se basa en categorías:

| Valor | Falso |
|---|---|
| Ausencia | `null` |
| Booleano | `false` |
| Número | cero en `int`, `float` o `decimal` |
| String | vacío |
| Colección | array o map vacío |

Los demás valores son verdaderos. `"0"` es verdadero por ser no vacío.

### Seguridad numérica

La única promoción implícita entre clases numéricas es `int → decimal`, que es
exacta. `int → float`, `float → decimal` y operaciones mixtas que dependan de
ellas requieren conversión explícita. La división entera, cuyo resultado es
`float`, rechaza operandos que no puedan representarse exactamente. El código
estable es `JOSS-ARITH-003`.

`const` continúa significando binding inmutable. No promete inmutabilidad del
objeto, de una colección ni del grafo alcanzable.

## 3. Compatibilidad y migración

| Cambio | Clase | Migración automática | Acción |
|---|---|---:|---|
| Precedencia convencional | breaking semántico | No en general | Agregar paréntesis para preservar el resultado anterior. |
| `??` sólo para null | breaking y corrección de seguridad | No | Usar `try`/`catch` si la recuperación era intencional. |
| Truthiness uniforme | breaking para `0.0`, `{}` y `"0"` | No | Comparar explícitamente cuando el dominio use otra regla. |
| Conversiones numéricas estrictas | breaking estático | Parcial | Insertar `floatval(...)` o `decimal(...)` sólo con intención revisada. |

No se introduce una rama permanente de semántica legacy. Los proyectos que
dependan de la conducta anterior deben hacer visible su intención mediante
paréntesis, comparaciones o conversiones.

## 4. Bugs bloqueados por regresión

- Agrupación completa de aritmética, lógica, comparación, rango, shifts,
  pipeline, coalescencia y ternario.
- Propagación de división por cero e índice inválido a través de `??`.
- Igualdad conceptual de cero entre `int`, `float` y `decimal`.
- Igualdad conceptual de vacío entre arrays y maps.
- Rechazo estático y defensa runtime de pérdidas de precisión conocidas.

## 5. Fase 2 — contratos e migração de sintaxe

A Fase 2 estabelece peças compatíveis para a próxima versão maior:

- `void` é um tipo canônico. Uma callable `: void` pode usar `return;` ou
  finalizar naturalmente; `return valor` emite `JOSS-TYPE-008`.
- Closures inferem um tipo callable e preservam contratos conhecidos de
  parâmetros e retorno.
- `joss check` informa advertências de migração para `let`, declarações
  implícitas, construtores legados e retorno omitido. `joss fix` migra casos
  inequívocos de `let`, `nil` e `Init constructor`.
- `null` é a ortografia canônica; o fixer não altera literais de texto ou comentários.
- `async` cria um contexto filho cancelável; `Future.Cancel()` propaga o
  cancelamento cooperativo para o runtime forkeado.

`const` permanece imutabilidade do binding, e não imutabilidade profunda.

## 6. Superfícies preparadas para fases posteriores

La siguiente fase consolidará `var`, tipo explícito, `mixed` y `const`, con
deprecación finita de `let` y declaraciones implícitas. Antes de modificar el
parser deberá existir inventario de usos, diagnóstico específico y codemod que
sólo transforme casos semánticamente demostrables.

También quedan inventariados para fases posteriores: retornos obligatorios y
`void`, inferencia de closures, reducción de `unknown`, metadata nativa,
namespaces, constructor canónico, concurrencia estructurada y perfiles de
capacidades. Ninguno altera la FASE 1.

## 7. Riesgos abertos

- `floatval` expresa aceptación deliberada de aproximación; sus callers deben
  decidir si el dominio permite esa pérdida.
- Fuentes externas pueden entregar floats ya aproximados antes de convertirlos
  a decimal. El sufijo `m` o texto decimal evita esa frontera.
- La VM y los plugins tienen evaluadores propios. No se anunciará equivalencia
  hasta que las pruebas diferenciales cubran estas reglas.
- Agregaciones, JSON y drivers DB requieren ampliar pruebas de frontera sin
  alterar silenciosamente contratos externos.

[Índice](README.md)

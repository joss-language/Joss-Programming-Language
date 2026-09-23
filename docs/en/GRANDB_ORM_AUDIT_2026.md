# Auditoría y evolución del ORM GranDB — 2026

[Índice](README.md) · [GranDB actual](MODELOS.md) · [Schema Builder](SCHEMA_BUILDER.md)

## 1. Resumen ejecutivo

GranDB es actualmente un **query builder integrado al runtime de Joss**, no un ORM de registros activos comparable todavía con Eloquent. Construye SQL sobre un `Instance` mutable, ejecuta mediante `database/sql` y devuelve mapas o arrays de mapas. No existe una capa implementada de `Model`, `Hydrator`, relaciones, identidad, dirty tracking, casts de modelo, eager loading ni colecciones ORM.

La base existente sí es útil: ofrece bindings para valores, consultas encadenadas, agregados SQL, transacciones ligadas al runtime, conexiones para cuatro familias de motores, Schema Builder y un catálogo nativo visible al analyzer y al editor. Sin embargo, las diferencias de dialecto están repartidas entre builder, drivers y schema; partes estructurales de SQL aceptan texto sin validar; el estado mutable dificulta reutilización y concurrencia; y el catálogo anuncia operaciones que no llegan correctamente al dispatcher.

La evolución recomendada conserva el API existente, corrige primero los defectos demostrados y crea tres fronteras pequeñas:

```text
API Joss / Model metadata
          ↓
Query + Relation specs inmutables
          ↓
Dialect compiler (SQLite/MySQL/PostgreSQL/SQL Server)
          ↓
database/sql + transacción del Runtime
          ↓
Rows → Hydrator → Model / colección nativa
```

No se recomienda empezar por relaciones polimórficas, eventos mágicos ni accessors implícitos. Esas funciones dependen de contratos de modelo, claves, hidratación, casts y consultas confiables que hoy no existen.

### Evaluación general

| Área | Estado comprobado | Riesgo |
|---|---|---|
| Bindings de valores | Se usan `?` y adaptadores de placeholders | Medio |
| Identificadores y operadores | Texto estructural insuficientemente validado | Crítico |
| Builder | Amplio, mutable y almacenado en `Instance.Fields` | Alto |
| Dialectos | Adaptación parcial y condicionales dispersos | Alto |
| Lecturas y agregados | Funcionales principalmente en SQLite | Medio |
| Escrituras | CRUD básico; `upsert` anunciado sin ejecución | Alto |
| Transacciones | Callback con commit/rollback; anidación rechazada | Medio |
| Modelos e hidratación | No existen como capa ORM | Oportunidad |
| Relaciones y eager loading | No implementados | Oportunidad |
| Observabilidad | `fmt.Printf` incondicional con bindings | Alto |
| Pruebas cross database | No existe suite ejecutada en cuatro motores | Alto |

## 2. Arquitectura original comprobada

### 2.1 Flujo de ejecución

```text
Fuente Joss
  → parser / AST
  → analyzer consulta catálogo de GranDB
  → evaluator resuelve método nativo
  → Runtime.executeGranDBMethod
  → Instance.Fields (_table, _wheres, _bindings, ...)
  → buildSelectQuery o helper de escritura
  → databaseExecutor (*sql.DB o *sql.Tx)
  → wrapper de driver (rebind PostgreSQL / SQL Server)
  → database/sql
  → rowsToMap
  → []map[string]interface{} / map / escalar
```

El analyzer conoce nombres y algunos retornos mediante `native_signatures.go`; no conoce la consulta, columnas, schema ni tipos de cada fila. La validación de SQL ocurre al ejecutar en el motor.

### 2.2 Responsabilidades reales

| Componente | Responsabilidad actual |
|---|---|
| `pkg/core/native.go` | Publica nombres de métodos nativos de GranDB. |
| `pkg/core/native_signatures.go` | Proyecta tipos de retorno parciales al analyzer/LSP. |
| `pkg/core/database.go` | Dispatcher, estado del builder y compilación principal de SELECT. |
| `database_read.go` | Terminales de lectura, paginación offset, chunk y agregados. |
| `database_insert.go` | Insert de un mapa e ID insertado. |
| `database_update.go` | Update, updateOrInsert, increment/decrement y timestamps. |
| `database_delete.go` | Delete protegido, deleteAll y truncate. |
| `database_helpers.go` | Ejecutor contextual, conversión de filas, quoting y utilidades. |
| `database_postgres.go` | Apertura de SQLite/MySQL/PostgreSQL y rebind PostgreSQL. |
| `database_sqlserver.go` | Apertura y rebind SQL Server. |
| `schema*.go` | DDL, Blueprint y adaptación parcial por motor. |
| `migration_manager.go` | Registro y eliminación de tablas para migraciones. |
| `runtime.go` | Conexión perezosa, cambio de conexión y configuración fija del pool. |

### 2.3 Dependencias y acoplamiento

`Instance.Fields` mezcla representación pública de objetos Joss con estado interno de consultas. Los helpers leen claves privadas por string y realizan aserciones concretas. El SQL se arma antes de que el driver adapte comillas y placeholders. Schema posee validación de identificadores más estricta que el query builder, por lo que hay dos políticas distintas para el mismo problema.

`Runtime` es owner de la transacción activa y referencia la conexión externa. Un builder conserva el runtime implícitamente al ejecutarse. No hay una estructura que pueda clonarse o compilarse independientemente del objeto Joss.

## 3. Inventario del API público actual

### Construcción

- Tabla y proyección: `table`, `select`, `distinct`.
- Predicados: `where`, `orWhere`, `whereNot`, `whereColumn`, `whereLike`, `whereIn`, `whereNotIn`, `whereNull`, `whereNotNull`, `whereBetween` y variantes OR.
- Predicados especializados: `whereDate`, `whereYear`, `whereMonth`, `whereDay`, `whereTime`, `whereJsonContains`.
- Composición: `join`, `innerJoin`, `leftJoin`, `rightJoin`, `crossJoin`, `groupBy`, `having`.
- Orden y ventana: `orderBy`, `latest`, `oldest`, `inRandomOrder`, `limit/take`, `offset/skip`, `forPage`.
- Condicionales: `when`, `unless`.

### Terminales de lectura

| Operación | Retorno runtime | Ausencia | Estado posterior |
|---|---|---|---|
| `get` | array de mapas | array vacío | reinicia lectura |
| `first`, `find`, `firstWhere` | mapa o null | null | reinicia lectura |
| `firstOrFail`, `findOrFail` | mapa | panic | reinicia lectura |
| `sole` | mapa | panic en 0 o >1 | reinicia lectura |
| `findMany` | debería ser array | actualmente puede hacer panic | inconsistente |
| `value` | mixed o null | null | reinicia lectura |
| `pluck` | array o map | vacío | reinicia lectura |
| `exists`, `doesntExist` | bool | false/true | reinicia lectura |
| `count` | int | error runtime | conserva filtros |
| `sum/avg/min/max` | float o null | null | conserva filtros |
| `paginate` | map | data vacío | consume el builder |
| `chunk` | bool | true | usa offset y builder mutable |

### Escrituras y control

- `insert`, `insertGetId`: un mapa; no hay insert masivo.
- `update`: mapa; permite update sin filtro con un warning.
- `updateOrInsert`: SELECT seguido de UPDATE/INSERT, sin atomicidad.
- `upsert`: registrado públicamente, sin case de ejecución comprobable.
- `delete`: exige filtros; `deleteAll` y `truncate` son explícitos.
- `increment`, `decrement`, `touch`.
- `transaction`: callback, commit/rollback, transacciones anidadas rechazadas.
- `toSql`, `getBindings`, `dump`, `dd`.

Los aliases duplican nombres camelCase y minúsculos. Esta compatibilidad debe conservarse, pero el catálogo y el dispatcher deben derivarse de una sola definición para impedir divergencias.

## 4. Hallazgos y bugs

### CRITICAL

#### GDB-SEC-001 — Escape de la política de identificadores

`quoteIdentifier` devuelve el texto sin cambios si contiene espacio o `(`. Además, operadores de `where`, `having` y `join` se interpolan directamente. Un valor correctamente ligado no protege una columna, alias, tabla u operador manipulable.

**Causa raíz:** el builder representa estructura SQL como strings y trata texto complejo como raw implícito.

**Solución:** identificadores segmentados y validados, operadores en allowlist y `RawExpression` explícito. La compatibilidad puede conservar raw previo con warning de deprecación antes de rechazarlo.

#### GDB-SEC-002 — Bindings sensibles impresos siempre

Insert, update y delete escriben SQL y bindings mediante `fmt.Printf`. Tokens, emails y secretos pueden terminar en logs de producción.

**Solución:** logger opcional estructurado, apagado por defecto, redacción configurable y duración medida alrededor del executor.

### HIGH

#### GDB-COR-001 — `findMany` realiza una aserción imposible

El método llama `wherein`, que devuelve el builder, y lo convierte a `[]map[string]interface{}`. La llamada hace panic antes de ejecutar `get`.

#### GDB-COR-002 — Métodos publicados sin ruta válida

`upsert` está en `native.go` pero no existe un case equivalente en el dispatcher. `firstofail` está registrado, mientras el dispatcher reconoce `firstorfail`. El analyzer y el autocompletado pueden aprobar llamadas que fallan en runtime.

#### GDB-COR-003 — `update` masivo accidental

`delete` aborta sin `where`; `update` solo imprime una advertencia y modifica toda la tabla. La diferencia es peligrosa e inesperada.

#### GDB-COR-004 — `updateOrInsert` no es atómico

Dos procesos pueden observar ausencia y ambos insertar. El nombre sugiere una garantía mayor que la implementación. Debe apoyarse en una clave única y upsert de dialecto, o documentarse como operación de conveniencia no atómica.

#### GDB-SQL-001 — Dialectos parciales

- Fecha y JSON usan funciones con semántica MySQL.
- Orden aleatorio distingue SQLite frente a “resto”; PostgreSQL y SQL Server difieren.
- `SELECT * ... LIMIT 0` para detectar timestamps no es SQL Server portable.
- Insert ID, paginación, truncate y quoting contienen ramas dispersas.

#### GDB-CON-001 — Builder mutable compartible

Filtros, bindings, límite y selección viven en mapas y se mutan in place. Reusar el mismo objeto en consultas intercaladas o goroutines mezcla estado. No existe contrato de clone ni ownership de builder.

#### GDB-OBS-001 — Errores inconsistentes

Unas rutas hacen panic, otras retornan false y otras imprimen y continúan. No existe un error tipado que diferencie not found, constraint, conexión, timeout y compilación inválida.

### MEDIUM

- **GDB-PERF-001:** insert/update consultan columnas físicas para timestamps en cada operación.
- **GDB-COR-005:** los mapas generan orden de columnas no determinista, dificultando cache de statements, snapshots y pruebas.
- **GDB-COR-006:** valores map en insert se omiten silenciosamente.
- **GDB-COR-007:** strings como `CURRENT_TIMESTAMP` o prefijos admitidos se interpretan como SQL raw; un dato literal igual cambia de significado.
- **GDB-PERF-002:** `chunk` usa offset sin exigir orden estable; cambios concurrentes pueden duplicar u omitir filas.
- **GDB-API-001:** `pluck` puede devolver array o map, pero su firma publicada solo dice array.
- **GDB-API-002:** timestamps se infieren del schema por convención sin metadata de modelo configurable.
- **GDB-PORT-001:** el default silencioso para drivers desconocidos es MySQL; errores de configuración pueden conectar al motor equivocado.
- **GDB-TEST-001:** PostgreSQL y SQL Server solo prueban transformaciones locales; no hay suite CRUD real contra cuatro motores.

### LOW

- Mensajes mezclan español, prefijos libres y panic sin código estable.
- Comentarios de truncate describen motores de forma incompleta.
- Aliases incrementan superficie sin una política explícita de deprecación.

## 5. Seguridad SQL

### Fronteras que deben quedar explícitas

| Entrada | Tratamiento correcto |
|---|---|
| Valores | Siempre binding del driver. |
| Tabla/columna/alias | Parser de identificador + quoting de dialecto. |
| Operador | Enum/allowlist canónica. |
| Dirección de orden | `ASC` o `DESC`; ningún otro texto. |
| Función/expresión | `RawExpression` explícita y marcada como trusted. |
| Lista variable | Placeholders generados; nunca concatenar valores. |

Una API raw no puede prometer seguridad. Debe ser visible, deliberada y excluida de mass assignment. Los timestamps del ORM deben representarse como una expresión interna, no como el string de usuario `CURRENT_TIMESTAMP`.

## 6. Dialectos y compatibilidad demostrada

La tabla distingue implementación de verificación. “Parcial” significa que hay ramas de código sin suite de integración contra el motor.

| Función | SQLite | MySQL/MariaDB | PostgreSQL | SQL Server |
|---|---:|---:|---:|---:|
| Apertura/configuración | Probada localmente | Implementada | Implementada | Implementada |
| CRUD básico | Probado | Parcial | Parcial | Parcial |
| Placeholders | `?` probado | `?` por driver | Rebind `$n` unitario | Rebind `@pn` sin suite equivalente |
| Identificadores | Backticks aceptados | Backticks | Rebind a comillas | Rebind a corchetes |
| Limit/offset | Probado parcialmente | Implementado | Implementado parcialmente | Rama TOP/OFFSET |
| Insert ID | LastInsertId | LastInsertId | `RETURNING id` | `OUTPUT`/`SCOPE_IDENTITY` |
| Upsert | No implementado | No implementado | No implementado | No implementado |
| JSON predicates | No portable | Sintaxis MySQL | Incorrecta/no probada | Incorrecta/no probada |
| Date parts | Parcial | Sintaxis MySQL | Incorrecta/no probada | Incorrecta/no probada |
| Transacción callback | Probada | No integrada | No integrada | No integrada |
| Savepoints | No | No | No | No |
| Schema Builder | Pruebas amplias | Parcial | Parcial | Parcial |

No debe publicarse una matriz con ✓ en motores que CI no ejecuta. La suite común deberá activarse con servicios de CI y permitir skips únicamente con razón declarada.

## 7. Modelos, hidratación y relaciones

### Estado actual

Una clase que hereda `GranDB` sigue produciendo consultas y mapas. No hay metadata canónica de tabla, primary key, fillable, guarded, hidden, casts, timestamps o relaciones. `rowsToMap` escanea valores del driver y no crea instancias Joss.

Por tanto, hoy no existen lazy loading ni N+1 de relaciones del ORM porque tampoco existen relaciones. Una aplicación puede producir N+1 manualmente dentro de un loop, pero GranDB no dispone de metadata para detectarlo o agruparlo.

### Diseño mínimo recomendado

```text
ModelMetadata (inmutable, cache por ClassID)
  table, connection, primaryKey
  fillable/guarded, hidden/visible
  casts, timestamps, softDelete
  relations: map[RelationID]RelationMetadata

ModelState (por instancia)
  attributes
  original
  changed
  loadedRelations
  exists
```

El hydrator recibe metadata y una fila. Aplica casts de lectura, guarda original y marca la instancia como existente sin disparar mutators de escritura. `save` calcula cambios y usa primary key. Mass assignment filtra antes de asignar. Serialización aplica hidden/visible después de accessors explícitos.

### Orden de relaciones

1. `belongsTo`, `hasOne`, `hasMany` con claves explícitas y convenciones documentadas.
2. `belongsToMany` con metadata de pivot y operaciones transaccionales.
3. Eager loading de un nivel mediante dos consultas y agrupación en memoria.
4. Rutas anidadas y constraints.
5. Modo de prevención de lazy loading.
6. Relaciones through o polimórficas solo tras medir demanda y complejidad.

`with("posts")` debe consultar padres, reunir claves no nulas, ejecutar una sola consulta `WHERE IN`, agrupar por foreign key y asignar colección/nullable según cardinalidad. La deduplicación de claves y el límite de parámetros deben pertenecer al dialecto/batcher.

## 8. Arquitectura objetivo

```text
GranDB native facade
  ├── QueryBuilder (API compatible; copia barata)
  └── ModelQuery (metadata de clase)
             │
             ▼
        QuerySpec / MutationSpec
        (nodos estructurados, bindings separados)
             │
             ▼
        Dialect.Compile(spec)
  ┌──────────┼───────────┬────────────┐
 SQLite     MySQL     PostgreSQL   SQL Server
  └──────────┼───────────┴────────────┘
             ▼
       Executor (*sql.DB / *sql.Tx)
             ▼
       RowDecoder / Hydrator
        ├── mapas para API legacy
        └── Model/Collection para ModelQuery
```

Interfaces justificadas:

- `Dialect`: existe sustitución real entre cuatro compiladores.
- `SQLExecutor`: ya existe sustitución entre DB y Tx.
- `QueryObserver`: opcional para logging, métricas y pruebas.

No se justifican managers, factories o repositories internos por cada operación. Metadata, specs y compiladores pueden ser structs y funciones pequeñas.

## 9. Roadmap de implementación

### Fase 0 — Corrección y contrato público (P0)

**Problema:** catálogo/dispatcher divergentes, `findMany` roto, actualizaciones masivas accidentales.

**Solución:** definición nativa única con test de alcanzabilidad; corregir aliases; ejecutar `whereIn(...).get()`; exigir `where` en update y añadir `updateAll` explícito si se necesita.

**Archivos:** `native.go`, `native_signatures.go`, `database.go`, `database_read.go`, `database_update.go`, tests y `MODELOS.md`.

**Compatibilidad:** corrección compatible; protección de update será compatible con warning/deprecation antes del rechazo si existen consumidores demostrados.

**Riesgo:** bajo. **Beneficio:** crítico.

**Tests:** cada método anunciado alcanza handler; aliases equivalentes; findMany vacío/no vacío; update sin filtro no modifica filas.

### Fase 1 — Seguridad estructural y observabilidad (P0)

**Problema:** identificadores/operadores/raw ambiguos y logs sensibles.

**Solución:** `Identifier`, allowlist de operadores/direcciones, `RawExpression`, observer apagado por defecto y redacción.

**Archivos:** nuevo módulo cohesivo en `pkg/core`, builder, writes, schema y metadata nativa.

**Compatibilidad:** expresiones raw implícitas pasan por deprecación; values permanecen compatibles.

**Riesgo:** medio por consultas que dependan de strings raw. **Beneficio:** crítico.

**Tests:** payloads de inyección en tablas/columnas/operadores; raw deliberado; logs apagados/redactados; fuzz del parser de identificadores.

### Fase 2 — QuerySpec y dialectos (P1)

**Problema:** SQL string y ramas de motor dispersas.

**Solución:** representación estructurada gradual; `Dialect` compila select/insert/update/delete/upsert, quoting, pagination, random, date parts, JSON y return ID.

**Compatibilidad:** builder público igual; `toSql` refleja dialecto seleccionado.

**Riesgo:** alto; migrar operación por operación con golden tests. **Beneficio:** alto.

**Tests:** golden SQL/bindings por dialecto y suite de integración común.

### Fase 3 — Escrituras masivas y schema cache (P1)

**Problema:** una fila por insert, timestamps consultan schema repetidamente, upsert ausente.

**Solución:** `insertMany`, batching por límite de parámetros, upsert por dialecto, cache inmutable por conexión/schema y orden estable de columnas.

**Compatibilidad:** aditiva. **Riesgo:** medio. **Beneficio:** alto.

**Tests:** filas heterogéneas rechazadas claramente, rollback, constraints, lotes límite+1 y upsert con clave única.

### Fase 4 — Metadata, hidratación y casts (P1)

**Problema:** ausencia de modelos persistentes.

**Solución:** `ModelMetadata`, `ModelState`, hydrator, casts simétricos, fillable/guarded, hidden/visible y timestamps configurables.

**Compatibilidad:** API legacy continúa devolviendo mapas; consultas iniciadas desde Model devuelven modelos.

**Riesgo:** alto por nueva semántica. **Beneficio:** alto.

**Tests:** round trip de cada cast, null, decimal, datetime/zonas, JSON inválido, mass assignment y serialización.

### Fase 5 — Persistencia de modelo (P1)

**Problema:** no hay create/save/delete ni dirty tracking de instancia.

**Solución:** create seguro, save insert/update, partial updates, original/changes, refresh y primary key configurable.

**Compatibilidad:** aditiva. **Riesgo:** medio. **Beneficio:** alto.

**Tests:** modelo nuevo/existente, sin cambios, PK no id, concurrent update y optimistic locking opt in.

### Fase 6 — Relaciones y eager loading (P1/P2)

**Problema:** relaciones y prevención de N+1 ausentes.

**Solución:** relaciones tradicionales, eager loader por lotes, `with/load/loadMissing`, modo estricto de lazy loading.

**Compatibilidad:** aditiva. **Riesgo:** alto. **Beneficio:** alto.

**Tests:** grafo User/Profile/Posts/Comments/Roles, relaciones vacías, null, claves personalizadas, nested eager load y conteo exacto de queries.

### Fase 7 — Pivot y operaciones relacionales (P2)

**Solución:** `attach/detach/sync/toggle/associate/dissociate`, atributos pivot y transacciones automáticas para operaciones compuestas.

**Tests:** duplicados, rollback parcial, claves compuestas y concurrencia.

### Fase 8 — Scopes, soft deletes y colecciones (P2)

Scopes son specs componibles. Soft delete es un global scope explícitamente removible. Una colección ORM solo añade operaciones dependientes de modelos; map/filter/reduce reutilizan colecciones del lenguaje.

### Fase 9 — Paginación, cursor y streaming (P2)

Añadir `simplePaginate`, cursor estable compuesto y `chunkById`. `stream` debe documentar lifetime de rows, cancelación y prohibición de escape tras cierre.

### Fase 10 — Tooling, diccionario y diagnósticos (P1 continuo)

Toda API implementada se añade a metadata declarativa, catálogo generado, analyzer, LSP y docs en el mismo cambio. Errores reciben códigos GranDB estables y categorías: compile, not found, constraint, connection, transaction y cancellation.

### Fase 11 — Benchmarks y gates cross database (P1 continuo)

Benchmarks: compile simple/complex, select, 100/1000 row decode e hydration, insert/bulk/upsert, eager one-to-many, many-to-many y serialization. Reportar `ns/op`, `B/op`, allocs y queries. CI usa contenedores para los tres servidores; SQLite sigue local.

## 10. Matriz de recomendaciones

Las clasificaciones derivan de fallos observados, dependencia de otras fases y superficie de cambio.

| Propuesta | Seguridad | Estabilidad | Rendimiento | DX | Complejidad | Prioridad | Justificación |
|---|---:|---:|---:|---:|---:|---:|---|
| Paridad catálogo/handler | Media | Alta | Baja | Alta | Baja | P0 | Evita llamadas aprobadas que fallan en runtime. |
| Identificadores y operadores seguros | Alta | Alta | Baja | Media | Media | P0 | Cierra una frontera de inyección estructural. |
| Logging opt in y redacción | Alta | Alta | Baja | Alta | Baja | P0 | Evita exposición de bindings y permite medir. |
| QuerySpec + dialectos | Alta | Alta | Media | Alta | Alta | P1 | Resuelve divergencias que hoy están dispersas. |
| Bulk/upsert | Media | Alta | Alta | Alta | Media | P1 | Reduce round trips y carreras con constraints. |
| Metadata/hydrator/casts | Alta | Alta | Media | Alta | Alta | P1 | Base requerida por todo el ORM real. |
| Relaciones + eager loading | Media | Alta | Alta | Alta | Alta | P1 | Resuelve N+1 y consistencia relacional. |
| Dirty tracking/partial update | Media | Alta | Media | Alta | Media | P2 | Evita writes y habilita locking opt in. |
| Cursor/chunkById | Baja | Alta | Alta | Alta | Media | P2 | Escala datasets sin offset inestable. |
| Polimorfismo relacional | Baja | Media | Media | Media | Alta | Experimental | No resuelve una carencia base antes de relaciones normales. |
| Eventos mágicos/accessors implícitos | Baja | Baja | Baja | Media | Alta | No recomendado ahora | Dificultan depuración antes de estabilizar modelos. |

## 11. Mapeo conceptual Eloquent → GranDB

| Concepto Eloquent | Estado/objetivo GranDB |
|---|---|
| Query Builder | Existe, requiere seguridad y dialectos. |
| `get/first/find` | Existen y retornan mapas en API legacy. |
| `firstOrFail/findOrFail` | Existen con panic; requieren error tipado. |
| Model | Objetivo basado en metadata explícita. |
| Collection | Reutilizar array/colecciones Joss, añadir solo semántica de modelo. |
| `fillable/guarded` | Objetivo obligatorio antes de `Model.create`. |
| casts | Objetivo simétrico de lectura/escritura. |
| relations | No existen; implementar primero cuatro tradicionales. |
| `with/load` | Dependen de relaciones e hydrator. |
| scopes | Specs componibles, no reflexión por nombre. |
| soft deletes | Global scope opt in con escape explícito. |
| events | Postergar hasta definir transacciones y orden observable. |

## 12. Criterios de finalización

Una fase solo está completa cuando:

1. existe problema y contrato documentados;
2. SQL y bindings tienen tests por dialecto;
3. el comportamiento se ejecuta al menos en SQLite y, cuando se marque portable, en los demás motores;
4. analyzer, runtime, catálogo y docs coinciden;
5. pruebas de regresión cubren cada bug;
6. race detector cubre estado compartido;
7. benchmarks respaldan afirmaciones de rendimiento;
8. no se registran secretos por defecto;
9. `go vet`, `go test`, `go test -race`, `go build`, generadores y extensión VS Code pasan.

## 13. Decisión

GranDB debe evolucionar incrementalmente desde su query builder actual. La primera entrega debe corregir contratos y seguridad; la segunda debe introducir query specs y dialectos; la tercera debe construir modelos sobre esa base. Esta secuencia conserva compatibilidad, evita una reescritura y permite demostrar cada garantía con pruebas.

## 14. Estado de implementación posterior a la auditoría

La primera etapa aplicada a partir de este informe incluye:

- corrección de `findMany` y del alias `firstofail` con regresiones;
- rechazo de `update` sin filtros;
- validación de identificadores simples y operadores de comparación;
- quoting de tablas y columnas de joins;
- eliminación de logs automáticos de SQL y bindings;
- strings con apariencia de función SQL tratados como datos ligados;
- orden determinista de columnas en escrituras;
- dialecto central para random, introspección de columnas y límites de parámetros;
- extracción de fecha y contención escalar JSON compiladas por dialecto;
- `upsert` real para los cuatro dialectos, con ejecución SQLite y golden tests para los otros tres;
- `insertMany` con batching y rollback atómico comprobado en SQLite;
- `simplePaginate`, `cursorPaginate` y `chunkById` sin consumir el builder original;
- `explain()` para SQLite/MySQL/PostgreSQL, con límite SQL Server explícito;
- reset seguro de `sqlite_sequence` mediante binding.

Persisten como trabajo planificado QuerySpec completo, tests con servidores reales,
metadata de modelos, hidratación, casts, relaciones, eager loading, scopes, soft
deletes y streaming. No se consideran implementados por aparecer en este roadmap.

### Baseline de rendimiento

Medición local en Windows/amd64, Intel i5-10300H, `-benchtime=100ms`. Es un
baseline reproducible, no una comparación antes/después ni una promesa para otros
equipos:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Compilar SELECT con filtros/orden/límite | 1,391 | 400 | 11 |
| Compilar upsert PostgreSQL de 100 filas | 65,476 | 24,433 | 529 |
| Consultar y mapear 100 filas SQLite | 133,068 | 39,151 | 443 |
| Consultar y mapear 1000 filas SQLite | 1,174,891 | 384,206 | 4,792 |

`database_benchmark_test.go` conserva los escenarios. El costo por fila confirma
que el futuro hydrator debe medirse contra `rowsToMap` y que optimizar la
compilación antes de reducir round trips tendría poco impacto en lecturas grandes.

### Suite cross database

`TestGranDBCrossDatabaseContract` ejecuta siempre SQLite y comparte CRUD,
bindings, bulk y upsert con los otros motores. MySQL, PostgreSQL y SQL Server se
activan mediante `JOSS_TEST_MYSQL_DSN`, `JOSS_TEST_POSTGRES_DSN` y
`JOSS_TEST_SQLSERVER_DSN`. Un skip sin DSN significa “no verificado”, no éxito de
compatibilidad.

## 15. Phase 2 — Model ORM

### Implementado

- `Model` nativo heredable con despacho estático y de instancia desde Joss.
- `modelMetadata` inmutable integrado en el cache por runtime de metadata de clases.
- Tabla, primary key, key type, incremento, timestamps, fillable, guarded,
  hidden, visible y casts configurables mediante literales del AST.
- Estado por instancia para original, últimos cambios, relaciones cargadas,
  existencia, eliminación y modo query.
- Hydrator determinista que conserva `NULL` y aplica casts de números, decimal,
  bool, fechas y JSON.
- `create`, `fill`, `forceFill`, `save`, `refresh`, `isDirty`, `isClean`,
  `wasChanged`, `getOriginal`, `getChanges`, `toMap` y `toJSON`.
- INSERT frente a UPDATE decidido por estado explícito. UPDATE parcial y omisión
  comprobada de SQL cuando el modelo no cambió.
- `query`, `all`, `find`, `findMany`, `first` y `get` con hidratación de la clase
  concreta y primary keys personalizadas. GranDB directo conserva mapas.
- `belongsTo`, `hasOne`, `hasMany`, `with`, eager loading anidado, `load` y
  `loadMissing`, usando consultas por lote e índices por clave.
- Serialización con `hidden`/`visible` y protección de ciclos indirecta al
  incluir únicamente relaciones que fueron cargadas explícitamente.

### Arquitectura real

```text
Model subclass
   │ literals → cached modelMetadata
   ▼
Model query state
   ▼
GranDB Query Builder
   ▼
Dialect → database/sql
   ▼
rowsToMap → model hydrator → casts → Instance + original state
                                      │
                                      └→ relation eager loader
```

No existe un segundo compilador SQL ni un cache global. `Instance` posee su
estado mutable; la metadata compartida pertenece al ciclo de vida del runtime.

### Relaciones

| Feature | Implemented | Tested SQLite | MySQL | PostgreSQL | SQL Server |
|---|---:|---:|---:|---:|---:|
| belongsTo | Sí | Sí | No verificado | No verificado | No verificado |
| hasOne | Sí | Cobertura de infraestructura | No verificado | No verificado | No verificado |
| hasMany | Sí | Sí | No verificado | No verificado | No verificado |
| belongsToMany | No | No | No | No | No |
| Eager Loading | Sí | Sí | No verificado | No verificado | No verificado |
| Nested Eager | Sí | Cobertura funcional compartida | No verificado | No verificado | No verificado |
| attach / detach / sync | No | No | No | No | No |

### Seguridad y semántica

La política predeterminada es `guarded = ["*"]`; `create` y `fill` fallan ante
atributos no permitidos. `forceFill` es una vía interna explícita. Los casts
inválidos producen `InvalidCast`; metadata no literal produce
`ModelMetadataError`. Un modelo eliminado no puede volver a guardarse y un
modelo existente sin primary key no puede actualizarse ni borrarse.

### Rendimiento

`BenchmarkModelHydration` mide 1, 100 y 1000 filas con casts bool y JSON y
reporta allocations mediante `ReportAllocs`. Baseline local Windows/amd64,
Intel i5-10300H, `-benchtime=100ms`:

| Filas | ns/op | B/op | allocs/op |
|---:|---:|---:|---:|
| 1 | 7,140 | 1,498 | 21 |
| 100 | 818,021 | 150,063 | 2,001 |
| 1000 | 6,162,125 | 1,498,866 | 20,004 |

Estas cifras son un baseline del host, no una promesa portable. El benchmark
reproducible es la evidencia canónica.
El eager loader extrae claves únicas, usa una consulta `WHERE IN` por relación
y agrupa en mapas; evita el algoritmo padres por relacionados.

### Limitaciones explícitas

- Pivot, `belongsToMany`, `attach`, `detach` y `sync` siguen pendientes.
- No hay lazy loading automático ni modo `preventLazyLoading`.
- Scopes, soft deletes, accessors, mutators y eventos siguen pendientes.
- Timestamps de DB se apoyan todavía en las reglas del builder; el modelo no
  refresca automáticamente valores generados por el servidor después de INSERT.
- MySQL, PostgreSQL y SQL Server requieren sus DSN para validar esta capa en
  servidores reales; compilación no se presenta como verificación runtime.
- La serialización evita exponer campos ocultos, pero no sustituye autorización.

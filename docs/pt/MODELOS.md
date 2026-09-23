# Datos y modelos con GranDB

[Índice](README.md) · Antes: [mapas](COLECCIONES.md), [configuración](CONFIGURACION.md) · Después: [migraciones](MIGRACIONES.md)

Una base de datos conserva registros organizados en tablas. Cada fila representa
un registro; cada columna, un dato como nombre o precio. GranDB construye SQL
mediante llamadas encadenadas: los filtros preparan la consulta y una operación
como `get()` la ejecuta. Su API se inspira en query builders conocidos; **no
es una implementación completa de Laravel Eloquent**. Cuando una clase hereda
`Model`, la misma infraestructura hidrata instancias con estado y casts.

## Model ORM

Un modelo declara configuración mediante propiedades protegidas con valores
literales. La metadata se calcula una vez por clase. Por seguridad, la asignación
masiva queda bloqueada por defecto: declara `fillable` o configura `guarded`.

<!-- joss-check: requiere tabla users y conexión configurada -->
```joss
public class User extends Model {
    protected string $table = "users"
    protected string $primaryKey = "uuid"
    protected string $keyType = "string"
    protected bool $incrementing = false
    protected bool $timestamps = false
    protected array $fillable = ["uuid", "name", "active", "profile"]
    protected array $hidden = ["password"]
    protected map $casts = {"active": "bool", "profile": "json"}

    public func posts(): mixed {
        return $this->hasMany("Post", "user_uuid", "uuid")
    }
}
```

`User::find(valor)`, `first()` y `get()` retornan `User` o arrays de modelos.
Las consultas iniciadas directamente con `GranDB::table()` continúan retornando
mapas. Se admiten casts `int`, `float`, `decimal`, `string`, `bool`, `date`,
`datetime`, `json`, `array` y `object`; un `NULL` SQL permanece `null`.

<!-- joss-check: CRUD de Model requiere tabla users -->
```joss
public class User extends Model {
    protected string $table = "users"
    protected string $primaryKey = "uuid"
    protected bool $incrementing = false
    protected array $fillable = ["uuid", "name", "active"]
}
$user = User::create({"uuid": "u-1", "name": "Ada", "active": true})
$user->name = "Grace"
$dirty = $user->isDirty("name")
$user->save()
$user->refresh()
```

`save()` elige INSERT para un modelo nuevo y UPDATE parcial para uno hidratado.
Un modelo limpio no ejecuta UPDATE. `getOriginal()`, `getChanges()`, `isClean()`,
`isDirty()` y `wasChanged()` exponen el estado. `forceFill()` omite la protección
de asignación masiva y debe reservarse para datos internos ya validados.

Las relaciones disponibles son `belongsTo`, `hasOne` y `hasMany`, incluidas
claves personalizadas. `with("posts")` las carga en lote y
`with("posts.comments")` admite rutas anidadas. `load()` y `loadMissing()` usan
el mismo cargador. Una relación cargada vacía se distingue de una no cargada.

`belongsToMany(clase, pivot, clavePadrePivot, claveRelacionadaPivot,
clavePadre, claveRelacionada)` conserva columnas adicionales en el atributo
separado `pivot`. La relación ofrece `attach`, `detach` y `sync`. Un array vacío
en `detach([])` no borra nada; `detach()` sin argumentos elimina explícitamente
todas las asociaciones del padre. `sync()` calcula el delta y ejecuta todas sus
operaciones en una transacción con rollback completo.

Los modelos con `protected bool $softDeletes = true` reciben el filtro
`deleted_at IS NULL`. `withTrashed()`, `onlyTrashed()`, `withoutTrashed()`,
`restore()` y `forceDelete()` controlan ese scope. `scope("named", valor)` llama
de forma explícita a un método `scopeNamed(query, valor)` del modelo.

Los hooks `saving`, `creating`, `created`, `updating`, `updated`, `saved`,
`deleting`, `deleted`, `restoring` y `restored` se ejecutan alrededor de la
persistencia. Un hook previo que retorna `false` cancela la operación. Los hooks
posteriores solo se ejecutan después de SQL exitoso.

`firstOrNew`, `firstOrCreate` y `updateOrCreate` reutilizan hydration y `save`.
La base de datos debe tener una constraint única para resolver carreras entre
la búsqueda y el INSERT; estos helpers no prometen atomicidad por sí solos.

<!-- joss-check: eager loading requiere modelos User/Post y sus tablas -->
```joss
public class User extends Model {
    protected string $table = "users"
    public func posts(): mixed {
        return $this->hasMany("Post", "user_id", "id")
    }
}
$users = User::query()->with("posts")->orderBy("name", "asc")->get()
```

Todavía no forman parte del contrato: lazy loading automático, accessors,
mutators, scopes globales personalizados y guardado automático de grafos.

## Primera consulta

Este fragmento requiere una conexión configurada y una tabla products con
las columnas indicadas. No crea la tabla; para hacerlo lee Schema Builder.

<!-- joss-check: requiere tabla products -->
```joss
$products = GranDB::table("products")
    ->where("active", true)
    ->orderByDesc("id")
    ->get()
foreach ($products as $product) {
    print($product["name"])
}
```

`get()` devuelve una lista nativa de mapas. No llames a json_decode sobre ella.
`first()` devuelve un mapa o null. Una clase que hereda `GranDB` puede centralizar
consultas del dominio; no adquiere relaciones Eloquent, eventos ni validadores
que no estén implementados. Los builders son mutables: crea uno por consulta
independiente para evitar arrastrar filtros. El prefijo de tablas usa
`PREFIX` (alias `DB_PREFIX`); no lo agregues dos veces.

## Construcción de consultas

Todos estos métodos devuelven el builder salvo los terminales de la siguiente
tabla. Las variantes `orWhere...` agregan OR; los aliases en minúsculas
registrados están enumerados en el [catálogo](CATALOGO_NATIVO.md).

| Método y argumentos | Efecto |
|---|---|
| `table(nombre)` | Selecciona tabla y reinicia estado de lectura. |
| `select(stringOArray)`, `distinct()` | Columnas o expresión SQL de confianza; filas distintas. |
| `where(col,valor)`, `where(col,op,valor)`, `orWhere(...)` | Compara con valores ligados como parámetros. |
| `where(callback)` | Agrupa filtros del callback, que recibe un GranDB. |
| `whereColumn(a,[op,]b)`, `orWhereColumn` | Compara columnas. |
| `whereNot(col,valor)`, `orWhereNot` | NOT de igualdad. |
| `whereLike(col,texto)`, `orWhereLike` | Añade % a ambos lados si no hay % en el patrón. |
| `whereIn(col,array)`, `whereNotIn`, variantes OR | Pertenencia; IN vacío es falso y NOT IN vacío verdadero. |
| `whereNull(col)`, `whereNotNull`, variantes OR | Ausencia/presencia SQL. |
| `whereBetween(col,[min,max])`, `whereNotBetween`, variantes OR | Intervalos inclusivos SQL. |
| `whereDate/Year/Month/Day/Time(col,valor)`, variantes OR | Compila la extracción de fecha para cada dialecto. SQLite normaliza año/mes/día numéricos al texto producido por `strftime`. |
| `whereJsonContains(col,valor)`, `orWhereJsonContains` | Busca un escalar en un array JSON mediante JSON1, JSON_CONTAINS, jsonb u OPENJSON según el motor. Estructuras anidadas requieren una consulta explícita. |
| `join/innerJoin/leftJoin/rightJoin(tabla,a,op,b)` | Unión de tablas con condición de columnas. |
| `crossJoin(tabla)` | Producto de filas; puede multiplicar mucho el resultado. |
| `groupBy(columnas...)`, `having(col,op,valor)`, `orHaving` | Agrupación y filtros sobre grupos. |
| `orderBy(col,direccion)`, `orderByAsc(col)`, `orderByDesc(col)` | Establece orden; ASC/DESC. |
| `latest([col])`, `oldest([col])`, `inRandomOrder()`, `reorder()` | Orden temporal, aleatorio o limpieza de orden. |
| `limit(n)` / `take(n)`, `offset(n)` / `skip(n)` | Tamaño y desplazamiento. |
| `forPage(pagina,tamaño)` | Calcula límite y desplazamiento. |
| `when(condicion,callback,[alternativo])` | Ejecuta callback si verdadero, alternativo si falso. Ambos reciben builder y condición. |
| `unless(condicion,callback)` | Ejecuta si falso; también recibe dos argumentos. |

No pases nombres de columnas, operadores ni SQL arbitrario desde una petición.
Los valores ligados no convierten las partes estructurales de SQL en seguras.

<!-- joss-check: construcción de consulta con parámetros tipados -->
```joss
$roleFilter = "editor"
$query = GranDB::table("users")
    ->when($roleFilter, func(GranDB $q, mixed $valor) {
        $q->where("role", $valor)
    })
    ->unless(false, func(GranDB $q, mixed $condicion) {
        $q->where("is_deprecated", 0)
    })
```

## Lectura e inspección

| Terminal | Resultado |
|---|---|
| `get()` | Array de mapas; errores SQL pueden imprimirse y producir colección vacía. |
| `first()`, `find(id)`, `firstWhere(col,[op,]valor)` | Primer mapa o null. |
| `findMany(ids)` | Array de filas. |
| `firstOrFail()`, `findOrFail(id)` | Fila o excepción por ausencia. |
| `sole()` | Exige exactamente una fila; falla en cero o más de una. |
| `value(col)` | Valor de primera fila o null. |
| `pluck(col,[clave])` | Array; con clave, map. El retorno publicado no refleja todas las variantes. |
| `exists()`, `doesntExist()` | Bool. |
| `count()`, `sum(col)`, `avg(col)`, `min(col)`, `max(col)` | Agregados; no reemplazan una comprobación de error SQL. |
| `paginate(tamaño,pagina)` | Map con filas y metadatos de paginación. |
| `chunk(tamaño,callback)` | Procesa lotes; callback recibe array de filas. |
| `simplePaginate(tamaño,pagina)` | Página offset sin consulta COUNT; incluye `has_more`. |
| `cursorPaginate(tamaño,cursor,[columna])` | Página estable ascendente por clave; retorna `next_cursor`. |
| `chunkById(tamaño,callback,[columna])` | Lotes por clave creciente, resistentes a desplazamientos por cambios previos. |
| `toSql()`, `getBindings()` | SQL construido y array de valores ligados. |
| `explain()` | Filas del plan de SQLite, MySQL o PostgreSQL sin consumir el builder. SQL Server falla explícitamente hasta disponer de un batch SHOWPLAN seguro. |
| `dump()` | Imprime consulta y bindings. |
| `dd()` | Interrumpe mediante panic recuperable, no salida incondicional del proceso. |

`firstOrFail` y su alias histórico `firstofail` alcanzan el mismo handler. No se
publican como APIs los cases internos que no estén registrados (por ejemplo
`from`).

## Escrituras

`insert(mapa)` e `insertGetId(mapa)` reciben un único mapa de columnas y valores.
El primero informa éxito y el segundo devuelve el ID según el motor.
`insertMany(arrayDeMapas)` exige las mismas columnas en cada fila, usa orden de
columnas determinista, divide el trabajo según el límite de parámetros del motor
y envuelve todos los lotes en una transacción cuando no existe otra activa.
`update(mapa)` modifica las filas filtradas; `updateOrInsert(busqueda,valores)`
busca antes de actualizar o insertar y no promete atomicidad. `update` sin
`where` se rechaza; una actualización total debe expresarse mediante una API
masiva explícita cuando exista.

`upsert(filas, clavesUnicas, [columnasAActualizar])` acepta un mapa o un array de
mapas homogéneos. Compila `ON CONFLICT` en SQLite/PostgreSQL,
`ON DUPLICATE KEY UPDATE` en MySQL/MariaDB y `MERGE` en SQL Server. Requiere que
la base de datos tenga el constraint único correspondiente. Actualmente SQLite
posee prueba de integración local; los otros tres dialectos tienen pruebas de
compilación y deben validarse en la suite de integración antes de afirmar
compatibilidad completa.

`increment(col,[cantidad])`, `decrement(col,[cantidad])` construyen actualización
del contador; `touch()` actualiza la marca temporal. `delete()` sin where se
aborta. `deleteAll()` y `truncate()` son explícitamente destructivos.

Fragmento que requiere tabla products:

<!-- joss-check: escritura contextual -->
```joss
$id = GranDB::table("products")->insertGetId({"name": "Cuaderno", "active": true})
GranDB::table("products")->where("id", $id)->update({"name": "Cuaderno azul"})
```

Las tablas, columnas y operadores pasan validación estructural. Los valores,
incluidos textos como `CURRENT_TIMESTAMP`, se envían como bindings. Las
expresiones internas del ORM no se confunden con strings de usuario. `select`
continúa aceptando una expresión SQL de confianza por compatibilidad; no debe
recibir texto procedente de una petición.

## Transacciones

`GranDB::transaction(callback)` abre un `sql.Tx` y dirige las consultas SQL
ordinarias ejecutadas por el mismo runtime durante el callback a ese Tx. Si el
callback falla, revierte esas consultas; si termina, confirma. El retorno es
el del callback, o null ante ciertos fallos de apertura/commit. Las
transacciones anidadas se rechazan. El alcance no incluye trabajo asíncrono,
conexiones externas ni efectos de red/archivos; mantén esas operaciones fuera
del callback cuando necesites atomicidad SQL.

## Motores y disponibilidad

Hay adaptadores para SQLite, MySQL, PostgreSQL y SQL Server. El soporte de
conexión, placeholders y operaciones principales no significa equivalencia de
cada función SQL, índice o migración. Prueba la consulta con el motor objetivo.
`GranDB::connection(motor,opciones)`, `changeDB` y `use` seleccionan conexión
según el handler; `System::change_db` **no está registrado**.

Fuentes: [builder](../pkg/core/database.go),
[lecturas](../pkg/core/database_read.go), [inserts](../pkg/core/database_insert.go),
[updates](../pkg/core/database_update.go), [borrados](../pkg/core/database_delete.go).

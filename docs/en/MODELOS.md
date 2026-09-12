# Data and models with GranDB

[Index](README.md) · Before: [maps](COLECCIONES.md), [configuration](CONFIGURACION.md) · After: [migrations](MIGRACIONES.md)

A database keeps records organized in tables. Each row represents
a record; each column, a piece of information such as name or price. GranDB builds SQL
through chained calls: filters prepare the query and an operation
as `get()` executes it. Its API is inspired by well-known query builders; **no
is a complete implementation of Laravel Eloquent**.

## First consultation

This snippet requires a configured connection and a products table with
the indicated columns. It doesn't create the table; To do this read Schema Builder.<!-- joss-check: requiere tabla products -->
```joss
$products = GranDB::table("products")
    ->where("active", true)
    ->orderByDesc("id")
    ->get()
foreach ($products as $product) {
    print($product["name"])
}
```
`get()` returns a native list of maps. Don't call json_decode on it.
`first()` returns a map or null. A class that inherits `GranDB` can centralize
domain queries; does not acquire Eloquent relationships, events or validators
that are not implemented. Builders are mutable: create one per query
independent to avoid dragging filters. The tables prefix uses
`PREFIX` (alias `DB_PREFIX`); don't add it twice.

##Query construction

All of these methods return the builder except the terminals in the following
table. The `orWhere...` variants add OR; aliases in lowercase
registered are listed in the [catalog](CATALOGO_NATIVO.md).

| Method and arguments | Effect |
|---|---|
| `table(nombre)` | Select table and reset reading status. |
| `select(stringOArray)`, `distinct()` | Columns or trusted SQL expression; different rows. |
| `where(col,valor)`, `where(col,op,valor)`, `orWhere(...)` | Compares with values ​​bound as parameters. |
| `where(callback)` | Groups filters from the callback, which receives a GranDB. |
| `whereColumn(a,[op,]b)`, `orWhereColumn` | Compare columns. |
| `whereNot(col,valor)`, `orWhereNot` | NOT equality. |
| `whereLike(col,texto)`, `orWhereLike` | Add % to both sides if there is no % in the pattern. |
| `whereIn(col,array)`, `whereNotIn`, OR variants | Belonging; IN empty is false and NOT IN empty is true. |
| `whereNull(col)`, `whereNotNull`, OR variants | SQL absence/presence. |
| `whereBetween(col,[min,max])`, `whereNotBetween`, OR variants | SQL inclusive intervals. |
| `whereDate/Year/Month/Day/Time(col,valor)`, OR variants | Generates SQL functions from the component. Its portability depends on the engine. |
| `whereJsonContains(col,valor)`, `orWhereJsonContains` | Generate JSON_CONTAINS; does not guarantee support on all engines. |
| `join/innerJoin/leftJoin/rightJoin(tabla,a,op,b)` | Union of tables with column condition. |
| `crossJoin(tabla)` | Product of rows; can greatly multiply the result. |
| `groupBy(columnas...)`, `having(col,op,valor)`, `orHaving` | Grouping and filters on groups. |
| `orderBy(col,direccion)`, `orderByAsc(col)`, `orderByDesc(col)` | Establish order; ASC/OFF |
| `latest([col])`, `oldest([col])`, `inRandomOrder()`, `reorder()` | Temporary, random order or clean order. |
| `limit(n)` / `take(n)`, `offset(n)` / `skip(n)` | Size and displacement. |
| `forPage(pagina,tamaño)` | Calculate limit and displacement. |
| `when(condicion,callback,[alternativo])` | Execute callback if true, alternative if false. Both receive builder and condition. |
| `unless(condicion,callback)` | Execute if false; also receives two arguments. |

Don't pass column names, operators, or arbitrary SQL from a request.
Bound values ​​do not make the structural parts of SQL safe.<!-- joss-check: construcción de consulta con parámetros tipados -->
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
## Reading and inspection

| Terminal | Result |
|---|---|
| `get()` | Map Array; SQL errors can be printed and produce empty collection. |
| `first()`, `find(id)`, `firstWhere(col,[op,]valor)` | First map or null. |
| `findMany(ids)` | Array of rows. |
| `firstOrFail()`, `findOrFail(id)` | Row or exception due to absence. |
| `sole()` | It requires exactly one row; fails at zero or more than one. |
| `value(col)` | First row value or null. |
| `pluck(col,[clave])` | Array; with key, map. The published return does not reflect all variants. |
| `exists()`, `doesntExist()` | Bool. |
| `count()`, `sum(col)`, `avg(col)`, `min(col)`, `max(col)` | Aggregates; they do not replace an SQL error check. |
| `paginate(tamaño,pagina)` | Map with rows and pagination metadata. |
| `chunk(tamaño,callback)` | Process batches; callback receives array of rows. |
| `toSql()`, `getBindings()` | Constructed SQL and array of bound values. |
| `dump()` | Print query and bindings. |
| `dd()` | Interrupts through recoverable panic, not unconditional exit of the process. |

Registered alias `firstofail` does not match selector `firstorfail`
of the handler; use `firstOrFail`, not the shortened spelling. They are not published as APIs
internal cases that are not registered (for example `from`).

## Scriptures

`insert(mapa)` and `insertGetId(mapa)` receive a single map of columns and values.
The first one reports success and the second one returns the ID based on the engine.
`update(mapa)` modifies the filtered rows; `updateOrInsert(busqueda,valores)`
search before updating or inserting. `upsert` is a separate operation:
check your implementation and unique keys before assuming atomicity.

`increment(col,[cantidad])`, `decrement(col,[cantidad])` construct update
of the accountant; `touch()` updates the timestamp. `delete()` without where
aborts. `deleteAll()` and `truncate()` are explicitly destructive.

Fragment that requires products table:<!-- joss-check: escritura contextual -->
```joss
$id = GranDB::table("products")->insertGetId({"name": "Cuaderno", "active": true})
GranDB::table("products")->where("id", $id)->update({"name": "Cuaderno azul"})
```
##Transactions: limitation checked

A transaction should cause multiple writes to be committed or rolled back
together. `GranDB::transaction(callback)` opens a `sql.Tx`, calls the callback and
does commit/rollback, **but does not link that Tx to the queries executed by the
callback**: these continue using the normal connection. Does not currently offer the
expected atomicity for transfers or related changes. The return is
that of the callback, or null in the event of certain failures; don't use this API to promise
write reversal. This defect requires a runtime fix.

## Engines and availability

There are adapters for SQLite, MySQL, PostgreSQL and SQL Server. The support of
connection, placeholders and main operations does not mean equivalence of
each SQL function, index or migration. Test the query with the target engine.
`GranDB::connection(motor,opciones)`, `changeDB` and `use` select connection
according to the handler; `System::change_db` **is not registered**.

Sources: [builder](../../pkg/core/database.go),
[reads](../../pkg/core/database_read.go), [inserts](../../pkg/core/database_insert.go),
[updates](../../pkg/core/database_update.go), [deleted](../../pkg/core/database_delete.go).
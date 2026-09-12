# SchemaBuilder

A table defines what data a database stores. Schema creates or modifies that structure; Blueprint describes its columns within a closure. There are adapters for SQLite, MySQL, PostgreSQL and SQL Server, with dialect differences. Schema applies `PREFIX`/`DB_PREFIX` automatically.

[Index](README.md) · Before: [models](MODELOS.md) · After: [migrations](MIGRACIONES.md)

The example requires connection and an owners table that supports the composite key. It is a schematic fragment, not a stand-alone program.```joss
Schema::create("products", func(Blueprint $table) {
    $table->id()
    $table->string("sku", 50)->unique()
    $table->decimal("price", 10, 2)->default(0)
    $table->unsignedBigInteger("tenant_id")
    $table->unsignedBigInteger("owner_id")
    $table->unique(["tenant_id", "sku"])
    $table->foreign(["tenant_id", "owner_id"])
        ->references(["tenant_id", "id"])
        ->on("owners")
        ->onDelete("cascade")
    $table->timestamps()
})
```
## Schema

- `create($table, func(Blueprint $blueprint))`
- `table($table, func(Blueprint $blueprint))`
- `rename($from, $to)`
- `drop($table)` and `dropIfExists($table)`
- `hasTable($table)` and `hasColumn($table, $column)`

##Blueprint

Types: `id`, `increments`, `integer`, `tinyInteger`, `smallInteger`, `mediumInteger`, `bigInteger`, `unsignedInteger`, `unsignedBigInteger`, `float`, `double`, `decimal`, `char`, `string`, `text`, `mediumText`, `longText`, `date`, `dateTime`, `time`, `timestamp`, `timestamps`, `softDeletes`, `boolean`, `json` and `enum`.

Last column modifiers: `nullable`, `unsigned`, `unique()`, `default` and `comment`. `unsigned` and the inline SQL comment are MySQL properties; the other engines preserve the portable type without inventing an equivalent semantics.

Table commands:

- `dropColumn($column)` or `dropColumn([$a, $b])`
- `renameColumn($from, $to)`
- `index($columns, $name=nil)`
- `unique($columns, $name=nil)` or `uniqueIndex(...)`
- `dropIndex($name)`
- `foreign($columns, $name=nil)->references($columns)->on($table)->onDelete($action)->onUpdate($action)`

SQLite rebuilds the table transactionally when a foreign key is added using `Schema::table()`, preserving data, indexes, and explicit triggers. PostgreSQL uses `SERIAL`/`BIGSERIAL`, `JSONB` and equivalent types. SQL Server uses `IDENTITY(1,1) PRIMARY KEY`, `NVARCHAR(MAX)`, `DATETIME2`, and `[...]` delimiters.
# Schema Builder

Una tabla define qué datos guarda una base de datos. Schema crea o modifica esa estructura; Blueprint describe sus columnas dentro de una closure. Hay adaptadores para SQLite, MySQL, PostgreSQL y SQL Server, con diferencias de dialecto. Schema aplica `PREFIX`/`DB_PREFIX` automáticamente.

[Índice](README.md) · Antes: [modelos](MODELOS.md) · Después: [migraciones](MIGRACIONES.md)

El ejemplo requiere conexión y una tabla owners compatible con la clave compuesta. Es un fragmento de esquema, no un programa independiente.

```joss
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
- `drop($table)` y `dropIfExists($table)`
- `hasTable($table)` y `hasColumn($table, $column)`

## Blueprint

Tipos: `id`, `increments`, `integer`, `tinyInteger`, `smallInteger`, `mediumInteger`, `bigInteger`, `unsignedInteger`, `unsignedBigInteger`, `float`, `double`, `decimal`, `char`, `string`, `text`, `mediumText`, `longText`, `date`, `dateTime`, `time`, `timestamp`, `timestamps`, `softDeletes`, `boolean`, `json` y `enum`.

Modificadores de la última columna: `nullable`, `unsigned`, `unique()`, `default` y `comment`. `unsigned` y el comentario SQL inline son propiedades de MySQL; los demás motores conservan el tipo portable sin inventar una semántica equivalente.

Comandos de tabla:

- `dropColumn($column)` o `dropColumn([$a, $b])`
- `renameColumn($from, $to)`
- `index($columns, $name=nil)`
- `unique($columns, $name=nil)` o `uniqueIndex(...)`
- `dropIndex($name)`
- `foreign($columns, $name=nil)->references($columns)->on($table)->onDelete($action)->onUpdate($action)`
- `dropForeign($name)`

SQLite reconstruye la tabla de forma transaccional cuando se agrega una clave foránea mediante `Schema::table()`, preservando datos, índices y triggers explícitos. PostgreSQL usa `SERIAL`/`BIGSERIAL`, `JSONB` y tipos equivalentes. SQL Server usa `IDENTITY(1,1) PRIMARY KEY`, `NVARCHAR(MAX)`, `DATETIME2` y delimitadores `[...]`.

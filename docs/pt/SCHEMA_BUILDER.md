# SchemaBuilder

Uma tabela define quais dados um banco de dados armazena. O esquema cria ou modifica essa estrutura; O Blueprint descreve suas colunas dentro de um fechamento. Existem adaptadores para SQLite, MySQL, PostgreSQL e SQL Server, com diferenças de dialeto. O esquema aplica `PREFIX`/`DB_PREFIX` automaticamente.

[Índice](README.md) · Antes: [modelos](MODELOS.md) · Depois: [migrações](MIGRACIONES.md)

O exemplo requer conexão e uma tabela de proprietários que suporte a chave composta.
É um fragmento esquemático, não um programa independente.

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

`Schema::hasForeign($table, $constraint)` verifica se existe uma chave estrangeira com esse nome. Use-a em migrações que precisem tolerar bancos MySQL parcialmente atualizados:

```joss
(!Schema::hasForeign("backups", "fk_backups_user")) ? {
    Schema::table("backups", func(mixed $table) {
        $table->foreign("user_id", "fk_backups_user")->references("id")->on("users")->onDelete("cascade")
    })
}
```
<!-- joss-check: requiere una base de datos configurada -->

## Esquema

- `create($table, func(Blueprint $blueprint))`
- `table($table, func(Blueprint $blueprint))`
- `rename($from, $to)`
- `drop($table)` e `dropIfExists($table)`
- `hasTable($table)`, `hasColumn($table, $column)` e `hasForeign($table, $constraint)`

## Projeto

Tipos: `id`, `increments`, `integer`, `tinyInteger`, `smallInteger`, `mediumInteger`, `bigInteger`, `unsignedInteger`, `unsignedBigInteger`, `float`, `double`, `decimal`, `char`, `string`, `text`, `mediumText`, `longText`, `date`, `dateTime`, `time`, `timestamp`, `timestamps`, `softDeletes`, `boolean`, `json` e `enum`.

Modificadores da última coluna: `nullable`, `unsigned`, `unique()`, `default` e `comment`. `unsigned` e o comentário SQL embutido são propriedades do MySQL; os outros motores preservam o tipo portátil sem inventar uma semântica equivalente.

Comandos de tabela:

- `dropColumn($column)` ou `dropColumn([$a, $b])`
- `renameColumn($from, $to)`
- `index($columns, $name=nil)`
- `unique($columns, $name=nil)` ou `uniqueIndex(...)`
- `dropIndex($name)`
- `foreign($columns, $name=nil)->references($columns)->on($table)->onDelete($action)->onUpdate($action)`
- `dropForeign($name)`

SQLite reconstrói a tabela transacionalmente quando uma chave estrangeira é adicionada usando `Schema::table()`, preservando dados, índices e gatilhos explícitos. PostgreSQL usa `SERIAL`/`BIGSERIAL`, `JSONB` e tipos equivalentes. O SQL Server usa delimitadores `IDENTITY(1,1) PRIMARY KEY`, `NVARCHAR(MAX)`, `DATETIME2` e `[...]`.

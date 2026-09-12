# Migrations

[Index](README.md) · Before: [Schema](SCHEMA_BUILDER.md) · After: [CLI](CLI.md)

A migration saves a structure change so that different environments can apply it in the same order. `up` applies the change; `down` describes how to rollback, although it does not imply that there is a public rollback command.```bash
joss make:migration create_products
joss migrate
joss migrate:fresh
```
The name can be written as `create_products`, `create_products_table` or
`product`; all three ways normalize the logical table to `products` and generate
`CreateProductsTable`. The spelling of the command is `make:migration`.

The generator creates a class that extends `Migration`, with `up()` and `down()`. The runner executes pending migrations in name order and registers the batch in the migration table with the configured prefix.```joss
public class CreateProductsTable extends Migration {
    public func up() {
        Schema::create("products", func(Blueprint $table) {
            $table->id()
            $table->string("name")
            $table->timestamps()
        })
    }

    public func down() {
        Schema::drop("products")
    }
}
```
Don't manually add the prefix unless you want to set it in code; `Schema` applies it from `PREFIX`/`DB_PREFIX`.

`migrate:fresh` removes all visible tables from the schema, recreates internal tables, and runs the migrations. It is destructive and intended for development or disposable environments.

A migration is only considered complete after registering your name and
batch. A read, parse, structure or register failure stops the command with
non-zero exit code.

There are adapters for SQLite, MySQL, PostgreSQL and SQL Server; validates the specific operations in the chosen engine. Details of columns, indexes and foreign keys are in [Schema Builder](SCHEMA_BUILDER.md).
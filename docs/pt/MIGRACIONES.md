# Migrações

[Índice](README.md) · Antes: [Esquema](SCHEMA_BUILDER.md) · Depois: [CLI](CLI.md)

Uma migração salva uma alteração de estrutura para que diferentes ambientes possam aplicá-la na mesma ordem. `up` aplica a mudança; `down` descreve como reverter, embora não implique que exista um comando público de reversão.```bash
joss make:migration create_products
joss migrate
joss migrate:fresh
```
O nome pode ser escrito como `create_products`, `create_products_table` ou
`product`; todas as três maneiras normalizam a tabela lógica para `products` e geram
`CreateProductsTable`. A grafia do comando é `make:migration`.

O gerador cria uma classe que estende `Migration`, com `up()` e `down()`. O executor executa migrações pendentes em ordem de nome e registra o lote na tabela de migração com o prefixo configurado.```joss
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
Não adicione manualmente o prefixo, a menos que queira defini-lo no código; `Schema` aplica-o a partir de `PREFIX`/`DB_PREFIX`.

`migrate:fresh` remove todas as tabelas visíveis do esquema, recria tabelas internas e executa as migrações. É destrutivo e destinado a ambientes de desenvolvimento ou descartáveis.

Uma migração só é considerada concluída após registrar seu nome e
lote. Uma falha de leitura, análise, estrutura ou registro interrompe o comando com
código de saída diferente de zero.

Existem adaptadores para SQLite, MySQL, PostgreSQL e SQL Server; valida as operações específicas no motor escolhido. Detalhes de colunas, índices e chaves estrangeiras estão em [Schema Builder](SCHEMA_BUILDER.md).
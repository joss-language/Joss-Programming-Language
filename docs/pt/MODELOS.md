# Dados e modelos com GranDB

[Índice](README.md) · Antes: [mapas](COLECCIONES.md), [configuração](CONFIGURACION.md) · Depois: [migrações](MIGRACIONES.md)

Um banco de dados mantém os registros organizados em tabelas. Cada linha representa
um registro; cada coluna, uma informação como nome ou preço. GranDB compila SQL
através de chamadas encadeadas: os filtros preparam a consulta e uma operação
como `get()` o executa. Sua API é inspirada em construtores de consultas conhecidos; **não
é uma implementação completa do Laravel Eloquent**.

## Primeira consulta

Este snippet requer uma conexão configurada e uma tabela de produtos com
as colunas indicadas. Não cria a tabela; Para fazer isso, leia Schema Builder.<!-- joss-check: requiere tabla products -->
```joss
$products = GranDB::table("products")
    ->where("active", true)
    ->orderByDesc("id")
    ->get()
foreach ($products as $product) {
    print($product["name"])
}
```
`get()` retorna uma lista nativa de mapas. Não chame json_decode nele.
`first()` retorna um mapa ou nulo. Uma classe que herda `GranDB` pode centralizar
consultas de domínio; não adquire relacionamentos, eventos ou validadores Eloquent
que não são implementados. Os construtores são mutáveis: crie um por consulta
independente para evitar arrastar filtros. O prefixo das tabelas usa
`PREFIX` (também conhecido como `DB_PREFIX`); não adicione duas vezes.

##Construção de consulta

Todos esses métodos retornam o construtor, exceto os terminais a seguir
mesa. As variantes `orWhere...` adicionam OR; aliases em letras minúsculas
registrados estão listados no [catálogo](CATALOGO_NATIVO.md).

| Método e argumentos | Efeito |
|---|---|
| `table(nombre)` | Selecione a tabela e redefina o status da leitura. |
| `select(stringOArray)`, `distinct()` | Colunas ou expressão SQL confiável; linhas diferentes. |
| `where(col,valor)`, `where(col,op,valor)`, `orWhere(...)` | Compara com valores vinculados como parâmetros. |
| `where(callback)` | Agrupa filtros do retorno de chamada, que recebe um GranDB. |
| `whereColumn(a,[op,]b)`, `orWhereColumn` | Compare colunas. |
| `whereNot(col,valor)`, `orWhereNot` | NÃO igualdade. |
| `whereLike(col,texto)`, `orWhereLike` | Adicione % em ambos os lados se não houver % no padrão. |
| `whereIn(col,array)`, `whereNotIn`, variantes OR | Pertencimento; IN vazio é falso e NOT IN vazio é verdadeiro. |
| `whereNull(col)`, `whereNotNull`, variantes OR | Ausência/presença de SQL. |
| `whereBetween(col,[min,max])`, `whereNotBetween`, variantes OR | Intervalos SQL inclusivos. |
| `whereDate/Year/Month/Day/Time(col,valor)`, OR variantes | Gera funções SQL do componente. Sua portabilidade depende do motor. |
| `whereJsonContains(col,valor)`, `orWhereJsonContains` | Gerar JSON_CONTAINS; não garante suporte em todos os motores. |
| `join/innerJoin/leftJoin/rightJoin(tabla,a,op,b)` | União de tabelas com condição de coluna. |
| `crossJoin(tabla)` | Produto de linhas; pode multiplicar enormemente o resultado. |
| `groupBy(columnas...)`, `having(col,op,valor)`, `orHaving` | Agrupamento e filtros em grupos. |
| `orderBy(col,direccion)`, `orderByAsc(col)`, `orderByDesc(col)` | Estabelecer ordem; ASC/DESLIGADO |
| `latest([col])`, `oldest([col])`, `inRandomOrder()`, `reorder()` | Ordem temporária, aleatória ou ordem limpa. |
| `limit(n)` / `take(n)`, `offset(n)` / `skip(n)` | Tamanho e deslocamento. |
| `forPage(pagina,tamaño)` | Calcular limite e deslocamento. |
| `when(condicion,callback,[alternativo])` | Execute o retorno de chamada se for verdadeiro, alternativa se for falso. Ambos recebem construtor e condição. |
| `unless(condicion,callback)` | Execute se for falso; também recebe dois argumentos. |

Não passe nomes de colunas, operadores ou SQL arbitrário de uma solicitação.
Os valores vinculados não tornam as partes estruturais do SQL seguras.<!-- joss-check: construcción de consulta con parámetros tipados -->
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
## Leitura e inspeção

| Terminais | Resultado |
|---|---|
| `get()` | Matriz de mapas; Erros SQL podem ser impressos e produzir uma coleção vazia. |
| `first()`, `find(id)`, `firstWhere(col,[op,]valor)` | Primeiro mapa ou nulo. |
| `findMany(ids)` | Matriz de linhas. |
| `firstOrFail()`, `findOrFail(id)` | Linha ou exceção por ausência. |
| `sole()` | Requer exatamente uma linha; falha em zero ou mais de um. |
| `value(col)` | Valor da primeira linha ou nulo. |
| `pluck(col,[clave])` | Variedade; com chave, mapa. O retorno publicado não reflete todas as variantes. |
| `exists()`, `doesntExist()` | Bool. |
| `count()`, `sum(col)`, `avg(col)`, `min(col)`, `max(col)` | Agregados; eles não substituem uma verificação de erro SQL. |
| `paginate(tamaño,pagina)` | Mapa com linhas e metadados de paginação. |
| `chunk(tamaño,callback)` | Processar lotes; retorno de chamada recebe uma matriz de linhas. |
| `toSql()`, `getBindings()` | SQL construído e matriz de valores vinculados. |
| `dump()` | Imprimir consulta e ligações. |
| `dd()` | Interrupções por pânico recuperável, não por saída incondicional do processo. |

O alias registrado `firstofail` não corresponde ao seletor `firstorfail`
do manipulador; use `firstOrFail`, não a ortografia abreviada. Eles não são publicados como APIs
casos internos que não estão registrados (por exemplo `from`).

## Escrituras

`insert(mapa)` e `insertGetId(mapa)` recebem um único mapa de colunas e valores.
O primeiro reporta sucesso e o segundo retorna o ID baseado no mecanismo.
`update(mapa)` modifica as linhas filtradas; `updateOrInsert(busqueda,valores)`
pesquise antes de atualizar ou inserir. `upsert` é uma operação separada:
verifique sua implementação e chaves exclusivas antes de assumir a atomicidade.

`increment(col,[cantidad])`, `decrement(col,[cantidad])` atualização de construção
do contador; `touch()` atualiza o carimbo de data/hora. `delete()` sem onde
aborta. `deleteAll()` e `truncate()` são explicitamente destrutivos.

Fragmento que requer tabela de produtos:<!-- joss-check: escritura contextual -->
```joss
$id = GranDB::table("products")->insertGetId({"name": "Cuaderno", "active": true})
GranDB::table("products")->where("id", $id)->update({"name": "Cuaderno azul"})
```
##Transações: limitação verificada

Uma transação deve fazer com que múltiplas gravações sejam confirmadas ou revertidas
juntos. `GranDB::transaction(callback)` abre um `sql.Tx`, chama o retorno de chamada e
faz commit/rollback, **mas não vincula esse Tx às consultas executadas pelo
retorno de chamada**: continuam usando a conexão normal. Atualmente não oferece o
atomicidade esperada para transferências ou mudanças relacionadas. O retorno é
o do retorno de chamada, ou nulo no caso de certas falhas; não use esta API para prometer
escrever reversão. Esse defeito requer uma correção em tempo de execução.

## Motores e disponibilidade

Existem adaptadores para SQLite, MySQL, PostgreSQL e SQL Server. O apoio de
conexão, espaços reservados e operações principais não significa equivalência de
cada função SQL, índice ou migração. Teste a consulta com o mecanismo de destino.
`GranDB::connection(motor,opciones)`, `changeDB` e `use` selecionam conexão
de acordo com o manipulador; `System::change_db` **não está registrado**.

Fontes: [construtor](../../pkg/core/database.go),
[lê](../../pkg/core/database_read.go), [inserções](../../pkg/core/database_insert.go),
[atualizações](../../pkg/core/database_update.go), [excluído](../../pkg/core/database_delete.go).
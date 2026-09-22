# Datos y modelos con GranDB

[Índice](README.md) · Antes: [mapas](COLECCIONES.md), [configuración](CONFIGURACION.md) · Después: [migraciones](MIGRACIONES.md)

Una base de datos conserva registros organizados en tablas. Cada fila representa
un registro; cada columna, un dato como nombre o precio. GranDB construye SQL
mediante llamadas encadenadas: los filtros preparan la consulta y una operación
como `get()` la ejecuta. Su API se inspira en query builders conocidos; **no
es una implementación completa de Laravel Eloquent**.

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
| `whereDate/Year/Month/Day/Time(col,valor)`, variantes OR | Genera funciones SQL del componente. Su portabilidad depende del motor. |
| `whereJsonContains(col,valor)`, `orWhereJsonContains` | Genera JSON_CONTAINS; no garantiza soporte en todos los motores. |
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
| `toSql()`, `getBindings()` | SQL construido y array de valores ligados. |
| `dump()` | Imprime consulta y bindings. |
| `dd()` | Interrumpe mediante panic recuperable, no salida incondicional del proceso. |

El alias registrado `firstofail` no coincide con el selector `firstorfail`
del handler; usa `firstOrFail`, no la grafía abreviada. No se publican como APIs
los cases internos que no estén registrados (por ejemplo `from`).

## Escrituras

`insert(mapa)` e `insertGetId(mapa)` reciben un único mapa de columnas y valores.
El primero informa éxito y el segundo devuelve el ID según el motor.
`update(mapa)` modifica las filas filtradas; `updateOrInsert(busqueda,valores)`
busca antes de actualizar o insertar. `upsert` es una operación separada:
consulta su implementación y claves únicas antes de asumir atomicidad.

`increment(col,[cantidad])`, `decrement(col,[cantidad])` construyen actualización
del contador; `touch()` actualiza la marca temporal. `delete()` sin where se
aborta. `deleteAll()` y `truncate()` son explícitamente destructivos.

Fragmento que requiere tabla products:

<!-- joss-check: escritura contextual -->
```joss
$id = GranDB::table("products")->insertGetId({"name": "Cuaderno", "active": true})
GranDB::table("products")->where("id", $id)->update({"name": "Cuaderno azul"})
```

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

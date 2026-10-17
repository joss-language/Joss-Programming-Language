# Global functions

[Index](README.md) · Before: [functions](FUNCIONES.md) · [Native classes](MODULOS_NATIVOS.md)

These functions are built into the runtime (`pkg/core/builtins.go`), without imports or installation.
The signature on this page describes the usage supported by the dispatcher; not all
parameters are published to the analyzer. `[x]` indicates an optional
argument and `...` multiple arguments. They are not characters you should copy.

Families share examples at the end. Function aliases such as
`doubleval` still exist; this does not restore the removed source type
`double`. Return types declared for tooling can be checked in the
[generated catalog](CATALOGO_NATIVO.md); the runtime result is described below.

## Type conversion and queries

| Signature | Result and limits | Example |
|---|---|---|
| `intval(valor)` | Integer; truncates float/decimal, bool becomes 0/1; invalid or missing gives 0. | `intval("12")` → 12 |
| `floatval(valor)`, `doubleval(valor)` | Approximate float; invalid gives 0. | `floatval("1.5")` |
| `decimal([valor])` | Decimal from number/string/bool; invalid or null gives zero. Textual suffixes m/M/d/D are trimmed. | `decimal("1.25")` |
| `strval(valor)` | Text via Go formatting, null gives empty string. Not JSON. | `strval(12)` |
| `boolval(valor)` | Bool according to runtime truthiness. Zero float and empty map differ from other languages. | `boolval("")` → false |
| `is_numeric(valor)` | Bool: number or text parseable as float/decimal. Does not validate a domain like “age”. | `is_numeric("12")` |
| `is_int(valor)`, `is_integer(valor)` | Bool; integer, without converting strings. | `is_int("12")` → false |
| `is_float(valor)`, `is_double(valor)` | Bool; float, without converting. | `is_float(1.5)` |
| `is_decimal(valor)` | Bool; concrete decimal. | `is_decimal(1m)` |
| `is_string(valor)` | Bool; concrete string. | `is_string("a")` |
| `is_array(valor)` | Bool; array/slice, not map. | `is_array([1])` |
| `is_null(valor)` | Bool; absence of value; with no argument also true. | `is_null(null)` |
| `isset(expresiones...)` | Special parser form: checks existence without error for missing variables. A variable bound to null counts as existing; not a general map-key check. | `isset($nombre)` |
| `empty(expresion)` | Special form: true if missing or if the value is falsy according to runtime rules. | `empty($ausente)` |

Explicit conversions are permissive; they do not replace validation.
For exact conditions compare against `0`, `null`, or `""` depending on what you
need. See [types](SISTEMA_TIPOS.md) and [syntax](SINTAXIS.md).

## Arrays, maps, and functional collections

| Signature | Result, mutation, and errors | Example |
|---|---|---|
| `len(valor)`, `count(valor)` | Length of array/map; string bytes; other values give 0. | `count([2,3])` → 2 |
| `keys(map)`, `array_keys(map)` | Array of keys without guaranteed order; invalid gives []. | `keys({"a":1})` |
| `values(map)`, `array_values(map)` | Array of values without guaranteed order; invalid gives []. | `values({"a":1})` |
| `explode(separador, texto)` | Array of segments; requires two strings, invalid gives null. | `explode(",", "a,b")` |
| `append(array, elemento)` | Returns extended array; reassign the result. Invalid gives null. | `$a = append($a, 2)` |
| `merge(array1, array2)` | New container with both sequences; invalid gives null. | `merge([1],[2])` |
| `array_merge(primero, otros...)` | Concatenated array or merged map depending on first argument; later keys win. Ignores subsequent arguments of another type; invalid gives []. | `array_merge({"a":1},{"a":2})` |
| `array_push(array, elementos...)` | Extended array; does not return length. Reassign. Invalid gives null. | `$a = array_push($a,2,3)` |
| `end(array)`, `array_pop(array)` | Last element or null. **Does not shrink the array**. | `array_pop([1,2])` → 2 |
| `array_shift(array)` | First element or null. **Does not shrink the array**. | `array_shift([1,2])` → 1 |
| `array_slice(array,inicio,[longitud])` | Shallow view; negatives relative to end; out of range gives []. Shares backing storage. | `array_slice([1,2,3],1)` |
| `array_unique(array)` | New array, equality by textual representation; may merge values of different types. | `array_unique([1,1,2])` |
| `array_reverse(array)` | New reversed array; invalid gives []. | `array_reverse([1,2])` |
| `array_column(array,clave)` | Values for that key across map elements; skips other elements/missing keys. | `array_column([{"id":1}],"id")` |
| `in_array(valor,array)` | Bool; in Joss arrays supports deep or textual equality. | `in_array(1,["1"])` → true |
| `array_key_exists(clave,map)` | Existence bool; do not confuse with non-null value. | `array_key_exists("a",{"a":null})` |
| `map(array, closure)` | Returns a new array applying `closure` to each element. | `map([1,2], func(int $x): int { return $x * 2 })` |
| `filter(array, closure)` | Returns a new array with elements where `closure` returns truthy. | `filter([1,2,3], func(int $x): bool { return $x > 1 })` |
| `sum(array)` | Numeric sum of elements; preserves `decimal`, checks integer overflow, and rejects mixing `float` with `decimal` (`JOSS-ARITH-003`). | `sum([10, 20, 30])` → 60 |

## Text and formatting

| Signature | Result and limits | Example |
|---|---|---|
| `print(valores...)`, `echo(valores...)` | Print each argument with a newline; returns null. | `print("Hola")` |
| `printf(formato,valores...)` | Go format, without implicit newline; returns null. | `printf("%s: %d\\n","Edad",20)` |
| `strlen(texto)` | Number of Unicode code points; null gives 0. | `strlen("é")` → 1 |
| `str_contains(texto,parte)`, `contains(texto,parte)` | Bool; arguments are converted to text. | `contains("casa","as")` |
| `str_starts_with(texto,prefijo)`, `starts_with(...)` | Prefix bool. | `starts_with("abc","a")` |
| `str_ends_with(texto,sufijo)`, `ends_with(...)` | Suffix bool. | `ends_with("abc","c")` |
| `str_replace(buscar,reemplazo,texto)` | Replaces all matches. Argument order differs from Str::replace. | `str_replace("a","o","casa")` |
| `strtolower(texto)`, `to_lower(texto)` | Unicode lowercase. | `to_lower("HOLA")` |
| `strtoupper(texto)`, `to_upper(texto)` | Unicode uppercase. | `to_upper("hola")` |
| `ucfirst(texto)`, `lcfirst(texto)` | Changes first Unicode code point to uppercase/lowercase. | `ucfirst("hola")` |
| `ucwords(texto)` | Capitalization via Go's strings.Title; not linguistic analysis. | `ucwords("hola mundo")` |
| `trim(texto,[conjunto])` | Strips Unicode whitespace or characters in the cutset from both ends. | `trim(" hola ")` |
| `ltrim(texto,[conjunto])`, `rtrim(texto,[conjunto])` | Single-end trim; defaults to ASCII whitespace. | `ltrim(" hola")` |
| `substr(texto,inicio,[longitud])` | Unicode code points; negatives relative to end; out of range gives "". | `substr("abc",-2)` → bc |
| `strpos(texto,parte)` | Position in Unicode code points or false if missing. Zero is a valid match. | `strpos("abc","a")` → 0 |
| `implode(separador,array)`, `join(separador,array)` | Converts elements to text and joins; invalid gives "". | `join("-",[1,2])` |
| `str_pad(texto,longitud,[relleno])` | Right-pads by **bytes**, defaults to space. Empty pad string may panic; do not use. | `str_pad("7",3,"0")` → 700 |
| `str_repeat(texto,cantidad)` | Repeated text; negative counts as zero. | `str_repeat("a",3)` |
| `html_escape(valor)` | Escapes &, <, >, and quotes for HTML; null gives "". | `html_escape("<b>")` |
| `md5(texto)`, `sha1(texto)`, `sha256(texto)` | Hexadecimal byte digest; not encryption or password hashing. | `sha256("dato")` |
| `base64_encode(texto)` | Base64 text, no cryptographic secrecy. | `base64_encode("a")` → YQ== |
| `base64_decode(texto)` | Byte string or false if invalid. | `base64_decode("YQ==")` |

For passwords use the Auth contract. For user-perceived characters
such as composite emojis, see [text units](COLECCIONES.md).

## Numbers and time

| Signature | Result and limits | Example |
|---|---|---|
| `round(numero,[precision])` | Rounded float; default precision 0. Does not process decimal.Decimal. | `round(1.25,1)` |
| `floor(numero)`, `ceil(numero)` | Integer downward/upward; accept float/int, not decimal. | `floor(1.9)` → 1 |
| `abs(numero)` | Magnitude for int/float. Minimum int64 may overflow without a diagnostic here. | `abs(-2)` |
| `min(valores...)`, `max(valores...)` | Also accept a non-empty array; no arguments gives null. A single empty array is returned as-is. | `max([1,3,2])` |
| `rand([min,max])` | Inclusive integer; without two bounds uses 0..MaxInt32. max<=min returns min. Non-cryptographic. | `rand(1,6)` |
| `time()` | Unix seconds as integer. | `time()` |
| `microtime([comoFloat])` | With true, float in seconds; defaults to string "fraction seconds". | `microtime(true)` |
| `date([formato,timestamp])` | Text; defaults to Y-m-d H:i:s and current time. Timestamp accepts number, string, or host time.Time value. | `date("Y-m-d",0)` |
| `now([formato])` | Current date text; same default format. | `now("H:i:s")` |
| `strtotime(texto,[base])` | Unix integer or null if unrecognized. Limited parser for dates and offsets. | `strtotime("+1 day",0)` |
| `sleep(segundos)`, `usleep(microsegundos)` | Block current task; returns null. sleep accepts float fractions. | `sleep(0.01)` |

Supported date tokens and relative expressions are implemented in
[date_utils.go](../../pkg/core/date_utils.go). Not all of PHP's date
grammar is included. Time zone depends on the process; avoid local date
outputs in tests that must be identical on any machine.

## Files, serialization, and processes

| Signature | Result and errors | Example |
|---|---|---|
| `file_exists(ruta)` | Bool; includes directories; false on stat error. | `file_exists("datos.json")` |
| `is_dir(ruta)`, `is_file(ruta)` | Bool; is_file means existing and not a directory. | `is_dir("storage")` |
| `file_get_contents(ruta)` | Byte string or null on failure. Local files only. | `file_get_contents("datos.json")` |
| `file_put_contents(ruta,texto)` | Overwrites/creates file; true or false. Does not create parent directories. | `file_put_contents("nota.txt","Hola")` |
| `unlink(ruta)`, `file_delete(ruta)` | Deletes file or empty directory; success bool. | `unlink("nota.txt")` |
| `mkdir(ruta)` | Creates directories including parents; bool. | `mkdir("storage/informes")` |
| `json_encode(valor)` | Indented JSON via JsonEncode; "" on error. | `json_encode({"ok":true})` |
| `json_decode(texto)` | Native value or null on error (and JSON null). Numbers decode to float. | `json_decode("[1,2]")` |
| `json_verify(texto)` | Bool for JSON syntax, not business structure. | `json_verify("{}")` |
| `toon_encode(valor)` | Simplified textual record format; not binary or full escaping. | `toon_encode([{"a":"b"}])` |
| `toon_decode(texto)` | Array of records from the subset or null. String values. | Use compatible encoder output. |
| `toon_verify(texto)` | Limited header/structure check; not cryptographic integrity. | Do not use to validate hostile data. |
| `hive_read_box(ruta)` | Array of entries or null; prints error. Partial Hive format reader. | Requires compatible Hive file. |
| `run(ruta,args...)` | Executes .py with python or .php with php; combined output. Requires ALLOW_SYSTEM_RUN=true and installed binary. Failure prints and returns output/""; does not always throw. | `run("script.py","dato")` |

Paths are relative to the working directory. Operations have no
file sandbox. The complete example is in [purchasing report](PROYECTO_CONSOLA.md).

## Web context, testing, and concurrency

| Signature | Contract and availability |
|---|---|
| `env(clave,[default])`, `config(clave,[default])` | Queries r.Env; returns string or arbitrary default if missing/empty. Does not re-read .env. |
| `view(nombre,[datos])` | Delegates to View::render; project template context. |
| `json(datos,[status])` | Delegates to Response::json and returns WebResponse; **not string serialization**. |
| `response(cuerpo,[status,mime,headers])` | Delegates to Response::raw. |
| `redirect(url,[status])`, `back()` | Redirect WebResponse; back uses request context. |
| `request([clave,default])` | Without key gets Request::all; with key Request::input. |
| `session([clave])` | Without key session object or null; with key Session::get. Does not provide a console session. |
| `__(clave)` | Translation using current locale and i18n manager. |
| `csrf_field()` | HTML for _token field using session; empty token without one. |
| `async { instrucciones }` | Syntax that creates a Future and runs a closure in a fork/goroutine. |
| `await(future)` | Waits for and returns result; propagates failure. |
| `make_chan([capacidad])` | Channel; capacity 0 synchronizes sender and receiver. |
| `send(channel,valor)` | Sends; may block, closed channel causes failure. Also channel << valor. |
| `recv(channel)` | Receives; blocks until data/close; drained closed channel yields null. |
| `close(channel)` | Closes; closing twice or sending afterwards causes failure. |
| `Ok(valor)`, `Err(error)` | Constructors of the `Result<T, E>` type for explicit success or failure return. |
| `test(nombre, closure)`, `it(nombre, closure)` | Registers and executes a test case in `*_test.joss` files (`joss test`). |
| `assert(cond,[msg])`, `assertTrue(...)`, `assertFalse(...)`, `assertEqual(a,b)`, `assertNotEqual(a,b)`, `assertNull(v)`, `assertNotNull(v)`, `assertThrows(closure)` | Built-in native assertions for automated testing with `joss test`. |

See [HTTP and native classes](MODULOS_NATIVOS.md), [views](VISTAS.md), and
[concurrency](CONCURRENCIA.md) for examples with the required context.

## Executable utility example

<!-- joss-run: ["Ana", "3", "a-b", "true", "Hola", "2"] -->
```joss
print(ucfirst(trim(" ana ")))
print(strlen("sol"))
print(join("-", ["a", "b"]))
print(array_key_exists("id", {"id": null}))
print(base64_decode(base64_encode("Hola")))
print(floor(2.8))
```

## Differences between metadata and implementation

Incomplete published return types have been observed: boolval advertises int but
returns bool; json advertises string but returns WebResponse; microtime
advertises float although without true it returns string; strpos and base64_decode can
return false; array_merge also returns map. Do not blame those mismatches
on the user or force incompatible annotations just to satisfy the catalog.
The [audit report](DOCUMENTATION_AUDIT.md) records this debt.

Sources: [catalog](../../pkg/core/builtins.go),
[arrays and conversions](../../pkg/core/builtins_array.go),
[strings](../../pkg/core/builtins_string.go), [I/O](../../pkg/core/builtins_io.go),
[time](../../pkg/core/builtins_date.go), [async](../../pkg/core/builtins_async.go).

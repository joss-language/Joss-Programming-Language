# Global functions

[Index](README.md) · Before: [functions](FUNCIONES.md) · [Native classes](MODULOS_NATIVOS.md)

These functions are integrated into the runtime, without imports or installation.
The signature on this page describes the usage supported by the dispatcher; not all
the parameters are published to the analyzer. `[x]` indicates an argument
optional and `...` various arguments. They are not characters you should copy.

Families share examples at the end. Function aliases like
`doubleval` still exist; this does not restore the deleted source type
`double`. The returns declared for tooling can be consulted in the
[generated catalog](CATALOGO_NATIVO.md); The runtime result is described below.

## Conversion and type queries

| Signature | Result and limits | Example |
|---|---|---|
| `intval(valor)` | Whole; truncate float/decimal, bool set to 0/1; invalid or absent gives 0. | `intval("12")` → 12 |
| `floatval(valor)`, `doubleval(valor)` | Approximate float; invalid gives 0. | `floatval("1.5")` |
| `decimal([valor])` | Decimal from number/text/bool; invalid or null gives zero. Textual suffixes m/M/d/D are trimmed. | `decimal("1.25")` |
| `strval(valor)` | Text by Go format, null gives empty text. It is not JSON. | `strval(12)` |
| `boolval(valor)` | Bool according to runtime truth. Zero float and empty map have differences from other languages. | `boolval("")` → false |
| `is_numeric(valor)` | Bool: number or text interpretable as float/decimal. It does not validate a domain such as “age”. | `is_numeric("12")` |
| `is_int(valor)`, `is_integer(valor)` | Bool; integer, without converting strings. | `is_int("12")` → false |
| `is_float(valor)`, `is_double(valor)` | Bool; float, unconverted. | `is_float(1.5)` |
| `is_decimal(valor)` | Bool; concrete decimal. | `is_decimal(1m)` |
| `is_string(valor)` | Bool; specific text. | `is_string("a")` |
| `is_array(valor)` | Bool; array/slice, not map. | `is_array([1])` |
| `is_null(valor)` | Bool; absence of value; no argument also true. | `is_null(nil)` |
| `isset(expresiones...)` | Special form of the parser: existence query without error due to missing variable. A variable bound to null counts as existing; It does not serve as a general check for map keys. | `isset($nombre)` |
| `empty(expresion)` | Special form: true if it does not exist or if the value is false according to runtime rules. | `empty($ausente)` |

Explicit conversions are permissive; They are not a substitute for validation.
For exact conditions compare against `0`, `null` or `""` depending on what
you need See [types](SISTEMA_TIPOS.md) and [syntax](SINTAXIS.md).

## Arrays and maps

| Signature | Result, mutation and errors | Example |
|---|---|---|
| `len(valor)`, `count(valor)` | array/map length; string bytes; other values ​​give 0. | `count([2,3])` → 2 |
| `keys(map)`, `array_keys(map)` | Array of keys without guaranteed order; invalid gives []. | `keys({"a":1})` |
| `values(map)`, `array_values(map)` | Array of values ​​without guaranteed order; invalid gives []. | `values({"a":1})` |
| `explode(separador, texto)` | Array of segments; requires two strings, invalid gives null. | `explode(",", "a,b")` |
| `append(array, elemento)` | Return expanded array; reassigns the result. Invalid gives null. | `$a = append($a, 2)` |
| `merge(array1, array2)` | New container with both sequences; invalid gives null. | `merge([1],[2])` |
| `array_merge(primero, otros...)` | Concatenated array or combined map according to first argument; last keys win. Ignore following of other types; invalid gives []. | `array_merge({"a":1},{"a":2})` |
| `array_push(array, elementos...)` | Expanded Array; does not return length. Reassign. Invalid gives null. | `$a = array_push($a,2,3)` |
| `end(array)`, `array_pop(array)` | Last element or null. **They do not reduce the array**. | `array_pop([1,2])` → 2 |
| `array_shift(array)` | First element or null. **Does not reduce the array**. | `array_shift([1,2])` → 1 |
| `array_slice(array,inicio,[longitud])` | Surface view; negatives relative to the end; out gives []. Share storage. | `array_slice([1,2,3],1)` |
| `array_unique(array)` | New array, equality by textual representation; you can merge values ​​of different types. | `array_unique([1,1,2])` |
| `array_reverse(array)` | new array inverted; invalid gives []. | `array_reverse([1,2])` |
| `array_column(array,clave)` | Values ​​of that key in map elements; omits other missing elements/keys. | `array_column([{"id":1}],"id")` |
| `in_array(valor,array)` | Bool; in Joss arrays it supports deep or textual equality. | `in_array(1,["1"])` → true |
| `array_key_exists(clave,map)` | existence bool; not to be confused with non-null value. | `array_key_exists("a",{"a":null})` |

## Text and formatting

| Signature | Result and limits | Example |
|---|---|---|
| `print(valores...)`, `echo(valores...)` | They print each argument with a line break; return null. | `print("Hola")` |
| `printf(formato,valores...)` | Go format, no implicit jump; return null. | `printf("%s: %d\\n","Edad",20)` |
| `strlen(texto)` | Number of Unicode points; null gives 0. | `strlen("é")` → 1 |
| `str_contains(texto,parte)`, `contains(texto,parte)` | Bool; arguments are converted to text. | `contains("casa","as")` |
| `str_starts_with(texto,prefijo)`, `starts_with(...)` | Prefix bool. | `starts_with("abc","a")` |
| `str_ends_with(texto,sufijo)`, `ends_with(...)` | Postfix bool. | `ends_with("abc","c")` |
| `str_replace(buscar,reemplazo,texto)` | Replaces all matches. The order differs from Str::replace. | `str_replace("a","o","casa")` |
| `strtolower(texto)`, `to_lower(texto)` | Unicode lowercase. | `to_lower("HOLA")` |
| `strtoupper(texto)`, `to_upper(texto)` | Unicode uppercase. | `to_upper("hola")` |
| `ucfirst(texto)`, `lcfirst(texto)` | Change Unicode first point to upper/lower case. | `ucfirst("hola")` |
| `ucwords(texto)` | Capitalization according to Go strings.Title; not linguistic analysis. | `ucwords("hola mundo")` |
| `trim(texto,[conjunto])` | Removes Unicode spaces or characters from the set at ends. | `trim(" hola ")` |
| `ltrim(texto,[conjunto])`, `rtrim(texto,[conjunto])` | Trim at one end; default ASCII spaces. | `ltrim(" hola")` |
| `substr(texto,inicio,[longitud])` | Unicode Points; negatives relative to the end; out gives "". | `substr("abc",-2)` → bc |
| `strpos(texto,parte)` | Position in Unicode points or false if missing. Zero is a valid match. | `strpos("abc","a")` → 0 |
| `implode(separador,array)`, `join(separador,array)` | Convert elements to text and join; invalid gives "". | `join("-",[1,2])` |
| `str_pad(texto,longitud,[relleno])` | Adds padding to the right by **bytes**, default space. Empty filling can cause panic; do not use. | `str_pad("7",3,"0")` → 700 |
| `str_repeat(texto,cantidad)` | Repeated text; Negative equals zero. | `str_repeat("a",3)` |
| `html_escape(valor)` | Escapes &, <, > and quotes for HTML; null gives "". | `html_escape("<b>")` |
| `md5(texto)`, `sha1(texto)`, `sha256(texto)` | hexadecimal digest of bytes; no encryption or password hashing. | `sha256("dato")` |
| `base64_encode(texto)` | Base64 text, without cryptographic secret. | `base64_encode("a")` → YQ== |
| `base64_decode(texto)` | Byte string or false if invalid. | `base64_decode("YQ==")` |

For passwords use the Auth contract. For perceived characters
as compound emojis, see [text units](COLECCIONES.md).

## Numbers and time

| Signature | Result and limits | Example |
|---|---|---|
| `round(numero,[precision])` | Rounded float; Default precision 0. Does not process decimal.Decimal. | `round(1.25,1)` |
| `floor(numero)`, `ceil(numero)` | Integer down/up; They accept float/int, not decimal. | `floor(1.9)` → 1 |
| `abs(numero)` | int/float magnitude. The minimum int64 can overflow without diagnostics here. | `abs(-2)` |
| `min(valores...)`, `max(valores...)` | They also accept a non-empty array; no null arguments. A single empty array is returned as such. | `max([1,3,2])` |
| `rand([min,max])` | inclusive integer; without two limits use 0..MaxInt32. max<=min returns min. Not cryptographic. | `rand(1,6)` |
| `time()` | Unix seconds as integer. | `time()` |
| `microtime([comoFloat])` | With true, float in seconds; by default text "fraction seconds". | `microtime(true)` |
| `date([formato,timestamp])` | Text; by default Y-m-d H:i:s and current time. Timestamp supports number, text, or host time.Time value. | `date("Y-m-d",0)` |
| `now([formato])` | Current date text; same default format. | `now("H:i:s")` |
| `strtotime(texto,[base])` | Unix integer or null if the text is not recognized. Limited parser of dates and displacements. | `strtotime("+1 day",0)` |
| `sleep(segundos)`, `usleep(microsegundos)` | They lock the current task; return null. sleep supports float fractions. | `sleep(0.01)` |

The supported date tokens and relative expressions are implemented in
[date_utils.go](../../pkg/core/date_utils.go). Not all grammar is incorporated
of PHP dates. The time zone depends on the process; avoid date departures
local in tests that must be identical on any machine.

## Files, serialization and processes

| Signature | Result and errors | Example |
|---|---|---|
| `file_exists(ruta)` | Bool; includes directories; false on stat error. | `file_exists("datos.json")` |
| `is_dir(ruta)`, `is_file(ruta)` | Bool; is_file means existing and not directory. | `is_dir("storage")` |
| `file_get_contents(ruta)` | Byte string or null on failure. Local files only. | `file_get_contents("datos.json")` |
| `file_put_contents(ruta,texto)` | Overwrite/create file; true or false. Don't create parents. | `file_put_contents("nota.txt","Hola")` |
| `unlink(ruta)`, `file_delete(ruta)` | Delete empty file or directory; success bool. | `unlink("nota.txt")` |
| `mkdir(ruta)` | Create parent including directories; bool. | `mkdir("storage/informes")` |
| `json_encode(valor)` | JSON with indentation according to JsonEncode; "" on error. | `json_encode({"ok":true})` |
| `json_decode(texto)` | Native value or null on error (also JSON null). Numbers are decoded to float. | `json_decode("[1,2]")` |
| `json_verify(texto)` | JSON syntax bool, not business structure bool. | `json_verify("{}")` |
| `toon_encode(valor)` | Simplified textual format of records; non-binary and non-complete escape. | `toon_encode([{"a":"b"}])` |
| `toon_decode(texto)` | Array of records of the subset or null. Textual values. | Use compatible encoder output. |
| `toon_verify(texto)` | Limited header/structure checking; not cryptographic integrity. | Do not use to validate hostile data. |
| `hive_read_box(ruta)` | Array of entries or null; print error. Partial Hive format reader. | Requires compatible Hive file. |
| `run(ruta,args...)` | Run .py with python or .php with php; combined output. Requires ALLOW_SYSTEM_RUN=true and executable installed. Failed print and return output/""; He doesn't always throw. | `run("script.py","dato")` |

Paths are relative to the working directory. The operations do not have
file sandbox. The complete example is at [purchase report](PROYECTO_CONSOLA.md).

## Web context and concurrency

| Signature | Contract and availability |
|---|---|
| `env(clave,[default])`, `config(clave,[default])` | Query r.Env; returns string or arbitrary default if absent/empty. It doesn't read .env again. |
| `view(nombre,[datos])` | Delegate View::render; project template context. |
| `json(datos,[status])` | Delegates Response::json and returns WebResponse; **it is not serialization to string**. |
| `response(cuerpo,[status,mime,headers])` | Delegates Response::raw. |
| `redirect(url,[status])`, `back()` | redirect WebResponse; back uses request context. |
| `request([clave,default])` | Without key you get Request::all; with key Request::input. |
| `session([clave])` | No session object key or null; with key Session::get. It does not incorporate a console session. |
| `__(clave)` | Translation using current locale and i18n manager. |
| `csrf_field()` | _token field HTML using session; without it empty token. |
| `async { instrucciones }` | Syntax that creates Future and executes a closure in fork/goroutine. |
| `await(future)` | Waits and returns result; propagates failure. |
| `make_chan([capacidad])` | Channel; capacity 0 synchronizes transmitter and receiver. |
| `send(channel,valor)` | Send; can block, closed channel causes failure. Also channel << value. |
| `recv(channel)` | Receive; blocks until data/close; exhausted closure produces null. |
| `close(channel)` | Close; closing twice or sending after fails. |

See [HTTP and native classes](MODULOS_NATIVOS.md), [views](VISTAS.md) and
[concurrency](CONCURRENCIA.md) for examples with the necessary context.

## Utilities executable example

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

Observed incomplete posted returns: boolval announces int but
return bool; json announces string but returns WebResponse; microtime
announces float although without true it returns string; strpos and base64_decode can
return false; array_merge also returns map. Don't attribute those mistakes
to the user or force incompatible annotations to satisfy the catalog.
The [audit report](DOCUMENTATION_AUDIT.md) records this debt.

Sources: [catalog](../../pkg/core/builtins.go),
[arrays and conversions](../../pkg/core/builtins_array.go),
[texts](../../pkg/core/builtins_string.go), [I/O](../../pkg/core/builtins_io.go),
[time](../../pkg/core/builtins_date.go), [async](../../pkg/core/builtins_async.go).

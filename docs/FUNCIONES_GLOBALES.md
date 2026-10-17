# Funciones globales

[Índice](README.md) · Antes: [funciones](FUNCIONES.md) · [Clases nativas](MODULOS_NATIVOS.md)

Estas funciones están integradas en el runtime (`pkg/core/builtins.go`), sin imports ni instalación.
La firma de esta página describe el uso admitido por el dispatcher; no todos
los parámetros están publicados al analizador. `[x]` indica un argumento
opcional y `...` varios argumentos. No son caracteres que debas copiar.

Las familias comparten ejemplos al final. Los aliases de funciones como
`doubleval` siguen existiendo; esto no restaura el tipo fuente eliminado
`double`. Los retornos declarados para tooling se pueden consultar en el
[catálogo generado](CATALOGO_NATIVO.md); abajo se describe el resultado runtime.

## Conversión y consultas de tipo

| Firma | Resultado y límites | Ejemplo |
|---|---|---|
| `intval(valor)` | Entero; trunca float/decimal, bool pasa a 0/1; inválido o ausente da 0. | `intval("12")` → 12 |
| `floatval(valor)`, `doubleval(valor)` | Float aproximado; inválido da 0. | `floatval("1.5")` |
| `decimal([valor])` | Decimal desde número/texto/bool; inválido o null da cero. Sufijos textuales m/M/d/D se recortan. | `decimal("1.25")` |
| `strval(valor)` | Texto por formato Go, null da texto vacío. No es JSON. | `strval(12)` |
| `boolval(valor)` | Bool según verdad runtime. Float cero y map vacío tienen diferencias respecto a otros lenguajes. | `boolval("")` → false |
| `is_numeric(valor)` | Bool: número o texto interpretable como float/decimal. No valida un dominio como “edad”. | `is_numeric("12")` |
| `is_int(valor)`, `is_integer(valor)` | Bool; entero, sin convertir strings. | `is_int("12")` → false |
| `is_float(valor)`, `is_double(valor)` | Bool; float, sin convertir. | `is_float(1.5)` |
| `is_decimal(valor)` | Bool; decimal concreto. | `is_decimal(1m)` |
| `is_string(valor)` | Bool; texto concreto. | `is_string("a")` |
| `is_array(valor)` | Bool; array/slice, no map. | `is_array([1])` |
| `is_null(valor)` | Bool; ausencia de valor; sin argumento también true. | `is_null(null)` |
| `isset(expresiones...)` | Forma especial del parser: consulta existencia sin error por variable ausente. Una variable ligada a null cuenta como existente; no sirve como comprobación general de claves map. | `isset($nombre)` |
| `empty(expresion)` | Forma especial: true si no existe o si el valor es falso según las reglas runtime. | `empty($ausente)` |

Las conversiones explícitas son permisivas; no sustituyen la validación.
Para condiciones exactas compara contra `0`, `null` o `""` según lo que
necesites. Véase [tipos](SISTEMA_TIPOS.md) y [sintaxis](SINTAXIS.md).

## Arrays, mapas y colecciones funcionales

| Firma | Resultado, mutación y errores | Ejemplo |
|---|---|---|
| `len(valor)`, `count(valor)` | Longitud array/map; bytes de string; otros valores dan 0. | `count([2,3])` → 2 |
| `keys(map)`, `array_keys(map)` | Array de claves sin orden garantizado; inválido da []. | `keys({"a":1})` |
| `values(map)`, `array_values(map)` | Array de valores sin orden garantizado; inválido da []. | `values({"a":1})` |
| `explode(separador, texto)` | Array de segmentos; requiere dos strings, inválido da null. | `explode(",", "a,b")` |
| `append(array, elemento)` | Retorna array ampliado; reasigna el resultado. Inválido da null. | `$a = append($a, 2)` |
| `merge(array1, array2)` | Nuevo contenedor con ambas secuencias; inválido da null. | `merge([1],[2])` |
| `array_merge(primero, otros...)` | Array concatenado o map combinado según primer argumento; últimas claves ganan. Ignora siguientes de otro tipo; inválido da []. | `array_merge({"a":1},{"a":2})` |
| `array_push(array, elementos...)` | Array ampliado; no devuelve longitud. Reasigna. Inválido da null. | `$a = array_push($a,2,3)` |
| `end(array)`, `array_pop(array)` | Último elemento o null. **No reducen el array**. | `array_pop([1,2])` → 2 |
| `array_shift(array)` | Primer elemento o null. **No reduce el array**. | `array_shift([1,2])` → 1 |
| `array_slice(array,inicio,[longitud])` | Vista superficial; negativos relativos al final; fuera da []. Comparte almacenamiento. | `array_slice([1,2,3],1)` |
| `array_unique(array)` | Array nuevo, igualdad por representación textual; puede fusionar valores de tipos diferentes. | `array_unique([1,1,2])` |
| `array_reverse(array)` | Array nuevo invertido; inválido da []. | `array_reverse([1,2])` |
| `array_column(array,clave)` | Valores de esa clave en elementos map; omite otros elementos/claves ausentes. | `array_column([{"id":1}],"id")` |
| `in_array(valor,array)` | Bool; en arrays Joss admite igualdad profunda o textual. | `in_array(1,["1"])` → true |
| `array_key_exists(clave,map)` | Bool de existencia; no confundir con valor no nulo. | `array_key_exists("a",{"a":null})` |
| `map(array, closure)` | Retorna un nuevo array aplicando `closure` a cada elemento. | `map([1,2], func(int $x): int { return $x * 2 })` |
| `filter(array, closure)` | Retorna un nuevo array con los elementos donde `closure` devuelva verdadero. | `filter([1,2,3], func(int $x): bool { return $x > 1 })` |
| `sum(array)` | Suma numérica de los elementos; conserva `decimal`, verifica overflow entero y rechaza mezclar `float` con `decimal` (`JOSS-ARITH-003`). | `sum([10, 20, 30])` → 60 |

## Texto y formato

| Firma | Resultado y límites | Ejemplo |
|---|---|---|
| `print(valores...)`, `echo(valores...)` | Imprimen cada argumento con salto de línea; retorno null. | `print("Hola")` |
| `printf(formato,valores...)` | Formato Go, sin salto implícito; retorno null. | `printf("%s: %d\\n","Edad",20)` |
| `strlen(texto)` | Cantidad de puntos Unicode; null da 0. | `strlen("é")` → 1 |
| `str_contains(texto,parte)`, `contains(texto,parte)` | Bool; argumentos se convierten a texto. | `contains("casa","as")` |
| `str_starts_with(texto,prefijo)`, `starts_with(...)` | Bool de prefijo. | `starts_with("abc","a")` |
| `str_ends_with(texto,sufijo)`, `ends_with(...)` | Bool de sufijo. | `ends_with("abc","c")` |
| `str_replace(buscar,reemplazo,texto)` | Sustituye todas las coincidencias. El orden difiere de Str::replace. | `str_replace("a","o","casa")` |
| `strtolower(texto)`, `to_lower(texto)` | Minúsculas Unicode. | `to_lower("HOLA")` |
| `strtoupper(texto)`, `to_upper(texto)` | Mayúsculas Unicode. | `to_upper("hola")` |
| `ucfirst(texto)`, `lcfirst(texto)` | Cambian primer punto Unicode a mayúscula/minúscula. | `ucfirst("hola")` |
| `ucwords(texto)` | Capitalización según strings.Title de Go; no análisis lingüístico. | `ucwords("hola mundo")` |
| `trim(texto,[conjunto])` | Quita espacios Unicode o caracteres del conjunto en extremos. | `trim(" hola ")` |
| `ltrim(texto,[conjunto])`, `rtrim(texto,[conjunto])` | Recorte por un extremo; predeterminado espacios ASCII. | `ltrim(" hola")` |
| `substr(texto,inicio,[longitud])` | Puntos Unicode; negativos relativos al final; fuera da "". | `substr("abc",-2)` → bc |
| `strpos(texto,parte)` | Posición en puntos Unicode o false si falta. Cero es una coincidencia válida. | `strpos("abc","a")` → 0 |
| `implode(separador,array)`, `join(separador,array)` | Convierte elementos a texto y une; inválido da "". | `join("-",[1,2])` |
| `str_pad(texto,longitud,[relleno])` | Añade relleno a la derecha por **bytes**, por defecto espacio. Relleno vacío puede provocar panic; no usar. | `str_pad("7",3,"0")` → 700 |
| `str_repeat(texto,cantidad)` | Texto repetido; negativo equivale a cero. | `str_repeat("a",3)` |
| `html_escape(valor)` | Escapa &, <, > y comillas para HTML; null da "". | `html_escape("<b>")` |
| `md5(texto)`, `sha1(texto)`, `sha256(texto)` | Digest hexadecimal de bytes; no cifrado ni hash de contraseña. | `sha256("dato")` |
| `base64_encode(texto)` | Texto Base64, sin secreto criptográfico. | `base64_encode("a")` → YQ== |
| `base64_decode(texto)` | String de bytes o false si inválido. | `base64_decode("YQ==")` |

Para contraseñas utiliza el contrato de Auth. Para caracteres percibidos
como emojis compuestos, consulta [unidades de texto](COLECCIONES.md).

## Números y tiempo

| Firma | Resultado y límites | Ejemplo |
|---|---|---|
| `round(numero,[precision])` | Float redondeado; precision predeterminada 0. No procesa decimal.Decimal. | `round(1.25,1)` |
| `floor(numero)`, `ceil(numero)` | Entero hacia abajo/arriba; aceptan float/int, no decimal. | `floor(1.9)` → 1 |
| `abs(numero)` | Magnitud int/float. El mínimo int64 puede desbordar sin diagnóstico aquí. | `abs(-2)` |
| `min(valores...)`, `max(valores...)` | También aceptan un array no vacío; sin argumentos null. Un único array vacío se devuelve como tal. | `max([1,3,2])` |
| `rand([min,max])` | Entero inclusivo; sin dos límites usa 0..MaxInt32. max<=min devuelve min. No criptográfico. | `rand(1,6)` |
| `time()` | Segundos Unix como entero. | `time()` |
| `microtime([comoFloat])` | Con true, float en segundos; por defecto texto "fraccion segundos". | `microtime(true)` |
| `date([formato,timestamp])` | Texto; por defecto Y-m-d H:i:s y hora actual. Timestamp admite número, texto o valor time.Time del host. | `date("Y-m-d",0)` |
| `now([formato])` | Texto de fecha actual; mismo formato predeterminado. | `now("H:i:s")` |
| `strtotime(texto,[base])` | Entero Unix o null si no reconoce el texto. Parser limitado de fechas y desplazamientos. | `strtotime("+1 day",0)` |
| `sleep(segundos)`, `usleep(microsegundos)` | Bloquean la tarea actual; retorno null. sleep admite fracciones float. | `sleep(0.01)` |

Los tokens de fecha y expresiones relativas admitidos están implementados en
[date_utils.go](../pkg/core/date_utils.go). No se incorpora toda la gramática
de fechas de PHP. La zona horaria depende del proceso; evita salidas de fecha
local en pruebas que deban ser idénticas en cualquier máquina.

## Archivos, serialización y procesos

| Firma | Resultado y errores | Ejemplo |
|---|---|---|
| `file_exists(ruta)` | Bool; incluye directorios; false ante error de stat. | `file_exists("datos.json")` |
| `is_dir(ruta)`, `is_file(ruta)` | Bool; is_file significa existente y no directorio. | `is_dir("storage")` |
| `file_get_contents(ruta)` | String de bytes o null al fallar. Sólo archivos locales. | `file_get_contents("datos.json")` |
| `file_put_contents(ruta,texto)` | Sobrescribe/crea archivo; true o false. No crea padres. | `file_put_contents("nota.txt","Hola")` |
| `unlink(ruta)`, `file_delete(ruta)` | Elimina archivo o directorio vacío; bool de éxito. | `unlink("nota.txt")` |
| `mkdir(ruta)` | Crea directorios incluidos padres; bool. | `mkdir("storage/informes")` |
| `json_encode(valor)` | JSON con indentación según JsonEncode; "" ante error. | `json_encode({"ok":true})` |
| `json_decode(texto)` | Valor nativo o null por error (también JSON null). Números se decodifican a float. | `json_decode("[1,2]")` |
| `json_verify(texto)` | Bool de sintaxis JSON, no de estructura del negocio. | `json_verify("{}")` |
| `toon_encode(valor)` | Formato textual simplificado de registros; no binario ni escape completo. | `toon_encode([{"a":"b"}])` |
| `toon_decode(texto)` | Array de registros del subconjunto o null. Valores textuales. | Usar salida compatible del encoder. |
| `toon_verify(texto)` | Comprobación limitada de cabecera/estructura; no integridad criptográfica. | No usar para validar datos hostiles. |
| `hive_read_box(ruta)` | Array de entradas o null; imprime error. Lector parcial de formato Hive. | Requiere archivo Hive compatible. |
| `run(ruta,args...)` | Ejecuta .py con python o .php con php; salida combinada. Requiere ALLOW_SYSTEM_RUN=true y ejecutable instalado. Fallo imprime y retorna salida/""; no lanza siempre. | `run("script.py","dato")` |

Las rutas son relativas al directorio de trabajo. Las operaciones no tienen
sandbox de archivos. El ejemplo completo está en [informe de compras](PROYECTO_CONSOLA.md).

## Contexto web, pruebas y concurrencia

| Firma | Contrato y disponibilidad |
|---|---|
| `env(clave,[default])`, `config(clave,[default])` | Consulta r.Env; devuelve string o default arbitrario si ausente/vacío. No vuelve a leer .env. |
| `view(nombre,[datos])` | Delega View::render; contexto de plantillas del proyecto. |
| `json(datos,[status])` | Delega Response::json y devuelve WebResponse; **no es serialización a string**. |
| `response(cuerpo,[status,mime,headers])` | Delega Response::raw. |
| `redirect(url,[status])`, `back()` | WebResponse de redirección; back usa contexto de petición. |
| `request([clave,default])` | Sin clave obtiene Request::all; con clave Request::input. |
| `session([clave])` | Sin clave objeto de sesión o null; con clave Session::get. No incorpora una sesión en consola. |
| `__(clave)` | Traducción usando locale actual y gestor i18n. |
| `csrf_field()` | HTML del campo _token usando sesión; sin ella token vacío. |
| `async { instrucciones }` | Sintaxis que crea Future y ejecuta una closure en fork/goroutine. |
| `await(future)` | Espera y devuelve resultado; propaga fallo. |
| `make_chan([capacidad])` | Channel; capacidad 0 sincroniza emisor y receptor. |
| `send(channel,valor)` | Envía; puede bloquear, canal cerrado provoca fallo. También channel << valor. |
| `recv(channel)` | Recibe; bloquea hasta dato/cierre; cierre agotado produce null. |
| `close(channel)` | Cierra; cerrar dos veces o enviar después produce fallo. |
| `Ok(valor)`, `Err(error)` | Constructores del tipo `Result<T, E>` para retorno explícito de éxito o fallo. |
| `test(nombre, closure)`, `it(nombre, closure)` | Registra y ejecuta un caso de prueba en archivos `*_test.joss` (`joss test`). |
| `assert(cond,[msg])`, `assertTrue(...)`, `assertFalse(...)`, `assertEqual(a,b)`, `assertNotEqual(a,b)`, `assertNull(v)`, `assertNotNull(v)`, `assertThrows(closure)` | Aserciones nativas integradas para pruebas automatizadas con `joss test`. |

Consulta [HTTP y clases nativas](MODULOS_NATIVOS.md), [vistas](VISTAS.md) y
[concurrencia](CONCURRENCIA.md) para ejemplos con el contexto necesario.

## Ejemplo ejecutable de utilidades

<!-- joss-run: ["Ana", "3", "a-b", "true", "Hola", "2"] -->
```joss
print(ucfirst(trim(" ana ")))
print(strlen("sol"))
print(join("-", ["a", "b"]))
print(array_key_exists("id", {"id": null}))
print(base64_decode(base64_encode("Hola")))
print(floor(2.8))
```

## Diferencias entre metadata e implementación

Se han observado retornos publicados incompletos: boolval anuncia int pero
devuelve bool; json anuncia string pero devuelve WebResponse; microtime
anuncia float aunque sin true devuelve string; strpos y base64_decode pueden
devolver false; array_merge también devuelve map. No atribuyas esos errores
al usuario ni fuerces anotaciones incompatibles para satisfacer el catálogo.
El [informe de auditoría](DOCUMENTATION_AUDIT.md) registra esta deuda.

Fuentes: [catálogo](../pkg/core/builtins.go),
[arrays y conversiones](../pkg/core/builtins_array.go),
[textos](../pkg/core/builtins_string.go), [I/O](../pkg/core/builtins_io.go),
[tiempo](../pkg/core/builtins_date.go), [async](../pkg/core/builtins_async.go).

# Native classes and built-in services

[Index](README.md) · [Global functions](FUNCIONES_GLOBALES.md) · [Complete catalog](CATALOGO_NATIVO.md)

A native class provides operations implemented in Go. Registered when preparing
the runtime; does not require imports. A facade uses `Clase::metodo(...)`; an object
with state use `$objeto->metodo(...)`. The catalog lists **each registered name,
its published return and the source handler**. This guide explains contracts and context;
Catalog aliases inherit the contract from their main name unless indicated debt.

Parameters in square brackets are optional in the reference notation.
Many APIs return null/false or print an error instead of throwing an exception.
That the parser accepts a native call does not demonstrate correct arity: missing
parameter metadata in part of the library.

## Utilities without external services

| Class and signature | Result, errors and example |
|---|---|
| `Math::random(min,max)` | inclusive integer; requires two valid int and range; Inverted range can cause panic. Not cryptographic. Ex.: Math::random(1,6). |
| `Math::floor(n)`, `ceil(n)`, `abs(n)` | float result for convertible numeric inputs; They are not equivalent in type to the floor/ceil global helpers. |
| `Str::length(texto)` | UTF-8 bytes, 0 for invalid argument. |
| `Str::random([longitud])` | Alphanumeric text, default 16, math/rand; do not use as secret token. Negative length fails. |
| `Str::startsWith(texto,prefijo)`, `contains(texto,parte)` | Bool; requires strings. |
| `Str::substring(texto,inicio,[longitud])` | Unicode Points; negative start is set to zero. Negative length may fail. |
| `Str::indexOf(texto,parte)` | Index in Unicode points or -1. |
| `Str::trim(texto)`, `replace(texto,buscar,nuevo)` | String. The order of replace differs from str_replace. |
| `UUID::generate()`, `v4()` | Textual UUID identifier. |
| `JSON::parse(texto)`, `decode(texto)` | Decoded value or null; JSON numbers are floats, not arbitrarily precise integers. |
| `JSON::stringify(valor)`, `encode(valor)` | Compact JSON or "" on failure. |
| `Markdown::toHtml(texto)`, `readFile(ruta)` | rendered HTML; readFile requires local file. It is not a substitute for route authorization or sanitization of untrusted content. |
| `new Stack()` → `push(valor)`, `pop()`, `peek()` | Stack: last in, first out. pop does remove; void returns null. |
| `new Queue()` → `enqueue(valor)`, `dequeue()`, `peek()` | Queue: first in, first out. dequeue removes; void null. |
| `new Exception(mensaje,[codigo])` → `getMessage()`, `getCode()` | Error object with fields; default code 0. Does not itself generate a JOSS diagnostic. |<!-- joss-run: ["abc", "1", "dos", "uno"] -->
```joss
print(Str::trim(" abc "))
print(Str::indexOf("casa", "a"))
$pila = new Stack()
$pila->push("uno")
$pila->push("dos")
print($pila->pop())
print($pila->peek())
```
## HTTP outgoing

An HTTP client requests information from another server; Router responds to requests
that come to your application. Don't confuse them.

| Http Signature | Return and behavior |
|---|---|
| `get(url,[headers])`, `delete(url,[headers])` | Body string; error may look like "". Timeout 15 seconds. |
| `post/put/patch(url,[datos,headers])` | String body. Map is serialized to JSON, or form if Content-Type indicates it. |
| `head(url,[headers])`, `options(url,[headers])` | Map of first headers by name; empty in the face of certain errors. |
| `json(metodo,url,[datos,headers])` | Decoded JSON value; errors can be converted to map error.message. Timeout 30 seconds. An HTTP 4xx with valid JSON still returns that JSON. |
| `request(metodo,url,[opciones])` | status map, status_text, body, headers, success; may include json or error. success requires 2xx. |

Request options: `headers` and `query` as maps; priority body
`body`, then `json`, then `form`; `timeout` in seconds (15 if <=0);
`follow_redirects` bool, true by default. The json key is decoded
automatically only when the body starts with { or [. A network failure
gives status 0 and error, not a successful HTTP response.

Contextual Fragment: Requires a server listening at that address.<!-- joss-check: necesita servicio HTTP local -->
```joss
$respuesta = Http::request("GET", "http://127.0.0.1:8080/saludo/Ana", {"timeout": 3})
$respuesta["success"] ? {
    print($respuesta["body"])
} : {
    print("No se pudo consultar el servicio")
}
```
`Http::query` has internal code but is **not registered**. Use
`Http::request("QUERY", url, opciones)` if the server supports that method.

## Server, request and response

| Class | Contracts |
|---|---|
| `Router` | get/post/put/patch/delete/head/options/query(path,handler); any(path,handler); match(methods, path, handler); api(path,handler); ws(path,handler). Register routes; does not make outgoing requests. group(name,callback), middleware(name), registerMiddleware(name,callback), end() manage middleware. See [HTTP](CONTROLADORES.md) and [middleware](MIDDLEWARE.md). |
| `Request` | input/post(key,[default]) get combined data; all() and except(arrayClaves) return a map filtered by a specific list of internal fields. They are not validation or a list of allowed fields. |
| `Request` | file(key) → map with content or null; hasFile/hasfile(key) → bool; has(key) requires value other than null and "". |
| `Request` | cookie(key,[default]), header(key), root(), method(), isMethod/ismethod(verb), path(), url(), ip(), userAgent/useragent(), bearerToken/bearertoken(). url currently prioritizes _path; does not promise absolute URL. uri is not registered. Without context, several return default values. |
| `Response` | json(data,[status=200]), error(message,[status=400]), redirect(url,[status=302]), back(), raw(body,[status=200,mime,headers]), stream(callback), download(path,[name]). return WebResponse; download and stream are resolved when dispatching HTTP. |
| `Redirect` | to(url,[status=302]) → WebResponse. |
| `WebResponse` | with(key,value) add flash; withCookie(name,value), withHeader(name,value), status(code) mutate and return the same response. |
| `Session` | get(key), put(key,value), has(key), forget(key), all(). No session injected returns null; does not log in by calling the facade. |
| `View` | render(name,[map]), exists(name), share(key,value) or share(map). See [views](VISTAS.md). |
| `Stream` | Object received by SSE callback: send(data) or send(type, data), close(). Without a valid writer it doesn't work. |
| `WebSocket` | send(message), onMessage(callback), onClose(callback), subscribe(channel), unsubscribe(channel), publish(channel, message), subscriberCount(channel), broadcast(message), close(). Local hub to the process; [full contract](WEBSOCKETS.md). |
| `Server` | start() requests server mode from the host; It is not a new standalone listener in any context. spawn(name, command, port) launches auxiliary process and registers proxy according to configuration; needs execute permission. |
| `Middleware`, `Migration` | Registered base classes without public native methods; Framework conventions, not type system interfaces. |

In uploads use `$archivo["content"]`. For binaries use raw with MIME and
Suitable Content-Disposition, or download for file. The normal HTML body
can receive hot reload in development.

## Data, state and storage

| Class and signatures | Return, context and errors |
|---|---|
| `GranDB` | Builder and SQL operations: [full reference](MODELOS.md). Requires DB configured. || `Schema`, `Blueprint` | Table/column changes: [Schema Builder](SCHEMA_BUILDER.md). Names like integer, double, and boolean are valid SQL methods, not Joss type aliases. |
| `SQLite::open(ruta)`, `query(sql,[bindings])`, `close()` | Native SQLite connection; open/close bool, query collection or null and error printing. Maintain connection to the instance. |
| `Cache::put(clave,valor,[segundos=60])` | Global cache in process memory; true or null for invalid arguments. Non-persistent. |
| `Cache::get(clave,[default])`, `has(clave)`, `forget(clave)` | get returns value/default/null. Expired entry returns null even if default was delivered. have bool; forget true/null. |
| `Redis::connect(host,[password,db])` | Configure client; also auto-connects with REDIS_URL or REDIS_HOST. Requires external Redis. |
| `Redis::set(clave,valor,[ttl])`, `get(clave)`, `has(clave)` | Writing, reading or existence; absence/errors can produce null/false. Check serialization before assuming the retrieved type. |
| `Redis::del(clave)`, `forget(clave)`, `ttl(clave)`, `flush()` | Clear, TTL in seconds (-1 no expiration, -2 absent); flush flushes the current Redis DB. |
| `UserStorage::put(token,nombre,contenido)` | Bool; local storage/OCI selected by environment; requires configuration and internal tables. |
| `UserStorage::get(token,nombre)`, `getToFile(token,nombre,destino)`, `delete(token,nombre)` | String/null, bool and bool respectively. Token can be user with user_token. |
| `UserStorage::path([ruta])` | Local route under storage; does not download OCI objects. |
| `Zip::extract(archivo,destino)` | Bool; extract files with path checking. You can write before encountering a later error: it is not atomic operation. |

Do not use a cache as the only copy of irreplaceable information. A saved map
in Cache can still share its content: the concurrent container does not
automatically makes all stored values safe.

## System, plugins and tasks

| Signature | Contract |
|---|---|
| `System::env(clave,[default])` | Value of r.Env or default. |
| `System::Run(comando,[arrayArgs])` | Executes external process and returns output; requires ALLOW_SYSTEM_RUN=true/1. Does not evaluate Joss code. |
| `System::load_driver(ruta,[nombre])` | Bool when loading DLL/SO/dylib ABI C v1; depends on platform/build. |
| `System::driver_call(nombre,metodo,[args])` | Decoded JSON result, text or null on error. |
| `System::log(mensaje)`, `sleep(segundos)`, `now([dias])` | Log, integer wait and textual date with days offset. System::now is not formatted by the now helper. |
| `Plugin::platform()`, `path(nombre,ruta)` | os-arch platform and resource path; Invalid routes can launch. |
| `Plugin::call(nombre,metodo,[args])`, `stream(nombre,metodo,[args,callback])` | Bridge to plugin; polymorphic results and map/null error depending on route. [Plugins](PLUGINS.md). || `new Process(comando,[arrayArgs])` | Prepare process; requires permission. start() → bool; wait() → exit code or -1; kill() → bool; pid() → integer; stdin(text) → instance; stdout_chan()/stderr_chan() → channels. Drain outlets to avoid blockage. |
| `Cron::schedule(nombre,expresion,callback)` | Register homework; minute cron with limited grammar and local state. |
| `Task::on_request(nombre,intervalo,callback)` | Currently starts goroutine when calling; interval is not used. It does not promise execution for each request. |

For coordination and closure of channels read [concurrency](CONCURRENCIA.md).
External resources are not serializable like ordinary language data.

## Identity, translation and publication

Auth, AuthLoginResult, MFA and TwoFactor are explained in
[authentication](AUTENTICACION.md): need context, tables and a policy
authorization of the application.

`Lang::get(clave,[reemplazos])` return translation; `set(locale)` changes locale;
`locale()` queries current and `locales()` lists available ones. The files
language are loaded by i18n infrastructure. The translation is not obtained
from the network automatically.

`SEO::title(texto)`, `description(texto)`, `keywords(textoOArray)`,
`canonical(url)`, `og(propiedad,contenido)` and `meta(nombre,contenido)`
update metadata; `render()` produces HTML. **SEO::twitter is not registered**:
use meta for twitter names:*.

`Sitemap::add(url,[lastmod,changefreq,priority])` or `add(mapa)`, `exclude(rutaOArray)`,
`generate()`, `xsl()` generate XML/XSL. `provider(callback)` is registered,
but the handler only accepts FunctionLiteral and a source closure evaluates to
CapturedFunction: Your registration is not guaranteed. Use explicit add while
that border is corrected. See [SEO and sitemap](SEO_SITEMAP.md).

The closed list of the [catalog](CATALOGO_NATIVO.md) allows you to check which APIs
are available. The presence of a Go function, a comment or a suggestion
of the editor is not enough to make it a public method.
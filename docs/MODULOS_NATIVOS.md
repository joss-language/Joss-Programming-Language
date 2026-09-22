# Clases nativas y servicios integrados

[Índice](README.md) · [Funciones globales](FUNCIONES_GLOBALES.md) · [Catálogo completo](CATALOGO_NATIVO.md)

Una clase nativa ofrece operaciones implementadas en Go. Se registra al preparar
el runtime; no requiere imports. Una fachada usa `Clase::metodo(...)`; un objeto
con estado usa `$objeto->metodo(...)`. El catálogo enumera **cada nombre registrado,
su retorno publicado y el handler fuente**. Esta guía explica contratos y contexto;
los aliases del catálogo heredan el contrato de su nombre principal salvo deuda indicada.

Los parámetros entre corchetes son opcionales en la notación de referencia.
Muchas APIs retornan null/false o imprimen un error en vez de lanzar una excepción.
Que el analizador acepte una llamada nativa no demuestra aridad correcta: faltan
metadatos de parámetros en parte de la biblioteca.

## Utilidades sin servicios externos

| Clase y firma | Resultado, errores y ejemplo |
|---|---|
| `Math::random(min,max)` | Entero inclusivo; exige dos int y rango válido; rango invertido puede provocar panic. No criptográfico. Ej.: Math::random(1,6). |
| `Math::floor(n)`, `ceil(n)`, `abs(n)` | Resultado float para entradas numéricas convertibles; no equivalen en tipo a los helpers globales floor/ceil. |
| `Str::length(texto)` | Bytes UTF-8, 0 para argumento inválido. |
| `Str::random([longitud])` | Texto alfanumérico, 16 por defecto, math/rand; no usar como token secreto. Longitud negativa falla. |
| `Str::startsWith(texto,prefijo)`, `contains(texto,parte)` | Bool; exige strings. |
| `Str::substring(texto,inicio,[longitud])` | Puntos Unicode; inicio negativo se ajusta a cero. Longitud negativa puede fallar. |
| `Str::indexOf(texto,parte)` | Índice en puntos Unicode o -1. |
| `Str::trim(texto)`, `replace(texto,buscar,nuevo)` | String. El orden de replace difiere de str_replace. |
| `UUID::generate()`, `v4()` | Identificador UUID textual. |
| `JSON::parse(texto)`, `decode(texto)` | Valor decodificado o null; números JSON son float, no enteros arbitrariamente precisos. |
| `JSON::stringify(valor)`, `encode(valor)` | JSON compacto o "" al fallar. |
| `Markdown::toHtml(texto)`, `readFile(ruta)` | HTML renderizado; readFile requiere archivo local. No sustituye autorización sobre la ruta ni saneamiento de contenido no confiable. |
| `new Stack()` → `push(valor)`, `pop()`, `peek()` | Pila: último en entrar, primero en salir. pop sí elimina; vacío retorna null. |
| `new Queue()` → `enqueue(valor)`, `dequeue()`, `peek()` | Cola: primero en entrar, primero en salir. dequeue elimina; vacío null. |
| `new Exception(mensaje,[codigo])` → `getMessage()`, `getCode()` | Objeto de error con campos; código predeterminado 0. No genera por sí mismo un diagnóstico JOSS. |
| `FileStream::open(ruta, modo)` | Flujo de archivo binario o de texto (`read`, `write`, `seek`, `size`, `close`). |
| `StreamReader::open(fuente)` | Lector de flujos por líneas o completo (`readLine`, `readToEnd`, `close`). |
| `StreamWriter::open(destino, append)` | Escritor bufferizado de streams (`write`, `writeLine`, `flush`, `close`). |
| `Socket::tcp(host, puerto)`, `listen(...)` | Sockets de red TCP cliente/servidor (`connect`, `listen`, `accept`, `send`, `receive`, `close`). |
| `DateTime::now()`, `parse(texto)`, `create(...)` | Fechas y horas orientadas a objetos (`format`, `timestamp`, `iso`, `add`, `sub`, `diff`). |
| `DateInterval::days(n)`, `hours(n)`, ... | Intervalos de tiempo para aritmética sobre `DateTime`. |
| `new Mutex()`, `new RWMutex()`, `new WaitGroup()` | Primitivas de sincronización concurrente seguras (`lock`, `unlock`, `add`, `done`, `wait`). |


<!-- joss-run: ["abc", "1", "dos", "uno"] -->
```joss
print(Str::trim(" abc "))
print(Str::indexOf("casa", "a"))
$pila = new Stack()
$pila->push("uno")
$pila->push("dos")
print($pila->pop())
print($pila->peek())
```

## HTTP saliente

Un cliente HTTP pide información a otro servidor; Router responde a peticiones
que llegan a tu aplicación. No los confundas.

| Firma Http | Retorno y comportamiento |
|---|---|
| `get(url,[headers])`, `delete(url,[headers])` | Cuerpo string; error puede verse como "". Timeout 15 segundos. |
| `post/put/patch(url,[datos,headers])` | Cuerpo string. Map se serializa a JSON, o formulario si Content-Type lo indica. |
| `head(url,[headers])`, `options(url,[headers])` | Map de primeras cabeceras por nombre; vacío ante ciertos errores. |
| `json(metodo,url,[datos,headers])` | Valor JSON decodificado; errores pueden convertirse a map error.message. Timeout 30 segundos. Un HTTP 4xx con JSON válido sigue devolviendo ese JSON. |
| `request(metodo,url,[opciones])` | Map de status, status_text, body, headers, success; puede incluir json o error. success exige 2xx. |

Opciones de request: `headers` y `query` como mapas; cuerpo con prioridad
`body`, luego `json`, luego `form`; `timeout` en segundos (15 si <=0);
`follow_redirects` bool, true por defecto. La clave json se decodifica
automáticamente sólo cuando el cuerpo empieza con { o [. Un fallo de red
da status 0 y error, no una respuesta HTTP satisfactoria.

Fragmento contextual: requiere un servidor escuchando en esa dirección.

<!-- joss-check: necesita servicio HTTP local -->
```joss
$respuesta = Http::request("GET", "http://127.0.0.1:8080/saludo/Ana", {"timeout": 3})
$respuesta["success"] ? {
    print($respuesta["body"])
} : {
    print("No se pudo consultar el servicio")
}
```

`Http::query` tiene código interno pero **no está registrado**. Usa
`Http::request("QUERY", url, opciones)` si el servidor admite ese método.

## Servidor, petición y respuesta

| Clase | Contratos |
|---|---|
| `Router` | get/post/put/patch/delete/head/options/query(ruta,handler); any(ruta,handler); match(metodos,ruta,handler); api(ruta,handler); ws(ruta,handler). Registra rutas; no hace peticiones salientes. group(nombre,callback), middleware(nombre), registerMiddleware(nombre,callback), end() administran middleware. Véase [HTTP](CONTROLADORES.md) y [middleware](MIDDLEWARE.md). |
| `Request` | input/post(clave,[default]) obtienen datos combinados; all() y except(arrayClaves) devuelven mapa filtrado por una lista concreta de campos internos. No son validación ni lista de campos permitidos. |
| `Request` | file(clave) → map con content o null; hasFile/hasfile(clave) → bool; has(clave) exige valor distinto de null y "". |
| `Request` | cookie(clave,[default]), header(clave), root(), method(), isMethod/ismethod(verbo), path(), url(), ip(), userAgent/useragent(), bearerToken/bearertoken(). url actualmente prioriza _path; no promete URL absoluta. uri no está registrado. Sin contexto varios retornan valores por defecto. |
| `Response` | json(datos,[status=200]), error(mensaje,[status=400]), redirect(url,[status=302]), back(), raw(cuerpo,[status=200,mime,headers]), stream(callback), download(ruta,[nombre]). Retornan WebResponse; descarga y stream se resuelven al despachar HTTP. |
| `Redirect` | to(url,[status=302]) → WebResponse. |
| `WebResponse` | with(clave,valor) añade flash; withCookie(nombre,valor), withHeader(nombre,valor), status(codigo) mutan y devuelven la misma respuesta. |
| `Session` | get(clave), put(clave,valor), has(clave), forget(clave), all(). Sin sesión inyectada retorna null; no inicia sesión por llamar a la fachada. |
| `View` | render(nombre,[mapa]), exists(nombre), share(clave,valor) o share(mapa). Véase [vistas](VISTAS.md). |
| `Stream` | Objeto recibido por callback SSE: send(datos) o send(tipo,datos), close(). Sin writer válido no funciona. |
| `WebSocket` | send(mensaje), onMessage(callback), onClose(callback), subscribe(canal), unsubscribe(canal), publish(canal,mensaje), subscriberCount(canal), broadcast(mensaje), close(). Hub local al proceso; [contrato completo](WEBSOCKETS.md). |
| `Server` | start() solicita modo servidor al host; no es un nuevo listener independiente en cualquier contexto. spawn(nombre,comando,puerto) lanza proceso auxiliar y registra proxy según configuración; necesita permiso de ejecución. |
| `Middleware`, `Migration` | Clases base registradas sin métodos nativos públicos; convenciones del framework, no interfaces del sistema de tipos. |

En uploads usa `$archivo["content"]`. Para binarios utiliza raw con MIME y
Content-Disposition adecuados, o download para archivo. El cuerpo HTML normal
puede recibir hot reload en desarrollo.

## Datos, estado y almacenamiento

| Clase y firmas | Retorno, contexto y errores |
|---|---|
| `GranDB` | Builder y operaciones SQL: [referencia completa](MODELOS.md). Requiere DB configurada. |
| `Schema`, `Blueprint` | Cambios de tablas/columnas: [Schema Builder](SCHEMA_BUILDER.md). Nombres como integer, double y boolean son métodos SQL válidos, no aliases de tipos Joss. |
| `SQLite::open(ruta)`, `query(sql,[bindings])`, `close()` | Conexión SQLite nativa; open/close bool, query colección o null e impresión de error. Conserva conexión en la instancia. |
| `Cache::put(clave,valor,[segundos=60])` | Cache global en memoria del proceso; true o null por argumentos inválidos. No persistente. |
| `Cache::get(clave,[default])`, `has(clave)`, `forget(clave)` | get retorna valor/default/null. Entrada expirada retorna null aun si se entregó default. has bool; forget true/null. |
| `Redis::connect(host,[password,db])` | Configura cliente; también auto-conecta con REDIS_URL o REDIS_HOST. Requiere Redis externo. |
| `Redis::set(clave,valor,[ttl])`, `get(clave)`, `has(clave)` | Escritura, lectura o existencia; ausencia/errores pueden producir null/false. Revisa serialización antes de asumir el tipo recuperado. |
| `Redis::del(clave)`, `forget(clave)`, `ttl(clave)`, `flush()` | Borrado, TTL en segundos (-1 sin expiración, -2 ausente); flush vacía la DB Redis actual. |
| `UserStorage::put(token,nombre,contenido)` | Bool; almacenamiento local/OCI seleccionado por entorno; requiere configuración y tablas internas. |
| `UserStorage::get(token,nombre)`, `getToFile(token,nombre,destino)`, `delete(token,nombre)` | String/null, bool y bool respectivamente. Token puede ser usuario con user_token. |
| `UserStorage::path([ruta])` | Ruta local bajo storage; no descarga objetos OCI. |
| `Zip::extract(archivo,destino)` | Bool; extrae archivos con comprobación de rutas. Puede escribir antes de encontrar un error posterior: no es operación atómica. |

No uses una cache como única copia de información irremplazable. Un map guardado
en Cache puede seguir compartiendo su contenido: el contenedor concurrente no
vuelve automáticamente seguros todos los valores almacenados.

## Sistema, plugins y tareas

| Firma | Contrato |
|---|---|
| `System::env(clave,[default])` | Valor de r.Env o default. |
| `System::Run(comando,[arrayArgs])` | Ejecuta proceso externo y devuelve salida; exige ALLOW_SYSTEM_RUN=true/1. No evalúa código Joss. |
| `System::load_driver(ruta,[nombre])` | Bool al cargar DLL/SO/dylib ABI C v1; depende de plataforma/build. |
| `System::driver_call(nombre,metodo,[args])` | Resultado JSON decodificado, texto o null ante error. |
| `System::log(mensaje)`, `sleep(segundos)`, `now([dias])` | Log, espera entera y fecha textual con desplazamiento de días. System::now no recibe el formato del helper now. |
| `Plugin::platform()`, `path(nombre,ruta)` | Plataforma os-arch y ruta de recurso; rutas inválidas pueden lanzar. |
| `Plugin::call(nombre,metodo,[args])`, `stream(nombre,metodo,[args,callback])` | Puente al plugin; resultados polimórficos y error map/null según ruta. [Plugins](PLUGINS.md). |
| `new Process(comando,[arrayArgs])` | Prepara proceso; requiere permiso. start() → bool; wait() → exit code o -1; kill() → bool; pid() → entero; stdin(texto) → instancia; stdout_chan()/stderr_chan() → channels. Drena salidas para evitar bloqueo. |
| `Cron::schedule(nombre,expresion,callback)` | Registra tarea; cron de minutos con gramática limitada y estado local. |
| `Task::on_request(nombre,intervalo,callback)` | Actualmente inicia goroutine al llamar; intervalo no se utiliza. No promete ejecución por cada petición. |

Para coordinación y cierre de canales lee [concurrencia](CONCURRENCIA.md).
Los recursos externos no son serializables como datos ordinarios del lenguaje.

## Identidad, traducción y publicación

Auth, AuthLoginResult, MFA y TwoFactor se explican en
[autenticación](AUTENTICACION.md): necesitan contexto, tablas y una política de
autorización de la aplicación.

`Lang::get(clave,[reemplazos])` devuelve traducción; `set(locale)` cambia locale;
`locale()` consulta actual y `locales()` enumera disponibles. Los archivos
de idioma se cargan por infraestructura de i18n. La traducción no se obtiene
de la red automáticamente.

`SEO::title(texto)`, `description(texto)`, `keywords(textoOArray)`,
`canonical(url)`, `og(propiedad,contenido)` y `meta(nombre,contenido)`
actualizan metadatos; `render()` produce HTML. **SEO::twitter no está registrado**:
usa meta para nombres twitter:*.

`Sitemap::add(url,[lastmod,changefreq,priority])` o `add(mapa)`, `exclude(rutaOArray)`,
`generate()`, `xsl()` generan XML/XSL. `provider(callback)` registra un proveedor
dinámico de URLs. Véase [SEO y sitemap](SEO_SITEMAP.md).

`IndexNow::key([clave])`, `enabled()`, `keyLocation([base])` y `submit(urls,[clave])`
permiten indexación instantánea en tiempo real hacia Bing y Yandex. El comando
`joss indexnow generate` configura la clave automáticamente en el entorno.

La lista cerrada del [catálogo](CATALOGO_NATIVO.md) permite comprobar qué APIs
están disponibles. La presencia de una función Go, un comentario o una sugerencia
del editor no basta para convertirla en método público.

# Drivers and HTTP

[Index](README.md) · Before: [web project](PROYECTO_WEB.md) · After: [middleware](MIDDLEWARE.md)

An HTTP request is a message that the browser sends to the server. Includes a method (GET to query, POST to submit changes), a URL, and optional data. The response contains a state, headers and a body. The examples on this page are fragments of a web application: they require the tables, views and controllers mentioned.

A controller is a Joss class. The dispatcher resolves `Controller@method` and also route closures.

```joss
public class ProductController {
    public func index() {
        $products = GranDB::table("products")->get()
        return view("products.index", {"products": $products})
    }

    public func store() {
        $name = request("name")
        (empty($name)) ? {
            return json({"error": "El nombre es obligatorio"}, 422)
        } : {}

        GranDB::table("products")->insert({"name": $name})
        return redirect("/products")
    }
}
```
## Domain and Recursive Loading Subdirectories

Joss automatically scans and preloads all drivers organized within the subfolder tree of `app/controllers/` (for example, `app/controllers/web/`, `app/controllers/auth/`, `app/controllers/api/`, `app/controllers/admin/`).

When using the CLI builder:
```bash
joss make:controller admin/DashboardController
```
The engine generates the file at `app/controllers/admin/DashboardController.joss` sanitizing the class name as `class DashboardController`, keeping the code clean and free of invalid path prefixes.

## Routes

```joss
// Verbos individuales
Router::get("/products", "ProductController@index")
Router::post("/products", "ProductController@store")
Router::put("/products/{id}", "ProductController@update")
Router::patch("/products/{id}", "ProductController@patch")
Router::delete("/products/{id}", "ProductController@destroy")
Router::query("/search", "SearchController@query")

// Captura de todos los verbos HTTP (estilo Laravel)
Router::any("/login", "AuthController@showLogin@doLogin")

// Match explícito para múltiples verbos
Router::match("GET|POST", "/contact", "ContactController@show@submit")

// Closures
Router::get("/sound/{id}", func(string $id) {
    return Redirect::to("https://example.com/" . $id, 302)
})
```

The `{name}` parameters are injected into HTTP handlers. WebSocket routes also support them (`Router::ws`); there `$ws` is the first argument and the parameters are still in order.

## Native HTTP Client (`Http`)

The language includes the general-purpose native class `Http` for making external requests:

```joss
// 1. Peticiones directas (GET, POST, PUT, PATCH, DELETE, QUERY, HEAD, OPTIONS)
$body = Http::get("https://api.github.com/zen")
$res = Http::post("https://api.ejemplo.com/item", JSON::stringify({"name": "nuevo"}), {"Authorization": "Bearer TOKEN"})
$queryResult = Http::request("QUERY", "https://api.example.com/search", {"json": {"filter": "active"}})

// 2. Cliente JSON inteligente (serializa y deserializa datos automáticamente)
$data = Http::json("GET", "https://api.github.com/users/octocat")
$nombre = $data["name"]

// 3. Petición universal hiper-configurable (Http::request)
$response = Http::request("POST", "https://api.ejemplo.com/v1/resource", {
    "query": { "page": "1" },
    "headers": { "Accept": "application/json" },
    "json": { "status": "active" },
    "timeout": 10,
    "follow_redirects": true
})

($response["success"]) ? {
    $code = $response["status"]
    $json = $response["json"]
} : {}
```

##Request

- `input()` and `post()` read the combined map of the request.
- `all()` return public fields; `except([...])` excludes keys.
- `header()`, `cookie()` and `root()` query HTTP metadata.
- `file()` returns a map; The uploaded content is at `content`.

## Response

- `json($data, $status=200)`.
- `error($message, $status=400)` returns `{"error": ...}`.
- `redirect($url)` and `back()`.
- `raw($content, $status=200, $mime="text/plain", $headers={})`.
- `stream($callback)` for SSE.

An answer supports `->with()`, `->withCookie()`, `->withHeader()` and `->status()`. For binaries use `raw`; a normal HTML string can receive the hot reload script during development.

# Drivers e HTTP

[Índice](README.md) · Antes: [projeto web](PROYECTO_WEB.md) · Depois: [middleware](MIDDLEWARE.md)

Uma solicitação HTTP é uma mensagem que o navegador envia ao servidor. Inclui um método (GET para consultar, POST para enviar alterações), uma URL e dados opcionais. A resposta contém um estado, cabeçalhos e um corpo. Os exemplos nesta página são fragmentos de uma aplicação web: eles requerem as tabelas, visualizações e controladores mencionados.

Um controlador é uma classe Joss. O despachante resolve `Controller@method` e também fecha rotas.```joss
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
## Domínio e subdiretórios de carregamento recursivo

Joss verifica e pré-carrega automaticamente todos os controladores organizados na árvore de subpastas `app/controllers/` (por exemplo, `app/controllers/web/`, `app/controllers/auth/`, `app/controllers/api/`, `app/controllers/admin/`).

Ao usar o construtor CLI:```bash
joss make:controller admin/DashboardController
```
O mecanismo gera o arquivo em `app/controllers/admin/DashboardController.joss` limpando o nome da classe como `class DashboardController`, mantendo o código limpo e livre de prefixos de caminho inválidos.

## Rotas```joss
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
Os parâmetros `{name}` são injetados em manipuladores HTTP. As rotas WebSocket também as suportam (`Router::ws`); lá `$ws` é o primeiro argumento e os parâmetros seguem em ordem.

## Cliente HTTP nativo (`Http`)

A linguagem inclui a classe nativa `Http` de uso geral para fazer solicitações externas:```joss
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
##Solicitar

- `input()` e `post()` leem o mapa combinado da solicitação.
- `all()` retorna campos públicos; `except([...])` exclui chaves.
- `header()`, `cookie()` e `root()` consultam metadados HTTP.
- `file()` retorna um mapa; o conteúdo enviado está em `content`.

## Resposta

- `json($data, $status=200)`.
- `error($message, $status=400)` retorna `{"error": ...}`.
- `redirect($url)` e `back()`.
- `raw($content, $status=200, $mime="text/plain", $headers={})`.
- `stream($callback)` para SSE.

Uma resposta suporta `->with()`, `->withCookie()`, `->withHeader()` e `->status()`. Para binários use `raw`; uma string HTML normal pode receber o script de recarga a quente durante o desenvolvimento.
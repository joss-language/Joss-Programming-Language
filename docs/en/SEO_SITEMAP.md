# SEO and Sitemap

[Index](README.md) · Before: [web project](PROYECTO_WEB.md) · [Native classes](MODULOS_NATIVOS.md)

SEO prepares tags for search engines and networks. A sitemap lists URLs that a
search engine can visit; it does not by itself improve the quality or authorization of a page.

```joss
SEO::title("Productos")
SEO::description("Catálogo")
SEO::keywords(["joss", "productos"])
SEO::canonical("https://example.com/products")
SEO::og("image", "https://example.com/cover.png")
$tags = SEO::render()
```

There is also `SEO::meta($name, $content)`. The output escapes HTML attributes and adds a default Twitter card.

## Dynamic Sitemap Generation (`/sitemap.xml`)

`/sitemap.xml` is generated live on every request with a beautiful interactive XSL interface (`/sitemap.xsl`). Does not write physical files to disk.

### 1. Manual Static Entries
```joss
Sitemap::add("/docs", "2026-07-15", "weekly", 0.8)

// O con mapas/arrays asociativos:
Sitemap::add({
    "url": "/contacto",
    "changefreq": "monthly",
    "priority": 0.6
})
```

### 2. Dynamic providers

`Sitemap::provider` accepts a closure or function that returns a list of sitemap entries, dynamically adding records from the database:

```joss
Sitemap::provider(func() {
    return [
        {"url": "/blog/mi-post", "changefreq": "daily", "priority": 0.9}
    ]
})
```

### 3. Route Exclusions
```joss
Sitemap::exclude([
    "/api/*",
    "/admin/*",
    "/checkout/*"
])
```

The base URL uses the current request, then `APP_URL` and finally `http://localhost`. Behind a proxy correctly configure `Host` and `X-Forwarded-Proto`.

`SEO::twitter` is not registered. `SEO::render` already adds a default card;
use `SEO::meta("twitter:...", valor)` for additional metadata.

## IndexNow (Instant Indexing)

IndexNow allows notifying search engines (Bing, Yandex, Seznam, Naver) in real time about changes or new pages without waiting for a crawler to crawl the sitemap.

<!-- joss-check: Requiere servidor HTTP activo y clave INDEXNOW_KEY -->
```joss
// Generar la clave con CLI: joss indexnow generate
// O enviar URLs a IndexNow directamente:
IndexNow::submit("/blog/mi-nuevo-articulo")

// Notificar múltiples URLs por lote:
IndexNow::submit([
    "/blog/post-1",
    "/blog/post-2"
])
```

When starting the server (`joss server start`, `joss program start`, etc.), the runtime detects `INDEXNOW_KEY` in `.env` and the Joss HTTP server automatically responds internally on `GET /{key}.txt` with the key in plain text, validating your domain ownership instantly. You do not need to create physical files on disk or define routes in `routes.joss`.
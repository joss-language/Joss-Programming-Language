# SEO and Sitemap

[Index](README.md) · Before: [web project](PROYECTO_WEB.md) · [Native classes](MODULOS_NATIVOS.md)

SEO prepares tags for search engines and networks. A sitemap lists URLs that a
search engine can visit; it does not by itself improve the quality or authorization of a page.```joss
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

### 1. Manual Static Entries```joss
Sitemap::add("/docs", "2026-07-15", "weekly", 0.8)

// O con mapas/arrays asociativos:
Sitemap::add({
    "url": "/contacto",
    "changefreq": "monthly",
    "priority": 0.6
})
```
### 2. Dynamic providers

`Sitemap::provider` is registered, but the handler accepts a
`parser.FunctionLiteral` while an evaluated closure arrives as
`CapturedFunction`. Its source operation is not guaranteed. Consult the
records and calls `Sitemap::add` for each URL while the border is corrected.

### 3. Route Exclusions```joss
Sitemap::exclude([
    "/api/*",
    "/admin/*",
    "/checkout/*"
])
```
The base URL uses the current request, then `APP_URL`, and finally `http://localhost`. Behind a proxy correctly configure `Host` and `X-Forwarded-Proto`.

`SEO::twitter` is not registered. `SEO::render` already adds a default card;
use `SEO::meta("twitter:...", valor)` for additional metadata.
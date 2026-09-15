# SEO y Sitemap

[Índice](README.md) · Antes: [proyecto web](PROYECTO_WEB.md) · [Clases nativas](MODULOS_NATIVOS.md)

SEO prepara etiquetas para buscadores y redes. Un sitemap enumera URLs que un
buscador puede visitar; no mejora por sí solo la calidad o autorización de una página.

```joss
SEO::title("Productos")
SEO::description("Catálogo")
SEO::keywords(["joss", "productos"])
SEO::canonical("https://example.com/products")
SEO::og("image", "https://example.com/cover.png")
$tags = SEO::render()
```

También existe `SEO::meta($name, $content)`. La salida escapa atributos HTML y agrega una Twitter card predeterminada.

## Generación Dinámica de Sitemap (`/sitemap.xml`)

`/sitemap.xml` se genera en vivo en cada petición con una hermosa interfaz interactiva XSL (`/sitemap.xsl`). No escribe archivos físicos en disco.

### 1. Entradas Estáticas Manuales
```joss
Sitemap::add("/docs", "2026-07-15", "weekly", 0.8)

// O con mapas/arrays asociativos:
Sitemap::add({
    "url": "/contacto",
    "changefreq": "monthly",
    "priority": 0.6
})
```

### 2. Proveedores dinámicos

`Sitemap::provider` acepta una closure o función que retorna una lista de entradas de sitemap, agregando dinámicamente registros desde la base de datos:

```joss
Sitemap::provider(func() {
    return [
        {"url": "/blog/mi-post", "changefreq": "daily", "priority": 0.9}
    ]
})
```

### 3. Exclusiones de Rutas
```joss
Sitemap::exclude([
    "/api/*",
    "/admin/*",
    "/checkout/*"
])
```

La URL base usa el request actual, después `APP_URL` y finalmente `http://localhost`. Detrás de un proxy configura correctamente `Host` y `X-Forwarded-Proto`.

`SEO::twitter` no está registrado. `SEO::render` ya agrega una card predeterminada;
usa `SEO::meta("twitter:...", valor)` para metadata adicional.

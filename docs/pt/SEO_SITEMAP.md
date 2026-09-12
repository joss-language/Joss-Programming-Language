# SEO e Sitemap

[Índice](README.md) · Antes: [projeto web](PROYECTO_WEB.md) · [Classes nativas](MODULOS_NATIVOS.md)

SEO prepara tags para mecanismos de busca e redes. Um mapa do site lista URLs que um
mecanismo de pesquisa pode visitar; por si só, não melhora a qualidade ou a autorização de uma página.```joss
SEO::title("Productos")
SEO::description("Catálogo")
SEO::keywords(["joss", "productos"])
SEO::canonical("https://example.com/products")
SEO::og("image", "https://example.com/cover.png")
$tags = SEO::render()
```
Há também `SEO::meta($name, $content)`. A saída escapa dos atributos HTML e adiciona um cartão padrão do Twitter.

## Geração dinâmica de Sitemap (`/sitemap.xml`)

`/sitemap.xml` é gerado ao vivo em cada solicitação com uma bela interface XSL interativa (`/sitemap.xsl`). Não grava arquivos físicos no disco.

### 1. Entradas estáticas manuais```joss
Sitemap::add("/docs", "2026-07-15", "weekly", 0.8)

// O con mapas/arrays asociativos:
Sitemap::add({
    "url": "/contacto",
    "changefreq": "monthly",
    "priority": 0.6
})
```
### 2. Provedores dinâmicos

`Sitemap::provider` está registrado, mas o manipulador aceita um
`parser.FunctionLiteral` enquanto um fechamento avaliado chega como
`CapturedFunction`. Sua operação de origem não é garantida. Consulte o
registra e chama `Sitemap::add` para cada URL enquanto a borda é corrigida.

### 3. Exclusões de rota```joss
Sitemap::exclude([
    "/api/*",
    "/admin/*",
    "/checkout/*"
])
```
A URL base usa a solicitação atual, depois `APP_URL` e, finalmente, `http://localhost`. Atrás de um proxy configure corretamente `Host` e `X-Forwarded-Proto`.

`SEO::twitter` não está registrado. `SEO::render` já adiciona um cartão padrão;
use `SEO::meta("twitter:...", valor)` para metadados adicionais.
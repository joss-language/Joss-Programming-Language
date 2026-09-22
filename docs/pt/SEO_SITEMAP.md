# SEO e Sitemap

[Índice](README.md) · Antes: [projeto web](PROYECTO_WEB.md) · [Classes nativas](MODULOS_NATIVOS.md)

SEO prepara etiquetas para buscadores e redes. Um sitemap lista URLs que um
buscador pode visitar; não melhora por si só a qualidade ou autorização de uma página.

```joss
SEO::title("Productos")
SEO::description("Catálogo")
SEO::keywords(["joss", "productos"])
SEO::canonical("https://example.com/products")
SEO::og("image", "https://example.com/cover.png")
$tags = SEO::render()
```

Também existe `SEO::meta($name, $content)`. A saída escapa atributos HTML e adiciona um Twitter card padrão.

## Geração Dinâmica de Sitemap (`/sitemap.xml`)

`/sitemap.xml` é gerado ao vivo em cada requisição com uma bela interface interativa XSL (`/sitemap.xsl`). Não escreve arquivos físicos no disco.

### 1. Entradas Estáticas Manuais
```joss
Sitemap::add("/docs", "2026-07-15", "weekly", 0.8)

// O con mapas/arrays asociativos:
Sitemap::add({
    "url": "/contacto",
    "changefreq": "monthly",
    "priority": 0.6
})
```

### 2. Provedores dinâmicos

`Sitemap::provider` aceita uma closure ou função que retorna uma lista de entradas de sitemap, adicionando dinamicamente registros a partir do banco de dados:

```joss
Sitemap::provider(func() {
    return [
        {"url": "/blog/mi-post", "changefreq": "daily", "priority": 0.9}
    ]
})
```

### 3. Exclusões de Rotas
```joss
Sitemap::exclude([
    "/api/*",
    "/admin/*",
    "/checkout/*"
])
```

A URL base usa a requisição atual, depois `APP_URL` e finalmente `http://localhost`. Atrás de um proxy, configure corretamente `Host` e `X-Forwarded-Proto`.

`SEO::twitter` não está registrado. `SEO::render` já adiciona um card padrão;
use `SEO::meta("twitter:...", valor)` para metadados adicionais.

## IndexNow (Indexação Instantânea)

IndexNow permite notificar em tempo real os motores de busca (Bing, Yandex, Seznam, Naver) sobre mudanças ou novas páginas sem esperar que um crawler percorra o sitemap.

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

Ao iniciar o servidor (`joss server start`, `joss program start`, etc.), o runtime detecta `INDEXNOW_KEY` em `.env` e o servidor HTTP de Joss responde automaticamente de forma interna em `GET /{key}.txt` com a chave em texto plano, validando a propriedade do seu domínio instantaneamente. Não é necessário criar arquivos físicos no disco nem definir rotas em `routes.joss`.
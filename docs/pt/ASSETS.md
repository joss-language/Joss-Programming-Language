# Ativos

[Índice](README.md)

O servidor atende os arquivos estáticos de `public/` diretamente na raiz `/` (por exemplo, `public/joss.jpg` responde em `/joss.jpg`, sem o prefixo `/public/`), e todos os recursos dinâmicos (CSS, SCSS compilado, JS e componentes) de `assets/` em `/assets/`. Em desenvolvimento também expõe ativos detectados em `node_modules` por meio de `/assets/vendor/`.

O detector lê apenas `dependencies` de `package.json`. Para cada pacote instalado procure `style`, `main` terminando em `.js` e, como substituto, arquivos `*.min.css`/`*.min.js` na raiz ou `dist/`. Não é garantido detectar todos os formatos de pacotes modernos.

As tags CSS são inseridas antes de `</head>` e as tags JS antes de `</body>` ao renderizar visualizações. Em builds standalone, os ativos de `assets/` e `public/` são embutidos diretamente no binário nativo mantendo os mesmos caminhos (`/assets/...` e `/...`).

Os arquivos `.scss` de `assets/css/` são compilados ao iniciar o servidor e em cada hot-reload para um cache efêmero em `assets/.joss_cache/css/`, servido em `/assets/css/*.css`, que é removido automaticamente ao parar o servidor sem gravar arquivos em `public/`. O compilador incluído implementa um subconjunto de SCSS e resolução básica de importação; não substitui toda a semântica do Sass.

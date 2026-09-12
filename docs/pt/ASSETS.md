# Ativos

[Índice](README.md)

O servidor atende `public/` em `/public/` e `/assets/`. Em desenvolvimento também expõe ativos detectados em `node_modules` por meio de `/assets/vendor/`.

O detector lê apenas `dependencies` de `package.json`. Para cada pacote instalado procure `style`, `main` terminando em `.js` e, como substituto, arquivos `*.min.css`/`*.min.js` na raiz ou `dist/`. Não é garantido detectar todos os formatos de pacotes modernos.

As tags CSS são inseridas antes de `</head>` e as tags JS antes de `</body>` ao renderizar visualizações. Na construção VFS, `/assets/vendor/` não funciona diretamente de `node_modules`; Inclua na construção todos os ativos que o aplicativo precisa.

Os `.scss` de `assets/css/` são recompilados durante a recarga. O compilador incluído implementa um subconjunto de SCSS e resolução básica de importação; não substitui toda a semântica do Sass.

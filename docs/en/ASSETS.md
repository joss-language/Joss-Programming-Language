#Assets

[Index](README.md)

The server serves `public/` under `/public/` and `/assets/`. In development also exposes assets detected within `node_modules` through `/assets/vendor/`.

The detector only reads `dependencies` from `package.json`. For each installed package look for `style`, `main` ending in `.js` and, as a fallback, files `*.min.css`/`*.min.js` in the root or `dist/`. It is not guaranteed to detect all modern packet formats.

CSS tags are inserted before `</head>` and JS tags before `</body>` when rendering views. In VFS build, `/assets/vendor/` does not work directly from `node_modules`; Include in the build any assets that the application needs.

The `.scss` and `assets/css/` are recompiled during reload. The included compiler implements a subset of SCSS and basic import resolution; it does not replace all Sass semantics.

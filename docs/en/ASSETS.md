# Assets

[Index](README.md)

The server serves `public/` under `/public/` and `/assets/` .In development also exposes assets detected within `node_modules` using `/assets/vendor/` .

The detector reads only `dependencies` from `package.json` .For each installed package look for `style` , `main` ending in `.js` and, as a fallback, `*.min.css` / `*.min.js` files in the root or `dist/` .It is not guaranteed to detect all modern packet formats.

CSS tags are inserted before `</head>` and JS tags before `</body>` when rendering views.In VFS build, `/assets/vendor/` is not served directly from `node_modules` ;Include in the build any assets that the application needs.

The `.scss` of `assets/css/` are recompiled during reload.The included compiler implements a subset of SCSS and basic import resolution;it does not replace all Sass semantics.

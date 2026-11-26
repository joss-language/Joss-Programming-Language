# Assets

[Index](README.md)

The server serves static files from `public/` directly at the root `/` (for example, `public/joss.jpg` responds at `/joss.jpg`, without the `/public/` prefix), and all dynamic resources (CSS, compiled SCSS, JS, and components) from `assets/` under `/assets/`. In development it also exposes assets detected within `node_modules` using `/assets/vendor/`.

The detector reads only `dependencies` from `package.json`. For each installed package look for `style`, `main` ending in `.js` and, as a fallback, `*.min.css`/`*.min.js` files in the root or `dist/`. It is not guaranteed to detect all modern package formats.

CSS tags are inserted before `</head>` and JS tags before `</body>` when rendering views. In standalone builds, assets from `assets/` and `public/` are embedded directly into the native binary preserving the same paths (`/assets/...` and `/...`).

`.scss` files in `assets/css/` are compiled on server startup and on each hot-reload into an ephemeral cache at `assets/.joss_cache/css/`, served at `/assets/css/*.css`, which is automatically deleted when the server stops without writing files into `public/`. The included compiler implements a subset of SCSS and basic import resolution; it does not replace all Sass semantics.

# Assets

[Índice](README.md)

El servidor sirve los archivos estáticos de `public/` directamente desde la raíz `/` (por ejemplo, `public/joss.jpg` responde en `/joss.jpg`, sin prefijo `/public/`), y todos los recursos dinámicos (CSS, SCSS compilado, JS y componentes) desde `assets/` bajo `/assets/`. En desarrollo también expone assets detectados dentro de `node_modules` mediante `/assets/vendor/`.

El detector lee únicamente `dependencies` de `package.json`. Para cada paquete instalado busca `style`, `main` terminado en `.js` y, como fallback, archivos `*.min.css`/`*.min.js` en la raíz o `dist/`. No garantiza detectar todos los formatos modernos de paquetes.

Las etiquetas CSS se insertan antes de `</head>` y las JS antes de `</body>` al renderizar vistas. En build standalone, los assets de `assets/` y `public/` se embeben directamente en el binario nativo respetando las mismas rutas (`/assets/...` y `/...`).

Los `.scss` de `assets/css/` se compilan al iniciar el servidor y durante cada hot-reload hacia una caché efímera en `assets/.joss_cache/css/`, servida en `/assets/css/*.css`, que se elimina automáticamente al detener el servidor sin escribir archivos en `public/`. El compilador incluido implementa un subconjunto de SCSS y resolución básica de imports; no sustituye toda la semántica de Sass.

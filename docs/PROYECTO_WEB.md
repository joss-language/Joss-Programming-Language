# Proyecto web completo: Arquitectura MVC con el stack nativo

Antes: [Proyecto de consola](PROYECTO_CONSOLA.md). Después: [Controladores y HTTP](CONTROLADORES.md).
Referencia técnica: [Motor de vistas](VISTAS.md), [Servidor nativo](SERVIDOR.md), [Modelos GranDB](MODELOS.md).

---

## ¿Qué vas a construir aquí?

Una **aplicación web** es un programa que corre en un servidor escuchando peticiones de red (generalmente de un navegador o una aplicación móvil) y responde con páginas HTML interactivas o datos en formato JSON.

A diferencia de otros lenguajes donde necesitas instalar servidores externos complejos (como Apache, Nginx o paquetes pesados de Node.js), **Joss incluye su propio servidor HTTP de alto rendimiento, router dinámico y motor de plantillas HTML**.

En este tutorial guiado construirás tu primera aplicación web basada en el patrón arquitectónico **MVC (Modelo - Vista - Controlador)**:
1. Crear la estructura del proyecto usando el generador de plantillas `joss new web`.
2. Comprender el flujo de una petición web en Joss.
3. Definir una ruta con parámetros dinámicos en `routes.joss`.
4. Crear un controlador en `app/controllers/` que procese la lógica.
5. Diseñar una vista HTML segura en `app/views/` con escape automático contra inyecciones XSS.
6. Poner en marcha el servidor y probar la aplicación en el navegador.

---

## 1. El flujo de una petición web en Joss

Cuando un usuario escribe una dirección en su navegador, ocurre la siguiente secuencia:

```text
1. Navegador web
   │  Solicita: GET http://127.0.0.1:8080/saludo/Ana
   ▼
2. Servidor HTTP nativo de Joss (construido sobre Go de alta concurrencia)
   │
   ▼
3. Router (routes.joss)
   │  Encuentra coincidencia: "/saludo/{nombre}"
   │  Extrae el parámetro: $nombre = "Ana"
   ▼
4. Controlador (app/controllers/SaludoController.joss)
   │  Ejecuta el método: SaludoController@show($nombre)
   │  Prepara los datos y llama a la vista: view("saludo", {"nombre": ...})
   ▼
5. Motor de Vistas (app/views/saludo.joss.html)
   │  Interpola las variables y escapa caracteres peligrosos
   ▼
6. Respuesta HTTP (HTML renderizado devuelto al navegador del usuario)
```

---

## 2. Generar el proyecto web

Abre tu terminal y ejecuta el generador oficial:

```bash
joss new web saludo_web
cd saludo_web
```

Este comando creará la estructura estándar de una aplicación web Joss:
- `main.joss`: Punto de entrada del servidor.
- `env.joss`: Variables de configuración (puerto, base de datos, claves secretas).
- `routes.joss`: El mapa de URLs de la aplicación.
- `app/controllers/`: Dónde residen los controladores.
- `app/models/`: Modelos de base de datos con GranDB.
- `app/views/`: Plantillas HTML dinámicas (`.joss.html`).
- `public/`: Archivos estáticos directos (hojas de estilo CSS, scripts JS, imágenes).

---

## 3. Configurar el puerto del servidor

Abre el archivo `env.joss` en la raíz del proyecto. Verás una línea que define el puerto. Asegúrate de configurar un puerto disponible (por ejemplo `8080`):

```joss
PORT = "8080"
APP_NAME = "Mi Web Joss"
```

---

## 4. Definir la ruta dinámica

Abre el archivo `routes.joss` y agrega la siguiente definición:

<!-- joss-check: fragmento de routes.joss -->
```joss
Router::get("/saludo/{nombre}", "SaludoController@show")
```

### ¿Qué significa esta línea?
- `Router::get(...)`: Indica que responderemos a peticiones HTTP de tipo `GET` (las que hace el navegador al abrir un enlace).
- `"/saludo/{nombre}"`: El patrón de la URL. `{nombre}` es un **segmento dinámico**: cualquier texto que el usuario escriba en esa posición de la URL será capturado.
- `"SaludoController@show"`: El destinatario. Le indica al router que debe instanciar la clase `SaludoController` y ejecutar su método `show`.

---

## 5. Crear el controlador

Crea el archivo `app/controllers/SaludoController.joss` con el siguiente código:

<!-- joss-check: fragmento de controlador web -->
```joss
public class SaludoController {
    public func show(string $nombre) {
        var $nombreLimpio = trim($nombre)
        return view("saludo", {"nombre": $nombreLimpio})
    }
}
```

### Anatomía del controlador:
1. `public class SaludoController`: Clase pública que Joss descubre automáticamente sin necesidad de `import`.
2. `public func show(string $nombre)`: El método recibe el parámetro dinámico `{nombre}` capturado por la ruta. En Joss, los parámetros del controlador deben declarar su tipo (`string $nombre`).
3. `$nombreLimpio = trim($nombre)`: Limpia posibles espacios en blanco en los extremos.
4. `return view("saludo", {"nombre": $nombreLimpio})`:
   - Llama a la función nativa `view(...)`.
   - El primer argumento es el nombre del archivo de plantilla (buscará `app/views/saludo.joss.html`).
   - El segundo argumento es un mapa asociativo con los datos que queremos inyectar dentro del HTML.

---

## 6. Diseñar la vista HTML

Crea el archivo de plantilla `app/views/saludo.joss.html`:

```html
<!doctype html>
<html lang="es">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Saludo - Mi Web Joss</title>
    <style>
        body { font-family: system-ui, sans-serif; margin: 40px; background: #f8fafc; color: #1e293b; }
        .card { background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1); max-width: 500px; }
        h1 { color: #0f766e; margin-top: 0; }
    </style>
</head>
<body>
    <div class="card">
        <h1>¡Hola, {{ $nombre }}!</h1>
        <p>Bienvenido a tu primera aplicación web construida con <strong>Joss</strong>.</p>
    </div>
</body>
</html>
```

### Seguridad nativa contra ataques XSS:
Nota la sintaxis `{{ $nombre }}`:
- El motor de plantillas de Joss **escapa automáticamente caracteres HTML peligrosos** como `<`, `>`, `"` o `&`.
- Si un usuario malicioso intenta ingresar como nombre `<script>alert('hack')</script>`, Joss lo convertirá a entidades seguras (`&lt;script&gt;...`), protegiendo a tus visitantes de vulnerabilidades de Cross-Site Scripting (XSS).

---

## 7. Iniciar y probar el servidor

En tu terminal, ubicado en la raíz del proyecto `saludo_web`, escribe:

```bash
joss server start
```

Verás el mensaje de confirmación del servidor HTTP:

```text
[CLI] Ejecutando script de inicio (main.joss)...
[Joss Server] Servidor HTTP escuchando en http://127.0.0.1:8080
[Joss Server] Presiona 'q' o Ctrl+C para detener el servidor.
```

Abre tu navegador web y visita:

`http://127.0.0.1:8080/saludo/Ana`

Verás la tarjeta estilizada saludando a Ana. Ahora prueba a cambiar la URL a:

`http://127.0.0.1:8080/saludo/Carlos`

El servidor responderá al instante con el nuevo saludo.

Para detener el servidor en cualquier momento, presiona simplemente la tecla **`q`** en tu terminal.

---

## 8. Verificación y comprobación estática

Antes de desplegar a producción, puedes auditar todo el proyecto con los comandos de verificación de Joss:

```bash
joss analyze main.joss
joss check .
```

---

## Siguiente paso

Ahora que conoces el ciclo completo de una petición web, puedes explorar las capacidades avanzadas del stack web de Joss: autenticación de usuarios, middlewares de seguridad, bases de datos GranDB y WebSockets en tiempo real:

Continúa con: [HTTP y Controladores](CONTROLADORES.md), [Middlewares y Seguridad](MIDDLEWARE.md) y [Modelos GranDB](MODELOS.md).

# Complete Web Project: MVC Architecture with the Native Stack

Previous: [Console project](PROYECTO_CONSOLA.md). Next: [Controllers and HTTP](CONTROLADORES.md).
Technical reference: [View engine](VISTAS.md), [Native server](SERVIDOR.md), [GranDB models](MODELOS.md).

---

## What Will You Build Here?

A **web application** is a program that runs on a server listening to network requests (usually from a browser or mobile application) and responds with interactive HTML pages or data in JSON format.

Unlike other languages where you need to install complex external servers (such as Apache, Nginx, or heavy Node.js packages), **Joss includes its own high-performance HTTP server, dynamic router, and HTML template engine**.

In this guided tutorial you will build your first web application based on the **MVC (Model - View - Controller)** architectural pattern:
1. Create the project structure using the `joss new` template generator (`joss new web`).
2. Understand the flow of a web request in Joss.
3. Configure the port in `env.joss` and define a route with dynamic parameters in `routes.joss`.
4. Create a controller in `app/controllers/` that processes the logic.
5. Design a secure HTML view in `app/views/` with automatic escaping against XSS injections.
6. Start the server with `joss server start` and test the application in the browser.

---

## 1. The Flow of a Web Request in Joss

When a user types an address into their browser, the following sequence occurs:

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

## 2. Generate the Web Project

Open your terminal and run the official generator (you can type `joss new saludo_web` or explicitly `joss new web saludo_web`):

```bash
joss new web saludo_web
cd saludo_web
```

This command will create the standard structure of a Joss web application:
- `main.joss`: Entry point of the project.
- `env.joss`: Configuration variables (port, database, secret keys).
- `routes.joss` and `api.joss`: The map of web URLs and API endpoints of the application.
- `app/controllers/`: Where controllers reside (organized into `web/`, `auth/`, `api/`).
- `app/models/`: Database models with GranDB.
- `app/views/`: Dynamic HTML templates (`.joss.html`).
- `public/`: Direct static files (CSS stylesheets, JS scripts, images).

---

## 3. Configure the Server Port

Open the `env.joss` file at the root of the project. By default the template generates `PORT="80"`. To develop locally without administrator privileges, change it to a free port such as `"8080"`:

```joss
PORT="8080"
APP_NAME="Mi Web Joss"
```

> [!NOTE]
> `env.joss` is an environment configuration file that Joss loads automatically on startup; it is not an executable script to run with `joss run env.joss`.

---

## 4. Define the Dynamic Route

Open the `routes.joss` file and add the following definition:

<!-- joss-check: fragmento de routes.joss -->
```joss
Router::get("/saludo/{nombre}", "SaludoController@show")
```

### What Does This Line Mean?
- `Router::get(...)`: Indicates that we will respond to HTTP requests of type `GET` (the ones made by the browser when opening a link).
- `"/saludo/{nombre}"`: The URL pattern. `{nombre}` is a **dynamic segment**: any text the user types into that position of the URL will be captured.
- `"SaludoController@show"`: The target handler. Tells the router to instantiate the `SaludoController` class and execute its `show` method.

---

## 5. Create the Controller

Create the `app/controllers/SaludoController.joss` file with the following code:

<!-- joss-check: fragmento de controlador web -->
```joss
public class SaludoController {
    public func show(string $nombre) {
        var $nombreLimpio = trim($nombre)
        return view("saludo", {"nombre": $nombreLimpio})
    }
}
```

### Anatomy of the Controller:
1. `public class SaludoController`: Public class that Joss automatically discovers inside `app/` without needing an `import`.
2. `public func show(string $nombre)`: The method receives the dynamic parameter `{nombre}` captured by the route. In Joss, controller parameters must declare their type (`string $nombre`).
3. `var $nombreLimpio = trim($nombre)`: Trims any leading or trailing whitespace.
4. `return view("saludo", {"nombre": $nombreLimpio})`:
   - Calls the native function `view(...)`.
   - The first argument is the name of the template file (it will look for `app/views/saludo.joss.html`).
   - The second argument is an associative map with the data we want to inject into the HTML.

---

## 6. Design the HTML View

Create the template file `app/views/saludo.joss.html`:

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

### Native Security Against XSS Attacks:
Notice the `{{ $nombre }}` syntax:
- Joss's template engine **automatically escapes dangerous HTML characters** such as `<`, `>`, `"`, or `&`.
- If a malicious user tries to enter `<script>alert('hack')</script>` as a name, Joss will convert it into safe entities (`&lt;script&gt;...`), protecting your visitors from Cross-Site Scripting (XSS) vulnerabilities.

---

## 7. Start and Test the Server

In your terminal, located at the root of the `saludo_web` project, type:

```bash
joss server start
```

(Or its equivalent shortcut `joss server`). You will see the HTTP server confirmation message:

```text
[CLI] Ejecutando script de inicio (main.joss)...
[Joss Server] Servidor HTTP escuchando en http://127.0.0.1:8080
[Joss Server] Presiona 'q' o Ctrl+C para detener el servidor.
```

Open your web browser and visit:

`http://127.0.0.1:8080/saludo/Ana`

You will see the styled card greeting Ana. Now try changing the URL to:

`http://127.0.0.1:8080/saludo/Carlos`

The server will respond instantly with the new greeting.

To stop the server at any time, simply press the **`q`** key or `Ctrl + C` in your terminal.

---

## 8. Verification and Static Checking

Before deploying to production, you can audit the entire project with Joss's verification commands:

```bash
joss analyze main.joss
joss check .
```

---

## 9. Practical Exercises

### Level 1 — Quick Test
Pass a second piece of data from `SaludoController@show` to the view by adding `"rol": "Desarrollador Joss"` to the `view(...)` map and display it inside `saludo.joss.html` below the title using `<p>Rol: {{ $rol }}</p>`.

### Level 2 — Guided Exercise
1. Open `api.joss` (or `routes.joss`) and register a route `Router::get("/api/estado", "SaludoController@estado")`.
2. Add in `SaludoController` the method `public func estado()` that returns `Response::json({"ok": true, "servidor": "Joss HTTP"})`.
3. Visit `http://127.0.0.1:8080/api/estado` in your browser and check the JSON response.

### Level 3 — Challenge
1. Register a route with two dynamic parameters: `Router::get("/calcular/{a}/{b}", "SaludoController@sumar")`.
2. Implement `public func sumar(string $a, string $b)` in `SaludoController`, convert both parameters with `intval($a)` and `intval($b)`, calculate their sum, and render a view or JSON response with the operation details.

---

## Next Step

Now that you know the complete lifecycle of a web request, you can explore the advanced capabilities of the Joss web stack and the complete type system of the language:

Continue with: [Type system and coercion](SISTEMA_TIPOS.md), [HTTP and Controllers](CONTROLADORES.md), [Middlewares and Security](MIDDLEWARE.md), and [GranDB Models](MODELOS.md).
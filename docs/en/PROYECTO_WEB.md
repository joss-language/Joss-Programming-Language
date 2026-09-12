# Complete web project: MVC architecture with the native stack

Before: [Console project](PROYECTO_CONSOLA.md). After: [Controllers and HTTP](CONTROLADORES.md).
Technical reference: [View Engine](VISTAS.md), [Native Server](SERVIDOR.md), [GranDB Models](MODELOS.md).

---

## What are you going to build here?

A **web application** is a program that runs on a server listening to network requests (usually from a browser or a mobile application) and responds with interactive HTML pages or data in JSON format.

Unlike other languages ​​where you need to install complex external servers (like Apache, Nginx or heavy Node.js packages), **Joss includes its own high-performance HTTP server, dynamic router and HTML templating engine**.

In this guided tutorial you will build your first web application based on the **MVC (Model - View - Controller)** architectural pattern:
1. Create the project structure using the `joss new web` template builder.
2. Understand the flow of a web request in Joss.
3. Define a route with dynamic parameters in `routes.joss`.
4. Create a controller in `app/controllers/` that processes the logic.
5. Design a secure HTML view in `app/views/` with automatic escaping against XSS injections.
6. Start up the server and test the application in the browser.

---

## 1. The flow of a web request in Joss

When a user types an address into their browser, the following sequence occurs:```text
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

## 2. Generate the web project

Open your terminal and run the official generator:```bash
joss new web saludo_web
cd saludo_web
```
This command will create the standard structure of a Joss web application:
- `main.joss`: Server entry point.
- `env.joss`: Configuration variables (port, database, secret keys).
- `routes.joss`: The application URL map.
- `app/controllers/`: Where the controllers reside.
- `app/models/`: Database models with GranDB.
- `app/views/`: Dynamic HTML templates (`.joss.html`).
- `public/`: Direct static files (CSS style sheets, JS scripts, images).

---

## 3. Configure the server port

Open the `env.joss` file in the root of the project. You will see a line that defines the port. Make sure you configure an available port (for example `8080`):```joss
PORT = "8080"
APP_NAME = "Mi Web Joss"
```
---

## 4. Define the dynamic route

Open the `routes.joss` file and add the following definition:<!-- joss-check: fragmento de routes.joss -->
```joss
Router::get("/saludo/{nombre}", "SaludoController@show")
```
### What does this line mean?
- `Router::get(...)`: Indicates that we will respond to HTTP requests of type `GET` (those made by the browser when opening a link).
- `"/saludo/{nombre}"`: The URL pattern. `{nombre}` is a **dynamic segment**: any text the user types at that position in the URL will be captured.
- `"SaludoController@show"`: The recipient. Tells the router to instantiate the `SaludoController` class and execute its `show` method.

---

## 5. Create the controller

Create the file `app/controllers/SaludoController.joss` with the following code:<!-- joss-check: fragmento de controlador web -->
```joss
public class SaludoController {
    public func show(string $nombre) {
        $nombreLimpio = trim($nombre)
        return view("saludo", {"nombre": $nombreLimpio})
    }
}
```
### Controller Anatomy:
1. `public class SaludoController`: Public class that Joss automatically discovers without the need for `import`.
2. `public func show(string $nombre)`: The method receives the dynamic parameter `{nombre}` captured by the route. In Joss, controller parameters must declare their type (`string $nombre`).
3. `$nombreLimpio = trim($nombre)`: Cleans possible whitespace at the ends.
4. `return view("saludo", {"nombre": $nombreLimpio})`:
   - Call the native function `view(...)`.
   - The first argument is the name of the template file (it will look for `app/views/saludo.joss.html`).
   - The second argument is an associative map with the data that we want to inject into the HTML.

---

## 6. Style the HTML view

Create the template file `app/views/saludo.joss.html`:```html
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
### Native security against XSS attacks:
Note the `{{ $nombre }}` syntax:
- Joss templating engine **automatically escapes dangerous HTML characters** such as `<`, `>`, `"` or `&`.
- If a malicious user tries to enter as name `<script>alert('hack')</script>`, Joss will convert them to safe entities (`&lt;script&gt;...`), protecting your visitors from Cross-Site Scripting (XSS) vulnerabilities.

---

## 7. Start and test the server

In your terminal, located at the root of the `saludo_web` project, type:```bash
joss server start
```
You will see the confirmation message from the HTTP server:```text
[CLI] Ejecutando script de inicio (main.joss)...
[Joss Server] Servidor HTTP escuchando en http://127.0.0.1:8080
[Joss Server] Presiona 'q' o Ctrl+C para detener el servidor.
```
Open your web browser and visit:

`http://127.0.0.1:8080/saludo/Ana`

You will see the stylized card greeting Ana. Now try changing the URL to:

`http://127.0.0.1:8080/saludo/Carlos`

The server will respond instantly with the new greeting.

To stop the server at any time, simply press the **`q`** key on your terminal.

---

## 8. Verification and static verification

Before deploying to production, you can audit the entire project with Joss verification commands:```bash
joss analyze main.joss
joss check .
```
---

## Next step

Now that you know the complete cycle of a web request, you can explore the advanced capabilities of the Joss web stack: user authentication, security middlewares, GranDB databases and real-time WebSockets:

Continue with: [HTTP and Drivers](CONTROLADORES.md), [Middlewares and Security](MIDDLEWARE.md) and [GranDB Models](MODELOS.md).
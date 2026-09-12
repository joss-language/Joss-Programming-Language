# Middleware

[Index](README.md) · Before: [controllers](CONTROLADORES.md) · After: [server](SERVIDOR.md)

A middleware checks or transforms a request before calling the controller. For example, you can redirect someone who is not logged in. Register it with a closure and apply it while defining routes. The following snippet requires DashboardController and a /login path.```joss
Router::registerMiddleware("auth", func(string $name) {
    (!Auth::check()) ? {
        return Response::redirect("/login")
    } : {}
})

Router::middleware("auth")
Router::get("/dashboard", "DashboardController@index")
Router::end()
```
`middleware($name)` adds the name to the registered routes after the call; `end()` removes the last name. `group($name, func() { ... })` executes a closure while that middleware is active. The first argument of `group` is a middleware name, not a URL prefix.

Middlewares are executed for dispatched HTTP requests. The WebSocket upgrade occurs before the normal HTTP middleware; explicitly authenticates those connections.
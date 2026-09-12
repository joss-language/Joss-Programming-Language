#Middleware

[Índice](README.md) · Antes: [controladores](CONTROLADORES.md) · Depois: [servidor](SERVIDOR.md)

Um middleware verifica ou transforma uma solicitação antes de chamar o controlador. Por exemplo, você pode redirecionar alguém que não está logado. Cadastre-o com um fechamento e aplique-o na definição de rotas. O snippet a seguir requer DashboardController e um caminho /login.```joss
Router::registerMiddleware("auth", func(string $name) {
    (!Auth::check()) ? {
        return Response::redirect("/login")
    } : {}
})

Router::middleware("auth")
Router::get("/dashboard", "DashboardController@index")
Router::end()
```
`middleware($name)` adiciona o nome às rotas registradas após a chamada; `end()` remove o sobrenome. `group($name, func() { ... })` executa um encerramento enquanto o middleware está ativo. O primeiro argumento de `group` é um nome de middleware, não um prefixo de URL.

Middlewares são executados para solicitações HTTP despachadas. A atualização do WebSocket ocorre antes do middleware HTTP normal; autentica explicitamente essas conexões.
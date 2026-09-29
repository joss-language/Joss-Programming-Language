package files

import "path/filepath"

func GetRoutesFiles(path string) map[string]string {
	return map[string]string{
		filepath.Join(path, "routes.joss"): `// Web Routes
// Rutas Públicas
Router::get("/", "HomeController@index")

// Rutas de Autenticación (Solo invitados)
Router::middleware("guest")
Router::match("GET|POST", "/login", "AuthController@showLogin@doLogin")
Router::match("GET|POST", "/register", "AuthController@showRegister@doRegister")
Router::get("/verify/{token}", "AuthController@verify")
Router::get("/2fa/verify", "AuthController@showVerify2FA")
Router::post("/2fa/verify", "AuthController@doVerify2FA")

// Social Login (OAuth)
Router::get("/auth/{provider}/redirect", "AuthController@socialRedirect")
Router::get("/auth/{provider}/callback", "AuthController@socialCallback")

// Password Recovery
Router::get("/password/forgot", "PasswordController@showForgot")
Router::post("/password/email", "PasswordController@sendResetLink")
Router::get("/password/reset", "PasswordController@showReset")
Router::post("/password/reset", "PasswordController@resetPassword")
Router::end()

// Rutas Protegidas (Solo autenticados)
Router::middleware("auth")
    Router::get("/dashboard", "DashboardController@index")
    Router::get("/profile", "ProfileController@index")
    Router::post("/profile/update", "ProfileController@update")
    Router::post("/profile/delete", "ProfileController@delete")
    Router::post("/profile/2fa/activate", "ProfileController@activate2FA")
    Router::post("/profile/2fa/deactivate", "ProfileController@deactivate2FA")
    Router::get("/logout", "AuthController@logout")
Router::end()
`,
		filepath.Join(path, "api.joss"): `// API Routes
Router::api() // Enable API headers

// Public Routes
Router::post("/api/register", "ApiController@register")
Router::post("/api/login", "ApiController@login")
Router::post("/api/password/email", "ApiController@forgotPassword")
Router::post("/api/password/reset", "ApiController@resetPassword")

// Protected Routes (JWT)
Router::middleware("auth_api")
    Router::post("/api/refresh", "ApiController@refresh")
    Router::delete("/api/delete", "ApiController@delete")
Router::end()
`,
	}
}

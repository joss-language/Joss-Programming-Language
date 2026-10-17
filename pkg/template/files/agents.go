package files

import "path/filepath"

func GetAgentsFiles(path string) map[string]string {
	return map[string]string{
		filepath.Join(path, "AGENTS.md"): `# ⚡ AGENTS.md - Joss Programming Language & Framework Guide

Welcome! This application has been built using **Joss (JosSecurity)** — a high-performance, developer-first programming language and full-stack web framework created by Joss Estrada (JOSPROX).

This document provides AI assistants (Antigravity, Cursor, Claude, Copilot, ChatGPT) with essential context, directory conventions, language syntax rules, Model ORM patterns, and standard library helpers.

---

## 🏛️ Project Architecture & Directory Layout

Joss applications use a domain-scoped MVC directory layout:

` + "```" + `
project-root/
├── main.joss                 # Entry point (Starts HTTP server & background services)
├── env.joss                  # Environment configuration variables (.env)
├── routes.joss               # Web HTTP routes & middleware definitions
├── api.joss                  # REST API endpoints
├── joss.yaml                 # Package manifest & dependencies (joss pub)
├── AGENTS.md                 # AI Agent instructions & language guide
├── config/                   # Global configuration & constants
│   └── reglas.joss
├── app/
│   ├── controllers/          # Domain-scoped HTTP Controllers
│   │   ├── web/              # Web page controllers (HTML views)
│   │   ├── auth/             # Authentication & Account controllers
│   │   └── api/              # REST JSON API controllers
│   ├── models/               # Model ORM & GranDB Models
│   │   └── auth/             # User & Auth models
│   ├── services/             # Background services & business domain logic
│   ├── middleware/           # Request & Security Middleware
│   ├── database/             # Migrations & Database Seeds
│   └── libs/                 # Custom helper libraries
├── app/views/                # HTML/View Templates
│   ├── layouts/              # Master layouts (@extends, @section)
│   ├── auth/                 # Login & Registration templates
│   └── dashboard/            # Admin dashboard templates
├── assets/                   # CSS, JS, and Documentation assets
│   └── docs/                 # Offline markdown documentation
├── public/                   # Static web root (CSS, JS, images, uploads)
└── storage/                  # SQLite DB, uploads, sessions & temporary logs
` + "```" + `

---

## 🔤 Joss Language Core Syntax Cheatsheet

### 1. Variables & Data Types
- **Variables must be declared**: Use ` + "`var`" + ` for strict inference or an explicit type (` + "`int`" + `, ` + "`string`" + `, ` + "`bool`" + `, ` + "`float`" + `, ` + "`decimal`" + `, ` + "`array`" + `, ` + "`map`" + `, ` + "`mixed`" + `, ` + "`const`" + `).
- Direct assignment without declaration (e.g. ` + "`$x = 1`" + `) is deprecated (` + "`JOSS-DECL-007`" + `).
  ` + "```joss" + `
  var $name = "Joss Red"                     // Inferred as string
  int $count = 42                            // Explicit type
  bool $isActive = true                      // Explicit bool
  decimal $price = 99.99m                    // Exact financial decimal
  array $items = ["apple", "banana", "orange"]
  map $user = {"id": 1, "username": "joss"}
  mixed $flexible = "dynamic value"          // Dynamic variable
  const int $MAX_ATTEMPTS = 5                // Immutable constant
  ` + "```" + `

### 2. Control Flow & Expressions (No ` + "`if/else`" + `, Only Ternaries & Match)
- Joss does **not** have ` + "`if/else`" + ` statements. All conditional branching is done via **ternary syntax** ` + "`(cond) ? { ... } : { ... }`" + ` or ` + "`match`" + `:
  ` + "```joss" + `
  // Branching with ternary blocks:
  ($user != null) ? {
      print("User found: " . $user["username"])
  } : {
      print("User not found")
  }

  // Single-branch ternary (no else needed):
  (!$user) ? {
      return redirect("/login")
  }

  // Guard statement with required exit (: { return / throw }) & Smart Casts:
  guard ($monto > 0) : {
      return "Monto inválido"
  }

  // Inline value ternary:
  var $status = ($age >= 18) ? "Adult" : "Minor"

  // Multi-branch with match:
  var $label = match ($role) {
      "admin"   => "Administrator",
      "editor"  => "Content Editor",
      default   => "Standard User"
  }
  ` + "```" + `

### 3. OOP Classes, Methods & Constructors
- Classes at file level require ` + "`public`" + ` or ` + "`private`" + `.
- Methods and properties require ` + "`public`" + `, ` + "`protected`" + `, or ` + "`private`" + `.
- All parameters must specify explicit types (` + "`mixed $arg`" + ` for dynamic).
- All named methods must declare a return type (` + "`: void`" + `, ` + "`: mixed`" + `, ` + "`: string`" + `, etc.):
  ` + "```joss" + `
  public class UserService {
      private string $prefix = "usr_"

      public func constructor(string $prefix = "usr_"): void {
          $this->prefix = $prefix
      }

      public func getUserName(map $user): string {
          return $this->prefix . $user["username"]
      }
  }

  var $service = new UserService()
  var $name = $service->getUserName({"username": "joss"})
  ` + "```" + `

### 4. Database Models (Model ORM)
- Models extend ` + "`Model`" + ` and configure metadata via protected properties (` + "`$table`" + `, ` + "`$fillable`" + `, ` + "`$hidden`" + `, ` + "`$casts`" + `):
  ` + "```joss" + `
  public class User extends Model {
      protected string $table = "users"
      protected array $fillable = ["first_name", "last_name", "email", "password", "active"]
      protected array $hidden = ["password"]
      protected map $casts = {"active": "bool"}

      public func posts(): mixed {
          return $this->hasMany("Post", "user_id", "id")
      }
  }

  // Model Queries & Operations
  var $users = User::all()
  var $user = User::find(1)
  var $activeUsers = User::where("active", true)->get()

  // Creation & Updates
  var $newUser = User::create({
      "first_name": "Ada",
      "email": "ada@example.com",
      "active": true
  })
  $newUser->first_name = "Grace"
  $newUser->save()
  ` + "```" + `

### 5. Routing (` + "`routes.joss`" + ` & ` + "`api.joss`" + `)
  ` + "```joss" + `
  // Public web routes
  Router::get("/", "HomeController@index")

  // Protected route group with middleware
  Router::middleware("auth")
      Router::get("/dashboard", "DashboardController@index")
      Router::get("/profile", "ProfileController@index")
  Router::end()

  // JSON REST API endpoints
  Router::post("/api/login", "ApiController@login")
  ` + "```" + `

### 6. Standard Native Classes & Helpers
- **Http Client**: ` + "`Http::get($url, $headers)`" + `, ` + "`Http::post($url, $body, $headers)`" + `, ` + "`Http::json($method, $url, $body, $headers)`" + `
- **HTTP Responses**: ` + "`Response::json($data, $code)`" + `, ` + "`Response::redirect($url)`" + `, ` + "`view($viewPath, $data)`" + `
- **Authentication**: ` + "`Auth::user()`" + `, ` + "`Auth::id()`" + `, ` + "`Auth::check()`" + `, ` + "`Auth::login($email, $password)`" + `, ` + "`Auth::logout()`" + `
- **Security & Crypto**: ` + "`Hash::make($password)`" + `, ` + "`Hash::check($password, $hash)`" + `, ` + "`AES::encrypt($text, $key)`" + `, ` + "`Turnstile::verify($token)`" + `
- **String Helpers**: ` + "`Str::contains($str, $sub)`" + `, ` + "`Str::upper($str)`" + `, ` + "`Str::lower($str)`" + `, ` + "`Str::trim($str)`" + `
- **Desktop Windowing**: ` + "`new Window($title, $width, $height)`" + ` (Native OS WebView engine)
- **File I/O**: ` + "`file_put_contents($path, $content)`" + `, ` + "`file_get_contents($path)`" + `
- **Cron Jobs**: ` + "`Cron::schedule($name, $cronExpr, $closure)`" + `
- **System**: ` + "`System::env($key)`" + `, ` + "`System::log($msg)`" + `, ` + "`System::now()`" + `

---

## 🛠️ CLI Development Commands

- **Start Development Server (with Live Hot Reload)**:
  ` + "```bash" + `
  joss server start
  ` + "```" + `
- **Build Standalone Native Binary (AOT Compilation)**:
  ` + "```bash" + `
  joss build main.joss -o build/myapp.exe
  joss build --target=linux-amd64
  joss build --target=darwin-arm64
  ` + "```" + `
- **Generate Controllers & Models**:
  ` + "```bash" + `
  joss make:controller web/CatalogController
  joss make:controller api/v1/OrderController
  joss make:model shop/Product
  ` + "```" + `
- **Generate Full CRUD**:
  ` + "```bash" + `
  joss make:crud products
  ` + "```" + `
- **Database Migrations**:
  ` + "```bash" + `
  joss migrate
  ` + "```" + `
- **Run Script or Console Program**:
  ` + "```bash" + `
  joss run main.joss
  ` + "```" + `
- **Package Management & Plugins**:
  ` + "```bash" + `
  joss pub install
  joss pub add <package>
  joss plugin compile .
  ` + "```" + `

---

## 📚 Official Documentation & Links

- **Official Website**: [https://astriom.org](https://astriom.org)
- **GitHub Repository**: [https://github.com/joss-language/Joss-Programming-Language](https://github.com/joss-language/Joss-Programming-Language)
- **Offline Markdown Docs**: Check ` + "`assets/docs/`" + ` in this project.
`,
	}
}

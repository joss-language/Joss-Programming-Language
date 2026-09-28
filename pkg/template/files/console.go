package files

import "path/filepath"

// GetConsoleConfigFiles returns configuration files for console projects
func GetConsoleConfigFiles(path string) map[string]string {
	return map[string]string{
		filepath.Join(path, "main.joss"): `// Aplicación de Consola Joss
// Entry Point

public class Main {
    Init main() {
        print("=== Aplicación de Consola Joss ===")
        print("")
        
        // ========================================
        // Tu lógica de aplicación aquí
        // ========================================
        
        print("¡Hola desde Joss Console!")
        print("")
        print("Este es un proyecto de consola backend-only.")
        print("Puedes agregar tu lógica en este archivo main.joss")
        print("")
        
        // Ejemplo: Usar modelos y controladores
        // $controller = new ExampleController()
        // $controller->ejecutar()
        
        // Ejemplo: Trabajar con base de datos
        // $model = new ExampleModel()
        // $datos = $model->obtenerTodos()
        // print($datos)
        
        print("Aplicación finalizada correctamente.")
    }
}`,
		filepath.Join(path, "env.joss"): `APP_ENV="development"

# Database Configuration (sqlite or mysql)
DB="sqlite"
DB_PATH="database.sqlite"

# MySQL Configuration (Only if DB="mysql")
# DB_HOST="localhost"
# DB_NAME="joss_console_db"
# DB_USER="root"
# DB_PASS=""

# Database Table Prefix
PREFIX="js_"

# Application Settings
APP_NAME="Joss Console App"
APP_VERSION="1.0.0"`,
		filepath.Join(path, "config", "reglas.joss"): `// Constantes Globales para Aplicación de Consola
const string $APP_NAME = "Joss Console"
const string $APP_VERSION = "1.0.0"

// Configuración de la aplicación
const bool $DEBUG_MODE = true
const int $MAX_RETRIES = 3`,
		filepath.Join(path, "tests", "main_test.joss"): `test("Ejecución de aplicación de consola", func() {
    string $appName = "Joss Console"
    assertEqual($appName, "Joss Console")
    assertTrue(true)
})`,
		filepath.Join(path, ".gitignore"): `plugins/
env.joss
env.enc
database.sqlite
log.txt
`,
		filepath.Join(path, ".dockerignore"): `plugins/
env.joss
env.enc
database.sqlite
log.txt
.git/
.github/
`,
		filepath.Join(path, "entrypoint.sh"): `#!/bin/sh
set -e

ENV_FILE="/app/env.joss"

echo "[entrypoint] Generando env.joss..."
rm -f "$ENV_FILE"
touch "$ENV_FILE"

for var in $(env | cut -d= -f1); do
    case "$var" in
        PATH|HOME|HOSTNAME|TERM|SHLVL|PWD|_|OLDPWD|DEBIAN_FRONTEND|LANG)
            continue
            ;;
        *)
            val=$(eval echo "\$$var" | sed 's/"/\\"/g')
            echo "${var}=\"${val}\"" >> "$ENV_FILE"
            ;;
    esac
done

echo "[entrypoint] ✓ env.joss generado."
echo "[entrypoint] Ejecutando aplicación de consola Joss..."
exec joss run main.joss
`,
		filepath.Join(path, "Dockerfile"): `# ============================================================
# Joss Console — Dockerfile (Debian Minimal + Joss Release)
# ============================================================

FROM debian:bookworm-slim

# Dependencias mínimas del sistema
RUN apt-get update && apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        unzip \
        jq \
    && rm -rf /var/lib/apt/lists/*

# Descargar e instalar la versión release oficial de Joss CLI
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    releases_url="https://api.github.com/repos/joss-language/Joss-Programming-Language/releases"; \
    rel_json="$(curl -fsSL "${releases_url}" | jq -c '.[] | select(.draft == false)' | head -n 1)"; \
    asset_url="$(printf '%s' "${rel_json}" | jq -r ".assets[] | select(.name | contains(\"${arch}\")) | .browser_download_url" | head -n 1)"; \
    if [ -z "${asset_url}" ] || [ "${asset_url}" = "null" ]; then \
        asset_url="$(printf '%s' "${rel_json}" | jq -r '.assets[] | select(.name | contains("linux")) | .browser_download_url' | head -n 1)"; \
    fi; \
    echo "Descargando Joss CLI para ${arch} desde: ${asset_url}"; \
    curl -fsSL "${asset_url}" -o /tmp/joss_pkg.zip; \
    unzip -q /tmp/joss_pkg.zip -d /tmp/joss_out; \
    mv /tmp/joss_out/joss* /usr/local/bin/joss || mv /tmp/joss_out/*/joss* /usr/local/bin/joss; \
    rm -rf /tmp/joss_pkg.zip /tmp/joss_out; \
    chmod +x /usr/local/bin/joss; \
    joss version

WORKDIR /app

# Copiar el código del proyecto
COPY . .

# Instalar dependencias/plugins si joss.yaml las declara
RUN joss pub install || true

RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
`,
	}
}

// GetConsoleAppFiles returns app structure files for console projects
func GetConsoleAppFiles(path string) map[string]string {
	return map[string]string{
		// .gitkeep files to maintain directory structure
		filepath.Join(path, "app", "models", ".gitkeep"):                 "",
		filepath.Join(path, "app", "controllers", ".gitkeep"):            "",
		filepath.Join(path, "app", "libs", ".gitkeep"):                   "",
		filepath.Join(path, "app", "database", "migrations", ".gitkeep"): "",

		// Example controller
		filepath.Join(path, "app", "controllers", "ExampleController.joss"): `// Controlador de Ejemplo para Consola
public class ExampleController {
    
    public func ejecutar() {
        print("Ejecutando ExampleController...")
        
        // Tu lógica aquí
        var $resultado = $this->procesarDatos()
        
        return $resultado
    }
    
    public func procesarDatos() {
        // Ejemplo de procesamiento
        var $datos = ["item1", "item2", "item3"]
        
        foreach ($datos as $item) {
            print("Procesando: " . $item)
        }
        
        return true
    }
}`,

		// Example model
		filepath.Join(path, "app", "models", "ExampleModel.joss"): `// Modelo de Ejemplo
public class ExampleModel extends GranDB {
    
    Init constructor() {
        $this->tabla = "js_example"
    }
    
    public func obtenerTodos() {
        return (new GranDB())->table($this->tabla)->get()
    }
    
    public func buscarPorId(mixed $id) {
        return (new GranDB())->table($this->tabla)->where("id", $id)->first()
    }
}`,

		// README for console project
		filepath.Join(path, "README.md"): `# Proyecto de Consola Joss

Este es un proyecto backend-only de Joss, diseñado para aplicaciones de línea de comandos.

## Estructura del Proyecto

` + "```" + `
/
├── main.joss              # Punto de entrada de la aplicación
├── env.joss               # Variables de entorno
├── config/
│   └── reglas.joss        # Constantes globales
├── app/
│   ├── controllers/       # Lógica de negocio
│   ├── models/            # Acceso a datos
│   ├── libs/              # Librerías personalizadas
│   └── database/
│       └── migrations/    # Migraciones de base de datos
└── README.md              # Este archivo
` + "```" + `

## Ejecutar la Aplicación

` + "```bash" + `
joss run main.joss
` + "```" + `

## Comandos Útiles

` + "```bash" + `
# Ejecutar migraciones
joss migrate

# Crear un nuevo controlador
joss make:controller MiControlador

# Crear un nuevo modelo
joss make:model MiModelo
` + "```" + `

## Desarrollo

1. Edita ` + "`main.joss`" + ` para agregar tu lógica principal
2. Crea controladores en ` + "`app/controllers/`" + `
3. Crea modelos en ` + "`app/models/`" + `
4. Configura variables de entorno en ` + "`env.joss`" + `

## Base de Datos

Por defecto, este proyecto usa SQLite. Para cambiar a MySQL:

1. Edita ` + "`env.joss`" + ` y cambia ` + "`DB=\"mysql\"`" + `
2. Configura las credenciales de MySQL
3. Ejecuta ` + "`joss migrate`" + ` para crear las tablas

## Notas

Este es un proyecto de **consola** (backend-only). No incluye:
- Rutas web (` + "`routes.joss`" + `)
- API REST (` + "`api.joss`" + `)
- Vistas HTML (` + "`app/views/`" + `)
- Assets estáticos (` + "`assets/`" + `)

Si necesitas un proyecto web completo, usa:
` + "```bash" + `
joss new web mi_proyecto_web
` + "```" + `
`,
	}
}

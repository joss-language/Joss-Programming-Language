# CLI de Joss — Referencia Completa de Comandos

[Índice](README.md) · Antes: [primeros pasos](PRIMEROS_PASOS.md) · Después: [analizador](ANALIZADOR.md)

La fuente canónica de comandos es `cmd/joss/main.go`. `joss help` muestra la ayuda interactiva instalada y `joss version` la versión actual del runtime.

La CLI es el programa que recibe comandos en la terminal. Incorpora un entorno REPL
interactivo (`joss repl`), ejecución directa de scripts (`joss run`), servidor web de alto
rendimiento (`joss server start`) y un conjunto completo de herramientas de calidad de código y gestión de paquetes (`pub`).

---

## 1. Ejecución, REPL y Build

```bash
joss run archivo.joss
joss repl
joss server start
joss program start
joss analyze [archivo.joss]
joss update [-f|--canary|--stable]
joss build [web|program|native|package]
joss build native [os] [arch] [--gui]
```

- `run [archivo]`: Ejecuta un script `.joss` después de analizar el proyecto. Los errores semánticos bloquean la ejecución; los warnings no.
- `repl`: Inicia la consola interactiva (Read-Eval-Print Loop) para evaluar expresiones, probar funciones y experimentar con código en tiempo real. Escribe `exit` o presiona `Ctrl+C` para salir.
- `server start`: Requiere el punto de entrada `main.joss` e inicia el servidor HTTP multinivel de alto rendimiento. Presiona `q` para detenerlo de manera segura.
- `program start`: Inicia la aplicación en modo escritorio.
- `analyze [archivo]`: Analiza la entrada (por defecto `main.joss`) y `app/**/*.joss`. No incluye automáticamente `routes.joss`, `api.joss` ni otros hermanos. Devuelve código distinto de cero si existen errores y conserva archivo/línea/columna. Consulte [ANALIZADOR.md](ANALIZADOR.md).
- `build native [os] [arch]`: Genera un binario independiente para `windows`, `linux` o `darwin`; empaqueta el AST serializado y el runner Go. No es un backend LLVM/AOT del programa Joss. Usa `--gui` para aplicaciones con interfaz de escritorio.
- `update`: Usa el actualizador implementado por el CLI y puede requerir red/permisos del sistema. Comprueba sus canales y artefactos reales antes de prometer que una distribución contiene SDK o editor.

---

## 2. Creación de Proyectos (`new`)

```bash
joss new mi_proyecto
joss new web mi_proyecto
joss new console mi_cli
joss new package mi_paquete
joss new plugin mi_plugin
```

- `new web` / `new`: Genera la estructura MVC completa de una aplicación web con motor de vistas, rutas, middleware y ORM.
- `new console`: Genera una plantilla ligera para herramientas de línea de comandos.
- `new package`: Crea una estructura declarativa de paquete para el gestor `pub`.
- `new plugin`: Crea un proyecto de plugin multilenguaje oficial traducible a bytecode binario `.jp`.

---

## 3. Generadores de Código (`make:*` y `remove:*`)

```bash
joss make:controller Users
joss make:middleware AuthGuard
joss make:model User
joss make:view users/index
joss make:mvc Product
joss make:crud products
joss remove:crud products
joss make:migration create_products
```

### 🛠️ `make:crud [Tabla]` (Generador Relacional Inteligente)
Conecta a la base de datos configurada en `env.joss`, inspecciona el esquema de la tabla y genera automáticamente un módulo administrativo completo:
1. **Inspección de Claves Foráneas (`_id`)**: Detecta relaciones con otras tablas, infiere nombres de modelos relacionales y auto-detecta columnas visibles (`username`, `name`, `title`).
2. **Modelo y Modelos Relacionados**: Genera `app/models/Model.joss` y cualquier modelo relacional faltante.
3. **Controlador CRUD completo**: Genera `app/controllers/ModelController.joss` con métodos `index`, `create`, `store`, `edit`, `update` y `delete` que incluyen `joins` y `selects` automáticos.
4. **Vistas Tailwind CSS`: Genera `app/views/model/index.joss.html`, `create.joss.html` y `edit.joss.html` con formularios dinámicos y menús desplegables `<select>` para relaciones.
5. **Inyección en Navbar y Rutas**: Inyecta la opción en `app/views/layouts/master.joss.html` e inserta las rutas protegidas dentro del grupo `Router::middleware("auth")` en `routes.joss`.

El comando sólo se admite en proyectos web y requiere que la tabla ya exista.
Los nombres de tabla/columnas se validan como identificadores antes de consultar
el esquema. El controlador generado acepta únicamente las columnas editables
descubiertas (no hace asignación masiva), el borrado usa `POST` con CSRF y volver
a ejecutar el generador no duplica rutas ni enlaces de navegación.

### 🗑️ `remove:crud [Tabla]`
Deshace limpiamente la generación: elimina el controlador, modelo, carpeta de vistas y remueve las rutas inyectadas en `routes.joss` y el enlace del navbar.

---

## 4. Base de Datos y Migraciones

```bash
joss make:migration create_users_table
joss migrate
joss migrate:fresh
joss db:seed
joss change db mysql
joss change db sqlite
joss change db prefix app_
joss change db migrate --host=HOST --port=3306 --database=DB --user=USER --password=PASS
```

- `make:migration`: Genera una nueva migración con timestamp en `app/database/migrations/`. `create_users`, `create_users_table` y `user` se normalizan a la tabla lógica `users`; `make:miggrate` no es un alias y muestra la corrección sugerida.
- `migrate`: Ejecuta las migraciones pendientes en orden cronológico.
- `migrate:fresh`: Elimina todas las tablas de la base de datos y vuelve a ejecutar todas las migraciones desde cero.
- `db:seed`: Ejecuta los seeders pobladores definidos en `app/database/seeders/`.
- `change db`: Cambia el motor configurado (`mysql` o `sqlite`) o modifica el prefijo global de tablas (`DB_PREFIX`).
- `change db migrate`: Migra en caliente los datos y estructura de la conexión actual hacia un nuevo servidor MySQL remoto.

---

## 5. Compilación y Gestión de Plugins (`.jp`)

```bash
joss plugin compile .
joss plugin compile script.py --lang=python --name=mi_plugin --exports=calcular
joss plugin inspect mi_plugin.jp
joss plugin verify mi_plugin.jp
```

- `plugin compile`: Produce JPBC firmado desde backends parciales. Python/PHP/Java traducen subconjuntos; la ruta Wasm sólo valida la cabecera y genera stubs de demostración. Consulte [Plugins](PLUGINS.md).
- `plugin inspect`: Muestra metadatos, permisos declarados y tabla de símbolos del paquete `.jp`.
- `plugin verify`: Comprueba la firma digital Ed25519 y la integridad estructural del contenedor `.jp`.

---

## 6. Gestor de Paquetes (`pub`)

```bash
joss pub add paquete ^1.2.0
joss pub remove paquete
joss pub install
joss pub install --offline
joss pub update
joss pub search termino
joss pub info paquete
joss pub publish
joss pub login
joss pub logout
joss pub cache clean
```

Si no se especifica `PUB_REGISTRY_URL`, Pub resuelve las dependencias utilizando el registro oficial en `https://joss.red`.

---

## 7. Calidad de Código y Tooling (`check`, `format`, `lint`, `fix`, `test`)

```bash
joss check [ruta]
joss format [ruta] [--write|--check]
joss lint [ruta] [--json]
joss fix [ruta] [--dry-run]
joss test [--filter=nombre] [ruta]
```

- `check`: Ejecuta formato, sintaxis, análisis y linter. Revisa su salida: un problema de formato se comunica como warning en este pipeline.
- `format`: En un archivo, modifica por defecto salvo que uses `--check`; en un directorio sólo escribe con `--write`. Usa siempre `joss format ruta --check` en CI.
- `lint`: Ejecuta análisis estático con reglas de consistencia de tipos, estilo y detección de secretos o credenciales en código duro (`--json` para integración estructurada).
- `fix`: Aplica correcciones automáticas seguras (visibilidad requerida, formateo) con soporte para `--dry-run`.
- `test`: Ejecuta archivos `*_test.joss` con `test`/`it`, `assert`, `assertTrue`, `assertFalse`, `assertEqual`, `assertNotEqual`, `assertNull`, `assertNotNull` y `assertThrows`. Coloca `--filter` antes de la ruta: el parser de flags deja de leer opciones tras el primer argumento posicional.

---

## 8. Comandos de Plugins y Ayuda Dinámica

```bash
joss help plugins
joss help plugins [nombre_plugin]
joss ai:activate
joss brevo:config [--enable|--disable] [--api-key=CLAVE]
joss backup:create [destino]
joss backup:restore <archivo.zip>
joss bg:remove <input.jpg> [output.png]
joss notify:send <canal> <mensaje>
```

- `help plugins`: Muestra todos los plugins instalados y disponibles junto con sus comandos CLI expuestos y su estado (`[protegido]`).
- `help plugins [nombre_plugin]`: Muestra la ficha técnica, repositorio, opciones y comandos específicos del plugin seleccionado.
- **Comandos de plugins**: Los plugins instalados en `plugins/` o declarados en `joss.yaml` pueden registrar y despachar comandos CLI autónomos y protegidos.

---

## 9. Almacenamiento y Servicios en la Nube (`userstorage`)

```bash
joss userstorage local
joss userstorage oci
joss userstorage sync-oci
joss userstorage sync-local
```

- `userstorage`: Conmuta el proveedor de almacenamiento entre disco local y **Oracle Cloud Infrastructure (OCI)**, permitiendo sincronización bidireccional mediante `sync-oci` y `sync-local`.

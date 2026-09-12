<p align="center">
  <img src="assets/logo.png" alt="Logo de Joss Language" width="150" height="150">
</p>

<h1 align="center">El Lenguaje de Programación Joss</h1>

<p align="center">
  <b>Moderno · Fuertemente Tipado · Alto Rendimiento · Cero Imports · Pila Web Nativa</b>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/jossecurity/joss"><img src="https://goreportcard.com/badge/github.com/jossecurity/joss" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://joss.red/docs"><img src="https://img.shields.io/badge/docs-joss.red-teal.svg" alt="Docs"></a>
  <a href="https://github.com/jossecurity/joss/actions"><img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build Status"></a>
</p>

---

**Joss** es un lenguaje de programación moderno, tipado y de alto rendimiento implementado en Go. Está diseñado para el desarrollo ágil de servicios backend, APIs concurrentes, aplicaciones web de gran escala y herramientas de línea de comandos seguras y deterministas.

---

## 🚀 ¿Por qué elegir Joss?

- **Cero imports en el código (Zero-Imports)**: Olvídate de gestionar rutas relativas o árboles interminables de `import`/`require`. Joss descubre, analiza y organiza automáticamente todas las clases y funciones públicas de tu proyecto y plugins.
- **Análisis estático exhaustivo (`joss analyze`)**: El verificador semántico de Joss analiza tipos de datos, rutas de retorno de funciones, scopes y contratos nominales antes de ejecutar una sola línea.
- **Sintaxis limpia y sin fricción**: Diseñado con delimitación natural por **saltos de línea**. No necesitas colocar punto y coma (`;`) al final de cada sentencia salvo que decidas escribir varias en la misma línea.
- **Pila web y backend integrada (Batteries-Included)**: Servidor HTTP multinivel nativo, enrutamiento dinámico, motor de vistas HTML dinámico con escape anti-XSS, ORM y migraciones versionadas (GranDB / Schema), autenticación, sesiones seguras y WebSockets.
- **Concurrencia limpia con Canales y Async**: Ejecuta tareas en segundo plano con bloques `async { ... }`, espera resultados con `await` y comunica procesos de forma segura mediante canales (`channel`).
- **Aritmética financiera exacta**: Incorpora el tipo primitivo `decimal` para cálculos monetarios y de precisión en base diez, previniendo errores de redondeo del estándar binario IEEE-754.

---

## 📦 Instalación Rápida

### Instalador automático oficial

En **Windows** (PowerShell):
```powershell
iwr -useb https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.ps1 | iex
```

En **Linux o macOS** (Terminal):
```bash
curl -fsSL https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.sh | bash
```

### Compilar desde el código fuente

Si dispones de [Go](https://go.dev) (versión 1.22 o superior):
```bash
git clone https://github.com/jossecurity/joss.git
cd joss
go build -o joss ./cmd/joss
```

Comprueba que la instalación sea correcta:
```bash
joss version
```

---

## ⚡ Tu primer programa en 30 segundos

Crea un archivo llamado `hola.joss`:

<!-- joss-run: ["Hola, Joss!"] -->
```joss
print("Hola, Joss!")
```

Verifícalo y ejecútalo directamente desde tu terminal:

```bash
joss analyze hola.joss
joss run hola.joss
```

Salida esperada:
```text
Hola, Joss!
```

---

## 🎨 Un vistazo a la sintaxis de Joss

Joss combina claridad visual, tipado estático riguroso y expresividad funcional:

<!-- joss-run: ["Hola, Ada", "Puedes participar"] -->
```joss
public func saludar(string $nombre): string {
    return "Hola, " . $nombre
}

$edad = 20
print(saludar("Ada"))
print(($edad >= 18) ? "Puedes participar" : "Aún debes esperar")
```

### Reglas sintácticas clave
1. **Prefijo `$` en variables**: Todas las variables inician obligatoriamente con el signo `$`.
2. **Delimitación por saltos de línea**: Una sentencia por línea no requiere punto y coma (`;`). Si necesitas escribir múltiples sentencias en una misma línea, sepáralas explícitamente con `;`.
3. **Inferencia y tipado**:
   - Inferencia fija: `$total = 100` (infiere y fija el tipo `int`).
   - Declaración explícita: `int $contador = 0` o `string $mensaje = "Hola"`.
   - Dinamismo explícito: `mixed $variable = "dinámico"` (permite cambiar de tipo posteriormente).
   - Constantes inmutables: `const $pi = 3.1416` o `const int $max = 100`.
4. **Concatenación con punto (`.`)**: El operador `.` concatena cadenas de texto; el operador `+` se reserva estrictamente para la suma numérica.
5. **Decisiones elegantes**: Expresiones ternarias concisas con bloques `(condición) ? { ... } : { ... }` y coincidencia de patrones con `match ($valor) { ... }`.

---

## 🛠️ Comandos esenciales de la CLI

| Comando | Descripción |
|---|---|
| `joss init [nombre]` | Crea un nuevo proyecto con estructura estandarizada y configuración |
| `joss run <archivo.joss>` | Ejecuta un archivo o script de Joss |
| `joss analyze [ruta]` | Ejecuta el análisis semántico y comprobación exhaustiva de tipos |
| `joss lint [ruta]` | Inspecciona el código en busca de advertencias, desuso y buenas prácticas |
| `joss fmt [ruta]` | Formatea el código fuente según las reglas canónicas del lenguaje |
| `joss test` | Ejecuta la suite de pruebas unitarias y de integración del proyecto |
| `joss serve` | Inicia el servidor web nativo de alto rendimiento |
| `joss pub [comando]` | Administra paquetes y plugins del ecosistema Joss |

---

## 🗺️ Mapa de la Documentación

La documentación oficial se organiza en cuatro áreas temáticas:

### 1. Aprender Joss desde cero (Tutorial guiado)
- 0. [Primeros pasos: De un archivo a un programa](docs/PRIMEROS_PASOS.md)
- 1. [Valores, variables y operaciones fundamentales](docs/FUNDAMENTOS.md)
- 2. [Control de flujo, decisiones y bucles](docs/CONTROL_FLUJO.md)
- 3. [Funciones, ámbito (scope), closures y referencias](docs/FUNCIONES.md)
- 4. [Colecciones: Arrays, Maps y texto Unicode](docs/COLECCIONES.md)
- 5. [Sistema de tipos, inferencia y conversiones](docs/SISTEMA_TIPOS.md)
- 6. [Clases, objetos, métodos y herencia](docs/CLASES.md)
- 7. [Manejo de errores, excepciones y try/catch](docs/ERRORES.md)
- 8. [Concurrencia, asincronía, Future y canales](docs/CONCURRENCIA.md)
- 9. [Proyecto práctico: Aplicación de consola con persistencia JSON](docs/PROYECTO_CONSOLA.md)
- 10. [Proyecto práctico: Aplicación web MVC con el stack nativo](docs/PROYECTO_WEB.md)
- [Glosario completo de términos de programación](docs/GLOSARIO.md)

### 2. Referencia técnica exhaustiva
- [Sintaxis, tokens y precedencia de operadores](docs/SINTAXIS.md)
- [Gramática EBNF formal y correspondencia con el AST](docs/GRAMATICA.md)
- [Catálogo y referencia de diagnósticos (JOSS-*)](docs/DIAGNOSTICOS.md)
- [Guía de las 117 funciones globales integradas](docs/FUNCIONES_GLOBALES.md)
- [Clases nativas y servicios del runtime](docs/MODULOS_NATIVOS.md)
- [Catálogo nativo generado por docgen](docs/CATALOGO_NATIVO.md)
- [Referencia de comandos de la CLI (`joss`)](docs/CLI.md)
- [Extensión oficial para Visual Studio Code](docs/VSCODE_EXTENSION.md)
- [Estado real de implementación y límites del sistema](docs/ESTADO_IMPLEMENTACION.md)

### 3. Desarrollo de aplicaciones reales
- [Estructura de proyectos y convenciones](docs/ESTRUCTURA_PROYECTO.md)
- [Configuración y variables de entorno](docs/CONFIGURACION.md)
- [Carga automática y Zero Imports](docs/MODULOS_IMPORTS.md)
- [Arquitectura de plugins y paquetes binarios JP](docs/PLUGINS.md)
- [Servidor HTTP nativo de alto rendimiento](docs/SERVIDOR.md)
- [Controladores web y peticiones HTTP](docs/CONTROLADORES.md)
- [Middlewares y capas de seguridad](docs/MIDDLEWARE.md)
- [Motor de vistas y plantillas HTML dinámicas](docs/VISTAS.md)
- [Gestión y compilación de assets](docs/ASSETS.md)
- [WebSockets y comunicación en tiempo real](docs/WEBSOCKETS.md)
- [Modelos relacionales y consultas GranDB](docs/MODELOS.md)
- [Schema Builder y migraciones versionadas](docs/SCHEMA_BUILDER.md)
- [Sistema de autenticación, sesiones y MFA](docs/AUTENTICACION.md)

### 4. Arquitectura interna y contribución
- [Arquitectura del compilador, analizador y runtime](docs/ARQUITECTURA.md)
- [Guía para contribuidores del núcleo](docs/CONTRIBUIR.md)
- [Informe de auditoría integral y reconstrucción documental](docs/DOCUMENTATION_AUDIT.md)
- [Auditoría técnica y optimización del runtime](docs/AUDITORIA_TECNICA_2026.md)

---

## 🤝 Cómo Contribuir

¡Toda contribución es bienvenida! Antes de comenzar a trabajar en cambios al compilador, analizador o documentación:

1. Lee la [Guía de arquitectura y reglas operativas (AGENTS.md)](AGENTS.md) y la [Guía de contribución](docs/CONTRIBUIR.md).
2. Utiliza las **plantillas oficiales de GitHub**:
   - **Reportar un error**: Completa el formulario de [Reporte de Error (.github/ISSUE_TEMPLATE/bug_report.yml)](.github/ISSUE_TEMPLATE/bug_report.yml) incluyendo obligatoriamente la versión de Joss (`joss version`), sistema operativo y un código mínimo reproducible en `.joss`.
   - **Proponer una mejora**: Utiliza la plantilla de [Propuesta de Mejora (.github/ISSUE_TEMPLATE/feature_request.yml)](.github/ISSUE_TEMPLATE/feature_request.yml) describiendo la motivación y ejemplos de sintaxis.
   - **Enviar un Pull Request**: Cumple con la lista de verificación en [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md).

### Ejecutar validaciones locales obligatorias

```bash
# Formateo canónico
gofmt -w <archivos-modificados>

# Verificación de generadores automáticos
go run ./tools/cataloggen --check
go run ./tools/docgen --check

# Pruebas y análisis estático
go vet ./...
go test ./...
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core ./pkg/server

# Verificación de contratos y documentación
go test ./pkg/core -run TestDocumentation -v
```

---

## 📜 Licencia y Seguridad

Joss es software libre publicado bajo los términos de la [Licencia MIT](LICENSE). Para reportar vulnerabilidades de seguridad de forma privada y responsable, consulta la directriz en [SECURITY.md](SECURITY.md).

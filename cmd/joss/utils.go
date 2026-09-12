package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
)

func GetEnvFile() string {
	return core.GetEnvFile()
}

func readEnvFile(path string) map[string]string {
	return core.ReadEnvFile(path)
}

func updateEnvFile(path, key, value string) {
	_ = core.UpdateEnvFile(path, key, value)
}

func printHelp(topics ...string) {
	if len(topics) > 0 {
		topic := strings.ToLower(topics[0])
		if topic == "plugins" {
			targetPlugin := ""
			if len(topics) > 1 {
				targetPlugin = topics[1]
			}
			handlePluginHelp(targetPlugin)
			return
		}
		if topic == "plugin" && len(topics) > 1 {
			handlePluginHelp(topics[1])
			return
		}
		printTopicHelp(topic)
		return
	}

	fmt.Println(i18n.Tr("cliAppTitle"))
	fmt.Println(i18n.Tr("cliUsage"))
	fmt.Println()
	fmt.Println("EJECUCIÓN Y REPL:")
	fmt.Printf("  run <archivo.joss>             %s\n", i18n.Tr("cliCmdRun"))
	fmt.Printf("  repl                           %s\n", i18n.Tr("cliCmdRepl"))
	fmt.Printf("  server start                   %s\n", i18n.Tr("cliCmdServer"))
	fmt.Printf("  program start                  %s\n", i18n.Tr("startProgramDesktop"))
	fmt.Printf("  build [web|program|native]     %s\n", i18n.Tr("cliCmdBuild"))
	fmt.Println("    build native [os] [arch] [--gui]")
	fmt.Println()
	fmt.Println("CALIDAD DE CÓDIGO Y TOOLING:")
	fmt.Println("  check [ruta]                   Verificación integral (formato, sintaxis, tipos y lint)")
	fmt.Printf("  format [ruta] [--write|--check] %s\n", i18n.Tr("cliCmdFormat"))
	fmt.Printf("  lint [ruta] [--json]           %s\n", i18n.Tr("cliCmdLint"))
	fmt.Println("  fix [ruta] [--dry-run]         Aplica correcciones automáticas seguras y formato")
	fmt.Printf("  analyze [archivo]              %s\n", i18n.Tr("cliCmdAnalyze"))
	fmt.Printf("  test [ruta] [--filter=nombre]  %s\n", i18n.Tr("cliCmdTest"))
	fmt.Println()
	fmt.Println("GENERADORES Y ESTRUCTURA (SCAFFOLDING):")
	fmt.Printf("  new [web|console|package|plugin] <ruta>  %s\n", i18n.Tr("createProject"))
	fmt.Println("  make:controller <Nombre>       Genera un controlador web")
	fmt.Println("  make:model <Nombre>            Genera un modelo de datos")
	fmt.Println("  make:view <Nombre>             Genera una plantilla de vista")
	fmt.Println("  make:middleware <Nombre>       Genera un middleware HTTP")
	fmt.Println("  make:mvc <Nombre>              Genera Modelo, Vista y Controlador en un paso")
	fmt.Println("  make:crud <Tabla>              Genera CRUD completo con rutas y vistas")
	fmt.Printf("  remove:crud <Tabla>            %s\n", i18n.Tr("removeCRUD"))
	fmt.Printf("  make:migration <Nombre>        %s\n", i18n.Tr("createMigration"))
	fmt.Println()
	fmt.Println("BASE DE DATOS Y STORAGE:")
	fmt.Printf("  migrate                        %s\n", i18n.Tr("exeMigrate"))
	fmt.Printf("  migrate:fresh                  %s\n", i18n.Tr("exeMigrateFresh"))
	fmt.Println("  db:seed                        Ejecuta seeders de app/database/seeders")
	fmt.Printf("  change db [motor]              %s\n", i18n.Tr("changeDBMotor"))
	fmt.Println("  change db migrate              Migra la conexión actual a un nuevo MySQL")
	fmt.Printf("  change db prefix <prefijo>     %s\n", i18n.Tr("changeDBPrefix"))
	fmt.Printf("  userstorage [local|oci]        %s\n", i18n.Tr("settingsUserStorage"))
	fmt.Println("  userstorage sync-oci|sync-local Sincroniza archivos con Oracle Cloud")
	fmt.Println()
	fmt.Println("PAQUETES Y PLUGINS:")
	fmt.Printf("  pub <add|remove|install|publish|search|update>  %s\n", i18n.Tr("cliCmdPub"))
	fmt.Println("  plugin compile <fuente/dir>    Compila código a paquete binario .jp")
	fmt.Println("  plugin inspect <archivo.jp>    Inspecciona bytecode y símbolos de un plugin")
	fmt.Println("  plugin verify <archivo.jp>     Verifica firmas Ed25519 e integridad de un plugin")
	fmt.Println("  package inspect <archivo.jp>   Inspecciona metadatos y firmas de un paquete")
	fmt.Println("  help plugins [nombre]          Lista comandos y opciones provistos por plugins")
	fmt.Println()
	fmt.Println("SISTEMA Y UTILIDADES:")
	fmt.Printf("  version                        %s\n", i18n.Tr("version"))
	fmt.Println("  update [-f|--canary|--stable]  Actualiza la versión de Joss, SDK y plugins")
	fmt.Println("  ai:activate                    Configura proveedores y modelos de Inteligencia Artificial")
	fmt.Printf("  help [comando]                 %s\n", i18n.Tr("helpPrint"))
	fmt.Println()
	fmt.Println("Usa 'joss help <comando>' o 'joss help plugins' para ver opciones y ejemplos.")
}

func printTopicHelp(cmd string) {
	switch strings.ToLower(cmd) {
	case "run":
		fmt.Println("Uso: joss run <archivo.joss>")
		fmt.Println("Ejecuta un script Joss directamente tras validar sintaxis y análisis semántico.")
		fmt.Println("Los errores de tipos bloquean la ejecución; los warnings se reportan sin detenerla.")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss run main.joss")
		fmt.Println("  joss run script.joss")

	case "repl":
		fmt.Println("Uso: joss repl")
		fmt.Println("Inicia la consola interactiva (Read-Eval-Print Loop) de Joss.")
		fmt.Println("Permite evaluar expresiones, probar funciones, declarar variables y experimentar en tiempo real.")
		fmt.Println("\nComandos interactivos:")
		fmt.Println("  exit, quit     Sale del REPL interactivo")
		fmt.Println("  Ctrl+C         Interrumpe la línea actual o sale del REPL")

	case "server":
		fmt.Println("Uso: joss server start")
		fmt.Println("Inicia el servidor web HTTP de alto rendimiento ejecutando el punto de entrada 'main.joss'.")
		fmt.Println("Carga automáticamente todos los controladores, modelos y rutas en app/.")
		fmt.Println("Presiona 'q' o Ctrl+C en la consola para detener el servidor de forma segura.")

	case "program":
		fmt.Println("Uso: joss program start")
		fmt.Println("Inicia la aplicación en modo escritorio.")

	case "build":
		fmt.Println("Uso: joss build [web|program|native|package] [opciones]")
		fmt.Println("Compila o empaqueta el proyecto para producción y distribución.")
		fmt.Println("\nModos:")
		fmt.Println("  web                            Prepara los assets y archivos para despliegue web")
		fmt.Println("  program                        Prepara la versión de escritorio")
		fmt.Println("  native [os] [arch] [--gui]     Compila un ejecutable nativo autocontenido")
		fmt.Println("  package <ruta>                 Empaqueta una biblioteca Joss en formato .jp")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss build web")
		fmt.Println("  joss build native windows amd64 --gui")
		fmt.Println("  joss build native linux arm64")

	case "analyze":
		fmt.Println("Uso: joss analyze [archivo.joss]")
		fmt.Println("Realiza un análisis semántico estricto del proyecto sin ejecutarlo.")
		fmt.Println("Verifica coherencia de tipos, variables no declaradas, firmas y flujo de control.")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss analyze")
		fmt.Println("  joss analyze main.joss")

	case "format":
		fmt.Println("Uso: joss format [ruta] [opciones]")
		fmt.Println("Formatea archivos .joss según el estándar canónico del lenguaje.")
		fmt.Println("\nOpciones:")
		fmt.Println("  --write, -w    Escribe los cambios en el archivo (por defecto al pasar un archivo)")
		fmt.Println("  --check, -c    Verifica si los archivos están formateados sin modificarlos (útil en CI)")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss format app/controllers/UserController.joss")
		fmt.Println("  joss format --write .")
		fmt.Println("  joss format --check .")

	case "lint":
		fmt.Println("Uso: joss lint [ruta] [opciones]")
		fmt.Println("Ejecuta análisis estático y detecta problemas de estilo, tipos y seguridad.")
		fmt.Println("\nOpciones:")
		fmt.Println("  --json         Emite el reporte de diagnósticos en formato JSON estructurado")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss lint .")
		fmt.Println("  joss lint main.joss")
		fmt.Println("  joss lint --json .")

	case "fix":
		fmt.Println("Uso: joss fix [ruta] [opciones]")
		fmt.Println("Aplica correcciones automáticas seguras y formateo canónico.")
		fmt.Println("\nOpciones:")
		fmt.Println("  --dry-run, -d  Muestra qué cambios se aplicarían sin modificar los archivos")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss fix .")
		fmt.Println("  joss fix --dry-run .")

	case "check":
		fmt.Println("Uso: joss check [ruta]")
		fmt.Println("Ejecuta una verificación integral de calidad en 1 paso:")
		fmt.Println("  1. Comprobación de formato canónico")
		fmt.Println("  2. Chequeo de sintaxis y parseo")
		fmt.Println("  3. Análisis semántico estricto y tipos")
		fmt.Println("  4. Reglas de linter y seguridad")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss check .")

	case "test":
		fmt.Println("Uso: joss test [ruta] [opciones]")
		fmt.Println("Ejecuta la suite oficial de pruebas unitarias y aserciones (*_test.joss).")
		fmt.Println("\nOpciones:")
		fmt.Println("  --filter, -f   Filtra y ejecuta solo las pruebas cuyo nombre contenga el texto")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss test")
		fmt.Println("  joss test tests/")
		fmt.Println("  joss test --filter=login")

	case "new":
		fmt.Println("Uso: joss new [web|console|package|plugin] <ruta/nombre>")
		fmt.Println("Genera una nueva estructura de proyecto.")
		fmt.Println("\nTipos:")
		fmt.Println("  web       Proyecto web MVC completo (por defecto)")
		fmt.Println("  console   Proyecto de consola CLI")
		fmt.Println("  package   Paquete distribuible para el ecosistema Joss")
		fmt.Println("  plugin    Plugin nativo con compilación a bytecode .jp")

	case "make:controller", "controller":
		fmt.Println("Uso: joss make:controller <Nombre>")
		fmt.Println("Genera un nuevo controlador web en app/controllers/.")
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:controller UserController")

	case "make:model", "model":
		fmt.Println("Uso: joss make:model <Nombre>")
		fmt.Println("Genera un nuevo modelo ORM en app/models/.")
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:model User")

	case "make:view", "view":
		fmt.Println("Uso: joss make:view <Ruta/Nombre>")
		fmt.Println("Genera una nueva plantilla de vista en app/views/.")
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:view users/profile")

	case "make:middleware", "middleware":
		fmt.Println("Uso: joss make:middleware <Nombre>")
		fmt.Println("Genera un nuevo middleware HTTP en app/middleware/.")
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:middleware AuthGuard")

	case "make:mvc", "mvc":
		fmt.Println("Uso: joss make:mvc <Nombre>")
		fmt.Println("Genera simultáneamente Modelo, Vista y Controlador para la entidad.")
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:mvc Product")

	case "make:crud", "crud":
		fmt.Println("Uso: joss make:crud <Tabla>")
		fmt.Println("Genera automáticamente Modelo, Vistas, Controlador y Rutas para una tabla de base de datos.")
		fmt.Println("Detecta claves foráneas, columnas visibles e inyecta enlaces de navegación.")

	case "remove:crud":
		fmt.Println("Uso: joss remove:crud <Tabla>")
		fmt.Println("Elimina limpiamente un módulo CRUD generado:")
		fmt.Println("borra el controlador, modelo, vistas y retira las rutas y enlaces del navbar.")
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss remove:crud products")

	case "make:migration", "migration":
		fmt.Println("Uso: joss make:migration <Nombre>")
		fmt.Println("Genera un nuevo archivo de migración con timestamp en app/database/migrations/.")
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss make:migration create_users_table")
		fmt.Println("  joss make:migration add_avatar_to_users")

	case "migrate":
		fmt.Println("Uso: joss migrate")
		fmt.Println("Aplica las migraciones pendientes en app/database/migrations.")
		fmt.Println("Usa 'joss migrate:fresh' para reiniciar la base de datos y migrar desde cero.")

	case "migrate:fresh":
		fmt.Println("Uso: joss migrate:fresh")
		fmt.Println("Elimina todas las tablas de la base de datos y vuelve a ejecutar todas las migraciones.")
		fmt.Println("Útil durante el desarrollo para reiniciar el esquema.")

	case "db:seed", "seed":
		fmt.Println("Uso: joss db:seed")
		fmt.Println("Ejecuta los seeders pobladores definidos en app/database/seeders.")

	case "change", "db":
		fmt.Println("Uso: joss change db [motor|migrate|prefix]")
		fmt.Println("Configura o modifica la conexión de base de datos del proyecto.")
		fmt.Println("\nSubcomandos:")
		fmt.Println("  change db <motor>           Cambia el motor (sqlite, mysql)")
		fmt.Println("  change db prefix <prefijo>  Establece el prefijo global de tablas")
		fmt.Println("  change db migrate           Migra los datos y esquema a un nuevo MySQL")

	case "userstorage", "storage":
		fmt.Println("Uso: joss userstorage [local|oci|sync-oci|sync-local]")
		fmt.Println("Configura y sincroniza el almacenamiento de archivos del proyecto.")
		fmt.Println("\nOpciones:")
		fmt.Println("  local        Configura almacenamiento en el disco local")
		fmt.Println("  oci          Configura almacenamiento en Oracle Cloud Infrastructure")
		fmt.Println("  sync-oci     Sube los archivos locales al bucket OCI")
		fmt.Println("  sync-local   Descarga los archivos de OCI al disco local")

	case "pub":
		fmt.Println("Uso: joss pub <subcomando> [paquete]")
		fmt.Println("Gestor de dependencias y paquetes de Joss.")
		fmt.Println("\nSubcomandos:")
		fmt.Println("  add <paquete>      Añade una dependencia a joss.yaml e instálala")
		fmt.Println("  remove <paquete>   Elimina una dependencia de joss.yaml")
		fmt.Println("  install            Descarga e instala las dependencias de joss.yaml")
		fmt.Println("  update             Actualiza las dependencias instaladas")
		fmt.Println("  publish            Publica tu paquete en el registro oficial")

	case "plugin":
		printPluginUsage()
		fmt.Println("\nNota: Para ver plugins instalados y sus comandos, usa 'joss help plugins'.")

	case "package":
		fmt.Println("Uso: joss package inspect <archivo.jp>")
		fmt.Println("Inspecciona metadatos, símbolos y dependencias de un paquete Joss compilado.")

	case "update":
		fmt.Println("Uso: joss update [opciones]")
		fmt.Println("Actualiza el binario de Joss, SDK y herramientas oficiales.")
		fmt.Println("\nOpciones:")
		fmt.Println("  -f, --force    Fuerza la reinstalación")
		fmt.Println("  --canary       Actualiza a la versión de desarrollo más reciente")
		fmt.Println("  --stable       Actualiza a la última versión estable (por defecto)")

	case "version":
		fmt.Println("Uso: joss version")
		fmt.Println("Muestra la versión de Joss instalada y su nombre en clave.")

	case "ai:activate", "ai":
		fmt.Println("Uso: joss ai:activate")
		fmt.Println("Configura proveedores y modelos de Inteligencia Artificial (Groq, OpenAI, Gemini).")

	default:
		fmt.Printf("No hay ayuda específica para el comando '%s'.\n\n", cmd)
		printHelp()
	}
}

// readLine reads a line of input from stdin in a platform-independent way, handling \r, \r\n and \n.
func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	var line []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		if b == '\n' {
			break
		}
		if b == '\r' {
			next, err := reader.Peek(1)
			if err == nil && next[0] == '\n' {
				_, _ = reader.ReadByte()
			}
			break
		}
		line = append(line, b)
	}
	return strings.TrimSpace(string(line))
}

func getCLIOption(flag string) string {
	flagPrefix := "--" + flag + "="
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, flagPrefix) {
			return strings.TrimPrefix(arg, flagPrefix)
		}
	}
	for i, arg := range os.Args[1:] {
		if arg == "--"+flag && i+1 < len(os.Args[1:]) {
			return os.Args[1:][i+1]
		}
	}
	return ""
}

func removeEnvKey(path, key string) {
	_ = core.RemoveEnvKey(path, key)
}

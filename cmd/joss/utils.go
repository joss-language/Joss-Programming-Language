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
	fmt.Printf("%s joss <comando> [argumentos] [opciones]\n", i18n.Tr("cliUsageLabel"))
	fmt.Println()
	fmt.Println(i18n.Tr("cliSectionExec"))
	fmt.Printf("  run <archivo.joss>             %s\n", i18n.Tr("cliCmdRun"))
	fmt.Printf("  repl                           %s\n", i18n.Tr("cliCmdRepl"))
	fmt.Printf("  server start                   %s\n", i18n.Tr("cliCmdServer"))
	fmt.Printf("  program start                  %s\n", i18n.Tr("startProgramDesktop"))
	fmt.Printf("  build [web|program|native]     %s\n", i18n.Tr("cliCmdBuild"))
	fmt.Println("    build native [os] [arch] [--gui]")
	fmt.Println()
	fmt.Println(i18n.Tr("cliSectionQuality"))
	fmt.Printf("  check [ruta]                   %s\n", i18n.Tr("cliCmdCheck"))
	fmt.Printf("  format [ruta] [--write|--check] %s\n", i18n.Tr("cliCmdFormat"))
	fmt.Printf("  lint [ruta] [--json]           %s\n", i18n.Tr("cliCmdLint"))
	fmt.Printf("  fix [ruta] [--dry-run]         %s\n", i18n.Tr("cliCmdFix"))
	fmt.Printf("  analyze [archivo]              %s\n", i18n.Tr("cliCmdAnalyze"))
	fmt.Printf("  test [ruta] [--filter=nombre]  %s\n", i18n.Tr("cliCmdTest"))
	fmt.Println()
	fmt.Println(i18n.Tr("cliSectionScaffolding"))
	fmt.Printf("  new [web|console|package|plugin] <ruta>  %s\n", i18n.Tr("createProject"))
	fmt.Printf("  make:controller <Nombre>       %s\n", i18n.Tr("cliCmdMakeController"))
	fmt.Printf("  make:model <Nombre>            %s\n", i18n.Tr("cliCmdMakeModel"))
	fmt.Printf("  make:view <Nombre>             %s\n", i18n.Tr("cliCmdMakeView"))
	fmt.Printf("  make:middleware <Nombre>       %s\n", i18n.Tr("cliCmdMakeMiddleware"))
	fmt.Printf("  make:mvc <Nombre>              %s\n", i18n.Tr("cliCmdMakeMvc"))
	fmt.Printf("  make:crud <Tabla>              %s\n", i18n.Tr("cliCmdMakeCrud"))
	fmt.Printf("  remove:crud <Tabla>            %s\n", i18n.Tr("removeCRUD"))
	fmt.Printf("  make:migration <Nombre>        %s\n", i18n.Tr("createMigration"))
	fmt.Println()
	fmt.Println(i18n.Tr("cliSectionDatabase"))
	fmt.Printf("  migrate                        %s\n", i18n.Tr("exeMigrate"))
	fmt.Printf("  migrate:fresh                  %s\n", i18n.Tr("exeMigrateFresh"))
	fmt.Printf("  db:seed                        %s\n", i18n.Tr("cliCmdDbSeed"))
	fmt.Printf("  change db [motor]              %s\n", i18n.Tr("changeDBMotor"))
	fmt.Printf("  change db migrate              %s\n", i18n.Tr("cliCmdChangeDbMigrate"))
	fmt.Printf("  change db prefix <prefijo>     %s\n", i18n.Tr("changeDBPrefix"))
	fmt.Printf("  userstorage [local|oci]        %s\n", i18n.Tr("settingsUserStorage"))
	fmt.Printf("  userstorage sync-oci|sync-local %s\n", i18n.Tr("cliCmdStorageSync"))
	fmt.Println()
	fmt.Println(i18n.Tr("cliSectionPackages"))
	fmt.Printf("  pub <add|remove|install|publish|search|update>  %s\n", i18n.Tr("cliCmdPub"))
	fmt.Printf("  plugin compile <fuente/dir>    %s\n", i18n.Tr("cliCmdPluginCompile"))
	fmt.Printf("  plugin inspect <archivo.jp>    %s\n", i18n.Tr("cliCmdPluginInspect"))
	fmt.Printf("  plugin verify <archivo.jp>     %s\n", i18n.Tr("cliCmdPluginVerify"))
	fmt.Printf("  package inspect <archivo.jp>   %s\n", i18n.Tr("cliCmdPackageInspect"))
	fmt.Printf("  help plugins [nombre]          %s\n", i18n.Tr("cliCmdHelpPlugins"))
	fmt.Println()
	fmt.Println(i18n.Tr("cliSectionSystem"))
	fmt.Printf("  version                        %s\n", i18n.Tr("version"))
	fmt.Printf("  update [-f|--canary|--stable]  %s\n", i18n.Tr("cliCmdUpdate"))
	fmt.Printf("  help [comando]                 %s\n", i18n.Tr("helpPrint"))

	fmt.Println()
	fmt.Println(i18n.Tr("cliHelpFooter"))
}

func printTopicHelp(cmd string) {
	switch strings.ToLower(cmd) {
	case "run":
		fmt.Println("Uso: joss run <archivo.joss>")
		fmt.Println(i18n.Tr("helpTopicRun"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss run main.joss")
		fmt.Println("  joss run script.joss")

	case "repl":
		fmt.Println("Uso: joss repl")
		fmt.Println(i18n.Tr("helpTopicRepl"))
		fmt.Println("\nComandos interactivos:")
		fmt.Printf("  exit, quit     %s\n", i18n.Tr("helpTopicReplExitHint"))
		fmt.Printf("  Ctrl+C         %s\n", i18n.Tr("helpTopicReplCtrlCHint"))

	case "server":
		fmt.Println("Uso: joss server start")
		fmt.Println(i18n.Tr("helpTopicServer"))

	case "program":
		fmt.Println("Uso: joss program start")
		fmt.Println(i18n.Tr("helpTopicProgram"))

	case "build":
		fmt.Println("Uso: joss build [web|program|native|package] [opciones]")
		fmt.Println(i18n.Tr("helpTopicBuild"))
		fmt.Println("\nModos:")
		fmt.Printf("  web                            %s\n", i18n.Tr("helpTopicBuildModeWeb"))
		fmt.Printf("  program                        %s\n", i18n.Tr("helpTopicBuildModeProgram"))
		fmt.Printf("  native [os] [arch] [--gui]     %s\n", i18n.Tr("helpTopicBuildModeNative"))
		fmt.Printf("  package <ruta>                 %s\n", i18n.Tr("helpTopicBuildModePackage"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss build web")
		fmt.Println("  joss build native windows amd64 --gui")
		fmt.Println("  joss build native linux arm64")

	case "analyze":
		fmt.Println("Uso: joss analyze [archivo.joss]")
		fmt.Println(i18n.Tr("helpTopicAnalyze"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss analyze")
		fmt.Println("  joss analyze main.joss")

	case "format":
		fmt.Println("Uso: joss format [ruta] [opciones]")
		fmt.Println(i18n.Tr("helpTopicFormat"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --write, -w    %s\n", i18n.Tr("helpTopicFormatOptWrite"))
		fmt.Printf("  --check, -c    %s\n", i18n.Tr("helpTopicFormatOptCheck"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss format app/controllers/UserController.joss")
		fmt.Println("  joss format --write .")
		fmt.Println("  joss format --check .")

	case "lint":
		fmt.Println("Uso: joss lint [ruta] [opciones]")
		fmt.Println(i18n.Tr("helpTopicLint"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --json         %s\n", i18n.Tr("helpTopicLintOptJson"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss lint .")
		fmt.Println("  joss lint main.joss")
		fmt.Println("  joss lint --json .")

	case "fix":
		fmt.Println("Uso: joss fix [ruta] [opciones]")
		fmt.Println(i18n.Tr("helpTopicFix"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --dry-run, -d  %s\n", i18n.Tr("helpTopicFixOptDryRun"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss fix .")
		fmt.Println("  joss fix --dry-run .")

	case "check":
		fmt.Println("Uso: joss check [ruta]")
		fmt.Println(i18n.Tr("helpTopicCheck"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss check .")

	case "test":
		fmt.Println("Uso: joss test [ruta] [opciones]")
		fmt.Println(i18n.Tr("helpTopicTest"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --filter, -f   %s\n", i18n.Tr("helpTopicTestOptFilter"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss test")
		fmt.Println("  joss test tests/")
		fmt.Println("  joss test --filter=login")

	case "new":
		fmt.Println("Uso: joss new [web|console|package|plugin] <ruta/nombre>")
		fmt.Println(i18n.Tr("helpTopicNew"))
		fmt.Println("\nTipos:")
		fmt.Printf("  web       %s\n", i18n.Tr("helpTopicNewTypeWeb"))
		fmt.Printf("  console   %s\n", i18n.Tr("helpTopicNewTypeConsole"))
		fmt.Printf("  package   %s\n", i18n.Tr("helpTopicNewTypePackage"))
		fmt.Printf("  plugin    %s\n", i18n.Tr("helpTopicNewTypePlugin"))

	case "make:controller", "controller":
		fmt.Println("Uso: joss make:controller <Nombre>")
		fmt.Println(i18n.Tr("helpTopicMakeController"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:controller UserController")

	case "make:model", "model":
		fmt.Println("Uso: joss make:model <Nombre>")
		fmt.Println(i18n.Tr("helpTopicMakeModel"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:model User")

	case "make:view", "view":
		fmt.Println("Uso: joss make:view <Ruta/Nombre>")
		fmt.Println(i18n.Tr("helpTopicMakeView"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:view users/profile")

	case "make:middleware", "middleware":
		fmt.Println("Uso: joss make:middleware <Nombre>")
		fmt.Println(i18n.Tr("helpTopicMakeMiddleware"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:middleware AuthGuard")

	case "make:mvc", "mvc":
		fmt.Println("Uso: joss make:mvc <Nombre>")
		fmt.Println(i18n.Tr("helpTopicMakeMvc"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:mvc Product")

	case "make:crud", "crud":
		fmt.Println("Uso: joss make:crud <Tabla>")
		fmt.Println(i18n.Tr("helpTopicMakeCrud"))

	case "remove:crud":
		fmt.Println("Uso: joss remove:crud <Tabla>")
		fmt.Println(i18n.Tr("helpTopicRemoveCrud"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss remove:crud products")

	case "make:migration", "migration":
		fmt.Println("Uso: joss make:migration <Nombre>")
		fmt.Println(i18n.Tr("helpTopicMakeMigration"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss make:migration create_users_table")
		fmt.Println("  joss make:migration add_avatar_to_users")

	case "migrate":
		fmt.Println("Uso: joss migrate")
		fmt.Println(i18n.Tr("helpTopicMigrate"))

	case "migrate:fresh":
		fmt.Println("Uso: joss migrate:fresh")
		fmt.Println(i18n.Tr("helpTopicMigrateFresh"))

	case "db:seed", "seed":
		fmt.Println("Uso: joss db:seed")
		fmt.Println(i18n.Tr("helpTopicDbSeed"))

	case "change", "db":
		fmt.Println("Uso: joss change db [motor|migrate|prefix]")
		fmt.Println(i18n.Tr("helpTopicChangeDb"))
		fmt.Println("\nSubcomandos:")
		fmt.Printf("  change db <motor>           %s\n", i18n.Tr("helpTopicChangeDbMotor"))
		fmt.Printf("  change db prefix <prefijo>  %s\n", i18n.Tr("helpTopicChangeDbPrefix"))
		fmt.Printf("  change db migrate           %s\n", i18n.Tr("helpTopicChangeDbMigrate"))

	case "userstorage", "storage":
		fmt.Println("Uso: joss userstorage [local|oci|sync-oci|sync-local]")
		fmt.Println(i18n.Tr("helpTopicUserStorage"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  local        %s\n", i18n.Tr("helpTopicUserStorageLocal"))
		fmt.Printf("  oci          %s\n", i18n.Tr("helpTopicUserStorageOci"))
		fmt.Printf("  sync-oci     %s\n", i18n.Tr("helpTopicUserStorageSyncOci"))
		fmt.Printf("  sync-local   %s\n", i18n.Tr("helpTopicUserStorageSyncLocal"))

	case "pub":
		fmt.Println("Uso: joss pub <subcomando> [paquete]")
		fmt.Println(i18n.Tr("helpTopicPub"))
		fmt.Println("\nSubcomandos:")
		fmt.Printf("  add <paquete>      %s\n", i18n.Tr("helpTopicPubAdd"))
		fmt.Printf("  remove <paquete>   %s\n", i18n.Tr("helpTopicPubRemove"))
		fmt.Printf("  install            %s\n", i18n.Tr("helpTopicPubInstall"))
		fmt.Printf("  update             %s\n", i18n.Tr("helpTopicPubUpdate"))
		fmt.Printf("  publish            %s\n", i18n.Tr("helpTopicPubPublish"))

	case "plugin":
		printPluginUsage()
		fmt.Printf("\n%s\n", i18n.Tr("helpTopicPlugin"))

	case "package":
		fmt.Println("Uso: joss package inspect <archivo.jp>")
		fmt.Println(i18n.Tr("helpTopicPackage"))

	case "update":
		fmt.Println("Uso: joss update [opciones]")
		fmt.Println(i18n.Tr("helpTopicUpdate"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  -f, --force    %s\n", i18n.Tr("helpTopicUpdateOptForce"))
		fmt.Printf("  --canary       %s\n", i18n.Tr("helpTopicUpdateOptCanary"))
		fmt.Printf("  --stable       %s\n", i18n.Tr("helpTopicUpdateOptStable"))

	case "version":
		fmt.Println("Uso: joss version")
		fmt.Println(i18n.Tr("helpTopicVersion"))

	case "ai:activate", "ai":
		fmt.Println("Uso: joss ai:activate")
		fmt.Println(i18n.Tr("helpTopicAiActivate"))

	default:
		fmt.Printf("%s\n\n", i18n.Tr("helpTopicNotFound", map[string]interface{}{"cmd": cmd}))
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

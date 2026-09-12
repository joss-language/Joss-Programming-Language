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
		fmt.Printf("%s joss run <archivo.joss>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicRun"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss run main.joss")
		fmt.Println("  joss run script.joss")

	case "repl":
		fmt.Printf("%s joss repl\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicRepl"))
		fmt.Println("\nComandos interactivos:")
		fmt.Printf("  exit, quit     %s\n", i18n.Tr("helpTopicReplExitHint"))
		fmt.Printf("  Ctrl+C         %s\n", i18n.Tr("helpTopicReplCtrlCHint"))

	case "server":
		fmt.Printf("%s joss server start\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicServer"))

	case "program":
		fmt.Printf("%s joss program start\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicProgram"))

	case "build":
		fmt.Printf("%s joss build [web|program|native|package] [opciones]\n", i18n.Tr("cliUsageLabel"))
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
		fmt.Printf("%s joss analyze [archivo.joss]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicAnalyze"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss analyze")
		fmt.Println("  joss analyze main.joss")

	case "format":
		fmt.Printf("%s joss format [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicFormat"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --write, -w    %s\n", i18n.Tr("helpTopicFormatOptWrite"))
		fmt.Printf("  --check, -c    %s\n", i18n.Tr("helpTopicFormatOptCheck"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss format app/controllers/UserController.joss")
		fmt.Println("  joss format --write .")
		fmt.Println("  joss format --check .")

	case "lint":
		fmt.Printf("%s joss lint [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicLint"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --json         %s\n", i18n.Tr("helpTopicLintOptJson"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss lint .")
		fmt.Println("  joss lint main.joss")
		fmt.Println("  joss lint --json .")

	case "fix":
		fmt.Printf("%s joss fix [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicFix"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --dry-run, -d  %s\n", i18n.Tr("helpTopicFixOptDryRun"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss fix .")
		fmt.Println("  joss fix --dry-run .")

	case "check":
		fmt.Printf("%s joss check [ruta]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicCheck"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss check .")

	case "test":
		fmt.Printf("%s joss test [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicTest"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  --filter, -f   %s\n", i18n.Tr("helpTopicTestOptFilter"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss test")
		fmt.Println("  joss test tests/")
		fmt.Println("  joss test --filter=login")

	case "new":
		fmt.Printf("%s joss new [web|console|package|plugin] <ruta/nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicNew"))
		fmt.Println("\nTipos:")
		fmt.Printf("  web       %s\n", i18n.Tr("helpTopicNewTypeWeb"))
		fmt.Printf("  console   %s\n", i18n.Tr("helpTopicNewTypeConsole"))
		fmt.Printf("  package   %s\n", i18n.Tr("helpTopicNewTypePackage"))
		fmt.Printf("  plugin    %s\n", i18n.Tr("helpTopicNewTypePlugin"))

	case "make:controller", "controller":
		fmt.Printf("%s joss make:controller <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeController"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:controller UserController")

	case "make:model", "model":
		fmt.Printf("%s joss make:model <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeModel"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:model User")

	case "make:view", "view":
		fmt.Printf("%s joss make:view <Ruta/Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeView"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:view users/profile")

	case "make:middleware", "middleware":
		fmt.Printf("%s joss make:middleware <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeMiddleware"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:middleware AuthGuard")

	case "make:mvc", "mvc":
		fmt.Printf("%s joss make:mvc <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeMvc"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss make:mvc Product")

	case "make:crud", "crud":
		fmt.Printf("%s joss make:crud <Tabla>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeCrud"))

	case "remove:crud":
		fmt.Printf("%s joss remove:crud <Tabla>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicRemoveCrud"))
		fmt.Println("\nEjemplo:")
		fmt.Println("  joss remove:crud products")

	case "make:migration", "migration":
		fmt.Printf("%s joss make:migration <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeMigration"))
		fmt.Println("\nEjemplos:")
		fmt.Println("  joss make:migration create_users_table")
		fmt.Println("  joss make:migration add_avatar_to_users")

	case "migrate":
		fmt.Printf("%s joss migrate\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMigrate"))

	case "migrate:fresh":
		fmt.Printf("%s joss migrate:fresh\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMigrateFresh"))

	case "db:seed", "seed":
		fmt.Printf("%s joss db:seed\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicDbSeed"))

	case "change", "db":
		fmt.Printf("%s joss change db [motor|migrate|prefix]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicChangeDb"))
		fmt.Println("\nSubcomandos:")
		fmt.Printf("  change db <motor>           %s\n", i18n.Tr("helpTopicChangeDbMotor"))
		fmt.Printf("  change db prefix <prefijo>  %s\n", i18n.Tr("helpTopicChangeDbPrefix"))
		fmt.Printf("  change db migrate           %s\n", i18n.Tr("helpTopicChangeDbMigrate"))

	case "userstorage", "storage":
		fmt.Printf("%s joss userstorage [local|oci|sync-oci|sync-local]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicUserStorage"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  local        %s\n", i18n.Tr("helpTopicUserStorageLocal"))
		fmt.Printf("  oci          %s\n", i18n.Tr("helpTopicUserStorageOci"))
		fmt.Printf("  sync-oci     %s\n", i18n.Tr("helpTopicUserStorageSyncOci"))
		fmt.Printf("  sync-local   %s\n", i18n.Tr("helpTopicUserStorageSyncLocal"))

	case "pub":
		fmt.Printf("%s joss pub <subcomando> [paquete]\n", i18n.Tr("cliUsageLabel"))
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
		fmt.Printf("%s joss package inspect <archivo.jp>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicPackage"))

	case "update":
		fmt.Printf("%s joss update [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicUpdate"))
		fmt.Println("\nOpciones:")
		fmt.Printf("  -f, --force    %s\n", i18n.Tr("helpTopicUpdateOptForce"))
		fmt.Printf("  --canary       %s\n", i18n.Tr("helpTopicUpdateOptCanary"))
		fmt.Printf("  --stable       %s\n", i18n.Tr("helpTopicUpdateOptStable"))

	case "version":
		fmt.Printf("%s joss version\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicVersion"))

	case "ai:activate", "ai":
		fmt.Printf("%s joss ai:activate\n", i18n.Tr("cliUsageLabel"))
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

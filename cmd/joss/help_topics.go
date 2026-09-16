package main

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/i18n"
)

func printTopicHelp(cmd string) {
	lower := strings.ToLower(cmd)
	if printTopicHelpScaffolding(lower) {
		return
	}
	if printTopicHelpDatabase(lower) {
		return
	}
	if printTopicHelpTools(lower) {
		return
	}
	if printTopicHelpCore(lower) {
		return
	}

	fmt.Printf("%s\n\n", i18n.Tr("helpTopicNotFound", map[string]interface{}{"cmd": cmd}))
	printHelp()
}

func printTopicHelpCore(cmd string) bool {
	switch cmd {
	case "run":
		fmt.Printf("%s joss run <archivo.joss>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicRun"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss run main.joss")
		fmt.Println("  joss run script.joss")
		return true

	case "repl":
		fmt.Printf("%s joss repl\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicRepl"))
		fmt.Println("\nComandos interactivos:")
		fmt.Printf("  exit, quit     %s\n", i18n.Tr("helpTopicReplExitHint"))
		fmt.Printf("  Ctrl+C         %s\n", i18n.Tr("helpTopicReplCtrlCHint"))
		return true

	case "server":
		fmt.Printf("%s joss server start\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicServer"))
		return true

	case "program":
		fmt.Printf("%s joss program start\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicProgram"))
		return true

	case "build":
		fmt.Printf("%s joss build [web|program|native|package] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicBuild"))
		fmt.Println("\nModos:")
		fmt.Printf("  web                            %s\n", i18n.Tr("helpTopicBuildModeWeb"))
		fmt.Printf("  program                        %s\n", i18n.Tr("helpTopicBuildModeProgram"))
		fmt.Printf("  native [os] [arch] [--gui]     %s\n", i18n.Tr("helpTopicBuildModeNative"))
		fmt.Printf("  package <ruta>                 %s\n", i18n.Tr("helpTopicBuildModePackage"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss build web")
		fmt.Println("  joss build native windows amd64 --gui")
		fmt.Println("  joss build native linux arm64")
		return true

	case "version":
		fmt.Printf("%s joss version\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicVersion"))
		return true

	case "indexnow":
		fmt.Printf("%s joss indexnow generate\n", i18n.Tr("cliUsageLabel"))
		fmt.Println("Genera una clave criptográfica de verificación y activa IndexNow en el entorno (.env).")
		fmt.Println("\nEl servidor web nativo de Joss responde internamente en:")
		fmt.Println("  GET /{key}.txt")
		fmt.Println("(Sin necesidad de crear archivos físicos en disco ni rutas en routes.joss)")
		return true

	case "ai:activate", "ai":
		fmt.Printf("%s joss ai:activate\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicAiActivate"))
		return true
	}
	return false
}

func printTopicHelpTools(cmd string) bool {
	switch cmd {
	case "analyze":
		fmt.Printf("%s joss analyze [archivo.joss]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicAnalyze"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss analyze")
		fmt.Println("  joss analyze main.joss")
		return true

	case "format":
		fmt.Printf("%s joss format [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicFormat"))
		fmt.Println(helpHeaderOptions)
		fmt.Printf("  --write, -w    %s\n", i18n.Tr("helpTopicFormatOptWrite"))
		fmt.Printf("  --check, -c    %s\n", i18n.Tr("helpTopicFormatOptCheck"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss format app/controllers/UserController.joss")
		fmt.Println("  joss format --write .")
		fmt.Println("  joss format --check .")
		return true

	case "lint":
		fmt.Printf("%s joss lint [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicLint"))
		fmt.Println(helpHeaderOptions)
		fmt.Printf("  --json         %s\n", i18n.Tr("helpTopicLintOptJson"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss lint .")
		fmt.Println("  joss lint main.joss")
		fmt.Println("  joss lint --json .")
		return true

	case "fix":
		fmt.Printf("%s joss fix [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicFix"))
		fmt.Println(helpHeaderOptions)
		fmt.Printf("  --dry-run, -d  %s\n", i18n.Tr("helpTopicFixOptDryRun"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss fix .")
		fmt.Println("  joss fix --dry-run .")
		return true

	case "check":
		fmt.Printf("%s joss check [ruta]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicCheck"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss check .")
		return true

	case "test":
		fmt.Printf("%s joss test [ruta] [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicTest"))
		fmt.Println(helpHeaderOptions)
		fmt.Printf("  --filter, -f   %s\n", i18n.Tr("helpTopicTestOptFilter"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss test")
		fmt.Println("  joss test tests/")
		fmt.Println("  joss test --filter=login")
		return true

	case "pub":
		fmt.Printf("%s joss pub <subcomando> [paquete]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicPub"))
		fmt.Println("\nSubcomandos:")
		fmt.Printf("  add <paquete>      %s\n", i18n.Tr("helpTopicPubAdd"))
		fmt.Printf("  remove <paquete>   %s\n", i18n.Tr("helpTopicPubRemove"))
		fmt.Printf("  install            %s\n", i18n.Tr("helpTopicPubInstall"))
		fmt.Printf("  update             %s\n", i18n.Tr("helpTopicPubUpdate"))
		fmt.Printf("  publish            %s\n", i18n.Tr("helpTopicPubPublish"))
		return true

	case "plugin":
		printPluginUsage()
		fmt.Printf("\n%s\n", i18n.Tr("helpTopicPlugin"))
		return true

	case "package":
		fmt.Printf("%s joss package inspect <archivo.jp>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicPackage"))
		return true

	case "update":
		fmt.Printf("%s joss update [opciones]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicUpdate"))
		fmt.Println(helpHeaderOptions)
		fmt.Printf("  -f, --force    %s\n", i18n.Tr("helpTopicUpdateOptForce"))
		fmt.Printf("  --canary       %s\n", i18n.Tr("helpTopicUpdateOptCanary"))
		fmt.Printf("  --stable       %s\n", i18n.Tr("helpTopicUpdateOptStable"))
		return true
	}
	return false
}

func printTopicHelpScaffolding(cmd string) bool {
	switch cmd {
	case "new":
		fmt.Printf("%s joss new [web|console|package|plugin] <ruta/nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicNew"))
		fmt.Println("\nTipos:")
		fmt.Printf("  web       %s\n", i18n.Tr("helpTopicNewTypeWeb"))
		fmt.Printf("  console   %s\n", i18n.Tr("helpTopicNewTypeConsole"))
		fmt.Printf("  package   %s\n", i18n.Tr("helpTopicNewTypePackage"))
		fmt.Printf("  plugin    %s\n", i18n.Tr("helpTopicNewTypePlugin"))
		return true

	case "make:controller", "controller":
		fmt.Printf("%s joss make:controller <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeController"))
		fmt.Println(helpHeaderExample)
		fmt.Println("  joss make:controller UserController")
		return true

	case "make:model", "model":
		fmt.Printf("%s joss make:model <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeModel"))
		fmt.Println(helpHeaderExample)
		fmt.Println("  joss make:model User")
		return true

	case "make:view", "view":
		fmt.Printf("%s joss make:view <Ruta/Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeView"))
		fmt.Println(helpHeaderExample)
		fmt.Println("  joss make:view users/profile")
		return true

	case "make:middleware", "middleware":
		fmt.Printf("%s joss make:middleware <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeMiddleware"))
		fmt.Println(helpHeaderExample)
		fmt.Println("  joss make:middleware AuthGuard")
		return true

	case "make:mvc", "mvc":
		fmt.Printf("%s joss make:mvc <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeMvc"))
		fmt.Println(helpHeaderExample)
		fmt.Println("  joss make:mvc Product")
		return true

	case "make:crud", "crud":
		fmt.Printf("%s joss make:crud <Tabla>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeCrud"))
		return true

	case "remove:crud":
		fmt.Printf("%s joss remove:crud <Tabla>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicRemoveCrud"))
		fmt.Println(helpHeaderExample)
		fmt.Println("  joss remove:crud products")
		return true

	case "make:migration", "migration":
		fmt.Printf("%s joss make:migration <Nombre>\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMakeMigration"))
		fmt.Println(helpHeaderExamples)
		fmt.Println("  joss make:migration create_users_table")
		fmt.Println("  joss make:migration add_avatar_to_users")
		return true
	}
	return false
}

func printTopicHelpDatabase(cmd string) bool {
	switch cmd {
	case "migrate":
		fmt.Printf("%s joss migrate\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMigrate"))
		return true

	case "migrate:fresh":
		fmt.Printf("%s joss migrate:fresh\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicMigrateFresh"))
		return true

	case "db:seed", "seed":
		fmt.Printf("%s joss db:seed\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicDbSeed"))
		return true

	case "change", "db":
		fmt.Printf("%s joss change db [motor|migrate|prefix]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicChangeDb"))
		fmt.Println("\nSubcomandos:")
		fmt.Printf("  change db <motor>           %s\n", i18n.Tr("helpTopicChangeDbMotor"))
		fmt.Printf("  change db prefix <prefijo>  %s\n", i18n.Tr("helpTopicChangeDbPrefix"))
		fmt.Printf("  change db migrate           %s\n", i18n.Tr("helpTopicChangeDbMigrate"))
		return true

	case "userstorage", "storage":
		fmt.Printf("%s joss userstorage [local|oci|sync-oci|sync-local]\n", i18n.Tr("cliUsageLabel"))
		fmt.Println(i18n.Tr("helpTopicUserStorage"))
		fmt.Println(helpHeaderOptions)
		fmt.Printf("  local        %s\n", i18n.Tr("helpTopicUserStorageLocal"))
		fmt.Printf("  oci          %s\n", i18n.Tr("helpTopicUserStorageOci"))
		fmt.Printf("  sync-oci     %s\n", i18n.Tr("helpTopicUserStorageSyncOci"))
		fmt.Printf("  sync-local   %s\n", i18n.Tr("helpTopicUserStorageSyncLocal"))
		return true
	}
	return false
}

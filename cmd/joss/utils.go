package main

import (
	"fmt"
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
	fmt.Println("  indexnow generate              Genera y configura una clave IndexNow para indexación instantánea")
	fmt.Printf("  update [-f|--canary|--stable]  %s\n", i18n.Tr("cliCmdUpdate"))
	fmt.Printf("  help [comando]                 %s\n", i18n.Tr("helpPrint"))

	fmt.Println()
	fmt.Println(i18n.Tr("cliHelpFooter"))
}

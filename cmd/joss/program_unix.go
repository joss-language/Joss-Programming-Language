//go:build !windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"

	"github.com/jossecurity/joss/pkg/i18n"
)

//go:embed default_logo.png
var defaultLogo []byte

func startProgram(args []string) {
	_, rest := takeExecutionMode(args)

	if _, err := os.Stat("main.joss"); os.IsNotExist(err) {
		fmt.Println(i18n.Tr("cliErrNoMain"))
		fmt.Println("  main.joss")
		fmt.Println(i18n.Tr("cliErrRequireMain"))
		os.Exit(1)
	}

	fmt.Println("⚡ [Desktop GUI] Compilando aplicación a binario Joss Native AOT...")
	exePath, err := buildNativeProgram("main.joss", "", "", false, false, false, true, "auto")
	if err != nil {
		fmt.Printf("❌ [Desktop GUI] Error compilando binario nativo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🚀 [Desktop GUI] Ejecutando aplicación nativa standalone (%s)...\n", exePath)
	cmd := exec.Command(exePath, rest...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "JOSS_GUI=true", "JOSS_PROGRAM_START=true")
	if runErr := cmd.Run(); runErr != nil {
		fmt.Printf("❌ [Desktop GUI] Error ejecutando aplicación nativa: %v\n", runErr)
		os.Exit(1)
	}
}

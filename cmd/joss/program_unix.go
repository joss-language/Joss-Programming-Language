//go:build !windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed default_logo.png
var defaultLogo []byte

func containsWindowAPI(src string) bool {
	return strings.Contains(src, "Window::open") || strings.Contains(src, "new Window")
}

func startProgram(args []string) {
	mode, rest := takeExecutionMode(args)
	if mode == executionModeNative {
		if rawSrc, err := os.ReadFile("main.joss"); err == nil && containsWindowAPI(string(rawSrc)) {
			fmt.Println("⚡ [Desktop GUI] Compilando aplicación a binario nativo AOT...")
			exePath, buildErr := buildNativeProgram("main.joss", "", "", false, false, false, true, "auto")
			if buildErr != nil {
				fmt.Printf("❌ [Desktop GUI] Error compilando binario nativo: %v\n", buildErr)
				os.Exit(1)
			}
			cmd := exec.Command(exePath, rest...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Env = append(os.Environ(), "JOSS_GUI=true")
			if runErr := cmd.Run(); runErr != nil {
				fmt.Printf("❌ [Desktop GUI] Error ejecutando aplicación nativa: %v\n", runErr)
				os.Exit(1)
			}
			return
		}
	}
	handleServerStart(append(args, "--gui"))
}

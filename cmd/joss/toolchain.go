package main

import (
	"fmt"
	"os"

	llvmbackend "github.com/jossecurity/joss/pkg/backend/llvm"
	"github.com/jossecurity/joss/pkg/backend/native"
)

func handleToolchainCommand(args []string) int {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		tc := llvmbackend.ResolveToolchain()
		if tc.Available() {
			fmt.Printf("✓ Toolchain nativo activo: %s (%s)\n", tc.Kind, tc.CompilerPath)
			if tc.RootDir != "" {
				fmt.Printf("  Raíz: %s\n", tc.RootDir)
			}
			if rtPath, err := llvmbackend.EnsureRuntimeC(""); err == nil {
				fmt.Printf("✓ Runtime C11 embebido listo: %s\n", rtPath)
			}
			return 0
		}
		fmt.Println("ℹ️  No hay toolchain LLVM/C local instalado todavía.")
		fmt.Println("   Ejecuta 'joss toolchain install' o 'joss build <archivo> --native' para instalarlo automáticamente en ~/.joss/toolchain/.")
		return 0

	case "install":
		tc, err := llvmbackend.InstallManagedToolchain()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error instalando toolchain nativo: %v\n", err)
			return 1
		}
		llvmbackend.RegisterAvailableToolchain()
		fmt.Printf("✓ Toolchain nativo instalado: %s (%s)\n", tc.Kind, tc.CompilerPath)
		return 0

	default:
		fmt.Println("Uso: joss toolchain [status | install]")
		return 1
	}
}

func ensureNativeToolchainReady() bool {
	if native.NewNativeCompiler().HasNativeToolchain() {
		return true
	}
	if _, err := llvmbackend.EnsureToolchain(); err == nil {
		llvmbackend.RegisterAvailableToolchain()
		return native.NewNativeCompiler().HasNativeToolchain()
	} else {
		fmt.Fprintf(os.Stderr, "⚠️  [Toolchain] No se pudo preparar el toolchain nativo: %v\n", err)
	}
	return false
}

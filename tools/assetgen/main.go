package main

import (
	"fmt"
	"os"

	"github.com/jossecurity/joss/pkg/server"
)

func main() {
	if len(os.Args) > 1 {
		err := os.Chdir(os.Args[1])
		if err != nil {
			fmt.Printf("Error cambiando de directorio: %v\n", err)
			os.Exit(1)
		}
	}
	server.CompileStyles()
	fmt.Println("Compilación completada exitosamente.")
}

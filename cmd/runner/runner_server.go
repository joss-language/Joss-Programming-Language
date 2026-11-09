//go:build !cli

package main

import (
	"fmt"
	"net/http"
)

func startServerHandler(fs http.FileSystem) {
	fmt.Println("[Runner] El servidor web ahora se compila y ejecuta exclusivamente en modo nativo.")
}

func initServerFS(fs http.FileSystem) {
}

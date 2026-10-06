//go:build cli

package main

import (
	"log"
	"net/http"
)

func startServerHandler(fs http.FileSystem) {
	log.Println("[Joss Runner] Servidor HTTP no disponible en el perfil CLI mínimo.")
}

func initServerFS(fs http.FileSystem) {
	// No-op en perfil CLI
}

//go:build windows

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jchv/go-webview2"
)

func runGUIOrWait(finalPort string) {
	if os.Getenv("JOSS_GUI") != "true" {
		fmt.Println("[Joss Runner] Modo Servidor Headless (Consola).")
		fmt.Printf("[Joss Runner] Servidor activo escuchando en http://localhost:%s\n", finalPort)
		waitForSignal()
		return
	}

	w := webview2.New(true)
	if w == nil {
		log.Println("[Joss Runner] WebView2 no disponible. Ejecutando servidor sin GUI.")
		log.Printf("[Joss Runner] Servidor activo escuchando en http://localhost:%s\n", finalPort)
		waitForSignal()
		return
	}
	defer w.Destroy()

	w.SetTitle("Joss Application")
	w.SetSize(1024, 768, webview2.HintNone)
	w.Navigate("http://localhost:" + finalPort)
	w.Run()
	os.Exit(0)
}

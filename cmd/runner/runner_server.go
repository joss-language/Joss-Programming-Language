//go:build !cli

package main

import (
	"net/http"

	"github.com/jossecurity/joss/pkg/server"
)

func startServerHandler(fs http.FileSystem) {
	server.GlobalFileSystem = fs
	server.Start(fs)
}

func initServerFS(fs http.FileSystem) {
	server.GlobalFileSystem = fs
}

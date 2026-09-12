//go:build windows

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/jchv/go-webview2"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/server"
)

func startProgram() {
	go launchProgramRuntime()

	port := os.Getenv("PORT")
	if port == "" {
		port = getEnvPort(GetEnvFile())
	}
	if port == "" {
		port = "8000"
	}
	host := "localhost:" + port

	waitForServer(host)
	launchGUIWindow(host)
}

func launchProgramRuntime() {
	r := core.NewRuntime()
	r.LoadEnv(server.GlobalFileSystem)

	data, err := readMainJossData()
	if err != nil {
		fmt.Println(i18n.Tr("programNoMainStarting"))
		server.Start(nil)
		return
	}

	fmt.Println(i18n.Tr("programRunningMain"))
	l := parser.NewLexer(string(data))
	p := parser.NewParser(l)
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		r.Execute(program)
	} else {
		fmt.Println(i18n.Tr("programErrorsInMain", i18n.M{"errors": fmt.Sprintf("%v", p.Errors())}))
	}
}

func readMainJossData() ([]byte, error) {
	if server.GlobalFileSystem != nil {
		content, errOpen := server.GlobalFileSystem.Open("main.joss")
		if errOpen == nil {
			defer content.Close()
			stat, _ := content.Stat()
			data := make([]byte, stat.Size())
			_, errRead := content.Read(data)
			return data, errRead
		}
		return nil, errOpen
	}
	return os.ReadFile("main.joss")
}

func launchGUIWindow(host string) {
	w := webview2.New(true)
	if w == nil {
		log.Println(i18n.Tr("programWebViewError"))
		return
	}
	defer w.Destroy()

	w.SetTitle("Joss App")
	w.SetSize(1024, 768, webview2.HintNone)
	w.Navigate("http://" + host)
	w.Run()
}

func waitForServer(address string) {
	for i := 0; i < 30; i++ {
		conn, err := net.DialTimeout("tcp", address, 1*time.Second)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

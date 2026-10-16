//go:build windows

package server

import (
	"net"
	"os"
	"time"

	"github.com/jchv/go-webview2"
)

func maybeLaunchDesktopWindow(port string, isGUI bool) bool {
	if !isGUI && os.Getenv("JOSS_GUI") != "true" {
		return false
	}

	host := "localhost:" + port
	for i := 0; i < 40; i++ {
		conn, err := net.DialTimeout("tcp", host, 250*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	w := webview2.New(true)
	if w == nil {
		return false
	}
	defer w.Destroy()

	title := "Joss Application"
	if currentRuntime != nil && currentRuntime.Env != nil {
		if appName := currentRuntime.Env["APP_NAME"]; appName != "" {
			title = appName
		}
	}

	w.SetTitle(title)
	w.SetSize(1120, 760, webview2.HintNone)
	w.Navigate("http://" + host)
	w.Run()
	os.Exit(0)
	return true
}

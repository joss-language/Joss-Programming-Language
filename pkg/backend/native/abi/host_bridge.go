package main

import "C"

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/server"
)

//export _joss_host_bridge
func _joss_host_bridge(methodNamePtr *C.char) {
	if methodNamePtr == nil {
		return
	}
	methodName := C.GoString(methodNamePtr)
	fmt.Printf("[HostBridge ABI] Llamada nativa recibida para: %s\n", methodName)

	if methodName == "Server::start" || strings.HasSuffix(methodName, "Server_start") {
		// Launch the HTTP server via the canonical server service
		server.Start(nil)
	} else if methodName == "Plugin::call" || strings.HasSuffix(methodName, "Plugin_call") {
		// Non-fatal plugin preload hook
		fmt.Printf("[HostBridge ABI] Plugin call: %s\n", methodName)
	}
}

func main() {}

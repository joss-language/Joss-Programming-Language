package main

import "C"

import (
	"fmt"
)

//export _joss_host_bridge
func _joss_host_bridge(methodNamePtr *C.char) {
	methodName := C.GoString(methodNamePtr)
	fmt.Printf("[HostBridge ABI] Llamada nativa recibida para: %s\n", methodName)
}

func main() {}

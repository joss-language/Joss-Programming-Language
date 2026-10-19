package jit

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/jossecurity/joss/pkg/server"
)

// HostBridgeFn is the signature for functions reachable via the in-process host bridge.
type HostBridgeFn func(args ...uintptr) uintptr

var (
	hostBridgeMu         sync.RWMutex
	customBridgeHandlers = make(map[string]HostBridgeFn)
)

// RegisterHostBridgeHandler registers a handler for a given host service or method name.
func RegisterHostBridgeHandler(name string, handler HostBridgeFn) {
	hostBridgeMu.Lock()
	defer hostBridgeMu.Unlock()
	customBridgeHandlers[name] = handler
}

// InProcessHostBridge is the Go dispatcher invoked by JIT-compiled machine code.
// C-ABI convention on Windows: RCX points to null-terminated ASCII/UTF-8 method string.
func InProcessHostBridge(ptr *byte) uintptr {
	if ptr == nil {
		return 0
	}

	pointer := unsafe.Pointer(ptr)
	data := make([]byte, 0, 64)
	for index := 0; index < 512; index++ {
		value := *(*byte)(unsafe.Add(pointer, index))
		if value == 0 {
			break
		}
		data = append(data, value)
	}
	methodName := string(data)

	hostBridgeMu.RLock()
	handler, ok := customBridgeHandlers[methodName]
	hostBridgeMu.RUnlock()

	if ok && handler != nil {
		return handler(uintptr(unsafe.Pointer(ptr)))
	}

	// Built-in standard handlers
	if methodName == "Server::start" || strings.HasSuffix(methodName, "Server_start") {
		server.Start(nil)
		return 0
	}

	if methodName == "Plugin::call" || strings.HasSuffix(methodName, "Plugin_call") {
		return 0
	}

	fmt.Printf("[JIT HostBridge] Llamada a servicio: %s\n", methodName)
	return 0
}

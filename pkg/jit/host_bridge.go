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
// C-ABI convention on Windows:
// RCX = methodName (null-terminated string)
// RDX = arg1
// R8  = arg2
// R9  = arg3
func toUnsafe(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

func InProcessHostBridge(namePtr, arg1, arg2, arg3 uintptr) uintptr {
	if namePtr == 0 {
		return 0
	}

	pointer := toUnsafe(namePtr)
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
		return handler(arg1, arg2, arg3)
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

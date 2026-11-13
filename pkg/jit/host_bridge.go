package jit

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

// HostBridgeFn is the signature for functions reachable via the in-process host bridge.
type HostBridgeFn func(args ...uintptr) uintptr

var (
	hostBridgeMu         sync.RWMutex
	customBridgeHandlers = make(map[string]HostBridgeFn)
	lastFileSizeAddr     uintptr
)

// SetLastFileSizeAddress configures the RAM address to dataOffLastFileSize for the active JIT image.
func SetLastFileSizeAddress(addr uintptr) {
	hostBridgeMu.Lock()
	defer hostBridgeMu.Unlock()
	lastFileSizeAddr = addr
}

// RecordLastFileSize stores the size of the most recent binary payload.
func RecordLastFileSize(size int64) {
	hostBridgeMu.RLock()
	defer hostBridgeMu.RUnlock()
	if lastFileSizeAddr != 0 {
		p := toUnsafe(lastFileSizeAddr)
		*(*int64)(p) = size
	}
}

// RegisterHostBridgeHandler registers a handler for a given host service or method name.
func RegisterHostBridgeHandler(name string, handler HostBridgeFn) {
	hostBridgeMu.Lock()
	defer hostBridgeMu.Unlock()
	customBridgeHandlers[name] = handler
}

// HasHostBridgeHandler reports whether a JIT callback is registered for a service.
func HasHostBridgeHandler(name string) bool {
	hostBridgeMu.RLock()
	defer hostBridgeMu.RUnlock()
	handler, ok := customBridgeHandlers[name]
	return ok && handler != nil
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
		fmt.Println("[JIT HostBridge] El servidor HTTP es administrado de forma nativa por Joss Native AOT.")
		return 0
	}

	if methodName == "Plugin::call" || strings.HasSuffix(methodName, "Plugin_call") {
		return 0
	}

	fmt.Printf("[JIT HostBridge] Llamada a servicio: %s\n", methodName)
	return 0
}

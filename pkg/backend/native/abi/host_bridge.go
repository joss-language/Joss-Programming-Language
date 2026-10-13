package main

import "C"

import (
	"fmt"
	// Importamos pkg/core sólo si es necesario para acceder a GranDB
	// "github.com/jossecurity/joss/pkg/core"
)

//export _joss_host_bridge
func _joss_host_bridge(methodNamePtr *C.char) {
	// Esta es la frontera ABI (Application Binary Interface) C-Compatible
	// El código máquina de Joss pondrá el nombre del método en RCX (arg0 en Win64, o RDI en SysV)
	// Aquí usaremos la implementación canónica de Go (GranDB, HTTP, etc.)

	methodName := C.GoString(methodNamePtr)
	fmt.Printf("[HostBridge ABI] Llamada nativa recibida para: %s\n", methodName)

	// Aquí irá el dispatcher a `core.Runtime.executeGranDBMethod` u otro servicio.
}

func main() {
	// Necesario para buildmode=c-archive
}

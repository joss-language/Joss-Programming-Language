//go:build cgo

package main

import "C"

//export _joss_host_bridge
func _joss_host_bridge(methodNamePtr *C.char) {
	if methodNamePtr == nil {
		return
	}
	ExecuteHostBridge(C.GoString(methodNamePtr))
}

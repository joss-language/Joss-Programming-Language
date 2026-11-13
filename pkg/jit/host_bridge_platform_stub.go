//go:build !windows || !amd64

package jit

// SupportsInProcessHostBridge reports whether this target can call a Go host
// callback from generated machine code. Other targets currently have no ABI
// trampoline registered in the native JIT image.
func SupportsInProcessHostBridge() bool { return false }

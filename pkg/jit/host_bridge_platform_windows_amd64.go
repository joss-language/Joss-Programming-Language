//go:build windows && amd64

package jit

// SupportsInProcessHostBridge reports whether this target can call a Go host
// callback from generated machine code using the Windows AMD64 callback ABI.
func SupportsInProcessHostBridge() bool { return true }

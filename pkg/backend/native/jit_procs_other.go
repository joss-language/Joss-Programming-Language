//go:build !windows

package native

func getPlatformIATPointers() [iatSlotCount]uintptr {
	// For non-Windows platforms, system calls are emitted directly in machine code.
	return [iatSlotCount]uintptr{}
}

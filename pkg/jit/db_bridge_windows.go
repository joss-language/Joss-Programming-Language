//go:build windows

package jit

import (
	"syscall"
)

var (
	modKernel32        = syscall.NewLazyDLL("kernel32.dll")
	procGetProcessHeap = modKernel32.NewProc("GetProcessHeap")
	procHeapAlloc      = modKernel32.NewProc("HeapAlloc")
)

func allocNative(size int) uintptr {
	if size <= 0 {
		return 0
	}
	hHeap, _, _ := procGetProcessHeap.Call()
	ptr, _, _ := procHeapAlloc.Call(hHeap, 8 /* HEAP_ZERO_MEMORY */, uintptr(size))
	return ptr
}

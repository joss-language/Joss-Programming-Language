//go:build !windows

package jit

import (
	"syscall"
	"unsafe"
)

func allocNative(size int) uintptr {
	if size <= 0 {
		return 0
	}
	size = (size + 15) &^ 15
	data, err := syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil || len(data) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&data[0]))
}

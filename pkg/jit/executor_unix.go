//go:build !windows

package jit

import (
	"fmt"
	"unsafe"
)

// CallFunc0 executes a native machine code function taking 0 arguments on Unix (AMD64 / ARM64).
func CallFunc0(entry uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	fn := *(*func() uintptr)(unsafe.Pointer(&entry))
	return fn(), nil
}

// CallFunc1 executes a native machine code function taking 1 argument on Unix.
func CallFunc1(entry uintptr, a1 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	fn := *(*func(uintptr) uintptr)(unsafe.Pointer(&entry))
	return fn(a1), nil
}

// CallFunc2 executes a native machine code function taking 2 arguments on Unix.
func CallFunc2(entry uintptr, a1, a2 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	fn := *(*func(uintptr, uintptr) uintptr)(unsafe.Pointer(&entry))
	return fn(a1, a2), nil
}

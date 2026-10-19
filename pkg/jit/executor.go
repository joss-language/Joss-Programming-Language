package jit

import (
	"fmt"
	"runtime"
	"syscall"
)

// CallFunc0 executes a native machine code function taking 0 arguments and returning uintptr.
// Uses Windows x64 ABI or System V AMD64 ABI depending on the platform.
func CallFunc0(entry uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}

	if runtime.GOOS == "windows" {
		r1, _, err := syscall.SyscallN(entry)
		return r1, err
	}

	// For Unix platforms, SyscallN is also supported in modern Go versions (1.18+)
	r1, _, err := syscall.SyscallN(entry)
	return r1, err
}

// CallFunc1 executes a native machine code function taking 1 argument.
func CallFunc1(entry uintptr, a1 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	r1, _, err := syscall.SyscallN(entry, a1)
	return r1, err
}

// CallFunc2 executes a native machine code function taking 2 arguments.
func CallFunc2(entry uintptr, a1, a2 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	r1, _, err := syscall.SyscallN(entry, a1, a2)
	return r1, err
}

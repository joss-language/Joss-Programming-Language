//go:build windows

package jit

import (
	"fmt"
	"syscall"
)

// CallFunc0 executes a native machine code function taking 0 arguments and returning uintptr.
func CallFunc0(entry uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	// SyscallN applies the Win32 API last-error convention. A Joss JIT entrypoint
	// is a plain native function, so a zero return value is valid and stale
	// GetLastError state must not be reported as an execution failure.
	r1, _, _ := syscall.SyscallN(entry)
	return r1, nil
}

// CallFunc1 executes a native machine code function taking 1 argument.
func CallFunc1(entry uintptr, a1 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	r1, _, _ := syscall.SyscallN(entry, a1)
	return r1, nil
}

// CallFunc2 executes a native machine code function taking 2 arguments.
func CallFunc2(entry uintptr, a1, a2 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	r1, _, _ := syscall.SyscallN(entry, a1, a2)
	return r1, nil
}

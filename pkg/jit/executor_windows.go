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
	r1, _, errno := syscall.SyscallN(entry)
	if errno != 0 {
		return r1, errno
	}
	return r1, nil
}

// CallFunc1 executes a native machine code function taking 1 argument.
func CallFunc1(entry uintptr, a1 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	r1, _, errno := syscall.SyscallN(entry, a1)
	if errno != 0 {
		return r1, errno
	}
	return r1, nil
}

// CallFunc2 executes a native machine code function taking 2 arguments.
func CallFunc2(entry uintptr, a1, a2 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	r1, _, errno := syscall.SyscallN(entry, a1, a2)
	if errno != 0 {
		return r1, errno
	}
	return r1, nil
}

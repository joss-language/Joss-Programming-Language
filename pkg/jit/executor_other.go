//go:build !windows && !amd64

package jit

import "fmt"

// CallFunc0 returns an error on unsupported non-x86_64 Unix platforms.
func CallFunc0(entry uintptr) (uintptr, error) {
	return 0, fmt.Errorf("in-memory JIT execution is not supported on this architecture")
}

// CallFunc1 returns an error on unsupported non-x86_64 Unix platforms.
func CallFunc1(entry uintptr, a1 uintptr) (uintptr, error) {
	return 0, fmt.Errorf("in-memory JIT execution is not supported on this architecture")
}

// CallFunc2 returns an error on unsupported non-x86_64 Unix platforms.
func CallFunc2(entry uintptr, a1, a2 uintptr) (uintptr, error) {
	return 0, fmt.Errorf("in-memory JIT execution is not supported on this architecture")
}

//go:build !windows && arm64

package jit

import "fmt"

func callFunc0(entry uintptr) uintptr
func callFunc1(entry uintptr, a1 uintptr) uintptr
func callFunc2(entry uintptr, a1, a2 uintptr) uintptr

func CallFunc0(entry uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	return callFunc0(entry), nil
}

func CallFunc1(entry uintptr, a1 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	return callFunc1(entry, a1), nil
}

func CallFunc2(entry uintptr, a1, a2 uintptr) (uintptr, error) {
	if entry == 0 {
		return 0, fmt.Errorf("cannot call null function pointer")
	}
	return callFunc2(entry, a1, a2), nil
}

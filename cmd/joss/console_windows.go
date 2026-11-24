//go:build windows

package main

import "syscall"

func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	if proc := kernel32.NewProc("SetConsoleOutputCP"); proc.Find() == nil {
		_, _, _ = proc.Call(65001) // CP_UTF8
	}
	if proc := kernel32.NewProc("SetConsoleCP"); proc.Find() == nil {
		_, _, _ = proc.Call(65001) // CP_UTF8
	}
}

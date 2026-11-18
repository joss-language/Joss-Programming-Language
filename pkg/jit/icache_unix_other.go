//go:build !windows && !arm64

package jit

func flushInstructionCache(uintptr, uintptr) {}

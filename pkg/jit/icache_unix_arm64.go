//go:build !windows && arm64

package jit

func flushInstructionCache(addr uintptr, size uintptr)

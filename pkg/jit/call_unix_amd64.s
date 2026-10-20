//go:build !windows && amd64

#include "textflag.h"

// func callFunc0(entry uintptr) uintptr
TEXT ·callFunc0(SB), NOSPLIT, $0-16
	MOVQ entry+0(FP), AX
	CALL AX
	MOVQ AX, ret+8(FP)
	RET

// func callFunc1(entry uintptr, a1 uintptr) uintptr
TEXT ·callFunc1(SB), NOSPLIT, $0-24
	MOVQ entry+0(FP), AX
	MOVQ a1+8(FP), DI
	CALL AX
	MOVQ AX, ret+16(FP)
	RET

// func callFunc2(entry uintptr, a1, a2 uintptr) uintptr
TEXT ·callFunc2(SB), NOSPLIT, $0-32
	MOVQ entry+0(FP), AX
	MOVQ a1+8(FP), DI
	MOVQ a2+16(FP), SI
	CALL AX
	MOVQ AX, ret+24(FP)
	RET

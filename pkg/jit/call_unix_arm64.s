#include "textflag.h"

// func callFunc0(entry uintptr) uintptr
TEXT ·callFunc0(SB), NOSPLIT, $16-16
	MOVD entry+0(FP), R16
	MOVD R30, 8(RSP)
	CALL (R16)
	MOVD 8(RSP), R30
	MOVD R0, ret+8(FP)
	RET

// func callFunc1(entry uintptr, a1 uintptr) uintptr
TEXT ·callFunc1(SB), NOSPLIT, $16-24
	MOVD entry+0(FP), R16
	MOVD a1+8(FP), R0
	MOVD R30, 8(RSP)
	CALL (R16)
	MOVD 8(RSP), R30
	MOVD R0, ret+16(FP)
	RET

// func callFunc2(entry uintptr, a1, a2 uintptr) uintptr
TEXT ·callFunc2(SB), NOSPLIT, $16-32
	MOVD entry+0(FP), R16
	MOVD a1+8(FP), R0
	MOVD a2+16(FP), R1
	MOVD R30, 8(RSP)
	CALL (R16)
	MOVD 8(RSP), R30
	MOVD R0, ret+24(FP)
	RET

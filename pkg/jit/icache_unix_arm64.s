#include "textflag.h"

// func flushInstructionCache(addr uintptr, size uintptr)
// ARM64 instruction and data caches are not guaranteed to be coherent.
TEXT ·flushInstructionCache(SB), NOSPLIT, $0-16
	MOVD addr+0(FP), R0
	MOVD size+8(FP), R1
	CBZ R1, done
	MOVD R0, R5
	MOVD $-4, R7
	AND R7, R5, R5
	ADD $3, R1, R1
	LSR $2, R1, R1
dcache:
	DC CVAU, R5
	ADD $4, R5, R5
	SUB $1, R1, R1
	CBNZ R1, dcache
	DSB $15
	MOVD addr+0(FP), R5
	MOVD size+8(FP), R1
	MOVD $-4, R7
	AND R7, R5, R5
	ADD $3, R1, R1
	LSR $2, R1, R1
icache:
	WORD $0xd50b7525 // IC IVAU, R5
	ADD $4, R5, R5
	SUB $1, R1, R1
	CBNZ R1, icache
	DSB $15
	ISB $15
done:
	RET

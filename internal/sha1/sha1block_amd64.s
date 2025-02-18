#include "textflag.h"

// func blockSIMD(h *[5]uint32, p []byte)
TEXT ·blockSIMD(SB), NOSPLIT, $0-32
	MOVQ h+0(FP), AX
	MOVQ p+8(FP), SI
	MOVQ p_len+16(FP), DX
	SHRQ $6, DX // length in blocks

	// Load initial hash values
	MOVL (0*4)(AX), R8
	MOVL (1*4)(AX), R9
	MOVL (2*4)(AX), R10
	MOVL (3*4)(AX), R11
	MOVL (4*4)(AX), R12

loop:
	// Save hash state
	MOVL R8, BX
	MOVL R9, CX
	MOVL R10, R13
	MOVL R11, R14
	MOVL R12, R15

	// Load block into XMM registers
	MOVOU (0*16)(SI), X0
	MOVOU (1*16)(SI), X1
	MOVOU (2*16)(SI), X2
	MOVOU (3*16)(SI), X3

	// Byte swap
	PSHUFB X4, X0
	PSHUFB X4, X1
	PSHUFB X4, X2
	PSHUFB X4, X3

	// 80 rounds of SHA1
	MOVL $4, CX
rounds:
	// Intel SHA1 instructions
	SHA1RNDS4 $0, X0, X4
	SHA1NEXTE X1, X0
	SHA1MSG1 X1, X2
	SHA1MSG2 X3, X2

	SHA1RNDS4 $0, X1, X4
	SHA1NEXTE X2, X1
	SHA1MSG1 X2, X3
	SHA1MSG2 X0, X3

	SHA1RNDS4 $0, X2, X4
	SHA1NEXTE X3, X2
	SHA1MSG1 X3, X0
	SHA1MSG2 X1, X0

	SHA1RNDS4 $0, X3, X4
	SHA1NEXTE X0, X3
	SHA1MSG1 X0, X1
	SHA1MSG2 X2, X1

	DECL CX
	JNZ rounds

	// Add old hash values
	ADDL BX, R8
	ADDL CX, R9
	ADDL R13, R10
	ADDL R14, R11
	ADDL R15, R12

	// Move to next block
	ADDQ $64, SI
	DECQ DX
	JNZ loop

	// Store hash state
	MOVL R8, (0*4)(AX)
	MOVL R9, (1*4)(AX)
	MOVL R10, (2*4)(AX)
	MOVL R11, (3*4)(AX)
	MOVL R12, (4*4)(AX)
	RET 
	
// +build amd64

#include "go_asm.h"
#include "textflag.h"

TEXT ·swap(SB), NOSPLIT, $0-16
	// 保存当前协程寄存器到 *old
	MOVQ old+0(FP), DI       // DI = old
	MOVQ R15, 0(DI)          // old->r15 = R15
	MOVQ R14, 8(DI)          // old->r14 = R14
	MOVQ R13, 16(DI)         // old->r13 = R13
	MOVQ R12, 24(DI)         // old->r12 = R12
	MOVQ BX, 32(DI)          // old->rbx = RBX
	MOVQ BP, 40(DI)          // old->rbp = RBP
	LEAQ 8(SP), AX           // SP+8
	MOVQ AX, 48(DI)          // old->rsp = SP+8
	MOVQ (SP), AX            // return address
	MOVQ AX, 56(DI)          // old->rip = *(SP)

	MOVQ new+8(FP), DI       // DI = new
	MOVQ 0(DI), R15          // R15 = new->r15
	MOVQ 8(DI), R14          // R14 = new->r14
	MOVQ 16(DI), R13         // R13 = new->r13
	MOVQ 24(DI), R12         // R12 = new->r12
	MOVQ 32(DI), BX          // RBX = new->rbx
	MOVQ 40(DI), BP          // RBP = new->rbp
	MOVQ 48(DI), SP          // SP = new->rsp
	MOVQ 56(DI), AX          // AX = new->rip
	JMP AX                   // jmp new->rip

// func coroEntry()
TEXT ·coroEntry(SB), NOSPLIT, $0-0
	CALL ·coroutineWrapper(SB)

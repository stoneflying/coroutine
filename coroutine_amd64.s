//go:build amd64
// +build amd64

#include "go_asm.h"
#include "textflag.h"

// Go ABI0 has no general-purpose callee-save registers. Preserve the frame
// pointer and the stack (including its return PC); leave R14 (g) and TLS alone.
// NOFRAME is essential: SP must refer to our caller's return address.
TEXT ·swap(SB), NOSPLIT|NOFRAME, $0-16
	MOVQ old+0(FP), AX
	MOVQ next+8(FP), DX
	MOVQ SP, Context_sp(AX)
	MOVQ BP, Context_bp(AX)
	MOVQ Context_bp(DX), BP
	MOVQ Context_sp(DX), SP
	RET

// Bootstrap the same RET path used by every subsequent context switch.
// top points into a Go-owned stack slice; no uintptr-to-pointer conversion
// or reflection is needed in Go code.
TEXT ·initContext(SB), NOSPLIT|NOFRAME, $0-16
	MOVQ ctx+0(FP), AX
	MOVQ top+8(FP), DX
	LEAQ ·coroEntry(SB), CX
	MOVQ CX, 0(DX)
	MOVQ DX, Context_sp(AX)
	MOVQ $0, Context_bp(AX)
	RET

TEXT ·coroEntry(SB), NOSPLIT|NOFRAME, $0-0
	CALL ·coroutineWrapper(SB)
	// coroutineWrapper finishes by switching away and must never return.
	BYTE $0x0f
	BYTE $0x0b // UD2

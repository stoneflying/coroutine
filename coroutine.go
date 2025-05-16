package coroutine

import (
	"reflect"
	"unsafe"
)

type Context struct {
	r15, r14, r13, r12, rbx, rbp, rsp, rip uintptr
}

type Coroutine struct {
	ctx      Context
	stack    []byte
	fn       func()
	finished bool
}

var mainCoro = &Coroutine{}
var current = mainCoro

func swap(old, new *Context)
func coroEntry()

func coroutineWrapper() {
	current.fn()
	current.finished = true
	Yield()
	panic("unreachable: coroutine should never resume here")
}

func NewCoroutine(fn func()) *Coroutine {
	const stackSize = 1024 * 1024
	stack := make([]byte, stackSize)
	c := &Coroutine{
		stack: stack,
		fn:    fn,
	}
	stackTop := uintptr(unsafe.Pointer(&stack[0])) + uintptr(len(stack))
	stackTop &^= 15
	stackTop -= 8
	*(*uintptr)(unsafe.Pointer(stackTop)) = uintptr(unsafe.Pointer(c))
	c.ctx.rsp = stackTop
	c.ctx.rip = reflect.ValueOf(coroEntry).Pointer()
	return c
}

func Resume(c *Coroutine) {
	prev := current
	current = c
	swap(&prev.ctx, &c.ctx)
}

func Yield() {
	if current == mainCoro {
		return
	}
	c := current
	current = mainCoro
	swap(&c.ctx, &mainCoro.ctx)
}

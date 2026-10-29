// Package coroutine is an experimental, cooperative coroutine implementation
// for amd64. It switches fixed-size stacks with Plan 9 assembly.
//
// These stacks are not registered with the Go runtime. See README.md for the
// execution restrictions; this package is not a general-purpose Go scheduler.
package coroutine

import (
	"runtime"
	"unsafe"
)

// State describes a coroutine's lifecycle.
type State uint8

const (
	Ready State = iota
	Running
	Suspended
	Finished
)

// Context contains only the state needed at a Go ABI0 call boundary. Go's
// caller spills live registers before the call; BP and SP locate its frame.
// In particular R14 is the runtime's g pointer, not a coroutine register.
type Context struct {
	sp uintptr
	bp uintptr
}

type Coroutine struct {
	ctx       Context
	stack     []byte
	fn        func()
	caller    *Coroutine
	scheduler *Scheduler
	state     State
	finished  bool
}

const stackSize = 1024 * 1024

// Only one goroutine may use this package, including independent schedulers.
var mainCoro = &Coroutine{state: Running}
var current = mainCoro

//go:noescape
func swap(old, next *Context)

// initContext writes the assembly entry PC into the initial return slot.
// Keeping this in assembly avoids reflect's ABIInternal function pointer.
//
//go:noescape
func initContext(ctx *Context, top unsafe.Pointer)

func coroEntry()

func coroutineWrapper() {
	c := current
	c.fn()
	c.fn = nil
	c.finished = true
	c.state = Finished
	Yield()
	// A finished coroutine is rejected before reaching swap. If that invariant
	// is broken, returning to coroEntry executes its UD2 trap.
}

// NewCoroutine creates a suspended computation without running fn.
// fn must not be nil. The stack is allocated lazily on the first Resume.
func NewCoroutine(fn func()) *Coroutine {
	if fn == nil {
		panic("coroutine: nil function")
	}
	return &Coroutine{fn: fn, state: Ready}
}

// State returns the current lifecycle state. Like Resume, it must be called
// from the goroutine that owns this package.
func (c *Coroutine) State() State { return c.state }

// Finished reports whether fn has returned.
func (c *Coroutine) Finished() bool { return c.finished }

// Resume runs c until it yields or returns. Resuming a finished coroutine is
// a no-op. Scheduler-owned coroutines must be run through their scheduler.
func Resume(c *Coroutine) { c.resume(nil) }

func (c *Coroutine) resume(owner *Scheduler) {
	if c == nil || c.fn == nil && !c.finished {
		panic("coroutine: invalid coroutine")
	}
	if c.scheduler != owner {
		panic("coroutine: resume through the owning scheduler")
	}
	if c.finished {
		return
	}
	if c.state == Running {
		panic("coroutine: cannot resume a running coroutine")
	}
	if c.stack == nil {
		c.stack = make([]byte, stackSize)
		initContext(&c.ctx, unsafe.Pointer(&c.stack[len(c.stack)-16]))
	}
	prev := current
	c.caller = prev
	c.state = Running
	current = c
	// Keep the root execution on one OS thread until it regains control.
	// This does not make the foreign stacks visible to the Go runtime.
	if prev == mainCoro {
		runtime.LockOSThread()
	}
	swap(&prev.ctx, &c.ctx)
	if prev == mainCoro {
		runtime.UnlockOSThread()
	}
	c.caller = nil
	if c.finished {
		// Release only after returning to the caller's stack.
		c.ctx = Context{}
		c.stack = nil
	}
	runtime.KeepAlive(c)
}

// Yield transfers control to the most recent caller of Resume. Outside a
// coroutine it is a no-op. A scheduler places a yielding coroutine last.
func Yield() {
	c := current
	if c == mainCoro {
		return
	}
	if !c.finished {
		c.state = Suspended
	}
	current = c.caller
	swap(&c.ctx, &current.ctx)
}

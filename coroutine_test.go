package coroutine

import (
	"reflect"
	"testing"
)

func TestNewCoroutine(t *testing.T) {
	var got []int
	co := NewCoroutine(func() {
		for i := 0; i < 5; i++ {
			got = append(got, i)
			Yield()
		}
	})
	if co.State() != Ready || co.stack != nil {
		t.Fatal("new coroutine should be ready with no allocated stack")
	}
	for i := 0; i < 5; i++ {
		Resume(co)
		if co.State() != Suspended || co.Finished() {
			t.Fatalf("state after yield %d = %v", i, co.State())
		}
	}
	Resume(co)
	if !co.Finished() || co.State() != Finished {
		t.Fatal("coroutine did not finish")
	}
	if co.stack != nil || co.fn != nil || co.caller != nil || co.ctx != (Context{}) {
		t.Fatal("finished coroutine retains its execution context")
	}
	Resume(co) // Completion is idempotent, not a jump into a dead stack.
	if !reflect.DeepEqual(got, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("values = %v", got)
	}
}

func TestNestedResumeReturnsToCaller(t *testing.T) {
	var got []int
	inner := NewCoroutine(func() {
		got = append(got, 2)
		Yield()
		got = append(got, 5)
	})
	outer := NewCoroutine(func() {
		got = append(got, 1)
		Resume(inner)
		got = append(got, 3)
		Yield()
		Resume(inner)
		got = append(got, 6)
	})
	Resume(outer)
	got = append(got, 4)
	Resume(outer)
	if want := []int{1, 2, 3, 4, 5, 6}; !reflect.DeepEqual(got, want) {
		t.Fatalf("execution order = %v, want %v", got, want)
	}
	if !inner.Finished() || !outer.Finished() || current != mainCoro {
		t.Fatal("nested coroutines did not return to main")
	}
}

// Keep several live stack frames and mixed values across thousands of swaps.
// The small fixed depth deliberately does not request Go stack growth.
//
//go:noinline
func yieldFrames(depth int, seed uint64) (uint64, float64) {
	local := [4]uint64{seed, seed + 1, seed + 2, seed + 3}
	f := float64(seed) + 0.5
	if depth > 0 {
		x, y := yieldFrames(depth-1, seed+1)
		return x + local[0] + local[3], y + f
	}
	for i := 0; i < 1000; i++ {
		Yield()
	}
	return local[0] + local[3], f
}

func TestValuesSurviveSwitches(t *testing.T) {
	var got uint64
	var floatGot float64
	co := NewCoroutine(func() { got, floatGot = yieldFrames(3, 10) })
	resumes := 0
	for !co.Finished() {
		Resume(co)
		resumes++
	}
	if got != 104 || floatGot != 48 || resumes != 1001 {
		t.Fatalf("got (%d, %g), resumes %d", got, floatGot, resumes)
	}
}

func TestYieldOutsideCoroutine(t *testing.T) {
	Yield()
	if current != mainCoro {
		t.Fatal("Yield changed the root context")
	}
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestInvalidCalls(t *testing.T) {
	mustPanic(t, func() { NewCoroutine(nil) })
	mustPanic(t, func() { Resume(nil) })
	mustPanic(t, func() { Resume(&Coroutine{}) })
	// Test validation on the real Go stack: panics on foreign stacks are
	// outside this experiment's supported execution model.
	co := NewCoroutine(func() {})
	co.state = Running
	mustPanic(t, func() { Resume(co) })
	var s Scheduler
	owned := s.Spawn(func() {})
	mustPanic(t, func() { Resume(owned) })
	s.running = true
	mustPanic(t, s.Run)
	s.running = false
	s.Run()
	if !owned.Finished() {
		t.Fatal("rejected manual Resume damaged the scheduler queue")
	}
}

func BenchmarkResumeYield(b *testing.B) {
	co := NewCoroutine(func() {
		for i := 0; i < b.N; i++ {
			Yield()
		}
	})
	// Allocate the stack outside the measured loop.
	Resume(co)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Resume(co)
	}
	b.StopTimer()
	if !co.Finished() {
		b.Fatal("benchmark did not finish")
	}
}

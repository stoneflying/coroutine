package coroutine

import "testing"

func TestNewCoroutine(t *testing.T) {
	co := NewCoroutine(func() {
		for i := 0; i < 5; i++ {
			println("Coroutine:", i)
			Yield()
		}
	})
	for !co.finished {
		println("Main: Resuming coroutine")
		Resume(co)
	}
	println("Coroutine finished")
}

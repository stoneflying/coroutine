package coroutine

import (
	"reflect"
	"testing"
)

func TestSchedulerRoundRobin(t *testing.T) {
	var s Scheduler
	var got []int
	for id := 0; id < 3; id++ {
		id := id
		s.Spawn(func() {
			for i := 0; i < 3; i++ {
				got = append(got, id)
				Yield()
			}
		})
	}
	s.Run()
	want := []int{0, 1, 2, 0, 1, 2, 0, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("execution order = %v, want %v", got, want)
	}
	if len(s.queue) != 0 || s.head != 0 || s.running {
		t.Fatalf("scheduler did not drain: %+v", s)
	}
}

func TestSchedulerSpawnDuringRunAndReuse(t *testing.T) {
	s := NewScheduler()
	s.Run()
	var got []int
	s.Spawn(func() {
		got = append(got, 1)
		s.Spawn(func() { got = append(got, 3) })
		Yield()
		got = append(got, 4)
	})
	s.Spawn(func() { got = append(got, 2) })
	s.Run()
	s.Spawn(func() { got = append(got, 5) })
	s.Run()
	if want := []int{1, 2, 3, 4, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("execution order = %v, want %v", got, want)
	}
}

func TestSchedulerQueueCompaction(t *testing.T) {
	var s Scheduler
	counts := make([]int, 129)
	for i := range counts {
		i := i
		s.Spawn(func() {
			counts[i]++
			Yield()
			counts[i]++
		})
	}
	s.Run()
	for i, count := range counts {
		if count != 2 {
			t.Fatalf("coroutine %d ran %d times", i, count)
		}
	}
	for i, c := range s.queue[:cap(s.queue)] {
		if c != nil {
			t.Fatalf("queue retains coroutine at slot %d", i)
		}
	}
}

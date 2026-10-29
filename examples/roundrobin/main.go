package main

import (
	"coroutine"
	"fmt"
)

func main() {
	s := coroutine.NewScheduler()
	// Record on the experimental stacks; format output on the real Go stack.
	order := make([]int, 0, 9)
	for id := 1; id <= 3; id++ {
		id := id
		s.Spawn(func() {
			for step := 0; step < 3; step++ {
				order = append(order, id)
				coroutine.Yield()
			}
		})
	}
	s.Run()
	fmt.Println(order) // [1 2 3 1 2 3 1 2 3]
}

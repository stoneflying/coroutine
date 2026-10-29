package coroutine

// Scheduler runs coroutines cooperatively in FIFO order. Each coroutine runs
// until it calls Yield or returns. Scheduler is not safe for concurrent use.
// A coroutine that blocks or never yields blocks the entire scheduler.
// The zero value is ready to use. A Scheduler must not be copied after use.
type Scheduler struct {
	queue   []*Coroutine
	head    int
	running bool
}

// NewScheduler returns an empty scheduler.
func NewScheduler() *Scheduler { return &Scheduler{} }

// Spawn appends fn to the ready queue without running it. It may be called
// while Run is executing, including from one of this scheduler's coroutines.
func (s *Scheduler) Spawn(fn func()) *Coroutine {
	c := NewCoroutine(fn)
	c.scheduler = s
	s.queue = append(s.queue, c)
	return c
}

// Run drains the ready queue. A yielding coroutine goes to the back of the
// queue; a finished coroutine is removed. Run may be called again after it
// returns, but it must not be called recursively on the same scheduler.
func (s *Scheduler) Run() {
	if s.running {
		panic("coroutine: recursive Scheduler.Run")
	}
	s.running = true
	defer func() { s.running = false }()
	for s.head < len(s.queue) {
		c := s.queue[s.head]
		s.queue[s.head] = nil // Do not retain finished stacks in the queue.
		s.head++
		// Compact before running user code so Spawn can safely append work.
		if s.head == len(s.queue) {
			s.queue = s.queue[:0]
			s.head = 0
		} else if s.head >= 64 && s.head >= len(s.queue)/2 {
			n := copy(s.queue, s.queue[s.head:])
			for i := n; i < len(s.queue); i++ {
				s.queue[i] = nil
			}
			s.queue = s.queue[:n]
			s.head = 0
		}
		c.resume(s)
		if !c.finished {
			s.queue = append(s.queue, c)
		}
	}
}

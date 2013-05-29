package semaphore

import (
	"sync"
	"sync/atomic"
)

// An integer-valued semaphore
type Semaphore struct {
	value int64
	acquireMu sync.Mutex
	wake chan struct{}
}

// Creates a new semaphore with initial value n. Panics if n is negative.
func New(n int) *Semaphore {
	if n < 0 {
		panic("negative initial value for a semaphore")
	}
	return &Semaphore{
		value: int64(n),
		wake: make(chan struct{}, 1),
	}
}

// Tries to decrease the semaphore's value by n. If it is smaller than n, waits until it grows large enough.
func (s *Semaphore) Acquire(n int) {
	if n < 0 {
		panic("Semaphore.Acquire called with negative decrement")
	}
	s.acquireMu.Lock()
	v := atomic.AddInt64(&s.value, int64(-n))
	for v < 0 {
		<-s.wake
		v = atomic.LoadInt64(&s.value)
	}
	s.acquireMu.Unlock()
}

// Increases the semaphore's value by n. Will never sleep.
func (s *Semaphore) Release(n int) {
	if n < 0 {
		panic("Semaphore.Release called with negative increment")
	}
	v := atomic.AddInt64(&s.value, int64(n))
	if v - int64(n) < 0 && v >= 0 {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// Decreases the semaphore value to 0 and returns the difference. Can sleep.
func (s *Semaphore) Drain() int {
	s.acquireMu.Lock()
	v := atomic.LoadInt64(&s.value)
	atomic.AddInt64(&s.value, -v)
	s.acquireMu.Unlock()
	return int(v)
}


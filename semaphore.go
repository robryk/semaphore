package semaphore

import (
	"sync"
	"sync/atomic"
)

// An integer-valued semaphore
type Semaphore struct {
	value     int64
	acquireMu sync.Mutex
	wake      chan struct{}
}

// Creates a new semaphore with initial value n. Panics if n is negative.
func New(n int) *Semaphore {
	if n < 0 {
		panic("negative initial value for a semaphore")
	}
	return &Semaphore{
		value: int64(n),
		wake:  make(chan struct{}, 1),
	}
}

// Tries to decrease the semaphore's value by n. If it is smaller than n, waits until it grows large enough.
func (s *Semaphore) Acquire(n int) {
	if n < 0 {
		panic("Semaphore.Acquire called with negative decrement")
	}
	v := atomic.LoadInt64(&s.value)
	for v >= int64(n) {
		if atomic.CompareAndSwapInt64(&s.value, v, v-int64(n)) {
			return
		}
		v = atomic.LoadInt64(&s.value)
	}
	s.acquireMu.Lock()
	v = atomic.AddInt64(&s.value, int64(-n))
	if v < 0 {
		<-s.wake
		v = atomic.LoadInt64(&s.value)
		if v < 0 {
			panic("semaphore: spurious wakeup")
		}
	}
	s.acquireMu.Unlock()
}

// Increases the semaphore's value by n. Will never sleep.
func (s *Semaphore) Release(n int) {
	if n < 0 {
		panic("Semaphore.Release called with negative increment")
	}
	v := atomic.AddInt64(&s.value, int64(n))
	if v-int64(n) < 0 && v >= 0 {
		select {
		case s.wake <- struct{}{}:
		default:
			panic("semaphore: unconsumed wakeup")
		}
	}
}

// Decreases the semaphore value to 0 and returns the difference.
func (s *Semaphore) Drain() int {
	for {
		v := atomic.LoadInt64(&s.value)
		if v <= 0 {
			return 0
		}
		if atomic.CompareAndSwapInt64(&s.value, v, 0) {
			return int(v)
		}
	}
	panic("unreachable")
}

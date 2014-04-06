package semaphore

import (
	"sync/atomic"
	"time"
)

// An integer-valued semaphore
type Semaphore struct {
	value     int64         // the value of the semaphore, accessed via atomic operations only
	acquireMu chan struct{} // used as a mutex to gate goroutines that are going to sleep when acquiring the semaphore
	wake      chan struct{} // used to notify a sleeping goroutine that the semaphore is once more positive
}

// Creates a new semaphore with initial value n. Panics if n is negative.
func New(n int) *Semaphore {
	if n < 0 {
		panic("negative initial value for a semaphore")
	}
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return &Semaphore{
		value:     int64(n),
		acquireMu: ch,
		wake:      make(chan struct{}, 1),
	}
}

// Tries to decrease the semaphore's value by n. Panics if n is negative. If it is smaller than n, waits until it grows
// large enough. If the cancel channel becomes readable before that happens, the request is cancelled. Returns true
// if the semaphore was decreased, false if the operation was cancelled.
func (s *Semaphore) AcquireCancellable(n int, cancel <-chan struct{}) bool {
	if n < 0 {
		panic("Semaphore.Acquire called with negative decrement")
	}
	v := atomic.LoadInt64(&s.value)
	for v >= int64(n) {
		if atomic.CompareAndSwapInt64(&s.value, v, v-int64(n)) {
			return true
		}
		v = atomic.LoadInt64(&s.value)
	}

	select {
	case <-cancel:
		return false
	case <-s.acquireMu:
	}
	defer func() {
		s.acquireMu <- struct{}{}
	}()

	v = atomic.AddInt64(&s.value, int64(-n))
	for v < 0 {
		select {
		case <-cancel:
			atomic.AddInt64(&s.value, int64(n))
			return false
		case <-s.wake:
			v = atomic.LoadInt64(&s.value)
		}
	}
	return true
}

// Tries to decrease the semaphore's value by n. Panics if n is negative. If it is smaller than n, waits until it grows
// large enough.
func (s *Semaphore) Acquire(n int) {
	s.AcquireCancellable(n, nil)
}

// Tries to decrease the semaphore's value by n. Panics if n is negative. If it is smaller than n, waits until it grows
// large enough or until delay has passed. Returns true on success and false on timeout.
func (s *Semaphore) TimedAcquire(n int, delay time.Duration) bool {
	// Fast path: we can acquire the semaphore right now
	closed := make(chan struct{})
	close(closed)
	if s.AcquireCancellable(n, closed) {
		return true
	}

	cancel := make(chan struct{})
	timer := time.AfterFunc(delay, func() {
		close(cancel)
	})
	defer timer.Stop()
	return s.AcquireCancellable(n, cancel)
}

// Increases the semaphore's value by n. Panics if n is negative. Will never sleep.
func (s *Semaphore) Release(n int) {
	if n < 0 {
		panic("Semaphore.Release called with negative increment")
	}
	v := atomic.AddInt64(&s.value, int64(n))
	if v-int64(n) < 0 && v >= 0 {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// Decreases the semaphore value to 0 and returns the difference. Will never sleep.
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

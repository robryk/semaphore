package semaphore

import (
	"sync/atomic"
	"testing"
)

type testSemaphore struct {
	s     *Semaphore
	value int64
	err   chan string
}

func (s *testSemaphore) setError(e string) {
	select {
	case s.err <- e:
	default:
	}
}

func (s *testSemaphore) Error() string {
	select {
	case e := <-s.err:
		s.err <- e
		return e
	default:
		return ""
	}
}

func (s *testSemaphore) Acquire(n int) {
	s.s.Acquire(n)
	v := atomic.AddInt64(&s.value, int64(-n))
	if v < 0 {
		s.setError("Acquire lowered the semaphore to a negative value")
	}
}

func (s *testSemaphore) Drain() int {
	n := s.s.Drain()
	v := atomic.AddInt64(&s.value, int64(-n))
	if v < 0 {
		s.setError("Drain lowered the semaphore to a negative value")
	}
	return n
}

func (s *testSemaphore) Release(n int) {
	atomic.AddInt64(&s.value, int64(n))
	s.s.Release(n)
}

func newTestSemaphore(value int) *testSemaphore {
	return &testSemaphore{
		s:     New(value),
		value: int64(value),
		err:   make(chan string, 1),
	}
}

func TestSemaphore(t *testing.T) {
	const N = 5
	const M = 1000
	s := newTestSemaphore(N)
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			for j := 0; j < M; j++ {
				s.Acquire(1)
				s.Release(1)
				s.Acquire(2)
				s.Release(2)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
	if err := s.Error(); err != "" {
		t.Error(err)
	}
}

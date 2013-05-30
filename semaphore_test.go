package semaphore

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testSemaphore struct {
	s     *Semaphore
	value int64 // value contains an upper bound on the current semaphore's value
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
	const P = 5
	const N = 1000
	s := newTestSemaphore(P)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for j := 0; j < N; j++ {
			v := s.Drain()
			s.Release(v)
		}
		wg.Done()
	}()
	for i := 0; i < P; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < N; j++ {
				s.Acquire(1)
				s.Release(1)
				s.Acquire(2)
				s.Release(2)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if err := s.Error(); err != "" {
		t.Error(err)
	}
}

func TestDrain(t *testing.T) {
	const P = 5
	const N = 1000
	s := newTestSemaphore(P)
	var wg sync.WaitGroup
	for i := 0; i < P; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < N; j++ {
				v := s.Drain()
				s.Release(v)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if err := s.Error(); err != "" {
		t.Error(err)
	}
}

func TestCancel(t *testing.T) {
	ch := make(chan struct{})
	s := New(1)
	if s.AcquireCancellable(1, ch) != true {
		t.Fatal("AcquireCancellable spuriously cancelled.")
	}
	s.Release(1)
	go func() {
		time.Sleep(time.Duration(100) * time.Millisecond)
		close(ch)
	}()
	if s.AcquireCancellable(2, ch) != false {
		t.Fatal("AcquireCancellable incorrectly succeeded")
	}
	if v := s.Drain(); v != 1 {
		t.Errorf("Drain drained %d units when %d should have been available", v, 1)
	}
}

func TestCancelConcurrent(t *testing.T) {
	const M = 5
	const N = 5
	ch := make(chan struct{}, M)
	for i := 0; i < M; i++ {
		ch <- struct{}{}
	}

	results := make(chan bool, M+N)

	s := New(2*N + 1)
	for i := 0; i < M+N; i++ {
		go func() {
			results <- s.AcquireCancellable(2, ch)
		}()
	}

	successes := 0
	for i := 0; i < M+N; i++ {
		if <-results {
			successes++
		}
	}
	close(results)

	if successes != N {
		t.Fatalf("Expected %d AcquireCancellable calls to succeed; instead %d succeeded.", N, successes)
	}
	if v := s.Drain(); v != 1 {
		t.Errorf("Drain drained %d units when %d should have been available", v, 1)
	}
}

func TestPromptCancel(t *testing.T) {
	s := New(0)
	result := make(chan bool)
	go func() {
		result <- s.AcquireCancellable(1, nil)
	}()
	time.Sleep(time.Duration(100) * time.Millisecond)
	cancel := make(chan struct{}, 1)
	cancel <- struct{}{}
	if s.AcquireCancellable(1, cancel) == true {
		t.Fatal("AcquireCancellable succeeded on a zero-valued semaphore.")
	}
	s.Release(1)
	if <-result != true {
		t.Fatal("AcquireCancellable spuriously failed.")
	}
}

func BenchmarkSemaphore(b *testing.B) {
	const P = 10
	const UnitSize = 1000
	N := int64(b.N / UnitSize / 4)
	s := newTestSemaphore(P)
	done := make(chan struct{}, P)
	for i := 0; i < P; i++ {
		go func() {
			for atomic.AddInt64(&N, int64(-1)) > 0 {
				for j := 0; j < UnitSize; j++ {
					s.Acquire(1)
					s.Release(1)
					s.Acquire(2)
					s.Release(2)
				}
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < P; i++ {
		<-done
	}
	if err := s.Error(); err != "" {
		b.Error(err)
	}
}

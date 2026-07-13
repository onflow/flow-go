package concurrentqueue

import (
	"sync"

	"github.com/ef-ds/deque"
)

// ConcurrentQueue implements a thread safe interface for unbounded FIFO queue.
type ConcurrentQueue struct {
	q deque.Deque
	m sync.Mutex
}

func (s *ConcurrentQueue) Len() int {
	s.m.Lock()
	defer s.m.Unlock()

	return s.q.Len()
}

func (s *ConcurrentQueue) Push(v any) {
	s.m.Lock()
	defer s.m.Unlock()

	s.q.PushBack(v)
}

func (s *ConcurrentQueue) Pop() (any, bool) {
	s.m.Lock()
	defer s.m.Unlock()

	return s.q.PopFront()
}

func (s *ConcurrentQueue) PopBatch(batchSize int) ([]any, bool) {
	s.m.Lock()
	defer s.m.Unlock()

	count := s.q.Len()
	if count == 0 {
		return nil, false
	}

	if count > batchSize {
		count = batchSize
	}

	result := make([]any, count)
	for i := 0; i < count; i++ {
		v, _ := s.q.PopFront()
		result[i] = v
	}

	return result, true
}

func (s *ConcurrentQueue) Front() (any, bool) {
	s.m.Lock()
	defer s.m.Unlock()

	return s.q.Front()
}

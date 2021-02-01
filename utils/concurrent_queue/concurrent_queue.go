package concurrent_queue

import (
	"sync"

	"github.com/ef-ds/deque"
)

type ConcurrentQueue struct {
	q deque.Deque
	m sync.Mutex
}

func (s *ConcurrentQueue) Len() int {
	s.m.Lock()
	defer s.m.Unlock()

	return s.q.Len()
}

func (s *ConcurrentQueue) Push(v interface{}) {
	s.m.Lock()
	defer s.m.Unlock()

	s.q.PushBack(v)
}

func (s *ConcurrentQueue) Pop() (interface{}, bool) {
	s.m.Lock()
	defer s.m.Unlock()

	return s.q.PopFront()
}

func (s *ConcurrentQueue) Front() (interface{}, bool) {
	s.m.Lock()
	defer s.m.Unlock()

	return s.q.Front()
}

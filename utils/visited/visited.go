package visited

// Visited is a simple object that tracks whether or not a particular value has been visited.
// Use this when iterating over a collection and you need to know if a value has been encountered.
//
// CAUTION: Not concurrency safe.
type Visited[T comparable] struct {
	m map[T]struct{}
}

func New[T comparable]() Visited[T] {
	return Visited[T]{
		m: make(map[T]struct{}),
	}
}

// Visit returns true if the value has been visited, false otherwise.
// It also adds the value to the set of visited values.
//
// CAUTION: Not concurrency safe.
func (s *Visited[T]) Visit(key T) bool {
	if _, ok := s.m[key]; ok {
		return true
	}
	s.m[key] = struct{}{}
	return false
}

// PeekVisited returns true if the value has been visited, false otherwise.
// It does not add the value to the set of visited values.
// Use this for use cases where marking the value as visited needs to happen after processing.
//
// CAUTION: Not concurrency safe.
func (s *Visited[T]) PeekVisited(key T) bool {
	_, ok := s.m[key]
	return ok
}

// Count returns the number of visited values.
//
// CAUTION: Not concurrency safe.
func (s *Visited[T]) Count() int {
	return len(s.m)
}

package util

import (
	"container/heap"
)

// TopN keeps track of the top N elements.
// Use Add to add elements to the list.
type TopN[T any] struct {
	Tree   []T
	N      int
	IsLess func(T, T) bool
}

func NewTopN[T any](n int, isLess func(T, T) bool) *TopN[T] {
	return &TopN[T]{
		Tree:   make([]T, 0, n),
		N:      n,
		IsLess: isLess,
	}
}

func (h *TopN[T]) Len() int {
	return len(h.Tree)
}

func (h *TopN[T]) Less(i, j int) bool {
	a := h.Tree[i]
	b := h.Tree[j]
	return h.IsLess(a, b)
}

func (h *TopN[T]) Swap(i, j int) {
	h.Tree[i], h.Tree[j] =
		h.Tree[j], h.Tree[i]
}

func (h *TopN[T]) Push(x any) {
	h.Tree = append(h.Tree, x.(T))
}

func (h *TopN[T]) Pop() any {
	tree := h.Tree
	count := len(tree)
	lastIndex := count - 1
	last := tree[lastIndex]
	var empty T
	tree[lastIndex] = empty
	h.Tree = tree[0:lastIndex]
	return last
}

// Add tries to add a value to the list.
// If the list is full, it will return the smallest value and true.
// If the list is not full, it will return the zero value and false.
func (h *TopN[T]) Add(value T) (popped T, didPop bool) {
	heap.Push(h, value)
	if h.Len() > h.N {
		popped := heap.Pop(h).(T)
		return popped, true
	}
	var empty T
	return empty, false
}

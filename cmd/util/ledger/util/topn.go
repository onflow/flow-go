package util

import (
	"container/heap"
)

// TopN keeps track of the top N elements.
// Use Add to add elements to the list.
type TopN[T any] struct {
	Tree     []T
	N        int
	IsLarger func(T, T) bool
}

func NewTopN[T any](n int, isLarger func(T, T) bool) *TopN[T] {
	return &TopN[T]{
		Tree:     make([]T, 0, n),
		N:        n,
		IsLarger: isLarger,
	}
}

func (h *TopN[T]) Len() int {
	return len(h.Tree)
}

func (h *TopN[T]) Less(i, j int) bool {
	a := h.Tree[i]
	b := h.Tree[j]
	return h.IsLarger(a, b)
}

func (h *TopN[T]) Swap(i, j int) {
	h.Tree[i], h.Tree[j] =
		h.Tree[j], h.Tree[i]
}

func (h *TopN[T]) Push(x interface{}) {
	h.Tree = append(h.Tree, x.(T))
}

func (h *TopN[T]) Pop() interface{} {
	tree := h.Tree
	count := len(tree)
	lastIndex := count - 1
	last := tree[lastIndex]
	var empty T
	tree[lastIndex] = empty
	h.Tree = tree[0:lastIndex]
	return last
}

func (h *TopN[T]) Add(value T) {
	heap.Push(h, value)
	if h.Len() > h.N {
		heap.Pop(h)
	}
}

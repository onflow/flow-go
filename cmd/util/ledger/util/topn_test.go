package util

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopN(t *testing.T) {
	t.Parallel()

	topN := NewTopN(
		3,
		func(a, b int) bool {
			return a < b
		},
	)

	_, didPop := topN.Add(5)
	assert.False(t, didPop)
	assert.ElementsMatch(t,
		[]int{5},
		topN.Tree,
	)

	_, didPop = topN.Add(2)
	assert.False(t, didPop)
	assert.ElementsMatch(t,
		[]int{5, 2},
		topN.Tree,
	)

	_, didPop = topN.Add(3)
	assert.False(t, didPop)
	assert.ElementsMatch(t,
		[]int{5, 3, 2},
		topN.Tree,
	)

	popped, didPop := topN.Add(3)
	assert.True(t, didPop)
	assert.Equal(t, 2, popped)
	assert.ElementsMatch(t,
		[]int{5, 3, 3},
		topN.Tree,
	)

	popped, didPop = topN.Add(1)
	assert.True(t, didPop)
	assert.Equal(t, 1, popped)
	assert.ElementsMatch(t,
		[]int{5, 3, 3},
		topN.Tree,
	)

	popped, didPop = topN.Add(4)
	assert.True(t, didPop)
	assert.Equal(t, 3, popped)
	assert.ElementsMatch(t,
		[]int{5, 4, 3},
		topN.Tree,
	)

	sorted := make([]int, len(topN.Tree))
	for index := topN.Len() - 1; index >= 0; index-- {
		sorted[index] = heap.Pop(topN).(int)
	}
	assert.Equal(t,
		[]int{5, 4, 3},
		sorted,
	)
	assert.Empty(t, topN.Tree)
}

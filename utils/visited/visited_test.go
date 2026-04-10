package visited

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestVisited_Visit verifies that Visit returns false on first encounter and true on repeat.
func TestVisited_Visit(t *testing.T) {
	t.Run("first visit returns false", func(t *testing.T) {
		v := New[int]()
		assert.False(t, v.Visit(1))
	})

	t.Run("second visit returns true", func(t *testing.T) {
		v := New[int]()
		v.Visit(1)
		assert.True(t, v.Visit(1))
	})

	t.Run("subsequent visits all return true", func(t *testing.T) {
		v := New[int]()
		v.Visit(1)
		assert.True(t, v.Visit(1))
		assert.True(t, v.Visit(1))
	})

	t.Run("distinct values tracked independently", func(t *testing.T) {
		v := New[string]()
		assert.False(t, v.Visit("a"))
		assert.False(t, v.Visit("b"))
		assert.True(t, v.Visit("a"))
		assert.True(t, v.Visit("b"))
	})
}

// TestVisited_Count verifies that Count reflects the number of unique visited values.
func TestVisited_Count(t *testing.T) {
	t.Run("zero on empty", func(t *testing.T) {
		v := New[int]()
		assert.Equal(t, 0, v.Count())
	})

	t.Run("increments on each new value", func(t *testing.T) {
		v := New[int]()
		v.Visit(1)
		assert.Equal(t, 1, v.Count())
		v.Visit(2)
		assert.Equal(t, 2, v.Count())
	})

	t.Run("does not increment on repeat visit", func(t *testing.T) {
		v := New[int]()
		v.Visit(1)
		v.Visit(1)
		assert.Equal(t, 1, v.Count())
	})

	t.Run("PeekVisited does not increment count", func(t *testing.T) {
		v := New[int]()
		v.PeekVisited(1)
		assert.Equal(t, 0, v.Count())
	})
}

// TestVisited_PeekVisited verifies that PeekVisited reports membership without mutating the set.
func TestVisited_PeekVisited(t *testing.T) {
	t.Run("returns false for unvisited key", func(t *testing.T) {
		v := New[int]()
		assert.False(t, v.PeekVisited(42))
	})

	t.Run("returns true for visited key", func(t *testing.T) {
		v := New[int]()
		v.Visit(42)
		assert.True(t, v.PeekVisited(42))
	})

	t.Run("does not add key to visited set", func(t *testing.T) {
		v := New[int]()
		v.PeekVisited(42)
		// Visit should still return false — PeekVisited must not have mutated the set.
		assert.False(t, v.Visit(42))
	})
}

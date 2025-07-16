package queue

import (
	"container/heap"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPriorityQueueItem tests the PriorityQueueItem struct and its methods
func TestPriorityQueueItem(t *testing.T) {
	t.Run("NewPriorityQueueItem creates item with correct values", func(t *testing.T) {
		message := "test message"
		priority := uint64(42)

		item := NewPriorityQueueItem(message, priority)

		assert.Equal(t, message, item.message)
		assert.Equal(t, priority, item.priority)
		assert.Equal(t, -1, item.index)
		assert.WithinDuration(t, time.Now(), item.timestamp, time.Minute)
	})

	t.Run("Message returns the stored message", func(t *testing.T) {
		message := "test message"
		item := NewPriorityQueueItem(message, 1)

		result := item.Message()

		assert.Equal(t, message, result)
	})
}

// TestPriorityQueue_Len tests the Len method
func TestPriorityQueue_Len(t *testing.T) {
	t.Run("empty queue has length 0", func(t *testing.T) {
		pq := PriorityQueue[string]{}

		assert.Equal(t, 0, pq.Len())
	})

	t.Run("queue with items has correct length", func(t *testing.T) {
		pq := PriorityQueue[string]{
			NewPriorityQueueItem("item1", 1),
			NewPriorityQueueItem("item2", 2),
			NewPriorityQueueItem("item3", 3),
		}

		assert.Equal(t, 3, pq.Len())
	})
}

// TestPriorityQueue_Less tests the Less method for priority ordering
func TestPriorityQueue_Less(t *testing.T) {
	t.Run("higher priority comes first", func(t *testing.T) {
		pq := PriorityQueue[string]{
			NewPriorityQueueItem("low", 1),
			NewPriorityQueueItem("high", 10),
		}

		// high priority should be "less" (come first in heap)
		assert.True(t, pq.Less(1, 0))
		assert.False(t, pq.Less(0, 1))
	})

	t.Run("equal priority uses timestamp ordering", func(t *testing.T) {
		now := time.Now()
		item1 := &PriorityQueueItem[string]{
			message:   "first",
			priority:  5,
			timestamp: now,
		}
		item2 := &PriorityQueueItem[string]{
			message:   "second",
			priority:  5,
			timestamp: now.Add(time.Millisecond),
		}

		pq := PriorityQueue[string]{item1, item2}

		// older timestamp should be "less" (come first in heap)
		assert.True(t, pq.Less(0, 1))
		assert.False(t, pq.Less(1, 0))
	})

	t.Run("same priority and timestamp", func(t *testing.T) {
		now := time.Now()
		item1 := &PriorityQueueItem[string]{
			message:   "item1",
			priority:  5,
			timestamp: now,
		}
		item2 := &PriorityQueueItem[string]{
			message:   "item2",
			priority:  5,
			timestamp: now,
		}

		pq := PriorityQueue[string]{item1, item2}

		// Should be consistent (not less)
		assert.False(t, pq.Less(0, 1))
		assert.False(t, pq.Less(1, 0))
	})
}

// TestPriorityQueue_Swap tests the Swap method
func TestPriorityQueue_Swap(t *testing.T) {
	t.Run("swap exchanges items and updates indices", func(t *testing.T) {
		item1 := NewPriorityQueueItem("item1", 1)
		item2 := NewPriorityQueueItem("item2", 2)

		pq := PriorityQueue[string]{item1, item2}

		// Set initial indices
		pq[0].index = 0
		pq[1].index = 1

		// Swap items
		pq.Swap(0, 1)

		// Check that items are swapped
		assert.Equal(t, "item2", pq[0].message)
		assert.Equal(t, "item1", pq[1].message)

		// Check that indices are updated
		assert.Equal(t, 0, pq[0].index)
		assert.Equal(t, 1, pq[1].index)
	})
}

// TestPriorityQueue_Push tests the Push method
func TestPriorityQueue_Push(t *testing.T) {
	t.Run("push adds item to queue", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		item := NewPriorityQueueItem("test", 5)

		pq.Push(item)

		assert.Equal(t, 1, pq.Len())
		assert.Equal(t, item, (*pq)[0])
		assert.Equal(t, 0, item.index)
	})

	t.Run("push sets correct index", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		item1 := NewPriorityQueueItem("item1", 1)
		item2 := NewPriorityQueueItem("item2", 2)

		pq.Push(item1)
		pq.Push(item2)

		assert.Equal(t, 0, item1.index)
		assert.Equal(t, 1, item2.index)
	})

	t.Run("push panics on non-PriorityQueueItem", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		initialLen := pq.Len()

		defer func() {
			r := recover()
			require.Equal(t, "unexpected type added to priority queue: string", r)
			assert.Equal(t, initialLen, pq.Len())
		}()

		pq.Push("not an item")
	})
}

// TestPriorityQueue_Pop tests the Pop method
func TestPriorityQueue_Pop(t *testing.T) {
	t.Run("pop removes and returns last item", func(t *testing.T) {
		item1 := NewPriorityQueueItem("item1", 1)
		item2 := NewPriorityQueueItem("item2", 2)

		pq := &PriorityQueue[string]{item1, item2}
		initialLen := pq.Len()

		result := pq.Pop()

		assert.Equal(t, item2, result)
		assert.Equal(t, initialLen-1, pq.Len())
		assert.Equal(t, -1, item2.index)
	})
}

// TestPriorityQueue_HeapOperations tests the priority queue as a heap
func TestPriorityQueue_HeapOperations(t *testing.T) {
	t.Run("heap operations maintain priority order", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		heap.Init(pq)

		// Add items with different priorities
		heap.Push(pq, NewPriorityQueueItem("low", 1))
		heap.Push(pq, NewPriorityQueueItem("high", 10))
		heap.Push(pq, NewPriorityQueueItem("medium", 5))

		// Pop items and verify they come out in priority order
		item1 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "high", item1.message)
		assert.Equal(t, uint64(10), item1.priority)

		item2 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "medium", item2.message)
		assert.Equal(t, uint64(5), item2.priority)

		item3 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "low", item3.message)
		assert.Equal(t, uint64(1), item3.priority)

		assert.Equal(t, 0, pq.Len())
	})

	t.Run("heap operations with equal priorities use timestamp", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		heap.Init(pq)

		// Add items with same priority but different timestamps
		item1 := NewPriorityQueueItem("first", 5)
		time.Sleep(time.Millisecond)
		item2 := NewPriorityQueueItem("second", 5)
		time.Sleep(time.Millisecond)
		item3 := NewPriorityQueueItem("third", 5)

		heap.Push(pq, item2) // Add in different order
		heap.Push(pq, item3)
		heap.Push(pq, item1)

		// Pop items and verify they come out in timestamp order (oldest first)
		result1 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "first", result1.message)

		result2 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "second", result2.message)

		result3 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "third", result3.message)
	})

	t.Run("heap operations with mixed priorities and timestamps", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		heap.Init(pq)

		// Add items with different priorities and timestamps
		item1 := NewPriorityQueueItem("low1", 1)
		time.Sleep(time.Millisecond)
		item2 := NewPriorityQueueItem("low2", 1)
		time.Sleep(time.Millisecond)
		item3 := NewPriorityQueueItem("high1", 10)
		time.Sleep(time.Millisecond)
		item4 := NewPriorityQueueItem("high2", 10)

		heap.Push(pq, item4)
		heap.Push(pq, item1)
		heap.Push(pq, item3)
		heap.Push(pq, item2)

		// Pop items and verify order
		results := make([]string, 4)
		for i := 0; i < 4; i++ {
			item := heap.Pop(pq).(*PriorityQueueItem[string])
			results[i] = item.message
		}

		// High priority items should come first, then low priority items
		// Within same priority, older timestamps should come first
		expected := []string{"high1", "high2", "low1", "low2"}
		assert.Equal(t, expected, results)
	})
}

// TestPriorityQueue_EdgeCases tests edge cases and error conditions
func TestPriorityQueue_EdgeCases(t *testing.T) {
	t.Run("empty queue operations", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		heap.Init(pq)

		assert.Equal(t, 0, pq.Len())

		// Pop on empty queue should panic (heap behavior)
		assert.Panics(t, func() {
			heap.Pop(pq)
		})
	})

	t.Run("single item queue", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		heap.Init(pq)

		item := NewPriorityQueueItem("single", 5)
		heap.Push(pq, item)

		assert.Equal(t, 1, pq.Len())

		result := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, item, result)
		assert.Equal(t, 0, pq.Len())
	})

	t.Run("zero priority items", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		heap.Init(pq)

		item1 := NewPriorityQueueItem("zero1", 0)
		time.Sleep(time.Millisecond)
		item2 := NewPriorityQueueItem("zero2", 0)

		heap.Push(pq, item2)
		heap.Push(pq, item1)

		// Should come out in timestamp order
		result1 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "zero1", result1.message)

		result2 := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "zero2", result2.message)
	})

	t.Run("very high priority values", func(t *testing.T) {
		pq := &PriorityQueue[string]{}
		heap.Init(pq)

		item1 := NewPriorityQueueItem("normal", 1000)
		item2 := NewPriorityQueueItem("very high", math.MaxUint64)

		heap.Push(pq, item1)
		heap.Push(pq, item2)

		result := heap.Pop(pq).(*PriorityQueueItem[string])
		assert.Equal(t, "very high", result.message)
		assert.Equal(t, uint64(math.MaxUint64), result.priority)
	})
}

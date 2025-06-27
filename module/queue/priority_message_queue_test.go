package queue

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
)

// TestNewPriorityMessageQueue tests the constructor
func TestNewPriorityMessageQueue(t *testing.T) {
	t.Run("creates queue with larger values first", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		assert.NotNil(t, mq)
		assert.NotNil(t, mq.queue)
		assert.False(t, mq.smallerValuesFirst)
		assert.NotNil(t, mq.ch)
		assert.Equal(t, 0, mq.Len())
	})

	t.Run("creates queue with smaller values first", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](true)

		assert.NotNil(t, mq)
		assert.NotNil(t, mq.queue)
		assert.True(t, mq.smallerValuesFirst)
		assert.NotNil(t, mq.ch)
		assert.Equal(t, 0, mq.Len())
	})
}

// TestPriorityMessageQueue_Len tests the Len method
func TestPriorityMessageQueue_Len(t *testing.T) {
	t.Run("empty queue returns 0", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		assert.Equal(t, 0, mq.Len())
	})

	t.Run("queue with items returns correct length", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		mq.Push("item1", 1)
		mq.Push("item2", 2)
		mq.Push("item3", 3)

		assert.Equal(t, 3, mq.Len())
	})

	t.Run("length decreases after pop", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		mq.Push("item1", 1)
		mq.Push("item2", 2)

		assert.Equal(t, 2, mq.Len())

		_, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, 1, mq.Len())
	})
}

// TestPriorityMessageQueue_Push tests the Push method
func TestPriorityMessageQueue_Push(t *testing.T) {
	t.Run("push adds items with larger values first", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		mq.Push("low", 1)
		mq.Push("high", 10)
		mq.Push("medium", 5)

		assert.Equal(t, 3, mq.Len())

		// Pop items and verify they come out in priority order
		item1, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "high", item1)

		item2, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "medium", item2)

		item3, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "low", item3)

		_, ok = mq.Pop()
		assert.False(t, ok)
	})

	t.Run("push adds items with smaller values first", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](true)

		mq.Push("high", 10)
		mq.Push("low", 1)
		mq.Push("medium", 5)

		assert.Equal(t, 3, mq.Len())

		// Pop items and verify they come out in priority order (smaller first)
		item1, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "low", item1)

		item2, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "medium", item2)

		item3, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "high", item3)

		_, ok = mq.Pop()
		assert.False(t, ok)
	})

	t.Run("push with zero priority", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		mq.Push("zero", 0)
		mq.Push("high", 100)

		// Zero priority should come last when larger values are first
		item1, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "high", item1)

		item2, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "zero", item2)
	})

	t.Run("push with zero priority and smaller values first", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](true)

		mq.Push("high", 100)
		mq.Push("zero", 0)

		// Zero priority should come first when smaller values are first
		item1, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "zero", item1)

		item2, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "high", item2)
	})
}

// TestPriorityMessageQueue_Pop tests the Pop method
func TestPriorityMessageQueue_Pop(t *testing.T) {
	t.Run("pop on empty queue returns false", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		item, ok := mq.Pop()
		assert.False(t, ok)
		var zero string
		assert.Equal(t, zero, item)
	})

	t.Run("pop returns items in priority order", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		mq.Push("item1", 1)
		mq.Push("item3", 3)
		mq.Push("item2", 2)

		// Should come out in descending priority order
		item1, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "item3", item1)

		item2, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "item2", item2)

		item3, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "item1", item3)

		_, ok = mq.Pop()
		assert.False(t, ok)
	})

	t.Run("pop with equal priorities uses timestamp ordering", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		// Add items with same priority but different timestamps
		mq.Push("first", 5)
		time.Sleep(time.Millisecond)
		mq.Push("second", 5)
		time.Sleep(time.Millisecond)
		mq.Push("third", 5)

		// Should come out in insertion order (oldest first)
		item1, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "first", item1)

		item2, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "second", item2)

		item3, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "third", item3)
	})
}

// TestPriorityMessageQueue_Channel tests the Channel method
func TestPriorityMessageQueue_Channel(t *testing.T) {
	t.Run("channel receives notification on push", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)
		ch := mq.Channel()

		// Push an item
		mq.Push("test", 1)

		// Should receive notification
		select {
		case <-ch:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Fatal("did not receive notification within timeout")
		}
	})

	t.Run("channel does not block on multiple pushes", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)
		ch := mq.Channel()

		// Push multiple items rapidly
		for i := 0; i < 10; i++ {
			mq.Push("test", uint64(i))
		}

		// Should receive at least one notification
		select {
		case <-ch:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Fatal("did not receive notification within timeout")
		}

		// Channel should be buffered and not block
		assert.Equal(t, 10, mq.Len())
	})

	t.Run("channel is buffered", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)
		ch := mq.Channel()

		// Push multiple items without reading from channel
		for i := 0; i < 5; i++ {
			mq.Push("test", uint64(i))
		}

		// Should not block
		assert.Equal(t, 5, mq.Len())

		// Should be able to read from channel
		select {
		case <-ch:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Fatal("did not receive notification within timeout")
		}
	})
}

// TestPriorityMessageQueue_Concurrency tests thread safety
func TestPriorityMessageQueue_Concurrency(t *testing.T) {
	t.Run("concurrent push operations", func(t *testing.T) {
		mq := NewPriorityMessageQueue[int](false)
		numGoroutines := 1000
		var wg sync.WaitGroup

		// Start multiple goroutines pushing items
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				mq.Push(id, uint64(id))
			}(i)
		}

		wg.Wait()

		// Verify all items were added
		assert.Equal(t, numGoroutines, mq.Len())

		// Verify items can be popped correctly
		popped := make(map[int]bool)
		for i := 0; i < numGoroutines; i++ {
			item, ok := mq.Pop()
			assert.True(t, ok)
			assert.False(t, popped[item], "duplicate item popped: %d", item)
			popped[item] = true
		}

		assert.Equal(t, numGoroutines, len(popped))
	})

	t.Run("concurrent push and pop operations", func(t *testing.T) {
		mq := NewPriorityMessageQueue[int](false)
		numGoroutines := 1000
		var wg sync.WaitGroup
		var mu sync.Mutex
		popped := make(map[int]bool)

		// Start goroutines that push items
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				mq.Push(id, uint64(id))
			}(i)
		}

		// Wait for all pushes to complete
		wg.Wait()

		// Now start goroutines that pop items
		var popWg sync.WaitGroup
		for i := 0; i < numGoroutines/2; i++ {
			popWg.Add(1)
			go func() {
				defer popWg.Done()
				for {
					item, ok := mq.Pop()
					if !ok {
						break
					}
					mu.Lock()
					assert.False(t, popped[item], "duplicate item popped: %d", item)
					popped[item] = true
					mu.Unlock()
				}
			}()
		}

		popWg.Wait()

		// Verify no duplicates and all items were processed
		assert.Equal(t, numGoroutines, len(popped))
	})

	t.Run("concurrent len operations", func(t *testing.T) {
		mq := NewPriorityMessageQueue[int](false)
		numGoroutines := 1000
		var wg sync.WaitGroup

		// Start goroutines that push items
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				mq.Push(id, uint64(id))
			}(i)
		}

		// Start goroutines that call Len
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_ = mq.Len()
				}
			}()
		}

		wg.Wait()

		// Verify final length
		assert.Equal(t, numGoroutines, mq.Len())
	})
}

// TestPriorityMessageQueue_EdgeCases tests edge cases
func TestPriorityMessageQueue_EdgeCases(t *testing.T) {
	t.Run("max uint64 priority with larger values first", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		mq.Push("normal", 1000)
		mq.Push("max", math.MaxUint64)

		item, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "max", item)
	})

	t.Run("max uint64 priority with smaller values first", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](true)

		mq.Push("normal", 1000)
		mq.Push("max", math.MaxUint64)

		item, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, "normal", item) // max priority becomes 0 after inversion
	})

	t.Run("empty queue after popping all items", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		mq.Push("item1", 1)
		mq.Push("item2", 2)

		assert.Equal(t, 2, mq.Len())

		_, ok := mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, 1, mq.Len())

		_, ok = mq.Pop()
		assert.True(t, ok)
		assert.Equal(t, 0, mq.Len())

		_, ok = mq.Pop()
		assert.False(t, ok)
		assert.Equal(t, 0, mq.Len())
	})

	t.Run("channel notification after queue becomes empty", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)
		ch := mq.Channel()

		mq.Push("item", 1)

		// Read the notification
		<-ch

		// Pop the item
		_, ok := mq.Pop()
		assert.True(t, ok)

		// Push another item
		mq.Push("item2", 2)

		// Should receive another notification
		select {
		case <-ch:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Fatal("did not receive notification within timeout")
		}
	})
}

// TestPriorityMessageQueue_Integration tests integration scenarios
func TestPriorityMessageQueue_Integration(t *testing.T) {
	t.Run("mixed operations with different priorities", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](false)

		// Add items with various priorities
		mq.Push("urgent", 100)
		mq.Push("normal", 50)
		mq.Push("low", 10)
		mq.Push("critical", 200)
		mq.Push("medium", 75)

		// Pop all items and verify order
		expected := []string{"critical", "urgent", "medium", "normal", "low"}
		for _, exp := range expected {
			item, ok := mq.Pop()
			assert.True(t, ok)
			assert.Equal(t, exp, item)
		}

		_, ok := mq.Pop()
		assert.False(t, ok)
	})

	t.Run("priority inversion with mixed operations", func(t *testing.T) {
		mq := NewPriorityMessageQueue[string](true)

		// Add items with various priorities
		mq.Push("urgent", 100)
		mq.Push("normal", 50)
		mq.Push("low", 10)
		mq.Push("critical", 200)
		mq.Push("medium", 75)

		// Pop all items and verify order (smaller values first)
		expected := []string{"low", "normal", "medium", "urgent", "critical"}
		for _, exp := range expected {
			item, ok := mq.Pop()
			assert.True(t, ok)
			assert.Equal(t, exp, item)
		}

		_, ok := mq.Pop()
		assert.False(t, ok)
	})

	t.Run("queue processing using channel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mq := NewPriorityMessageQueue[string](true)

		itemCount := 100
		go func() {
			for i := range itemCount {
				time.Sleep(time.Millisecond)
				mq.Push(fmt.Sprintf("item-%d", i), uint64(i))
			}
		}()

		unittest.RequireReturnsBefore(t, func() {
			for i := range itemCount {
				select {
				case <-ctx.Done():
					return
				case <-mq.Channel():
				}

				for {
					message, ok := mq.Pop()
					if !ok {
						break
					}
					assert.Equal(t, fmt.Sprintf("item-%d", i), message)
				}
			}
		}, time.Second, "did not receive all messages within timeout")

		// make sure the queue is empty
		assert.Zero(t, mq.Len())
	})
}

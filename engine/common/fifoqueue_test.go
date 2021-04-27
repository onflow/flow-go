package fifoqueue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushAndPull(t *testing.T) {
	queue, err := NewFifoQueue()
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		queue.Push(i)
	}

	require.Equal(t, 10, queue.Len())

	for i := 0; i < 10; i++ {
		n, ok := queue.Pop()
		require.True(t, ok)
		require.Equal(t, i, n)
	}
	require.Equal(t, 0, queue.Len())

	_, ok := queue.Pop()
	require.False(t, ok)
}

func TestConcurrentPushPull(t *testing.T) {
	queue, err := NewFifoQueue()
	require.NoError(t, err)

	count := 100
	// verify that concurrent push will end up having 100 items in the queue
	var sent sync.WaitGroup
	for i := 0; i < count; i++ {
		sent.Add(1)
		go func(i int) {
			queue.Push(i)
			sent.Done()
		}(i)
	}
	sent.Wait()

	require.Equal(t, count, queue.Len())

	// verify that concurrent get will alway get one, and in the end, the queue
	// is empty
	for i := 0; i < count; i++ {
		sent.Add(1)
		go func(i int) {
			_, ok := queue.Pop()
			sent.Done()
			require.True(t, ok)
		}(i)
	}
	sent.Wait()

	require.Equal(t, 0, queue.Len())
}

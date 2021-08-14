package concurrentqueue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type testEntry struct {
	workerIndex int
	value       int
}

// TestSafeQueue checks if `Push` and `Pop` operations work in concurrent environment
func TestSafeQueue(t *testing.T) {
	var q ConcurrentQueue
	workers := 1000
	samples := 100
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(worker int) {
			for i := 1; i <= samples; i++ {
				q.Push(testEntry{
					workerIndex: worker,
					value:       i,
				})
			}
			wg.Done()
		}(i)
	}

	wg.Wait()

	r := make([]int, workers)

	// accumulate all values
	for {
		v, found := q.Pop()
		if !found {
			break
		}
		entry := v.(testEntry)
		r[entry.workerIndex] += entry.value
	}
	// (a1 + aN) * N / 2
	expected := (samples + 1) * samples / 2
	for _, acc := range r {
		require.Equal(t, expected, acc)
	}
}

// TestPopBatch checks correctness of `PopBatch`.
func TestPopBatch(t *testing.T) {
	var q ConcurrentQueue
	samples := 100
	for i := 1; i <= samples; i++ {
		q.Push(i)
	}

	// (a1 + aN) * N / 2
	expected := (samples + 1) * samples / 2
	actual := 0
	for {
		v, found := q.PopBatch(11)
		if !found {
			break
		}
		for _, item := range v {
			actual += item.(int)
		}
	}

	require.Equal(t, 0, q.Len())
	require.Equal(t, expected, actual)
}

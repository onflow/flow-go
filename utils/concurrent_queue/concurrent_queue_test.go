package concurrent_queue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type testEntry struct {
	workerIndex int
	value       int
}

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

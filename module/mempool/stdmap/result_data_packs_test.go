package stdmap_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// check the implementation
var _ mempool.ResultDataPacks = (*stdmap.ResultDataPacks)(nil)

func TestAddByID(t *testing.T) {
	results := stdmap.NewResultDataPacks(10)
	result := &flow.ExecutionResult{
		BlockID: flow.Identifier{0xbb},
	}
	pending := &verification.ResultDataPack{
		ExecutorID:      flow.Identifier{0xaa},
		ExecutionResult: result,
	}
	added := results.Add(pending)
	require.True(t, added)

	p, exists := results.Get(result.ID())
	require.True(t, exists)
	require.Equal(t, pending, p)

	_, exists = results.Get(flow.Identifier{0xcc})
	require.False(t, exists)
}

func TestConcurrency(t *testing.T) {
	size := uint(10)
	results := stdmap.NewResultDataPacks(size)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			header := &flow.Header{View: uint64(i)}
			result := &flow.ExecutionResult{
				BlockID: header.ID(),
			}
			pending := &verification.ResultDataPack{
				ExecutorID:      flow.Identifier{0xaa},
				ExecutionResult: result,
			}
			added := results.Add(pending)
			require.True(t, added)
			wg.Done()
		}(i)
	}
	wg.Wait()
	require.Equal(t, uint(10), results.Size())
}

// TestEjection evaluates that adding items beyond the size limit of mempool
// ejects the oldest item and keeps mempool size within limit.
func TestEjection(t *testing.T) {
	size := uint(10)
	results := stdmap.NewResultDataPacks(size)
	rdps := make([]*verification.ResultDataPack, size+1)
	for i := 0; i < int(size)+1; i++ {
		header := &flow.Header{View: uint64(i)}
		result := &flow.ExecutionResult{
			BlockID: header.ID(),
		}
		rdp := &verification.ResultDataPack{
			ExecutorID:      flow.Identifier{0xaa},
			ExecutionResult: result,
		}
		added := results.Add(rdp)
		require.True(t, added)

		rdps[i] = rdp
	}

	// size of mempool should result on the limit size
	require.Equal(t, size, results.Size())

	// first item should be ejected to make room the 11th one
	assert.False(t, results.Has(rdps[0].ID()))

	// all results except the first one should still reside in mempool
	for i := 1; i < int(size)+1; i++ {
		assert.True(t, results.Has(rdps[i].ID()))
	}
}

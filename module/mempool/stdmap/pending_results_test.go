package stdmap_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

// check the implementation
var _ mempool.PendingResults = (*stdmap.PendingResults)(nil)

func TestAddByID(t *testing.T) {
	results := stdmap.NewPendingResults()
	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID: flow.Identifier{0xbb},
		},
	}
	pending := &flow.PendingResult{
		ExecutorID:      flow.Identifier{0xaa},
		ExecutionResult: result,
	}
	added := results.Add(pending)
	require.True(t, added)

	p, exists := results.ByID(result.ID())
	require.True(t, exists)
	require.Equal(t, pending, p)

	_, exists = results.ByID(flow.Identifier{0xcc})
	require.False(t, exists)
}

func TestConcurrency(t *testing.T) {
	results := stdmap.NewPendingResults()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			header := &flow.Header{View: uint64(i)}
			result := &flow.ExecutionResult{
				ExecutionResultBody: flow.ExecutionResultBody{
					BlockID: header.ID(),
				},
			}
			pending := &flow.PendingResult{
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

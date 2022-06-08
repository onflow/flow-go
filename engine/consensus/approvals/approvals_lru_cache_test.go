package approvals

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestApprovalsCache_Get_Put_All tests common use cases for approvals cache.
func TestApprovalsLRUCacheSecondaryIndexPurge(t *testing.T) {
	numElements := uint(10)
	cache := NewApprovalsLRUCache(numElements)
	approvals := make([]*flow.ResultApproval, 2*numElements)
	for i := range approvals {
		approval := unittest.ResultApprovalFixture()
		approvals[i] = approval
		cache.Put(approval)
		require.Equal(t, approval, cache.Get(approval.Body.PartialID()))
	}

	// LRU kept the last 10 approvals
	for i := 10; i < 20; i++ {
		approval := approvals[i]
		require.Equal(t, approval, cache.Get(approval.Body.PartialID()))
	}
	require.Len(t, cache.byResultID, int(numElements))
}

func TestApprovalsLRUCacheSecondaryIndexPurgeConcurrently(t *testing.T) {
	numElements := 100
	cache := NewApprovalsLRUCache(uint(numElements))

	var wg sync.WaitGroup

	for i := 0; i < 2*numElements; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			approval := unittest.ResultApprovalFixture()
			cache.Put(approval)
		}()
	}
	wg.Wait()
	require.Len(t, cache.byResultID, int(numElements))
}

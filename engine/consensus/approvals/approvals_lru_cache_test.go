package approvals

import (
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
	require.Equal(t, int(numElements), len(cache.byResultID))
}

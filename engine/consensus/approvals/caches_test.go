package approvals

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestApprovalsCache_Get_Put_All tests common use cases for approvals cache.
func TestApprovalsCache_Get_Put_All(t *testing.T) {
	cache := NewApprovalsCache(10)
	approvals := make([]*flow.ResultApproval, 0, 10)
	for i := range approvals {
		approval := unittest.ResultApprovalFixture()
		approvals[i] = approval
		require.True(t, cache.Put(approval.Body.PartialID(), approval))
		require.Equal(t, approval, cache.Get(approval.Body.PartialID()))
	}
	require.ElementsMatch(t, approvals, cache.All())
}

// TestIncorporatedResultsCache_Get_Put_All tests common use cases for incorporated results cache.
func TestIncorporatedResultsCache_Get_Put_All(t *testing.T) {
	cache := NewIncorporatedResultsCache(10)
	results := make([]*flow.IncorporatedResult, 0, 10)
	for i := range results {
		result := unittest.IncorporatedResult.Fixture()
		results[i] = result
		require.True(t, cache.Put(result.ID(), result))
		require.Equal(t, result, cache.Get(result.ID()))
	}
	require.ElementsMatch(t, results, cache.All())
}

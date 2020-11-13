package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovals(t *testing.T) {
	approvalPL, err := NewApprovals(10)

	t.Run("creating new mempool", func(t *testing.T) {
		require.NoError(t, err)
	})

	approval1 := unittest.ResultApprovalFixture()
	t.Run("adding first approval", func(t *testing.T) {
		ok, err := approvalPL.Add(approval1)
		require.True(t, ok)
		require.NoError(t, err)

		// checks the existence of approval for key
		approvals := approvalPL.ByChunk(approval1.Body.ExecutionResultID, approval1.Body.ChunkIndex)
		require.Contains(t, approvals, approval1.Body.ApproverID)
	})

	// insert another approval for the same chunk
	approval2 := unittest.ResultApprovalFixture(
		unittest.WithExecutionResultID(approval1.Body.ExecutionResultID),
		unittest.WithBlockID(approval1.Body.BlockID),
		unittest.WithChunk(approval1.Body.ChunkIndex),
	)
	t.Run("adding second approval in same chunk", func(t *testing.T) {
		ok, err := approvalPL.Add(approval2)
		require.True(t, ok)
		require.NoError(t, err)

		// checks the existence of approvals for key
		approvals := approvalPL.ByChunk(approval2.Body.ExecutionResultID, approval2.Body.ChunkIndex)
		require.Contains(t, approvals, approval1.Body.ApproverID)
		require.Contains(t, approvals, approval2.Body.ApproverID)
	})

	approval3 := unittest.ResultApprovalFixture()
	t.Run("adding third approval", func(t *testing.T) {
		ok, err := approvalPL.Add(approval3)
		require.True(t, ok)
		require.NoError(t, err)

		// checks the existence of approval for key
		approvals := approvalPL.ByChunk(approval3.Body.ExecutionResultID, approval3.Body.ChunkIndex)
		require.Contains(t, approvals, approval3.Body.ApproverID)
		require.Equal(t, 1, len(approvals))
	})

	t.Run("getting all approvals", func(t *testing.T) {
		all := approvalPL.All()

		// All should return all approvals in mempool
		assert.Contains(t, all, approval1)
		assert.Contains(t, all, approval2)
		assert.Contains(t, all, approval3)
	})

	// tests against removing a chunk's approvals
	t.Run("removing chunk", func(t *testing.T) {
		ok, err := approvalPL.RemChunk(approval1.Body.ExecutionResultID, approval1.Body.ChunkIndex)
		require.True(t, ok)
		require.NoError(t, err)

		// getting chunk should return empty map
		approvals := approvalPL.ByChunk(approval1.Body.ExecutionResultID, approval1.Body.ChunkIndex)
		require.Empty(t, approvals)

		// All method should only return approval3
		all := approvalPL.All()
		assert.NotContains(t, all, approval1)
		assert.NotContains(t, all, approval2)
		assert.Contains(t, all, approval3)
	})

	// tests against appending an existing approval
	t.Run("duplicate approval", func(t *testing.T) {
		ok, err := approvalPL.Add(approval3)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("check size", func(t *testing.T) {
		size := approvalPL.Size()
		require.Equal(t, uint(1), size)
	})
}

// Test that size gets decremented when items are automatically ejected
func TestApprovalsEjectSize(t *testing.T) {

	t.Run("check ejection of chunk with only a single approval", func(t *testing.T) {
		approvalPL, _ := NewApprovals(10)

		// insert 20 items (10 above limit)
		for i := 0; i < 20; i++ {
			_, _ = approvalPL.Add(unittest.ResultApprovalFixture())
		}

		// 10 items should have been evicted, so size 10
		require.Equal(t, uint(10), approvalPL.Size())
	})

	t.Run("check ejection of chunk with multiple approvals", func(t *testing.T) {
		var ejector EjectFunc = NewLRUEjector().Eject
		approvalPL, _ := NewApprovals(10, WithEject(ejector))

		blockID := unittest.IdentifierFixture()
		executionResultID := unittest.IdentifierFixture()

		for c := 0; c < 10; c++ {
			// insert 3 approvals for Block's chunk 1
			for i := 0; i < 3; i++ {
				// insert another approval for the same chunk
				a := unittest.ResultApprovalFixture(
					unittest.WithBlockID(blockID),
					unittest.WithExecutionResultID(executionResultID),
					unittest.WithChunk(uint64(c)),
				)
				_, _ = approvalPL.Add(a)
			}
			// The mempool stores all approvals for the same chunk internally in one data structure.
			// Therefore, all approvals for the same chunk consume only capacity 1.
		}
		// mempool should now be at capacity limit
		require.Equal(t, uint(30), approvalPL.Size())

		// Adding another element should overflow the mempool;
		a := unittest.ResultApprovalFixture(
			unittest.WithBlockID(blockID),
			unittest.WithExecutionResultID(executionResultID),
			unittest.WithChunk(10),
		)
		_, _ = approvalPL.Add(a)

		// The mempool stores all approvals for the same chunk internally in one data structure.
		// Hence, eviction should lead to _all_ approvals for a single chunk being dropped.
		//  * for this specific test, we use the LRU ejector
		//  * this ejector drops the chunk that was added earliest, i.e. chunk 0
		// Hence, we expect 3 approvals for each of the chunks 1, 2, ..., 9
		// plus one approval for chunk 10
		require.Equal(t, uint(9*3+1), approvalPL.Size())
	})
}

package stdmap

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

var empty []*flow.ExecutionReceipt

func TestPendingReceipts(t *testing.T) {
	t.Parallel()

	headers := &mockstorage.Headers{}
	zeroHeader := &flow.Header{}
	headers.On("ByBlockID", mock.Anything).Return(zeroHeader, nil).Maybe()

	t.Run("get nothing", func(t *testing.T) {
		pool := NewPendingReceipts(headers, 100)

		r := unittest.ExecutionReceiptFixture()
		actual := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
		require.Equal(t, empty, actual)
	})

	// after adding one receipt, should be able to query it back by previous result id
	// after removing, should not be able to query it back.
	t.Run("add remove get", func(t *testing.T) {
		pool := NewPendingReceipts(headers, 100)

		r := unittest.ExecutionReceiptFixture()

		ok := pool.Add(r)
		require.True(t, ok)

		actual := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
		require.Equal(t, []*flow.ExecutionReceipt{r}, actual)

		deleted := pool.Rem(r.ID())
		require.True(t, deleted)

		actual = pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
		require.Equal(t, empty, actual)
	})

	chainedReceipts := func(n int) []*flow.ExecutionReceipt {
		rs := make([]*flow.ExecutionReceipt, n)
		parent := unittest.ExecutionReceiptFixture()
		rs[0] = parent
		for i := 1; i < n; i++ {
			rs[i] = unittest.ExecutionReceiptFixture(func(receipt *flow.ExecutionReceipt) {
				receipt.ExecutionResult.PreviousResultID = parent.ID()
				parent = receipt
			})
		}
		return rs
	}

	t.Run("add 100 remove 100", func(t *testing.T) {
		pool := NewPendingReceipts(headers, 100)

		rs := chainedReceipts(100)
		for i := 0; i < 100; i++ {
			rs[i] = unittest.ExecutionReceiptFixture()
		}

		for i := 0; i < 100; i++ {
			r := rs[i]
			ok := pool.Add(r)
			require.True(t, ok)
		}

		for i := 0; i < 100; i++ {
			r := rs[i]
			actual := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.Equal(t, []*flow.ExecutionReceipt{r}, actual)
		}

		for i := 0; i < 100; i++ {
			r := rs[i]
			ok := pool.Rem(r.ID())
			require.True(t, ok)
		}

		for i := 0; i < 100; i++ {
			r := rs[i]
			actual := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.Equal(t, empty, actual)
		}
	})

	t.Run("add receipts having same previous result id", func(t *testing.T) {
		pool := NewPendingReceipts(headers, 100)

		parent := unittest.ExecutionReceiptFixture()
		parentID := parent.ID()
		rs := make([]*flow.ExecutionReceipt, 100)
		for i := 0; i < 100; i++ {
			rs[i] = unittest.ExecutionReceiptFixture(func(receipt *flow.ExecutionReceipt) {
				// having the same parent
				receipt.ExecutionResult.PreviousResultID = parentID
			})
		}

		for _, r := range rs {
			ok := pool.Add(r)
			require.True(t, ok)
		}

		actual := pool.ByPreviousResultID(parentID)
		require.ElementsMatch(t, rs, actual)
	})

	t.Run("adding too many will eject", func(t *testing.T) {
		pool := NewPendingReceipts(headers, 60)

		rs := chainedReceipts(100)
		for i := 0; i < 100; i++ {
			rs[i] = unittest.ExecutionReceiptFixture()
		}

		for i := 0; i < 100; i++ {
			r := rs[i]
			ok := pool.Add(r)
			require.True(t, ok)
		}

		// adding 100 will cause 40 to be ejected,
		// since there are 60 left and be found
		total := 0
		for i := 0; i < 100; i++ {
			r := rs[i]
			actual := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			if len(actual) > 0 {
				total++
			}
		}
		require.Equal(t, 100, total)

		// since there are 60 left, should remove 60 in total
		total = 0
		for i := 0; i < 100; i++ {
			ok := pool.Rem(rs[i].ID())
			if ok {
				total++
			}
		}
		require.Equal(t, 100, total)
	})

	t.Run("concurrent adding and removing", func(t *testing.T) {
		pool := NewPendingReceipts(headers, 100)

		rs := chainedReceipts(100)
		for i := 0; i < 100; i++ {
			rs[i] = unittest.ExecutionReceiptFixture()
		}

		unittest.Concurrently(100, func(i int) {
			r := rs[i]
			ok := pool.Add(r)
			require.True(t, ok)
		})

		unittest.Concurrently(100, func(i int) {
			r := rs[i]
			actual := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.Equal(t, []*flow.ExecutionReceipt{r}, actual)
		})

		unittest.Concurrently(100, func(i int) {
			r := rs[i]
			ok := pool.Rem(r.ID())
			require.True(t, ok)
		})

		unittest.Concurrently(100, func(i int) {
			r := rs[i]
			actual := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.Equal(t, empty, actual)
		})
	})

	t.Run("pruning", func(t *testing.T) {
		headers := &mockstorage.Headers{}
		pool := NewPendingReceipts(headers, 100)
		executedBlock := unittest.BlockFixture()
		nextExecutedBlock := unittest.BlockWithParentFixture(executedBlock.Header)
		er := unittest.ExecutionResultFixture(unittest.WithBlock(&executedBlock))
		headers.On("ByBlockID", executedBlock.ID()).Return(executedBlock.Header, nil)
		headers.On("ByBlockID", nextExecutedBlock.ID()).Return(nextExecutedBlock.Header, nil)
		ids := make(map[flow.Identifier]struct{})
		for i := 0; i < 10; i++ {
			receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(er))
			pool.Add(receipt)
			ids[receipt.ID()] = struct{}{}
		}

		nextReceipt := unittest.ExecutionReceiptFixture(unittest.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithBlock(nextExecutedBlock))))
		pool.Add(nextReceipt)

		for id := range ids {
			require.True(t, pool.Has(id))
		}

		err := pool.PruneUpToHeight(nextExecutedBlock.Header.Height)
		require.NoError(t, err)

		// these receipts should be pruned
		for id := range ids {
			require.False(t, pool.Has(id))
		}

		// receipt for this block should be still present
		require.True(t, pool.Has(nextReceipt.ID()))
	})
}

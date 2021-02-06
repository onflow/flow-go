package stdmap

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPendingReceipts(t *testing.T) {
	t.Parallel()

	// after adding one receipt, should be able to query it back by previous result id
	// after removing, should not be able to query it back.
	t.Run("add remove get", func(t *testing.T) {
		pool := NewPendingReceipts(100)

		r := unittest.ExecutionReceiptFixture()

		ok := pool.Add(r)
		require.True(t, ok)

		actual, found := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
		require.True(t, found)
		require.Equal(t, r, actual)

		deleted := pool.Rem(r.ID())
		require.True(t, deleted)

		_, found = pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
		require.False(t, found)
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
			fmt.Println(rs[i].ExecutionResult.PreviousResultID)
		}
		return rs
	}

	t.Run("add 100 remove 100", func(t *testing.T) {
		pool := NewPendingReceipts(100)

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
			_, found := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.True(t, found)
		}

		for i := 0; i < 100; i++ {
			r := rs[i]
			ok := pool.Rem(r.ID())
			require.True(t, ok)
		}

		for i := 0; i < 100; i++ {
			r := rs[i]
			_, found := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.False(t, found)
		}
	})

	t.Run("adding too many will eject", func(t *testing.T) {
		pool := NewPendingReceipts(60)

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
			_, found := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			if found {
				total++
			}
		}
		require.Equal(t, 60, total)

		// since there are 60 left, should remove 60 in total
		total = 0
		for i := 0; i < 100; i++ {
			ok := pool.Rem(rs[i].ID())
			if ok {
				total++
			}
		}
		require.Equal(t, 60, total)
	})

	concurrently := func(n int, f func(int)) {
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				f(i)
				wg.Done()
			}(i)
		}
		wg.Wait()
	}

	t.Run("concurrent adding and removing", func(t *testing.T) {
		pool := NewPendingReceipts(100)

		rs := chainedReceipts(100)
		for i := 0; i < 100; i++ {
			rs[i] = unittest.ExecutionReceiptFixture()
		}

		concurrently(100, func(i int) {
			r := rs[i]
			ok := pool.Add(r)
			require.True(t, ok)
		})

		concurrently(100, func(i int) {
			r := rs[i]
			_, found := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.True(t, found)
		})

		concurrently(100, func(i int) {
			r := rs[i]
			ok := pool.Rem(r.ID())
			require.True(t, ok)
		})

		concurrently(100, func(i int) {
			r := rs[i]
			_, found := pool.ByPreviousResultID(r.ExecutionResult.PreviousResultID)
			require.False(t, found)
		})
	})
}

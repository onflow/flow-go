package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDeriveUnknownTransactionStatus tests each scenario when deriving the status of transaction whose block is not known.
func TestDeriveUnknownTransactionStatus(t *testing.T) {
	rootHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(flow.DefaultTransactionExpiry + 1))
	blocks := unittest.ChainBlockFixtureWithRoot(rootHeader, 10)

	lastFullBlockHeight := func(height uint64) *counters.PersistentStrictMonotonicCounter {
		progress := storagemock.NewConsumerProgress(t)
		progress.On("ProcessedIndex").Return(height, nil)
		lastFullBlockHeight, err := counters.NewPersistentStrictMonotonicCounter(progress)
		require.NoError(t, err)
		return lastFullBlockHeight
	}

	refBlock := blocks[0]
	refBlockID := refBlock.ID()

	expiredBlock := unittest.BlockFixture(unittest.Block.WithHeight(refBlock.Height - flow.DefaultTransactionExpiry - 1))
	expiredBlockID := expiredBlock.ID()

	t.Run("tx not expired", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.AllowValidBlocks(t)

		deriver := NewTxStatusDeriver(s.State, lastFullBlockHeight(refBlock.Height))

		status, err := deriver.DeriveUnknownTransactionStatus(refBlockID)
		assert.NoError(t, err)
		assert.Equal(t, flow.TransactionStatusPending, status)
	})

	t.Run("tx expired", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.AllowValidBlocks(t)

		s.HeaderAt(t, expiredBlock.ToHeader())

		deriver := NewTxStatusDeriver(s.State, lastFullBlockHeight(refBlock.Height))

		status, err := deriver.DeriveUnknownTransactionStatus(expiredBlockID)
		assert.NoError(t, err)
		assert.Equal(t, flow.TransactionStatusExpired, status)
	})

	t.Run("tx not indexed", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.AllowValidBlocks(t)

		s.HeaderAt(t, expiredBlock.ToHeader())

		deriver := NewTxStatusDeriver(s.State, lastFullBlockHeight(expiredBlock.Height+1))

		status, err := deriver.DeriveUnknownTransactionStatus(expiredBlockID)
		assert.NoError(t, err)
		assert.Equal(t, flow.TransactionStatusPending, status)
	})

	t.Run("reference block not found", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.AtBlockID(t, refBlockID).
			On("Head").
			Return(nil, storage.ErrNotFound)

		deriver := NewTxStatusDeriver(s.State, lastFullBlockHeight(refBlock.Height))

		status, err := deriver.DeriveUnknownTransactionStatus(refBlockID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNotFound)
		assert.Equal(t, flow.TransactionStatusUnknown, status)
	})

	t.Run("error getting finalized header", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.HeaderAt(t, refBlock.ToHeader())
		s.FinalizedSnapshot.
			On("Head").
			Return(nil, storage.ErrNotFound)

		deriver := NewTxStatusDeriver(s.State, lastFullBlockHeight(refBlock.Height))

		status, err := deriver.DeriveUnknownTransactionStatus(refBlockID)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, storage.ErrNotFound)
		assert.Equal(t, flow.TransactionStatusUnknown, status)
	})
}

// TestDeriveFinalizedTransactionStatus tests each scenario when deriving the status of transaction in a finalized block.
func TestDeriveFinalizedTransactionStatus(t *testing.T) {
	rootHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(flow.DefaultTransactionExpiry + 1))
	blocks := unittest.ChainBlockFixtureWithRoot(rootHeader, 10)

	t.Run("tx not executed", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		deriver := NewTxStatusDeriver(s.State, nil)

		status, err := deriver.DeriveFinalizedTransactionStatus(s.Finalized.Height, false)
		assert.NoError(t, err)
		assert.Equal(t, flow.TransactionStatusFinalized, status)
	})

	t.Run("error getting sealed header", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.SealedSnapshot.On("Head").Return(nil, storage.ErrNotFound)

		deriver := NewTxStatusDeriver(s.State, nil)
		status, err := deriver.DeriveFinalizedTransactionStatus(s.Finalized.Height, true)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, storage.ErrNotFound)
		assert.Equal(t, flow.TransactionStatusUnknown, status)
	})

	t.Run("tx not sealed", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.AllowValidBlocks(t)

		deriver := NewTxStatusDeriver(s.State, nil)
		status, err := deriver.DeriveFinalizedTransactionStatus(s.Finalized.Height, true)
		assert.NoError(t, err)
		assert.Equal(t, flow.TransactionStatusExecuted, status)
	})

	t.Run("tx sealed", func(t *testing.T) {
		s := testutil.NewMockState(t, blocks)
		s.AllowValidBlocks(t)

		deriver := NewTxStatusDeriver(s.State, nil)
		status, err := deriver.DeriveFinalizedTransactionStatus(s.Sealed.Height, true)
		assert.NoError(t, err)
		assert.Equal(t, flow.TransactionStatusSealed, status)
	})
}

// TestIsExpired tests the isExpired function.
func TestIsExpired(t *testing.T) {
	tests := []struct {
		name              string
		refHeight         uint64
		finalizedToHeight uint64
		expectedExpired   bool
		description       string
	}{
		{
			// transaction at same height as comparison should not be expired
			name:              "not expired - same height",
			refHeight:         100,
			finalizedToHeight: 100,
			expectedExpired:   false,
		},
		{
			// transaction with higher reference height should not be expired
			name:              "not expired - finalized height lower",
			refHeight:         100,
			finalizedToHeight: 50,
			expectedExpired:   false,
		},
		{
			// transaction exactly at expiry boundary should not be expired
			name:              "not expired - within expiry window",
			refHeight:         100,
			finalizedToHeight: 100 + flow.DefaultTransactionExpiry,
			expectedExpired:   false,
		},
		{
			// transaction beyond expiry window should be expired
			name:              "expired - beyond expiry window",
			refHeight:         100,
			finalizedToHeight: 100 + flow.DefaultTransactionExpiry + 1,
			expectedExpired:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExpired(tt.refHeight, tt.finalizedToHeight)
			assert.Equal(t, tt.expectedExpired, result)
		})
	}
}

package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	statepkg "github.com/onflow/flow-go/state"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetMaxCollectionSizeForSealingLag tests different sealing lag values and their corresponding to the expected value.
func TestGetMaxCollectionSizeForSealingLag(t *testing.T) {
	testCases := []struct {
		name                   string
		minSealingLag          uint
		maxSealingLag          uint
		halvingInterval        uint
		minCollectionSize      uint
		maxCollectionSize      uint
		sealingLag             uint
		expectedCollectionSize uint
	}{
		{"no-halving", 0, 10, 5, 0, 10, 2, 10},
		{"one-halving", 0, 10, 5, 0, 10, 6, 5},
		{"two-halving", 0, 11, 5, 0, 10, 10, 2},
		{"max-reached", 0, 10, 5, 0, 10, 11, 0},
		{"almost-binary", 300, 600, 299, 0, 10, 599, 5},
		{"binary", 300, 600, 300, 0, 10, 599, 10},
	}

	state := protocol.NewState(t)
	sealedBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(100))
	state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(sealedBlock, nil))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			finalBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(sealedBlock.Height + uint64(tc.sealingLag)))
			state.On("Final").Return(unittest.StateSnapshotForKnownBlock(finalBlock, nil)).Once()
			result, err := GetMaxCollectionSizeForSealingLag(
				state,
				tc.minSealingLag,
				tc.maxSealingLag,
				tc.halvingInterval,
				tc.minCollectionSize,
				tc.maxCollectionSize,
			)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCollectionSize, result)
		})
	}
}

// TestTestGetMaxCollectionSizeForSealingLag_Errors tests error scenarios when retrieving finalized or sealed snapshots fails and the
// error is propagated to the caller.
func TestTestGetMaxCollectionSizeForSealingLag_Errors(t *testing.T) {
	t.Run("finalized-err", func(t *testing.T) {
		state := protocol.NewState(t)
		state.On("Final").Return(unittest.StateSnapshotForUnknownBlock()).Once()
		_, err := GetMaxCollectionSizeForSealingLag(
			state,
			0,
			10,
			5,
			0,
			10,
		)
		assert.ErrorIs(t, err, statepkg.ErrUnknownSnapshotReference)
	})
	t.Run("sealed-err", func(t *testing.T) {
		state := protocol.NewState(t)
		state.On("Final").Return(unittest.StateSnapshotForKnownBlock(unittest.BlockHeaderFixture(), nil)).Once()
		state.On("Sealed").Return(unittest.StateSnapshotForUnknownBlock()).Once()
		_, err := GetMaxCollectionSizeForSealingLag(
			state,
			0,
			10,
			5,
			0,
			10,
		)
		assert.ErrorIs(t, err, statepkg.ErrUnknownSnapshotReference)
	})
}

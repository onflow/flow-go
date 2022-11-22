package delta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type mockedStorage struct {
	GetRegisterFunc      func(key flow.RegisterID) (value flow.RegisterValue, exists bool, err error)
	CommitBlockDeltaFunc func(blockHeight uint64, delta delta.Delta) error
	BootstrapFunc        func(blockHeight uint64, registers []flow.RegisterEntry) error
	BlockHeightFunc      func() (uint64, error)
}

// add methods

func (ms *mockedStorage) GetRegister(key flow.RegisterID) (value flow.RegisterValue, exists bool, err error) {
	return ms.GetRegisterFunc(key)
}

func (ms *mockedStorage) CommitBlockDelta(blockHeight uint64, delta delta.Delta) error {
	return ms.CommitBlockDeltaFunc(blockHeight, delta)

}

func (ms *mockedStorage) Bootstrap(blockHeight uint64, registers []flow.RegisterEntry) error {
	return ms.BootstrapFunc(blockHeight, registers)
}

func (ms *mockedStorage) BlockHeight() (uint64, error) {
	return ms.BlockHeightFunc()
}

func TestOracle(t *testing.T) {

	// (test out of order, test chain of blocks, ...)

	t.Run("HappyPath", func(t *testing.T) {

		blockCounter := uint64(0)
		data := make(map[flow.RegisterID]flow.RegisterValue)
		storage := &mockedStorage{
			GetRegisterFunc: func(key flow.RegisterID) (value flow.RegisterValue, exists bool, err error) {
				value, exists = data[key]
				return
			},
			CommitBlockDeltaFunc: func(blockHeight uint64, delta delta.Delta) error {
				assert.Equal(t, blockHeight, blockCounter+1)
				blockCounter++
				for key, value := range delta.Data {
					data[key] = value
				}
				return nil
			},
			BlockHeightFunc: func() (uint64, error) {
				return blockCounter, nil
			},
		}

		oracle, err := delta.NewOracle(storage)
		require.NoError(t, err)

		block1header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(1))
		block2header := unittest.BlockHeaderWithParentFixture(block1header)
		view1, err := oracle.NewBlockView(block1header.ID(), block1header)
		require.NoError(t, err)

		val := []byte{1, 2}
		err = view1.Set("owner", "key1", val)
		require.NoError(t, err)

		retValue, err := view1.Get("owner", "key1")
		require.NoError(t, err)
		require.Equal(t, val, retValue)

		view2, err := oracle.NewBlockView(block2header.ID(), block2header)
		require.NoError(t, err)

		retValue, err = view2.Get("owner", "key1")
		require.NoError(t, err)
		require.Equal(t, val, retValue)

		h, err := oracle.LastSealedBlockHeight()
		require.NoError(t, err)
		require.Equal(t, uint64(0), h)

		require.Equal(t, 2, oracle.BlocksInFlight())
		require.Equal(t, 2, oracle.HeightsInFlight())

		err = oracle.BlockIsSealed(block1header.ID(), block1header)
		require.NoError(t, err)
		h, err = oracle.LastSealedBlockHeight()
		require.NoError(t, err)
		require.Equal(t, uint64(1), h)

		// ReadFunc get should have been updated for children (block2 view)
		retValue, err = view2.Get("owner", "key1")
		require.NoError(t, err)
		require.Equal(t, val, retValue)

		require.Equal(t, 1, oracle.BlocksInFlight())
		require.Equal(t, 1, oracle.HeightsInFlight())
	})

}

package types_test

import (
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestBlockHashList(t *testing.T) {

	capacity := 5
	bhl := types.NewBlockHashList(capacity)
	require.True(t, bhl.IsEmpty())

	require.Equal(t, gethCommon.Hash{}, bhl.LastAddedBlockHash())

	found, h := bhl.BlockHashByHeight(0)
	require.False(t, found)
	require.Equal(t, gethCommon.Hash{}, h)

	// first full range
	for i := 0; i < capacity; i++ {
		err := bhl.Push(uint64(i), gethCommon.Hash{byte(i)})
		require.NoError(t, err)
		require.Equal(t, uint64(0), bhl.MinAvailableHeight())
		require.Equal(t, uint64(i), bhl.MaxAvailableHeight())
	}
	for i := 0; i < capacity; i++ {
		found, h := bhl.BlockHashByHeight(uint64(i))
		require.True(t, found)
		require.Equal(t, gethCommon.Hash{byte(i)}, h)
	}
	require.Equal(t, gethCommon.Hash{byte(capacity - 1)}, bhl.LastAddedBlockHash())

	// over border range
	for i := capacity; i < capacity+3; i++ {
		err := bhl.Push(uint64(i), gethCommon.Hash{byte(i)})
		require.NoError(t, err)
		require.Equal(t, uint64(i-capacity+1), bhl.MinAvailableHeight())
		require.Equal(t, uint64(i), bhl.MaxAvailableHeight())
	}
	for i := 0; i < capacity-2; i++ {
		found, _ := bhl.BlockHashByHeight(uint64(i))
		require.False(t, found)
	}
	for i := capacity - 2; i < capacity+3; i++ {
		found, h := bhl.BlockHashByHeight(uint64(i))
		require.True(t, found)
		require.Equal(t, gethCommon.Hash{byte(i)}, h)
	}
	require.Equal(t, gethCommon.Hash{byte(capacity + 2)}, bhl.LastAddedBlockHash())

	encoded := bhl.Encode()
	bhl2, err := types.NewBlockHashListFromEncoded(encoded)
	require.NoError(t, err)
	require.Equal(t, bhl, bhl2)
}

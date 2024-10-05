package blocks_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestBlocks(t *testing.T) {
	storage := testutils.GetSimpleValueStore()
	chainID := flow.Emulator.Chain().ChainID()
	rootAddr := flow.Address{1, 2, 3, 4}
	blks, err := blocks.NewBlocks(chainID, rootAddr, storage)
	require.NoError(t, err)

	// no insertion - genesis block
	bm, err := blks.LatestBlock()
	require.NoError(t, err)
	genesis := types.GenesisBlock(chainID)
	require.Equal(t, genesis.Height, bm.Height)
	require.Equal(t, genesis.Timestamp, bm.Timestamp)
	require.Equal(t, genesis.PrevRandao, bm.Random)

	h, err := blks.BlockHash(0)
	require.NoError(t, err)
	expectedHash, err := genesis.Hash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, h)

	// push next block
	height := uint64(1)
	timestamp := uint64(2)
	random := testutils.RandomCommonHash(t)
	hash := testutils.RandomCommonHash(t)

	err = blks.PushBlockMeta(blocks.NewMeta(height, timestamp, random))
	require.NoError(t, err)
	err = blks.PushBlockHash(height, hash)
	require.NoError(t, err)

	// check values
	h, err = blks.BlockHash(1)
	require.NoError(t, err)
	require.Equal(t, hash, h)
	bm, err = blks.LatestBlock()
	require.NoError(t, err)
	require.Equal(t, height, bm.Height)
	require.Equal(t, timestamp, bm.Timestamp)
	require.Equal(t, random, bm.Random)
}

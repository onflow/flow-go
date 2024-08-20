package types

import (
	"math/big"
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func Test_GenesisBlock(t *testing.T) {
	testnetGenesis := GenesisBlock(flow.Testnet)
	require.Equal(t, testnetGenesis.Timestamp, GenesisTimestamp(flow.Testnet))
	testnetGenesisHash := GenesisBlockHash(flow.Testnet)
	h, err := testnetGenesis.Hash()
	require.NoError(t, err)
	require.Equal(t, h, testnetGenesisHash)

	mainnetGenesis := GenesisBlock(flow.Mainnet)
	require.Equal(t, mainnetGenesis.Timestamp, GenesisTimestamp(flow.Mainnet))
	mainnetGenesisHash := GenesisBlockHash(flow.Mainnet)
	h, err = mainnetGenesis.Hash()
	require.NoError(t, err)
	require.Equal(t, h, mainnetGenesisHash)

	assert.NotEqual(t, testnetGenesisHash, mainnetGenesisHash)
}

func Test_BlockHash(t *testing.T) {
	b := Block{
		ParentBlockHash:     gethCommon.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Height:              1,
		TotalSupply:         big.NewInt(1000),
		ReceiptRoot:         gethCommon.Hash{0x2, 0x3, 0x4},
		TotalGasUsed:        135,
		TransactionHashRoot: gethCommon.Hash{0x5, 0x6, 0x7},
	}

	h1, err := b.Hash()
	require.NoError(t, err)

	b.Height = 2

	h2, err := b.Hash()
	require.NoError(t, err)

	// hashes should not equal if any data is changed
	assert.NotEqual(t, h1, h2)
}

func Test_BlockProposal(t *testing.T) {
	bp := NewBlockProposal(gethCommon.Hash{1}, 1, 0, nil)

	bp.AppendTransaction(nil)
	require.Empty(t, bp.TxHashes)
	require.Equal(t, uint64(0), bp.TotalGasUsed)

	bp.PopulateRoots()
	require.Equal(t, gethTypes.EmptyReceiptsHash, bp.ReceiptRoot)
	require.Equal(t, gethTypes.EmptyRootHash, bp.TransactionHashRoot)

	res := &Result{
		TxHash:            gethCommon.Hash{2},
		GasConsumed:       10,
		CumulativeGasUsed: 20,
	}
	bp.AppendTransaction(res)
	require.Equal(t, res.TxHash, bp.TxHashes[0])
	require.Equal(t, res.CumulativeGasUsed, bp.TotalGasUsed)
	require.Equal(t, *res.LightReceipt(), bp.Receipts[0])

	bp.PopulateRoots()
	require.NotEqual(t, gethTypes.EmptyReceiptsHash, bp.ReceiptRoot)
}

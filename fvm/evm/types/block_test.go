package types

import (
	"math/big"
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethRLP "github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_BlockHash(t *testing.T) {
	b := Block{
		ParentBlockHash: gethCommon.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Height:          1,
		TotalSupply:     big.NewInt(1000),
		ReceiptRoot:     gethCommon.Hash{0x2, 0x3, 0x4},
		TotalGasUsed:    135,
		TransactionHashes: []gethCommon.Hash{
			gethCommon.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		},
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
	require.Empty(t, bp.TransactionHashes)
	require.Equal(t, uint64(0), bp.TotalGasUsed)

	bp.PopulateReceiptsHash()
	require.Equal(t, gethTypes.EmptyReceiptsHash, bp.ReceiptRoot)

	res := &Result{
		TxHash:      gethCommon.Hash{2},
		GasConsumed: 10,
	}
	bp.AppendTransaction(res)
	require.Equal(t, res.TxHash, bp.TransactionHashes[0])
	require.Equal(t, res.GasConsumed, bp.TotalGasUsed)
	require.Equal(t, res.Receipt(), bp.Receipts[0])

	bp.PopulateReceiptsHash()
	require.NotEqual(t, gethTypes.EmptyReceiptsHash, bp.ReceiptRoot)
}

func Test_DecodeBlocks(t *testing.T) {
	bv0 := blockV0{
		ParentBlockHash: GenesisBlockHash,
		Height:          1,
		UUIDIndex:       2,
		TotalSupply:     3,
		StateRoot:       gethCommon.Hash{0x01},
		ReceiptRoot:     gethCommon.Hash{0x02},
	}
	b0, err := gethRLP.EncodeToBytes(bv0)
	require.NoError(t, err)

	b := decodeBlockBreakingChanges(b0)

	require.Equal(t, b.TotalSupply.Uint64(), bv0.TotalSupply)
	require.Equal(t, b.Height, bv0.Height)
	require.Equal(t, b.ParentBlockHash, bv0.ParentBlockHash)
	require.Empty(t, b.Timestamp)
	require.Empty(t, b.TotalGasUsed)

	bv1 := blockV1{
		ParentBlockHash:   GenesisBlockHash,
		Height:            1,
		UUIDIndex:         2,
		TotalSupply:       3,
		StateRoot:         gethCommon.Hash{0x01},
		ReceiptRoot:       gethCommon.Hash{0x02},
		TransactionHashes: []gethCommon.Hash{{0x04}},
	}

	b1, err := gethRLP.EncodeToBytes(bv1)
	require.NoError(t, err)

	b = decodeBlockBreakingChanges(b1)

	require.Equal(t, b.TotalSupply.Uint64(), bv1.TotalSupply)
	require.Equal(t, b.Height, bv1.Height)
	require.Equal(t, b.ParentBlockHash, bv1.ParentBlockHash)
	require.Equal(t, b.TransactionHashes, bv1.TransactionHashes)
	require.Empty(t, b.Timestamp)
	require.Empty(t, b.TotalGasUsed)

	bv2 := blockV2{
		ParentBlockHash:   GenesisBlockHash,
		Height:            1,
		TotalSupply:       2,
		StateRoot:         gethCommon.Hash{0x01},
		ReceiptRoot:       gethCommon.Hash{0x02},
		TransactionHashes: []gethCommon.Hash{{0x04}},
	}

	b2, err := gethRLP.EncodeToBytes(bv2)
	require.NoError(t, err)

	b = decodeBlockBreakingChanges(b2)

	require.Equal(t, b.TotalSupply.Uint64(), bv2.TotalSupply)
	require.Equal(t, b.Height, bv2.Height)
	require.Equal(t, b.ParentBlockHash, bv2.ParentBlockHash)
	require.Equal(t, b.TransactionHashes, bv2.TransactionHashes)
	require.Empty(t, b.Timestamp)
	require.Empty(t, b.TotalGasUsed)

	bv3 := blockV3{
		ParentBlockHash:   GenesisBlockHash,
		Height:            1,
		TotalSupply:       2,
		ReceiptRoot:       gethCommon.Hash{0x02},
		TransactionHashes: []gethCommon.Hash{{0x04}},
	}

	b3, err := gethRLP.EncodeToBytes(bv3)
	require.NoError(t, err)

	b = decodeBlockBreakingChanges(b3)

	require.Equal(t, b.TotalSupply.Uint64(), bv3.TotalSupply)
	require.Equal(t, b.Height, bv3.Height)
	require.Equal(t, b.ParentBlockHash, bv3.ParentBlockHash)
	require.Equal(t, b.TransactionHashes, bv3.TransactionHashes)
	require.Empty(t, b.Timestamp)
	require.Empty(t, b.TotalGasUsed)

	bv4 := blockV4{
		ParentBlockHash:   GenesisBlockHash,
		Height:            1,
		TotalSupply:       big.NewInt(4),
		ReceiptRoot:       gethCommon.Hash{0x02},
		TransactionHashes: []gethCommon.Hash{{0x04}},
	}

	b4, err := gethRLP.EncodeToBytes(bv4)
	require.NoError(t, err)

	b = decodeBlockBreakingChanges(b4)

	require.Equal(t, b.TotalSupply, bv4.TotalSupply)
	require.Equal(t, b.Height, bv4.Height)
	require.Equal(t, b.ParentBlockHash, bv4.ParentBlockHash)
	require.Equal(t, b.TransactionHashes, bv4.TransactionHashes)
	require.Empty(t, b.Timestamp)
	require.Empty(t, b.TotalGasUsed)

	bv5 := blockV5{
		ParentBlockHash:   GenesisBlockHash,
		Height:            1,
		TotalSupply:       big.NewInt(2),
		ReceiptRoot:       gethCommon.Hash{0x02},
		TransactionHashes: []gethCommon.Hash{{0x04}},
		Timestamp:         100,
	}

	b5, err := gethRLP.EncodeToBytes(bv5)
	require.NoError(t, err)

	b = decodeBlockBreakingChanges(b5)

	require.Equal(t, b.Timestamp, bv5.Timestamp)
	require.Equal(t, b.TotalSupply, bv5.TotalSupply)
	require.Equal(t, b.Height, bv5.Height)
	require.Equal(t, b.ParentBlockHash, bv5.ParentBlockHash)
	require.Equal(t, b.TransactionHashes, bv5.TransactionHashes)
	require.Empty(t, b.TotalGasUsed)
}

package types

import (
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_BlockHash(t *testing.T) {
	b := Block{
		ParentBlockHash: gethCommon.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Height:          1,
		TotalSupply:     big.NewInt(1000),
		ReceiptRoot:     gethCommon.Hash{0x2, 0x3, 0x4},
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

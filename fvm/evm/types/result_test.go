package types_test

import (
	"testing"

	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethTrie "github.com/onflow/go-ethereum/trie"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestLightReceipts(t *testing.T) {
	resCount := 10
	receipts := make(gethTypes.Receipts, resCount)
	reconstructedReceipts := make(gethTypes.Receipts, resCount)
	var totalGas uint64
	for i := 0; i < resCount; i++ {
		res := testutils.RandomResultFixture(t)
		receipts[i] = res.Receipt(totalGas)
		reconstructedReceipts[i] = res.LightReceipt(totalGas).ToReceipt()
		totalGas += res.GasConsumed
	}
	// the root hash for reconstructed receipts should match the receipts
	root1 := gethTypes.DeriveSha(receipts, gethTrie.NewStackTrie(nil))
	root2 := gethTypes.DeriveSha(reconstructedReceipts, gethTrie.NewStackTrie(nil))
	require.Equal(t, root1, root2)
}

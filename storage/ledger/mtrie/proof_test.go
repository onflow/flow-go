package mtrie_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
)

func TestBatchProofEncoderDecoder(t *testing.T) {
	trieHeight := 9
	fStore, err := mtrie.NewMForest(trieHeight, "", 5)
	require.NoError(t, err)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(1)})
	v1 := []byte{'A'}
	keys := [][]byte{k1}
	values := [][]byte{v1}
	rootHash, err = fStore.Update(keys, values, rootHash)
	require.NoError(t, err)
	batchProof, err := fStore.Proofs(keys, rootHash)
	require.NoError(t, err)

	p, err := mtrie.DecodeBatchProof(mtrie.EncodeBatchProof(batchProof))
	require.NoError(t, err)
	require.Equal(t, p, batchProof, "Proof encoder and/or decoder has an issue")

}

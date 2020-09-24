package proof_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/ledger/mtrie"
	"github.com/onflow/flow-go/storage/ledger/mtrie/proof"
)

func TestBatchProofEncoderDecoder(t *testing.T) {
	keyByteSize := 1 // key size of 8 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	fStore, err := mtrie.NewMForest(keyByteSize, dir, 5, metricsCollector, nil)
	require.NoError(t, err)

	k1 := []byte([]uint8{uint8(1)})
	v1 := []byte{'A'}
	keys := [][]byte{k1}
	values := [][]byte{v1}
	testTrie, err := fStore.Update(fStore.GetEmptyRootHash(), keys, values)
	require.NoError(t, err)
	batchProof, err := fStore.Proofs(testTrie.RootHash(), keys)
	require.NoError(t, err)

	encProf, _ := proof.EncodeBatchProof(batchProof)
	p, err := proof.DecodeBatchProof(encProf)
	require.NoError(t, err)
	require.Equal(t, p, batchProof, "Proof encoder and/or decoder has an issue")
}

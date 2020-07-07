package proof_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/proof"
)

func TestBatchProofEncoderDecoder(t *testing.T) {
	pathByteSize := 1 // key size of 8 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	fStore, err := mtrie.NewMForest(pathByteSize, dir, 5, metricsCollector, nil)
	require.NoError(t, err)

	p1 := []byte([]uint8{uint8(1)})
	k1 := []byte{'k'}
	v1 := []byte{'v'}
	paths := [][]byte{p1}
	keys := [][]byte{k1}
	values := [][]byte{v1}
	testTrie, err := fStore.Update(fStore.GetEmptyRootHash(), paths, keys, values)
	require.NoError(t, err)
	batchProof, err := fStore.Proofs(testTrie.RootHash(), paths)
	require.NoError(t, err)

	encProf, _ := proof.EncodeBatchProof(batchProof)
	p, err := proof.DecodeBatchProof(encProf)
	require.NoError(t, err)
	require.Equal(t, p, batchProof, "Proof encoder and/or decoder has an issue")
}

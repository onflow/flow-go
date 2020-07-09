package ledger_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/outright/mtrie"
	"github.com/dapperlabs/flow-go/ledger/outright/mtrie/proof"
	"github.com/dapperlabs/flow-go/module/metrics"
)

// TODO RAMTIN HERE
func TestBatchProofEncoderDecoder(t *testing.T) {
	pathByteSize := 1 // key size of 8 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	fStore, err := mtrie.NewMForest(pathByteSize, dir, 5, metricsCollector, nil)
	require.NoError(t, err)

	p1 := ledger.Path([]byte{'p'})
	v1 := ledger.Payload{Key: ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0,
		Value: []byte{'k'}}}},
		Value: ledger.Value([]byte{'v'})}

	testTrie, err := fStore.Update(fStore.GetEmptyRootHash(), []ledger.Path{p1}, []ledger.Payload{v1})
	require.NoError(t, err)
	batchProof, err := fStore.Proofs(testTrie.RootHash(), []ledger.Path{p1})
	require.NoError(t, err)

	encProf, _ := proof.EncodeBatchProof(batchProof)
	p, err := proof.DecodeBatchProof(encProf)
	require.NoError(t, err)
	require.Equal(t, p, batchProof, "Proof encoder and/or decoder has an issue")
}

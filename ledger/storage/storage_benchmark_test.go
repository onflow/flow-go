package storage_test

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func RunWithPayloadStorage(t testing.TB, runner func(ps ledger.PayloadStorage)) {
	unittest.RunWithTempDir(t, func(dir string) {
		pebbleOptions := storage.PebleStorageOptions{
			Options: &pebble.Options{},
			Dirname: dir,
		}
		store, err := storage.NewPebbleStorage(pebbleOptions)
		require.NoError(t, err)

		ps := storage.NewPayloadStorage(store)

		runner(ps)

		store.Close()

	})
}

func BenchmarkTestPayloadStorage_Add(b *testing.B) {
	RunWithPayloadStorage(b, func(ps ledger.PayloadStorage) {
		// Create some test data
		nBatchSize := 10

		// Run the benchmark for the Add function
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			batch := GenerateRandomBatch(nBatchSize)
			err := ps.Add(batch)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
	})
}

func GenerateRandomBatch(nBatchSize int) []ledger.LeafNode {
	batch := make([]ledger.LeafNode, nBatchSize)
	for j := 0; j < nBatchSize; j++ {
		batch[j] = RandomLeafNode()
	}

	return batch
}

func RandomLeafNode() ledger.LeafNode {
	var path ledger.Path
	copy(path[:], unittest.RandomBytes(32))
	var hash hash.Hash
	copy(hash[:], unittest.RandomBytes(32))
	minPayloadByteSize := 2 << 15 // 64  KB
	maxPayloadByteSize := 2 << 16 // 128 KB
	payload := testutils.RandomPayload(minPayloadByteSize, maxPayloadByteSize)

	return ledger.LeafNode{
		Hash:    hash,
		Path:    path,
		Payload: *payload,
	}
}

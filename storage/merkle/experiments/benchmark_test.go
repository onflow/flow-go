package experiments

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/merkle"
	"github.com/onflow/flow-go/utils/debug"
	"github.com/onflow/flow-go/utils/unittest"
)

func BenchmarkPerformanceForTxDeduplicationNorthStar(b *testing.B) {
	b.Skip("no need to regularly benchmark the north start case")
	benchmarkBroccoli(600, 1000, 1000, b)
}

func BenchmarkPerformanceForTxDeduplicationCommonCase(b *testing.B) {
	const (
		numberOfBlocksInFlight            = 600 // based on expiry
		numberOfCollections               = 10  //
		numberOfTransactionsPerCollection = 160 //
	)
	benchmarkBroccoli(600, 10, 160, b)
}

const pathLen = 64 // (32 for ref size, 32 )

// benchmarkBroccoli benchmarks the performance of merkle tree when used for transaction deduplication
func benchmarkBroccoli(numOfBlocks, numberOfCollections, numOfTxsPerCollection int, b *testing.B) {

	rand.Seed(time.Now().UnixNano())

	tree, err := merkle.NewTree(pathLen)
	require.NoError(b, err)

	memAllocBefore := debug.GetHeapAllocsBytes()
	var firstBlockRef flow.Identifier
	for i := 0; i < numOfBlocks; i++ {
		// generate random block reference IDs
		blockRef := unittest.IdentifierFixture()
		// capturing first block ref for the next step
		if i == 0 {
			firstBlockRef = blockRef
		}
		for j := 0; j < numberOfCollections*numOfTxsPerCollection; j++ {
			// generate random payload hashes
			payloadHash := unittest.IdentifierFixture()
			path := constructPath(blockRef, payloadHash)
			_, err = tree.Put(path, payloadHash[:])
			require.NoError(b, err)
		}
	}
	memAllocAfter := debug.GetHeapAllocsBytes()

	b.ReportMetric(float64(tree.MaxDepthTouched()), "max_depth")
	b.ReportMetric(float64(memAllocAfter)-float64(memAllocBefore), "memory_usage_of_trie_without_cached_hashes_in_bytes")

	tree.MakeItReadOnly()
	start := time.Now()
	_ = tree.Hash()
	b.ReportMetric(float64(time.Since(start).Milliseconds()), "root_hash_generation_time_without_cached_hash_values_in_ms")

	memAllocAfter = debug.GetHeapAllocsBytes()
	b.ReportMetric(float64(memAllocAfter)-float64(memAllocBefore), "memory_usage_of_trie_with_cached_hashes_in_bytes")

	// look up transactions to make sure no transaction is included
	start = time.Now()
	for i := 0; i < numOfTxsPerCollection; i++ {
		// generate random payload hashes
		payloadHash := unittest.IdentifierFixture()
		// block ref is used to be more realistic
		path := constructPath(firstBlockRef, payloadHash)
		_, found := tree.Get(path)
		require.False(b, found)
	}
	b.ReportMetric(float64(time.Since(start).Milliseconds()), "collection_deduplication_time_in_ms")

	// capture proof data

	// rest the tree
	tree, err = merkle.NewTree(pathLen)
	tree.MakeItReadOnly()
	require.NoError(b, err)

	pathsForProof := make([][]byte, 0, numOfTxsPerCollection)
	for i := 0; i < numOfBlocks; i++ {
		blockRef := unittest.IdentifierFixture()
		for j := 0; j < numberOfCollections; j++ {
			for h := 0; h < numOfTxsPerCollection; h++ {
				payloadHash := unittest.IdentifierFixture()
				path := constructPath(blockRef, payloadHash)
				// only for the first batch collect the paths
				if i == 0 && j == 0 {
					pathsForProof = append(pathsForProof, path)
				}
				_, err = tree.Put(path, payloadHash[:])
				require.NoError(b, err)
			}
		}
	}

	proofSizeWithoutOptimization := 0
	start = time.Now()
	for _, path := range pathsForProof {
		proof, found := tree.Prove(path)
		require.True(b, found)
		proofSizeWithoutOptimization += size(proof)
	}
	b.ReportMetric(float64(time.Since(start).Milliseconds()), "proof_generation_time_without_batch_proof_in_ms")
	b.ReportMetric(float64(proofSizeWithoutOptimization), "proof_size_in_bytes_for_a_collection")

}

func constructPath(blockRef, payloadHash flow.Identifier) []byte {
	path := make([]byte, pathLen)
	copy(path[:32], blockRef[:])
	copy(path[32:64], payloadHash[:])
	return path
}

func size(p *merkle.Proof) int {
	size := 0
	for _, sib := range p.SiblingHashes {
		size += len(sib)
	}
	return size + len(p.Key) + len(p.Value) + len(p.InterimNodeTypes) + len(p.ShortPathLengths)*2
}

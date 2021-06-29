package approvals

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// TestAggregatedSignatures_PutSignature tests putting signature with different indexes
func TestAggregatedSignatures_PutSignature(t *testing.T) {
	sigs := NewAggregatedSignatures(10)
	n := sigs.PutSignature(0, flow.AggregatedSignature{})
	require.Equal(t, uint64(1), n)
	require.NotEmpty(t, sigs.numberOfChunks)

	// this shouldn't add a signature since index is larger than number of chunks
	n = sigs.PutSignature(sigs.numberOfChunks, flow.AggregatedSignature{})
	require.Equal(t, uint64(1), n)
	require.Equal(t, 1, len(sigs.signatures))
}

// TestAggregatedSignatures_Collect tests that collecting over full signatures and partial signatures behaves as expected
func TestAggregatedSignatures_Collect(t *testing.T) {
	chunks := uint64(10)
	sigs := NewAggregatedSignatures(chunks)
	sig := flow.AggregatedSignature{}
	sigs.PutSignature(5, sig)

	// collecting over signatures with missing chunks results in empty array
	require.Empty(t, sigs.Collect())
	for index := uint64(0); index < chunks; index++ {
		sigs.PutSignature(index, sig)
	}
	require.Len(t, sigs.Collect(), int(chunks))
}

// TestAggregatedSignatures_HasSignature tests that after putting a signature we can get it
func TestAggregatedSignatures_HasSignature(t *testing.T) {
	sigs := NewAggregatedSignatures(10)
	index := uint64(5)
	sigs.PutSignature(index, flow.AggregatedSignature{})
	require.True(t, sigs.HasSignature(index))
	require.False(t, sigs.HasSignature(0))
}

// TestAggregatedSignatures_ChunksWithoutAggregatedSignature tests that we can retrieve all chunks with missing signatures
func TestAggregatedSignatures_ChunksWithoutAggregatedSignature(t *testing.T) {
	numberOfChunks := uint64(10)
	sigs := NewAggregatedSignatures(numberOfChunks)
	sigs.PutSignature(0, flow.AggregatedSignature{})
	chunks := sigs.ChunksWithoutAggregatedSignature()
	require.Len(t, chunks, int(numberOfChunks)-1)

	expectedChunks := make([]uint64, 0, numberOfChunks)
	for i := uint64(1); i < numberOfChunks; i++ {
		expectedChunks = append(expectedChunks, i)
	}
	require.ElementsMatch(t, expectedChunks, chunks)
}

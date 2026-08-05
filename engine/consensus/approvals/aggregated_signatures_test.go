package approvals

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAggregatedSignatures_NoChunks verifies that NewAggregatedSignatures
// errors when initialized with 0 chunks. This is compatible with Flow's
// architecture, because each block contains at least one chunk (the system chunk).
func TestAggregatedSignatures_NoChunks(t *testing.T) {
	_, err := NewAggregatedSignatures(0)
	require.Error(t, err)
}

// TestAggregatedSignatures_PutSignature tests putting signature with different indexes
func TestAggregatedSignatures_PutSignature(t *testing.T) {
	// create NewAggregatedSignatures for block with chunk indices 0, 1, ... , 9
	sigs, err := NewAggregatedSignatures(10)
	require.NoError(t, err)
	require.Empty(t, sigs.signatures)

	// add signature for chunk with index 3
	n, err := sigs.PutSignature(3, flow.AggregatedSignature{})
	require.NoError(t, err)
	require.Equal(t, uint64(1), n)
	require.Len(t, sigs.signatures, 1)

	// attempt to add signature for chunk with index 10 should error
	n, err = sigs.PutSignature(sigs.numberOfChunks, flow.AggregatedSignature{})
	require.Error(t, err)
	require.Equal(t, uint64(1), n)
}

// TestAggregatedSignatures_Repeated_PutSignature tests that repeated calls to
// PutSignature for the same chunk index are no-ops except for the first one.
func TestAggregatedSignatures_Repeated_PutSignature(t *testing.T) {
	// create NewAggregatedSignatures for block with chunk indices 0, 1, ... , 9
	sigs, err := NewAggregatedSignatures(10)
	require.NoError(t, err)
	require.Empty(t, sigs.signatures)

	// add signature for chunk with index 3
	as1 := flow.AggregatedSignature{
		VerifierSignatures: unittest.SignaturesFixture(22),
		SignerIDs:          unittest.IdentifierListFixture(22),
	}
	n, err := sigs.PutSignature(3, as1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), n)

	// add _different_ sig for chunk index 3 (should be no-op)
	as2 := flow.AggregatedSignature{
		VerifierSignatures: unittest.SignaturesFixture(2),
		SignerIDs:          unittest.IdentifierListFixture(2),
	}
	n, err = sigs.PutSignature(3, as2)
	require.NoError(t, err)
	require.Equal(t, uint64(1), n)

	aggSigs := sigs.Collect()
	for idx, s := range aggSigs {
		if idx == 3 {
			require.Equal(t, 22, s.CardinalitySignerSet())
		} else {
			require.Equal(t, 0, s.CardinalitySignerSet())
		}
	}
}

// TestAggregatedSignatures_Repeated_Signer tests that repeated calls to
// PutSignature for the same chunk index are no-ops except for the first one.
func TestAggregatedSignatures_Repeated_Signer(t *testing.T) {
	// create NewAggregatedSignatures for block with chunk indices 0, 1, ... , 9
	sigs, err := NewAggregatedSignatures(10)
	require.NoError(t, err)
	require.Empty(t, sigs.signatures)

	// add signature for chunk with index 3
	as1 := flow.AggregatedSignature{
		VerifierSignatures: unittest.SignaturesFixture(22),
		SignerIDs:          unittest.IdentifierListFixture(22),
	}
	n, err := sigs.PutSignature(3, as1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), n)

	// add _different_ sig for chunk index 3 (should be no-op)
	as2 := flow.AggregatedSignature{
		VerifierSignatures: unittest.SignaturesFixture(2),
		SignerIDs:          unittest.IdentifierListFixture(2),
	}
	n, err = sigs.PutSignature(3, as2)
	require.NoError(t, err)
	require.Equal(t, uint64(1), n)

	aggSigs := sigs.Collect()
	for idx, s := range aggSigs {
		if idx == 3 {
			require.Equal(t, 22, s.CardinalitySignerSet())
		} else {
			require.Equal(t, 0, s.CardinalitySignerSet())
		}
	}
}

// TestAggregatedSignatures_PutSignature_Sequence tests PutSignature for a full sequence
func TestAggregatedSignatures_PutSignature_Sequence(t *testing.T) {
	chunks := uint64(10)
	sigs, err := NewAggregatedSignatures(chunks)
	require.NoError(t, err)

	for index := uint64(0); index < chunks; index++ {
		n, err := sigs.PutSignature(index, flow.AggregatedSignature{})
		require.NoError(t, err)
		require.Equal(t, n, index+1)
	}
}

// TestAggregatedSignatures_Collect tests that collecting over full signatures and partial signatures behaves as expected
func TestAggregatedSignatures_Collect(t *testing.T) {
	chunks := uint64(10)
	sigs, err := NewAggregatedSignatures(chunks)
	require.NoError(t, err)
	sig := flow.AggregatedSignature{}
	_, err = sigs.PutSignature(5, sig)
	require.NoError(t, err)

	// collecting over signatures with missing chunks results in empty array
	require.Len(t, sigs.Collect(), int(chunks))
	for index := uint64(0); index < chunks; index++ {
		_, err := sigs.PutSignature(index, sig)
		require.NoError(t, err)
		require.Len(t, sigs.Collect(), int(chunks))
	}
}

// TestAggregatedSignatures_HasSignature tests that after putting a signature we can get it
func TestAggregatedSignatures_HasSignature(t *testing.T) {
	sigs, err := NewAggregatedSignatures(10)
	require.NoError(t, err)
	index := uint64(5)
	_, err = sigs.PutSignature(index, flow.AggregatedSignature{})
	require.NoError(t, err)
	require.True(t, sigs.HasSignature(index))
	require.False(t, sigs.HasSignature(0))
}

// TestAggregatedSignatures_ChunksWithoutAggregatedSignature tests that we can retrieve all chunks with missing signatures
func TestAggregatedSignatures_ChunksWithoutAggregatedSignature(t *testing.T) {
	numberOfChunks := uint64(10)
	sigs, err := NewAggregatedSignatures(numberOfChunks)
	require.NoError(t, err)
	_, err = sigs.PutSignature(0, flow.AggregatedSignature{})
	require.NoError(t, err)
	chunks := sigs.ChunksWithoutAggregatedSignature()
	require.Len(t, chunks, int(numberOfChunks)-1)

	expectedChunks := make([]uint64, 0, numberOfChunks)
	for i := uint64(1); i < numberOfChunks; i++ {
		expectedChunks = append(expectedChunks, i)
	}
	require.ElementsMatch(t, expectedChunks, chunks)
}

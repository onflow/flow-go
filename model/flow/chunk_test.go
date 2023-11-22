package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkList_ByIndex evaluates reliability of ByIndex method against within and
// out of range indices
func TestChunkList_ByIndex(t *testing.T) {
	// creates a chunk list with the size of 10
	var chunkList flow.ChunkList = make([]*flow.Chunk, 10)

	// an out of index chunk by index
	_, ok := chunkList.ByIndex(11)
	require.False(t, ok)

	// a within range chunk by index
	_, ok = chunkList.ByIndex(1)
	require.True(t, ok)
}

// TestDistinctChunkIDs_EmptyChunks evaluates that two empty chunks
// with the distinct block ids would have distinct chunk ids.
func TestDistinctChunkIDs_EmptyChunks(t *testing.T) {
	// generates two random block ids and requires them
	// being distinct
	blockIdA := unittest.IdentifierFixture()
	blockIdB := unittest.IdentifierFixture()
	require.NotEqual(t, blockIdA, blockIdB)

	// generates a chunk associated with each block id
	chunkA := &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			BlockID: blockIdA,
		},
	}

	chunkB := &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			BlockID: blockIdB,
		},
	}

	require.NotEqual(t, chunkA.ID(), chunkB.ID())
}

// TestDistinctChunkIDs_FullChunks evaluates that two full chunks
// with completely identical fields but distinct block ids have
// distinct chunk ids.
func TestDistinctChunkIDs_FullChunks(t *testing.T) {
	// generates two random block ids and requires them
	// being distinct
	blockIdA := unittest.IdentifierFixture()
	blockIdB := unittest.IdentifierFixture()
	require.NotEqual(t, blockIdA, blockIdB)

	// generates a chunk associated with blockA
	chunkA := unittest.ChunkFixture(blockIdA, 42)

	// generates a deep copy of chunkA in chunkB
	chunkB := *chunkA

	// since chunkB is a deep copy of chunkA their
	// chunk ids should be the same
	require.Equal(t, chunkA.ID(), chunkB.ID())

	// changes block id in chunkB
	chunkB.BlockID = blockIdB

	// chunks with distinct block ids should have distinct chunk ids
	require.NotEqual(t, chunkA.ID(), chunkB.ID())
}

// TestChunkList_Indices evaluates the Indices method of ChunkList on lists of different sizes.
func TestChunkList_Indices(t *testing.T) {
	cl := unittest.ChunkListFixture(5, unittest.IdentifierFixture())
	t.Run("empty chunk subset indices", func(t *testing.T) {
		// subset of chunk list that is empty should return an empty list
		subset := flow.ChunkList{}
		indices := subset.Indices()
		require.Len(t, indices, 0)
	})

	t.Run("single chunk subset indices", func(t *testing.T) {
		// subset of chunk list that contains chunk index of zero, should
		// return a uint64 slice that only contains chunk index of zero.
		subset := cl[:1]
		indices := subset.Indices()
		require.Len(t, indices, 1)
		require.Contains(t, indices, uint64(0))
	})

	t.Run("multiple chunk subset indices", func(t *testing.T) {
		// subset that only contains even chunk indices, should return
		// a uint64 slice that only contains even chunk indices
		subset := flow.ChunkList{cl[0], cl[2], cl[4]}
		indices := subset.Indices()
		require.Len(t, indices, 3)
		require.Contains(t, indices, uint64(0), uint64(2), uint64(4))
	})
}

func TestChunkIndexIsSet(t *testing.T) {

	i, err := rand.Uint()
	require.NoError(t, err)
	chunk := flow.NewChunk(
		unittest.IdentifierFixture(),
		int(i),
		unittest.StateCommitmentFixture(),
		21,
		unittest.IdentifierFixture(),
		unittest.StateCommitmentFixture(),
		17995,
	)

	assert.Equal(t, i, uint(chunk.Index))
	assert.Equal(t, i, uint(chunk.CollectionIndex))
}

func TestChunkNumberOfTxsIsSet(t *testing.T) {

	i, err := rand.Uint32()
	require.NoError(t, err)
	chunk := flow.NewChunk(
		unittest.IdentifierFixture(),
		3,
		unittest.StateCommitmentFixture(),
		int(i),
		unittest.IdentifierFixture(),
		unittest.StateCommitmentFixture(),
		17995,
	)

	assert.Equal(t, i, uint32(chunk.NumberOfTransactions))
}

func TestChunkTotalComputationUsedIsSet(t *testing.T) {

	i, err := rand.Uint64()
	require.NoError(t, err)
	chunk := flow.NewChunk(
		unittest.IdentifierFixture(),
		3,
		unittest.StateCommitmentFixture(),
		21,
		unittest.IdentifierFixture(),
		unittest.StateCommitmentFixture(),
		i,
	)

	assert.Equal(t, i, chunk.TotalComputationUsed)
}

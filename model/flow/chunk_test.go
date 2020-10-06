package flow_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
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
	chunkA := unittest.ChunkFixture(blockIdA)

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

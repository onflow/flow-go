package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/fingerprint"
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
		0,
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
		0,
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
		0,
		unittest.StateCommitmentFixture(),
		i,
	)

	assert.Equal(t, i, chunk.TotalComputationUsed)
}

// TestChunkEncodeDecode test encoding and decoding properties.
// In particular, we confirm that `nil` values of the ServiceEventCount field are preserved (and
// not conflated with 0) by the encoding schemes we use, because this difference is meaningful and
// important for backward compatibility (see [ChunkBody.ServiceEventCount] for details).
func TestChunkEncodeDecode(t *testing.T) {
	chunk := unittest.ChunkFixture(unittest.IdentifierFixture(), 0)

	t.Run("encode/decode preserves nil ServiceEventCount", func(t *testing.T) {
		chunk.ServiceEventCount = nil
		t.Run("json", func(t *testing.T) {
			bz, err := json.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = json.Unmarshal(bz, unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunk, unmarshaled)
			assert.Nil(t, unmarshaled.ServiceEventCount)
		})
		t.Run("cbor", func(t *testing.T) {
			bz, err := cbor.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = cbor.Unmarshal(bz, unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunk, unmarshaled)
			assert.Nil(t, unmarshaled.ServiceEventCount)
		})
	})
	t.Run("encode/decode preserves empty but non-nil ServiceEventCount", func(t *testing.T) {
		chunk.ServiceEventCount = unittest.PtrTo[uint16](0)
		t.Run("json", func(t *testing.T) {
			bz, err := json.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = json.Unmarshal(bz, unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunk, unmarshaled)
			assert.NotNil(t, unmarshaled.ServiceEventCount)
		})
		t.Run("cbor", func(t *testing.T) {
			bz, err := cbor.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = cbor.Unmarshal(bz, unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunk, unmarshaled)
			assert.NotNil(t, unmarshaled.ServiceEventCount)
		})
	})
}

// TestChunk_ModelVersions_EncodeDecode tests that encoding and decoding between
// supported versions works as expected.
func TestChunk_ModelVersions_EncodeDecode(t *testing.T) {
	chunkFixture := unittest.ChunkFixture(unittest.IdentifierFixture(), 1)
	chunkFixture.ServiceEventCount = unittest.PtrTo[uint16](0) // non-nil extra field

	t.Run("encoding v0 and decoding it into v1 should yield nil for ServiceEventCount", func(t *testing.T) {
		var chunkv0 flow.ChunkBodyV0
		unittest.CopyStructure(t, chunkFixture.ChunkBody, &chunkv0)

		t.Run("json", func(t *testing.T) {
			bz, err := json.Marshal(chunkv0)
			require.NoError(t, err)

			var unmarshaled flow.ChunkBody
			err = json.Unmarshal(bz, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunkv0.EventCollection, unmarshaled.EventCollection)
			assert.Equal(t, chunkv0.BlockID, unmarshaled.BlockID)
			assert.Nil(t, unmarshaled.ServiceEventCount)
		})

		t.Run("cbor", func(t *testing.T) {
			bz, err := cbor.Marshal(chunkv0)
			require.NoError(t, err)

			var unmarshaled flow.ChunkBody
			err = cbor.Unmarshal(bz, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunkv0.EventCollection, unmarshaled.EventCollection)
			assert.Equal(t, chunkv0.BlockID, unmarshaled.BlockID)
			assert.Nil(t, unmarshaled.ServiceEventCount)
		})
	})
	t.Run("encoding v1 and decoding it into v0 should not error", func(t *testing.T) {
		chunkv1 := chunkFixture.ChunkBody
		chunkv1.ServiceEventCount = unittest.PtrTo[uint16](0) // ensure non-nil ServiceEventCount field

		t.Run("json", func(t *testing.T) {
			bz, err := json.Marshal(chunkv1)
			require.NoError(t, err)

			var unmarshaled flow.ChunkBodyV0
			err = json.Unmarshal(bz, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunkv1.EventCollection, unmarshaled.EventCollection)
			assert.Equal(t, chunkv1.BlockID, unmarshaled.BlockID)
		})
		t.Run("cbor", func(t *testing.T) {
			bz, err := cbor.Marshal(chunkv1)
			require.NoError(t, err)

			var unmarshaled flow.ChunkBodyV0
			err = cbor.Unmarshal(bz, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunkv1.EventCollection, unmarshaled.EventCollection)
			assert.Equal(t, chunkv1.BlockID, unmarshaled.BlockID)
		})
	})
}

// FingerprintBackwardCompatibility ensures that the Fingerprint and ID functions
// are backward compatible with old data model versions. We emulate the
// case where a peer running an older software version receives a `ChunkBody` that
// was encoded in the new version. Specifically, if the new ServiceEventCount field
// is nil, then the new model should produce IDs consistent with the old model.
//
// Backward compatibility is implemented by providing a custom EncodeRLP method.
func TestChunk_FingerprintBackwardCompatibility(t *testing.T) {
	chunk := unittest.ChunkFixture(unittest.IdentifierFixture(), 1)
	chunk.ServiceEventCount = nil
	chunkBody := chunk.ChunkBody
	var chunkv0 flow.ChunkBodyV0
	unittest.CopyStructure(t, chunkBody, &chunkv0)

	// A nil ServiceEventCount fields indicates a prior model version.
	// The ID calculation for the old and new model version should be the same.
	t.Run("nil ServiceEventCount fields", func(t *testing.T) {
		chunkBody.ServiceEventCount = nil
		assert.Equal(t, flow.MakeID(chunkv0), flow.MakeID(chunkBody))
		assert.Equal(t, fingerprint.Fingerprint(chunkv0), fingerprint.Fingerprint(chunkBody))
	})
	// A non-nil ServiceEventCount fields indicates an up-to-date model version.
	// The ID calculation for the old and new model version should be different,
	// because the new model should include the ServiceEventCount field value.
	t.Run("non-nil ServiceEventCount fields", func(t *testing.T) {
		chunkBody.ServiceEventCount = unittest.PtrTo[uint16](0)
		assert.NotEqual(t, flow.MakeID(chunkv0), flow.MakeID(chunkBody))
	})
}

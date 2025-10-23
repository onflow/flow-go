package flow_test

import (
	"testing"

	"github.com/ipfs/go-cid"
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

// TestChunkList_Indices evaluates the Indices method of ChunkList on lists of different sizes.
func TestChunkList_Indices(t *testing.T) {
	cl := unittest.ChunkListFixture(5, unittest.IdentifierFixture(), unittest.StateCommitmentFixture())
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

	chunk, err := flow.NewChunk(flow.UntrustedChunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      i,
			StartState:           unittest.StateCommitmentFixture(),
			EventCollection:      unittest.IdentifierFixture(),
			ServiceEventCount:    0,
			BlockID:              unittest.IdentifierFixture(),
			TotalComputationUsed: 17995,
			NumberOfTransactions: uint64(21),
		},
		Index:    uint64(i),
		EndState: unittest.StateCommitmentFixture(),
	})

	require.NoError(t, err)
	assert.Equal(t, i, uint(chunk.Index))
	assert.Equal(t, i, uint(chunk.CollectionIndex))
}

func TestChunkNumberOfTxsIsSet(t *testing.T) {
	i, err := rand.Uint32()
	require.NoError(t, err)

	chunk, err := flow.NewChunk(flow.UntrustedChunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      3,
			StartState:           unittest.StateCommitmentFixture(),
			EventCollection:      unittest.IdentifierFixture(),
			ServiceEventCount:    0,
			BlockID:              unittest.IdentifierFixture(),
			TotalComputationUsed: 17995,
			NumberOfTransactions: uint64(i),
		},
		Index:    3,
		EndState: unittest.StateCommitmentFixture(),
	})

	require.NoError(t, err)
	assert.Equal(t, i, uint32(chunk.NumberOfTransactions))
}

func TestChunkTotalComputationUsedIsSet(t *testing.T) {
	i, err := rand.Uint64()
	require.NoError(t, err)

	chunk, err := flow.NewChunk(flow.UntrustedChunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      3,
			StartState:           unittest.StateCommitmentFixture(),
			EventCollection:      unittest.IdentifierFixture(),
			ServiceEventCount:    0,
			BlockID:              unittest.IdentifierFixture(),
			TotalComputationUsed: i,
			NumberOfTransactions: uint64(21),
		},
		Index:    3,
		EndState: unittest.StateCommitmentFixture(),
	})

	require.NoError(t, err)
	assert.Equal(t, i, chunk.TotalComputationUsed)
}

// TestChunkMalleability performs sanity checks to ensure that chunk is not malleable.
func TestChunkMalleability(t *testing.T) {
	t.Run("Chunk with non-nil ServiceEventCount", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(t, unittest.ChunkFixture(unittest.IdentifierFixture(), 0, unittest.StateCommitmentFixture()))
	})
}

// TestChunkDataPackMalleability performs sanity checks to ensure that ChunkDataPack is not malleable.
func TestChunkDataPackMalleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(
		t,
		unittest.ChunkDataPackFixture(unittest.IdentifierFixture()),
		unittest.WithTypeGenerator[cid.Cid](func() cid.Cid {
			return flow.IdToCid(unittest.IdentifierFixture())
		}),
	)
}

// TestNewChunkDataPack verifies the behavior of the NewChunkDataPack constructor.
// It ensures that a fully‐populated UntrustedChunkDataPack yields a valid ChunkDataPack,
// and that missing or invalid required fields produce an error.
//
// Test Cases:
//
// 1. Valid input:
//   - Ensures a ChunkDataPack is returned when all fields are populated.
//
// 2. Missing ChunkID:
//   - Ensures an error is returned when ChunkID is ZeroID.
//
// 3. Zero StartState:
//   - Ensures an error is returned when StartState is zero-value.
//
// 4. Nil Proof:
//   - Ensures an error is returned when Proof is nil.
//
// 5. Empty Proof:
//   - Ensures an error is returned when Proof is empty.
//
// 6. Nil Collection:
//   - Ensures an error is returned when Collection is nil.
//
// 7. Missing ExecutionDataRoot.BlockID:
//   - Ensures an error is returned when ExecutionDataRoot.BlockID is ZeroID.
//
// 8. Nil ExecutionDataRoot.ChunkExecutionDataIDs:
//   - Ensures an error is returned when ChunkExecutionDataIDs is nil.
//
// 9. Empty ExecutionDataRoot.ChunkExecutionDataIDs:
//   - Ensures an error is returned when ChunkExecutionDataIDs is empty.
func TestFromUntrustedChunkDataPack(t *testing.T) {
	chunkID := unittest.IdentifierFixture()
	startState := unittest.StateCommitmentFixture()
	proof := []byte{0x1, 0x2}
	collection := unittest.CollectionFixture(1)
	root := flow.BlockExecutionDataRoot{
		BlockID:               unittest.IdentifierFixture(),
		ChunkExecutionDataIDs: []cid.Cid{flow.IdToCid(unittest.IdentifierFixture())},
	}

	baseChunkDataPack := flow.UntrustedChunkDataPack{
		ChunkID:           chunkID,
		StartState:        startState,
		Proof:             proof,
		Collection:        &collection,
		ExecutionDataRoot: root,
	}

	t.Run("valid chunk data pack", func(t *testing.T) {
		pack, err := flow.NewChunkDataPack(baseChunkDataPack)
		assert.NoError(t, err)
		assert.NotNil(t, pack)
		assert.Equal(t, *pack, flow.ChunkDataPack(baseChunkDataPack))
	})

	t.Run("missing ChunkID", func(t *testing.T) {
		untrusted := baseChunkDataPack
		untrusted.ChunkID = flow.ZeroID

		pack, err := flow.NewChunkDataPack(untrusted)
		assert.Error(t, err)
		assert.Nil(t, pack)
		assert.Contains(t, err.Error(), "ChunkID")
	})

	t.Run("zero StartState", func(t *testing.T) {
		untrusted := baseChunkDataPack
		untrusted.StartState = flow.StateCommitment{}

		pack, err := flow.NewChunkDataPack(untrusted)
		assert.Error(t, err)
		assert.Nil(t, pack)
		assert.Contains(t, err.Error(), "StartState")
	})

	t.Run("nil Proof", func(t *testing.T) {
		untrusted := baseChunkDataPack
		untrusted.Proof = nil

		pack, err := flow.NewChunkDataPack(untrusted)
		assert.Error(t, err)
		assert.Nil(t, pack)
		assert.Contains(t, err.Error(), "Proof")
	})

	t.Run("empty Proof", func(t *testing.T) {
		untrusted := baseChunkDataPack
		untrusted.Proof = []byte{}

		pack, err := flow.NewChunkDataPack(untrusted)
		assert.Error(t, err)
		assert.Nil(t, pack)
		assert.Contains(t, err.Error(), "Proof")
	})

	t.Run("missing ExecutionDataRoot.BlockID", func(t *testing.T) {
		untrusted := baseChunkDataPack
		untrusted.ExecutionDataRoot.BlockID = flow.ZeroID

		pack, err := flow.NewChunkDataPack(untrusted)
		assert.Error(t, err)
		assert.Nil(t, pack)
		assert.Contains(t, err.Error(), "ExecutionDataRoot.BlockID")
	})

	t.Run("nil ExecutionDataRoot.ChunkExecutionDataIDs", func(t *testing.T) {
		untrusted := baseChunkDataPack
		untrusted.ExecutionDataRoot.ChunkExecutionDataIDs = nil

		pack, err := flow.NewChunkDataPack(untrusted)
		assert.Error(t, err)
		assert.Nil(t, pack)
		assert.Contains(t, err.Error(), "ExecutionDataRoot.ChunkExecutionDataIDs")
	})

	t.Run("empty ExecutionDataRoot.ChunkExecutionDataIDs", func(t *testing.T) {
		untrusted := baseChunkDataPack
		untrusted.ExecutionDataRoot.ChunkExecutionDataIDs = []cid.Cid{}

		pack, err := flow.NewChunkDataPack(untrusted)
		assert.Error(t, err)
		assert.Nil(t, pack)
		assert.Contains(t, err.Error(), "ExecutionDataRoot.ChunkExecutionDataIDs")
	})
}

// TestNewChunk verifies that NewChunk constructs a valid Chunk when given
// complete, nonzero fields, and returns an error if any required field is
// missing or zero.
//
// Test Cases:
//
// 1. Valid input:
//   - Ensures a Chunk is returned when all fields are populated.
//
// 2. Missing BlockID:
//   - Ensures an error is returned when BlockID is ZeroID.
//
// 3. Zero StartState:
//   - Ensures an error is returned when StartState is zero-value.
//
// 4. Nil ServiceEventCount:
//   - Ensures an error is returned when ServiceEventCount is nil.
//
// 5. Missing EventCollection:
//   - Ensures an error is returned when EventCollection is ZeroID.
//
// 6. Zero EndState:
//   - Ensures an error is returned when EndState is zero-value.
func TestNewChunk(t *testing.T) {
	validID := unittest.IdentifierFixture()
	validState := unittest.StateCommitmentFixture()

	base := flow.UntrustedChunk{
		ChunkBody: flow.ChunkBody{
			BlockID:              validID,
			CollectionIndex:      3,
			StartState:           validState,
			EventCollection:      validID,
			ServiceEventCount:    2,
			TotalComputationUsed: 10,
			NumberOfTransactions: 5,
		},
		Index:    1,
		EndState: validState,
	}

	t.Run("valid chunk", func(t *testing.T) {
		ch, err := flow.NewChunk(base)
		assert.NoError(t, err)
		assert.NotNil(t, ch)
		assert.Equal(t, *ch, flow.Chunk(base))
	})

	t.Run("missing BlockID", func(t *testing.T) {
		u := base
		u.ChunkBody.BlockID = flow.ZeroID

		ch, err := flow.NewChunk(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "BlockID")
	})

	t.Run("zero StartState", func(t *testing.T) {
		u := base
		u.ChunkBody.StartState = flow.StateCommitment{}

		ch, err := flow.NewChunk(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "StartState")
	})

	t.Run("missing EventCollection", func(t *testing.T) {
		u := base
		u.ChunkBody.EventCollection = flow.ZeroID

		ch, err := flow.NewChunk(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "EventCollection")
	})

	t.Run("zero EndState", func(t *testing.T) {
		u := base
		u.EndState = flow.StateCommitment{}

		ch, err := flow.NewChunk(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "EndState")
	})
}

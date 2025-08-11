package flow_test

import (
	"testing"

<<<<<<< HEAD
=======
	"github.com/fxamacker/cbor/v2"
	"github.com/ipfs/go-cid"
>>>>>>> feature/malleability
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
			ServiceEventCount:    unittest.PtrTo[uint16](0),
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
			ServiceEventCount:    unittest.PtrTo[uint16](0),
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
			ServiceEventCount:    unittest.PtrTo[uint16](0),
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
<<<<<<< HEAD
=======

// TestChunkEncodeDecode test encoding and decoding properties.
// In particular, we confirm that `nil` values of the ServiceEventCount field are preserved (and
// not conflated with 0) by the encoding schemes we use, because this difference is meaningful and
// important for backward compatibility (see [ChunkBody.ServiceEventCount] for details).
func TestChunkEncodeDecode(t *testing.T) {
	chunk := unittest.ChunkFixture(unittest.IdentifierFixture(), 0, unittest.StateCommitmentFixture())

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
		t.Run("lax non-BFT cbor decoding", func(t *testing.T) {
			bz, err := cborcodec.EncMode.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = cborcodec.UnsafeDecMode.Unmarshal(bz, unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunk, unmarshaled)
			assert.Nil(t, unmarshaled.ServiceEventCount)
		})
		t.Run("default strict cbor decoding", func(t *testing.T) {
			bz, err := cborcodec.EncMode.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = cborcodec.DefaultDecMode.Unmarshal(bz, unmarshaled)
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
		t.Run("lax non-BFT cbor decoding", func(t *testing.T) {
			bz, err := cborcodec.EncMode.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = cborcodec.UnsafeDecMode.Unmarshal(bz, unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunk, unmarshaled)
			assert.NotNil(t, unmarshaled.ServiceEventCount)
		})
		t.Run("default strict cbor decoding", func(t *testing.T) {
			bz, err := cborcodec.EncMode.Marshal(chunk)
			require.NoError(t, err)
			unmarshaled := new(flow.Chunk)
			err = cborcodec.DefaultDecMode.Unmarshal(bz, unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunk, unmarshaled)
			assert.NotNil(t, unmarshaled.ServiceEventCount)
		})
	})
}

// TestChunk_ModelVersions_EncodeDecode tests that encoding and decoding between
// supported versions works as expected.
func TestChunk_ModelVersions_EncodeDecode(t *testing.T) {
	chunkFixture := unittest.ChunkFixture(unittest.IdentifierFixture(), 1, unittest.StateCommitmentFixture())
	chunkFixture.ServiceEventCount = unittest.PtrTo[uint16](0) // non-nil extra field

	t.Run("encoding v0 and decoding it into v1 should yield nil for ServiceEventCount", func(t *testing.T) {
		var chunkv0 flow.ChunkBodyV0
		unittest.EncodeDecodeDifferentVersions(t, chunkFixture.ChunkBody, &chunkv0)

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

		t.Run("lax non-BFT cbor decoding", func(t *testing.T) {
			bz, err := cborcodec.EncMode.Marshal(chunkv0)
			require.NoError(t, err)

			var unmarshaled flow.ChunkBody
			err = cborcodec.UnsafeDecMode.Unmarshal(bz, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunkv0.EventCollection, unmarshaled.EventCollection)
			assert.Equal(t, chunkv0.BlockID, unmarshaled.BlockID)
			assert.Nil(t, unmarshaled.ServiceEventCount)
		})

		t.Run("default strict cbor decoding", func(t *testing.T) {
			bz, err := cborcodec.EncMode.Marshal(chunkv0)
			require.NoError(t, err)

			var unmarshaled flow.ChunkBody
			err = cborcodec.DefaultDecMode.Unmarshal(bz, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, chunkv0.EventCollection, unmarshaled.EventCollection)
			assert.Equal(t, chunkv0.BlockID, unmarshaled.BlockID)
			assert.Nil(t, unmarshaled.ServiceEventCount)
		})
	})
	t.Run("encoding v1 and decoding it into v0", func(t *testing.T) {
		t.Run("non-nil ServiceEventCount field", func(t *testing.T) {
			chunkv1 := chunkFixture.ChunkBody
			chunkv1.ServiceEventCount = unittest.PtrTo[uint16](0) // ensure non-nil ServiceEventCount field

			t.Run("json - should not error", func(t *testing.T) {
				bz, err := json.Marshal(chunkv1)
				require.NoError(t, err)

				var unmarshaled flow.ChunkBodyV0
				err = json.Unmarshal(bz, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, chunkv1.EventCollection, unmarshaled.EventCollection)
				assert.Equal(t, chunkv1.BlockID, unmarshaled.BlockID)
			})
			t.Run("lax non-BFT cbor decoding - should not error", func(t *testing.T) {
				// CAUTION: using the lax decoding is not safe for data structures that are exchanged between nodes!
				bz, err := cborcodec.EncMode.Marshal(chunkv1)
				require.NoError(t, err)

				var unmarshaled flow.ChunkBodyV0
				err = cborcodec.UnsafeDecMode.Unmarshal(bz, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, chunkv1.EventCollection, unmarshaled.EventCollection)
				assert.Equal(t, chunkv1.BlockID, unmarshaled.BlockID)
			})
			// In the stricter mode (default), which we use for decoding on the networking layer, an error is expected
			// because the message includes a field not present in the v0 target,
			// when the new ServiceEventCount field is non-nil
			t.Run("cbor strict - error expected", func(t *testing.T) {
				bz, err := cborcodec.EncMode.Marshal(chunkv1)
				require.NoError(t, err)

				var unmarshaled flow.ChunkBodyV0
				err = cborcodec.DefaultDecMode.Unmarshal(bz, &unmarshaled)
				assert.Error(t, err)
				target := &cbor.UnknownFieldError{}
				assert.ErrorAs(t, err, &target)
			})
		})
		t.Run("nil ServiceEventCount field should not error", func(t *testing.T) {
			chunkv1 := chunkFixture.ChunkBody
			chunkv1.ServiceEventCount = nil // ensure non-nil ServiceEventCount field

			t.Run("json", func(t *testing.T) {
				bz, err := json.Marshal(chunkv1)
				require.NoError(t, err)

				var unmarshaled flow.ChunkBodyV0
				err = json.Unmarshal(bz, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, chunkv1.EventCollection, unmarshaled.EventCollection)
				assert.Equal(t, chunkv1.BlockID, unmarshaled.BlockID)
			})
			t.Run("lax non-BFT cbor decoding", func(t *testing.T) {
				bz, err := cborcodec.EncMode.Marshal(chunkv1)
				require.NoError(t, err)

				var unmarshaled flow.ChunkBodyV0
				err = cborcodec.UnsafeDecMode.Unmarshal(bz, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, chunkv1.EventCollection, unmarshaled.EventCollection)
				assert.Equal(t, chunkv1.BlockID, unmarshaled.BlockID)
			})
			// When the new ServiceEventCount field is nil, we expect even strict
			// cbor decoding to pass, because the new field will be omitted.
			t.Run("cbor strict", func(t *testing.T) {
				bz, err := cborcodec.EncMode.Marshal(chunkv1)
				require.NoError(t, err)

				var unmarshaled flow.ChunkBodyV0
				err = cborcodec.DefaultDecMode.Unmarshal(bz, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, chunkv1.EventCollection, unmarshaled.EventCollection)
				assert.Equal(t, chunkv1.BlockID, unmarshaled.BlockID)
			})
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
	chunk := unittest.ChunkFixture(unittest.IdentifierFixture(), 1, unittest.StateCommitmentFixture())
	chunk.ServiceEventCount = nil

	// Define an older type which use flow.ChunkBodyV0
	type ChunkV0 struct {
		flow.ChunkBodyV0
		Index    uint64
		EndState flow.StateCommitment
	}

	var chunkV0 ChunkV0
	unittest.EncodeDecodeDifferentVersions(t, chunk, &chunkV0)

	// A nil ServiceEventCount fields indicates a prior model version.
	// The ID calculation for the old and new model version should be the same.
	t.Run("nil ServiceEventCount fields", func(t *testing.T) {
		chunk.ServiceEventCount = nil
		assert.Equal(t, flow.MakeID(chunkV0), chunk.ID())
		assert.Equal(t, flow.MakeID(chunkV0), flow.MakeID(chunk))
	})
	// A non-nil ServiceEventCount fields indicates an up-to-date model version.
	// The ID calculation for the old and new model version should be different,
	// because the new model should include the ServiceEventCount field value.
	t.Run("non-nil ServiceEventCount fields", func(t *testing.T) {
		chunk.ServiceEventCount = unittest.PtrTo[uint16](0)
		assert.NotEqual(t, flow.MakeID(chunkV0), chunk.ID())
		assert.NotEqual(t, flow.MakeID(chunkV0), flow.MakeID(chunk))
	})
}

// TestChunkMalleability performs sanity checks to ensure that chunk is not malleable.
func TestChunkMalleability(t *testing.T) {
	t.Run("Chunk with non-nil ServiceEventCount", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(t, unittest.ChunkFixture(unittest.IdentifierFixture(), 0, unittest.StateCommitmentFixture()))
	})

	// TODO(mainnet27, #6773): remove this test according to https://github.com/onflow/flow-go/issues/6773
	t.Run("Chunk with nil ServiceEventCount", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(
			t,
			unittest.ChunkFixture(unittest.IdentifierFixture(), 0, unittest.StateCommitmentFixture(), func(c *flow.Chunk) {
				c.ServiceEventCount = nil
			}),
			// We pin the `ServiceEventCount` to the current value (nil), so `MalleabilityChecker` will not mutate this field:
			unittest.WithPinnedField("ChunkBody.ServiceEventCount"),
		)
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
func TestNewChunkDataPack(t *testing.T) {
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
	validServiceCount := uint16(2)

	base := flow.UntrustedChunk{
		ChunkBody: flow.ChunkBody{
			BlockID:              validID,
			CollectionIndex:      3,
			StartState:           validState,
			EventCollection:      validID,
			ServiceEventCount:    &validServiceCount,
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

	t.Run("nil ServiceEventCount", func(t *testing.T) {
		u := base
		u.ChunkBody.ServiceEventCount = nil

		ch, err := flow.NewChunk(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "ServiceEventCount")
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

// TestNewChunk_ProtocolVersion1 verifies that NewChunk_ProtocolVersion1 constructs a
// valid Chunk for protocol version 1 when given complete, non-zero fields, and
// returns an error if any required field is missing.
//
// Test Cases:
//
// 1. Valid protocol v1 input:
//   - Ensures a valid Chunk is returned with ServiceEventCount == nil.
//
// 2. Missing BlockID:
//   - Ensures an error is returned when BlockID is ZeroID.
//
// 3. Zero StartState:
//   - Ensures an error is returned when StartState is zero-value.
//
// 4. Missing EventCollection:
//   - Ensures an error is returned when EventCollection is ZeroID.
//
// 5. Zero EndState:
//   - Ensures an error is returned when EndState is zero-value.
func TestNewChunk_ProtocolVersion1(t *testing.T) {
	validID := unittest.IdentifierFixture()
	validState := unittest.StateCommitmentFixture()

	base := flow.UntrustedChunk{
		ChunkBody: flow.ChunkBody{
			BlockID:              validID,
			CollectionIndex:      2,
			StartState:           validState,
			EventCollection:      validID,
			ServiceEventCount:    nil, // ignored in v1
			TotalComputationUsed: 7,
			NumberOfTransactions: 3,
		},
		Index:    1,
		EndState: validState,
	}

	t.Run("valid protocol v1 chunk", func(t *testing.T) {
		ch, err := flow.NewChunk_ProtocolVersion1(base)
		assert.NoError(t, err)
		assert.NotNil(t, ch)

		// ServiceEventCount must be nil for protocol v1
		assert.Nil(t, ch.ChunkBody.ServiceEventCount)

		assert.Equal(t, *ch, flow.Chunk(base))
	})

	t.Run("missing BlockID", func(t *testing.T) {
		u := base
		u.ChunkBody.BlockID = flow.ZeroID

		ch, err := flow.NewChunk_ProtocolVersion1(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "BlockID")
	})

	t.Run("zero StartState", func(t *testing.T) {
		u := base
		u.ChunkBody.StartState = flow.StateCommitment{}

		ch, err := flow.NewChunk_ProtocolVersion1(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "StartState")
	})

	t.Run("missing EventCollection", func(t *testing.T) {
		u := base
		u.ChunkBody.EventCollection = flow.ZeroID

		ch, err := flow.NewChunk_ProtocolVersion1(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "EventCollection")
	})

	t.Run("zero EndState", func(t *testing.T) {
		u := base
		u.EndState = flow.StateCommitment{}

		ch, err := flow.NewChunk_ProtocolVersion1(u)
		assert.Error(t, err)
		assert.Nil(t, ch)
		assert.Contains(t, err.Error(), "EndState")
	})
}
>>>>>>> feature/malleability

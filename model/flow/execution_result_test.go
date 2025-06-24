package flow_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestExecutionResultID_Malleability confirms that the ExecutionResult struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestExecutionResultID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t,
		unittest.ExecutionResultFixture(),
		unittest.WithFieldGenerator("ServiceEvents", func() []flow.ServiceEvent {
			return unittest.ServiceEventsFixture(3)
		}))
}

// TestExecutionResultGroupBy tests the GroupBy method of ExecutionResultList:
// * grouping should preserve order and multiplicity of elements
// * group for unknown identifier should be empty
func TestExecutionResultGroupBy(t *testing.T) {

	er1 := unittest.ExecutionResultFixture()
	er2 := unittest.ExecutionResultFixture()
	er3 := unittest.ExecutionResultFixture()

	idA := unittest.IdentifierFixture()
	idB := unittest.IdentifierFixture()
	grouperFunc := func(er *flow.ExecutionResult) flow.Identifier {
		switch er.ID() {
		case er1.ID():
			return idA
		case er2.ID():
			return idB
		case er3.ID():
			return idA
		default:
			panic("unexpected ExecutionReceipt")
		}
	}

	groups := flow.ExecutionResultList{er1, er2, er3, er1}.GroupBy(grouperFunc)
	assert.Equal(t, 2, groups.NumberGroups())
	assert.Equal(t, flow.ExecutionResultList{er1, er3, er1}, groups.GetGroup(idA))
	assert.Equal(t, flow.ExecutionResultList{er2}, groups.GetGroup(idB))

	unknown := groups.GetGroup(unittest.IdentifierFixture())
	assert.Equal(t, 0, unknown.Size())
}

// FingerprintBackwardCompatibility ensures that the Fingerprint and ID functions
// are backward compatible with old data model versions. Specifically, if the new
// ServiceEventCount field is nil, then the new model should produce IDs consistent
// with the old model.
//
// Backward compatibility is implemented by providing a custom EncodeRLP method.
func TestExecutionResult_FingerprintBackwardCompatibility(t *testing.T) {
	// Define a series of types which use flow.ChunkBodyV0
	type ChunkV0 struct {
		flow.ChunkBodyV0
		Index    uint64
		EndState flow.StateCommitment
	}
	type ChunkListV0 []*ChunkV0
	type ExecutionResultV0 struct {
		PreviousResultID flow.Identifier
		BlockID          flow.Identifier
		Chunks           ChunkListV0
		ServiceEvents    flow.ServiceEventList
		ExecutionDataID  flow.Identifier
	}

	// Construct an ExecutionResult with nil ServiceEventCount fields
	result := unittest.ExecutionResultFixture()
	for i := range result.Chunks {
		result.Chunks[i].ServiceEventCount = nil
	}

	// Copy all fields to the prior-version model
	var resultv0 ExecutionResultV0
	unittest.EncodeDecodeDifferentVersions(t, result, &resultv0)

	assert.Equal(t, result.ID(), flow.MakeID(resultv0))
	assert.Equal(t, fingerprint.Fingerprint(result), fingerprint.Fingerprint(resultv0))
}

// Tests that [ExecutionResult.ServiceEventsByChunk] method works in a variety of circumstances.
// It also tests the method against an ExecutionResult instance backed by both the
// current and old data model version (with and with ServiceEventCount field)
func TestExecutionResult_ServiceEventsByChunk(t *testing.T) {
	t.Run("no service events", func(t *testing.T) {
		t.Run("nil ServiceEventCount field (old model)", func(t *testing.T) {
			result := unittest.ExecutionResultFixture()
			for _, chunk := range result.Chunks {
				chunk.ServiceEventCount = nil
			}
			// should return empty list for all chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
		})
		t.Run("populated ServiceEventCount field", func(t *testing.T) {
			result := unittest.ExecutionResultFixture()
			for _, chunk := range result.Chunks {
				chunk.ServiceEventCount = unittest.PtrTo[uint16](0)
			}
			// should return empty list for all chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
		})
	})

	t.Run("service events only in system chunk", func(t *testing.T) {
		t.Run("nil ServiceEventCount field (old model)", func(t *testing.T) {
			nServiceEvents := rand.Intn(10) + 1
			result := unittest.ExecutionResultFixture(unittest.WithServiceEvents(nServiceEvents))
			for _, chunk := range result.Chunks {
				chunk.ServiceEventCount = nil
			}

			// should return empty list for all chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len()-1; chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
			// should return list of service events for system chunk
			assert.Equal(t, result.ServiceEvents, result.ServiceEventsByChunk(result.SystemChunk().Index))
		})
		t.Run("populated ServiceEventCount field", func(t *testing.T) {
			nServiceEvents := rand.Intn(10) + 1
			result := unittest.ExecutionResultFixture(unittest.WithServiceEvents(nServiceEvents))
			for _, chunk := range result.Chunks[:result.Chunks.Len()-1] {
				chunk.ServiceEventCount = unittest.PtrTo[uint16](0)
			}
			result.SystemChunk().ServiceEventCount = unittest.PtrTo(uint16(nServiceEvents))

			// should return empty list for all non-system chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len()-1; chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
			// should return list of service events for system chunk
			assert.Equal(t, result.ServiceEvents, result.ServiceEventsByChunk(result.SystemChunk().Index))
		})
	})

	// NOTE: service events in non-system chunks is unsupported by the old data model
	t.Run("service only in non-system chunks", func(t *testing.T) {
		result := unittest.ExecutionResultFixture()
		unittest.WithServiceEvents(result.Chunks.Len() - 1)(result) // one service event per non-system chunk

		for _, chunk := range result.Chunks {
			// 1 service event per chunk
			chunk.ServiceEventCount = unittest.PtrTo(uint16(1))
		}
		result.SystemChunk().ServiceEventCount = unittest.PtrTo(uint16(0))

		// should return one service event per non-system chunk
		for chunkIndex := 0; chunkIndex < result.Chunks.Len()-1; chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Equal(t, result.ServiceEvents[chunkIndex:chunkIndex+1], serviceEvents)
		}
		// should return empty list for system chunk
		assert.Len(t, result.ServiceEventsByChunk(result.SystemChunk().Index), 0)
	})

	// NOTE: service events in non-system chunks is unsupported by the old data model
	t.Run("service events in all chunks", func(t *testing.T) {
		result := unittest.ExecutionResultFixture()
		unittest.WithServiceEvents(result.Chunks.Len())(result) // one service event per chunk

		for _, chunk := range result.Chunks {
			// 1 service event per chunk
			chunk.ServiceEventCount = unittest.PtrTo(uint16(1))
		}

		// should return one service event per chunk
		for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Equal(t, result.ServiceEvents[chunkIndex:chunkIndex+1], serviceEvents)
		}
	})
}

// TestNewExecutionResult verifies that NewExecutionResult constructs a valid
// ExecutionResult when given all required fields, and returns an error if any
// required field is missing.
// This test covers the test cases:
//   - valid result with non-nil Chunks and ServiceEvents
//   - valid result with nil ServiceEvents
//   - missing PreviousResultID
//   - missing BlockID
//   - nil Chunks
//   - missing ExecutionDataID
func TestNewExecutionResult(t *testing.T) {
	validPrevID := unittest.IdentifierFixture()
	validBlockID := unittest.IdentifierFixture()
	validExecDataID := unittest.IdentifierFixture()
	chunks := unittest.ChunkListFixture(5, unittest.IdentifierFixture(), unittest.StateCommitmentFixture())

	t.Run("valid result with non-nil slices", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          validBlockID,
			Chunks:           chunks,
			ServiceEvents:    flow.ServiceEventList{},
			ExecutionDataID:  validExecDataID,
		}
		res, err := flow.NewExecutionResult(u)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, validPrevID, res.PreviousResultID)
		assert.Equal(t, validBlockID, res.BlockID)
		assert.Equal(t, chunks, res.Chunks)
		assert.Equal(t, flow.ServiceEventList{}, res.ServiceEvents)
		assert.Equal(t, validExecDataID, res.ExecutionDataID)
	})

	t.Run("valid result with nil ServiceEvents", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          validBlockID,
			Chunks:           chunks,
			// ServiceEvents left nil
			ExecutionDataID: validExecDataID,
		}
		res, err := flow.NewExecutionResult(u)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Nil(t, res.ServiceEvents)
	})

	t.Run("missing PreviousResultID", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: flow.ZeroID,
			BlockID:          validBlockID,
			Chunks:           chunks,
			ExecutionDataID:  validExecDataID,
		}
		res, err := flow.NewExecutionResult(u)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "PreviousResultID")
	})

	t.Run("missing BlockID", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          flow.ZeroID,
			Chunks:           chunks,
			ExecutionDataID:  validExecDataID,
		}
		res, err := flow.NewExecutionResult(u)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "BlockID")
	})

	t.Run("nil Chunks", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          validBlockID,
			Chunks:           nil,
			ExecutionDataID:  validExecDataID,
		}
		res, err := flow.NewExecutionResult(u)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "Chunks")
	})

	t.Run("missing ExecutionDataID", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          validBlockID,
			Chunks:           chunks,
			ExecutionDataID:  flow.ZeroID,
		}
		res, err := flow.NewExecutionResult(u)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "ExecutionDataID")
	})
}

// TestNewRootExecutionResult verifies that NewRootExecutionResult constructs a valid root
// ExecutionResult when given all required fields, and returns an error if any
// required field is missing.
// This test covers the test cases:
//   - valid result with non-nil Chunks and ServiceEvents
//   - valid result with nil ServiceEvents
//   - missing PreviousResultID
//   - missing BlockID
//   - nil Chunks
//   - missing ExecutionDataID
func TestNewRootExecutionResult(t *testing.T) {
	validPrevID := unittest.IdentifierFixture()
	validBlockID := unittest.IdentifierFixture()
	validExecDataID := unittest.IdentifierFixture()
	chunks := unittest.ChunkListFixture(5, unittest.IdentifierFixture(), unittest.StateCommitmentFixture())

	t.Run("valid root result with non-nil slices", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: flow.ZeroID,
			BlockID:          validBlockID,
			Chunks:           chunks,
			ServiceEvents:    flow.ServiceEventList{},
			ExecutionDataID:  flow.ZeroID,
		}
		res, err := flow.NewRootExecutionResult(u)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, flow.ZeroID, res.PreviousResultID)
		assert.Equal(t, validBlockID, res.BlockID)
		assert.Equal(t, chunks, res.Chunks)
		assert.Equal(t, flow.ServiceEventList{}, res.ServiceEvents)
		assert.Equal(t, flow.ZeroID, res.ExecutionDataID)
	})

	t.Run("missing BlockID", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          flow.ZeroID,
			Chunks:           chunks,
			ServiceEvents:    flow.ServiceEventList{},
			ExecutionDataID:  validExecDataID,
		}
		res, err := flow.NewRootExecutionResult(u)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "BlockID")
	})

	t.Run("nil Chunks", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          validBlockID,
			Chunks:           nil,
			ExecutionDataID:  validExecDataID,
		}
		res, err := flow.NewRootExecutionResult(u)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "Chunks")
	})
}

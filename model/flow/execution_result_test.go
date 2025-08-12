package flow_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

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

// Tests that [ExecutionResult.ServiceEventsByChunk] method works in a variety of circumstances.
func TestExecutionResult_ServiceEventsByChunk(t *testing.T) {
	t.Run("no service events", func(t *testing.T) {
		result := unittest.ExecutionResultFixture()
		for _, chunk := range result.Chunks {
			chunk.ServiceEventCount = 0
		}
		// should return empty list for all chunks
		for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Len(t, serviceEvents, 0)
		}
	})

	t.Run("service events only in system chunk", func(t *testing.T) {
		nServiceEvents := rand.Intn(10) + 1
		result := unittest.ExecutionResultFixture(unittest.WithServiceEvents(nServiceEvents))
		for _, chunk := range result.Chunks[:result.Chunks.Len()-1] {
			chunk.ServiceEventCount = 0
		}
		result.SystemChunk().ServiceEventCount = uint16(nServiceEvents)

		// should return empty list for all non-system chunks
		for chunkIndex := 0; chunkIndex < result.Chunks.Len()-1; chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Len(t, serviceEvents, 0)
		}
		// should return list of service events for system chunk
		assert.Equal(t, result.ServiceEvents, result.ServiceEventsByChunk(result.SystemChunk().Index))
	})

	t.Run("service only in non-system chunks", func(t *testing.T) {
		result := unittest.ExecutionResultFixture()
		unittest.WithServiceEvents(result.Chunks.Len() - 1)(result) // one service event per non-system chunk

		for _, chunk := range result.Chunks {
			chunk.ServiceEventCount = 1
		}
		result.SystemChunk().ServiceEventCount = 0

		// should return one service event per non-system chunk
		for chunkIndex := 0; chunkIndex < result.Chunks.Len()-1; chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Equal(t, result.ServiceEvents[chunkIndex:chunkIndex+1], serviceEvents)
		}
		// should return empty list for system chunk
		assert.Len(t, result.ServiceEventsByChunk(result.SystemChunk().Index), 0)
	})

	t.Run("service events in all chunks", func(t *testing.T) {
		result := unittest.ExecutionResultFixture()
		unittest.WithServiceEvents(result.Chunks.Len())(result) // one service event per chunk

		for _, chunk := range result.Chunks {
			chunk.ServiceEventCount = 1
		}

		// should return one service event per chunk
		for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Equal(t, result.ServiceEvents[chunkIndex:chunkIndex+1], serviceEvents)
		}
	})
}

// TestNewExecutionResult verifies the behavior of the NewExecutionResult constructor.
// It ensures that a fully populated UntrustedExecutionResult yields a valid ExecutionResult,
// and that missing or invalid required fields produce an error.
//
// Test Cases:
//
// 1. Valid input with non‐nil Chunks and non‐nil ServiceEvents:
//   - PreviousResultID, BlockID, Chunks, ServiceEvents, and ExecutionDataID are all set.
//   - Expect no error and a properly populated ExecutionResult.
//
// 2. Valid input with non‐nil Chunks and nil ServiceEvents:
//   - ServiceEvents omitted (nil) but all other fields valid.
//   - Expect no error and ExecutionResult.ServiceEvents == nil.
//
// 3. Invalid input: PreviousResultID is ZeroID:
//   - Ensures error when the previous result ID is missing.
//
// 4. Invalid input: BlockID is ZeroID:
//   - Ensures error when the block ID is missing.
//
// 5. Invalid input: Chunks is nil:
//   - Ensures error when the chunk list is nil.
//
// 6. Invalid input: Chunks are empty:
//   - Ensures error when the chunk list is empty.
//
// 7. Invalid input: ExecutionDataID is ZeroID:
//   - Ensures error when the execution data ID is missing.
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
		assert.Equal(t, *res, flow.ExecutionResult(u))
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

	t.Run("empty Chunks", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          validBlockID,
			Chunks:           flow.ChunkList{},
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

// TestNewRootExecutionResult verifies the behavior of the NewRootExecutionResult constructor.
// It ensures that a “root” ExecutionResult can be created with an empty PreviousResultID
// and ExecutionDataID, given a valid BlockID and non‐empty Chunks, and that missing
// required fields produce an error.
//
// Test Cases:
//
// 1. Valid root input with non‐empty Chunks:
//   - BlockID set, Chunks non‐empty, PreviousResultID and ExecutionDataID left at ZeroID.
//   - Expect no error and ExecutionResult with zero PreviousResultID/ExecutionDataID.
//
// 2. Invalid input: BlockID is ZeroID:
//   - Ensures error when the block ID is missing.
//
// 3. Invalid input: Chunks are empty:
//   - Ensures error when the chunk list is empty.
//
// 4. Invalid input: Chunks is nil:
//   - Ensures error when the chunk list is nil.
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
		assert.Equal(t, *res, flow.ExecutionResult(u))
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

	t.Run("empty Chunks", func(t *testing.T) {
		u := flow.UntrustedExecutionResult{
			PreviousResultID: validPrevID,
			BlockID:          validBlockID,
			Chunks:           flow.ChunkList{},
			ExecutionDataID:  validExecDataID,
		}
		res, err := flow.NewRootExecutionResult(u)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "Chunks")
	})
}

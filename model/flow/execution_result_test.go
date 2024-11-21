package flow_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/slices"
	"github.com/onflow/flow-go/utils/unittest"
)

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
// It also tests the method against an ExecutionResult instance backed by both the
// current and old data model version (with and with ServiceEventIndices field)
func TestExecutionResult_ServiceEventsByChunk(t *testing.T) {
	t.Run("no service events", func(t *testing.T) {
		t.Run("nil ServiceEventIndices field (old model)", func(t *testing.T) {
			result := unittest.ExecutionResultFixture()
			for _, chunk := range result.Chunks {
				chunk.ServiceEventIndices = nil
			}
			// should return empty list for all chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
		})
		t.Run("populated ServiceEventIndices field", func(t *testing.T) {
			result := unittest.ExecutionResultFixture()
			for _, chunk := range result.Chunks {
				chunk.ServiceEventIndices = make([]uint32, 0)
			}
			// should return empty list for all chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
		})
	})

	t.Run("service events only in system chunk", func(t *testing.T) {
		t.Run("nil ServiceEventIndices field (old model)", func(t *testing.T) {
			result := unittest.ExecutionResultFixture()
			for _, chunk := range result.Chunks {
				chunk.ServiceEventIndices = nil
			}

			// should return empty list for all chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
		})
		t.Run("populated ServiceEventIndices field", func(t *testing.T) {
			nServiceEvents := rand.Intn(10) + 1
			result := unittest.ExecutionResultFixture(unittest.WithServiceEvents(nServiceEvents))
			for _, chunk := range result.Chunks[:result.Chunks.Len()-1] {
				chunk.ServiceEventIndices = make([]uint32, 0)
			}
			result.SystemChunk().ServiceEventIndices = slices.MakeRange(0, uint32(nServiceEvents))

			// should return empty list for all non-system chunks
			for chunkIndex := 0; chunkIndex < result.Chunks.Len()-1; chunkIndex++ {
				serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
				assert.Len(t, serviceEvents, 0)
			}
			// should return list of service events for system chunk
			assert.Equal(t, result.ServiceEvents, result.ServiceEventsByChunk(result.SystemChunk().Index))
		})
	})

	// NOTE: service events in non-system chunks in unsupported by the old data model
	t.Run("service only in non-system chunks", func(t *testing.T) {
		result := unittest.ExecutionResultFixture()
		unittest.WithServiceEvents(result.Chunks.Len() - 1)(result) // one service event per non-system chunk

		for chunkIndex, chunk := range result.Chunks {
			// 1 service event per chunk => service event indices match chunk indices
			chunk.ServiceEventIndices = []uint32{uint32(chunkIndex)}
		}
		result.SystemChunk().ServiceEventIndices = make([]uint32, 0) // none in system chunk

		// should return one service event per non-system chunk
		for chunkIndex := 0; chunkIndex < result.Chunks.Len()-1; chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Equal(t, result.ServiceEvents[chunkIndex:chunkIndex+1], serviceEvents)
		}
		// should return empty list for system chunk
		assert.Len(t, result.ServiceEventsByChunk(result.SystemChunk().Index), 0)
	})

	// NOTE: service events in non-system chunks in unsupported by the old data model
	t.Run("service events in all chunks", func(t *testing.T) {
		result := unittest.ExecutionResultFixture()
		unittest.WithServiceEvents(result.Chunks.Len())(result) // one service event per chunk

		for chunkIndex, chunk := range result.Chunks {
			// 1 service event per chunk => service event indices match chunk indices
			chunk.ServiceEventIndices = []uint32{uint32(chunkIndex)}
		}

		// should return one service event per chunk
		for chunkIndex := 0; chunkIndex < result.Chunks.Len(); chunkIndex++ {
			serviceEvents := result.ServiceEventsByChunk(uint64(chunkIndex))
			assert.Equal(t, result.ServiceEvents[chunkIndex:chunkIndex+1], serviceEvents)
		}
	})
}

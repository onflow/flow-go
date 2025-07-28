package flow_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
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

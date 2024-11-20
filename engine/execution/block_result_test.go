package execution

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/slices"
	"github.com/onflow/flow-go/utils/unittest"
)

// makeBlockExecutionResultFixture makes a BlockExecutionResult fixture
// with the specified allocation of service events to chunks.
func makeBlockExecutionResultFixture(serviceEventsPerChunk []int) *BlockExecutionResult {
	fixture := new(BlockExecutionResult)
	for _, nServiceEvents := range serviceEventsPerChunk {
		fixture.collectionExecutionResults = append(fixture.collectionExecutionResults,
			CollectionExecutionResult{
				serviceEvents:          unittest.EventsFixture(nServiceEvents),
				convertedServiceEvents: unittest.ServiceEventsFixture(nServiceEvents),
			})
	}
	return fixture
}

// Tests that ServiceEventIndicesForChunk method works as expected under various circumstances:
func TestBlockExecutionResult_ServiceEventIndicesForChunk(t *testing.T) {
	t.Run("no service events", func(t *testing.T) {
		nChunks := rand.Intn(10) + 1
		blockResult := makeBlockExecutionResultFixture(make([]int, nChunks))
		// all chunks should yield an empty list of service event indices
		for chunkIndex := 0; chunkIndex < nChunks; chunkIndex++ {
			indices := blockResult.ServiceEventIndicesForChunk(chunkIndex)
			assert.Equal(t, make([]uint32, 0), indices)
		}
	})
	t.Run("service events only in system chunk", func(t *testing.T) {
		nChunks := rand.Intn(10) + 2 // at least 2 chunks
		// add between 1 and 10 service events, all in the system chunk
		serviceEventAllocation := make([]int, nChunks)
		nServiceEvents := rand.Intn(10) + 1
		serviceEventAllocation[nChunks-1] = nServiceEvents

		blockResult := makeBlockExecutionResultFixture(serviceEventAllocation)
		// all non-system chunks should yield an empty list of service event indices
		for chunkIndex := 0; chunkIndex < nChunks-1; chunkIndex++ {
			indices := blockResult.ServiceEventIndicesForChunk(chunkIndex)
			assert.Equal(t, make([]uint32, 0), indices)
		}
		// the system chunk should contain indices for all events
		expected := slices.MakeRange[uint32](0, uint32(nServiceEvents))
		assert.Equal(t, expected, blockResult.ServiceEventIndicesForChunk(nChunks-1))
	})
	t.Run("service events only outside system chunk", func(t *testing.T) {
		nChunks := rand.Intn(10) + 2 // at least 2 chunks
		// add 1 service event to all non-system chunks
		serviceEventAllocation := slices.Fill(1, nChunks)
		serviceEventAllocation[nChunks-1] = 0

		blockResult := makeBlockExecutionResultFixture(serviceEventAllocation)
		// all non-system chunks should yield a length-1 list of service event indices
		for chunkIndex := 0; chunkIndex < nChunks-1; chunkIndex++ {
			indices := blockResult.ServiceEventIndicesForChunk(chunkIndex)
			// 1 service event per chunk => service event indices match chunk indices
			expected := slices.Fill(uint32(chunkIndex), 1)
			assert.Equal(t, expected, indices)
		}
		// the system chunk should contain indices for all events
		assert.Equal(t, make([]uint32, 0), blockResult.ServiceEventIndicesForChunk(nChunks-1))
	})
	t.Run("service events in both system chunk and other chunks", func(t *testing.T) {
		nChunks := rand.Intn(10) + 2 // at least 2 chunks
		// add 1 service event to all chunks (including system chunk
		serviceEventAllocation := slices.Fill(1, nChunks)

		blockResult := makeBlockExecutionResultFixture(serviceEventAllocation)
		// all  chunks should yield a length-1 list of service event indices
		for chunkIndex := 0; chunkIndex < nChunks; chunkIndex++ {
			indices := blockResult.ServiceEventIndicesForChunk(chunkIndex)
			// 1 service event per chunk => service event indices match chunk indices
			expected := slices.Fill(uint32(chunkIndex), 1)
			assert.Equal(t, expected, indices)
		}
	})
}

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

// Tests that ServiceEventCountForChunk method works as expected under various circumstances:
func TestBlockExecutionResult_ServiceEventCountForChunk(t *testing.T) {
	t.Run("no service events", func(t *testing.T) {
		nChunks := rand.Intn(10) + 1
		blockResult := makeBlockExecutionResultFixture(make([]int, nChunks))
		// all chunks should have 0 service event count
		for chunkIndex := 0; chunkIndex < nChunks; chunkIndex++ {
			count := blockResult.ServiceEventCountForChunk(chunkIndex)
			assert.Equal(t, uint16(0), count)
		}
	})
	t.Run("service events only in system chunk", func(t *testing.T) {
		nChunks := rand.Intn(10) + 2 // at least 2 chunks
		// add between 1 and 10 service events, all in the system chunk
		serviceEventAllocation := make([]int, nChunks)
		nServiceEvents := rand.Intn(10) + 1
		serviceEventAllocation[nChunks-1] = nServiceEvents

		blockResult := makeBlockExecutionResultFixture(serviceEventAllocation)
		// all non-system chunks should have zero service event count
		for chunkIndex := 0; chunkIndex < nChunks-1; chunkIndex++ {
			count := blockResult.ServiceEventCountForChunk(chunkIndex)
			assert.Equal(t, uint16(0), count)
		}
		// the system chunk should contain all service events
		assert.Equal(t, uint16(nServiceEvents), blockResult.ServiceEventCountForChunk(nChunks-1))
	})
	t.Run("service events only outside system chunk", func(t *testing.T) {
		nChunks := rand.Intn(10) + 2 // at least 2 chunks
		// add 1 service event to all non-system chunks
		serviceEventAllocation := slices.Fill(1, nChunks)
		serviceEventAllocation[nChunks-1] = 0

		blockResult := makeBlockExecutionResultFixture(serviceEventAllocation)
		// all non-system chunks should have 1 service event
		for chunkIndex := 0; chunkIndex < nChunks-1; chunkIndex++ {
			count := blockResult.ServiceEventCountForChunk(chunkIndex)
			assert.Equal(t, uint16(1), count)
		}
		// the system chunk service event count should include all service events
		assert.Equal(t, uint16(0), blockResult.ServiceEventCountForChunk(nChunks-1))
	})
	t.Run("service events in both system chunk and other chunks", func(t *testing.T) {
		nChunks := rand.Intn(10) + 2 // at least 2 chunks
		// add 1 service event to all chunks (including system chunk)
		serviceEventAllocation := slices.Fill(1, nChunks)

		blockResult := makeBlockExecutionResultFixture(serviceEventAllocation)
		// all chunks should have service event count of 1
		for chunkIndex := 0; chunkIndex < nChunks; chunkIndex++ {
			count := blockResult.ServiceEventCountForChunk(chunkIndex)
			assert.Equal(t, uint16(1), count)
		}
	})
}

package match

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// chunk can convert to job and converted back
func TestChunkToJob(t *testing.T) {
	block := unittest.BlockFixture()
	chunk := unittest.ChunkFixture(block.ID(), 0)
	actual := jobToChunk(chunkToJob(chunk))
	require.Equal(t, chunk, actual)
}

// 1. if pushing 10 jobs to chunks queue, then engine will
// receive only 3 jobs
// 2. if pushing 10 jobs to chunks queue, and engine will
// call finish will all the jobs, then engine will process
// 10 jobs in total
// 3. pushing 100 jobs concurrently, could end up having 100
// jobs processed by the consumer
func TestProduceConsume(t *testing.T) {
}

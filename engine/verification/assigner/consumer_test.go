package assigner

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestBlockToJob evaluates that a block can be converted to a job,
// and its corresponding job can be converted back to the same block.
func TestBlockToJob(t *testing.T) {
	block := unittest.BlockFixture()
	actual := jobToBlock(blockToJob(&block))
	require.Equal(t, &block, actual)
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

package verification

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

const ChunkExecutionSpanner = "chunk_execution_duration"

type MetricsConsumer struct {
	sync.RWMutex
	tracer trace.Tracer
}

func NewMetricsConsumer(t trace.Tracer) *MetricsConsumer {
	return &MetricsConsumer{
		tracer: t,
	}
}

// OnChunkVerificationStated is called whenever the verification of a chunk is started
// it starts the timer to record the execution time
func (c *MetricsConsumer) OnChunkVerificationStated(chunkID flow.Identifier) {
	// starts spanner tracer for this chunk ID
	c.tracer.StartSpan(chunkID, ChunkExecutionSpanner)
	// increases checked chunks for this block
}

// OnChunkVerificationFinished is called whenever chunkID verification gets finished
// it finishes recording the duration of execution and increases number of checked chunks for the blockID
func (c *MetricsConsumer) OnChunkVerificationFinished(chunkID flow.Identifier, blockID flow.Identifier) {
	c.tracer.FinishSpan(chunkID)
	c.Lock()
	incCheckedChecksCounter(blockID)
	c.Unlock()
}

// OnResultApproval is called whenever a result approval for block ID is emitted
// it increases the result approval counter for this chunk
func (c *MetricsConsumer) OnResultApproval(blockID flow.Identifier) {
	c.Lock()
	incResultApprovalCounter(blockID)
	c.Unlock()
}

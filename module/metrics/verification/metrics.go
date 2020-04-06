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
}

// OnChunkVerificationFinished is called whenever chunkID verification gets finished
// it finishes recording the duration of execution and increases number of checked chunks for the blockID
func (c *MetricsConsumer) OnChunkVerificationFinished(chunkID flow.Identifier, blockID flow.Identifier) {
	c.Lock()
	defer c.Unlock()
	c.tracer.FinishSpan(chunkID)
	// increases checked chunks for this block
	incCheckedChecksCounter(blockID)

}

// OnResultApproval is called whenever a result approval for block ID is emitted
// it increases the result approval counter for this chunk
func (c *MetricsConsumer) OnResultApproval(blockID flow.Identifier) {
	c.Lock()
	defer c.Unlock()
	incResultApprovalCounter(blockID)
}

// OnStorageAdded is called whenever something is added to the persistent (on disk) storage
// of verification node. It records the size of stored object.
func (c *MetricsConsumer) OnStorageAdded(size float64) {
	c.Lock()
	defer c.Unlock()
	updateTotalStorage(size)
}

// OnStorageAdded is called whenever something is removed from the persistent (on disk) storage
// of verification node. It records the size of stored object.
func (c *MetricsConsumer) OnStorageRemoved(size float64) {
	c.Lock()
	defer c.Unlock()
	if size > 0 {
		size *= -1
	}
	updateTotalStorage(size)
}

// OnChunkDataAdded is called whenever something is added to related to chunkID to the in-memory mempools
// of verification node. It records the size of stored object.
func (c *MetricsConsumer) OnChunkDataAdded(chunkID flow.Identifier, size float64) {
	c.Lock()
	defer c.Unlock()
	updateStoragePerChunk(size, chunkID)
}

// OnChunkDataRemoved is called whenever something is removed that is related to chunkID from the in-memory mempools
// of verification node. It records the size of stored object.
func (c *MetricsConsumer) OnChunkDataRemoved(chunkID flow.Identifier, size float64) {
	c.Lock()
	defer c.Unlock()
	if size > 0 {
		size *= -1
	}
	updateStoragePerChunk(size, chunkID)
}

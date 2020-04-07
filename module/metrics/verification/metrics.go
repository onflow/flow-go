package verification

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

const chunkExecutionSpanner = "chunk_execution_duration"

type Metrics struct {
	sync.RWMutex
	tracer trace.Tracer
}

func NewMetrics(t trace.Tracer) *Metrics {
	return &Metrics{
		tracer: t,
	}
}

// OnChunkVerificationStated is called whenever the verification of a chunk is started
// it starts the timer to record the execution time
func (c *Metrics) OnChunkVerificationStated(chunkID flow.Identifier) {
	// starts spanner tracer for this chunk ID
	c.tracer.StartSpan(chunkID, chunkExecutionSpanner)
}

// OnChunkVerificationFinished is called whenever chunkID verification gets finished
// it finishes recording the duration of execution and increases number of checked chunks for the blockID
func (c *Metrics) OnChunkVerificationFinished(chunkID flow.Identifier, blockID flow.Identifier) {
	c.Lock()
	defer c.Unlock()
	c.tracer.FinishSpan(chunkID)
	// increases checked chunks for this block
	incCheckedChecksCounter(blockID)

}

// OnResultApproval is called whenever a result approval for block ID is emitted
// it increases the result approval counter for this chunk
func (c *Metrics) OnResultApproval(blockID flow.Identifier) {
	c.Lock()
	defer c.Unlock()
	incResultApprovalCounter(blockID)
}

// OnStorageAdded is called whenever something is added to the persistent (on disk) storage
// of verification node. It records the size of stored object.
func (c *Metrics) OnStorageAdded(size float64) {
	c.Lock()
	defer c.Unlock()
	updateTotalStorage(size)
}

// OnStorageAdded is called whenever something is removed from the persistent (on disk) storage
// of verification node. It records the size of stored object.
func (c *Metrics) OnStorageRemoved(size float64) {
	c.Lock()
	defer c.Unlock()
	if size > 0 {
		size *= -1
	}
	updateTotalStorage(size)
}

// OnChunkDataAdded is called whenever something is added to related to chunkID to the in-memory mempools
// of verification node. It records the size of stored object.
func (c *Metrics) OnChunkDataAdded(chunkID flow.Identifier, size float64) {
	c.Lock()
	defer c.Unlock()
	updateStoragePerChunk(size, chunkID)
}

// OnChunkDataRemoved is called whenever something is removed that is related to chunkID from the in-memory mempools
// of verification node. It records the size of stored object.
func (c *Metrics) OnChunkDataRemoved(chunkID flow.Identifier, size float64) {
	c.Lock()
	defer c.Unlock()
	if size > 0 {
		size *= -1
	}
	updateStoragePerChunk(size, chunkID)
}

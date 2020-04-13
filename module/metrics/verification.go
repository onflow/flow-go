package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// a duration metrics
// time it takes to re-execute a chunk and verify its computation results
const chunkExecutionSpanner = "chunk_execution_duration"

// contains set of functions interacting with the Prometheus server
var (
	chunksCheckedPerBlock = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chunks_checked_per_block",
		Help: "The number of chunks checked per block",
	}, []string{"name"})
	resultApprovalsPerBlock = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "result_approvals_per_block",
		Help: "The number of emitted result approvals per block (i.e., number of approved chunks)",
	}, []string{"name"})
	totalStorage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "total_size",
		Help: "the duration of how long hotstuff's event loop has been busy processing one event",
	})
	storagePerChunk = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "storage_per_chunk",
		Help: "storage per chunk data",
	}, []string{"name"})
)

// VerificationMetrics implements an interface of event handlers that are called upon certain events to capture
// metrics for sake of monitoring
// OnChunkVerificationStarted is called whenever the verification of a chunk is started
// it starts the timer to record the execution time
func (c *Collector) OnChunkVerificationStarted(chunkID flow.Identifier) {
	// starts spanner tracer for this chunk ID
	c.tracer.StartSpan(chunkID, chunkExecutionSpanner)
}

// OnChunkVerificationFinished is called whenever chunkID verification gets finished
// it finishes recording the duration of execution and increases number of checked chunks for the blockID
func (c *Collector) OnChunkVerificationFinished(chunkID flow.Identifier, blockID flow.Identifier) {
	c.Lock()
	defer c.Unlock()
	c.tracer.FinishSpan(chunkID, chunkExecutionSpanner)
	// increases the checked chunks counter for blockID
	// checked chunks are the ones with a chunk data pack disseminated from
	// ingest to verifier engine
	chunksCheckedPerBlock.WithLabelValues(blockID.String()).Inc()

}

// OnResultApproval is called whenever a result approval for block ID is emitted
// it increases the result approval counter for this chunk
func (c *Collector) OnResultApproval(blockID flow.Identifier) {
	c.Lock()
	defer c.Unlock()
	// increases the counter of disseminated result approvals
	// for the blockID by one. Each result approval corresponds to a single chunk of the block
	// the approvals disseminated by verifier engine
	resultApprovalsPerBlock.WithLabelValues(blockID.String()).Inc()

}

// OnStorageAdded is called whenever something is added to the persistent (on disk) storage
// of verification node. It records the size of stored object.
func (c *Collector) OnStorageAdded(size float64) {
	c.Lock()
	defer c.Unlock()
	// updates the size of on disk storage overhead of the
	// verification node by value.
	// Todo wire this up to do monitoring
	// https://github.com/dapperlabs/flow-go/issues/3183
	totalStorage.Add(size)

}

// OnStorageAdded is called whenever something is removed from the persistent (on disk) storage
// of verification node. It records the size of stored object.
func (c *Collector) OnStorageRemoved(size float64) {
	c.Lock()
	defer c.Unlock()
	if size > 0 {
		size *= -1
	}
	// updates the size of on disk storage overhead of the
	// verification node by value.
	// Todo wire this up to do monitoring
	// https://github.com/dapperlabs/flow-go/issues/3183
	totalStorage.Add(size)
}

// OnChunkDataAdded is called whenever something is added to related to chunkID to the in-memory mempools
// of verification node. It records the size of stored object.
func (c *Collector) OnChunkDataAdded(chunkID flow.Identifier, size float64) {
	c.Lock()
	defer c.Unlock()
	// UpdateStoragePerChunk updates the size of on memory overhead of the
	// verification per chunk ID.
	// Todo wire this up to do monitoring
	// https://github.com/dapperlabs/flow-go/issues/3183
	storagePerChunk.WithLabelValues(chunkID.String()).Add(size)

}

// OnChunkDataRemoved is called whenever something is removed that is related to chunkID from the in-memory mempools
// of verification node. It records the size of stored object.
func (c *Collector) OnChunkDataRemoved(chunkID flow.Identifier, size float64) {
	c.Lock()
	defer c.Unlock()
	if size > 0 {
		size *= -1
	}
	// UpdateStoragePerChunk updates the size of on memory overhead of the
	// verification per chunk ID.
	// Todo wire this up to do monitoring
	// https://github.com/dapperlabs/flow-go/issues/3183
	storagePerChunk.WithLabelValues(chunkID.String()).Add(size)
}

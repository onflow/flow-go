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
	chunksCheckedPerBlock = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "checked_chunks_total",
		Namespace: namespaceVerification,
		Help:      "The total number of chunks checked",
	})
	resultApprovalsPerBlock = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "result_approvals_total",
		Namespace: namespaceVerification,
		Help:      "The total number of emitted result approvals",
	})

	// TODO(andrew) This metric is problematic. Label explosion and gauge sampling loss. Refactor needed
	storagePerChunk = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "verifications_storage_per_chunk",
		Namespace: namespaceVerification,
		Help:      "storage per chunk data",
	}, []string{"chunkID"})
)

// OnChunkVerificationStarted is called whenever the verification of a chunk is started
// it starts the timer to record the execution time
func (c *BaseMetrics) OnChunkVerificationStarted(chunkID flow.Identifier) {
	// starts spanner tracer for this chunk ID
	c.tracer.StartSpan(chunkID, chunkExecutionSpanner)
}

// OnChunkVerificationFinished is called whenever chunkID verification gets finished
// it finishes recording the duration of execution and increases number of checked chunks
func (c *BaseMetrics) OnChunkVerificationFinished(chunkID flow.Identifier) {
	c.tracer.FinishSpan(chunkID, chunkExecutionSpanner)
	// increases the checked chunks counter
	// checked chunks are the ones with a chunk data pack disseminated from
	// ingest to verifier engine
	chunksCheckedPerBlock.Inc()

}

// OnResultApproval is called whenever a result approval is emitted
// it increases the result approval counter for this chunk
func (c *BaseMetrics) OnResultApproval() {
	// increases the counter of disseminated result approvals
	// fo by one. Each result approval corresponds to a single chunk of the block
	// the approvals disseminated by verifier engine
	resultApprovalsPerBlock.Inc()

}

// OnChunkDataAdded is called whenever something is added to related to chunkID to the in-memory mempools
// of verification node. It records the size of stored object.
func (c *BaseMetrics) OnChunkDataAdded(chunkID flow.Identifier, size float64) {
	// UpdateStoragePerChunk updates the size of on memory overhead of the
	// verification per chunk ID.
	// Todo wire this up to do monitoring
	// https://github.com/dapperlabs/flow-go/issues/3183
	storagePerChunk.WithLabelValues(chunkID.String()).Add(size)

}

// OnChunkDataRemoved is called whenever something is removed that is related to chunkID from the in-memory mempools
// of verification node. It records the size of stored object.
func (c *BaseMetrics) OnChunkDataRemoved(chunkID flow.Identifier, size float64) {
	if size > 0 {
		size *= -1
	}
	// UpdateStoragePerChunk updates the size of on memory overhead of the
	// verification per chunk ID.
	// Todo wire this up to do monitoring
	// https://github.com/dapperlabs/flow-go/issues/3183
	storagePerChunk.WithLabelValues(chunkID.String()).Add(size)
}

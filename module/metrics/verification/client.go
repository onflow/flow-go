package verification

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

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

// IncCheckedChecks increases the checked chunks counter for blockID
// checked chunks are the ones with a chunk data pack disseminated from
// ingest to verifier engine
func incCheckedChecksCounter(blockID flow.Identifier) {
	chunksCheckedPerBlock.WithLabelValues(blockID.String()).Inc()
}

// IncResultApprovalCounter increases the counter of disseminated result approvals
// for the blockID by one. Each result approval corresponds to a single chunk of the block
// the approvals disseminated by verifier engine
func incResultApprovalCounter(blockID flow.Identifier) {
	resultApprovalsPerBlock.WithLabelValues(blockID.String()).Inc()
}

// UpdateTotalStorage updates the size of on disk storage overhead of the
// verification node by value. A positive value adds up the storage, while
// a negative value decreases it.
// Todo wire this up to do monitoring
// https://github.com/dapperlabs/flow-go/issues/3183
func updateTotalStorage(value float64) {
	totalStorage.Add(value)
}

// UpdateStoragePerChunk updates the size of on memory overhead of the
// verification per chunk ID. A positive value adds up the memory overhead, while
// a negative value decreases it.
// Todo wire this up to do monitoring
// https://github.com/dapperlabs/flow-go/issues/3183
func updateStoragePerChunk(value float64, chunkID flow.Identifier) {
	storagePerChunk.WithLabelValues(chunkID.String()).Add(value)
}

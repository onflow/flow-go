package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

// Verification spans.
const chunkExecutionSpanner = "chunk_execution_duration"

type VerificationCollector struct {
	tracer                      *trace.OpenTracer
	chunksCheckedPerBlock       prometheus.Counter
	resultApprovalsPerBlock     prometheus.Counter
	storagePerChunk             prometheus.Gauge
	pendingCollectionsNum       prometheus.Gauge
	authenticatedCollectionsNum prometheus.Gauge
	pendingReceiptsNum          prometheus.Gauge
	authenticatedReceiptsNum    prometheus.Gauge
	chunkTrackersNum            prometheus.Gauge
	chunkDataPacksNum           prometheus.Gauge
}

func NewVerificationCollector(tracer *trace.OpenTracer) *VerificationCollector {

	vc := &VerificationCollector{
		tracer: tracer,

		chunksCheckedPerBlock: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "checked_chunks_total",
			Namespace: namespaceVerification,
			Help:      "total number of chunks checked",
		}),

		resultApprovalsPerBlock: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "result_approvals_total",
			Namespace: namespaceVerification,
			Help:      "total number of emitted result approvals",
		}),

		storagePerChunk: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "storage_latest_chunk_size_bytes",
			Namespace: namespaceVerification,
			Help:      "latest ingested chunk resources storage (bytes)",
		}),

		pendingCollectionsNum: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "pending_collections_latest_number_collections",
			Namespace: namespaceVerification,
			Help:      "latest number of pending collections in mempool",
		}),

		authenticatedCollectionsNum: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "authenticated_collections_latest_number_collections",
			Namespace: namespaceVerification,
			Help:      "latest number of authenticated collections in mempool",
		}),

		pendingReceiptsNum: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "pending_receipts_latest_number_receipts",
			Namespace: namespaceVerification,
			Help:      "latest number of pending receipts in mempool",
		}),

		authenticatedReceiptsNum: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "authenticated_receipts_latest_number_receipts",
			Namespace: namespaceVerification,
			Help:      "latest number of authenticated receipts in mempool",
		}),

		chunkDataPacksNum: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "chunk_data_packs_latest_number_chunks",
			Namespace: namespaceVerification,
			Help:      "latest number of chunk data packs in mempool",
		}),

		chunkTrackersNum: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "chunk_trackers_number_latest_number_trackers",
			Namespace: namespaceVerification,
			Help:      "latest number of chunk trackers",
		}),
	}

	return vc
}

// OnResultApproval is called whenever a result approval is emitted.
// It increases the result approval counter for this chunk.
func (vc *VerificationCollector) OnResultApproval() {
	// increases the counter of disseminated result approvals
	// fo by one. Each result approval corresponds to a single chunk of the block
	// the approvals disseminated by verifier engine
	vc.resultApprovalsPerBlock.Inc()

}

// OnVerifiableChunkSubmitted is called whenever a verifiable chunk is shaped for a specific
// chunk. It adds the size of the verifiable chunk to the histogram. A verifiable chunk is assumed
// to capture all the resources needed to verify a chunk.
// The purpose of this function is to track the overall chunk resources size on disk.
// Todo wire this up to do monitoring
// https://github.com/dapperlabs/flow-go/issues/3183
func (vc *VerificationCollector) OnVerifiableChunkSubmitted(size float64) {
	vc.storagePerChunk.Set(size)
}

// OnAuthenticatedReceiptsUpdated is called whenever size of AuthenticatedReceipts mempool gets changed.
// It records the latest value of its size.
func (vc *VerificationCollector) OnAuthenticatedReceiptsUpdated(size uint) {
	vc.authenticatedReceiptsNum.Set(float64(size))
}

// OnPendingReceiptsUpdated is called whenever size of PendingReceipts mempool gets changed.
// It records the latest value of its size.
func (vc *VerificationCollector) OnPendingReceiptsUpdated(size uint) {
	vc.pendingReceiptsNum.Set(float64(size))
}

// OnAuthenticatedCollectionsUpdated is called whenever size of AuthenticatedCollections mempool gets changed.
// It records the latest value of its size.
func (vc *VerificationCollector) OnAuthenticatedCollectionsUpdated(size uint) {
	vc.authenticatedCollectionsNum.Set(float64(size))
}

// OnChunkDataPacksUpdated is called whenever size of ChunkDataPacks mempool gets changed.
// It records the latest value of its size.
func (vc *VerificationCollector) OnChunkDataPacksUpdated(size uint) {
	vc.chunkDataPacksNum.Set(float64(size))
}

// OnPendingCollectionsUpdated is called whenever size of PendingCollections mempool gets changed.
// It records the latest value of its size.
func (vc *VerificationCollector) OnPendingCollectionsUpdated(size uint) {
	vc.pendingCollectionsNum.Set(float64(size))
}

// OnChunkTrackersUpdated is called whenever size of ChunkTrackers mempool gets changed.
// It records the latest value of its size.
func (vc *VerificationCollector) OnChunkTrackersUpdated(size uint) {
	vc.chunkTrackersNum.Set(float64(size))
}

// OnChunkVerificationStarted is called whenever the verification of a chunk is started.
// It starts the timer to record the execution time.
func (vc *VerificationCollector) OnChunkVerificationStarted(chunkID flow.Identifier) {
	// starts spanner tracer for this chunk ID
	vc.tracer.StartSpan(chunkID, chunkExecutionSpanner)
}

// OnChunkVerificationFinished is called whenever chunkID verification gets finished.
// It finishes recording the duration of execution and increases number of checked chunks.
func (vc *VerificationCollector) OnChunkVerificationFinished(chunkID flow.Identifier) {
	vc.tracer.FinishSpan(chunkID, chunkExecutionSpanner)
	// increases the checked chunks counter
	// checked chunks are the ones with a chunk data pack disseminated from
	// ingest to verifier engine
	vc.chunksCheckedPerBlock.Inc()

}

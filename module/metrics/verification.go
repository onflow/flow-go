package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

type VerificationCollector struct {
	tracer module.Tracer

	// Assigner Engine
	rcvBlocksTotal      prometheus.Counter // total finalized blocks received by assigner engine
	assignedChunksTotal prometheus.Counter // total chunks assigned to this verification node
	sntChunksTotal      prometheus.Counter // total chunks sent by assigner engine to chunk consumer (i.e., fetcher input)

	// Finder Engine
	rcvReceiptsTotal         prometheus.Counter // total execution receipts arrived at finder engine
	sntExecutionResultsTotal prometheus.Counter // total execution results processed by finder engine

	// Match Engine
	rcvExecutionResultsTotal prometheus.Counter // total execution results received by match engine
	sntVerifiableChunksTotal prometheus.Counter // total chunks matched by match engine and sent to verifier engine
	rcvChunkDataPackTotal    prometheus.Counter // total chunk data packs received by match engine
	reqChunkDataPackTotal    prometheus.Counter // total number of chunk data packs requested by match engine

	// Verifier Engine
	rcvVerifiableChunksTotal prometheus.Counter // total verifiable chunks received by verifier engine
	resultApprovalsTotal     prometheus.Counter // total result approvals sent by verifier engine

}

func NewVerificationCollector(tracer module.Tracer, registerer prometheus.Registerer) *VerificationCollector {
	// Assigner Engine
	rcvBlocksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "block_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number of finalized blocks received by assigner engine",
	})

	assignedChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_assigned_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number of chunks assigned to verification node",
	})

	sntChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "processed_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number chunks sent by assigner engine to chunk consumer",
	})

	// till new pipeline of assigner-fetcher-verifier is in place, we need to keep these metrics
	// as they are. Though assigner and finder share this rcvReceiptsTotal, which should be refactored
	// later.
	// TODO rename name space to assigner
	// Finder (and Assigner) Engine
	rcvReceiptsTotals := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_receipt_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFinderEngine,
		Help:      "total number of execution receipts received by finder engine",
	})

	// TODO remove metric once finer removed
	sntExecutionResultsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFinderEngine,
		Help:      "total number of execution results sent by finder engine to match engine",
	})

	// Match Engine
	rcvExecutionResultsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of execution results received by match engine from finder engine",
	})

	sntVerifiableChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of verifiable chunks sent by match engine to verifier engine",
	})

	rcvChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of chunk data packs received by match engine",
	})

	reqChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_requested_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of chunk data packs requested by match engine",
	})

	// Verifier Engine
	rcvVerifiableChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number verifiable chunks received by verifier engine from match engine",
	})

	sntResultApprovalTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "result_approvals_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number of emitted result approvals by verifier engine",
	})

	// registers all metrics and panics if any fails.
	registerer.MustRegister(
		rcvBlocksTotal,
		assignedChunksTotal,
		sntChunksTotal,
		rcvReceiptsTotals,
		sntExecutionResultsTotal,
		rcvExecutionResultsTotal,
		sntVerifiableChunksTotal,
		rcvChunkDataPackTotal,
		reqChunkDataPackTotal,
		rcvVerifiableChunksTotal,
		sntResultApprovalTotal)

	vc := &VerificationCollector{
		tracer:                   tracer,
		rcvBlocksTotal:           rcvBlocksTotal,
		assignedChunksTotal:      assignedChunksTotal,
		sntChunksTotal:           sntChunksTotal,
		rcvReceiptsTotal:         rcvReceiptsTotals,
		sntExecutionResultsTotal: sntExecutionResultsTotal,
		rcvExecutionResultsTotal: rcvExecutionResultsTotal,
		sntVerifiableChunksTotal: sntVerifiableChunksTotal,
		rcvVerifiableChunksTotal: rcvVerifiableChunksTotal,
		resultApprovalsTotal:     sntResultApprovalTotal,
		rcvChunkDataPackTotal:    rcvChunkDataPackTotal,
		reqChunkDataPackTotal:    reqChunkDataPackTotal,
	}

	return vc
}

// OnExecutionReceiptReceived is called whenever a new execution receipt arrives
// at Finder engine. It increments total number of received receipts.
func (vc *VerificationCollector) OnExecutionReceiptReceived() {
	vc.rcvReceiptsTotal.Inc()
}

// OnExecutionResultSent is called whenever a new execution result is sent by
// Finder engine to the match engine. It increments total number of sent execution results.
func (vc *VerificationCollector) OnExecutionResultSent() {
	vc.sntExecutionResultsTotal.Inc()
}

// OnExecutionResultReceived is called whenever a new execution result is successfully received
// by Match engine from Finder engine.
// It increments the total number of received execution results.
func (vc *VerificationCollector) OnExecutionResultReceived() {
	vc.rcvExecutionResultsTotal.Inc()
}

// OnVerifiableChunkSent is called on a successful submission of matched chunk
// by Match engine to Verifier engine.
// It increments the total number of chunks matched by match engine.
func (vc *VerificationCollector) OnVerifiableChunkSent() {
	vc.sntVerifiableChunksTotal.Inc()
}

// OnChunkDataPackReceived is called on a receiving a chunk data pack by Match engine
// It increments the total number of chunk data packs received.
func (vc *VerificationCollector) OnChunkDataPackReceived() {
	vc.rcvChunkDataPackTotal.Inc()
}

// OnChunkDataPackRequested is called on requesting a chunk data pack by Match engine
// It increments the total number of chunk data packs requested.
func (vc *VerificationCollector) OnChunkDataPackRequested() {
	vc.reqChunkDataPackTotal.Inc()
}

// OnVerifiableChunkReceived is called whenever a verifiable chunk is received by Verifier engine
// from Match engine.It increments the total number of sent verifiable chunks.
func (vc *VerificationCollector) OnVerifiableChunkReceived() {
	vc.rcvVerifiableChunksTotal.Inc()
}

// OnResultApproval is called whenever a result approval for is emitted to consensus nodes.
// It increases the total number of result approvals.
func (vc *VerificationCollector) OnResultApproval() {
	// increases the counter of disseminated result approvals
	// fo by one. Each result approval corresponds to a single chunk of the block
	// the approvals disseminated by verifier engine
	vc.resultApprovalsTotal.Inc()
}

// OnFinalizedBlockReceived is called whenever a finalized block arrives at the assigner engine.
// It increments the total number of finalized blocks.
func (vc *VerificationCollector) OnFinalizedBlockReceived() {
	vc.rcvBlocksTotal.Inc()
}

// OnChunksAssigned is called whenever chunks assigned to this verification node by applying chunk assignment on an
// execution result.
// It increases the total number of assigned chunks by the input.
func (vc *VerificationCollector) OnChunksAssigned(chunks int) {
	vc.assignedChunksTotal.Add(float64(chunks))
}

// OnChunkProcessed is called whenever a chunk is pushed to the chunks queue by the assigner engine.
// It increments the total number of sent chunks.
func (vc *VerificationCollector) OnChunkProcessed() {
	vc.sntChunksTotal.Inc()
}

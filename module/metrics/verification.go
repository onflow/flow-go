package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

type VerificationCollector struct {
	tracer module.Tracer

	// Assigner Engine
	receivedFinalizedHeight prometheus.Gauge   // the last finalized height received by assigner engine
	assignedChunksTotal     prometheus.Counter // total chunks assigned to this verification node
	sentChunksTotal         prometheus.Counter // total chunks sent by assigner engine to chunk consumer (i.e., fetcher input)

	// Finder Engine
	receivedReceiptsTotal     prometheus.Counter // total execution receipts arrived at finder engine
	sentExecutionResultsTotal prometheus.Counter // total execution results processed by finder engine

	// Match Engine
	receivedExecutionResultsTotal prometheus.Counter // total execution results received by match engine
	sntVerifiableChunksTotal      prometheus.Counter // total chunks matched by match engine and sent to verifier engine
	receivedChunkDataPackTotal    prometheus.Counter // total chunk data packs received by match engine
	requestedChunkDataPackTotal   prometheus.Counter // total number of chunk data packs requested by match engine

	// Verifier Engine
	receivedVerifiableChunksTotal prometheus.Counter // total verifiable chunks received by verifier engine
	resultApprovalsTotal          prometheus.Counter // total result approvals sent by verifier engine

}

func NewVerificationCollector(tracer module.Tracer, registerer prometheus.Registerer) *VerificationCollector {
	// Assigner Engine
	receivedFinalizedHeight := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "finalized_height",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "the last finalized height received by assigner engine",
	})

	assignedChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_assigned_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number of chunks assigned to verification node",
	})

	sentChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "processed_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number chunks sent by assigner engine to chunk consumer",
	})

	// till new pipeline of assigner-fetcher-verifier is in place, we need to keep these metrics
	// as they are. Though assigner and finder share this receivedReceiptsTotal, which should be refactored
	// later.
	// TODO rename name space to assigner
	// Finder (and Assigner) Engine
	receivedReceiptsTotals := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_receipt_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFinderEngine,
		Help:      "total number of execution receipts received by finder engine",
	})

	// TODO remove metric once finer removed
	sentExecutionResultsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFinderEngine,
		Help:      "total number of execution results sent by finder engine to match engine",
	})

	// Match Engine
	receivedExecutionResultsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of execution results received by match engine from finder engine",
	})

	sentVerifiableChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of verifiable chunks sent by match engine to verifier engine",
	})

	receivedChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of chunk data packs received by match engine",
	})

	requestedChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_requested_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of chunk data packs requested by match engine",
	})

	// Verifier Engine
	receivedVerifiableChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number verifiable chunks received by verifier engine from match engine",
	})

	sentResultApprovalTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "result_approvals_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number of emitted result approvals by verifier engine",
	})

	// registers all metrics and panics if any fails.
	registerer.MustRegister(
		receivedFinalizedHeight,
		assignedChunksTotal,
		sentChunksTotal,
		receivedReceiptsTotals,
		sentExecutionResultsTotal,
		receivedExecutionResultsTotal,
		sentVerifiableChunksTotal,
		receivedChunkDataPackTotal,
		requestedChunkDataPackTotal,
		receivedVerifiableChunksTotal,
		sentResultApprovalTotal)

	vc := &VerificationCollector{
		tracer:                        tracer,
		receivedFinalizedHeight:       receivedFinalizedHeight,
		assignedChunksTotal:           assignedChunksTotal,
		sentChunksTotal:               sentChunksTotal,
		receivedReceiptsTotal:         receivedReceiptsTotals,
		sentExecutionResultsTotal:     sentExecutionResultsTotal,
		receivedExecutionResultsTotal: receivedExecutionResultsTotal,
		sntVerifiableChunksTotal:      sentVerifiableChunksTotal,
		receivedVerifiableChunksTotal: receivedVerifiableChunksTotal,
		resultApprovalsTotal:          sentResultApprovalTotal,
		receivedChunkDataPackTotal:    receivedChunkDataPackTotal,
		requestedChunkDataPackTotal:   requestedChunkDataPackTotal,
	}

	return vc
}

// OnExecutionReceiptReceived is called whenever a new execution receipt arrives
// at Finder engine. It increments total number of received receipts.
func (vc *VerificationCollector) OnExecutionReceiptReceived() {
	vc.receivedReceiptsTotal.Inc()
}

// OnExecutionResultSent is called whenever a new execution result is sent by
// Finder engine to the match engine. It increments total number of sent execution results.
func (vc *VerificationCollector) OnExecutionResultSent() {
	vc.sentExecutionResultsTotal.Inc()
}

// OnExecutionResultReceived is called whenever a new execution result is successfully received
// by Match engine from Finder engine.
// It increments the total number of received execution results.
func (vc *VerificationCollector) OnExecutionResultReceived() {
	vc.receivedExecutionResultsTotal.Inc()
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
	vc.receivedChunkDataPackTotal.Inc()
}

// OnChunkDataPackRequested is called on requesting a chunk data pack by Match engine
// It increments the total number of chunk data packs requested.
func (vc *VerificationCollector) OnChunkDataPackRequested() {
	vc.requestedChunkDataPackTotal.Inc()
}

// OnVerifiableChunkReceived is called whenever a verifiable chunk is received by Verifier engine
// from Match engine.It increments the total number of sent verifiable chunks.
func (vc *VerificationCollector) OnVerifiableChunkReceived() {
	vc.receivedVerifiableChunksTotal.Inc()
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
// It sets updates the latest finalized height processed at assigner engine.
//
// Note: it assumes blocks are coming to assigner engine in strictly increasing order of their height.
func (vc *VerificationCollector) OnAssignerProcessFinalizedBlock(height uint64) {
	vc.receivedFinalizedHeight.Set(float64(height))
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
	vc.sentChunksTotal.Inc()
}

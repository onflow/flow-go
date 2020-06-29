package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

// Verification spans.
const chunkExecutionSpanner = "chunk_execution_duration"

type VerificationCollector struct {
	tracer          *trace.OpenTracer
	storagePerChunk prometheus.Gauge // storage per chunk

	// Finder Engine
	rcvReceiptsTotal         prometheus.Counter // total execution receipts arrived at finder engine
	sntExecutionResultsTotal prometheus.Counter // total execution results processed by finder engine

	// Match Engine
	rcvExecutionResultsTotal prometheus.Counter // total execution results received by match engine
	sntVerifiableChunksTotal prometheus.Counter // total chunks matched by match engine and sent to verifier engine
	rcvChunkDataPack         prometheus.Counter // total chunk data packs received by match engine

	// Verifier Engine
	rcvVerifiableChunksTotal prometheus.Counter // total verifiable chunks received by verifier engine
	resultApprovalsTotal     prometheus.Counter // total result approvals sent by verifier engine

}

func NewVerificationCollector(tracer *trace.OpenTracer, registerer prometheus.Registerer, log zerolog.Logger) *VerificationCollector {

	// Finder Engine
	rcvReceiptsTotals := promauto.NewCounter(prometheus.CounterOpts{
		Name:      "execution_receipt_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFinderEngine,
		Help:      "total number of execution receipts received by finder engine",
	})
	err := registerer.Register(rcvReceiptsTotals)
	if err != nil {
		log.Debug().Err(err).Msg("could not register rcvReceiptsTotals metric")
	}

	sntExecutionResultsTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFinderEngine,
		Help:      "total number of execution results sent by finder engine to match engine",
	})
	err = registerer.Register(sntExecutionResultsTotal)
	if err != nil {
		log.Debug().Err(err).Msg("could not register sntExecutionResultsTotal metric")
	}

	// Match Engine
	rcvExecutionResultsTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of execution results received by match engine from finder engine",
	})
	err = registerer.Register(rcvExecutionResultsTotal)
	if err != nil {
		log.Debug().Err(err).Msg("could not register rcvExecutionResultsTotal) metric")
	}

	sntVerifiableChunksTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of verifiable chunks sent by match engine to verifier engine",
	})
	err = registerer.Register(sntVerifiableChunksTotal)
	if err != nil {
		log.Debug().Err(err).Msg("could not register sntVerifiableChunksTotal metric")
	}

	rcvChunkDataPack := promauto.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of chunk data packs received by match engine",
	})
	err = registerer.Register(rcvChunkDataPack)
	if err != nil {
		log.Debug().Err(err).Msg("could not register rcvChunkDataPack metric")
	}

	// Verifier Engine
	rcvVerifiableChunksTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number verifiable chunks received by verifier engine from match engine",
	})
	err = registerer.Register(rcvVerifiableChunksTotal)
	if err != nil {
		log.Debug().Err(err).Msg("could not register rcvVerifiableChunksTotal metric")
	}

	sntResultApprovalTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name:      "result_approvals_total",
		Namespace: namespaceVerification,
		Help:      "total number of emitted result approvals by verifier engine",
	})
	err = registerer.Register(sntResultApprovalTotal)
	if err != nil {
		log.Debug().Err(err).Msg("could not register sntResultApprovalTotal metric")
	}

	// Storage
	storagePerChunk := promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "storage_latest_chunk_size_bytes",
		Namespace: namespaceVerification,
		Help:      "latest ingested chunk resources storage (bytes)",
	})
	err = registerer.Register(storagePerChunk)
	if err != nil {
		log.Debug().Err(err).Msg("could not register storagePerChunk metric")
	}

	vc := &VerificationCollector{
		tracer:                   tracer,
		rcvReceiptsTotal:         rcvReceiptsTotals,
		sntExecutionResultsTotal: sntExecutionResultsTotal,
		rcvExecutionResultsTotal: rcvExecutionResultsTotal,
		sntVerifiableChunksTotal: sntVerifiableChunksTotal,
		rcvVerifiableChunksTotal: rcvVerifiableChunksTotal,
		resultApprovalsTotal:     sntResultApprovalTotal,
		storagePerChunk:          storagePerChunk,
		rcvChunkDataPack:         rcvChunkDataPack,
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
	vc.rcvChunkDataPack.Inc()
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

// OnChunkVerificationStarted is called whenever the verification of a chunk is started.
// It starts the timer to record the execution time.
func (vc *VerificationCollector) OnChunkVerificationStarted(chunkID flow.Identifier) {
	// starts spanner tracer for this chunk ID
	vc.tracer.StartSpan(chunkID, chunkExecutionSpanner)
}

// OnChunkVerificationFinished is called whenever chunkID verification gets finished.
// It finishes recording the duration of execution.
func (vc *VerificationCollector) OnChunkVerificationFinished(chunkID flow.Identifier) {
	vc.tracer.FinishSpan(chunkID, chunkExecutionSpanner)
}

// LogVerifiableChunkSize is called whenever a verifiable chunk is shaped for a specific
// chunk. It adds the size of the verifiable chunk to the histogram. A verifiable chunk is assumed
// to capture all the resources needed to verify a chunk.
// The purpose of this function is to track the overall chunk resources size on disk.
// Todo wire this up to do monitoring (3183)
func (vc *VerificationCollector) LogVerifiableChunkSize(size float64) {
	vc.storagePerChunk.Set(size)
}

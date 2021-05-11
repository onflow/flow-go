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

	// Fetcher Engine
	//
	// total assigned chunks received by fetcher engine from assigner engine (through chunk consumer),
	receivedAssignedChunksTotal prometheus.Counter

	// Requester Engine
	//
	// total number of chunk data pack requests received by requester engine from fetcher engine.
	receivedChunkDataPackRequestsTotal prometheus.Counter
	// total number of chunk data request messages dispatched in the network by requester engine.
	sentChunkDataRequestMessagesTotal prometheus.Counter
	// total number of chunk data response messages received by requester from network.
	receivedChunkDataResponseMessagesTotal prometheus.Counter
	// total number of chunk data pack sent by requester to fetcher engine.
	sentChunkDataPackTotal prometheus.Counter

	// Finder Engine
	receivedReceiptsTotal     prometheus.Counter // total execution receipts arrived at finder engine
	sentExecutionResultsTotal prometheus.Counter // total execution results processed by finder engine

	// Match Engine
	receivedExecutionResultsTotal prometheus.Counter // total execution results received by match engine
	sentVerifiableChunksTotal     prometheus.Counter // total chunks matched by match engine and sent to verifier engine
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

	// Fetcher Engine
	receivedAssignedChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "assigned_chunk_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of chunks received by fetcher engine from assigner engine through chunk consumer",
	})

	// Requester Engine
	receivedChunkDataPackRequestsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_request_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data pack requests received by requester engine from fetcher engine",
	})

	sentChunkDataRequestMessagesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_request_message_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data pack request messages sent in the network by requester engine",
	})

	receivedChunkDataResponseMessagesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_response_message_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data response messages received from network by requester engine",
	})

	sentChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data packs sent by requester engine to fetcher engine",
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

	// TODO remove metric once finder removed
	sentExecutionResultsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFinderEngine,
		Help:      "total number of execution results sent by finder engine to match engine",
	})

	// Match Engine
	// TODO remove metric once match removed
	receivedExecutionResultsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "execution_result_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of execution results received by match engine from finder engine",
	})

	// TODO rename subsystem and help to fetcher engine once match removed
	sentVerifiableChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of verifiable chunks sent by match engine to verifier engine",
	})

	// TODO rename subsystem and help to fetcher engine once match removed
	receivedChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of chunk data packs received by match engine",
	})

	// TODO rename subsystem and help to fetcher engine once match removed
	requestedChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_requested_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemMatchEngine,
		Help:      "total number of chunk data packs requested by match engine",
	})

	// Verifier Engine
	// TODO refactor helper from match engine to fetcher engine once match engine removed
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
		// assigner
		receivedFinalizedHeight,
		assignedChunksTotal,
		sentChunksTotal,

		// fetcher engine
		receivedAssignedChunksTotal,

		// requester engine
		receivedChunkDataPackRequestsTotal,
		sentChunkDataRequestMessagesTotal,
		receivedChunkDataResponseMessagesTotal,
		sentChunkDataPackTotal,

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
		receivedReceiptsTotal:         receivedReceiptsTotals,
		sentExecutionResultsTotal:     sentExecutionResultsTotal,
		receivedExecutionResultsTotal: receivedExecutionResultsTotal,
		sentVerifiableChunksTotal:     sentVerifiableChunksTotal,
		receivedVerifiableChunksTotal: receivedVerifiableChunksTotal,
		resultApprovalsTotal:          sentResultApprovalTotal,
		receivedChunkDataPackTotal:    receivedChunkDataPackTotal,
		requestedChunkDataPackTotal:   requestedChunkDataPackTotal,

		// assigner
		receivedFinalizedHeight: receivedFinalizedHeight,
		assignedChunksTotal:     assignedChunksTotal,
		sentChunksTotal:         sentChunksTotal,

		// fetcher
		receivedAssignedChunksTotal: receivedAssignedChunksTotal,

		// requester
		receivedChunkDataPackRequestsTotal:     receivedChunkDataPackRequestsTotal,
		sentChunkDataRequestMessagesTotal:      sentChunkDataRequestMessagesTotal,
		receivedChunkDataResponseMessagesTotal: receivedChunkDataResponseMessagesTotal,
		sentChunkDataPackTotal:                 sentChunkDataPackTotal,
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
// TODO: remove this method once fetcher is removed.
func (vc *VerificationCollector) OnVerifiableChunkSent() {
	vc.sentVerifiableChunksTotal.Inc()
}

// OnChunkDataPackReceived is called on a receiving a chunk data pack by Match engine
// It increments the total number of chunk data packs received.
// TODO: remove this method once fetcher is removed.
func (vc *VerificationCollector) OnChunkDataPackReceived() {
	vc.receivedChunkDataPackTotal.Inc()
}

// OnChunkDataPackRequested is called on requesting a chunk data pack by Match engine
// It increments the total number of chunk data packs requested.
// TODO: remove this method once fetcher is removed.
func (vc *VerificationCollector) OnChunkDataPackRequested() {
	vc.requestedChunkDataPackTotal.Inc()
}

// OnVerifiableChunkReceivedAtVerifierEngine is called whenever a verifiable chunk is received by Verifier engine
// from Match engine.It increments the total number of sent verifiable chunks.
func (vc *VerificationCollector) OnVerifiableChunkReceivedAtVerifierEngine() {
	vc.receivedVerifiableChunksTotal.Inc()
}

// OnResultApprovalDispatchedInNetwork is called whenever a result approval for is emitted to consensus nodes.
// It increases the total number of result approvals.
func (vc *VerificationCollector) OnResultApprovalDispatchedInNetwork() {
	// increases the counter of disseminated result approvals
	// fo by one. Each result approval corresponds to a single chunk of the block
	// the approvals disseminated by verifier engine
	vc.resultApprovalsTotal.Inc()
}

// OnFinalizedBlockArrivesAtAssigner sets a gauge that keeps track of number of the latest block height arrives
// at assigner engine. Note that it assumes blocks are coming to assigner engine in strictly increasing order of their height.
func (vc *VerificationCollector) OnFinalizedBlockArrivesAtAssigner(height uint64) {
	vc.receivedFinalizedHeight.Set(float64(height))
}

// OnChunksAssignmentDoneAtAssigner increments a counter that keeps track of the total number of assigned chunks to
// the verification node.
func (vc *VerificationCollector) OnChunksAssignmentDoneAtAssigner(chunks int) {
	vc.assignedChunksTotal.Add(float64(chunks))
}

// OnAssignedChunkProcessedAtAssigner increments a counter that keeps track of the total number of assigned chunks pushed by
// assigner engine to the fetcher engine.
func (vc *VerificationCollector) OnAssignedChunkProcessedAtAssigner() {
	vc.sentChunksTotal.Inc()
}

// OnAssignedChunkReceivedAtFetcher increments a counter that keeps track of number of assigned chunks arrive at fetcher engine.
func (vc *VerificationCollector) OnAssignedChunkReceivedAtFetcher() {
	vc.receivedAssignedChunksTotal.Inc()
}

// OnChunkDataPackRequestSentByFetcher increments a counter that keeps track of number of chunk data pack requests that fetcher engine
// sends to requester engine.
func (vc *VerificationCollector) OnChunkDataPackRequestSentByFetcher() {
	vc.requestedChunkDataPackTotal.Inc()
}

// OnChunkDataPackRequestReceivedByRequester increments a counter that keeps track of number of chunk data pack requests arrive at
// arrive at the requester engine from the fetcher engine.
func (vc *VerificationCollector) OnChunkDataPackRequestReceivedByRequester() {
	vc.receivedChunkDataPackRequestsTotal.Inc()
}

// OnChunkDataPackRequestDispatchedInNetwork increments a counter that keeps track of number of chunk data pack requests that the
// requester engine dispatches in the network (to the execution nodes).
func (vc *VerificationCollector) OnChunkDataPackRequestDispatchedInNetwork() {
	vc.sentChunkDataRequestMessagesTotal.Inc()
}

// OnChunkDataPackResponseReceivedFromNetwork increments a counter that keeps track of number of chunk data pack responses that the
// requester engine receives from execution nodes (through network).
func (vc *VerificationCollector) OnChunkDataPackResponseReceivedFromNetwork() {
	vc.receivedChunkDataResponseMessagesTotal.Inc()
}

// OnChunkDataPackSentToFetcher increases a counter that keeps track of number of chunk data packs sent to the fetcher engine from
// requester engine.
func (vc *VerificationCollector) OnChunkDataPackSentToFetcher() {
	vc.sentChunkDataPackTotal.Inc()
}

// OnChunkDataPackArrivedAtFetcher increments a counter that keeps track of number of chunk data packs arrived at fetcher engine from
// requester engine.
func (vc *VerificationCollector) OnChunkDataPackArrivedAtFetcher() {
	vc.receivedChunkDataPackTotal.Inc()
}

// OnVerifiableChunkSentToVerifier increments a counter that keeps track of number of verifiable chunks fetcher engine sent to verifier engine.
func (vc *VerificationCollector) OnVerifiableChunkSentToVerifier() {
	vc.sentVerifiableChunksTotal.Inc()
}

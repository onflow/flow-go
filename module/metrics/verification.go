package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

type VerificationCollector struct {
	tracer module.Tracer

	// Assigner Engine
	receivedFinalizedHeightAssigner prometheus.Gauge   // the last finalized height received by assigner engine
	assignedChunkTotalAssigner      prometheus.Counter // total chunks assigned to this verification node
	processedChunkTotalAssigner     prometheus.Counter // total chunks sent by assigner engine to chunk consumer (i.e., fetcher input)
	receivedResultsByAssignerTotal  prometheus.Counter // total execution results arrived at assigner

	// Fetcher Engine
	receivedAssignedChunkByFetcherTotal  prometheus.Counter // total assigned chunks received by fetcher engine from assigner engine.
	sentVerifiableChunksByFetcherTotal   prometheus.Counter // total verifiable chunk sent by fetcher engine and sent to verifier engine.
	receivedChunkDataPackByFetcherTotal  prometheus.Counter // total chunk data packs received by fetcher engine
	requestedChunkDataPackByFetcherTotal prometheus.Counter // total number of chunk data packs requested by fetcher engine

	// Requester Engine
	//
	// total number of chunk data pack requests received by requester engine from fetcher engine.
	receivedChunkDataPackRequestByRequesterTotal prometheus.Counter
	// total number of chunk data request messages dispatched in the network by requester engine.
	sentChunkDataRequestMessageByRequesterTotal prometheus.Counter
	// total number of chunk data response messages received by requester from network.
	receivedChunkDataResponseMessageByRequesterTotal prometheus.Counter
	// total number of chunk data pack sent by requester to fetcher engine.
	sentChunkDataPackByRequesterTotal prometheus.Counter

	// Verifier Engine
	receivedVerifiableChunkTotalVerifier prometheus.Counter // total verifiable chunks received by verifier engine
	sentResultApprovalTotalVerifier      prometheus.Counter // total result approvals sent by verifier engine

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
	receivedChunkDataPackRequestsByRequesterTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_request_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data pack requests received by requester engine from fetcher engine",
	})

	sentChunkDataRequestMessagesByRequesterTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_request_message_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data pack request messages sent in the network by requester engine",
	})

	receivedChunkDataResponseMessagesByRequesterTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_response_message_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data response messages received from network by requester engine",
	})

	sentChunkDataPackByRequesterTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data packs sent by requester engine to fetcher engine",
	})

	sentVerifiableChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of verifiable chunks sent by fetcher engine to verifier engine",
	})

	receivedChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of chunk data packs received by fetcher engine",
	})

	requestedChunkDataPackTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_requested_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of chunk data packs requested by fetcher engine",
	})

	// Verifier Engine
	receivedVerifiableChunksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number verifiable chunks received by verifier engine from fetcher engine",
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
		receivedChunkDataPackRequestsByRequesterTotal,
		sentChunkDataRequestMessagesByRequesterTotal,
		receivedChunkDataResponseMessagesByRequesterTotal,
		sentChunkDataPackByRequesterTotal,

		sentVerifiableChunksTotal,
		receivedChunkDataPackTotal,
		requestedChunkDataPackTotal,
		receivedVerifiableChunksTotal,
		sentResultApprovalTotal)

	vc := &VerificationCollector{
		tracer: tracer,

		// assigner
		receivedFinalizedHeightAssigner: receivedFinalizedHeight,
		assignedChunkTotalAssigner:      assignedChunksTotal,
		processedChunkTotalAssigner:     sentChunksTotal,

		// fetcher
		receivedAssignedChunkByFetcherTotal:  receivedAssignedChunksTotal,
		receivedChunkDataPackByFetcherTotal:  receivedChunkDataPackTotal,
		requestedChunkDataPackByFetcherTotal: requestedChunkDataPackTotal,
		sentVerifiableChunksByFetcherTotal:   sentVerifiableChunksTotal,

		// verifier
		sentResultApprovalTotalVerifier:      sentResultApprovalTotal,
		receivedVerifiableChunkTotalVerifier: receivedVerifiableChunksTotal,

		// requester
		receivedChunkDataPackRequestByRequesterTotal:     receivedChunkDataPackRequestsByRequesterTotal,
		sentChunkDataRequestMessageByRequesterTotal:      sentChunkDataRequestMessagesByRequesterTotal,
		receivedChunkDataResponseMessageByRequesterTotal: receivedChunkDataResponseMessagesByRequesterTotal,
		sentChunkDataPackByRequesterTotal:                sentChunkDataPackByRequesterTotal,
	}

	return vc
}

// OnExecutionResultReceivedAtAssignerEngine is called whenever a new execution result arrives
// at Assigner engine. It increments total number of received results.
func (vc *VerificationCollector) OnExecutionResultReceivedAtAssignerEngine() {
	vc.receivedResultsByAssignerTotal.Inc()
}

// OnVerifiableChunkReceivedAtVerifierEngine is called whenever a verifiable chunk is received by Verifier engine
// from Assigner engine.It increments the total number of sent verifiable chunks.
func (vc *VerificationCollector) OnVerifiableChunkReceivedAtVerifierEngine() {
	vc.receivedVerifiableChunkTotalVerifier.Inc()
}

// OnResultApprovalDispatchedInNetwork is called whenever a result approval for is emitted to consensus nodes.
// It increases the total number of result approvals.
func (vc *VerificationCollector) OnResultApprovalDispatchedInNetwork() {
	// increases the counter of disseminated result approvals
	// fo by one. Each result approval corresponds to a single chunk of the block
	// the approvals disseminated by verifier engine
	vc.sentResultApprovalTotalVerifier.Inc()
}

// OnFinalizedBlockArrivedAtAssigner sets a gauge that keeps track of number of the latest block height arrives
// at assigner engine. Note that it assumes blocks are coming to assigner engine in strictly increasing order of their height.
func (vc *VerificationCollector) OnFinalizedBlockArrivedAtAssigner(height uint64) {
	vc.receivedFinalizedHeightAssigner.Set(float64(height))
}

// OnChunksAssignmentDoneAtAssigner increments a counter that keeps track of the total number of assigned chunks to
// the verification node.
func (vc *VerificationCollector) OnChunksAssignmentDoneAtAssigner(chunks int) {
	vc.assignedChunkTotalAssigner.Add(float64(chunks))
}

// OnAssignedChunkProcessedAtAssigner increments a counter that keeps track of the total number of assigned chunks pushed by
// assigner engine to the fetcher engine.
func (vc *VerificationCollector) OnAssignedChunkProcessedAtAssigner() {
	vc.processedChunkTotalAssigner.Inc()
}

// OnAssignedChunkReceivedAtFetcher increments a counter that keeps track of number of assigned chunks arrive at fetcher engine.
func (vc *VerificationCollector) OnAssignedChunkReceivedAtFetcher() {
	vc.receivedAssignedChunkByFetcherTotal.Inc()
}

// OnChunkDataPackRequestSentByFetcher increments a counter that keeps track of number of chunk data pack requests that fetcher engine
// sends to requester engine.
func (vc *VerificationCollector) OnChunkDataPackRequestSentByFetcher() {
	vc.requestedChunkDataPackByFetcherTotal.Inc()
}

// OnChunkDataPackRequestReceivedByRequester increments a counter that keeps track of number of chunk data pack requests
// arrive at the requester engine from the fetcher engine.
func (vc *VerificationCollector) OnChunkDataPackRequestReceivedByRequester() {
	vc.receivedChunkDataPackRequestByRequesterTotal.Inc()
}

// OnChunkDataPackRequestDispatchedInNetwork increments a counter that keeps track of number of chunk data pack requests that the
// requester engine dispatches in the network (to the execution nodes).
func (vc *VerificationCollector) OnChunkDataPackRequestDispatchedInNetwork() {
	vc.sentChunkDataRequestMessageByRequesterTotal.Inc()
}

// OnChunkDataPackResponseReceivedFromNetwork increments a counter that keeps track of number of chunk data pack responses that the
// requester engine receives from execution nodes (through network).
func (vc *VerificationCollector) OnChunkDataPackResponseReceivedFromNetwork() {
	vc.receivedChunkDataResponseMessageByRequesterTotal.Inc()
}

// OnChunkDataPackSentToFetcher increases a counter that keeps track of number of chunk data packs sent to the fetcher engine from
// requester engine.
func (vc *VerificationCollector) OnChunkDataPackSentToFetcher() {
	vc.sentChunkDataPackByRequesterTotal.Inc()
}

// OnChunkDataPackArrivedAtFetcher increments a counter that keeps track of number of chunk data packs arrived at fetcher engine from
// requester engine.
func (vc *VerificationCollector) OnChunkDataPackArrivedAtFetcher() {
	vc.receivedChunkDataPackByFetcherTotal.Inc()
}

// OnVerifiableChunkSentToVerifier increments a counter that keeps track of number of verifiable chunks fetcher engine sent to verifier engine.
func (vc *VerificationCollector) OnVerifiableChunkSentToVerifier() {
	vc.sentVerifiableChunksByFetcherTotal.Inc()
}

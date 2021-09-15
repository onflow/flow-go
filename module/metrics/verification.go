package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

type VerificationCollector struct {
	tracer module.Tracer

	// Job Consumers
	lastProcessedBlockJobIndexBlockConsumer prometheus.Gauge
	lastProcessedChunkJobIndexChunkConsumer prometheus.Gauge

	// Assigner Engine
	receivedFinalizedHeightAssigner prometheus.Gauge   // the last finalized height received by assigner engine
	assignedChunkTotalAssigner      prometheus.Counter // total chunks assigned to this verification node
	processedChunkTotalAssigner     prometheus.Counter // total chunks sent by assigner engine to chunk consumer (i.e., fetcher input)
	receivedResultsTotalAssigner    prometheus.Counter // total execution results arrived at assigner

	// Fetcher Engine
	receivedAssignedChunkTotalFetcher  prometheus.Counter // total assigned chunks received by fetcher engine from assigner engine.
	sentVerifiableChunksTotalFetcher   prometheus.Counter // total verifiable chunk sent by fetcher engine and sent to verifier engine.
	receivedChunkDataPackTotalFetcher  prometheus.Counter // total chunk data packs received by fetcher engine
	requestedChunkDataPackTotalFetcher prometheus.Counter // total number of chunk data packs requested by fetcher engine

	// Requester Engine
	//
	// total number of chunk data pack requests received by requester engine from fetcher engine.
	receivedChunkDataPackRequestsTotalRequester prometheus.Counter
	// total number of chunk data request messages dispatched in the network by requester engine.
	sentChunkDataRequestMessagesTotalRequester prometheus.Counter
	// total number of chunk data response messages received by requester from network.
	receivedChunkDataResponseMessageTotalRequester prometheus.Counter
	// total number of chunk data pack sent by requester to fetcher engine.
	sentChunkDataPackByRequesterTotal prometheus.Counter
	// maximum number of attempts made for requesting a chunk data pack.
	maxChunkDataPackRequestAttempt prometheus.Gauge

	// Verifier Engine
	receivedVerifiableChunkTotalVerifier prometheus.Counter // total verifiable chunks received by verifier engine
	sentResultApprovalTotalVerifier      prometheus.Counter // total result approvals sent by verifier engine

}

func NewVerificationCollector(tracer module.Tracer, registerer prometheus.Registerer) *VerificationCollector {
	// Job Consumers
	lastProcessedBlockJobIndexBlockConsumer := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "last_processed_block_job_index",
		Namespace: namespaceVerification,
		Subsystem: subsystemBlockConsumer,
		Help:      "the last block job index processed by block consumer",
	})

	lastProcessedChunkJobIndexChunkConsumer := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "last_processed_chunk_job_index",
		Namespace: namespaceVerification,
		Subsystem: subsystemChunkConsumer,
		Help:      "the last chunk job index processed by chunk consumer",
	})

	// Assigner Engine
	receivedFinalizedHeightAssigner := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "finalized_height",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "the last finalized height received by assigner engine",
	})

	receivedResultsTotalAssigner := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "received_result_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number of execution results received by assigner engine",
	})

	assignedChunksTotalAssigner := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_assigned_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number of chunks assigned to verification node",
	})

	sentChunksTotalAssigner := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "processed_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemAssignerEngine,
		Help:      "total number chunks sent by assigner engine to chunk consumer",
	})

	// Fetcher Engine
	receivedAssignedChunksTotalFetcher := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "assigned_chunk_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of chunks received by fetcher engine from assigner engine through chunk consumer",
	})

	// Requester Engine
	receivedChunkDataPackRequestsTotalRequester := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_request_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data pack requests received by requester engine from fetcher engine",
	})

	sentChunkDataRequestMessagesTotalRequester := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_request_message_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		Help:      "total number of chunk data pack request messages sent in the network by requester engine",
	})

	receivedChunkDataResponseMessagesTotalRequester := prometheus.NewCounter(prometheus.CounterOpts{
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

	sentVerifiableChunksTotalFetcher := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_sent_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of verifiable chunks sent by fetcher engine to verifier engine",
	})

	receivedChunkDataPackTotalFetcher := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of chunk data packs received by fetcher engine",
	})

	requestedChunkDataPackTotalFetcher := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "chunk_data_pack_requested_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemFetcherEngine,
		Help:      "total number of chunk data packs requested by fetcher engine",
	})

	maxChunkDataPackRequestAttempt := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "next_unsealed_height_max_chunk_data_pack_request_attempt_times",
		Namespace: namespaceVerification,
		Subsystem: subsystemRequesterEngine,
		// an indicator for when execution nodes is unresponsive to chunk data pack requests,
		// in which case verification node will keep requesting the chunk data pack, and this
		// metrics number will go up.
		Help: "among all the pending chunk data packs requested by the requester engine for the next unsealed height the maximum number of times a" +
			" certain chunk data pack was requested",
	})

	// Verifier Engine
	receivedVerifiableChunksTotalVerifier := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "verifiable_chunk_received_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number verifiable chunks received by verifier engine from fetcher engine",
	})

	sentResultApprovalTotalVerifier := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "result_approvals_total",
		Namespace: namespaceVerification,
		Subsystem: subsystemVerifierEngine,
		Help:      "total number of emitted result approvals by verifier engine",
	})

	// registers all metrics and panics if any fails.
	registerer.MustRegister(
		// job consumers
		lastProcessedBlockJobIndexBlockConsumer,
		lastProcessedChunkJobIndexChunkConsumer,

		// assigner
		receivedFinalizedHeightAssigner,
		assignedChunksTotalAssigner,
		sentChunksTotalAssigner,
		receivedResultsTotalAssigner,

		// fetcher engine
		receivedAssignedChunksTotalFetcher,
		sentVerifiableChunksTotalFetcher,
		receivedChunkDataPackTotalFetcher,
		requestedChunkDataPackTotalFetcher,

		// requester engine
		receivedChunkDataPackRequestsTotalRequester,
		sentChunkDataRequestMessagesTotalRequester,
		receivedChunkDataResponseMessagesTotalRequester,
		sentChunkDataPackByRequesterTotal,
		maxChunkDataPackRequestAttempt,

		// verifier engine
		receivedVerifiableChunksTotalVerifier,
		sentResultApprovalTotalVerifier)

	vc := &VerificationCollector{
		tracer: tracer,

		// job consumers
		lastProcessedChunkJobIndexChunkConsumer: lastProcessedChunkJobIndexChunkConsumer,
		lastProcessedBlockJobIndexBlockConsumer: lastProcessedBlockJobIndexBlockConsumer,

		// assigner
		receivedFinalizedHeightAssigner: receivedFinalizedHeightAssigner,
		assignedChunkTotalAssigner:      assignedChunksTotalAssigner,
		processedChunkTotalAssigner:     sentChunksTotalAssigner,
		receivedResultsTotalAssigner:    receivedResultsTotalAssigner,

		// fetcher
		receivedAssignedChunkTotalFetcher:  receivedAssignedChunksTotalFetcher,
		receivedChunkDataPackTotalFetcher:  receivedChunkDataPackTotalFetcher,
		requestedChunkDataPackTotalFetcher: requestedChunkDataPackTotalFetcher,
		sentVerifiableChunksTotalFetcher:   sentVerifiableChunksTotalFetcher,

		// verifier
		sentResultApprovalTotalVerifier:      sentResultApprovalTotalVerifier,
		receivedVerifiableChunkTotalVerifier: receivedVerifiableChunksTotalVerifier,

		// requester
		receivedChunkDataPackRequestsTotalRequester:    receivedChunkDataPackRequestsTotalRequester,
		sentChunkDataRequestMessagesTotalRequester:     sentChunkDataRequestMessagesTotalRequester,
		receivedChunkDataResponseMessageTotalRequester: receivedChunkDataResponseMessagesTotalRequester,
		sentChunkDataPackByRequesterTotal:              sentChunkDataPackByRequesterTotal,
		maxChunkDataPackRequestAttempt:                 maxChunkDataPackRequestAttempt,
	}

	return vc
}

// OnExecutionResultReceivedAtAssignerEngine is called whenever a new execution result arrives
// at Assigner engine. It increments total number of received results.
func (vc *VerificationCollector) OnExecutionResultReceivedAtAssignerEngine() {
	vc.receivedResultsTotalAssigner.Inc()
}

// OnVerifiableChunkReceivedAtVerifierEngine is called whenever a verifiable chunk is received by Verifier engine
// from Assigner engine.It increments the total number of sent verifiable chunks.
func (vc *VerificationCollector) OnVerifiableChunkReceivedAtVerifierEngine() {
	vc.receivedVerifiableChunkTotalVerifier.Inc()
}

// OnResultApprovalDispatchedInNetwork is called whenever a result approval for is emitted to consensus nodes.
// It increases the total number of result approvals.
func (vc *VerificationCollector) OnResultApprovalDispatchedInNetworkByVerifier() {
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
	vc.receivedAssignedChunkTotalFetcher.Inc()
}

// OnChunkDataPackRequestSentByFetcher increments a counter that keeps track of number of chunk data pack requests that fetcher engine
// sends to requester engine.
func (vc *VerificationCollector) OnChunkDataPackRequestSentByFetcher() {
	vc.requestedChunkDataPackTotalFetcher.Inc()
}

// OnChunkDataPackRequestReceivedByRequester increments a counter that keeps track of number of chunk data pack requests
// arrive at the requester engine from the fetcher engine.
func (vc *VerificationCollector) OnChunkDataPackRequestReceivedByRequester() {
	vc.receivedChunkDataPackRequestsTotalRequester.Inc()
}

// OnChunkDataPackRequestDispatchedInNetworkByRequester increments a counter that keeps track of number of chunk data pack requests that the
// requester engine dispatches in the network (to the execution nodes).
func (vc *VerificationCollector) OnChunkDataPackRequestDispatchedInNetworkByRequester() {
	vc.sentChunkDataRequestMessagesTotalRequester.Inc()
}

// OnChunkDataPackResponseReceivedFromNetworkByRequester increments a counter that keeps track of number of chunk data pack responses that the
// requester engine receives from execution nodes (through network).
func (vc *VerificationCollector) OnChunkDataPackResponseReceivedFromNetworkByRequester() {
	vc.receivedChunkDataResponseMessageTotalRequester.Inc()
}

// OnChunkDataPackSentToFetcher increases a counter that keeps track of number of chunk data packs sent to the fetcher engine from
// requester engine.
func (vc *VerificationCollector) OnChunkDataPackSentToFetcher() {
	vc.sentChunkDataPackByRequesterTotal.Inc()
}

// OnChunkDataPackArrivedAtFetcher increments a counter that keeps track of number of chunk data packs arrived at fetcher engine from
// requester engine.
func (vc *VerificationCollector) OnChunkDataPackArrivedAtFetcher() {
	vc.receivedChunkDataPackTotalFetcher.Inc()
}

// OnVerifiableChunkSentToVerifier increments a counter that keeps track of number of verifiable chunks fetcher engine sent to verifier engine.
func (vc *VerificationCollector) OnVerifiableChunkSentToVerifier() {
	vc.sentVerifiableChunksTotalFetcher.Inc()
}

// OnChunkConsumerJobDone is invoked by chunk consumer whenever it is notified a job is done by a worker. It
// sets the last processed chunk job index.
func (vc *VerificationCollector) OnChunkConsumerJobDone(processedIndex uint64) {
	vc.lastProcessedChunkJobIndexChunkConsumer.Set(float64(processedIndex))
}

// OnBlockConsumerJobDone is invoked by block consumer whenever it is notified a job is done by a worker. It
// sets the last processed block job index.
func (vc *VerificationCollector) OnBlockConsumerJobDone(processedIndex uint64) {
	vc.lastProcessedBlockJobIndexBlockConsumer.Set(float64(processedIndex))
}

// SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester is invoked when a cycle of requesting chunk data packs is done by requester engine.
// It updates the maximum number of attempts made by requester engine for requesting the chunk data packs of the next unsealed height.
// The maximum is taken over the history of all chunk data packs requested during that cycle that belong to the next unsealed height.
func (vc *VerificationCollector) SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester(attempts uint64) {
	vc.maxChunkDataPackRequestAttempt.Set(float64(attempts))
}

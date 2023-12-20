package module

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	httpmetrics "github.com/slok/go-http-metrics/metrics"

	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

type EntriesFunc func() uint

// ResolverMetrics encapsulates the metrics collectors for dns resolver module of the networking layer.
type ResolverMetrics interface {
	// DNSLookupDuration tracks the time spent to resolve a DNS address.
	DNSLookupDuration(duration time.Duration)

	// OnDNSCacheMiss tracks the total number of dns requests resolved through looking up the network.
	OnDNSCacheMiss()

	// OnDNSCacheHit tracks the total number of dns requests resolved through the cache without
	// looking up the network.
	OnDNSCacheHit()

	// OnDNSCacheInvalidated is called whenever dns cache is invalidated for an entry
	OnDNSCacheInvalidated()

	// OnDNSLookupRequestDropped tracks the number of dns lookup requests that are dropped due to a full queue
	OnDNSLookupRequestDropped()
}

// NetworkSecurityMetrics metrics related to network protection.
type NetworkSecurityMetrics interface {
	// OnUnauthorizedMessage tracks the number of unauthorized messages seen on the network.
	OnUnauthorizedMessage(role, msgType, topic, offense string)

	// OnRateLimitedPeer tracks the number of rate limited unicast messages seen on the network.
	OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string)

	// OnViolationReportSkipped tracks the number of slashing violations consumer violations that were not
	// reported for misbehavior when the identity of the sender not known.
	OnViolationReportSkipped()
}

// GossipSubRpcInspectorMetrics encapsulates the metrics collectors for GossipSub RPC Inspector module of the networking layer.
// The RPC inspector is the entry point of the GossipSub protocol. It inspects the incoming RPC messages and decides
// whether to accept, prune, or reject the RPC message.
// The GossipSubRpcInspectorMetrics tracks the number of RPC messages received by the local node from other nodes over
// the GossipSub protocol. It also tracks the number of control messages included in the RPC messages, i.e., IHAVE, IWANT,
// GRAFT, PRUNE. It also tracks the number of actual messages included in the RPC messages.
// The GossipSubRpcInspectorMetrics differs from LocalGossipSubRouterMetrics in that the former tracks the messages
// received by the local node from other nodes over the GossipSub protocol but may not all be accepted by the local node,
// e.g., due to RPC pruning or throttling; while the latter tracks the local node's view of the GossipSub protocol, i.e., entirely
// containing the messages that are accepted by the local node (either as whole RPC or only for the control messages).
// Having this distinction is useful for debugging and troubleshooting the GossipSub protocol, for example, the number of
// messages received by the local node from other nodes over the GossipSub protocol may be much higher than the number
// of messages accepted by the local node, which may indicate that the local node is throttling the incoming messages.
type GossipSubRpcInspectorMetrics interface {
	// OnIncomingRpcAcceptedFully tracks the number of RPC messages received by the node that are fully accepted.
	// An RPC may contain any number of control messages, i.e., IHAVE, IWANT, GRAFT, PRUNE, as well as the actual messages.
	// A fully accepted RPC means that all the control messages are accepted and all the messages are accepted.
	OnIncomingRpcAcceptedFully()

	// OnIncomingRpcAcceptedOnlyForControlMessages tracks the number of RPC messages received by the node that are accepted
	// only for the control messages, i.e., only for the included IHAVE, IWANT, GRAFT, PRUNE. However, the actual messages
	// included in the RPC are not accepted.
	// This happens mostly when the validation pipeline of GossipSub is throttled, and cannot accept more actual messages for
	// validation.
	OnIncomingRpcAcceptedOnlyForControlMessages()

	// OnIncomingRpcRejected tracks the number of RPC messages received by the node that are rejected.
	// This happens mostly when the RPC is coming from a low-scored peer based on the peer scoring module of GossipSub.
	OnIncomingRpcRejected()

	// OnIWantReceived tracks the number of IWANT messages received by the node from other nodes over an RPC message.
	// iWant is a control message that is sent by a node to request a message that it has seen advertised in an iHAVE message.
	OnIWantReceived(count int)

	// OnIHaveReceived tracks the number of IHAVE messages received by the node from other nodes over an RPC message.
	// iHave is a control message that is sent by a node to another node to indicate that it has a new gossiped message.
	OnIHaveReceived(count int)

	// OnGraftReceived tracks the number of GRAFT messages received by the node from other nodes over an RPC message.
	// GRAFT is a control message of GossipSub protocol that connects two nodes over a topic directly as gossip partners.
	OnGraftReceived(count int)

	// OnPruneReceived tracks the number of PRUNE messages received by the node from other nodes over an RPC message.
	// PRUNE is a control message of GossipSub protocol that disconnects two nodes over a topic.
	OnPruneReceived(count int)

	// OnPublishedGossipMessagesReceived tracks the number of gossip messages received by the node from other nodes over an
	// RPC message.
	OnPublishedGossipMessagesReceived(count int)
}

// LocalGossipSubRouterMetrics encapsulates the metrics collectors for GossipSub router of the local node.
// It gives a lens into the local GossipSub node's view of the GossipSub protocol.
// LocalGossipSubRouterMetrics differs from GossipSubRpcInspectorMetrics in that the former tracks the local node's view
// of the GossipSub protocol, while the latter tracks the messages received by the local node from other nodes over the
// GossipSub protocol but may not all be accepted by the local node, e.g., due to RPC pruning or throttling.
// Having this distinction is useful for debugging and troubleshooting the GossipSub protocol, for example, the number of
// messages received by the local node from other nodes over the GossipSub protocol may be much higher than the number
// of messages accepted by the local node, which may indicate that the local node is throttling the incoming messages.
type LocalGossipSubRouterMetrics interface {
	// OnLocalMeshSizeUpdated tracks the size of the local mesh for a topic.
	OnLocalMeshSizeUpdated(topic string, size int)

	// OnPeerAddedToProtocol is called when the local node receives a stream from a peer on a gossipsub-related protocol.
	// Args:
	// 	protocol: the protocol name that the peer is connected to.
	OnPeerAddedToProtocol(protocol string)

	// OnPeerRemovedFromProtocol is called when the local considers a remote peer blacklisted or unavailable.
	OnPeerRemovedFromProtocol()

	// OnLocalPeerJoinedTopic is called when the local node subscribes to a gossipsub topic.
	OnLocalPeerJoinedTopic()

	// OnLocalPeerLeftTopic is called when the local node unsubscribes from a gossipsub topic.
	OnLocalPeerLeftTopic()

	// OnPeerGraftTopic is called when the local node receives a GRAFT message from a remote peer on a topic.
	// Note: the received GRAFT at this point is considered passed the RPC inspection, and is accepted by the local node.
	OnPeerGraftTopic(topic string)

	// OnPeerPruneTopic is called when the local node receives a PRUNE message from a remote peer on a topic.
	// Note: the received PRUNE at this point is considered passed the RPC inspection, and is accepted by the local node.
	OnPeerPruneTopic(topic string)

	// OnMessageEnteredValidation is called when a received pubsub message enters the validation pipeline. It is the
	// internal validation pipeline of GossipSub protocol. The message may be rejected or accepted by the validation
	// pipeline.
	OnMessageEnteredValidation(size int)

	// OnMessageRejected is called when a received pubsub message is rejected by the validation pipeline.
	// Args:
	//
	//	reason: the reason for rejection.
	// 	size: the size of the message in bytes.
	OnMessageRejected(size int, reason string)

	// OnMessageDuplicate is called when a received pubsub message is a duplicate of a previously received message, and
	// is dropped.
	// Args:
	// 	size: the size of the message in bytes.
	OnMessageDuplicate(size int)

	// OnPeerThrottled is called when a peer is throttled by the local node, i.e., the local node is not accepting any
	// pubsub message from the peer but may still accept control messages.
	OnPeerThrottled()

	// OnRpcReceived is called when an RPC message is received by the local node. The received RPC is considered
	// passed the RPC inspection, and is accepted by the local node.
	// Args:
	// 	msgCount: the number of messages included in the RPC.
	// 	iHaveCount: the number of iHAVE messages included in the RPC.
	// 	iWantCount: the number of iWANT messages included in the RPC.
	// 	graftCount: the number of GRAFT messages included in the RPC.
	// 	pruneCount: the number of PRUNE messages included in the RPC.
	OnRpcReceived(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int)

	// OnRpcSent is called when an RPC message is sent by the local node.
	// Note: the sent RPC is considered passed the RPC inspection, and is accepted by the local node.
	// Args:
	// 	msgCount: the number of messages included in the RPC.
	// 	iHaveCount: the number of iHAVE messages included in the RPC.
	// 	iWantCount: the number of iWANT messages included in the RPC.
	// 	graftCount: the number of GRAFT messages included in the RPC.
	// 	pruneCount: the number of PRUNE messages included in the RPC.
	OnRpcSent(msgCount int, iHaveCount int, iWantCount int, graftCount int, pruneCount int)

	// OnOutboundRpcDropped is called when an outbound RPC message is dropped by the local node, typically because the local node
	// outbound message queue is full; or the RPC is big and the local node cannot fragment it.
	OnOutboundRpcDropped()

	// OnUndeliveredMessage is called when a message is not delivered at least one subscriber of the topic, for example when
	// the subscriber is too slow to process the message.
	OnUndeliveredMessage()

	// OnMessageDeliveredToAllSubscribers is called when a message is delivered to all subscribers of the topic.
	OnMessageDeliveredToAllSubscribers(size int)
}

// UnicastManagerMetrics unicast manager metrics.
type UnicastManagerMetrics interface {
	// OnStreamCreated tracks the overall time it takes to create a stream successfully and the number of retry attempts.
	OnStreamCreated(duration time.Duration, attempts int)
	// OnStreamCreationFailure tracks the amount of time taken and number of retry attempts used when the unicast manager fails to create a stream.
	OnStreamCreationFailure(duration time.Duration, attempts int)
	// OnPeerDialed tracks the time it takes to dial a peer during stream creation and the number of retry attempts before a peer
	// is dialed successfully.
	OnPeerDialed(duration time.Duration, attempts int)
	// OnPeerDialFailure tracks the amount of time taken and number of retry attempts used when the unicast manager cannot dial a peer
	// to establish the initial connection between the two.
	OnPeerDialFailure(duration time.Duration, attempts int)
	// OnStreamEstablished tracks the time it takes to create a stream successfully on the available open connection during stream
	// creation and the number of retry attempts.
	OnStreamEstablished(duration time.Duration, attempts int)
	// OnEstablishStreamFailure tracks the amount of time taken and number of retry attempts used when the unicast manager cannot establish
	// a stream on the open connection between two peers.
	OnEstablishStreamFailure(duration time.Duration, attempts int)

	// OnDialRetryBudgetUpdated tracks the history of the dial retry budget updates.
	OnDialRetryBudgetUpdated(budget uint64)

	// OnStreamCreationRetryBudgetUpdated tracks the history of the stream creation retry budget updates.
	OnStreamCreationRetryBudgetUpdated(budget uint64)

	// OnDialRetryBudgetResetToDefault tracks the number of times the dial retry budget is reset to default.
	OnDialRetryBudgetResetToDefault()

	// OnStreamCreationRetryBudgetResetToDefault tracks the number of times the stream creation retry budget is reset to default.
	OnStreamCreationRetryBudgetResetToDefault()
}

type GossipSubMetrics interface {
	GossipSubScoringMetrics
	GossipSubRpcInspectorMetrics
	LocalGossipSubRouterMetrics
	GossipSubRpcValidationInspectorMetrics
}

type LibP2PMetrics interface {
	GossipSubMetrics
	ResolverMetrics
	DHTMetrics
	rcmgr.MetricsReporter
	LibP2PConnectionMetrics
	UnicastManagerMetrics
}

// GossipSubScoringMetrics encapsulates the metrics collectors for the peer scoring module of GossipSub protocol.
// It tracks the scores of the peers in the local mesh and the different factors that contribute to the score of a peer.
// It also tracks the scores of the topics in the local mesh and the different factors that contribute to the score of a topic.
type GossipSubScoringMetrics interface {
	// OnOverallPeerScoreUpdated tracks the overall score of peers in the local mesh.
	OnOverallPeerScoreUpdated(float64)
	// OnAppSpecificScoreUpdated tracks the application specific score of peers in the local mesh.
	OnAppSpecificScoreUpdated(float64)
	// OnIPColocationFactorUpdated tracks the IP colocation factor of peers in the local mesh.
	OnIPColocationFactorUpdated(float64)
	// OnBehaviourPenaltyUpdated tracks the behaviour penalty of peers in the local mesh.
	OnBehaviourPenaltyUpdated(float64)
	// OnTimeInMeshUpdated tracks the time in mesh factor of peers in the local mesh for a given topic.
	OnTimeInMeshUpdated(channels.Topic, time.Duration)
	// OnFirstMessageDeliveredUpdated tracks the first message delivered factor of peers in the local mesh for a given topic.
	OnFirstMessageDeliveredUpdated(channels.Topic, float64)
	// OnMeshMessageDeliveredUpdated tracks the mesh message delivered factor of peers in the local mesh for a given topic.
	OnMeshMessageDeliveredUpdated(channels.Topic, float64)
	// OnInvalidMessageDeliveredUpdated tracks the invalid message delivered factor of peers in the local mesh for a given topic.
	OnInvalidMessageDeliveredUpdated(channels.Topic, float64)
	// SetWarningStateCount tracks the warning score state of peers in the local mesh. It updates the total number of
	// peers in the local mesh that are in the warning state based on their score.
	SetWarningStateCount(uint)
}

// GossipSubRpcValidationInspectorMetrics encapsulates the metrics collectors for the gossipsub rpc validation control message inspectors.
type GossipSubRpcValidationInspectorMetrics interface {
	// AsyncProcessingStarted increments the metric tracking the number of inspect message request being processed by workers in the rpc validator worker pool.
	AsyncProcessingStarted()
	// AsyncProcessingFinished tracks the time spent by a rpc validation inspector worker to process an inspect message request asynchronously and decrements the metric tracking
	// the number of inspect message requests  being processed asynchronously by the rpc validation inspector workers.
	AsyncProcessingFinished(duration time.Duration)
}

// NetworkInboundQueueMetrics encapsulates the metrics collectors for the inbound queue of the networking layer.
type NetworkInboundQueueMetrics interface {

	// MessageAdded increments the metric tracking the number of messages in the queue with the given priority
	MessageAdded(priority int)

	// MessageRemoved decrements the metric tracking the number of messages in the queue with the given priority
	MessageRemoved(priority int)

	// QueueDuration tracks the time spent by a message with the given priority in the queue
	QueueDuration(duration time.Duration, priority int)
}

// NetworkCoreMetrics encapsulates the metrics collectors for the core networking layer functionality.
type NetworkCoreMetrics interface {
	NetworkInboundQueueMetrics
	AlspMetrics
	NetworkSecurityMetrics

	// OutboundMessageSent collects metrics related to a message sent by the node.
	OutboundMessageSent(sizeBytes int, topic string, protocol string, messageType string)
	// InboundMessageReceived collects metrics related to a message received by the node.
	InboundMessageReceived(sizeBytes int, topic string, protocol string, messageType string)
	// DuplicateInboundMessagesDropped increments the metric tracking the number of duplicate messages dropped by the node.
	DuplicateInboundMessagesDropped(topic string, protocol string, messageType string)
	// UnicastMessageSendingStarted increments the metric tracking the number of unicast messages sent by the node.
	UnicastMessageSendingStarted(topic string)
	// UnicastMessageSendingCompleted decrements the metric tracking the number of unicast messages sent by the node.
	UnicastMessageSendingCompleted(topic string)
	// MessageProcessingStarted increments the metric tracking the number of messages being processed by the node.
	MessageProcessingStarted(topic string)
	// MessageProcessingFinished tracks the time spent by the node to process a message and decrements the metric tracking
	// the number of messages being processed by the node.
	MessageProcessingFinished(topic string, duration time.Duration)
}

// LibP2PConnectionMetrics encapsulates the metrics collectors for the connection manager of the libp2p node.
type LibP2PConnectionMetrics interface {
	// OutboundConnections updates the metric tracking the number of outbound connections of this node
	OutboundConnections(connectionCount uint)

	// InboundConnections updates the metric tracking the number of inbound connections of this node
	InboundConnections(connectionCount uint)
}

// AlspMetrics encapsulates the metrics collectors for the Application Layer Spam Prevention (ALSP) module, which
// is part of the networking layer. ALSP is responsible to prevent spam attacks on the application layer messages that
// appear to be valid for the networking layer but carry on a malicious intent on the application layer (i.e., Flow protocols).
type AlspMetrics interface {
	// OnMisbehaviorReported is called when a misbehavior is reported by the application layer to ALSP.
	// An engine detecting a spamming-related misbehavior reports it to the ALSP module.
	// Args:
	// - channel: the channel on which the misbehavior was reported
	// - misbehaviorType: the type of misbehavior reported
	OnMisbehaviorReported(channel string, misbehaviorType string)
}

// NetworkMetrics is the blanket abstraction that encapsulates the metrics collectors for the networking layer.
type NetworkMetrics interface {
	LibP2PMetrics
	NetworkCoreMetrics
}

// EngineMetrics is a generic metrics consumer for node-internal data processing
// components (aka engines). Implementations must be non-blocking and concurrency safe.
type EngineMetrics interface {
	// MessageSent reports that the engine transmitted the message over the network.
	// Unicasts, broadcasts, and multicasts are all reported once.
	MessageSent(engine string, message string)
	// MessageReceived reports that the engine received the message over the network.
	MessageReceived(engine string, message string)
	// MessageHandled reports that the engine has finished processing the message.
	// Both invalid and valid messages should be reported.
	// A message must be reported as either handled or dropped, not both.
	MessageHandled(engine string, messages string)
	// InboundMessageDropped reports that the engine has dropped inbound message without processing it.
	// Inbound messages must be reported as either handled or dropped, not both.
	InboundMessageDropped(engine string, messages string)
	// OutboundMessageDropped reports that the engine has dropped outbound message without processing it.
	// Outbound messages must be reported as either sent or dropped, not both.
	OutboundMessageDropped(engine string, messages string)
}

type ComplianceMetrics interface {
	FinalizedHeight(height uint64)
	CommittedEpochFinalView(view uint64)
	EpochTransitionHeight(height uint64)
	SealedHeight(height uint64)
	BlockFinalized(*flow.Block)
	BlockSealed(*flow.Block)
	CurrentEpochCounter(counter uint64)
	CurrentEpochPhase(phase flow.EpochPhase)
	CurrentEpochFinalView(view uint64)
	CurrentDKGPhase1FinalView(view uint64)
	CurrentDKGPhase2FinalView(view uint64)
	CurrentDKGPhase3FinalView(view uint64)
	EpochEmergencyFallbackTriggered()
}

type CleanerMetrics interface {
	RanGC(took time.Duration)
}

type CacheMetrics interface {
	// CacheEntries report the total number of cached items
	CacheEntries(resource string, entries uint)
	// CacheHit report the number of times the queried item is found in the cache
	CacheHit(resource string)
	// CacheNotFound records the number of times the queried item was not found in either cache or database.
	CacheNotFound(resource string)
	// CacheMiss report the number of times the queried item is not found in the cache, but found in the database.
	CacheMiss(resource string)
}

type MempoolMetrics interface {
	MempoolEntries(resource string, entries uint)
	Register(resource string, entriesFunc EntriesFunc) error
}

type HotstuffMetrics interface {
	// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
	HotStuffBusyDuration(duration time.Duration, event string)

	// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
	HotStuffIdleDuration(duration time.Duration)

	// HotStuffWaitDuration reports Metrics C6 HotStuff Idle Duration - the time between receiving and
	// enqueueing a message to beginning to process that message.
	HotStuffWaitDuration(duration time.Duration, event string)

	// SetCurView reports Metrics C8: Current View maintained by Pacemaker.
	SetCurView(view uint64)

	// SetQCView reports Metrics C9: View of the newest QC known to Pacemaker.
	SetQCView(view uint64)

	// SetTCView reports last TC known to Pacemaker.
	SetTCView(view uint64)

	// CountSkipped counts the number of skips we did.
	CountSkipped()

	// CountTimeout tracks the number of views that this replica left due to observing a TC.
	CountTimeout()

	// SetTimeout sets the current timeout duration
	SetTimeout(duration time.Duration)

	// BlockProcessingDuration measures the time which the compliance engine
	// spends to process one block proposal.
	BlockProcessingDuration(duration time.Duration)

	// VoteProcessingDuration measures the time which the hotstuff.VoteAggregator
	// spends to process one vote.
	VoteProcessingDuration(duration time.Duration)

	// TimeoutObjectProcessingDuration measures the time which the hotstuff.TimeoutAggregator
	// spends to process one timeout object.
	TimeoutObjectProcessingDuration(duration time.Duration)

	// CommitteeProcessingDuration measures the time which the HotStuff's core logic
	// spends in the hotstuff.Replicas component, i.e. the time determining consensus
	// committee relations.
	CommitteeProcessingDuration(duration time.Duration)

	// SignerProcessingDuration measures the time which the HotStuff's core logic
	// spends in the hotstuff.Signer component, i.e. the with crypto-related operations.
	SignerProcessingDuration(duration time.Duration)

	// ValidatorProcessingDuration measures the time which the HotStuff's core logic
	// spends in the hotstuff.Validator component, i.e. the with verifying
	// consensus messages.
	ValidatorProcessingDuration(duration time.Duration)

	// PayloadProductionDuration measures the time which the HotStuff's core logic
	// spends in the module.Builder component, i.e. the with generating block payloads.
	PayloadProductionDuration(duration time.Duration)

	// TimeoutCollectorsRange collects information from the node's `TimeoutAggregator` component.
	// Specifically, it measurers the number of views for which we are currently collecting timeouts
	// (i.e. the number of `TimeoutCollector` instances we are maintaining) and their lowest/highest view.
	TimeoutCollectorsRange(lowestRetainedView uint64, newestViewCreatedCollector uint64, activeCollectors int)
}

type CruiseCtlMetrics interface {

	// PIDError measures the current error values for the proportional, integration,
	// and derivative terms of the PID controller.
	PIDError(p, i, d float64)

	// TargetProposalDuration measures the current value of the Block Time Controller output:
	// the target duration from parent to child proposal.
	TargetProposalDuration(duration time.Duration)

	// ControllerOutput measures the output of the cruise control PID controller.
	// Concretely, this is the quantity to subtract from the baseline view duration.
	ControllerOutput(duration time.Duration)
}

type CollectionMetrics interface {
	// TransactionIngested is called when a new transaction is ingested by the
	// node. It increments the total count of ingested transactions and starts
	// a tx->col span for the transaction.
	TransactionIngested(txID flow.Identifier)

	// ClusterBlockProposed is called when a new collection is proposed by us or
	// any other node in the cluster.
	ClusterBlockProposed(block *cluster.Block)

	// ClusterBlockFinalized is called when a collection is finalized.
	ClusterBlockFinalized(block *cluster.Block)
}

type ConsensusMetrics interface {
	// StartCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
	StartCollectionToFinalized(collectionID flow.Identifier)

	// FinishCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
	FinishCollectionToFinalized(collectionID flow.Identifier)

	// StartBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
	StartBlockToSeal(blockID flow.Identifier)

	// FinishBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
	FinishBlockToSeal(blockID flow.Identifier)

	// EmergencySeal increments the number of seals that were created in emergency mode
	EmergencySeal()

	// OnReceiptProcessingDuration records the number of seconds spent processing a receipt
	OnReceiptProcessingDuration(duration time.Duration)

	// OnApprovalProcessingDuration records the number of seconds spent processing an approval
	OnApprovalProcessingDuration(duration time.Duration)

	// CheckSealingDuration records absolute time for the full sealing check by the consensus match engine
	CheckSealingDuration(duration time.Duration)
}

type VerificationMetrics interface {
	// OnBlockConsumerJobDone is invoked by block consumer whenever it is notified a job is done by a worker. It
	// sets the last processed block job index.
	OnBlockConsumerJobDone(uint64)
	// OnChunkConsumerJobDone is invoked by chunk consumer whenever it is notified a job is done by a worker. It
	// sets the last processed chunk job index.
	OnChunkConsumerJobDone(uint64)
	// OnExecutionResultReceivedAtAssignerEngine is called whenever a new execution result arrives
	// at Assigner engine. It increments total number of received execution results.
	OnExecutionResultReceivedAtAssignerEngine()

	// OnVerifiableChunkReceivedAtVerifierEngine increments a counter that keeps track of number of verifiable chunks received at
	// verifier engine from fetcher engine.
	OnVerifiableChunkReceivedAtVerifierEngine()

	// OnFinalizedBlockArrivedAtAssigner sets a gauge that keeps track of number of the latest block height arrives
	// at assigner engine. Note that it assumes blocks are coming to assigner engine in strictly increasing order of their height.
	OnFinalizedBlockArrivedAtAssigner(height uint64)

	// OnChunksAssignmentDoneAtAssigner increments a counter that keeps track of the total number of assigned chunks to
	// the verification node.
	OnChunksAssignmentDoneAtAssigner(chunks int)

	// OnAssignedChunkProcessedAtAssigner increments a counter that keeps track of the total number of assigned chunks pushed by
	// assigner engine to the fetcher engine.
	OnAssignedChunkProcessedAtAssigner()

	// OnAssignedChunkReceivedAtFetcher increments a counter that keeps track of number of assigned chunks arrive at fetcher engine.
	OnAssignedChunkReceivedAtFetcher()

	// OnChunkDataPackRequestSentByFetcher increments a counter that keeps track of number of chunk data pack requests that fetcher engine
	// sends to requester engine.
	OnChunkDataPackRequestSentByFetcher()

	// OnChunkDataPackRequestReceivedByRequester increments a counter that keeps track of number of chunk data pack requests
	// arrive at the requester engine from the fetcher engine.
	OnChunkDataPackRequestReceivedByRequester()

	// OnChunkDataPackRequestDispatchedInNetwork increments a counter that keeps track of number of chunk data pack requests that the
	// requester engine dispatches in the network (to the execution nodes).
	OnChunkDataPackRequestDispatchedInNetworkByRequester()

	// OnChunkDataPackResponseReceivedFromNetwork increments a counter that keeps track of number of chunk data pack responses that the
	// requester engine receives from execution nodes (through network).
	OnChunkDataPackResponseReceivedFromNetworkByRequester()

	// SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester is invoked when a cycle of requesting chunk data packs is done by requester engine.
	// It updates the maximum number of attempts made by requester engine for requesting the chunk data packs of the next unsealed height.
	// The maximum is taken over the history of all chunk data packs requested during that cycle that belong to the next unsealed height.
	SetMaxChunkDataPackAttemptsForNextUnsealedHeightAtRequester(attempts uint64)

	// OnChunkDataPackSentToFetcher increments a counter that keeps track of number of chunk data packs sent to the fetcher engine from
	// requester engine.
	OnChunkDataPackSentToFetcher()

	// OnChunkDataPackArrivedAtFetcher increments a counter that keeps track of number of chunk data packs arrived at fetcher engine from
	// requester engine.
	OnChunkDataPackArrivedAtFetcher()

	// OnVerifiableChunkSentToVerifier increments a counter that keeps track of number of verifiable chunks fetcher engine sent to verifier engine.
	OnVerifiableChunkSentToVerifier()

	// OnResultApprovalDispatchedInNetwork increments a counter that keeps track of number of result approvals dispatched in the network
	// by verifier engine.
	OnResultApprovalDispatchedInNetworkByVerifier()
}

// LedgerMetrics provides an interface to record Ledger Storage metrics.
// Ledger storage is non-linear (fork-aware) so certain metrics are averaged
// and computed before emitting for better visibility
type LedgerMetrics interface {
	// ForestApproxMemorySize records approximate memory usage of forest (all in-memory trees)
	ForestApproxMemorySize(bytes uint64)

	// ForestNumberOfTrees current number of trees in a forest (in memory)
	ForestNumberOfTrees(number uint64)

	// LatestTrieRegCount records the number of unique register allocated (the latest created trie)
	LatestTrieRegCount(number uint64)

	// LatestTrieRegCountDiff records the difference between the number of unique register allocated of the latest created trie and parent trie
	LatestTrieRegCountDiff(number int64)

	// LatestTrieRegSize records the size of unique register allocated (the latest created trie)
	LatestTrieRegSize(size uint64)

	// LatestTrieRegSizeDiff records the difference between the size of unique register allocated of the latest created trie and parent trie
	LatestTrieRegSizeDiff(size int64)

	// LatestTrieMaxDepthTouched records the maximum depth touched of the lastest created trie
	LatestTrieMaxDepthTouched(maxDepth uint16)

	// UpdateCount increase a counter of performed updates
	UpdateCount()

	// ProofSize records a proof size
	ProofSize(bytes uint32)

	// UpdateValuesNumber accumulates number of updated values
	UpdateValuesNumber(number uint64)

	// UpdateValuesSize total size (in bytes) of updates values
	UpdateValuesSize(byte uint64)

	// UpdateDuration records absolute time for the update of a trie
	UpdateDuration(duration time.Duration)

	// UpdateDurationPerItem records update time for single value (total duration / number of updated values)
	UpdateDurationPerItem(duration time.Duration)

	// ReadValuesNumber accumulates number of read values
	ReadValuesNumber(number uint64)

	// ReadValuesSize total size (in bytes) of read values
	ReadValuesSize(byte uint64)

	// ReadDuration records absolute time for the read from a trie
	ReadDuration(duration time.Duration)

	// ReadDurationPerItem records read time for single value (total duration / number of read values)
	ReadDurationPerItem(duration time.Duration)
}

type WALMetrics interface {
}

type RateLimitedBlockstoreMetrics interface {
	BytesRead(int)
}

type BitswapMetrics interface {
	Peers(prefix string, n int)
	Wantlist(prefix string, n int)
	BlobsReceived(prefix string, n uint64)
	DataReceived(prefix string, n uint64)
	BlobsSent(prefix string, n uint64)
	DataSent(prefix string, n uint64)
	DupBlobsReceived(prefix string, n uint64)
	DupDataReceived(prefix string, n uint64)
	MessagesReceived(prefix string, n uint64)
}

type ExecutionDataRequesterMetrics interface {
	// ExecutionDataFetchStarted records an in-progress download
	ExecutionDataFetchStarted()

	// ExecutionDataFetchFinished records a completed download
	ExecutionDataFetchFinished(duration time.Duration, success bool, height uint64)

	// NotificationSent reports that ExecutionData received notifications were sent for a block height
	NotificationSent(height uint64)

	// FetchRetried reports that a download retry was processed
	FetchRetried()
}

type ExecutionStateIndexerMetrics interface {
	// BlockIndexed records metrics from indexing execution data from a single block.
	BlockIndexed(height uint64, duration time.Duration, events, registers, transactionResults int)

	// BlockReindexed records that a previously indexed block was indexed again.
	BlockReindexed()

	// InitializeLatestHeight records the latest height that has been indexed.
	// This should only be used during startup. After startup, use BlockIndexed to record newly
	// indexed heights.
	InitializeLatestHeight(height uint64)
}

type RuntimeMetrics interface {
	// RuntimeTransactionParsed reports the time spent parsing a single transaction
	RuntimeTransactionParsed(dur time.Duration)

	// RuntimeTransactionChecked reports the time spent checking a single transaction
	RuntimeTransactionChecked(dur time.Duration)

	// RuntimeTransactionInterpreted reports the time spent interpreting a single transaction
	RuntimeTransactionInterpreted(dur time.Duration)

	// RuntimeSetNumberOfAccounts Sets the total number of accounts on the network
	RuntimeSetNumberOfAccounts(count uint64)

	// RuntimeTransactionProgramsCacheMiss reports a programs cache miss
	// during transaction execution
	RuntimeTransactionProgramsCacheMiss()

	// RuntimeTransactionProgramsCacheHit reports a programs cache hit
	// during transaction execution
	RuntimeTransactionProgramsCacheHit()
}

type ProviderMetrics interface {
	// ChunkDataPackRequestProcessed is executed every time a chunk data pack request is picked up for processing at execution node.
	// It increases the request processed counter by one.
	ChunkDataPackRequestProcessed()
}

type ExecutionDataProviderMetrics interface {
	RootIDComputed(duration time.Duration, numberOfChunks int)
	AddBlobsSucceeded(duration time.Duration, totalSize uint64)
	AddBlobsFailed()
}

type ExecutionDataRequesterV2Metrics interface {
	FulfilledHeight(blockHeight uint64)
	ReceiptSkipped()
	RequestSucceeded(blockHeight uint64, duration time.Duration, totalSize uint64, numberOfAttempts int)
	RequestFailed(duration time.Duration, retryable bool)
	RequestCanceled()
	ResponseDropped()
}

type ExecutionDataPrunerMetrics interface {
	Pruned(height uint64, duration time.Duration)
}

type RestMetrics interface {
	// Example recorder taken from:
	// https://github.com/slok/go-http-metrics/blob/master/metrics/prometheus/prometheus.go
	httpmetrics.Recorder
	AddTotalRequests(ctx context.Context, method string, routeName string)
}

type GRPCConnectionPoolMetrics interface {
	// TotalConnectionsInPool updates the number connections to collection/execution nodes stored in the pool, and the size of the pool
	TotalConnectionsInPool(connectionCount uint, connectionPoolSize uint)

	// ConnectionFromPoolReused tracks the number of times a connection to a collection/execution node is reused from the connection pool
	ConnectionFromPoolReused()

	// ConnectionAddedToPool tracks the number of times a collection/execution node is added to the connection pool
	ConnectionAddedToPool()

	// NewConnectionEstablished tracks the number of times a new grpc connection is established
	NewConnectionEstablished()

	// ConnectionFromPoolInvalidated tracks the number of times a cached grpc connection is invalidated and closed
	ConnectionFromPoolInvalidated()

	// ConnectionFromPoolUpdated tracks the number of times a cached connection is updated
	ConnectionFromPoolUpdated()

	// ConnectionFromPoolEvicted tracks the number of times a cached connection is evicted from the cache
	ConnectionFromPoolEvicted()
}

type AccessMetrics interface {
	RestMetrics
	GRPCConnectionPoolMetrics
	TransactionMetrics
	BackendScriptsMetrics

	// UpdateExecutionReceiptMaxHeight is called whenever we store an execution receipt from a block from a newer height
	UpdateExecutionReceiptMaxHeight(height uint64)

	// UpdateLastFullBlockHeight tracks the height of the last block for which all collections were received
	UpdateLastFullBlockHeight(height uint64)
}

type ExecutionResultStats struct {
	ComputationUsed                 uint64
	MemoryUsed                      uint64
	EventCounts                     int
	EventSize                       int
	NumberOfRegistersTouched        int
	NumberOfBytesWrittenToRegisters int
	NumberOfCollections             int
	NumberOfTransactions            int
}

func (stats *ExecutionResultStats) Merge(other ExecutionResultStats) {
	stats.ComputationUsed += other.ComputationUsed
	stats.MemoryUsed += other.MemoryUsed
	stats.EventCounts += other.EventCounts
	stats.EventSize += other.EventSize
	stats.NumberOfRegistersTouched += other.NumberOfRegistersTouched
	stats.NumberOfBytesWrittenToRegisters += other.NumberOfBytesWrittenToRegisters
	stats.NumberOfCollections += other.NumberOfCollections
	stats.NumberOfTransactions += other.NumberOfTransactions
}

type ExecutionMetrics interface {
	LedgerMetrics
	RuntimeMetrics
	ProviderMetrics
	WALMetrics

	// StartBlockReceivedToExecuted starts a span to trace the duration of a block
	// from being received for execution to execution being finished
	StartBlockReceivedToExecuted(blockID flow.Identifier)

	// FinishBlockReceivedToExecuted finishes a span to trace the duration of a block
	// from being received for execution to execution being finished
	FinishBlockReceivedToExecuted(blockID flow.Identifier)

	// ExecutionStorageStateCommitment reports the storage size of a state commitment in bytes
	ExecutionStorageStateCommitment(bytes int64)

	// ExecutionLastExecutedBlockHeight reports last executed block height
	ExecutionLastExecutedBlockHeight(height uint64)

	// ExecutionLastFinalizedExecutedBlockHeight reports last finalized and executed block height
	ExecutionLastFinalizedExecutedBlockHeight(height uint64)

	// ExecutionBlockExecuted reports the total time and computation spent on executing a block
	ExecutionBlockExecuted(dur time.Duration, stats ExecutionResultStats)

	// ExecutionBlockExecutionEffortVectorComponent reports the unweighted effort of given ComputationKind at block level
	ExecutionBlockExecutionEffortVectorComponent(string, uint)

	// ExecutionBlockCachedPrograms reports the number of cached programs at the end of a block
	ExecutionBlockCachedPrograms(programs int)

	// ExecutionCollectionExecuted reports the total time and computation spent on executing a collection
	ExecutionCollectionExecuted(dur time.Duration, stats ExecutionResultStats)

	// ExecutionTransactionExecuted reports stats on executing a single transaction
	ExecutionTransactionExecuted(
		dur time.Duration,
		numTxnConflictRetries int,
		compUsed uint64,
		memoryUsed uint64,
		eventCounts int,
		eventSize int,
		failed bool)

	// ExecutionChunkDataPackGenerated reports stats on chunk data pack generation
	ExecutionChunkDataPackGenerated(proofSize, numberOfTransactions int)

	// ExecutionScriptExecuted reports the time and memory spent on executing an script
	ExecutionScriptExecuted(dur time.Duration, compUsed, memoryUsed, memoryEstimate uint64)

	// ExecutionCollectionRequestSent reports when a request for a collection is sent to a collection node
	ExecutionCollectionRequestSent()

	// Unused
	ExecutionCollectionRequestRetried()

	// ExecutionSync reports when the state syncing is triggered or stopped.
	ExecutionSync(syncing bool)

	// Upload metrics
	ExecutionBlockDataUploadStarted()
	ExecutionBlockDataUploadFinished(dur time.Duration)
	ExecutionComputationResultUploaded()
	ExecutionComputationResultUploadRetried()

	UpdateCollectionMaxHeight(height uint64)
}

type BackendScriptsMetrics interface {
	// ScriptExecuted records the round trip time while executing a script
	ScriptExecuted(dur time.Duration, size int)

	// ScriptExecutionErrorLocal records script execution failures from local execution
	ScriptExecutionErrorLocal()

	// ScriptExecutionErrorOnExecutionNode records script execution failures on Execution Nodes
	ScriptExecutionErrorOnExecutionNode()

	// ScriptExecutionResultMismatch records script execution result mismatches between local and
	// execution nodes
	ScriptExecutionResultMismatch()

	// ScriptExecutionResultMatch records script execution result matches between local and
	// execution nodes
	ScriptExecutionResultMatch()

	// ScriptExecutionErrorMismatch records script execution error mismatches between local and
	// execution nodes
	ScriptExecutionErrorMismatch()

	// ScriptExecutionErrorMatch records script execution error matches between local and
	// execution nodes
	ScriptExecutionErrorMatch()

	// ScriptExecutionNotIndexed records script execution matches where data for the block is not
	// indexed locally yet
	ScriptExecutionNotIndexed()
}

type TransactionMetrics interface {
	// Record the round trip time while getting a transaction result
	TransactionResultFetched(dur time.Duration, size int)

	// TransactionReceived starts tracking of transaction execution/finalization/sealing
	TransactionReceived(txID flow.Identifier, when time.Time)

	// TransactionFinalized reports the time spent between the transaction being received and finalized. Reporting only
	// works if the transaction was earlier added as received.
	TransactionFinalized(txID flow.Identifier, when time.Time)

	// TransactionExecuted reports the time spent between the transaction being received and executed. Reporting only
	// works if the transaction was earlier added as received.
	TransactionExecuted(txID flow.Identifier, when time.Time)

	// TransactionExpired tracks number of expired transactions
	TransactionExpired(txID flow.Identifier)

	// TransactionSubmissionFailed should be called whenever we try to submit a transaction and it fails
	TransactionSubmissionFailed()
}

type PingMetrics interface {
	// NodeReachable tracks the round trip time in milliseconds taken to ping a node
	// The nodeInfo provides additional information about the node such as the name of the node operator
	NodeReachable(node *flow.Identity, nodeInfo string, rtt time.Duration)

	// NodeInfo tracks the software version, sealed height and hotstuff view of a node
	NodeInfo(node *flow.Identity, nodeInfo string, version string, sealedHeight uint64, hotstuffCurView uint64)
}

type HeroCacheMetrics interface {
	// BucketAvailableSlots keeps track of number of available slots in buckets of cache.
	BucketAvailableSlots(uint64, uint64)

	// OnKeyPutAttempt is called whenever a new (key, value) pair is attempted to be put in cache.
	// It does not reflect whether the put was successful or not.
	// A (key, value) pair put attempt may fail if the cache is full, or the key already exists.
	OnKeyPutAttempt(size uint32)

	// OnKeyPutSuccess is called whenever a new (key, entity) pair is successfully added to the cache.
	OnKeyPutSuccess(size uint32)

	// OnKeyPutDrop is called whenever a new (key, entity) pair is dropped from the cache due to full cache.
	OnKeyPutDrop()

	// OnKeyPutDeduplicated is tracking the total number of unsuccessful writes caused by adding a duplicate key to the cache.
	// A duplicate key is dropped by the cache when it is written to the cache.
	// Note: in context of HeroCache, the key corresponds to the identifier of its entity. Hence, a duplicate key corresponds to
	// a duplicate entity.
	OnKeyPutDeduplicated()

	// OnKeyRemoved is called whenever a (key, entity) pair is removed from the cache.
	OnKeyRemoved(size uint32)

	// OnKeyGetSuccess tracks total number of successful read queries.
	// A read query is successful if the entity corresponding to its key is available in the cache.
	// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
	OnKeyGetSuccess()

	// OnKeyGetFailure tracks total number of unsuccessful read queries.
	// A read query is unsuccessful if the entity corresponding to its key is not available in the cache.
	// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
	OnKeyGetFailure()

	// OnEntityEjectionDueToFullCapacity is called whenever adding a new (key, entity) to the cache results in ejection of another (key', entity') pair.
	// This normally happens -- and is expected -- when the cache is full.
	// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
	OnEntityEjectionDueToFullCapacity()

	// OnEntityEjectionDueToEmergency is called whenever a bucket is found full and all of its keys are valid, i.e.,
	// each key belongs to an existing (key, entity) pair.
	// Hence, adding a new key to that bucket will replace the oldest valid key inside that bucket.
	// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
	OnEntityEjectionDueToEmergency()
}

type ChainSyncMetrics interface {
	// record pruned blocks. requested and received times might be zero values
	PrunedBlockById(status *chainsync.Status)

	PrunedBlockByHeight(status *chainsync.Status)

	// totalByHeight and totalById are the number of blocks pruned for blocks requested by height and by id
	// storedByHeight and storedById are the number of blocks still stored by height and id
	PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int)

	RangeRequested(ran chainsync.Range)

	BatchRequested(batch chainsync.Batch)
}

type DHTMetrics interface {
	RoutingTablePeerAdded()
	RoutingTablePeerRemoved()
}

package module

import (
	"time"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type EntriesFunc func() uint

// Network Metrics
type NetworkMetrics interface {
	// NetworkMessageSent size in bytes and count of the network message sent
	NetworkMessageSent(sizeBytes int, topic string, messageType string)

	// NetworkMessageReceived size in bytes and count of the network message received
	NetworkMessageReceived(sizeBytes int, topic string, messageType string)

	// NetworkDuplicateMessagesDropped counts number of messages dropped due to duplicate detection
	NetworkDuplicateMessagesDropped(topic string, messageType string)

	// Message receive queue metrics
	// MessageAdded increments the metric tracking the number of messages in the queue with the given priority
	MessageAdded(priority int)

	// MessageRemoved decrements the metric tracking the number of messages in the queue with the given priority
	MessageRemoved(priority int)

	// QueueDuration tracks the time spent by a message with the given priority in the queue
	QueueDuration(duration time.Duration, priority int)

	// InboundProcessDuration tracks the time a queue worker blocked by an engine for processing an incoming message on specified topic (i.e., channel).
	InboundProcessDuration(topic string, duration time.Duration)

	// OutboundConnections updates the metric tracking the number of outbound connections of this node
	OutboundConnections(connectionCount uint)

	// InboundConnections updates the metric tracking the number of inbound connections of this node
	InboundConnections(connectionCount uint)
}

type EngineMetrics interface {
	MessageSent(engine string, message string)
	MessageReceived(engine string, message string)
	MessageHandled(engine string, messages string)
}

type ComplianceMetrics interface {
	FinalizedHeight(height uint64)
	SealedHeight(height uint64)
	BlockFinalized(*flow.Block)
	BlockSealed(*flow.Block)
	BlockProposalDuration(duration time.Duration)
}

type CleanerMetrics interface {
	RanGC(took time.Duration)
}

type CacheMetrics interface {
	// report the total number of cached items
	CacheEntries(resource string, entries uint)
	// report the number of times the queried item is found in the cache
	CacheHit(resource string)
	// report the number of items the queried item is not found in the cache, nor found in the database
	CacheNotFound(resource string)
	// report the number of items the queried item is not found in the cache, but found in the database
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

	// HotStuffWaitDuration reports Metrics C6 HotStuff Idle Duration
	HotStuffWaitDuration(duration time.Duration, event string)

	// SetCurView reports Metrics C8: Current View
	SetCurView(view uint64)

	// SetQCView reports Metrics C9: View of Newest Known QC
	SetQCView(view uint64)

	// CountSkipped reports the number of times we skipped ahead.
	CountSkipped()

	// CountTimeout reports the number of times we timed out.
	CountTimeout()

	// SetTimeout sets the current timeout duration
	SetTimeout(duration time.Duration)

	// CommitteeProcessingDuration measures the time which the HotStuff's core logic
	// spends in the hotstuff.Committee component, i.e. the time determining consensus
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
	// TODO: remove this event handlers once we have new architecture in place.
	// OnExecutionReceiptReceived is called whenever a new execution receipt arrives
	// at Finder engine. It increments total number of received receipts.
	OnExecutionReceiptReceived()
	// OnExecutionResultSent is called whenever a new execution result is sent by
	// Finder engine to the match engine. It increments total number of sent execution results.
	OnExecutionResultSent()
	// OnExecutionResultReceived is called whenever a new execution result is successfully received
	// by Match engine from Finder engine.
	// It increments the total number of received execution results.
	OnExecutionResultReceived()
	// OnVerifiableChunkSent is called on a successful submission of matched chunk
	// by Match engine to Verifier engine.
	// It increments the total number of chunks matched by match engine.
	OnVerifiableChunkSent()
	// OnChunkDataPackReceived is called on a receiving a chunk data pack by Match engine
	// It increments the total number of chunk data packs received.
	OnChunkDataPackReceived()
	// OnChunkDataPackRequested is called on requesting a chunk data pack by Match engine
	// It increments the total number of chunk data packs requested.
	OnChunkDataPackRequested()

	// OnVerifiableChunkReceivedAtVerifierEngine increments a counter that keeps track of number of verifiable chunks received at
	// verifier engine from fetcher engine.
	OnVerifiableChunkReceivedAtVerifierEngine()

	// OnResultApprovalDispatchedInNetwork increments a counter that keeps track of number of result approvals dispatched in the network
	// by verifier engine.
	OnResultApprovalDispatchedInNetwork()

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
	OnChunkDataPackRequestDispatchedInNetwork()

	// OnChunkDataPackResponseReceivedFromNetwork increments a counter that keeps track of number of chunk data pack responses that the
	// requester engine receives from execution nodes (through network).
	OnChunkDataPackResponseReceivedFromNetwork()

	// OnChunkDataPackSentToFetcher increments a counter that keeps track of number of chunk data packs sent to the fetcher engine from
	// requester engine.
	OnChunkDataPackSentToFetcher()

	// OnChunkDataPackArrivedAtFetcher increments a counter that keeps track of number of chunk data packs arrived at fetcher engine from
	// requester engine.
	OnChunkDataPackArrivedAtFetcher()

	// OnVerifiableChunkSentToVerifier increments a counter that keeps track of number of verifiable chunks fetcher engine sent to verifier engine.
	OnVerifiableChunkSentToVerifier()
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
	LatestTrieRegCountDiff(number uint64)

	// LatestTrieMaxDepth records the maximum depth of the last created trie
	LatestTrieMaxDepth(number uint64)

	// LatestTrieMaxDepthDiff records the difference between the max depth of the latest created trie and parent trie
	LatestTrieMaxDepthDiff(number uint64)

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
	// DiskSize records the amount of disk space used by the storage (in bytes)
	DiskSize(uint64)
}

type RuntimeMetrics interface {
	// TransactionParsed reports the time spent parsing a single transaction
	TransactionParsed(dur time.Duration)

	// TransactionChecked reports the time spent checking a single transaction
	TransactionChecked(dur time.Duration)

	// TransactionInterpreted reports the time spent interpreting a single transaction
	TransactionInterpreted(dur time.Duration)
}

type ProviderMetrics interface {
	// ChunkDataPackRequested is executed every time a chunk data pack request is arrived at execution node.
	// It increases the request counter by one.
	ChunkDataPackRequested()
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

	// ExecutionComputationUsedPerBlock reports computation used per block
	ExecutionComputationUsedPerBlock(computation uint64)

	// ExecutionStateReadsPerBlock reports number of state access/read operations per block
	ExecutionStateReadsPerBlock(reads uint64)

	// ExecutionStorageStateCommitment reports the storage size of a state commitment in bytes
	ExecutionStorageStateCommitment(bytes int64)

	// ExecutionLastExecutedBlockHeight reports last executed block height
	ExecutionLastExecutedBlockHeight(height uint64)

	// ExecutionTotalExecutedTransactions adds num to the total number of executed transactions
	ExecutionTotalExecutedTransactions(numExecuted int)

	// ExecutionCollectionRequestSent reports when a request for a collection is sent to a collection node
	ExecutionCollectionRequestSent()

	// Unused
	ExecutionCollectionRequestRetried()

	// ExecutionSync reports when the state syncing is triggered or stopped.
	ExecutionSync(syncing bool)
}

type TransactionMetrics interface {
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

	// NodeInfo tracks the software version and sealed height of a node
	NodeInfo(node *flow.Identity, nodeInfo string, version string, sealedHeight uint64)
}

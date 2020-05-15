package module

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type NetworkMetrics interface {
	// Network Metrics
	// NetworkMessageSent size in bytes and count of the network message sent
	NetworkMessageSent(sizeBytes int, topic string)

	// Network Metrics
	// NetworkMessageReceived size in bytes and count of the network message received
	NetworkMessageReceived(sizeBytes int, topic string)
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
}

type CacheMetrics interface {
	CacheEntries(resource string, entries uint)
	CacheHit(resource string)
	CacheMiss(resource string)
}

type MempoolMetrics interface {
	MempoolEntries(resource string, entries uint)
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
}

type CollectionMetrics interface {
	// TransactionReceived is called when a new transaction is ingested by the
	// node. It increments the total count of ingested transactions and starts
	// a tx->col span for the transaction.
	TransactionReceived(txID flow.Identifier)

	// CollectionProposed is called when a new collection is proposed by us or
	// any other node in the cluster.
	CollectionProposed(collection flow.LightCollection)

	// CollectionGuaranteed is called when a collection is finalized.
	CollectionGuaranteed(collection flow.LightCollection)

	// PendingClusterBlocks the number of cluster blocks in the pending cache.
	PendingClusterBlocks(n uint)
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
}

type VerificationMetrics interface {
	// OnChunkVerificationStarted is called whenever the verification of a chunk is started
	// it starts the timer to record the execution time
	OnChunkVerificationStarted(chunkID flow.Identifier)

	// OnChunkVerificationFinished is called whenever chunkID verification gets finished
	// it records the duration of execution and increases number of checked chunks
	OnChunkVerificationFinished(chunkID flow.Identifier)

	// OnResultApproval is called whenever a result approval for is emitted
	// it increases the result approval counter for this chunk
	OnResultApproval()

	// OnChunkDataAdded is called whenever something is added to related to chunkID to the in-memory mempools
	// of verification node. It records the size of stored object.
	OnChunkDataAdded(chunkID flow.Identifier, size float64)

	// OnChunkDataRemoved is called whenever something is removed that is related to chunkID from the in-memory mempools
	// of verification node. It records the size of stored object.
	OnChunkDataRemoved(chunkID flow.Identifier, size float64)
}

// LedgerMetrics provides an interface to record Ledger Storage metrics.
// Ledger storage is non-linear (fork-aware) so certain metrics are averaged
// and computed before emitting for better visibility
type LedgerMetrics interface {
	// ForestApproxMemorySize records approximate memory usage of forest (all in-memory trees)
	ForestApproxMemorySize(bytes uint64)

	// ForestNumberOfTrees current number of trees in a forest (in memory)
	ForestNumberOfTrees(number uint64)

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

type ExecutionMetrics interface {
	LedgerMetrics

	// StartBlockReceivedToExecuted starts a span to trace the duration of a block
	// from being received for execution to execution being finished
	StartBlockReceivedToExecuted(blockID flow.Identifier)

	// FinishBlockReceivedToExecuted finishes a span to trace the duration of a block
	// from being received for execution to execution being finished
	FinishBlockReceivedToExecuted(blockID flow.Identifier)

	// ExecutionGasUsedPerBlock reports gas used per block
	ExecutionGasUsedPerBlock(gas uint64)

	// ExecutionStateReadsPerBlock reports number of state access/read operations per block
	ExecutionStateReadsPerBlock(reads uint64)

	// ExecutionStateStorageDiskTotal reports the total storage size of the execution state on disk in bytes
	ExecutionStateStorageDiskTotal(bytes int64)

	// ExecutionStorageStateCommitment reports the storage size of a state commitment in bytes
	ExecutionStorageStateCommitment(bytes int64)

	// ExecutionLastExecutedBlockView reports last executed block view
	ExecutionLastExecutedBlockView(view uint64)

	// ExecutionTotalExecutedTransactions adds num to the total number of executed transactions
	ExecutionTotalExecutedTransactions(numExecuted int)
}

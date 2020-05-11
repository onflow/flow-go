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
}

type BadgerMetrics interface {
	// BadgerLSMSize total size on-disk of the badger database.
	BadgerLSMSize(sizeBytes int64)
	BadgerVLogSize(sizeBytes int64)

	// Badger built-in metrics (from badger/y/metrics.go)
	BadgerNumReads(n int64)
	BadgerNumWrites(n int64)
	BadgerNumBytesRead(n int64)
	BadgerNumBytesWritten(n int64)
	BadgerNumGets(n int64)
	BadgerNumPuts(n int64)
	BadgerNumBlockedPuts(n int64)
	BadgerNumMemtableGets(n int64)
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

type ExecutionMetrics interface {
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
}

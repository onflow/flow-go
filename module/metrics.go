package module

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Metrics interface {
	// Common Metrics
	//

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

	// Network Metrics
	// NetworkMessageSent size in bytes and count of the network message sent
	NetworkMessageSent(sizeBytes int, topic string)

	// Network Metrics
	// NetworkMessageReceived size in bytes and count of the network message received
	NetworkMessageReceived(sizeBytes int, topic string)

	// Collection Metrics
	//

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

	// Consensus Metrics
	//

	// PendingBlocks the number of blocks in the pending cache.
	PendingBlocks(n uint)

	// StartCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
	StartCollectionToFinalized(collectionID flow.Identifier)

	// FinishCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
	FinishCollectionToFinalized(collectionID flow.Identifier)

	// CollectionsInFinalizedBlock reports Metric C2: Counter: Number of Collections included in finalized Blocks (per second)
	CollectionsInFinalizedBlock(count int)

	// CollectionsPerBlock reports Metric C3: Gauge type: number of Collections per incorporated Block
	CollectionsPerBlock(count int)

	// StartBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
	StartBlockToSeal(blockID flow.Identifier)

	// FinishBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
	FinishBlockToSeal(blockID flow.Identifier)

	// SealsInFinalizedBlock reports Metrics C5 Number of Blocks which are sealed by finalized blocks (per second)
	SealsInFinalizedBlock(count int)

	// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
	HotStuffBusyDuration(duration time.Duration, event string)

	// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
	HotStuffIdleDuration(duration time.Duration)

	// HotStuffWaitDuration reports Metrics C6 HotStuff Idle Duration
	HotStuffWaitDuration(duration time.Duration, event string)

	// FinalizedBlocks reports Metric C7: Number of Blocks Finalized (per second)
	FinalizedBlocks(count int)

	// StartNewView reports Metrics C8: Current View
	StartNewView(view uint64)

	// NewestKnownQC reports Metrics C9: View of Newest Known QC
	NewestKnownQC(view uint64)

	// MadeBlockProposal reports that a block proposal has been made
	MadeBlockProposal()

	// MempoolGuaranteesSize reports the size of the guarantees mempool
	MempoolGuaranteesSize(size uint)

	// MempoolReceiptsSize reports the size of the receipts mempool
	MempoolReceiptsSize(size uint)

	// MempoolApprovalsSize reports the size of the approvals mempool
	MempoolApprovalsSize(size uint)

	// MempoolSealsSize reports the size of the seals mempool
	MempoolSealsSize(size uint)

	// Verification Metrics
	//

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

	// Execution Metrics

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

	// ExecutionTransactionParsed reports the parse time of a transaction
	ExecutionTransactionParsed(dur time.Duration)

	// ExecutionTransactionChecked reports the checking time of a transaction
	ExecutionTransactionChecked(dur time.Duration)

	// ExecutionTransactionInterpreted reports the interpretation time of a transaction
	ExecutionTransactionInterpreted(dur time.Duration)
}

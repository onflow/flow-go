package module

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Metrics interface {

	// Common Metrics
	//

	// BadgerDBSize total size on-disk of the badger database.
	BadgerDBSize(sizeBytes int64)

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

	// Consensus Metrics
	//

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
	HotStuffBusyDuration(duration time.Duration)

	// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
	HotStuffIdleDuration(duration time.Duration)

	// FinalizedBlocks reports Metric C7: Number of Blocks Finalized (per second)
	FinalizedBlocks(count int)

	// StartNewView reports Metrics C8: Current View
	StartNewView(view uint64)

	// NewestKnownQC reports Metrics C9: View of Newest Known QC
	NewestKnownQC(view uint64)

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
}

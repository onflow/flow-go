package module

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Metrics interface {
	// Collection Metrics

	// StartCollectionToGuarantee starts a span to trace the duration of a collection
	// from being created to being submitted as a colleciton guarantee
	StartCollectionToGuarantee(collection flow.LightCollection)

	// FinishCollectionToGuarantee finishes a span to trace the duration of a collection
	// from being created to being submitted as a colleciton guarantee
	FinishCollectionToGuarantee(collectionID flow.Identifier)

	// StartTransactionToCollectionGuarantee starts a span to trace the duration of a transaction
	// from being created to being included as part of a collection guarantee
	StartTransactionToCollectionGuarantee(txID flow.Identifier)

	// FinishTransactionToCollectionGuarantee finishes a span to trace the duration of a transaction
	// from being created to being included as part of a collection guarantee
	FinishTransactionToCollectionGuarantee(txID flow.Identifier)

	// Consensus Metrics

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
}

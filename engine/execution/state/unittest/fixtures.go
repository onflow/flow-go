package unittest

import (
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

func StateInteractionsFixture() *delta.SpockSnapshot {
	return delta.NewView(nil).Interactions()
}

func ComputationResultFixture(collectionsSignerIDs [][]flow.Identifier) *execution.ComputationResult {
	stateViews := make([]*delta.SpockSnapshot, len(collectionsSignerIDs)+1) //+1 for system chunk
	stateCommitments := make([]flow.StateCommitment, len(collectionsSignerIDs)+1)
	eventHashes := make([]flow.Identifier, len(collectionsSignerIDs)+1)
	proofs := make([][]byte, len(collectionsSignerIDs)+1)
	for i := 0; i < len(collectionsSignerIDs)+1; i++ {
		stateViews[i] = StateInteractionsFixture()
		stateCommitments[i] = unittest.StateCommitmentFixture()
		eventHashes[i] = unittest.IdentifierFixture()
		proofs[i] = unittest.RandomBytes(2)
	}
	return &execution.ComputationResult{
		ExecutableBlock:  unittest.ExecutableBlockFixture(collectionsSignerIDs),
		StateSnapshots:   stateViews,
		StateCommitments: stateCommitments,
		EventsHashes:     eventHashes,
		Proofs:           proofs,
	}
}

func ComputationResultForBlockFixture(completeBlock *entity.ExecutableBlock) *execution.ComputationResult {
	n := len(completeBlock.CompleteCollections) + 1
	stateViews := make([]*delta.SpockSnapshot, n)
	stateCommitments := make([]flow.StateCommitment, n)
	proofs := make([][]byte, n)
	events := make([]flow.EventsList, n)
	eventHashes := make([]flow.Identifier, n)
	for i := 0; i < n; i++ {
		stateViews[i] = StateInteractionsFixture()
		stateCommitments[i] = *completeBlock.StartState
		proofs[i] = unittest.RandomBytes(6)
		events[i] = make(flow.EventsList, 0)
		eventHashes[i] = unittest.IdentifierFixture()
	}
	return &execution.ComputationResult{
		ExecutableBlock:  completeBlock,
		StateSnapshots:   stateViews,
		StateCommitments: stateCommitments,
		Proofs:           proofs,
		Events:           events,
		EventsHashes:     eventHashes,
	}
}

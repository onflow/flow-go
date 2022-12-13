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
	block := unittest.ExecutableBlockFixture(collectionsSignerIDs)
	startState := unittest.StateCommitmentFixture()
	block.StartState = &startState

	return ComputationResultForBlockFixture(block)
}

func ComputationResultForBlockFixture(
	completeBlock *entity.ExecutableBlock,
) *execution.ComputationResult {
	numChunks := len(completeBlock.CompleteCollections) + 1
	stateViews := make([]*delta.SpockSnapshot, numChunks)
	stateCommitments := make([]flow.StateCommitment, numChunks)
	proofs := make([][]byte, numChunks)
	events := make([]flow.EventsList, numChunks)
	eventHashes := make([]flow.Identifier, numChunks)
	for i := 0; i < numChunks; i++ {
		stateViews[i] = StateInteractionsFixture()
		stateCommitments[i] = *completeBlock.StartState
		proofs[i] = unittest.RandomBytes(6)
		events[i] = make(flow.EventsList, 0)
		eventHashes[i] = unittest.IdentifierFixture()
	}
	return &execution.ComputationResult{
		TransactionResultIndex: make([]int, numChunks),
		ExecutableBlock:        completeBlock,
		StateSnapshots:         stateViews,
		StateCommitments:       stateCommitments,
		Proofs:                 proofs,
		Events:                 events,
		EventsHashes:           eventHashes,
	}
}

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
	stateViews := make([]*delta.SpockSnapshot, len(collectionsSignerIDs))
	for i := 0; i < len(collectionsSignerIDs); i++ {
		stateViews[i] = StateInteractionsFixture()
	}
	return &execution.ComputationResult{
		ExecutableBlock: unittest.ExecutableBlockFixture(collectionsSignerIDs),
		StateSnapshots:  stateViews,
	}
}

func ComputationResultForBlockFixture(completeBlock *entity.ExecutableBlock) *execution.ComputationResult {
	n := len(completeBlock.CompleteCollections)
	stateViews := make([]*delta.SpockSnapshot, n)
	for i := 0; i < n; i++ {
		stateViews[i] = StateInteractionsFixture()
	}
	return &execution.ComputationResult{
		ExecutableBlock: completeBlock,
		StateSnapshots:  stateViews,
	}
}

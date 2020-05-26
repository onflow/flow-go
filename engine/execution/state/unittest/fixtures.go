package unittest

import (
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func StateInteractionsFixture() *delta.Snapshot {
	return delta.NewView(nil).Interactions()
}

func ComputationResultFixture(collectionsSignerIDs [][]flow.Identifier) *execution.ComputationResult {
	stateViews := make([]*delta.Snapshot, len(collectionsSignerIDs))
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
	stateViews := make([]*delta.Snapshot, n)
	for i := 0; i < n; i++ {
		stateViews[i] = StateInteractionsFixture()
	}
	return &execution.ComputationResult{
		ExecutableBlock: completeBlock,
		StateSnapshots:  stateViews,
	}
}

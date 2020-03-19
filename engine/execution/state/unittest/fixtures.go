package unittest

import (
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func EmptyView() *state.View {
	view := state.NewView(func(key flow.RegisterID) (bytes []byte, e error) {
		return nil, nil
	})

	bootstrap.BootstrapView(view) //create genesis state

	return view.NewChild() //return new view
}

func StateViewFixture() *state.View {
	return state.NewView(func(key flow.RegisterID) (bytes []byte, err error) {
		return nil, nil
	})
}
func ComputationResultFixture(n int) *execution.ComputationResult {
	stateViews := make([]*state.View, n)
	for i := 0; i < n; i++ {
		stateViews[i] = StateViewFixture()
	}
	return &execution.ComputationResult{
		CompleteBlock: unittest.CompleteBlockFixture(n),
		StateViews:    stateViews,
	}
}

func ComputationResultForBlockFixture(completeBlock *entity.ExecutableBlock) *execution.ComputationResult {
	n := len(completeBlock.CompleteCollections)
	stateViews := make([]*state.View, n)
	for i := 0; i < n; i++ {
		stateViews[i] = StateViewFixture()
	}
	return &execution.ComputationResult{
		CompleteBlock: completeBlock,
		StateViews:    stateViews,
	}
}

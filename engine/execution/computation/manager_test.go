package computation

import (
	"context"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/unittest"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	module "github.com/dapperlabs/flow-go/module/mock"
)

func TestComputeBlockWithStorage(t *testing.T) {

	tx1 := testutil.DeployCounterContractTransaction()
	tx2 := testutil.CreateCounterTransaction()

	err := testutil.SignTransactionByRoot(&tx1, 0)
	require.NoError(t, err)
	err = testutil.SignTransactionByRoot(&tx2, 1)
	require.NoError(t, err)

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signature:    nil,
	}

	block := flow.Block{
		Header: &flow.Header{
			View: 42,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	executableBlock := &entity.ExecutableBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
	}

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	rt := runtime.NewInterpreterRuntime()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)

	blockComputer := computer.NewBlockComputer(vm, nil, nil)

	engine := &Manager{
		blockComputer: blockComputer,
		me:            me,
	}

	view := unittest.EmptyView()

	require.Empty(t, view.Delta())

	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, view)
	require.NoError(t, err)

	require.NotEmpty(t, view.Delta())
	require.Len(t, returnedComputationResult.StateSnapshots, 1)
	assert.NotEmpty(t, returnedComputationResult.StateSnapshots[0].Delta)
}

package computation

import (
	"context"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	module "github.com/dapperlabs/flow-go/module/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestComputeBlockWithStorage(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)

	privateKeys, err := testutil.GenerateAccountPrivateKeys(2)
	require.NoError(t, err)

	ledger := testutil.RootBootstrappedLedger()
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys)
	require.NoError(t, err)

	tx1 := testutil.DeployCounterContractTransaction(accounts[0])
	tx1.SetProposalKey(flow.ServiceAddress(), 0, 0).
		SetPayer(flow.ServiceAddress())

	err = testutil.SignPayload(tx1, accounts[0], privateKeys[0])
	require.NoError(t, err)

	err = testutil.SignEnvelope(tx1, flow.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	tx2 := testutil.CreateCounterTransaction(accounts[0], accounts[1])
	tx2.SetProposalKey(flow.ServiceAddress(), 0, 0).
		SetPayer(flow.ServiceAddress())

	err = testutil.SignPayload(tx2, accounts[1], privateKeys[1])
	require.NoError(t, err)

	err = testutil.SignEnvelope(tx2, flow.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	transactions := []*flow.TransactionBody{tx1, tx2}

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

	blockComputer := computer.NewBlockComputer(vm, new(storage.Blocks), nil, nil)

	engine := &Manager{
		blockComputer: blockComputer,
		me:            me,
	}

	view := delta.NewView(ledger.Get)
	blockView := view.NewChild()

	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, blockView)
	require.NoError(t, err)

	require.NotEmpty(t, blockView.Delta())
	require.Len(t, returnedComputationResult.StateSnapshots, 1)
	assert.NotEmpty(t, returnedComputationResult.StateSnapshots[0].Delta)
}

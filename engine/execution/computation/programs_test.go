package computation

import (
	"context"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPrograms(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	chain := flow.Mainnet.Chain()
	vm := fvm.New(rt)
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(2)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, fvm.NewEmptyPrograms(), privateKeys, chain)
	require.NoError(t, err)

	// first test ---- TODO add details -- test update logic
	// setup transactions

	account := accounts[0]
	privKey := privateKeys[0]
	// tx1 deploys contract version 1
	tx1 := testutil.DeployEventContractTransaction(account, chain, 1)
	tx1.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// tx2 calls the method of the contract (version 1)
	tx2 := testutil.CreateEmitEventTransaction(account, account)
	tx2.SetProposalKey(chain.ServiceAddress(), 0, 1).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(tx2, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(tx2, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// tx3 updates the contract to version 2
	tx3 := testutil.UpdateEventContractTransaction(account, chain, 2)
	tx3.SetProposalKey(chain.ServiceAddress(), 0, 2).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(tx3, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(tx3, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// tx4 calls the method of the contract (version 2)
	tx4 := testutil.CreateEmitEventTransaction(account, account)
	tx4.SetProposalKey(chain.ServiceAddress(), 0, 3).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(tx4, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(tx4, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	transactions := []*flow.TransactionBody{tx1, tx2, tx3, tx4}

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

	blockComputer, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
	require.NoError(t, err)

	programsCache, err := NewProgramsCache(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer: blockComputer,
		me:            me,
		programsCache: programsCache,
	}

	view := delta.NewView(ledger.Get)
	blockView := view.NewChild()

	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, blockView)
	require.NoError(t, err)

	// first event should be contract deployed
	assert.EqualValues(t, "flow.AccountContractAdded", returnedComputationResult.Events[0].Type)

	hasValidEventValue(t, returnedComputationResult.Events[1], 1)
	// third event should be contract updated
	assert.EqualValues(t, "flow.AccountContractUpdated", returnedComputationResult.Events[2].Type)

	hasValidEventValue(t, returnedComputationResult.Events[3], 2)
}

func hasValidEventValue(t *testing.T, event flow.Event, value int) {
	data, err := jsoncdc.Decode(event.Payload)
	require.NoError(t, err)
	assert.Equal(t, int16(value), data.(cadence.Event).Fields[0].ToGoValue())
}

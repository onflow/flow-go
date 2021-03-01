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

func TestPrograms_TestContractUpdates(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	chain := flow.Mainnet.Chain()
	vm := fvm.New(rt)
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, fvm.NewEmptyPrograms(), privateKeys, chain)
	require.NoError(t, err)

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
			View: 26,
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

	// second event should have a value of 1 (since is calling version 1 of contract)
	hasValidEventValue(t, returnedComputationResult.Events[1], 1)

	// third event should be contract updated
	assert.EqualValues(t, "flow.AccountContractUpdated", returnedComputationResult.Events[2].Type)

	// 4th event should have a value of 2 (since is calling version 2 of contract)
	hasValidEventValue(t, returnedComputationResult.Events[3], 2)
}

func TestPrograms_TestBlockForks(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	chain := flow.Mainnet.Chain()
	vm := fvm.New(rt)
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, fvm.NewEmptyPrograms(), privateKeys, chain)
	require.NoError(t, err)

	// setup transactions
	account := accounts[0]
	privKey := privateKeys[0]

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
	block1View := view.NewChild()

	// Block1 (empty block)
	//     -> Block11 (deploy contract v1)
	//         -> Block111  (emit event - version should be 1), (update contract v3)
	//             -> Block1111   (emit event - version should be 3)
	//	       -> Block112 (emit event - version should be 1), (update contract v4)
	//             -> Block1121  (emit event - version should be 4)
	//     -> Block12 (deploy contract v2)
	//         -> Block121 (emit event - version should be 2)

	// form block 1
	block1 := flow.Block{
		Header: &flow.Header{
			View: 1,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{},
		},
	}
	executableBlock := &entity.ExecutableBlock{
		Block: &block1,
	}
	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block1View)
	require.NoError(t, err)

	// form block 11
	// 	deploys contract version 1
	block11tx1 := testutil.DeployEventContractTransaction(account, chain, 1)
	block11tx1.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block11tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block11tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	txs11 := []*flow.TransactionBody{block11tx1}
	col11 := flow.Collection{Transactions: txs11}
	guarantee11 := flow.CollectionGuarantee{
		CollectionID: col11.ID(),
		Signature:    nil,
	}
	block11 := flow.Block{
		Header: &flow.Header{
			ParentID: block1.ID(),
			View:     2,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee11},
		},
	}
	block11View := block1View.NewChild()
	executableBlock = &entity.ExecutableBlock{
		Block: &block11,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee11.ID(): {
				Guarantee:    &guarantee11,
				Transactions: txs11,
			},
		},
	}
	returnedComputationResult, err = engine.ComputeBlock(context.Background(), executableBlock, block11View)
	require.NoError(t, err)
	// 1st event should be contract deployed
	assert.EqualValues(t, "flow.AccountContractAdded", returnedComputationResult.Events[0].Type)

	// form block 111
	// emit event (should be v1)
	block111ExpectedValue := 1
	block111tx1 := testutil.CreateEmitEventTransaction(account, account)
	block111tx1.SetProposalKey(chain.ServiceAddress(), 0, 1).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block111tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block111tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// update contract version 3
	block111tx2 := testutil.UpdateEventContractTransaction(account, chain, 3)
	block111tx2.SetProposalKey(chain.ServiceAddress(), 0, 2).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block111tx2, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block111tx2, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	col111 := flow.Collection{Transactions: []*flow.TransactionBody{block111tx1, block111tx2}}
	guarantee111 := flow.CollectionGuarantee{
		CollectionID: col111.ID(),
		Signature:    nil,
	}
	block111 := flow.Block{
		Header: &flow.Header{
			ParentID: block11.ID(),
			View:     3,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee111},
		},
	}
	block111View := block11View.NewChild()
	executableBlock = &entity.ExecutableBlock{
		Block: &block111,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee111.ID(): {
				Guarantee:    &guarantee111,
				Transactions: col111.Transactions,
			},
		},
	}
	returnedComputationResult, err = engine.ComputeBlock(context.Background(), executableBlock, block111View)
	require.NoError(t, err)

	// 1st event
	hasValidEventValue(t, returnedComputationResult.Events[0], block111ExpectedValue)
	// second event should be contract deployed
	assert.EqualValues(t, "flow.AccountContractUpdated", returnedComputationResult.Events[1].Type)

	// form block 1111
	// emit event (expected v3)
	block1111ExpectedValue := 3
	block1111tx1 := testutil.CreateEmitEventTransaction(account, account)
	block1111tx1.SetProposalKey(chain.ServiceAddress(), 0, 3).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block1111tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block1111tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	col1111 := flow.Collection{Transactions: []*flow.TransactionBody{block1111tx1}}
	guarantee1111 := flow.CollectionGuarantee{
		CollectionID: col1111.ID(),
		Signature:    nil,
	}
	block1111 := flow.Block{
		Header: &flow.Header{
			ParentID: block111.ID(),
			View:     4,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee1111},
		},
	}
	block1111View := block111View.NewChild()
	executableBlock = &entity.ExecutableBlock{
		Block: &block1111,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee1111.ID(): {
				Guarantee:    &guarantee1111,
				Transactions: col1111.Transactions,
			},
		},
	}
	returnedComputationResult, err = engine.ComputeBlock(context.Background(), executableBlock, block1111View)
	require.NoError(t, err)

	// 1st event
	hasValidEventValue(t, returnedComputationResult.Events[0], block1111ExpectedValue)

	// form block 112
	// emit event (expected 1)
	block112ExpectedValue := 1
	block112tx1 := testutil.CreateEmitEventTransaction(account, account)
	block112tx1.SetProposalKey(chain.ServiceAddress(), 0, 1).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block112tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block112tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// update contract version 4
	block112tx2 := testutil.UpdateEventContractTransaction(account, chain, 4)
	block112tx2.SetProposalKey(chain.ServiceAddress(), 0, 2).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block112tx2, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block112tx2, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	col112 := flow.Collection{Transactions: []*flow.TransactionBody{block112tx1, block112tx2}}
	guarantee112 := flow.CollectionGuarantee{
		CollectionID: col112.ID(),
		Signature:    nil,
	}
	block112 := flow.Block{
		Header: &flow.Header{
			ParentID: block11.ID(),
			View:     3,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee112},
		},
	}
	block112View := block11View.NewChild()
	executableBlock = &entity.ExecutableBlock{
		Block: &block112,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee112.ID(): {
				Guarantee:    &guarantee112,
				Transactions: col112.Transactions,
			},
		},
	}
	returnedComputationResult, err = engine.ComputeBlock(context.Background(), executableBlock, block112View)
	require.NoError(t, err)
	// 1st event
	hasValidEventValue(t, returnedComputationResult.Events[0], block112ExpectedValue)
	// second event should be contract deployed
	assert.EqualValues(t, "flow.AccountContractUpdated", returnedComputationResult.Events[1].Type)

	// form block 1121
	// emit event (expected 4)
	block1121ExpectedValue := 4
	block1121tx1 := testutil.CreateEmitEventTransaction(account, account)
	block1121tx1.SetProposalKey(chain.ServiceAddress(), 0, 3).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block1121tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block1121tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	col1121 := flow.Collection{Transactions: []*flow.TransactionBody{block1121tx1}}
	guarantee1121 := flow.CollectionGuarantee{
		CollectionID: col1121.ID(),
		Signature:    nil,
	}
	block1121 := flow.Block{
		Header: &flow.Header{
			ParentID: block112.ID(),
			View:     4,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee1121},
		},
	}
	block1121View := block112View.NewChild()
	executableBlock = &entity.ExecutableBlock{
		Block: &block1121,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee1121.ID(): {
				Guarantee:    &guarantee1121,
				Transactions: col1121.Transactions,
			},
		},
	}
	returnedComputationResult, err = engine.ComputeBlock(context.Background(), executableBlock, block1121View)
	require.NoError(t, err)

	// 1st event
	hasValidEventValue(t, returnedComputationResult.Events[0], block1121ExpectedValue)

	// form block 12
	// 	deploys contract version 2
	block12tx1 := testutil.DeployEventContractTransaction(account, chain, 2)
	block12tx1.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block12tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block12tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	col12 := flow.Collection{Transactions: []*flow.TransactionBody{block12tx1}}
	guarantee12 := flow.CollectionGuarantee{
		CollectionID: col12.ID(),
		Signature:    nil,
	}
	block12 := flow.Block{
		Header: &flow.Header{
			ParentID: block1.ID(),
			View:     2,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee12},
		},
	}

	block12View := block1View.NewChild()
	executableBlock = &entity.ExecutableBlock{
		Block: &block12,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee12.ID(): {
				Guarantee:    &guarantee12,
				Transactions: col12.Transactions,
			},
		},
	}
	returnedComputationResult, err = engine.ComputeBlock(context.Background(), executableBlock, block12View)
	require.NoError(t, err)
	assert.EqualValues(t, "flow.AccountContractAdded", returnedComputationResult.Events[0].Type)

	// form block 121
	// emit event (should be v2)
	block121ExpectedValue := 2
	block121tx1 := testutil.CreateEmitEventTransaction(account, account)
	block121tx1.SetProposalKey(chain.ServiceAddress(), 0, 1).
		SetPayer(chain.ServiceAddress())
	err = testutil.SignPayload(block121tx1, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(block121tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	col121 := flow.Collection{Transactions: []*flow.TransactionBody{block121tx1}}
	guarantee121 := flow.CollectionGuarantee{
		CollectionID: col121.ID(),
		Signature:    nil,
	}
	block121 := flow.Block{
		Header: &flow.Header{
			ParentID: block12.ID(),
			View:     3,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee121},
		},
	}
	block121View := block12View.NewChild()
	executableBlock = &entity.ExecutableBlock{
		Block: &block121,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee121.ID(): {
				Guarantee:    &guarantee121,
				Transactions: col121.Transactions,
			},
		},
	}
	returnedComputationResult, err = engine.ComputeBlock(context.Background(), executableBlock, block121View)
	require.NoError(t, err)
	// 1st event
	hasValidEventValue(t, returnedComputationResult.Events[0], block121ExpectedValue)

}

func hasValidEventValue(t *testing.T, event flow.Event, value int) {
	data, err := jsoncdc.Decode(event.Payload)
	require.NoError(t, err)
	assert.Equal(t, int16(value), data.(cadence.Event).Fields[0].ToGoValue())
}

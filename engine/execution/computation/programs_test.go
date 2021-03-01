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
	prepareTx(t, tx1, account, privKey, 0, chain)

	// tx2 calls the method of the contract (version 1)
	tx2 := testutil.CreateEmitEventTransaction(account, account)
	prepareTx(t, tx2, account, privKey, 1, chain)

	// tx3 updates the contract to version 2
	tx3 := testutil.UpdateEventContractTransaction(account, chain, 2)
	prepareTx(t, tx3, account, privKey, 2, chain)

	// tx4 calls the method of the contract (version 2)
	tx4 := testutil.CreateEmitEventTransaction(account, account)
	prepareTx(t, tx4, account, privKey, 3, chain)

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

// TestPrograms_TestBlockForks tests the functionality of
// programsCache under contract deployment and contract updates on
// different block forks
//
// block structure and operations
// Block1 (empty block)
//     -> Block11 (deploy contract v1)
//         -> Block111  (emit event - version should be 1) and (update contract to v3)
//             -> Block1111   (emit event - version should be 3)
//	       -> Block112 (emit event - version should be 1) and (update contract to v4)
//             -> Block1121  (emit event - version should be 4)
//     -> Block12 (deploy contract v2)
//         -> Block121 (emit event - version should be 2)
func TestPrograms_TestBlockForks(t *testing.T) {
	// setup
	rt := runtime.NewInterpreterRuntime()
	chain := flow.Mainnet.Chain()
	vm := fvm.New(rt)
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, fvm.NewEmptyPrograms(), privateKeys, chain)
	require.NoError(t, err)

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

	var block1, block11, block111, block112, block1121, block1111, block12, block121 flow.Block
	var block1View, block11View, block111View, block112View, block1121View, block1111View, block12View, block121View *delta.View
	t.Run("executing block1 (no collection)", func(t *testing.T) {
		block1 = flow.Block{
			Header: &flow.Header{
				View: 1,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{},
			},
		}
		block1View = view.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block1,
		}
		_, err := engine.ComputeBlock(context.Background(), executableBlock, block1View)
		require.NoError(t, err)
	})

	t.Run("executing block11 (deploys contract version 1)", func(t *testing.T) {
		block11tx1 := testutil.DeployEventContractTransaction(account, chain, 1)
		prepareTx(t, block11tx1, account, privKey, 0, chain)

		txs11 := []*flow.TransactionBody{block11tx1}
		col11 := flow.Collection{Transactions: txs11}
		guarantee11 := flow.CollectionGuarantee{
			CollectionID: col11.ID(),
			Signature:    nil,
		}
		block11 = flow.Block{
			Header: &flow.Header{
				ParentID: block1.ID(),
				View:     2,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{&guarantee11},
			},
		}
		block11View = block1View.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block11,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
				guarantee11.ID(): {
					Guarantee:    &guarantee11,
					Transactions: txs11,
				},
			},
		}
		returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block11View)
		require.NoError(t, err)
		require.NotNil(t, programsCache.Get(block11.ID()))
		// 1st event should be contract deployed
		assert.EqualValues(t, "flow.AccountContractAdded", returnedComputationResult.Events[0].Type)
	})

	t.Run("executing block111 (emit event (expected v1), update contract to v3)", func(t *testing.T) {
		block111ExpectedValue := 1
		// emit event
		block111tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block111tx1, account, privKey, 1, chain)

		// update contract version 3
		block111tx2 := testutil.UpdateEventContractTransaction(account, chain, 3)
		prepareTx(t, block111tx2, account, privKey, 2, chain)

		col111 := flow.Collection{Transactions: []*flow.TransactionBody{block111tx1, block111tx2}}
		guarantee111 := flow.CollectionGuarantee{
			CollectionID: col111.ID(),
			Signature:    nil,
		}
		block111 = flow.Block{
			Header: &flow.Header{
				ParentID: block11.ID(),
				View:     3,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{&guarantee111},
			},
		}
		block111View = block11View.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block111,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
				guarantee111.ID(): {
					Guarantee:    &guarantee111,
					Transactions: col111.Transactions,
				},
			},
		}
		returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block111View)
		require.NoError(t, err)
		require.NotNil(t, programsCache.Get(block111.ID()))
		// had change so cache should not be equal to parent
		require.NotEqual(t, programsCache.Get(block11.ID()), programsCache.Get(block111.ID()))
		// 1st event
		hasValidEventValue(t, returnedComputationResult.Events[0], block111ExpectedValue)
		// second event should be contract deployed
		assert.EqualValues(t, "flow.AccountContractUpdated", returnedComputationResult.Events[1].Type)
	})

	t.Run("executing block1111 (emit event (expected v3))", func(t *testing.T) {
		block1111ExpectedValue := 3
		block1111tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block1111tx1, account, privKey, 3, chain)

		col1111 := flow.Collection{Transactions: []*flow.TransactionBody{block1111tx1}}
		guarantee1111 := flow.CollectionGuarantee{
			CollectionID: col1111.ID(),
			Signature:    nil,
		}
		block1111 = flow.Block{
			Header: &flow.Header{
				ParentID: block111.ID(),
				View:     4,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{&guarantee1111},
			},
		}
		block1111View = block111View.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block1111,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
				guarantee1111.ID(): {
					Guarantee:    &guarantee1111,
					Transactions: col1111.Transactions,
				},
			},
		}
		returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block1111View)
		require.NoError(t, err)
		// had no change so cache should be equal to parent
		require.Equal(t, programsCache.Get(block111.ID()), programsCache.Get(block1111.ID()))
		// 1st event
		hasValidEventValue(t, returnedComputationResult.Events[0], block1111ExpectedValue)

	})
	t.Run("executing block112 (emit event (expected v1))", func(t *testing.T) {
		block112ExpectedValue := 1
		block112tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block112tx1, account, privKey, 1, chain)

		// update contract version 4
		block112tx2 := testutil.UpdateEventContractTransaction(account, chain, 4)
		prepareTx(t, block112tx2, account, privKey, 2, chain)

		col112 := flow.Collection{Transactions: []*flow.TransactionBody{block112tx1, block112tx2}}
		guarantee112 := flow.CollectionGuarantee{
			CollectionID: col112.ID(),
			Signature:    nil,
		}

		block112 = flow.Block{
			Header: &flow.Header{
				ParentID: block11.ID(),
				View:     3,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{&guarantee112},
			},
		}
		block112View = block11View.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block112,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
				guarantee112.ID(): {
					Guarantee:    &guarantee112,
					Transactions: col112.Transactions,
				},
			},
		}
		returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block112View)
		require.NoError(t, err)
		// 1st event
		hasValidEventValue(t, returnedComputationResult.Events[0], block112ExpectedValue)
		// second event should be contract deployed
		assert.EqualValues(t, "flow.AccountContractUpdated", returnedComputationResult.Events[1].Type)

	})
	t.Run("executing block1121 (emit event (expected v4))", func(t *testing.T) {
		block1121ExpectedValue := 4
		block1121tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block1121tx1, account, privKey, 3, chain)

		col1121 := flow.Collection{Transactions: []*flow.TransactionBody{block1121tx1}}
		guarantee1121 := flow.CollectionGuarantee{
			CollectionID: col1121.ID(),
			Signature:    nil,
		}

		block1121 = flow.Block{
			Header: &flow.Header{
				ParentID: block112.ID(),
				View:     4,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{&guarantee1121},
			},
		}
		block1121View = block112View.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block1121,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
				guarantee1121.ID(): {
					Guarantee:    &guarantee1121,
					Transactions: col1121.Transactions,
				},
			},
		}
		returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block1121View)
		require.NoError(t, err)
		// 1st event
		hasValidEventValue(t, returnedComputationResult.Events[0], block1121ExpectedValue)

	})
	t.Run("executing block12 (deploys contract V2)", func(t *testing.T) {

		block12tx1 := testutil.DeployEventContractTransaction(account, chain, 2)
		prepareTx(t, block12tx1, account, privKey, 0, chain)

		col12 := flow.Collection{Transactions: []*flow.TransactionBody{block12tx1}}
		guarantee12 := flow.CollectionGuarantee{
			CollectionID: col12.ID(),
			Signature:    nil,
		}
		block12 = flow.Block{
			Header: &flow.Header{
				ParentID: block1.ID(),
				View:     2,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{&guarantee12},
			},
		}

		block12View = block1View.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block12,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
				guarantee12.ID(): {
					Guarantee:    &guarantee12,
					Transactions: col12.Transactions,
				},
			},
		}
		returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block12View)
		require.NoError(t, err)
		assert.EqualValues(t, "flow.AccountContractAdded", returnedComputationResult.Events[0].Type)
	})
	t.Run("executing block121 (emit event (expected V2)", func(t *testing.T) {
		block121ExpectedValue := 2
		block121tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block121tx1, account, privKey, 1, chain)

		col121 := flow.Collection{Transactions: []*flow.TransactionBody{block121tx1}}
		guarantee121 := flow.CollectionGuarantee{
			CollectionID: col121.ID(),
			Signature:    nil,
		}
		block121 = flow.Block{
			Header: &flow.Header{
				ParentID: block12.ID(),
				View:     3,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{&guarantee121},
			},
		}
		block121View = block12View.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block: &block121,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
				guarantee121.ID(): {
					Guarantee:    &guarantee121,
					Transactions: col121.Transactions,
				},
			},
		}
		returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, block121View)
		require.NoError(t, err)
		// 1st event
		hasValidEventValue(t, returnedComputationResult.Events[0], block121ExpectedValue)
	})
}

func prepareTx(t *testing.T,
	tx *flow.TransactionBody,
	account flow.Address,
	privKey flow.AccountPrivateKey,
	seqNumber uint64,
	chain flow.Chain) {
	tx.SetProposalKey(chain.ServiceAddress(), 0, seqNumber).
		SetPayer(chain.ServiceAddress())
	err := testutil.SignPayload(tx, account, privKey)
	require.NoError(t, err)
	err = testutil.SignEnvelope(tx, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)
}

func hasValidEventValue(t *testing.T, event flow.Event, value int) {
	data, err := jsoncdc.Decode(event.Payload)
	require.NoError(t, err)
	assert.Equal(t, int16(value), data.(cadence.Event).Fields[0].ToGoValue())
}

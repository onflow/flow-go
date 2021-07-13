package computation

import (
	"context"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPrograms_TestContractUpdates(t *testing.T) {
	rt := fvm.NewInterpreterRuntime()
	chain := flow.Mainnet.Chain()
	vm := fvm.NewVirtualMachine(rt)
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
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

	// tx5 updates the contract to version 3 but fails (no env signature of service account)
	tx5 := testutil.UnauthorizedDeployEventContractTransaction(account, chain, 3)
	tx5.SetProposalKey(account, 0, 4).SetPayer(account)
	err = testutil.SignEnvelope(tx5, account, privKey)
	require.NoError(t, err)

	// tx6 calls the method of the contract (version 2 expected)
	tx6 := testutil.CreateEmitEventTransaction(account, account)
	prepareTx(t, tx6, account, privKey, 5, chain)

	transactions := []*flow.TransactionBody{tx1, tx2, tx3, tx4, tx5, tx6}

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
		StartState: unittest.StateCommitmentPointerFixture(),
	}

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	blockComputer, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
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

	require.Len(t, returnedComputationResult.Events, 2) // 1 collection + 1 system chunk

	// first event should be contract deployed
	assert.EqualValues(t, "flow.AccountContractAdded", returnedComputationResult.Events[0][0].Type)

	// second event should have a value of 1 (since is calling version 1 of contract)
	hasValidEventValue(t, returnedComputationResult.Events[0][1], 1)

	// third event should be contract updated
	assert.EqualValues(t, "flow.AccountContractUpdated", returnedComputationResult.Events[0][2].Type)

	// 4th event should have a value of 2 (since is calling version 2 of contract)
	hasValidEventValue(t, returnedComputationResult.Events[0][3], 2)

	// 5th event should have a value of 2 (since is calling version 2 of contract)
	hasValidEventValue(t, returnedComputationResult.Events[0][4], 2)
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
//             -> Block1211 (emit event - version should be 2)
func TestPrograms_TestBlockForks(t *testing.T) {
	// setup
	rt := fvm.NewInterpreterRuntime()
	chain := flow.Mainnet.Chain()
	vm := fvm.NewVirtualMachine(rt)
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
	require.NoError(t, err)

	account := accounts[0]
	privKey := privateKeys[0]

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	blockComputer, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
	require.NoError(t, err)

	programsCache, err := NewProgramsCache(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer: blockComputer,
		me:            me,
		programsCache: programsCache,
	}

	view := delta.NewView(ledger.Get)

	var (
		res *execution.ComputationResult

		block1, block11, block111, block112, block1121,
		block1111, block12, block121, block1211 *flow.Block

		block1View, block11View, block111View, block112View, block1121View,
		block1111View, block12View, block121View, block1211View state.View
	)

	t.Run("executing block1 (no collection)", func(t *testing.T) {
		block1 = &flow.Block{
			Header: &flow.Header{
				View: 1,
			},
			Payload: &flow.Payload{
				Guarantees: []*flow.CollectionGuarantee{},
			},
		}
		block1View = view.NewChild()
		executableBlock := &entity.ExecutableBlock{
			Block:      block1,
			StartState: unittest.StateCommitmentPointerFixture(),
		}
		_, err := engine.ComputeBlock(context.Background(), executableBlock, block1View)
		require.NoError(t, err)
	})

	t.Run("executing block11 (deploys contract version 1)", func(t *testing.T) {
		block11tx1 := testutil.DeployEventContractTransaction(account, chain, 1)
		prepareTx(t, block11tx1, account, privKey, 0, chain)

		txs11 := []*flow.TransactionBody{block11tx1}
		col11 := flow.Collection{Transactions: txs11}
		block11View = block1View.NewChild()
		block11, res = createTestBlockAndRun(t, engine, block1, col11, block11View)
		// cache should include value for this block
		require.NotNil(t, programsCache.Get(block11.ID()))
		// cache should have changes
		require.True(t, programsCache.Get(block11.ID()).HasChanges())
		// 1st event should be contract deployed
		assert.EqualValues(t, "flow.AccountContractAdded", res.Events[0][0].Type)
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
		block111View = block11View.NewChild()
		block111, res = createTestBlockAndRun(t, engine, block11, col111, block111View)
		// cache should include a program for this block
		require.NotNil(t, programsCache.Get(block111.ID()))
		// cache should have changes
		require.True(t, programsCache.Get(block111.ID()).HasChanges())

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block111ExpectedValue)
		// second event should be contract deployed
		assert.EqualValues(t, "flow.AccountContractUpdated", res.Events[0][1].Type)
	})

	t.Run("executing block1111 (emit event (expected v3))", func(t *testing.T) {
		block1111ExpectedValue := 3
		block1111tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block1111tx1, account, privKey, 3, chain)

		col1111 := flow.Collection{Transactions: []*flow.TransactionBody{block1111tx1}}
		block1111View = block111View.NewChild()
		block1111, res = createTestBlockAndRun(t, engine, block111, col1111, block1111View)
		// cache should include a program for this block
		require.NotNil(t, programsCache.Get(block1111.ID()))

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block1111ExpectedValue)
	})

	t.Run("executing block112 (emit event (expected v1))", func(t *testing.T) {
		block112ExpectedValue := 1
		block112tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block112tx1, account, privKey, 1, chain)

		// update contract version 4
		block112tx2 := testutil.UpdateEventContractTransaction(account, chain, 4)
		prepareTx(t, block112tx2, account, privKey, 2, chain)

		col112 := flow.Collection{Transactions: []*flow.TransactionBody{block112tx1, block112tx2}}
		block112View = block11View.NewChild()
		block112, res = createTestBlockAndRun(t, engine, block11, col112, block112View)
		// cache should include a program for this block
		require.NotNil(t, programsCache.Get(block112.ID()))

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block112ExpectedValue)
		// second event should be contract deployed
		assert.EqualValues(t, "flow.AccountContractUpdated", res.Events[0][1].Type)

	})
	t.Run("executing block1121 (emit event (expected v4))", func(t *testing.T) {
		block1121ExpectedValue := 4
		block1121tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block1121tx1, account, privKey, 3, chain)

		col1121 := flow.Collection{Transactions: []*flow.TransactionBody{block1121tx1}}
		block1121View = block112View.NewChild()
		block1121, res = createTestBlockAndRun(t, engine, block112, col1121, block1121View)
		// cache should include a program for this block
		require.NotNil(t, programsCache.Get(block1121.ID()))

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block1121ExpectedValue)

	})
	t.Run("executing block12 (deploys contract V2)", func(t *testing.T) {

		block12tx1 := testutil.DeployEventContractTransaction(account, chain, 2)
		prepareTx(t, block12tx1, account, privKey, 0, chain)

		col12 := flow.Collection{Transactions: []*flow.TransactionBody{block12tx1}}
		block12View = block1View.NewChild()
		block12, res = createTestBlockAndRun(t, engine, block1, col12, block12View)
		// cache should include a program for this block
		require.NotNil(t, programsCache.Get(block12.ID()))

		require.Len(t, res.Events, 2)

		assert.EqualValues(t, "flow.AccountContractAdded", res.Events[0][0].Type)
	})
	t.Run("executing block121 (emit event (expected V2)", func(t *testing.T) {
		block121ExpectedValue := 2
		block121tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block121tx1, account, privKey, 1, chain)

		col121 := flow.Collection{Transactions: []*flow.TransactionBody{block121tx1}}
		block121View = block12View.NewChild()
		block121, res = createTestBlockAndRun(t, engine, block12, col121, block121View)
		// cache should include a program for this block
		require.NotNil(t, programsCache.Get(block121.ID()))

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block121ExpectedValue)
	})
	t.Run("executing Block1211 (emit event (expected V2)", func(t *testing.T) {
		block1211ExpectedValue := 2
		block1211tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block1211tx1, account, privKey, 2, chain)

		col1211 := flow.Collection{Transactions: []*flow.TransactionBody{block1211tx1}}
		block1211View = block121View.NewChild()
		block1211, res = createTestBlockAndRun(t, engine, block121, col1211, block1211View)
		// cache should include a program for this block
		require.NotNil(t, programsCache.Get(block1211.ID()))
		// had no change so cache should be equal to parent
		require.Equal(t, programsCache.Get(block121.ID()), programsCache.Get(block1211.ID()))

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block1211ExpectedValue)
	})

}

func createTestBlockAndRun(t *testing.T, engine *Manager, parentBlock *flow.Block, col flow.Collection, view state.View) (*flow.Block, *execution.ComputationResult) {
	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signature:    nil,
	}

	block := &flow.Block{
		Header: &flow.Header{
			ParentID: parentBlock.ID(),
			View:     parentBlock.Header.Height + 1,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	executableBlock := &entity.ExecutableBlock{
		Block: block,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: col.Transactions,
			},
		},
		StartState: unittest.StateCommitmentPointerFixture(),
	}
	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, view)
	require.NoError(t, err)
	return block, returnedComputationResult
}

func prepareTx(t *testing.T,
	tx *flow.TransactionBody,
	account flow.Address,
	privKey flow.AccountPrivateKey,
	seqNumber uint64,
	chain flow.Chain) {
	tx.SetProposalKey(account, 0, seqNumber).
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

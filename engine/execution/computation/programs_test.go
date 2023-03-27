package computation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPrograms_TestContractUpdates(t *testing.T) {
	chain := flow.Mainnet.Chain()
	vm := fvm.NewVirtualMachine()
	execCtx := fvm.NewContext(fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(
		vm,
		ledger,
		privateKeys,
		chain)
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
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	blockComputer, err := computer.NewBlockComputer(
		vm,
		execCtx,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		zerolog.Nop(),
		committer.NewNoopViewCommitter(),
		me,
		prov,
		nil)
	require.NoError(t, err)

	derivedChainData, err := derived.NewDerivedChainData(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer:    blockComputer,
		derivedChainData: derivedChainData,
	}

	returnedComputationResult, err := engine.ComputeBlock(
		context.Background(),
		unittest.IdentifierFixture(),
		executableBlock,
		ledger)
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

type blockProvider struct {
	blocks map[uint64]*flow.Block
}

func (b blockProvider) ByHeightFrom(height uint64, _ *flow.Header) (*flow.Header, error) {
	block, has := b.blocks[height]
	if has {
		return block.Header, nil
	}
	return nil, fmt.Errorf("block for height (%d) is not available", height)
}

// TestPrograms_TestBlockForks tests the functionality of
// derivedChainData under contract deployment and contract updates on
// different block forks
//
// block structure and operations
// Block1 (empty block)
//
//	    -> Block11 (deploy contract v1)
//	        -> Block111  (emit event - version should be 1) and (update contract to v3)
//	            -> Block1111   (emit event - version should be 3)
//		       -> Block112 (emit event - version should be 1) and (update contract to v4)
//	            -> Block1121  (emit event - version should be 4)
//	    -> Block12 (deploy contract v2)
//	        -> Block121 (emit event - version should be 2)
//	            -> Block1211 (emit event - version should be 2)
func TestPrograms_TestBlockForks(t *testing.T) {
	block := unittest.BlockFixture()
	chain := flow.Emulator.Chain()
	vm := fvm.NewVirtualMachine()
	execCtx := fvm.NewContext(
		fvm.WithBlockHeader(block.Header),
		fvm.WithBlocks(blockProvider{map[uint64]*flow.Block{0: &block}}),
		fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(
		vm,
		ledger,
		privateKeys,
		chain)
	require.NoError(t, err)

	account := accounts[0]
	privKey := privateKeys[0]

	me := new(module.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	blockComputer, err := computer.NewBlockComputer(
		vm,
		execCtx,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		zerolog.Nop(),
		committer.NewNoopViewCommitter(),
		me,
		prov,
		nil)
	require.NoError(t, err)

	derivedChainData, err := derived.NewDerivedChainData(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer:    blockComputer,
		derivedChainData: derivedChainData,
	}

	view := delta.NewDeltaView(ledger)

	var (
		res *execution.ComputationResult

		block1, block11, block111, block112, block1121,
		block1111, block12, block121, block1211 *flow.Block

		block1View, block11View, block111View, block112View,
		block12View, block121View, block1211View state.View
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
		_, err := engine.ComputeBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			executableBlock,
			block1View)
		require.NoError(t, err)
	})

	t.Run("executing block11 (deploys contract version 1)", func(t *testing.T) {
		block11tx1 := testutil.DeployEventContractTransaction(account, chain, 1)
		prepareTx(t, block11tx1, account, privKey, 0, chain)

		txs11 := []*flow.TransactionBody{block11tx1}
		col11 := flow.Collection{Transactions: txs11}
		block11, res, block11View = createTestBlockAndRun(
			t,
			engine,
			block1,
			col11,
			block1View)
		// cache should include value for this block
		require.NotNil(t, derivedChainData.Get(block11.ID()))
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
		block111, res, block111View = createTestBlockAndRun(
			t,
			engine,
			block11,
			col111,
			block11View)
		// cache should include a program for this block
		require.NotNil(t, derivedChainData.Get(block111.ID()))

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
		block1111, res, _ = createTestBlockAndRun(
			t,
			engine,
			block111,
			col1111,
			block111View)
		// cache should include a program for this block
		require.NotNil(t, derivedChainData.Get(block1111.ID()))

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
		block112, res, block112View = createTestBlockAndRun(
			t,
			engine,
			block11,
			col112,
			block11View)
		// cache should include a program for this block
		require.NotNil(t, derivedChainData.Get(block112.ID()))

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
		block1121, res, _ = createTestBlockAndRun(
			t,
			engine,
			block112,
			col1121,
			block112View)
		// cache should include a program for this block
		require.NotNil(t, derivedChainData.Get(block1121.ID()))

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block1121ExpectedValue)

	})
	t.Run("executing block12 (deploys contract V2)", func(t *testing.T) {

		block12tx1 := testutil.DeployEventContractTransaction(account, chain, 2)
		prepareTx(t, block12tx1, account, privKey, 0, chain)

		col12 := flow.Collection{Transactions: []*flow.TransactionBody{block12tx1}}
		block12View = block1View.NewChild()
		block12, res, block12View = createTestBlockAndRun(
			t,
			engine,
			block1,
			col12,
			block1View)
		// cache should include a program for this block
		require.NotNil(t, derivedChainData.Get(block12.ID()))

		require.Len(t, res.Events, 2)

		assert.EqualValues(t, "flow.AccountContractAdded", res.Events[0][0].Type)
	})
	t.Run("executing block121 (emit event (expected V2)", func(t *testing.T) {
		block121ExpectedValue := 2
		block121tx1 := testutil.CreateEmitEventTransaction(account, account)
		prepareTx(t, block121tx1, account, privKey, 1, chain)

		col121 := flow.Collection{Transactions: []*flow.TransactionBody{block121tx1}}
		block121, res, block121View = createTestBlockAndRun(
			t,
			engine,
			block12,
			col121,
			block12View)
		// cache should include a program for this block
		require.NotNil(t, derivedChainData.Get(block121.ID()))

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
		block1211, res, block1211View = createTestBlockAndRun(
			t,
			engine,
			block121,
			col1211,
			block1211View)
		// cache should include a program for this block
		require.NotNil(t, derivedChainData.Get(block1211.ID()))
		// had no change so cache should be equal to parent
		require.Equal(t, derivedChainData.Get(block121.ID()), derivedChainData.Get(block1211.ID()))

		require.Len(t, res.Events, 2)

		// 1st event
		hasValidEventValue(t, res.Events[0][0], block1211ExpectedValue)
	})

}

func createTestBlockAndRun(
	t *testing.T,
	engine *Manager,
	parentBlock *flow.Block,
	col flow.Collection,
	snapshot state.StorageSnapshot,
) (
	*flow.Block,
	*execution.ComputationResult,
	state.View,
) {
	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signature:    nil,
	}

	block := &flow.Block{
		Header: &flow.Header{
			ParentID:  parentBlock.ID(),
			View:      parentBlock.Header.Height + 1,
			Timestamp: time.Now(),
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
	returnedComputationResult, err := engine.ComputeBlock(
		context.Background(),
		unittest.IdentifierFixture(),
		executableBlock,
		snapshot)
	require.NoError(t, err)

	for _, txResult := range returnedComputationResult.TransactionResults {
		require.Empty(t, txResult.ErrorMessage)
	}

	view := delta.NewDeltaView(snapshot)
	for _, snapshot := range returnedComputationResult.StateSnapshots {
		for _, entry := range snapshot.UpdatedRegisters() {
			err := view.Set(entry.Key, entry.Value)
			require.NoError(t, err)
		}
	}

	return block, returnedComputationResult, view
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
	data, err := jsoncdc.Decode(nil, event.Payload)
	require.NoError(t, err)
	assert.Equal(t, int16(value), data.(cadence.Event).Fields[0].ToGoValue())
}

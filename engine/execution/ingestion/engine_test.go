package ingestion

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	engineCommon "github.com/dapperlabs/flow-go/engine"
	computation "github.com/dapperlabs/flow-go/engine/execution/computation/mock"
	provider "github.com/dapperlabs/flow-go/engine/execution/provider/mock"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	state "github.com/dapperlabs/flow-go/engine/execution/state/mock"
	executionUnittest "github.com/dapperlabs/flow-go/engine/execution/state/unittest"
	mocklocal "github.com/dapperlabs/flow-go/engine/testutil/mocklocal"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/metrics"
	module2 "github.com/dapperlabs/flow-go/module/mock"
	module "github.com/dapperlabs/flow-go/module/mocks"
	"github.com/dapperlabs/flow-go/module/trace"
	network "github.com/dapperlabs/flow-go/network/mocks"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	realStorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var (
	collection1Identity = unittest.IdentityFixture()
	collection2Identity = unittest.IdentityFixture()
	collection3Identity = unittest.IdentityFixture()
	myIdentity          = unittest.IdentityFixture()
)

func init() {
	collection1Identity.Role = flow.RoleCollection
	collection2Identity.Role = flow.RoleCollection
	collection3Identity.Role = flow.RoleCollection
	myIdentity.Role = flow.RoleExecution
}

type testingContext struct {
	t                  *testing.T
	engine             *Engine
	blocks             *storage.MockBlocks
	collections        *storage.MockCollections
	state              *protocol.State
	conduit            *network.MockConduit
	collectionConduit  *network.MockConduit
	computationManager *computation.ComputationManager
	providerEngine     *provider.ProviderEngine
	executionState     *state.ExecutionState
	snapshot           *protocol.Snapshot
}

func runWithEngine(t *testing.T, f func(testingContext)) {

	ctrl := gomock.NewController(t)

	net := module.NewMockNetwork(ctrl)

	// initialize the mocks and engine
	conduit := network.NewMockConduit(ctrl)
	collectionConduit := network.NewMockConduit(ctrl)
	syncConduit := network.NewMockConduit(ctrl)

	// generates signing identity including staking key for signing
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	myIdentity.StakingPubKey = sk.PublicKey()
	me := mocklocal.NewMockLocal(sk, myIdentity.ID(), t)

	blocks := storage.NewMockBlocks(ctrl)
	payloads := storage.NewMockPayloads(ctrl)
	collections := storage.NewMockCollections(ctrl)
	events := storage.NewMockEvents(ctrl)
	txResults := storage.NewMockTransactionResults(ctrl)

	computationManager := new(computation.ComputationManager)
	providerEngine := new(provider.ProviderEngine)
	protocolState := new(protocol.State)
	executionState := new(state.ExecutionState)
	snapshot := new(protocol.Snapshot)

	var engine *Engine

	defer func() {
		<-engine.Done()
		ctrl.Finish()
		computationManager.AssertExpectations(t)
		protocolState.AssertExpectations(t)
		executionState.AssertExpectations(t)
		providerEngine.AssertExpectations(t)
	}()

	identityList := flow.IdentityList{myIdentity, collection1Identity, collection2Identity, collection3Identity}

	executionState.On("DiskSize").Return(int64(1024*1024), nil).Maybe()

	snapshot.On("Identities", mock.Anything).Return(func(selector flow.IdentityFilter) flow.IdentityList {
		return identityList.Filter(selector)
	}, nil)

	snapshot.On("Identity", mock.Anything).Return(func(nodeID flow.Identifier) *flow.Identity {
		identity, ok := identityList.ByNodeID(nodeID)
		require.Truef(t, ok, "Could not find nodeID %v in identityList", nodeID)
		return identity
	}, nil)

	payloads.EXPECT().Store(gomock.Any(), gomock.Any()).AnyTimes()

	log := zerolog.Logger{}
	metrics := metrics.NewNoopCollector()

	tracer, err := trace.NewTracer(log, "test")
	require.NoError(t, err)

	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.BlockProvider)), gomock.AssignableToTypeOf(engine)).Return(conduit, nil)
	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.CollectionProvider)), gomock.AssignableToTypeOf(engine)).Return(collectionConduit, nil)
	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.ExecutionSync)), gomock.AssignableToTypeOf(engine)).Return(syncConduit, nil)
	blockSync := new(module2.BlockRequester)

	engine, err = New(
		log,
		net,
		me,
		protocolState,
		blocks,
		payloads,
		collections,
		events,
		txResults,
		computationManager,
		providerEngine,
		blockSync,
		executionState,
		21,
		metrics,
		tracer,
		false,
		1*time.Hour, //practically disable retrying
		10,
	)
	require.NoError(t, err)

	f(testingContext{
		t:                  t,
		engine:             engine,
		blocks:             blocks,
		collections:        collections,
		state:              protocolState,
		conduit:            conduit,
		collectionConduit:  collectionConduit,
		computationManager: computationManager,
		providerEngine:     providerEngine,
		executionState:     executionState,
		snapshot:           snapshot,
	})

	<-engine.Done()
}

func (ctx *testingContext) assertSuccessfulBlockComputation(executableBlock *entity.ExecutableBlock, previousExecutionResultID flow.Identifier) {
	computationResult := executionUnittest.ComputationResultForBlockFixture(executableBlock)
	newStateCommitment := unittest.StateCommitmentFixture()
	if len(computationResult.StateSnapshots) == 0 { // if block was empty, no new state commitment is produced
		newStateCommitment = executableBlock.StartState
	}
	ctx.executionState.On("NewView", executableBlock.StartState).Return(new(delta.View))

	ctx.computationManager.
		On("ComputeBlock", mock.Anything, executableBlock, mock.Anything).
		Return(computationResult, nil).Once()

	ctx.executionState.
		On("PersistStateInteractions", mock.Anything, executableBlock.Block.ID(), mock.Anything).
		Return(nil)

	for _, view := range computationResult.StateSnapshots {
		ctx.executionState.
			On("CommitDelta", mock.Anything, view.Delta, executableBlock.StartState).
			Return(newStateCommitment, nil)

		ctx.executionState.
			On("GetRegistersWithProofs", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil, nil)

		ctx.executionState.
			On("PersistChunkDataPack", mock.Anything, mock.MatchedBy(func(f *flow.ChunkDataPack) bool {
				return bytes.Equal(f.StartState, executableBlock.StartState)
			})).
			Return(nil)
	}

	ctx.executionState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	ctx.executionState.
		On("UpdateHighestExecutedBlockIfHigher", mock.Anything, executableBlock.Block.Header).
		Return(nil)

	ctx.executionState.
		On(
			"PersistExecutionResult",
			mock.Anything,
			executableBlock.Block.ID(),
			mock.MatchedBy(func(er flow.ExecutionResult) bool {
				return er.BlockID == executableBlock.Block.ID() && er.PreviousResultID == previousExecutionResultID
			}),
		).
		Return(nil)

	ctx.executionState.
		On("PersistStateCommitment", mock.Anything, executableBlock.Block.ID(), newStateCommitment).
		Return(nil)

	ctx.providerEngine.
		On(
			"BroadcastExecutionReceipt",
			mock.Anything,
			mock.MatchedBy(func(er *flow.ExecutionReceipt) bool {
				return er.ExecutionResult.BlockID == executableBlock.Block.ID() &&
					er.ExecutionResult.PreviousResultID == previousExecutionResultID
			}),
		).
		Run(func(args mock.Arguments) {
			receipt := args[1].(*flow.ExecutionReceipt)

			executor, err := ctx.snapshot.Identity(receipt.ExecutorID)
			assert.NoError(ctx.t, err, "could not find executor in protocol state")

			// verify the signature
			id := receipt.ID()
			validSig, err := executor.StakingPubKey.Verify(receipt.ExecutorSignature, id[:], ctx.engine.receiptHasher)
			assert.NoError(ctx.t, err)

			assert.True(ctx.t, validSig, "execution receipt signature invalid")

			spocks := receipt.Spocks

			assert.Len(ctx.t, spocks, len(computationResult.StateSnapshots))

			for i, stateSnapshot := range computationResult.StateSnapshots {

				valid, err := crypto.SPOCKVerifyAgainstData(
					ctx.engine.me.StakingKey().PublicKey(),
					spocks[i],
					stateSnapshot.SpockSecret,
					ctx.engine.spockHasher,
				)

				assert.NoError(ctx.t, err)
				assert.True(ctx.t, valid)
			}

		}).
		Return(nil)
}

func TestExecutionGenerationResultsAreChained(t *testing.T) {

	execState := new(state.ExecutionState)

	e := Engine{
		execState: execState,
	}

	executableBlock := unittest.ExecutableBlockFixture([][]flow.Identifier{{collection1Identity.NodeID}, {collection1Identity.NodeID}})
	endState := unittest.StateCommitmentFixture()
	previousExecutionResultID := unittest.IdentifierFixture()

	execState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	execState.
		On("PersistExecutionResult", mock.Anything, executableBlock.Block.ID(), mock.Anything).
		Return(nil)

	er, err := e.generateExecutionResultForBlock(context.Background(), executableBlock.Block, nil, endState)
	assert.NoError(t, err)

	assert.Equal(t, previousExecutionResultID, er.PreviousResultID)

	execState.AssertExpectations(t)
}

func TestBlockOutOfOrder(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		executableBlockA := unittest.ExecutableBlockFixture(nil)
		executableBlockB := unittest.ExecutableBlockFixtureWithParent(nil, executableBlockA.Block.Header)
		executableBlockC := unittest.ExecutableBlockFixtureWithParent(nil, executableBlockA.Block.Header)
		executableBlockD := unittest.ExecutableBlockFixtureWithParent(nil, executableBlockC.Block.Header)
		executableBlockA.StartState = unittest.StateCommitmentFixture()

		// blocks has no collections, so state is essentially the same
		executableBlockC.StartState = executableBlockA.StartState
		executableBlockB.StartState = executableBlockA.StartState
		executableBlockD.StartState = executableBlockC.StartState

		/* Artists recreation of the blocks structure:

		  b
		   \
		    a
		   /
		d-c

		*/

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockA.Block))
		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockB.Block))
		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockC.Block))
		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockD.Block))

		// initialize the proposals
		proposalA := unittest.ProposalFromBlock(executableBlockA.Block)
		proposalB := unittest.ProposalFromBlock(executableBlockB.Block)
		proposalC := unittest.ProposalFromBlock(executableBlockC.Block)
		proposalD := unittest.ProposalFromBlock(executableBlockD.Block)

		// no execution state, so puts to waiting queue
		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, executableBlockB.Block.Header.ParentID).
			Return(nil, realStorage.ErrNotFound)

		err := ctx.engine.handleBlockProposal(context.Background(), proposalB)
		require.NoError(t, err)

		// no execution state, no connection to other nodes
		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, executableBlockC.Block.Header.ParentID).
			Return(nil, realStorage.ErrNotFound)

		err = ctx.engine.handleBlockProposal(context.Background(), proposalC)
		require.NoError(t, err)

		// child of c so no need to query execution state

		// we account for every call, so if this call would have happen, test will fail
		// ctx.executionState.On("StateCommitmentByBlockID", executableBlockD.Block.Header.ParentID).Return(nil, realStorage.ErrNotFound)
		err = ctx.engine.handleBlockProposal(context.Background(), proposalD)
		require.NoError(t, err)

		// make sure there were no extra calls at this point in test
		ctx.executionState.AssertExpectations(t)
		ctx.computationManager.AssertExpectations(t)

		// once block A is computed, it should trigger B and C being sent to compute, which in turn should trigger D
		blockAExecutionResultID := unittest.IdentifierFixture()
		ctx.assertSuccessfulBlockComputation(executableBlockA, unittest.IdentifierFixture())
		ctx.assertSuccessfulBlockComputation(executableBlockB, blockAExecutionResultID)
		ctx.assertSuccessfulBlockComputation(executableBlockC, blockAExecutionResultID)
		ctx.assertSuccessfulBlockComputation(executableBlockD, unittest.IdentifierFixture())

		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, executableBlockA.Block.Header.ParentID).
			Return(executableBlockA.StartState, nil)

		err = ctx.engine.handleBlockProposal(context.Background(), proposalA)
		require.NoError(t, err)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		assert.False(t, more)
	})

}

func TestExecuteScriptAtBlockID(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		// Meaningless script
		script := []byte{1, 1, 2, 3, 5, 8, 11}
		scriptResult := []byte{1}

		// Ensure block we're about to query against is executable
		executableBlock := unittest.ExecutableBlockFixture(nil)
		executableBlock.StartState = unittest.StateCommitmentFixture()

		snapshot := new(protocol.Snapshot)
		snapshot.On("Head").Return(executableBlock.Block.Header, nil)

		// Add all data needed for execution of script
		ctx.executionState.
			On("StateCommitmentByBlockID", mock.Anything, executableBlock.Block.ID()).
			Return(executableBlock.StartState, nil)

		ctx.state.On("AtBlockID", executableBlock.Block.ID()).Return(snapshot)
		view := new(delta.View)
		ctx.executionState.On("NewView", executableBlock.StartState).Return(view)

		// Successful call to computation manager
		ctx.computationManager.
			On("ExecuteScript", script, [][]byte(nil), executableBlock.Block.Header, view).
			Return(scriptResult, nil)

		// Execute our script and expect no error
		res, err := ctx.engine.ExecuteScriptAtBlockID(context.Background(), script, nil, executableBlock.Block.ID())
		assert.NoError(t, err)
		assert.Equal(t, scriptResult, res)

		// Assert other components were called as expected
		ctx.computationManager.AssertExpectations(t)
		ctx.executionState.AssertExpectations(t)
		ctx.state.AssertExpectations(t)
	})
}

func Test_SPoCKGeneration(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {

		snapshots := []*delta.Snapshot{
			{
				SpockSecret: []byte{1, 2, 3},
			},
			{
				SpockSecret: []byte{3, 2, 1},
			},
			{
				SpockSecret: []byte{},
			},
			{
				SpockSecret: unittest.RandomBytes(100),
			},
		}

		executionReceipt, err := ctx.engine.generateExecutionReceipt(
			context.Background(),
			&flow.ExecutionResult{
				ExecutionResultBody: flow.ExecutionResultBody{},
				Signatures:          nil,
			},
			snapshots,
		)
		require.NoError(t, err)

		for i, snapshot := range snapshots {
			valid, err := crypto.SPOCKVerifyAgainstData(
				ctx.engine.me.StakingKey().PublicKey(),
				executionReceipt.Spocks[i],
				snapshot.SpockSecret,
				ctx.engine.spockHasher,
			)

			require.NoError(t, err)
			require.True(t, valid)
		}

	})
}

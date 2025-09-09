package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/cmd/build"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/events"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	communicatormock "github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	realstate "github.com/onflow/flow-go/state"
	realprotocol "github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/invalid"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

const TEST_MAX_HEIGHT = 100

var eventEncodingVersions = []entitiesproto.EventEncodingVersion{
	entitiesproto.EventEncodingVersion_JSON_CDC_V0,
	entitiesproto.EventEncodingVersion_CCF_V0,
}

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks             *storagemock.Blocks
	headers            *storagemock.Headers
	collections        *storagemock.Collections
	transactions       *storagemock.Transactions
	receipts           *storagemock.ExecutionReceipts
	results            *storagemock.ExecutionResults
	transactionResults *storagemock.LightTransactionResults
	events             *storagemock.Events
	txErrorMessages    *storagemock.TransactionResultErrorMessages

	db                  storage.DB
	dbDir               string
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
	versionControl      *version.VersionControl

	colClient              *accessmock.AccessAPIClient
	execClient             *accessmock.ExecutionAPIClient
	historicalAccessClient *accessmock.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory
	communicator      *communicatormock.Communicator

	chainID  flow.ChainID
	systemTx *flow.TransactionBody

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()
	params := new(protocol.Params)
	params.On("FinalizedRoot").Return(header, nil)
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("SporkRootBlockHeight").Return(header.Height, nil)
	params.On("SealedRoot").Return(header, nil)
	suite.state.On("Params").Return(params)

	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)
	suite.results = new(storagemock.ExecutionResults)
	suite.txErrorMessages = storagemock.NewTransactionResultErrorMessages(suite.T())
	suite.colClient = new(accessmock.AccessAPIClient)
	suite.execClient = new(accessmock.ExecutionAPIClient)
	suite.transactionResults = storagemock.NewLightTransactionResults(suite.T())
	suite.events = storagemock.NewEvents(suite.T())
	suite.chainID = flow.Testnet
	suite.historicalAccessClient = new(accessmock.AccessAPIClient)
	suite.connectionFactory = connectionmock.NewConnectionFactory(suite.T())

	suite.communicator = new(communicatormock.Communicator)

	var err error
	suite.systemTx, err = blueprints.SystemChunkTransaction(flow.Testnet.Chain())
	suite.Require().NoError(err)

	pdb, dbDir := unittest.TempPebbleDB(suite.T())
	suite.dbDir = dbDir
	suite.db = pebbleimpl.ToDB(pdb)
	progress, err := store.NewConsumerProgress(suite.db, module.ConsumeProgressLastFullBlockHeight).Initialize(0)
	require.NoError(suite.T(), err)
	suite.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(progress)
	suite.Require().NoError(err)
}

// TearDownTest cleans up the db
func (suite *Suite) TearDownTest() {
	err := os.RemoveAll(suite.dbDir)
	suite.Require().NoError(err)
}

func (suite *Suite) TestPing() {
	suite.colClient.
		On("Ping", mock.Anything, &accessproto.PingRequest{}).
		Return(&accessproto.PingResponse{}, nil)

	suite.execClient.
		On("Ping", mock.Anything, &execproto.PingRequest{}).
		Return(&execproto.PingResponse{}, nil)

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	err = backend.Ping(context.Background())
	suite.Require().NoError(err)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(block, nil).Once()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	// query the handler for the latest finalized block
	header, stat, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.Require().NoError(err)
	suite.Require().NotNil(header)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)
	suite.Require().Equal(stat, flow.BlockStatusFinalized)

	suite.assertAllExpectations()
}

// TestGetLatestProtocolStateSnapshot_NoTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the State requested at latest finalized  block does not contain any Blocks that
// spans an epoch or epoch phase transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_NoTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)
		// build epoch 1
		// Blocks in current State
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit|
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID mock returns for State
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height)).Once()
		}

		// Take snapshot at height of block D (epoch1.heights[2]) for valid segment and valid snapshot
		// where its sealing segment is A <- B <- C
		snap := state.AtHeight(epoch1.Range()[2])
		suite.state.On("Final").Return(snap).Once()

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().NoError(err)

		// we expect the endpoint to return the snapshot at the same height we requested
		// because it has a valid sealing segment with no Blocks spanning an epoch or phase transition
		expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_TransitionSpans tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the State requested for latest finalized block  contains a block that
// spans an epoch transition and Blocks that span epoch phase transitions.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_TransitionSpans() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)

		// building 2 epochs allows us to take a snapshot at a point in time where
		// an epoch transition happens
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)
		epoch2, ok := epochBuilder.EpochHeights(2)
		require.True(suite.T(), ok)

		// setup AtHeight mock returns for State
		for _, height := range append(epoch1.Range(), epoch2.Range()...) {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at height of the first block of epoch2, the sealing segment of this snapshot
		// will contain a block spanning an epoch transition as well as an epoch phase transition.

		snap := state.AtHeight(epoch2.Range()[0])
		suite.state.On("Final").Return(snap).Once()

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().NoError(err)
		fmt.Println()

		// we expect the endpoint to return the latest snapshot, even though it spans an epoch transition
		expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_PhaseTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the State requested at latest finalized  block contains a Blocks that
// spans an epoch phase transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_PhaseTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)
		// build epoch 1
		// Blocks in current State
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit|
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID mock returns for State
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at height of block D (epoch1.heights[3]) the sealing segment for this snapshot
		// is C(S_A) <- D(S_B) |setup|) which spans the epoch setup phase. This will force
		// our RPC endpoint to return a snapshot at block D which is the snapshot at the boundary where the phase
		// transition happens.
		snap := state.AtHeight(epoch1.Range()[3])
		suite.state.On("Final").Return(snap).Once()

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().NoError(err)

		// we expect the endpoint to return latest snapshot, even though it spans an epoch phase transition
		expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_EpochTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the State requested at latest finalized block contains a Blocks that
// spans an epoch transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_EpochTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)
		// build epoch 1
		// Blocks in current State
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit|
		epochBuilder.BuildEpoch()

		// add more Blocks to our State in the commit phase, this will allow
		// us to take a snapshot at the height where the epoch1 -> epoch2 transition
		// and no block spans an epoch phase transition. The third block added will
		// have a seal for the first block in the commit phase allowing us to avoid
		// spanning an epoch phase transition.
		epochBuilder.AddBlocksWithSeals(3, 1)
		epochBuilder.CompleteEpoch()

		// Now we build our second epoch
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)
		epoch2, ok := epochBuilder.EpochHeights(2)
		require.True(suite.T(), ok)

		// setup AtHeight mock returns for State
		for _, height := range append(epoch1.Range(), epoch2.Range()...) {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at the first block of epoch2 . The sealing segment
		// for this snapshot contains a block (highest) that spans the epoch1 -> epoch2
		// transition.
		snap := state.AtHeight(epoch2.Range()[0])
		suite.state.On("Final").Return(snap).Once()

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().NoError(err)

		// we expect endpoint to return the latest snapshot, even though it spans an epoch transition
		expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetProtocolStateSnapshotByBlockID tests our GetProtocolStateSnapshotByBlockID RPC endpoint.
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)
		// build epoch 1
		// Blocks in current State
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit|
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID, AtHeight and BlockIDByHeight mock returns for State
		for _, height := range epoch1.Range() {
			snap := state.AtHeight(height)
			blockHead, err := snap.Head()
			suite.Require().NoError(err)

			suite.state.On("AtHeight", height).Return(snap)
			suite.state.On("AtBlockID", blockHead.ID()).Return(snap)
			suite.headers.On("BlockIDByHeight", height).Return(blockHead.ID(), nil)
		}

		// Take snapshot at height of block D (epoch1.heights[2]) for valid segment and valid snapshot
		snap := state.AtHeight(epoch1.Range()[2])
		blockHead, err := snap.Head()
		suite.Require().NoError(err)

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetProtocolStateSnapshotByBlockID(context.Background(), blockHead.ID())
		suite.Require().NoError(err)

		// we expect the endpoint to return the snapshot at the same height we requested
		expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetProtocolStateSnapshotByBlockID_NonQueryBlock tests our GetProtocolStateSnapshotByBlockID RPC endpoint
// where no block with the given ID was found.
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_UnknownQueryBlock() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState) {
		rootBlock, err := rootSnapshot.Head()
		suite.Require().NoError(err)

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// create a new block with root block as parent
		newBlock := unittest.BlockWithParentFixture(rootBlock)
		ctx := context.Background()

		suite.state.On("AtBlockID", newBlock.ID()).Return(unittest.StateSnapshotForUnknownBlock())

		// query the handler for the snapshot for non existing block
		snapshotBytes, err := backend.GetProtocolStateSnapshotByBlockID(ctx, newBlock.ID())
		suite.Require().Nil(snapshotBytes)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Equal(status.Errorf(codes.NotFound, "failed to get a valid snapshot: block not found").Error(),
			err.Error())
		suite.assertAllExpectations()
	})
}

// TestGetProtocolStateSnapshotByBlockID_AtBlockIDInternalError tests our GetProtocolStateSnapshotByBlockID RPC endpoint
// where unexpected error from snapshot.AtBlockID appear because of closed database.
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_AtBlockIDInternalError() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState) {
		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		ctx := context.Background()
		newBlock := unittest.BlockFixture()

		expectedError := errors.New("runtime-error")
		suite.state.On("AtBlockID", newBlock.ID()).Return(invalid.NewSnapshot(expectedError))

		// query the handler for the snapshot
		snapshotBytes, err := backend.GetProtocolStateSnapshotByBlockID(ctx, newBlock.ID())
		suite.Require().Nil(snapshotBytes)
		suite.Require().Error(err)
		suite.Require().ErrorAs(err, &expectedError)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.assertAllExpectations()
	})
}

// TestGetProtocolStateSnapshotByBlockID_BlockNotFinalizedAtHeight tests our GetProtocolStateSnapshotByBlockID RPC endpoint
// where The block exists, but no block has been finalized at its height.
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_BlockNotFinalizedAtHeight() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolState, err := rootSnapshot.ProtocolState()
	require.NoError(suite.T(), err)
	rootProtocolStateID := rootProtocolState.ID()
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState) {
		rootBlock, err := rootSnapshot.Head()
		suite.Require().NoError(err)

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// create a new block with root block as parent
		newBlock := unittest.BlockWithParentAndPayload(
			rootBlock,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		ctx := context.Background()
		// add new block to the chain state
		err = state.Extend(ctx, unittest.ProposalFromBlock(newBlock))
		suite.Require().NoError(err)

		// since block was added to the block tree it must be queryable by block ID
		suite.state.On("AtBlockID", newBlock.ID()).Return(state.AtBlockID(newBlock.ID()))
		suite.headers.On("BlockIDByHeight", newBlock.Height).Return(flow.ZeroID, storage.ErrNotFound)

		// query the handler for the snapshot for non finalized block
		snapshotBytes, err := backend.GetProtocolStateSnapshotByBlockID(ctx, newBlock.ID())
		suite.Require().Nil(snapshotBytes)
		suite.Require().Error(err)
		suite.Require().Equal(codes.FailedPrecondition, status.Code(err))
		suite.assertAllExpectations()
	})
}

// TestGetProtocolStateSnapshotByBlockID_DifferentBlockFinalizedAtHeight tests our GetProtocolStateSnapshotByBlockID RPC
// endpoint where a different block than what was queried has been finalized at this height.
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_DifferentBlockFinalizedAtHeight() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolState, err := rootSnapshot.ProtocolState()
	require.NoError(suite.T(), err)
	rootProtocolStateID := rootProtocolState.ID()
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState) {
		rootBlock, err := rootSnapshot.Head()
		suite.Require().NoError(err)

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// create a new block with root block as parent
		finalizedBlock := unittest.BlockWithParentAndPayload(
			rootBlock,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		orphanBlock := unittest.BlockWithParentAndPayload(
			rootBlock,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		ctx := context.Background()

		// add new block to the chain state
		err = state.Extend(ctx, unittest.ProposalFromBlock(finalizedBlock))
		suite.Require().NoError(err)

		// add orphan block to the chain state as well
		err = state.Extend(ctx, unittest.ProposalFromBlock(orphanBlock))
		suite.Require().NoError(err)

		suite.Equal(finalizedBlock.Height, orphanBlock.Height,
			"expect both blocks to have same height to have a fork")

		// since block was added to the block tree it must be queryable by block ID
		suite.state.On("AtBlockID", orphanBlock.ID()).Return(state.AtBlockID(orphanBlock.ID()))

		// since there are two candidate blocks with the same height, we will return the one that was finalized
		suite.headers.On("BlockIDByHeight", finalizedBlock.Height).Return(finalizedBlock.ID(), nil)

		// query the handler for the snapshot for non finalized block
		snapshotBytes, err := backend.GetProtocolStateSnapshotByBlockID(ctx, orphanBlock.ID())
		suite.Require().Nil(snapshotBytes)
		suite.Require().Error(err)
		suite.Require().Equal(codes.InvalidArgument, status.Code(err))
		suite.assertAllExpectations()
	})
}

// TestGetProtocolStateSnapshotByBlockID_UnexpectedErrorBlockIDByHeight tests our GetProtocolStateSnapshotByBlockID RPC
// endpoint where a unexpected error from BlockIDByHeight appear.
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_UnexpectedErrorBlockIDByHeight() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolState, err := rootSnapshot.ProtocolState()
	require.NoError(suite.T(), err)
	rootProtocolStateID := rootProtocolState.ID()
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState) {
		rootBlock, err := rootSnapshot.Head()
		suite.Require().NoError(err)

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// create a new block with root block as parent
		newBlock := unittest.BlockWithParentAndPayload(
			rootBlock,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		ctx := context.Background()
		// add new block to the chain state
		err = state.Extend(ctx, unittest.ProposalFromBlock(newBlock))
		suite.Require().NoError(err)

		// since block was added to the block tree it must be queryable by block ID
		suite.state.On("AtBlockID", newBlock.ID()).Return(state.AtBlockID(newBlock.ID()))
		// expectedError := errors.New("runtime-error")
		suite.headers.On("BlockIDByHeight", newBlock.Height).Return(flow.ZeroID,
			status.Errorf(codes.Internal, "failed to lookup block id by height %d", newBlock.Height))

		// query the handler for the snapshot
		snapshotBytes, err := backend.GetProtocolStateSnapshotByBlockID(ctx, newBlock.ID())
		suite.Require().Nil(snapshotBytes)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.assertAllExpectations()
	})
}

// TestGetProtocolStateSnapshotByBlockID_InvalidSegment tests our GetProtocolStateSnapshotByBlockID RPC endpoint
// for segments between phases and between epochs. We should return a valid snapshot in these edge cases.
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_InvalidSegment() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)
		// build epoch 1
		// Blocks in current State
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit|
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID and AtHeight mock returns for State
		for _, height := range epoch1.Range() {
			snap := state.AtHeight(height)
			blockHead, err := snap.Head()
			suite.Require().NoError(err)

			suite.state.On("AtHeight", height).Return(snap)
			suite.state.On("AtBlockID", blockHead.ID()).Return(snap)
			suite.headers.On("BlockIDByHeight", height).Return(blockHead.ID(), nil)
		}

		backend, err := New(suite.defaultBackendParams())
		suite.Require().NoError(err)

		suite.T().Run("sealing segment between phases", func(t *testing.T) {
			// Take snapshot at height of first block of setup phase
			snap := state.AtHeight(epoch1.SetupRange()[0])
			block, err := snap.Head()
			suite.Require().NoError(err)

			expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
			suite.Require().NoError(err)

			suite.T().Run("ByBlockID", func(t *testing.T) {
				bytes, err := backend.GetProtocolStateSnapshotByBlockID(context.Background(), block.ID())
				suite.Require().NoError(err)
				suite.Require().Equal(expectedSnapshotBytes, bytes)
			})
			suite.T().Run("ByHeight", func(t *testing.T) {
				bytes, err := backend.GetProtocolStateSnapshotByHeight(context.Background(), block.Height)
				suite.Require().NoError(err)
				suite.Require().Equal(expectedSnapshotBytes, bytes)
			})
		})

		suite.T().Run("sealing segment between epochs", func(t *testing.T) {
			// Take snapshot at height of latest finalized block
			snap := state.Final()
			currentEpoch, err := snap.Epochs().Current()
			suite.Require().NoError(err)
			suite.Require().Equal(epoch1.Counter+1, currentEpoch.Counter(), "expect to be in next epoch")
			block, err := snap.Head()
			suite.Require().NoError(err)

			suite.state.On("AtBlockID", block.ID()).Return(snap)
			suite.state.On("AtHeight", block.Height).Return(snap)
			suite.headers.On("BlockIDByHeight", block.Height).Return(block.ID(), nil)

			expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
			suite.Require().NoError(err)

			suite.T().Run("ByBlockID", func(t *testing.T) {
				bytes, err := backend.GetProtocolStateSnapshotByBlockID(context.Background(), block.ID())
				suite.Require().NoError(err)
				suite.Require().Equal(expectedSnapshotBytes, bytes)
			})
			suite.T().Run("ByHeight", func(t *testing.T) {
				bytes, err := backend.GetProtocolStateSnapshotByHeight(context.Background(), block.Height)
				suite.Require().NoError(err)
				suite.Require().Equal(expectedSnapshotBytes, bytes)
			})
		})
	})
}

// TestGetProtocolStateSnapshotByHeight tests our GetProtocolStateSnapshotByHeight RPC endpoint
// where non finalized block is added to state
func (suite *Suite) TestGetProtocolStateSnapshotByHeight() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState, mutableState realprotocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)
		// build epoch 1
		// Blocks in current State
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit|
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtHeight mock returns for State
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at height of block D (epoch1.heights[2]) for valid segment and valid snapshot
		snap := state.AtHeight(epoch1.Range()[2])

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetProtocolStateSnapshotByHeight(context.Background(), epoch1.Range()[2])
		suite.Require().NoError(err)

		// we expect the endpoint to return the snapshot at the same height we requested
		expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetProtocolStateSnapshotByHeight_NonFinalizedBlocks tests our GetProtocolStateSnapshotByHeight RPC endpoint
// where non finalized block is added to state
func (suite *Suite) TestGetProtocolStateSnapshotByHeight_NonFinalizedBlocks() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolState, err := rootSnapshot.ProtocolState()
	require.NoError(suite.T(), err)
	rootProtocolStateID := rootProtocolState.ID()
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db storage.DB, state *bprotocol.ParticipantState) {
		rootBlock, err := rootSnapshot.Head()
		suite.Require().NoError(err)
		// create a new block with root block as parent
		newBlock := unittest.BlockWithParentAndPayload(
			rootBlock,
			unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
		)
		ctx := context.Background()
		// add new block to the chain state
		err = state.Extend(ctx, unittest.ProposalFromBlock(newBlock))
		suite.Require().NoError(err)

		// since block was not yet finalized AtHeight must return an invalid snapshot
		suite.state.On("AtHeight", newBlock.Height).Return(invalid.NewSnapshot(realstate.ErrUnknownSnapshotReference))

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the snapshot for non finalized block
		bytes, err := backend.GetProtocolStateSnapshotByHeight(context.Background(), newBlock.Height)

		suite.Require().Nil(bytes)
		suite.Require().Error(err)
		suite.Require().Equal(status.Errorf(codes.NotFound, "failed to find snapshot: %v",
			realstate.ErrUnknownSnapshotReference).Error(),
			err.Error())
	})
}

func (suite *Suite) TestGetLatestSealedBlockHeader() {
	// setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Sealed").Return(suite.snapshot, nil)

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.Run("GetLatestSealedBlockHeader - happy path", func() {
		block := unittest.BlockHeaderFixture()
		suite.snapshot.On("Head").Return(block, nil).Once()

		// query the handler for the latest sealed block
		header, stat, err := backend.GetLatestBlockHeader(context.Background(), true)
		suite.Require().NoError(err)
		suite.Require().NotNil(header)

		// make sure we got the latest sealed block
		suite.Require().Equal(block.ID(), header.ID())
		suite.Require().Equal(block.Height, header.Height)
		suite.Require().Equal(block.ParentID, header.ParentID)
		suite.Require().Equal(stat, flow.BlockStatusSealed)

		suite.assertAllExpectations()
	})

	// tests that signaler context received error when node state is inconsistent
	suite.Run("GetLatestSealedBlockHeader - fails with inconsistent node's state", func() {
		err := fmt.Errorf("inconsistent node's state")
		suite.snapshot.On("Head").Return(nil, err)

		// mock signaler context expect an error
		signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(), signCtxErr))

		actualHeader, actualStatus, err := backend.GetLatestBlockHeader(signalerCtx, true)
		suite.Require().Error(err)
		suite.Require().Nil(actualHeader)
		suite.Require().Equal(flow.BlockStatusUnknown, actualStatus)
	})
}

func (suite *Suite) TestGetTransaction() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	transaction := unittest.TransactionFixture()
	expected := transaction.TransactionBody

	suite.transactions.
		On("ByID", transaction.ID()).
		Return(&expected, nil).
		Once()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	actual, err := backend.GetTransaction(context.Background(), transaction.ID())
	suite.Require().NoError(err)
	suite.Require().NotNil(actual)

	suite.Require().Equal(expected, *actual)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetCollection() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	expected := unittest.CollectionFixture(1).Light()

	suite.collections.
		On("LightByID", expected.ID()).
		Return(expected, nil).
		Once()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	actual, err := backend.GetCollectionByID(context.Background(), expected.ID())
	suite.transactions.AssertExpectations(suite.T())
	suite.Require().NoError(err)
	suite.Require().NotNil(actual)

	suite.Equal(expected, actual)
	suite.assertAllExpectations()
}

// TestGetTransactionResultByIndex tests that the request is forwarded to EN
func (suite *Suite) TestGetTransactionResultByIndex() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	block := unittest.BlockFixture()
	blockId := block.ID()
	index := uint32(0)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)

	_, fixedENIDs := suite.setupReceipts(block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	exeEventReq := &execproto.GetTransactionByIndexRequest{
		BlockId: blockId[:],
		Index:   index,
	}

	exeEventResp := &execproto.GetTransactionResultResponse{
		Events: nil,
	}

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.execClient.
		On("GetTransactionResultByIndex", mock.Anything, exeEventReq).
		Return(exeEventResp, nil)

	suite.Run("TestGetTransactionResultByIndex - happy path", func() {
		suite.snapshot.On("Head").Return(block.ToHeader(), nil).Once()
		result, err := backend.GetTransactionResultByIndex(ctx, blockId, index, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().NoError(err)
		suite.Require().NotNil(result)
		suite.Assert().Equal(result.BlockHeight, block.Height)

		suite.assertAllExpectations()
	})

	// tests that signaler context received error when node state is inconsistent
	suite.Run("TestGetTransactionResultByIndex - fails with inconsistent node's state", func() {
		err := fmt.Errorf("inconsistent node's state")
		suite.snapshot.On("Head").Return(nil, err).Once()

		// mock signaler context expect an error
		signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(), signCtxErr))

		actual, err := backend.GetTransactionResultByIndex(signalerCtx, blockId, index, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().Error(err)
		suite.Require().Nil(actual)
	})
}

func (suite *Suite) TestGetTransactionResultsByBlockID() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()

	sporkRootBlockHeight := suite.state.Params().SporkRootBlockHeight()
	block := unittest.BlockFixture(
		unittest.Block.WithHeight(sporkRootBlockHeight + 1),
	)
	blockId := block.ID()

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)

	_, fixedENIDs := suite.setupReceipts(block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	exeEventReq := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockId[:],
	}

	exeEventResp := &execproto.GetTransactionResultsResponse{
		TransactionResults: []*execproto.GetTransactionResultResponse{{}},
	}

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.execClient.
		On("GetTransactionResultsByBlockID", mock.Anything, exeEventReq).
		Return(exeEventResp, nil)

	suite.Run("GetTransactionResultsByBlockID - happy path", func() {
		suite.snapshot.On("Head").Return(block.ToHeader(), nil).Once()

		result, err := backend.GetTransactionResultsByBlockID(ctx, blockId, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().NoError(err)
		suite.Require().NotNil(result)

		suite.assertAllExpectations()
	})

	// tests that signaler context received error when node state is inconsistent
	suite.Run("GetTransactionResultsByBlockID - fails with inconsistent node's state", func() {
		err := fmt.Errorf("inconsistent node's state")
		suite.snapshot.On("Head").Return(nil, err).Once()

		// mock signaler context expect an error
		signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(), signCtxErr))

		actual, err := backend.GetTransactionResultsByBlockID(signalerCtx, blockId, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().Error(err)
		suite.Require().Nil(actual)
	})
}

// TestTransactionStatusTransition tests that the status of transaction changes from Finalized to Sealed
// when the protocol State is updated
func (suite *Suite) TestTransactionStatusTransition() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture(
		unittest.Block.WithHeight(2),
		unittest.Block.WithPayload(unittest.PayloadFixture(
			unittest.WithGuarantees(
				unittest.CollectionGuaranteesWithCollectionIDFixture([]*flow.Collection{&collection})...),
		)),
	)
	headBlock := unittest.BlockFixture(
		unittest.Block.WithHeight(block.Height - 1), // head is behind the current block
	)

	suite.snapshot.
		On("Head").
		Return(func() *flow.Header {
			return headBlock.ToHeader()
		}, nil)

	light := collection.Light()
	suite.collections.On("LightByID", collection.ID()).Return(light, nil)

	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", transactionBody.ID()).
		Return(transactionBody, nil)

	// collection storage returns the corresponding collection
	suite.collections.
		On("LightByTransactionID", transactionBody.ID()).
		Return(light, nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByCollectionID", collection.ID()).
		Return(block, nil)

	txID := transactionBody.ID()
	blockID := block.ID()
	_, fixedENIDs := suite.setupReceipts(block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	exeEventReq := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}

	exeEventResp := &execproto.GetTransactionResultResponse{
		Events: nil,
	}

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()

	backend, err := New(params)
	suite.Require().NoError(err)

	// Successfully return empty event list
	suite.execClient.
		On("GetTransactionResult", ctx, exeEventReq).
		Return(exeEventResp, status.Errorf(codes.NotFound, "not found")).
		Times(len(fixedENIDs)) // should call each EN once

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.Require().NoError(err)
	suite.Require().NotNil(result)

	// status should be finalized since the sealed Blocks is smaller in height
	suite.Assert().Equal(flow.TransactionStatusFinalized, result.Status)

	// block ID should be included in the response
	suite.Assert().Equal(blockID, result.BlockID)

	// Successfully return empty event list from here on
	suite.execClient.
		On("GetTransactionResult", ctx, exeEventReq).
		Return(exeEventResp, nil)

	// second call - when block under test's height is greater height than the sealed head
	result, err = backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.Require().NoError(err)
	suite.Require().NotNil(result)

	// status should be executed since no `NotFound` error in the `GetTransactionResult` call
	suite.Assert().Equal(flow.TransactionStatusExecuted, result.Status)

	// now let the head block be finalized
	headBlock.Height = block.Height + 1

	// third call - when block under test's height is less than sealed head's height
	result, err = backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.Require().NoError(err)
	suite.Require().NotNil(result)

	// status should be sealed since the sealed Blocks is greater in height
	suite.Assert().Equal(flow.TransactionStatusSealed, result.Status)

	// now go far into the future
	headBlock.Height = block.Height + flow.DefaultTransactionExpiry + 1

	// fourth call - when block under test's height so much less than the head's height that it's considered expired,
	// but since there is a execution result, means it should retain it's sealed status
	result, err = backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.Require().NoError(err)
	suite.Require().NotNil(result)

	// status should be expired since
	suite.Assert().Equal(flow.TransactionStatusSealed, result.Status)

	suite.assertAllExpectations()
}

// TestTransactionExpiredStatusTransition tests that the status
// of transaction changes from Pending to Expired when enough Blocks pass
func (suite *Suite) TestTransactionExpiredStatusTransition() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture(
		unittest.Block.WithHeight(2),
	)
	transactionBody.ReferenceBlockID = block.ID()

	headBlock := unittest.BlockFixture(
		unittest.Block.WithHeight(block.Height - 1), // head is behind the current block
	)

	// set up GetLastFullBlockHeight mock
	fullHeight := headBlock.Height

	suite.snapshot.
		On("Head").
		Return(func() *flow.Header {
			return headBlock.ToHeader()
		}, nil)

	snapshotAtBlock := new(protocol.Snapshot)
	snapshotAtBlock.On("Head").Return(block.ToHeader(), nil)

	suite.state.
		On("AtBlockID", block.ID()).
		Return(snapshotAtBlock, nil)

	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", transactionBody.ID()).
		Return(transactionBody, nil)

	// collection storage returns a not found error
	suite.collections.
		On("LightByTransactionID", transactionBody.ID()).
		Return(nil, storage.ErrNotFound)

	txID := transactionBody.ID()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	// should return pending status when we have not observed an expiry block
	suite.Run("pending", func() {
		// referenced block isn't known yet, so should return pending status
		result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().NoError(err)
		suite.Require().NotNil(result)

		suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
	})

	// should return pending status when we have observed an expiry block but
	// have not observed all intermediary Collections
	suite.Run("expiry un-confirmed", func() {
		suite.Run("ONLY finalized expiry block", func() {
			// we have finalized an expiry block
			headBlock.Height = block.Height + flow.DefaultTransactionExpiry + 1
			// we have NOT observed all intermediary Collections
			fullHeight = block.Height + flow.DefaultTransactionExpiry/2
			err := suite.lastFullBlockHeight.Set(fullHeight)
			suite.Require().NoError(err)

			result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
			suite.Require().NoError(err)
			suite.Require().NotNil(result)
			suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		})

		// we have observed all intermediary Collections
		fullHeight = block.Height + flow.DefaultTransactionExpiry + 1
		err = suite.lastFullBlockHeight.Set(fullHeight)
		suite.Require().NoError(err)

		suite.Run("ONLY observed intermediary Collections", func() {
			// we have NOT finalized an expiry block
			headBlock.Height = block.Height + flow.DefaultTransactionExpiry/2

			result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
			suite.Require().NoError(err)
			suite.Require().NotNil(result)
			suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		})

		// should return expired status only when we have observed an expiry block
		// and have observed all intermediary Collections
		suite.Run("expired", func() {
			// we have finalized an expiry block
			headBlock.Height = block.Height + flow.DefaultTransactionExpiry + 1

			result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
			suite.Require().NoError(err)
			suite.Require().NotNil(result)
			suite.Assert().Equal(flow.TransactionStatusExpired, result.Status)
		})
	})

	suite.assertAllExpectations()
}

// TestTransactionPendingToFinalizedStatusTransition tests that the status of transaction changes from Finalized to Expired
func (suite *Suite) TestTransactionPendingToFinalizedStatusTransition() {
	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	// block which will eventually contain the transaction
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(
			unittest.WithGuarantees(
				unittest.CollectionGuaranteesWithCollectionIDFixture([]*flow.Collection{&collection})...),
		)),
	)
	blockID := block.ID()

	// reference block to which the transaction points to
	refBlock := unittest.BlockFixture(
		unittest.Block.WithHeight(2),
	)
	refBlockID := refBlock.ID()
	transactionBody.ReferenceBlockID = refBlockID
	txID := transactionBody.ID()

	headBlock := unittest.BlockFixture(
		unittest.Block.WithHeight(refBlock.Height - 1), // head is behind the current refBlock
	)

	suite.snapshot.
		On("Head").
		Return(headBlock.ToHeader(), nil)

	snapshotAtBlock := new(protocol.Snapshot)
	snapshotAtBlock.On("Head").Return(refBlock.ToHeader(), nil)

	_, enIDs := suite.setupReceipts(block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	suite.snapshot.On("Identities", mock.Anything).Return(enIDs, nil)

	suite.state.
		On("AtBlockID", refBlockID).
		Return(snapshotAtBlock, nil)

	suite.state.On("Final").Return(snapshotAtBlock, nil).Maybe()

	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", txID).
		Return(transactionBody, nil)

	currentState := flow.TransactionStatusPending // marker for the current State
	// collection storage returns a not found error if tx is pending, else it returns the collection light reference
	suite.collections.
		On("LightByTransactionID", txID).
		Return(func(txID flow.Identifier) *flow.LightCollection {
			if currentState == flow.TransactionStatusPending {
				return nil
			}
			collLight := collection.Light()
			return collLight
		},
			func(txID flow.Identifier) error {
				if currentState == flow.TransactionStatusPending {
					return storage.ErrNotFound
				}
				return nil
			})

	light := collection.Light()
	suite.collections.On("LightByID", mock.Anything).Return(light, nil)

	// refBlock storage returns the corresponding refBlock
	suite.blocks.
		On("ByCollectionID", collection.ID()).
		Return(block, nil)

	receipts, _ := suite.setupReceipts(block)

	exeEventReq := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}

	exeEventResp := &execproto.GetTransactionResultResponse{
		Events: nil,
	}

	// simulate that the execution node has not yet executed the transaction
	suite.execClient.
		On("GetTransactionResult", ctx, exeEventReq).
		Return(exeEventResp, status.Errorf(codes.NotFound, "not found")).
		Times(len(enIDs)) // should call each EN once

	// create a mock connection factory
	connFactory := suite.setupConnectionFactory()

	params := suite.defaultBackendParams()
	params.ConnFactory = connFactory
	params.MaxHeightRange = TEST_MAX_HEIGHT
	suite.preferredExecutionNodeIDs = flow.IdentifierList{receipts[0].ExecutorID}

	backend, err := New(params)
	suite.Require().NoError(err)

	// should return pending status when we have not observed collection for the transaction
	suite.Run("pending", func() {
		currentState = flow.TransactionStatusPending
		result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().NoError(err)
		suite.Require().NotNil(result)
		suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		// assert that no call to an execution node is made
		suite.execClient.AssertNotCalled(suite.T(), "GetTransactionResult", mock.Anything, mock.Anything)
	})

	// should return finalized status when we have observed collection for the transaction (after observing the
	// preceding sealed refBlock)
	suite.Run("finalized", func() {
		currentState = flow.TransactionStatusFinalized
		result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().NoError(err)
		suite.Require().NotNil(result)
		suite.Assert().Equal(flow.TransactionStatusFinalized, result.Status)
	})

	suite.assertAllExpectations()
}

// TestTransactionResultUnknown tests that the status of transaction is reported as unknown when it is not found in the
// local storage
func (suite *Suite) TestTransactionResultUnknown() {
	ctx := context.Background()
	txID := unittest.IdentifierFixture()

	// transaction storage returns an error
	suite.transactions.
		On("ByID", txID).
		Return(nil, storage.ErrNotFound)

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.Require().NoError(err)
	suite.Require().NotNil(result)

	// status should be reported as unknown
	suite.Assert().Equal(flow.TransactionStatusUnknown, result.Status)
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.Run("GetLatestFinalizedBlock - happy path", func() {
		// setup the mocks
		expected := unittest.BlockFixture()
		header := expected.ToHeader()

		suite.snapshot.
			On("Head").
			Return(header, nil).Once()

		suite.blocks.
			On("ByHeight", header.Height).
			Return(expected, nil)

		// query the handler for the latest finalized header
		actual, stat, err := backend.GetLatestBlock(context.Background(), false)
		suite.Require().NoError(err)
		suite.Require().NotNil(actual)

		// make sure we got the latest header
		suite.Require().Equal(expected, actual)
		suite.Assert().Equal(stat, flow.BlockStatusFinalized)

		suite.assertAllExpectations()
	})

	// tests that signaler context received error when node state is inconsistent
	suite.Run("GetLatestFinalizedBlock - fails with inconsistent node's state", func() {
		err := fmt.Errorf("inconsistent node's state")
		suite.snapshot.On("Head").Return(nil, err)

		// mock signaler context expect an error
		signCtxErr := irrecoverable.NewExceptionf("failed to lookup final header: %w", err)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(), signCtxErr))

		actualBlock, actualStatus, err := backend.GetLatestBlock(signalerCtx, false)
		suite.Require().Error(err)
		suite.Require().Nil(actualBlock)
		suite.Require().Equal(flow.BlockStatusUnknown, actualStatus)
	})
}

func (suite *Suite) TestGetExecutionResultByID() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	validExecutorIdentities := flow.IdentityList{}
	validENIDs := flow.IdentifierList(validExecutorIdentities.NodeIDs())

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())

	nonexistingID := unittest.IdentifierFixture()
	blockID := unittest.IdentifierFixture()
	executionResult := unittest.ExecutionResultFixture(
		unittest.WithExecutionResultBlockID(blockID))

	ctx := context.Background()

	results := new(storagemock.ExecutionResults)
	results.
		On("ByID", nonexistingID).
		Return(nil, storage.ErrNotFound)

	results.
		On("ByID", executionResult.ID()).
		Return(executionResult, nil)

	suite.Run("nonexisting execution result for id", func() {
		suite.fixedExecutionNodeIDs = validENIDs

		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		_, err = backend.GetExecutionResultByID(ctx, nonexistingID)

		suite.Assert().Error(err)
	})

	suite.Run("existing execution result id", func() {
		suite.fixedExecutionNodeIDs = validENIDs

		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		er, err := backend.GetExecutionResultByID(ctx, executionResult.ID())
		suite.Require().NoError(err)
		suite.Require().NotNil(er)

		require.Equal(suite.T(), executionResult, er)
	})

	results.AssertExpectations(suite.T())
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetExecutionResultByBlockID() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	validExecutorIdentities := flow.IdentityList{}
	validENIDs := flow.IdentifierList(validExecutorIdentities.NodeIDs())

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())

	blockID := unittest.IdentifierFixture()
	executionResult := unittest.ExecutionResultFixture(
		unittest.WithExecutionResultBlockID(blockID),
		unittest.WithServiceEvents(2))

	ctx := context.Background()

	nonexistingBlockID := unittest.IdentifierFixture()

	results := new(storagemock.ExecutionResults)
	results.
		On("ByBlockID", nonexistingBlockID).
		Return(nil, storage.ErrNotFound)

	results.
		On("ByBlockID", blockID).
		Return(executionResult, nil)

	suite.Run("nonexisting execution results", func() {
		suite.fixedExecutionNodeIDs = validENIDs

		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		_, err = backend.GetExecutionResultForBlockID(ctx, nonexistingBlockID)

		suite.Assert().Error(err)
	})

	suite.Run("existing execution results", func() {
		suite.fixedExecutionNodeIDs = validENIDs

		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		er, err := backend.GetExecutionResultForBlockID(ctx, blockID)
		suite.Require().NoError(err)
		suite.Require().NotNil(er)

		require.Equal(suite.T(), executionResult, er)
	})

	results.AssertExpectations(suite.T())
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetNodeVersionInfo() {
	sporkRootBlock := unittest.BlockHeaderFixture()
	nodeRootBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(sporkRootBlock.Height + 100))
	sporkID := unittest.IdentifierFixture()
	protocolStateVersion := uint64(1234)

	stateParams := protocol.NewParams(suite.T())
	stateParams.On("SporkID").Return(sporkID, nil)
	stateParams.On("SporkRootBlockHeight").Return(sporkRootBlock.Height, nil)
	stateParams.On("SealedRoot").Return(nodeRootBlock, nil)

	state := protocol.NewState(suite.T())
	snap := protocol.NewSnapshot(suite.T())
	kvstore := protocol.NewKVStoreReader(suite.T())
	state.On("Params").Return(stateParams, nil).Maybe()
	state.On("Final").Return(snap).Maybe()
	snap.On("ProtocolState").Return(kvstore, nil).Maybe()
	kvstore.On("GetProtocolStateVersion").Return(protocolStateVersion).Maybe()

	suite.Run("happy path", func() {
		expected := &accessmodel.NodeVersionInfo{
			Semver:               build.Version(),
			Commit:               build.Commit(),
			SporkId:              sporkID,
			ProtocolStateVersion: protocolStateVersion,
			SporkRootBlockHeight: sporkRootBlock.Height,
			NodeRootBlockHeight:  nodeRootBlock.Height,
			CompatibleRange:      nil,
		}

		params := suite.defaultBackendParams()
		params.State = state

		backend, err := New(params)
		suite.Require().NoError(err)

		actual, err := backend.GetNodeVersionInfo(context.Background())
		suite.Require().NoError(err)

		suite.Require().Equal(expected, actual)
	})

	suite.Run("start and end version set", func() {
		latestBlockHeight := nodeRootBlock.Height + 100
		versionBeacons := storagemock.NewVersionBeacons(suite.T())

		events := []*flow.SealedVersionBeacon{
			{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(flow.VersionBoundary{BlockHeight: nodeRootBlock.Height + 4, Version: "0.0.1"}),
				),
				SealHeight: nodeRootBlock.Height + 2,
			},
			{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(flow.VersionBoundary{BlockHeight: nodeRootBlock.Height + 12, Version: "0.0.2"}),
				),
				SealHeight: nodeRootBlock.Height + 10,
			},
			{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(flow.VersionBoundary{BlockHeight: latestBlockHeight - 8, Version: "0.0.3"}),
				),
				SealHeight: latestBlockHeight - 10,
			},
		}

		eventMap := make(map[uint64]*flow.SealedVersionBeacon, len(events))
		for _, event := range events {
			eventMap[event.SealHeight] = event
		}

		// make sure events are sorted descending by seal height
		sort.Slice(events, func(i, j int) bool {
			return events[i].SealHeight > events[j].SealHeight
		})

		versionBeacons.
			On("Highest", mock.AnythingOfType("uint64")).
			Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
				// iterating through events sorted descending by seal height
				// return the first event that was sealed in a height less than or equal to height
				for _, event := range events {
					if event.SealHeight <= height {
						return event, nil
					}
				}
				return nil, storage.ErrNotFound
			})

		var err error
		suite.versionControl, err = version.NewVersionControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("0.0.2"),
			nodeRootBlock.Height,
			latestBlockHeight,
		)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Start the VersionControl component.
		suite.versionControl.Start(irrecoverable.NewMockSignalerContext(suite.T(), ctx))
		unittest.RequireComponentsReadyBefore(suite.T(), 2*time.Second, suite.versionControl)

		expected := &accessmodel.NodeVersionInfo{
			Semver:               build.Version(),
			Commit:               build.Commit(),
			SporkId:              sporkID,
			ProtocolStateVersion: protocolStateVersion,
			SporkRootBlockHeight: sporkRootBlock.Height,
			NodeRootBlockHeight:  nodeRootBlock.Height,
			CompatibleRange: &accessmodel.CompatibleRange{
				StartHeight: nodeRootBlock.Height + 12,
				EndHeight:   latestBlockHeight - 9,
			},
		}

		params := suite.defaultBackendParams()
		params.State = state

		backend, err := New(params)
		suite.Require().NoError(err)

		actual, err := backend.GetNodeVersionInfo(ctx)
		suite.Require().NoError(err)

		suite.Require().Equal(expected, actual)
	})
}

func (suite *Suite) TestGetNetworkParameters() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	expectedChainID := flow.Mainnet

	params := suite.defaultBackendParams()
	params.ChainID = expectedChainID

	backend, err := New(params)
	suite.Require().NoError(err)

	actual := backend.GetNetworkParameters(context.Background())

	suite.Require().Equal(expectedChainID, actual.ChainID)
}

// TestGetTransactionResultEventEncodingVersion tests the GetTransactionResult function with different event encoding versions.
func (suite *Suite) TestGetTransactionResultEventEncodingVersion() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()

	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	// block which will eventually contain the transaction
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(
			unittest.WithGuarantees(
				unittest.CollectionGuaranteesWithCollectionIDFixture([]*flow.Collection{&collection})...),
		)),
	)
	blockId := block.ID()

	// reference block to which the transaction points to
	refBlock := unittest.BlockFixture(
		unittest.Block.WithHeight(2),
	)
	transactionBody.ReferenceBlockID = refBlock.ID()
	txId := transactionBody.ID()

	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", txId).
		Return(transactionBody, nil)

	light := collection.Light()
	suite.collections.On("LightByID", mock.Anything).Return(light, nil)

	suite.snapshot.On("Head").Return(block.ToHeader(), nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)

	_, fixedENIDs := suite.setupReceipts(block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()

	backend, err := New(params)
	suite.Require().NoError(err)

	ccfEvents, jsoncdcEvents := generateEncodedEvents(suite.T(), 1)
	eventMessages := convert.EventsToMessages(ccfEvents)

	for _, version := range eventEncodingVersions {
		suite.Run(fmt.Sprintf("test %s event encoding version for GetTransactionResult", version.String()), func() {
			exeEventResp := &execproto.GetTransactionResultResponse{
				Events:               eventMessages,
				EventEncodingVersion: entitiesproto.EventEncodingVersion_CCF_V0,
			}

			suite.execClient.
				On("GetTransactionResult", ctx, &execproto.GetTransactionResultRequest{
					BlockId:       blockId[:],
					TransactionId: txId[:],
				}).
				Return(exeEventResp, nil).
				Once()

			result, err := backend.GetTransactionResult(ctx, txId, blockId, flow.ZeroID, version)
			suite.Require().NoError(err)
			suite.Require().NotNil(result)

			var expectedResult []flow.Event
			switch version {
			case entitiesproto.EventEncodingVersion_CCF_V0:
				expectedResult = append(expectedResult, ccfEvents...)
			case entitiesproto.EventEncodingVersion_JSON_CDC_V0:
				expectedResult = append(expectedResult, jsoncdcEvents...)
			}

			suite.Assert().Equal(result.Events, expectedResult)
		})
	}
}

// TestGetTransactionResultEventEncodingVersion tests the GetTransactionResult function with different event encoding versions.
func (suite *Suite) TestGetTransactionResultByIndexAndBlockIdEventEncodingVersion() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	block := unittest.BlockFixture()
	blockId := block.ID()
	index := uint32(0)

	suite.snapshot.On("Head").Return(block.ToHeader(), nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)

	_, fixedENIDs := suite.setupReceipts(block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()

	backend, err := New(params)
	suite.Require().NoError(err)

	exeNodeEventEncodingVersion := entitiesproto.EventEncodingVersion_CCF_V0
	ccfEvents, jsoncdcEvents := generateEncodedEvents(suite.T(), 1)
	eventMessages := convert.EventsToMessages(ccfEvents)

	for _, version := range eventEncodingVersions {
		suite.Run(fmt.Sprintf("test %s event encoding version for GetTransactionResultByIndex", version.String()), func() {
			exeEventResp := &execproto.GetTransactionResultResponse{
				Events:               eventMessages,
				EventEncodingVersion: exeNodeEventEncodingVersion,
			}

			suite.execClient.
				On("GetTransactionResultByIndex", ctx, &execproto.GetTransactionByIndexRequest{
					BlockId: blockId[:],
					Index:   index,
				}).
				Return(exeEventResp, nil).
				Once()

			result, err := backend.GetTransactionResultByIndex(ctx, blockId, index, version)
			suite.Require().NoError(err)
			suite.Require().NotNil(result)

			var expectedResult []flow.Event
			switch version {
			case entitiesproto.EventEncodingVersion_CCF_V0:
				expectedResult = append(expectedResult, ccfEvents...)
			case entitiesproto.EventEncodingVersion_JSON_CDC_V0:
				expectedResult = append(expectedResult, jsoncdcEvents...)
			}

			suite.Assert().Equal(expectedResult, result.Events)
		})

		suite.Run(fmt.Sprintf("test %s event encoding version for GetTransactionResultsByBlockID", version.String()), func() {
			exeEventResp := &execproto.GetTransactionResultsResponse{
				TransactionResults: []*execproto.GetTransactionResultResponse{
					{
						Events:               eventMessages,
						EventEncodingVersion: exeNodeEventEncodingVersion,
					}},
				EventEncodingVersion: exeNodeEventEncodingVersion,
			}

			suite.execClient.
				On("GetTransactionResultsByBlockID", ctx, &execproto.GetTransactionsByBlockIDRequest{
					BlockId: blockId[:],
				}).
				Return(exeEventResp, nil).
				Once()

			results, err := backend.GetTransactionResultsByBlockID(ctx, blockId, version)
			suite.Require().NoError(err)
			suite.Require().NotNil(results)

			var expectedResult []flow.Event
			switch version {
			case entitiesproto.EventEncodingVersion_CCF_V0:
				expectedResult = append(expectedResult, ccfEvents...)
			case entitiesproto.EventEncodingVersion_JSON_CDC_V0:
				expectedResult = append(expectedResult, jsoncdcEvents...)
			}

			for _, result := range results {
				suite.Assert().Equal(result.Events, expectedResult)
			}
		})
	}
}

// TestNodeCommunicator tests special case for node communicator, when only one node available and communicator gets
// gobreaker.ErrOpenState
func (suite *Suite) TestNodeCommunicator() {
	head := unittest.BlockHeaderFixture()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(head, nil).Maybe()

	ctx := context.Background()
	block := unittest.BlockFixture()
	blockId := block.ID()

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)

	_, fixedENIDs := suite.setupReceipts(block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	exeEventReq := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockId[:],
	}

	// Left only one preferred execution node
	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()
	suite.preferredExecutionNodeIDs = flow.IdentifierList{fixedENIDs[0].NodeID}

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()

	backend, err := New(params)
	suite.Require().NoError(err)

	// Simulate closed circuit breaker error
	suite.execClient.
		On("GetTransactionResultsByBlockID", ctx, exeEventReq).
		Return(nil, gobreaker.ErrOpenState)

	result, err := backend.GetTransactionResultsByBlockID(ctx, blockId, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.Assert().Nil(result)
	suite.Assert().Error(err)
	suite.Assert().Equal(codes.Unavailable, status.Code(err))
}

func (suite *Suite) assertAllExpectations() {
	suite.snapshot.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.blocks.AssertExpectations(suite.T())
	suite.headers.AssertExpectations(suite.T())
	suite.collections.AssertExpectations(suite.T())
	suite.transactions.AssertExpectations(suite.T())
	suite.execClient.AssertExpectations(suite.T())
}

func (suite *Suite) setupReceipts(block *flow.Block) ([]*flow.ExecutionReceipt, flow.IdentityList) {
	ids := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	receipt1 := unittest.ReceiptForBlockFixture(block)
	receipt1.ExecutorID = ids[0].NodeID
	receipt2 := unittest.ReceiptForBlockFixture(block)
	receipt2.ExecutorID = ids[1].NodeID
	receipt1.ExecutionResult = receipt2.ExecutionResult

	receipts := flow.ExecutionReceiptList{receipt1, receipt2}
	suite.receipts.
		On("ByBlockID", block.ID()).
		Return(receipts, nil)

	return receipts, ids
}

func (suite *Suite) setupConnectionFactory() connection.ConnectionFactory {
	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mocks.MockCloser{}, nil)
	return connFactory
}

func generateEncodedEvents(t *testing.T, n int) ([]flow.Event, []flow.Event) {
	ccfEvents := unittest.EventGenerator.GetEventsWithEncoding(n, entities.EventEncodingVersion_CCF_V0)
	jsonEvents := make([]flow.Event, n)
	for i, e := range ccfEvents {
		jsonEvent, err := convert.CcfEventToJsonEvent(e)
		require.NoError(t, err)
		jsonEvents[i] = *jsonEvent
	}
	return ccfEvents, jsonEvents
}

func (suite *Suite) defaultBackendParams() Params {
	return Params{
		State:                suite.state,
		Blocks:               suite.blocks,
		Headers:              suite.headers,
		Collections:          suite.collections,
		Transactions:         suite.transactions,
		ExecutionReceipts:    suite.receipts,
		ExecutionResults:     suite.results,
		ChainID:              suite.chainID,
		CollectionRPC:        suite.colClient,
		MaxHeightRange:       events.DefaultMaxHeightRange,
		SnapshotHistoryLimit: DefaultSnapshotHistoryLimit,
		Communicator:         node_communicator.NewNodeCommunicator(false),
		AccessMetrics:        metrics.NewNoopCollector(),
		Log:                  suite.log,
		BlockTracker:         nil,
		TxResultQueryMode:    query_mode.IndexQueryModeExecutionNodesOnly,
		EventQueryMode:       query_mode.IndexQueryModeExecutionNodesOnly,
		ScriptExecutionMode:  query_mode.IndexQueryModeExecutionNodesOnly,
		LastFullBlockHeight:  suite.lastFullBlockHeight,
		VersionControl:       suite.versionControl,
		ExecNodeIdentitiesProvider: commonrpc.NewExecutionNodeIdentitiesProvider(
			suite.log,
			suite.state,
			suite.receipts,
			suite.preferredExecutionNodeIDs,
			suite.fixedExecutionNodeIDs,
		),
	}
}

// TestResolveHeightError tests the ResolveHeightError function for various scenarios where the block height
// is below the spork root height, below the node root height, above the node root height, or when a different
// error is provided. It validates that ResolveHeightError returns an appropriate error message for each case.
//
// Test cases:
// 1) If height is below the spork root height, it suggests using a historic node.
// 2) If height is below the node root height, it suggests using a different Access node.
// 3) If height is above the node root height, it returns the original error without modification.
// 4) If a non-storage-related error is provided, it returns the error as is.
func (suite *Suite) TestResolveHeightError() {
	tests := []struct {
		name              string
		height            uint64
		sporkRootHeight   uint64
		nodeRootHeight    uint64
		genericErr        error
		expectedErrorMsg  string
		expectOriginalErr bool
	}{
		{
			name:             "height below spork root height",
			height:           uint64(50),
			sporkRootHeight:  uint64(100),
			nodeRootHeight:   uint64(200),
			genericErr:       storage.ErrNotFound,
			expectedErrorMsg: "block height %d is less than the spork root block height 100. Try to use a historic node: %v"},
		{
			name:              "height below node root height",
			height:            uint64(150),
			sporkRootHeight:   uint64(100),
			nodeRootHeight:    uint64(200),
			genericErr:        storage.ErrNotFound,
			expectedErrorMsg:  "block height %d is less than the node's root block height 200. Try to use a different Access node: %v",
			expectOriginalErr: false,
		},
		{
			name:              "height above node root height",
			height:            uint64(205),
			sporkRootHeight:   uint64(100),
			nodeRootHeight:    uint64(200),
			genericErr:        storage.ErrNotFound,
			expectedErrorMsg:  "%v",
			expectOriginalErr: true,
		},
		{
			name:              "non-storage related error",
			height:            uint64(150),
			sporkRootHeight:   uint64(100),
			nodeRootHeight:    uint64(200),
			genericErr:        fmt.Errorf("some other error"),
			expectedErrorMsg:  "%v",
			expectOriginalErr: true,
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			stateParams := protocol.NewParams(suite.T())

			if errors.Is(test.genericErr, storage.ErrNotFound) {
				stateParams.On("SporkRootBlockHeight").Return(test.sporkRootHeight).Once()
				sealedRootHeader := unittest.BlockHeaderWithHeight(test.nodeRootHeight)
				stateParams.On("SealedRoot").Return(sealedRootHeader, nil).Once()
			}

			err := common.ResolveHeightError(stateParams, test.height, test.genericErr)

			if test.expectOriginalErr {
				suite.Assert().True(errors.Is(err, test.genericErr))
			} else {
				expectedError := fmt.Sprintf(test.expectedErrorMsg, test.height, test.genericErr)
				suite.Assert().Equal(err.Error(), expectedError)
			}
		})
	}
}

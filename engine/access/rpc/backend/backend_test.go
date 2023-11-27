package backend

import (
	"context"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessflow "github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/cmd/build"
	access "github.com/onflow/flow-go/engine/access/mock"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	realstate "github.com/onflow/flow-go/state"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/invalid"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
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

	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts
	results      *storagemock.ExecutionResults

	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	archiveClient          *access.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory
	communicator      *backendmock.Communicator

	chainID flow.ChainID
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
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(header.Height, nil)
	params.On("SealedRoot").Return(header, nil)
	suite.state.On("Params").Return(params)

	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)
	suite.results = new(storagemock.ExecutionResults)
	suite.colClient = new(access.AccessAPIClient)
	suite.archiveClient = new(access.AccessAPIClient)
	suite.execClient = new(access.ExecutionAPIClient)
	suite.chainID = flow.Testnet
	suite.historicalAccessClient = new(access.AccessAPIClient)
	suite.connectionFactory = connectionmock.NewConnectionFactory(suite.T())

	suite.communicator = new(backendmock.Communicator)
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
	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.snapshot.On("Head").Return(block, nil).Once()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	// query the handler for the latest finalized block
	header, stat, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.checkResponse(header, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)
	suite.Require().Equal(stat, flow.BlockStatusSealed)

	suite.assertAllExpectations()
}

// TestGetLatestProtocolStateSnapshot_NoTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the State requested at latest finalized  block does not contain any Blocks that
// spans an epoch or epoch phase transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_NoTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
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
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)

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
		// will have contain block spanning an epoch transition as well as an epoch phase transition.
		// This will cause our GetLatestProtocolStateSnapshot func to return a snapshot
		// at block with height 3, the first block of the staking phase of epoch1.

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

		// we expect the endpoint to return last valid snapshot which is the snapshot at block C (height 2)
		expectedSnapshotBytes, err := convert.SnapshotToBytes(state.AtHeight(epoch1.Range()[2]))
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
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
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

		// we expect the endpoint to return last valid snapshot which is the snapshot at block C (height 2)
		expectedSnapshotBytes, err := convert.SnapshotToBytes(state.AtHeight(epoch1.Range()[2]))
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_EpochTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the State requested at latest finalized  block contains a Blocks that
// spans an epoch transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_EpochTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
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

		// we expect the endpoint to return last valid snapshot which is the snapshot at the final block
		// of the previous epoch
		expectedSnapshotBytes, err := convert.SnapshotToBytes(state.AtHeight(epoch1.Range()[len(epoch1.Range())-1]))
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_EpochTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the length of the sealing segment is greater than the configured SnapshotHistoryLimit
func (suite *Suite) TestGetLatestProtocolStateSnapshot_HistoryLimit() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state).BuildEpoch().CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID mock returns for State
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at height of block E (epoch1.heights[4]) the sealing segment for this snapshot
		// is C(S_A) <- D(S_B) |setup| <- E(S_C) which spans the epoch setup phase. This will force
		// our RPC endpoint to return a snapshot at block D which is the snapshot at the boundary where a phase
		// transition happens.
		snap := state.AtHeight(epoch1.Range()[4])
		suite.state.On("Final").Return(snap).Once()

		params := suite.defaultBackendParams()
		// very short history limit, any segment with any Blocks spanning any transition should force the endpoint to return a history limit error
		params.SnapshotHistoryLimit = 1

		backend, err := New(params)
		suite.Require().NoError(err)

		// the handler should return a snapshot history limit error
		_, err = backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().ErrorIs(err, SnapshotHistoryLimitErr)
	})
}

// TestGetProtocolStateSnapshotByBlockID tests our GetProtocolStateSnapshotByBlockID RPC endpoint
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
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

// TestGetProtocolStateSnapshotByBlockID_Non_Finalized_Blocks tests our GetProtocolStateSnapshotByBlockID RPC endpoint
// where non finalized block is added to state
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_NonFinalizedBlocks() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		rootBlock, err := rootSnapshot.Head()
		suite.Require().NoError(err)
		// create a new block with root block as parent
		newBlock := unittest.BlockWithParentFixture(rootBlock)
		ctx := context.Background()
		// add new block to the chain state
		err = state.Extend(ctx, newBlock)
		suite.Require().NoError(err)

		// since block was not yet finalized AtHeight must return an invalid snapshot
		suite.state.On("AtHeight", newBlock.Header.Height).Return(invalid.NewSnapshot(realstate.ErrUnknownSnapshotReference))
		// since block was added to the block tree it must be queryable by block ID
		suite.state.On("AtBlockID", newBlock.ID()).Return(state.AtBlockID(newBlock.ID()))

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the snapshot for non finalized block
		snapshotBytes, err := backend.GetProtocolStateSnapshotByBlockID(ctx, newBlock.ID())
		suite.Require().Nil(snapshotBytes)
		suite.Require().Error(err)
		suite.Require().Equal(status.Errorf(codes.InvalidArgument, "failed to retrieve snapshot for block by height %v",
			fmt.Errorf("%d: %v", newBlock.Header.Height, "block not finalized")).Error(),
			err.Error())
	})
}

// TestGetProtocolStateSnapshotByBlockID_InvalidSegment tests our GetProtocolStateSnapshotByBlockID RPC endpoint
// for invalid segment between phases and between epochs
func (suite *Suite) TestGetProtocolStateSnapshotByBlockID_InvalidSegment() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
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
		}

		backend, err := New(suite.defaultBackendParams())
		suite.Require().NoError(err)

		suite.T().Run("sealing segment between phases", func(t *testing.T) {
			// Take snapshot at height of first block of setup phase
			snap := state.AtHeight(epoch1.SetupRange()[0])
			block, err := snap.Head()
			suite.Require().NoError(err)

			bytes, err := backend.GetProtocolStateSnapshotByBlockID(context.Background(), block.ID())
			suite.Require().Error(err)
			suite.Require().Empty(bytes)
			suite.Require().Equal(status.Errorf(codes.InvalidArgument, "failed to retrieve snapshot for block, try again with different block: %v",
				SnapshotPhaseMismatchError).Error(),
				err.Error())
		})

		suite.T().Run("sealing segment between epochs", func(t *testing.T) {
			// Take snapshot at height of latest finalized block
			snap := state.Final()
			epochCounter, err := snap.Epochs().Current().Counter()
			suite.Require().NoError(err)
			suite.Require().Equal(epoch1.Counter+1, epochCounter, "expect to be in next epoch")
			block, err := snap.Head()
			suite.Require().NoError(err)

			suite.state.On("AtBlockID", block.ID()).Return(snap)
			suite.state.On("AtHeight", block.Height).Return(snap)

			bytes, err := backend.GetProtocolStateSnapshotByBlockID(context.Background(), block.ID())
			suite.Require().Error(err)
			suite.Require().Empty(bytes)
			suite.Require().Equal(status.Errorf(codes.InvalidArgument, "failed to retrieve snapshot for block, try again with different block: %v",
				SnapshotPhaseMismatchError).Error(),
				err.Error())
		})
	})
}

// TestGetProtocolStateSnapshotByHeight tests our GetProtocolStateSnapshotByHeight RPC endpoint
// where non finalized block is added to state
func (suite *Suite) TestGetProtocolStateSnapshotByHeight() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
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
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		rootBlock, err := rootSnapshot.Head()
		suite.Require().NoError(err)
		// create a new block with root block as parent
		newBlock := unittest.BlockWithParentFixture(rootBlock)
		ctx := context.Background()
		// add new block to the chain state
		err = state.Extend(ctx, newBlock)
		suite.Require().NoError(err)

		// since block was not yet finalized AtHeight must return an invalid snapshot
		suite.state.On("AtHeight", newBlock.Header.Height).Return(invalid.NewSnapshot(realstate.ErrUnknownSnapshotReference))

		params := suite.defaultBackendParams()
		params.MaxHeightRange = TEST_MAX_HEIGHT

		backend, err := New(params)
		suite.Require().NoError(err)

		// query the handler for the snapshot for non finalized block
		bytes, err := backend.GetProtocolStateSnapshotByHeight(context.Background(), newBlock.Header.Height)

		suite.Require().Nil(bytes)
		suite.Require().Error(err)
		suite.Require().Equal(status.Errorf(codes.NotFound, "failed to get a valid snapshot: %v",
			realstate.ErrUnknownSnapshotReference).Error(),
			err.Error())
	})
}

// TestGetProtocolStateSnapshotByHeight_InvalidSegment tests our GetProtocolStateSnapshotByHeight RPC endpoint
// for invalid segment
func (suite *Suite) TestGetProtocolStateSnapshotByHeight_InvalidSegment() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
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

		backend, err := New(suite.defaultBackendParams())
		suite.Require().NoError(err)

		// query the handler for the snapshot with invalid segment
		bytes, err := backend.GetProtocolStateSnapshotByHeight(context.Background(), epoch1.SetupRange()[0])

		suite.Require().Nil(bytes)
		suite.Require().Error(err)
		suite.Require().Equal(status.Errorf(codes.InvalidArgument, "failed to get a valid snapshot: %v",
			SnapshotPhaseMismatchError).Error(),
			err.Error())
	})
}

func (suite *Suite) TestGetLatestSealedBlockHeader() {
	// setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(block, nil).Once()

	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.snapshot.On("Head").Return(block, nil).Once()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	// query the handler for the latest sealed block
	header, stat, err := backend.GetLatestBlockHeader(context.Background(), true)
	suite.checkResponse(header, err)

	// make sure we got the latest sealed block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)
	suite.Require().Equal(stat, flow.BlockStatusSealed)

	suite.assertAllExpectations()
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
	suite.checkResponse(actual, err)

	suite.Require().Equal(expected, *actual)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetCollection() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	expected := unittest.CollectionFixture(1).Light()

	suite.collections.
		On("LightByID", expected.ID()).
		Return(&expected, nil).
		Once()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	actual, err := backend.GetCollectionByID(context.Background(), expected.ID())
	suite.transactions.AssertExpectations(suite.T())
	suite.checkResponse(actual, err)

	suite.Equal(expected, *actual)
	suite.assertAllExpectations()
}

// TestGetTransactionResultByIndex tests that the request is forwarded to EN
func (suite *Suite) TestGetTransactionResultByIndex() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	block := unittest.BlockFixture()
	blockId := block.ID()
	index := uint32(0)

	suite.snapshot.On("Head").Return(block.Header, nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil)

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	exeEventReq := &execproto.GetTransactionByIndexRequest{
		BlockId: blockId[:],
		Index:   index,
	}

	exeEventResp := &execproto.GetTransactionResultResponse{
		Events: nil,
	}

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = (fixedENIDs.NodeIDs()).Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.execClient.
		On("GetTransactionResultByIndex", ctx, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	result, err := backend.GetTransactionResultByIndex(ctx, blockId, index, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.checkResponse(result, err)
	suite.Assert().Equal(result.BlockHeight, block.Header.Height)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetTransactionResultsByBlockID() {
	head := unittest.BlockHeaderFixture()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(head, nil).Maybe()

	ctx := context.Background()
	block := unittest.BlockFixture()
	blockId := block.ID()

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil)

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	exeEventReq := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockId[:],
	}

	exeEventResp := &execproto.GetTransactionResultsResponse{
		TransactionResults: []*execproto.GetTransactionResultResponse{{}},
	}

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = (fixedENIDs.NodeIDs()).Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.execClient.
		On("GetTransactionResultsByBlockID", ctx, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	result, err := backend.GetTransactionResultsByBlockID(ctx, blockId, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.checkResponse(result, err)

	suite.assertAllExpectations()
}

// TestTransactionStatusTransition tests that the status of transaction changes from Finalized to Sealed
// when the protocol State is updated
func (suite *Suite) TestTransactionStatusTransition() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	block.Header.Height = 2
	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = block.Header.Height - 1 // head is behind the current block
	block.SetPayload(
		unittest.PayloadFixture(
			unittest.WithGuarantees(
				unittest.CollectionGuaranteesWithCollectionIDFixture([]*flow.Collection{&collection})...)))

	suite.snapshot.
		On("Head").
		Return(headBlock.Header, nil)

	light := collection.Light()
	suite.collections.On("LightByID", light.ID()).Return(&light, nil)

	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", transactionBody.ID()).
		Return(transactionBody, nil)

	// collection storage returns the corresponding collection
	suite.collections.
		On("LightByTransactionID", transactionBody.ID()).
		Return(&light, nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByCollectionID", collection.ID()).
		Return(&block, nil)

	txID := transactionBody.ID()
	blockID := block.ID()
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	exeEventReq := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}

	exeEventResp := &execproto.GetTransactionResultResponse{
		Events: nil,
	}

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = (fixedENIDs.NodeIDs()).Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	// Successfully return empty event list
	suite.execClient.
		On("GetTransactionResult", ctx, exeEventReq).
		Return(exeEventResp, status.Errorf(codes.NotFound, "not found")).
		Times(len(fixedENIDs)) // should call each EN once

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.checkResponse(result, err)

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
	suite.checkResponse(result, err)

	// status should be executed since no `NotFound` error in the `GetTransactionResult` call
	suite.Assert().Equal(flow.TransactionStatusExecuted, result.Status)

	// now let the head block be finalized
	headBlock.Header.Height = block.Header.Height + 1

	// third call - when block under test's height is less than sealed head's height
	result, err = backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.checkResponse(result, err)

	// status should be sealed since the sealed Blocks is greater in height
	suite.Assert().Equal(flow.TransactionStatusSealed, result.Status)

	// now go far into the future
	headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1

	// fourth call - when block under test's height so much less than the head's height that it's considered expired,
	// but since there is a execution result, means it should retain it's sealed status
	result, err = backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	suite.checkResponse(result, err)

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
	block := unittest.BlockFixture()
	block.Header.Height = 2
	transactionBody.SetReferenceBlockID(block.ID())

	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = block.Header.Height - 1 // head is behind the current block

	// set up GetLastFullBlockHeight mock
	fullHeight := headBlock.Header.Height
	suite.blocks.On("GetLastFullBlockHeight").Return(
		func() uint64 { return fullHeight },
		func() error { return nil },
	)

	suite.snapshot.
		On("Head").
		Return(headBlock.Header, nil)

	snapshotAtBlock := new(protocol.Snapshot)
	snapshotAtBlock.On("Head").Return(block.Header, nil)

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
		suite.checkResponse(result, err)

		suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
	})

	// should return pending status when we have observed an expiry block but
	// have not observed all intermediary Collections
	suite.Run("expiry un-confirmed", func() {

		suite.Run("ONLY finalized expiry block", func() {
			// we have finalized an expiry block
			headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1
			// we have NOT observed all intermediary Collections
			fullHeight = block.Header.Height + flow.DefaultTransactionExpiry/2

			result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
			suite.checkResponse(result, err)
			suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		})
		suite.Run("ONLY observed intermediary Collections", func() {
			// we have NOT finalized an expiry block
			headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry/2
			// we have observed all intermediary Collections
			fullHeight = block.Header.Height + flow.DefaultTransactionExpiry + 1

			result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
			suite.checkResponse(result, err)
			suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		})

	})

	// should return expired status only when we have observed an expiry block
	// and have observed all intermediary Collections
	suite.Run("expired", func() {
		// we have finalized an expiry block
		headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1
		// we have observed all intermediary Collections
		fullHeight = block.Header.Height + flow.DefaultTransactionExpiry + 1

		result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.checkResponse(result, err)
		suite.Assert().Equal(flow.TransactionStatusExpired, result.Status)
	})

	suite.assertAllExpectations()
}

// TestTransactionPendingToFinalizedStatusTransition tests that the status of transaction changes from Finalized to Expired
func (suite *Suite) TestTransactionPendingToFinalizedStatusTransition() {

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	// block which will eventually contain the transaction
	block := unittest.BlockFixture()
	block.SetPayload(
		unittest.PayloadFixture(
			unittest.WithGuarantees(
				unittest.CollectionGuaranteesWithCollectionIDFixture([]*flow.Collection{&collection})...)))
	blockID := block.ID()

	// reference block to which the transaction points to
	refBlock := unittest.BlockFixture()
	refBlockID := refBlock.ID()
	refBlock.Header.Height = 2
	transactionBody.SetReferenceBlockID(refBlockID)
	txID := transactionBody.ID()

	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = refBlock.Header.Height - 1 // head is behind the current refBlock

	suite.snapshot.
		On("Head").
		Return(headBlock.Header, nil)

	snapshotAtBlock := new(protocol.Snapshot)
	snapshotAtBlock.On("Head").Return(refBlock.Header, nil)

	_, enIDs := suite.setupReceipts(&block)
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
			return &collLight
		},
			func(txID flow.Identifier) error {
				if currentState == flow.TransactionStatusPending {
					return storage.ErrNotFound
				}
				return nil
			})

	light := collection.Light()
	suite.collections.On("LightByID", mock.Anything).Return(&light, nil)

	// refBlock storage returns the corresponding refBlock
	suite.blocks.
		On("ByCollectionID", collection.ID()).
		Return(&block, nil)

	receipts, _ := suite.setupReceipts(&block)

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

	backend, err := New(params)
	suite.Require().NoError(err)

	preferredENIdentifiers = flow.IdentifierList{receipts[0].ExecutorID}

	// should return pending status when we have not observed collection for the transaction
	suite.Run("pending", func() {
		currentState = flow.TransactionStatusPending
		result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.checkResponse(result, err)
		suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		// assert that no call to an execution node is made
		suite.execClient.AssertNotCalled(suite.T(), "GetTransactionResult", mock.Anything, mock.Anything)
	})

	// should return finalized status when we have observed collection for the transaction (after observing the
	// preceding sealed refBlock)
	suite.Run("finalized", func() {
		currentState = flow.TransactionStatusFinalized
		result, err := backend.GetTransactionResult(ctx, txID, flow.ZeroID, flow.ZeroID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.checkResponse(result, err)
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
	suite.checkResponse(result, err)

	// status should be reported as unknown
	suite.Assert().Equal(flow.TransactionStatusUnknown, result.Status)
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	expected := unittest.BlockFixture()
	header := expected.Header

	suite.snapshot.
		On("Head").
		Return(header, nil).Once()

	headerClone := *header
	headerClone.Height = 0

	suite.snapshot.
		On("Head").
		Return(&headerClone, nil).
		Once()

	suite.blocks.
		On("ByHeight", header.Height).
		Return(&expected, nil)

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	// query the handler for the latest finalized header
	actual, stat, err := backend.GetLatestBlock(context.Background(), false)
	suite.checkResponse(actual, err)

	// make sure we got the latest header
	suite.Require().Equal(expected, *actual)
	suite.Assert().Equal(stat, flow.BlockStatusFinalized)

	suite.assertAllExpectations()
}

type mockCloser struct{}

func (mc *mockCloser) Close() error { return nil }

func (suite *Suite) TestGetEventsForBlockIDs() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	exeNodeEventEncodingVersion := entitiesproto.EventEncodingVersion_CCF_V0
	events := generator.GetEventsWithEncoding(10, exeNodeEventEncodingVersion)
	validExecutorIdentities := flow.IdentityList{}

	setupStorage := func(n int) []*flow.Header {
		headers := make([]*flow.Header, n)
		ids := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))

		for i := 0; i < n; i++ {
			b := unittest.BlockFixture()
			suite.headers.
				On("ByBlockID", b.ID()).
				Return(b.Header, nil).Once()

			headers[i] = b.Header

			receipt1 := unittest.ReceiptForBlockFixture(&b)
			receipt1.ExecutorID = ids[0].NodeID
			receipt2 := unittest.ReceiptForBlockFixture(&b)
			receipt2.ExecutorID = ids[1].NodeID
			receipt1.ExecutionResult = receipt2.ExecutionResult
			suite.receipts.
				On("ByBlockID", b.ID()).
				Return(flow.ExecutionReceiptList{receipt1, receipt2}, nil).Once()
			validExecutorIdentities = append(validExecutorIdentities, ids...)
		}

		return headers
	}
	blockHeaders := setupStorage(5)

	suite.snapshot.On("Identities", mock.Anything).Return(validExecutorIdentities, nil)
	validENIDs := flow.IdentifierList(validExecutorIdentities.NodeIDs())

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// create the expected results from execution node and access node
	exeResults := make([]*execproto.GetEventsForBlockIDsResponse_Result, len(blockHeaders))

	for i := 0; i < len(blockHeaders); i++ {
		exeResults[i] = &execproto.GetEventsForBlockIDsResponse_Result{
			BlockId:     convert.IdentifierToMessage(blockHeaders[i].ID()),
			BlockHeight: blockHeaders[i].Height,
			Events:      convert.EventsToMessages(events),
		}
	}

	expected := make([]flow.BlockEvents, len(blockHeaders))
	expectedEvents := generator.GetEventsWithEncoding(10, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	for i := 0; i < len(blockHeaders); i++ {
		expected[i] = flow.BlockEvents{
			BlockID:        blockHeaders[i].ID(),
			BlockHeight:    blockHeaders[i].Height,
			BlockTimestamp: blockHeaders[i].Timestamp,
			Events:         expectedEvents,
		}
	}

	// create the execution node response
	exeResp := &execproto.GetEventsForBlockIDsResponse{
		Results:              exeResults,
		EventEncodingVersion: exeNodeEventEncodingVersion,
	}

	ctx := context.Background()

	blockIDs := make([]flow.Identifier, len(blockHeaders))
	for i, header := range blockHeaders {
		blockIDs[i] = header.ID()
	}
	exeReq := &execproto.GetEventsForBlockIDsRequest{
		BlockIds: convert.IdentifiersToMessages(blockIDs),
		Type:     string(flow.EventAccountCreated),
	}

	// create receipt mocks that always returns empty
	receipts := new(storagemock.ExecutionReceipts)
	receipts.
		On("ByBlockID", mock.Anything).
		Return(flow.ExecutionReceiptList{}, nil)

	// expect two calls to the executor api client (one for each of the following 2 test cases)
	suite.execClient.
		On("GetEventsForBlockIDs", ctx, exeReq).
		Return(exeResp, nil).
		Once()

	suite.Run("with an execution node chosen using block ID form the list of Fixed ENs", func() {

		params := suite.defaultBackendParams()
		params.ConnFactory = connFactory
		// set the fixed EN Identifiers to the generated execution IDs
		params.FixedExecutionNodeIDs = validENIDs.Strings()

		// create the handler
		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		actual, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), blockIDs, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.checkResponse(actual, err)

		suite.Require().Equal(expected, actual)
	})

	suite.Run("with an empty block ID list", func() {

		params := suite.defaultBackendParams()
		params.ExecutionReceipts = receipts
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = validENIDs.Strings()

		// create the handler
		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request with an empty block id list and expect an empty list of events and no error
		resp, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), []flow.Identifier{}, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(suite.T(), err)
		require.Empty(suite.T(), resp)
	})

	for _, version := range eventEncodingVersions {
		suite.Run(fmt.Sprintf("test %s event encoding version for GetEventsForBlockIDs", version.String()), func() {
			params := suite.defaultBackendParams()
			params.ExecutionReceipts = receipts
			params.ConnFactory = connFactory
			params.FixedExecutionNodeIDs = validENIDs.Strings()

			// create the handler
			backend, err := New(params)
			suite.Require().NoError(err)

			// execute request with an empty block id list and expect an empty list of events and no error
			result, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), []flow.Identifier{}, version)
			expectedResult := generator.GetEventsWithEncoding(1, version)
			suite.checkResponse(result, err)

			for _, blockEvent := range result {
				suite.Assert().Equal(blockEvent.Events, expectedResult)
			}
		})
	}

	suite.assertAllExpectations()
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
		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = validENIDs.Strings()

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		_, err = backend.GetExecutionResultByID(ctx, nonexistingID)

		assert.Error(suite.T(), err)
	})

	suite.Run("existing execution result id", func() {
		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = validENIDs.Strings()

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		er, err := backend.GetExecutionResultByID(ctx, executionResult.ID())
		suite.checkResponse(er, err)

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
		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = validENIDs.Strings()

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		_, err = backend.GetExecutionResultForBlockID(ctx, nonexistingBlockID)

		assert.Error(suite.T(), err)
	})

	suite.Run("existing execution results", func() {
		params := suite.defaultBackendParams()
		params.ExecutionResults = results
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = validENIDs.Strings()

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		er, err := backend.GetExecutionResultForBlockID(ctx, blockID)
		suite.checkResponse(er, err)

		require.Equal(suite.T(), executionResult, er)
	})

	results.AssertExpectations(suite.T())
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetEventsForHeightRange() {

	ctx := context.Background()
	const minHeight uint64 = 5
	const maxHeight uint64 = 10
	var headHeight uint64
	var blockHeaders []*flow.Header
	var nodeIdentities flow.IdentityList

	headersDB := make(map[uint64]*flow.Header) // backend for storage.Headers
	var head *flow.Header                      // backend for Snapshot.Head

	state := new(protocol.State)
	snapshot := new(protocol.Snapshot)
	state.On("Final").Return(snapshot, nil)
	state.On("Sealed").Return(snapshot, nil)

	rootHeader := unittest.BlockHeaderFixture()
	stateParams := new(protocol.Params)
	stateParams.On("FinalizedRoot").Return(rootHeader, nil)
	state.On("Params").Return(stateParams).Maybe()

	// mock snapshot to return head backend
	snapshot.On("Head").Return(
		func() *flow.Header { return head },
		func() error { return nil },
	)
	snapshot.On("Identities", mock.Anything).Return(
		func(_ flow.IdentityFilter) flow.IdentityList {
			return nodeIdentities
		},
		func(flow.IdentityFilter) error { return nil },
	)

	// mock Headers to pull from Headers backend
	suite.headers.On("ByHeight", mock.Anything).Return(
		func(height uint64) *flow.Header {
			return headersDB[height]
		},
		func(height uint64) error {
			_, ok := headersDB[height]
			if !ok {
				return storage.ErrNotFound
			}
			return nil
		}).Maybe()

	setupHeadHeight := func(height uint64) {
		header := unittest.BlockHeaderFixture() // create a mock header
		header.Height = height                  // set the header height
		head = header
	}

	setupStorage := func(min uint64, max uint64) ([]*flow.Header, []*flow.ExecutionReceipt, flow.IdentityList) {
		headersDB = make(map[uint64]*flow.Header) // reset backend

		var headers []*flow.Header
		var ers []*flow.ExecutionReceipt
		var enIDs flow.IdentityList
		for i := min; i <= max; i++ {
			block := unittest.BlockFixture()
			header := block.Header
			headersDB[i] = header
			headers = append(headers, header)
			newErs, ids := suite.setupReceipts(&block)
			ers = append(ers, newErs...)
			enIDs = append(enIDs, ids...)
		}
		return headers, ers, enIDs
	}

	setupExecClient := func() []flow.BlockEvents {
		blockIDs := make([]flow.Identifier, len(blockHeaders))
		for i, header := range blockHeaders {
			blockIDs[i] = header.ID()
		}
		execReq := &execproto.GetEventsForBlockIDsRequest{
			BlockIds: convert.IdentifiersToMessages(blockIDs),
			Type:     string(flow.EventAccountCreated),
		}

		results := make([]flow.BlockEvents, len(blockHeaders))
		exeResults := make([]*execproto.GetEventsForBlockIDsResponse_Result, len(blockHeaders))

		exeNodeEventEncodingVersion := entitiesproto.EventEncodingVersion_CCF_V0

		for i, header := range blockHeaders {
			events := generator.GetEventsWithEncoding(1, exeNodeEventEncodingVersion)
			height := header.Height

			results[i] = flow.BlockEvents{
				BlockID:        header.ID(),
				BlockHeight:    height,
				BlockTimestamp: header.Timestamp,
				Events:         generator.GetEventsWithEncoding(1, entitiesproto.EventEncodingVersion_JSON_CDC_V0),
			}

			exeResults[i] = &execproto.GetEventsForBlockIDsResponse_Result{
				BlockId:     convert.IdentifierToMessage(header.ID()),
				BlockHeight: header.Height,
				Events:      convert.EventsToMessages(events),
			}
		}

		exeResp := &execproto.GetEventsForBlockIDsResponse{
			Results:              exeResults,
			EventEncodingVersion: exeNodeEventEncodingVersion,
		}

		suite.execClient.
			On("GetEventsForBlockIDs", ctx, execReq).
			Return(exeResp, nil).
			Once()

		return results
	}

	connFactory := suite.setupConnectionFactory()

	//suite.state = state
	suite.Run("invalid request max height < min height", func() {
		params := suite.defaultBackendParams()
		params.ConnFactory = connFactory

		backend, err := New(params)
		suite.Require().NoError(err)

		_, err = backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), maxHeight, minHeight, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().Error(err)

		suite.assertAllExpectations() // assert that request was not sent to execution node
	})

	suite.Run("valid request with min_height < max_height < last_sealed_block_height", func() {

		headHeight = maxHeight + 1

		// setup mocks
		setupHeadHeight(headHeight)
		blockHeaders, _, nodeIdentities = setupStorage(minHeight, maxHeight)
		expectedResp := setupExecClient()
		fixedENIdentifiersStr := flow.IdentifierList(nodeIdentities.NodeIDs()).Strings()

		stateParams.On("SporkID").Return(unittest.IdentifierFixture(), nil)
		stateParams.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
		stateParams.On("SporkRootBlockHeight").Return(headHeight, nil)
		stateParams.On("SealedRoot").Return(head, nil)

		params := suite.defaultBackendParams()
		params.State = state
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = fixedENIdentifiersStr

		backend, err := New(params)
		suite.Require().NoError(err)

		// execute request
		actualResp, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight, entitiesproto.EventEncodingVersion_JSON_CDC_V0)

		// check response
		suite.checkResponse(actualResp, err)
		suite.assertAllExpectations()
		suite.Require().Equal(expectedResp, actualResp)
	})

	suite.Run("valid request with max_height > last_sealed_block_height", func() {
		headHeight = maxHeight - 1
		setupHeadHeight(headHeight)
		blockHeaders, _, nodeIdentities = setupStorage(minHeight, headHeight)
		expectedResp := setupExecClient()
		fixedENIdentifiersStr := flow.IdentifierList(nodeIdentities.NodeIDs()).Strings()

		stateParams.On("SporkID").Return(unittest.IdentifierFixture(), nil)
		stateParams.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
		stateParams.On("SporkRootBlockHeight").Return(headHeight, nil)
		stateParams.On("SealedRoot").Return(head, nil)

		params := suite.defaultBackendParams()
		params.State = state
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = fixedENIdentifiersStr

		backend, err := New(params)
		suite.Require().NoError(err)

		actualResp, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.checkResponse(actualResp, err)

		suite.assertAllExpectations()
		suite.Require().Equal(expectedResp, actualResp)
	})

	// set max height range to 1 and request range of 2
	suite.Run("invalid request exceeding max height range", func() {
		headHeight = maxHeight - 1
		setupHeadHeight(headHeight)
		blockHeaders, _, nodeIdentities = setupStorage(minHeight, headHeight)
		fixedENIdentifiersStr := flow.IdentifierList(nodeIdentities.NodeIDs()).Strings()

		params := suite.defaultBackendParams()
		params.ConnFactory = connFactory
		params.MaxHeightRange = 1
		params.FixedExecutionNodeIDs = fixedENIdentifiersStr

		backend, err := New(params)
		suite.Require().NoError(err)

		_, err = backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, minHeight+1, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().Error(err)
	})

	suite.Run("invalid request last_sealed_block_height < min height", func() {

		// set sealed height to one less than the request start height
		headHeight = minHeight - 1

		// setup mocks
		setupHeadHeight(headHeight)
		blockHeaders, _, nodeIdentities = setupStorage(minHeight, maxHeight)
		fixedENIdentifiersStr := flow.IdentifierList(nodeIdentities.NodeIDs()).Strings()

		params := suite.defaultBackendParams()
		params.State = state
		params.ConnFactory = connFactory
		params.FixedExecutionNodeIDs = fixedENIdentifiersStr

		backend, err := New(params)
		suite.Require().NoError(err)

		_, err = backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().Error(err)
	})

	for _, version := range eventEncodingVersions {
		suite.Run(fmt.Sprintf("test %s event encoding version for GetEventsForHeightRange", version.String()), func() {
			headHeight = maxHeight - 1
			setupHeadHeight(headHeight)
			blockHeaders, _, nodeIdentities = setupStorage(minHeight, headHeight)
			_ = setupExecClient()
			fixedENIdentifiersStr := nodeIdentities.NodeIDs().Strings()

			stateParams.On("SporkID").Return(unittest.IdentifierFixture(), nil)
			stateParams.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
			stateParams.On("SporkRootBlockHeight").Return(headHeight, nil)
			stateParams.On("SealedRoot").Return(head, nil)

			params := suite.defaultBackendParams()
			params.State = state
			params.ConnFactory = connFactory
			params.FixedExecutionNodeIDs = fixedENIdentifiersStr

			backend, err := New(params)
			suite.Require().NoError(err)

			result, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight, version)
			expectedResult := generator.GetEventsWithEncoding(1, version)
			suite.checkResponse(result, err)

			for _, blockEvent := range result {
				suite.Assert().Equal(blockEvent.Events, expectedResult)
			}
		})
	}
}

func (suite *Suite) TestGetNodeVersionInfo() {
	sporkRootBlock := unittest.BlockHeaderFixture()
	nodeRootBlock := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(sporkRootBlock.Height + 100))
	sporkID := unittest.IdentifierFixture()
	protocolVersion := uint(1234)

	suite.Run("happy path", func() {
		stateParams := protocol.NewParams(suite.T())
		stateParams.On("SporkID").Return(sporkID, nil)
		stateParams.On("ProtocolVersion").Return(protocolVersion, nil)
		stateParams.On("SporkRootBlockHeight").Return(sporkRootBlock.Height, nil)
		stateParams.On("SealedRoot").Return(nodeRootBlock, nil)

		state := protocol.NewState(suite.T())
		state.On("Params").Return(stateParams, nil).Maybe()

		expected := &accessflow.NodeVersionInfo{
			Semver:               build.Version(),
			Commit:               build.Commit(),
			SporkId:              sporkID,
			ProtocolVersion:      uint64(protocolVersion),
			SporkRootBlockHeight: sporkRootBlock.Height,
			NodeRootBlockHeight:  nodeRootBlock.Height,
		}

		params := suite.defaultBackendParams()
		params.State = state

		backend, err := New(params)
		suite.Require().NoError(err)

		actual, err := backend.GetNodeVersionInfo(context.Background())
		suite.Require().NoError(err)

		suite.Require().Equal(expected, actual)
	})

	suite.Run("backend construct fails when SporkID lookup fails", func() {
		stateParams := protocol.NewParams(suite.T())
		stateParams.On("SporkID").Return(flow.ZeroID, fmt.Errorf("fail"))

		state := protocol.NewState(suite.T())
		state.On("Params").Return(stateParams, nil).Maybe()

		params := suite.defaultBackendParams()
		params.State = state

		backend, err := New(params)
		suite.Require().Error(err)
		suite.Require().Nil(backend)
	})

	suite.Run("backend construct fails when ProtocolVersion lookup fails", func() {
		stateParams := protocol.NewParams(suite.T())
		stateParams.On("SporkID").Return(sporkID, nil)
		stateParams.On("ProtocolVersion").Return(uint(0), fmt.Errorf("fail"))

		state := protocol.NewState(suite.T())
		state.On("Params").Return(stateParams, nil).Maybe()

		params := suite.defaultBackendParams()
		params.State = state

		backend, err := New(params)
		suite.Require().Error(err)
		suite.Require().Nil(backend)
	})

	suite.Run("backend construct fails when SporkRootBlockHeight lookup fails", func() {
		stateParams := protocol.NewParams(suite.T())
		stateParams.On("SporkID").Return(sporkID, nil)
		stateParams.On("ProtocolVersion").Return(protocolVersion, nil)
		stateParams.On("SporkRootBlockHeight").Return(uint64(0), fmt.Errorf("fail"))

		state := protocol.NewState(suite.T())
		state.On("Params").Return(stateParams, nil).Maybe()

		params := suite.defaultBackendParams()
		params.State = state

		backend, err := New(params)
		suite.Require().Error(err)
		suite.Require().Nil(backend)
	})

	suite.Run("backend construct fails when SealedRoot lookup fails", func() {
		stateParams := protocol.NewParams(suite.T())
		stateParams.On("SporkID").Return(sporkID, nil)
		stateParams.On("ProtocolVersion").Return(protocolVersion, nil)
		stateParams.On("SporkRootBlockHeight").Return(sporkRootBlock.Height, nil)
		stateParams.On("SealedRoot").Return(nil, fmt.Errorf("fail"))

		state := protocol.NewState(suite.T())
		state.On("Params").Return(stateParams, nil).Maybe()

		params := suite.defaultBackendParams()
		params.State = state

		backend, err := New(params)
		suite.Require().Error(err)
		suite.Require().Nil(backend)
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

// TestExecutionNodesForBlockID tests the common method backend.executionNodesForBlockID used for serving all API calls
// that need to talk to an execution node.
func (suite *Suite) TestExecutionNodesForBlockID() {

	totalReceipts := 5

	block := unittest.BlockFixture()

	// generate one execution node identities for each receipt assuming that each ER is generated by a unique exec node
	allExecutionNodes := unittest.IdentityListFixture(totalReceipts, unittest.WithRole(flow.RoleExecution))

	// one execution result for all receipts for this block
	executionResult := unittest.ExecutionResultFixture()

	// generate execution receipts
	receipts := make(flow.ExecutionReceiptList, totalReceipts)
	for j := 0; j < totalReceipts; j++ {
		r := unittest.ReceiptForBlockFixture(&block)
		r.ExecutorID = allExecutionNodes[j].NodeID
		er := *executionResult
		r.ExecutionResult = er
		receipts[j] = r
	}

	currentAttempt := 0
	attempt1Receipts, attempt2Receipts, attempt3Receipts := receipts, receipts, receipts

	// setup receipts storage mock to return different list of receipts on each call
	suite.receipts.
		On("ByBlockID", block.ID()).Return(
		func(id flow.Identifier) flow.ExecutionReceiptList {
			switch currentAttempt {
			case 0:
				currentAttempt++
				return attempt1Receipts
			case 1:
				currentAttempt++
				return attempt2Receipts
			default:
				currentAttempt = 0
				return attempt3Receipts
			}
		},
		func(id flow.Identifier) error { return nil })

	suite.snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter) flow.IdentityList {
			// apply the filter passed in to the list of all the execution nodes
			return allExecutionNodes.Filter(filter)
		},
		func(flow.IdentityFilter) error { return nil })
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	testExecutionNodesForBlockID := func(preferredENs, fixedENs, expectedENs flow.IdentityList) {

		if preferredENs != nil {
			preferredENIdentifiers = preferredENs.NodeIDs()
		}
		if fixedENs != nil {
			fixedENIdentifiers = fixedENs.NodeIDs()
		}

		if expectedENs == nil {
			expectedENs = flow.IdentityList{}
		}

		allExecNodes, err := executionNodesForBlockID(context.Background(), block.ID(), suite.receipts, suite.state, suite.log)
		require.NoError(suite.T(), err)

		execNodeSelectorFactory := NodeSelectorFactory{circuitBreakerEnabled: false}
		execSelector, err := execNodeSelectorFactory.SelectNodes(allExecNodes)
		require.NoError(suite.T(), err)

		actualList := flow.IdentityList{}
		for actual := execSelector.Next(); actual != nil; actual = execSelector.Next() {
			actualList = append(actualList, actual)
		}

		if len(expectedENs) > maxNodesCnt {
			for _, actual := range actualList {
				require.Contains(suite.T(), expectedENs, actual)
			}
		} else {
			require.ElementsMatch(suite.T(), actualList, expectedENs)
		}
	}
	// if we don't find sufficient receipts, executionNodesForBlockID should return a list of random ENs
	suite.Run("insufficient receipts return random ENs in State", func() {
		// return no receipts at all attempts
		attempt1Receipts = flow.ExecutionReceiptList{}
		attempt2Receipts = flow.ExecutionReceiptList{}
		attempt3Receipts = flow.ExecutionReceiptList{}
		suite.state.On("AtBlockID", mock.Anything).Return(suite.snapshot)

		allExecNodes, err := executionNodesForBlockID(context.Background(), block.ID(), suite.receipts, suite.state, suite.log)
		require.NoError(suite.T(), err)

		execNodeSelectorFactory := NodeSelectorFactory{circuitBreakerEnabled: false}
		execSelector, err := execNodeSelectorFactory.SelectNodes(allExecNodes)
		require.NoError(suite.T(), err)

		actualList := flow.IdentityList{}
		for actual := execSelector.Next(); actual != nil; actual = execSelector.Next() {
			actualList = append(actualList, actual)
		}

		require.Equal(suite.T(), len(actualList), maxNodesCnt)
	})

	// if no preferred or fixed ENs are specified, the ExecutionNodesForBlockID function should
	// return the exe node list without a filter
	suite.Run("no preferred or fixed ENs", func() {
		testExecutionNodesForBlockID(nil, nil, allExecutionNodes)
	})
	// if only preferred ENs are specified, the ExecutionNodesForBlockID function should
	// return the preferred ENs list
	suite.Run("two preferred ENs with zero fixed EN", func() {
		// mark the first two ENs as preferred
		preferredENs := allExecutionNodes[0:2]
		expectedList := preferredENs
		testExecutionNodesForBlockID(preferredENs, nil, expectedList)
	})
	// if only fixed ENs are specified, the ExecutionNodesForBlockID function should
	// return the fixed ENs list
	suite.Run("two fixed ENs with zero preferred EN", func() {
		// mark the first two ENs as fixed
		fixedENs := allExecutionNodes[0:2]
		expectedList := fixedENs
		testExecutionNodesForBlockID(nil, fixedENs, expectedList)
	})
	// if both are specified, the ExecutionNodesForBlockID function should
	// return the preferred ENs list
	suite.Run("four fixed ENs of which two are preferred ENs", func() {
		// mark the first four ENs as fixed
		fixedENs := allExecutionNodes[0:5]
		// mark the first two of the fixed ENs as preferred ENs
		preferredENs := fixedENs[0:2]
		expectedList := preferredENs
		testExecutionNodesForBlockID(preferredENs, fixedENs, expectedList)
	})
	// if both are specified, but the preferred ENs don't match the ExecutorIDs in the ER,
	// the ExecutionNodesForBlockID function should return the fixed ENs list
	suite.Run("four fixed ENs of which two are preferred ENs but have not generated the ER", func() {
		// mark the first two ENs as fixed
		fixedENs := allExecutionNodes[0:2]
		// specify two ENs not specified in the ERs as preferred
		preferredENs := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
		expectedList := fixedENs
		testExecutionNodesForBlockID(preferredENs, fixedENs, expectedList)
	})
	// if execution receipts are not yet available, the ExecutionNodesForBlockID function should retry twice
	suite.Run("retry execution receipt query", func() {
		// on first attempt, no execution receipts are available
		attempt1Receipts = flow.ExecutionReceiptList{}
		// on second attempt ony one is available
		attempt2Receipts = flow.ExecutionReceiptList{receipts[0]}
		// on third attempt all receipts are available
		attempt3Receipts = receipts
		currentAttempt = 0
		// mark the first two ENs as preferred
		preferredENs := allExecutionNodes[0:2]
		expectedList := preferredENs
		testExecutionNodesForBlockID(preferredENs, nil, expectedList)
	})
}

// TestGetTransactionResultEventEncodingVersion tests the GetTransactionResult function with different event encoding versions.
func (suite *Suite) TestGetTransactionResultEventEncodingVersion() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()

	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	// block which will eventually contain the transaction
	block := unittest.BlockFixture()
	block.SetPayload(
		unittest.PayloadFixture(
			unittest.WithGuarantees(
				unittest.CollectionGuaranteesWithCollectionIDFixture([]*flow.Collection{&collection})...)))
	blockId := block.ID()

	// reference block to which the transaction points to
	refBlock := unittest.BlockFixture()
	refBlockID := refBlock.ID()
	refBlock.Header.Height = 2
	transactionBody.SetReferenceBlockID(refBlockID)
	txId := transactionBody.ID()

	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", txId).
		Return(transactionBody, nil)

	light := collection.Light()
	suite.collections.On("LightByID", mock.Anything).Return(&light, nil)

	suite.snapshot.On("Head").Return(block.Header, nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil)

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = (fixedENIDs.NodeIDs()).Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	exeNodeEventEncodingVersion := entitiesproto.EventEncodingVersion_CCF_V0
	events := generator.GetEventsWithEncoding(1, exeNodeEventEncodingVersion)
	eventMessages := convert.EventsToMessages(events)

	for _, version := range eventEncodingVersions {
		suite.Run(fmt.Sprintf("test %s event encoding version for GetTransactionResult", version.String()), func() {
			exeEventResp := &execproto.GetTransactionResultResponse{
				Events:               eventMessages,
				EventEncodingVersion: exeNodeEventEncodingVersion,
			}

			suite.execClient.
				On("GetTransactionResult", ctx, &execproto.GetTransactionResultRequest{
					BlockId:       blockId[:],
					TransactionId: txId[:],
				}).
				Return(exeEventResp, nil).
				Once()

			result, err := backend.GetTransactionResult(ctx, txId, blockId, flow.ZeroID, version)
			expectedResult := generator.GetEventsWithEncoding(1, version)
			suite.checkResponse(result, err)

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

	suite.snapshot.On("Head").Return(block.Header, nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil)

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = (fixedENIDs.NodeIDs()).Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	exeNodeEventEncodingVersion := entitiesproto.EventEncodingVersion_CCF_V0
	events := generator.GetEventsWithEncoding(1, exeNodeEventEncodingVersion)
	eventMessages := convert.EventsToMessages(events)

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
			suite.checkResponse(result, err)

			expectedResult := generator.GetEventsWithEncoding(1, version)
			suite.Assert().Equal(result.Events, expectedResult)
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
			suite.checkResponse(results, err)

			expectedResult := generator.GetEventsWithEncoding(1, version)
			for _, result := range results {
				suite.Assert().Equal(result.Events, expectedResult)
			}
		})
	}
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

func (suite *Suite) checkResponse(resp interface{}, err error) {
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
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
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)
	return connFactory
}

func getEvents(n int) []flow.Event {
	events := make([]flow.Event, n)
	for i := range events {
		events[i] = flow.Event{Type: flow.EventAccountCreated}
	}
	return events
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
		MaxHeightRange:       DefaultMaxHeightRange,
		SnapshotHistoryLimit: DefaultSnapshotHistoryLimit,
		Communicator:         NewNodeCommunicator(false),
		AccessMetrics:        metrics.NewNoopCollector(),
		Log:                  suite.log,
	}
}

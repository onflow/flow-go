package backend

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

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

	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"

	access "github.com/onflow/flow-go/engine/access/mock"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks                 *storagemock.Blocks
	headers                *storagemock.Headers
	collections            *storagemock.Collections
	transactions           *storagemock.Transactions
	receipts               *storagemock.ExecutionReceipts
	results                *storagemock.ExecutionResults
	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	connectionFactory      *backendmock.ConnectionFactory
	chainID                flow.ChainID
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()
	params := new(protocol.Params)
	params.On("Root").Return(&header, nil)
	suite.state.On("Params").Return(params).Maybe()
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)
	suite.results = new(storagemock.ExecutionResults)
	suite.colClient = new(access.AccessAPIClient)
	suite.execClient = new(access.ExecutionAPIClient)
	suite.chainID = flow.Testnet
	suite.historicalAccessClient = new(access.AccessAPIClient)
	suite.connectionFactory = new(backendmock.ConnectionFactory)
}

func (suite *Suite) TestPing() {
	suite.colClient.
		On("Ping", mock.Anything, &accessproto.PingRequest{}).
		Return(&accessproto.PingResponse{}, nil)

	suite.execClient.
		On("Ping", mock.Anything, &execproto.PingRequest{}).
		Return(&execproto.PingResponse{}, nil)

	backend := New(
		suite.state,
		suite.colClient,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	err := backend.Ping(context.Background())

	suite.Require().NoError(err)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// query the handler for the latest finalized block
	header, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.checkResponse(header, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

	suite.assertAllExpectations()

}

// TestGetLatestProtocolStateSnapshot_NoTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the state requested at latest finalized  block does not contain any blocks that
// spans an epoch or epoch phase transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_NoTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
		// build epoch 1
		// blocks in current state
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit| <- G(S_E)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID mock returns for state
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height)).Once()
		}

		// Take snapshot at height of block D (epoch1.heights[3]) for valid segment and valid snapshot
		// where it's sealing segment is B <- C <- D
		snap := state.AtHeight(epoch1.Range()[3])
		suite.state.On("Final").Return(snap).Once()

		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().NoError(err)

		// we expect the endpoint to return the snapshot at the same height we requested
		// because it has a valid sealing segment with no blocks spanning an epoch or phase transition
		expectedSnapshotBytes, err := convert.SnapshotToBytes(snap)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_TransitionSpans tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the state requested for latest finalized block  contains a block that
// spans an epoch transition and blocks that span epoch phase transitions.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_TransitionSpans() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
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

		// setup AtHeight mock returns for state
		for _, height := range append(epoch1.Range(), epoch2.Range()...) {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at height of the first block of epoch2, the sealing segment of this snapshot
		// will have contain block spanning an epoch transition as well as an epoch phase transition.
		// This will cause our GetLatestProtocolStateSnapshot func to return a snapshot
		// at block with height 3, the first block of the staking phase of epoch1.

		snap := state.AtHeight(epoch2.Range()[0])
		suite.state.On("Final").Return(snap).Once()

		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().NoError(err)
		fmt.Println()

		// we expect the endpoint to return last valid snapshot which is the snapshot at block D (height 3)
		expectedSnapshotBytes, err := convert.SnapshotToBytes(state.AtHeight(epoch1.Range()[3]))
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_PhaseTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the state requested at latest finalized  block contains a blocks that
// spans an epoch phase transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_PhaseTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
		// build epoch 1
		// blocks in current state
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit| <- G(S_E)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID mock returns for state
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at height of block E (epoch1.heights[4]) the sealing segment for this snapshot
		// is C(S_A) <- D(S_B) |setup| <- E(S_C) which spans the epoch setup phase. This will force
		// our RPC endpoint to return a snapshot at block D which is the snapshot at the boundary where the phase
		// transition happens.
		snap := state.AtHeight(epoch1.Range()[4])
		suite.state.On("Final").Return(snap).Once()

		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// query the handler for the latest finalized snapshot
		bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().NoError(err)

		// we expect the endpoint to return last valid snapshot which is the snapshot at block D (height 3)
		expectedSnapshotBytes, err := convert.SnapshotToBytes(state.AtHeight(epoch1.Range()[3]))
		suite.Require().NoError(err)
		suite.Require().Equal(expectedSnapshotBytes, bytes)
	})
}

// TestGetLatestProtocolStateSnapshot_EpochTransitionSpan tests our GetLatestProtocolStateSnapshot RPC endpoint
// where the sealing segment for the state requested at latest finalized  block contains a blocks that
// spans an epoch transition.
func (suite *Suite) TestGetLatestProtocolStateSnapshot_EpochTransitionSpan() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)
		// build epoch 1
		// blocks in current state
		// P <- A(S_P-1) <- B(S_P) <- C(S_A) <- D(S_B) |setup| <- E(S_C) <- F(S_D) |commit| <- G(S_E)
		epochBuilder.BuildEpoch()

		// add more blocks to our state in the commit phase, this will allow
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

		// setup AtHeight mock returns for state
		for _, height := range append(epoch1.Range(), epoch2.Range()...) {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at the first block of epoch2 . The sealing segment
		// for this snapshot contains a block (highest) that spans the epoch1 -> epoch2
		// transition.
		snap := state.AtHeight(epoch2.Range()[0])
		suite.state.On("Final").Return(snap).Once()

		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

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
// where the length of the sealing segment is greater than the configured snapshotHistoryLimit
func (suite *Suite) TestGetLatestProtocolStateSnapshot_HistoryLimit() {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state).BuildEpoch().CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtBlockID mock returns for state
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", height).Return(state.AtHeight(height))
		}

		// Take snapshot at height of block E (epoch1.heights[4]) the sealing segment for this snapshot
		// is C(S_A) <- D(S_B) |setup| <- E(S_C) which spans the epoch setup phase. This will force
		// our RPC endpoint to return a snapshot at block D which is the snapshot at the boundary where a phase
		// transition happens.
		snap := state.AtHeight(epoch1.Range()[4])
		suite.state.On("Final").Return(snap).Once()

		// very short history limit, any segment with any blocks spanning any transition should force the endpoint to return a history limit error
		snapshotHistoryLimit := 1
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			DefaultMaxHeightRange,
			nil,
			nil,
			suite.log,
			snapshotHistoryLimit,
		)

		// the handler should return a snapshot history limit error
		_, err := backend.GetLatestProtocolStateSnapshot(context.Background())
		suite.Require().ErrorIs(err, SnapshotHistoryLimitErr)
	})
}

func (suite *Suite) TestGetLatestSealedBlockHeader() {
	// setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// query the handler for the latest sealed block
	header, err := backend.GetLatestBlockHeader(context.Background(), true)
	suite.checkResponse(header, err)

	// make sure we got the latest sealed block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

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

	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.transactions,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

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

	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		nil,
		suite.collections,
		suite.transactions,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	actual, err := backend.GetCollectionByID(context.Background(), expected.ID())
	suite.transactions.AssertExpectations(suite.T())
	suite.checkResponse(actual, err)

	suite.Equal(expected, *actual)
	suite.assertAllExpectations()
}

// TestTransactionStatusTransition tests that the status of transaction changes from Finalized to Sealed
// when the protocol state is updated
func (suite *Suite) TestTransactionStatusTransition() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	block.Header.Height = 2
	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = block.Header.Height - 1 // head is behind the current block

	suite.snapshot.
		On("Head").
		Return(headBlock.Header, nil)

	light := collection.Light()

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
	connFactory := new(backendmock.ConnectionFactory)
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	exeEventReq := execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}

	exeEventResp := execproto.GetTransactionResultResponse{
		Events: nil,
	}

	backend := New(
		suite.state,
		nil,
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.receipts,
		suite.results,
		suite.chainID,
		metrics.NewNoopCollector(),
		connFactory, // the connection factory should be used to get the execution node client
		false,
		DefaultMaxHeightRange,
		nil,
		flow.IdentifierList(fixedENIDs.NodeIDs()).Strings(),
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// Successfully return empty event list
	suite.execClient.
		On("GetTransactionResult", ctx, &exeEventReq).
		Return(&exeEventResp, status.Errorf(codes.NotFound, "not found")).
		Once()

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, err := backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	// status should be finalized since the sealed blocks is smaller in height
	suite.Assert().Equal(flow.TransactionStatusFinalized, result.Status)

	// block ID should be included in the response
	suite.Assert().Equal(blockID, result.BlockID)

	// Successfully return empty event list from here on
	suite.execClient.
		On("GetTransactionResult", ctx, &exeEventReq).
		Return(&exeEventResp, nil)

	// second call - when block under test's height is greater height than the sealed head
	result, err = backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	// status should be executed since no `NotFound` error in the `GetTransactionResult` call
	suite.Assert().Equal(flow.TransactionStatusExecuted, result.Status)

	// now let the head block be finalized
	headBlock.Header.Height = block.Header.Height + 1

	// third call - when block under test's height is less than sealed head's height
	result, err = backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	// status should be sealed since the sealed blocks is greater in height
	suite.Assert().Equal(flow.TransactionStatusSealed, result.Status)

	// now go far into the future
	headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1

	// fourth call - when block under test's height so much less than the head's height that it's considered expired,
	// but since there is a execution result, means it should retain it's sealed status
	result, err = backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	// status should be expired since
	suite.Assert().Equal(flow.TransactionStatusSealed, result.Status)

	suite.assertAllExpectations()
}

// TestTransactionExpiredStatusTransition tests that the status
// of transaction changes from Pending to Expired when enough blocks pass
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

	backend := New(
		suite.state,
		nil,
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// should return pending status when we have not observed an expiry block
	suite.Run("pending", func() {
		// referenced block isn't known yet, so should return pending status
		result, err := backend.GetTransactionResult(ctx, txID)
		suite.checkResponse(result, err)

		suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
	})

	// should return pending status when we have observed an expiry block but
	// have not observed all intermediary collections
	suite.Run("expiry un-confirmed", func() {

		suite.Run("ONLY finalized expiry block", func() {
			// we have finalized an expiry block
			headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1
			// we have NOT observed all intermediary collections
			fullHeight = block.Header.Height + flow.DefaultTransactionExpiry/2

			result, err := backend.GetTransactionResult(ctx, txID)
			suite.checkResponse(result, err)
			suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		})
		suite.Run("ONLY observed intermediary collections", func() {
			// we have NOT finalized an expiry block
			headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry/2
			// we have observed all intermediary collections
			fullHeight = block.Header.Height + flow.DefaultTransactionExpiry + 1

			result, err := backend.GetTransactionResult(ctx, txID)
			suite.checkResponse(result, err)
			suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		})

	})

	// should return expired status only when we have observed an expiry block
	// and have observed all intermediary collections
	suite.Run("expired", func() {
		// we have finalized an expiry block
		headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1
		// we have observed all intermediary collections
		fullHeight = block.Header.Height + flow.DefaultTransactionExpiry + 1

		result, err := backend.GetTransactionResult(ctx, txID)
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

	currentState := flow.TransactionStatusPending // marker for the current state
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

	// refBlock storage returns the corresponding refBlock
	suite.blocks.
		On("ByCollectionID", collection.ID()).
		Return(&block, nil)

	receipts, _ := suite.setupReceipts(&block)

	exeEventReq := execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}

	exeEventResp := execproto.GetTransactionResultResponse{
		Events: nil,
	}

	// simulate that the execution node has not yet executed the transaction
	suite.execClient.
		On("GetTransactionResult", ctx, &exeEventReq).
		Return(&exeEventResp, status.Errorf(codes.NotFound, "not found")).
		Once()

	// create a mock connection factory
	connFactory := suite.setupConnectionFactory()

	backend := New(
		suite.state,
		nil,
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.receipts,
		suite.results,
		suite.chainID,
		metrics.NewNoopCollector(),
		connFactory, // the connection factory should be used to get the execution node client
		false,
		100,
		nil,
		flow.IdentifierList(enIDs.NodeIDs()).Strings(),
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	preferredENIdentifiers = flow.IdentifierList{receipts[0].ExecutorID}

	// should return pending status when we have not observed collection for the transaction
	suite.Run("pending", func() {
		currentState = flow.TransactionStatusPending
		result, err := backend.GetTransactionResult(ctx, txID)
		suite.checkResponse(result, err)
		suite.Assert().Equal(flow.TransactionStatusPending, result.Status)
		// assert that no call to an execution node is made
		suite.execClient.AssertNotCalled(suite.T(), "GetTransactionResult", mock.Anything, mock.Anything)
	})

	// should return finalized status when we have have observed collection for the transaction (after observing the
	// a preceding sealed refBlock)
	suite.Run("finalized", func() {
		currentState = flow.TransactionStatusFinalized
		result, err := backend.GetTransactionResult(ctx, txID)
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

	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.transactions,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, err := backend.GetTransactionResult(ctx, txID)
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
		Return(header, nil).
		Once()

	suite.blocks.
		On("ByID", header.ID()).
		Return(&expected, nil).
		Once()

	backend := New(
		suite.state,
		nil,
		nil,
		suite.blocks,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// query the handler for the latest finalized header
	actual, err := backend.GetLatestBlock(context.Background(), false)
	suite.checkResponse(actual, err)

	// make sure we got the latest header
	suite.Require().Equal(expected, *actual)

	suite.assertAllExpectations()
}

type mockCloser struct{}

func (mc *mockCloser) Close() error { return nil }

func (suite *Suite) TestGetEventsForBlockIDs() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	events := getEvents(10)
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
	connFactory := new(backendmock.ConnectionFactory)
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
	for i := 0; i < len(blockHeaders); i++ {
		expected[i] = flow.BlockEvents{
			BlockID:        blockHeaders[i].ID(),
			BlockHeight:    blockHeaders[i].Height,
			BlockTimestamp: blockHeaders[i].Timestamp,
			Events:         events,
		}
	}

	// create the execution node response
	exeResp := execproto.GetEventsForBlockIDsResponse{
		Results: exeResults,
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
		Return(&exeResp, nil).
		Once()

	suite.Run("with an execution node chosen using block ID form the list of Fixed ENs", func() {

		// create the handler
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			suite.results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			validENIDs.Strings(), // set the fixed EN Identifiers to the generated execution IDs
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// execute request
		actual, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), blockIDs)
		suite.checkResponse(actual, err)

		suite.Require().Equal(expected, actual)
	})

	suite.Run("with an empty block ID list", func() {

		// create the handler
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			suite.headers,
			nil,
			nil,
			receipts,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			validENIDs.Strings(), // set the fixed EN Identifiers to the generated execution IDs
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// execute request with an empty block id list and expect an empty list of events and no error
		resp, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), []flow.Identifier{})
		require.NoError(suite.T(), err)
		require.Empty(suite.T(), resp)
	})

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetExecutionResultByID() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	validExecutorIdentities := flow.IdentityList{}
	validENIDs := flow.IdentifierList(validExecutorIdentities.NodeIDs())

	// create a mock connection factory
	connFactory := new(backendmock.ConnectionFactory)

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

		// create the handler
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			validENIDs.Strings(), // set the fixed EN Identifiers to the generated execution IDs
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// execute request
		_, err := backend.GetExecutionResultByID(ctx, nonexistingID)

		assert.Error(suite.T(), err)
	})

	suite.Run("existing execution result id", func() {
		// create the handler
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			suite.headers,
			nil,
			nil,
			nil,
			results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			validENIDs.Strings(), // set the fixed EN Identifiers to the generated execution IDs
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

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
	connFactory := new(backendmock.ConnectionFactory)

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

		// create the handler
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			validENIDs.Strings(), // set the fixed EN Identifiers to the generated execution IDs
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// execute request
		_, err := backend.GetExecutionResultForBlockID(ctx, nonexistingBlockID)

		assert.Error(suite.T(), err)
	})

	suite.Run("existing execution results", func() {

		// create the handler
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			suite.headers,
			nil,
			nil,
			nil,
			results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			validENIDs.Strings(), // set the fixed EN Identifiers to the generated execution IDs
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

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
	params := new(protocol.Params)
	params.On("Root").Return(&rootHeader, nil)
	state.On("Params").Return(params).Maybe()

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

	// mock headers to pull from headers backend
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
		head = &header
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

		for i, header := range blockHeaders {
			events := getEvents(1)
			height := header.Height

			results[i] = flow.BlockEvents{
				BlockID:        header.ID(),
				BlockHeight:    height,
				BlockTimestamp: header.Timestamp,
				Events:         events,
			}

			exeResults[i] = &execproto.GetEventsForBlockIDsResponse_Result{
				BlockId:     convert.IdentifierToMessage(header.ID()),
				BlockHeight: header.Height,
				Events:      convert.EventsToMessages(events),
			}
		}

		exeResp := &execproto.GetEventsForBlockIDsResponse{
			Results: exeResults,
		}

		suite.execClient.
			On("GetEventsForBlockIDs", ctx, execReq).
			Return(exeResp, nil).
			Once()

		return results
	}

	connFactory := suite.setupConnectionFactory()

	suite.Run("invalid request max height < min height", func() {
		backend := New(
			suite.state,
			nil,
			nil,
			nil,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			suite.results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		_, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), maxHeight, minHeight)
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

		// create handler
		backend := New(
			state,
			nil,
			nil,
			suite.blocks,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			suite.results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			fixedENIdentifiersStr,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		// execute request
		actualResp, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight)

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

		backend := New(
			state,
			nil,
			nil,
			suite.blocks,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			suite.results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			fixedENIdentifiersStr,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		actualResp, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight)
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

		// create handler
		backend := New(
			state,
			nil,
			nil,
			suite.blocks,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			suite.results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			1, // set maximum range to 1
			nil,
			fixedENIdentifiersStr,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		_, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, minHeight+1)
		suite.Require().Error(err)
	})

	suite.Run("invalid request last_sealed_block_height < min height", func() {

		// set sealed height to one less than the request start height
		headHeight = minHeight - 1

		// setup mocks
		setupHeadHeight(headHeight)
		blockHeaders, _, nodeIdentities = setupStorage(minHeight, maxHeight)
		fixedENIdentifiersStr := flow.IdentifierList(nodeIdentities.NodeIDs()).Strings()

		// create handler
		backend := New(
			state,
			nil,
			nil,
			suite.blocks,
			suite.headers,
			nil,
			nil,
			suite.receipts,
			suite.results,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			DefaultMaxHeightRange,
			nil,
			fixedENIdentifiersStr,
			suite.log,
			DefaultSnapshotHistoryLimit,
		)

		_, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight)
		suite.Require().Error(err)
	})

}

func (suite *Suite) TestGetAccount() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	address, err := suite.chainID.Chain().NewAddressGenerator().NextAddress()
	suite.Require().NoError(err)

	account := &entitiesproto.Account{
		Address: address.Bytes(),
	}
	ctx := context.Background()

	// setup the latest sealed block
	block := unittest.BlockFixture()
	header := block.Header          // create a mock header
	seal := unittest.Seal.Fixture() // create a mock seal
	seal.BlockID = header.ID()      // make the seal point to the header

	suite.snapshot.
		On("Head").
		Return(header, nil).
		Once()

	// create the expected execution API request
	blockID := header.ID()
	exeReq := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: address.Bytes(),
	}

	// create the expected execution API response
	exeResp := &execproto.GetAccountAtBlockIDResponse{
		Account: account,
	}

	// setup the execution client mock
	suite.execClient.
		On("GetAccountAtBlockID", ctx, exeReq).
		Return(exeResp, nil).
		Once()

	receipts, ids := suite.setupReceipts(&block)

	suite.snapshot.On("Identities", mock.Anything).Return(ids, nil)
	// create a mock connection factory
	connFactory := new(backendmock.ConnectionFactory)
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// create the handler with the mock
	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		suite.headers,
		nil,
		nil,
		suite.receipts,
		suite.results,
		suite.chainID,
		metrics.NewNoopCollector(),
		connFactory, // the connection factory should be used to get the execution node client
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	preferredENIdentifiers = flow.IdentifierList{receipts[0].ExecutorID}

	suite.Run("happy path - valid request and valid response", func() {
		account, err := backend.GetAccountAtLatestBlock(ctx, address)
		suite.checkResponse(account, err)

		suite.Require().Equal(address, account.Address)

		suite.assertAllExpectations()
	})
}

func (suite *Suite) TestGetAccountAtBlockHeight() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	height := uint64(5)
	address := unittest.AddressFixture()
	account := &entitiesproto.Account{
		Address: address.Bytes(),
	}
	ctx := context.Background()

	// create a mock block header
	b := unittest.BlockFixture()
	h := b.Header

	// setup headers storage to return the header when queried by height
	suite.headers.
		On("ByHeight", height).
		Return(h, nil).
		Once()

	receipts, ids := suite.setupReceipts(&b)
	suite.snapshot.On("Identities", mock.Anything).Return(ids, nil)

	// create a mock connection factory
	connFactory := new(backendmock.ConnectionFactory)
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// create the expected execution API request
	blockID := h.ID()
	exeReq := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: address.Bytes(),
	}

	// create the expected execution API response
	exeResp := &execproto.GetAccountAtBlockIDResponse{
		Account: account,
	}

	// setup the execution client mock
	suite.execClient.
		On("GetAccountAtBlockID", ctx, exeReq).
		Return(exeResp, nil).
		Once()

	// create the handler with the mock
	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		suite.headers,
		nil,
		nil,
		suite.receipts,
		suite.results,
		flow.Testnet,
		metrics.NewNoopCollector(),
		connFactory, // the connection factory should be used to get the execution node client
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	preferredENIdentifiers = flow.IdentifierList{receipts[0].ExecutorID}

	suite.Run("happy path - valid request and valid response", func() {
		account, err := backend.GetAccountAtBlockHeight(ctx, address, height)
		suite.checkResponse(account, err)

		suite.Require().Equal(address, account.Address)

		suite.assertAllExpectations()
	})
}

func (suite *Suite) TestGetNetworkParameters() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	expectedChainID := flow.Mainnet

	backend := New(nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		flow.Mainnet,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	params := backend.GetNetworkParameters(context.Background())

	suite.Require().Equal(expectedChainID, params.ChainID)
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
		actualList, err := executionNodesForBlockID(context.Background(), block.ID(), suite.receipts, suite.state, suite.log)
		require.NoError(suite.T(), err)
		if expectedENs == nil {
			expectedENs = flow.IdentityList{}
		}
		if len(expectedENs) > maxExecutionNodesCnt {
			for _, actual := range actualList {
				require.Contains(suite.T(), expectedENs, actual)
			}
		} else {
			require.ElementsMatch(suite.T(), actualList, expectedENs)
		}
	}
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

// TestExecuteScriptOnExecutionNode tests the method backend.scripts.executeScriptOnExecutionNode for script execution
func (suite *Suite) TestExecuteScriptOnExecutionNode() {

	// create a mock connection factory
	connFactory := new(backendmock.ConnectionFactory)
	connFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(suite.execClient, &mockCloser{}, nil)

	// create the handler with the mock
	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		suite.headers,
		nil,
		nil,
		suite.receipts,
		suite.results,
		flow.Mainnet,
		metrics.NewNoopCollector(),
		connFactory, // the connection factory should be used to get the execution node client
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// mock parameters
	ctx := context.Background()
	block := unittest.BlockFixture()
	blockID := block.ID()
	script := []byte("dummy script")
	arguments := [][]byte(nil)
	executionNode := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	execReq := execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    script,
		Arguments: arguments,
	}
	execRes := execproto.ExecuteScriptAtBlockIDResponse{
		Value: []byte{4, 5, 6},
	}

	suite.Run("happy path script execution success", func() {
		suite.execClient.On("ExecuteScriptAtBlockID", ctx, &execReq).Return(&execRes, nil).Once()
		res, err := backend.tryExecuteScript(ctx, executionNode, execReq)
		suite.execClient.AssertExpectations(suite.T())
		suite.checkResponse(res, err)
	})

	suite.Run("script execution failure returns status OK", func() {
		suite.execClient.On("ExecuteScriptAtBlockID", ctx, &execReq).
			Return(nil, status.Error(codes.InvalidArgument, "execution failure!")).Once()
		_, err := backend.tryExecuteScript(ctx, executionNode, execReq)
		suite.execClient.AssertExpectations(suite.T())
		suite.Require().Error(err)
		suite.Require().Equal(status.Code(err), codes.InvalidArgument)
	})

	suite.Run("execution node internal failure returns status code Internal", func() {
		suite.execClient.On("ExecuteScriptAtBlockID", ctx, &execReq).
			Return(nil, status.Error(codes.Internal, "execution node internal error!")).Once()
		_, err := backend.tryExecuteScript(ctx, executionNode, execReq)
		suite.execClient.AssertExpectations(suite.T())
		suite.Require().Error(err)
		suite.Require().Equal(status.Code(err), codes.Internal)
	})
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

func (suite *Suite) setupConnectionFactory() ConnectionFactory {
	// create a mock connection factory
	connFactory := new(backendmock.ConnectionFactory)
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

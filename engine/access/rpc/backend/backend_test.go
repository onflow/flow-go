package backend

import (
	"context"
	"math/rand"
	"testing"
	"time"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)
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
		suite.execClient,
		suite.colClient,
		nil, nil, nil, nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
	)

	err := backend.Ping(context.Background())

	suite.Require().NoError(err)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()

	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(
		suite.state,
		suite.execClient,
		nil, nil, nil, nil, nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
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

func (suite *Suite) TestGetLatestProtocolStateSnapshot() {
	// setup the snapshot mock
	snap := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	suite.state.On("Sealed").Return(snap).Once()

	backend := New(
		suite.state,
		nil, nil, nil, nil, nil,
		nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
	)

	// query the handler for the latest sealed snapshot
	bytes, err := backend.GetLatestProtocolStateSnapshot(context.Background())
	suite.Require().NoError(err)

	// make sure the returned bytes is equal to the serialized snapshot
	convertedSnapshot, err := convert.SnapshotToBytes(snap)
	suite.Require().NoError(err)
	suite.Require().Equal(bytes, convertedSnapshot)

	// suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestSealedBlockHeader() {
	// setup the mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(
		suite.state,
		nil, nil, nil, nil, nil,
		nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
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
		nil, nil, nil, nil, nil, nil,
		suite.transactions,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
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
		nil, nil, nil, nil, nil,
		suite.collections,
		suite.transactions,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
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
	fixedENIDs := flow.IdentifierList{}

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
	receipts := suite.setupReceipts(&block)
	fixedENIDs = append(fixedENIDs, receipts[0].ExecutorID)

	suite.setupReceipts(&block)

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
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.receipts,
		suite.chainID,
		metrics.NewNoopCollector(),
		connFactory,
		false,
		100,
		nil,
		nil,
		suite.log,
	)

	preferredENIdentifiers = fixedENIDs

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

// TestTransactionPendingToExpiredStatusTransition tests that the status of transaction changes from Pending to Expired
// when enough blocks pass
func (suite *Suite) TestTransactionExpiredStatusTransition() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

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
		suite.execClient,
		nil,
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
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

	suite.state.
		On("AtBlockID", refBlockID).
		Return(snapshotAtBlock, nil)

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

	ids := unittest.IdentityListFixture(2)
	receipt1 := unittest.ReceiptForBlockFixture(&block)
	receipt1.ExecutorID = ids[0].NodeID
	receipt2 := unittest.ReceiptForBlockFixture(&block)
	receipt2.ExecutorID = ids[1].NodeID
	receipt1.ExecutionResult = receipt2.ExecutionResult
	suite.receipts.
		On("ByBlockID", blockID).
		Return(flow.ExecutionReceiptList{receipt1, receipt2}, nil).Once()

	suite.snapshot.On("Identities", mock.Anything).Return(ids, nil)

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
	connFactory := new(backendmock.ConnectionFactory)
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	backend := New(
		suite.state,
		suite.execClient,
		nil,
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.receipts,
		suite.chainID,
		metrics.NewNoopCollector(),
		connFactory,
		false,
		100,
		nil,
		nil,
		suite.log,
	)

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
		nil,
		suite.transactions,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
	)

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, err := backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	// status should be reported as unknown
	suite.Assert().Equal(flow.TransactionStatusUnknown, result.Status)
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

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
		nil, nil, nil,
		suite.blocks,
		nil, nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
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

	events := getEvents(10)
	validExecutorIdentities := flow.IdentityList{}

	setupStorage := func(n int) []*flow.Header {
		headers := make([]*flow.Header, n)
		ids := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))

		for i := 0; i < n; i++ {
			b := unittest.BlockFixture()
			suite.blocks.
				On("ByID", b.ID()).
				Return(&b, nil).Twice()

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

	suite.snapshot.On("Identities", mock.Anything).Return(validExecutorIdentities, nil).Once()
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

	// expect two calls to the executor api client (one for each of the following 2 test cases)
	suite.execClient.
		On("GetEventsForBlockIDs", ctx, exeReq).
		Return(&exeResp, nil).
		Twice()

	suite.Run("with fixed execution node", func() {

		// create receipt mocks that always returns empty
		receipts := new(storagemock.ExecutionReceipts)
		receipts.
			On("ByBlockID", mock.Anything).
			Return(flow.ExecutionReceiptList{}, nil).Once()

		// create the handler
		backend := New(
			suite.state,
			suite.execClient, // pass the default client
			nil, nil,
			suite.blocks,
			nil, nil, nil,
			receipts,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
		)

		// execute request
		actual, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), blockIDs)
		suite.checkResponse(actual, err)

		suite.Require().Equal(expected, actual)
	})

	suite.Run("with an execution node chosen using block ID", func() {
		// create the handler
		backend := New(
			suite.state,
			nil,
			nil, nil,
			suite.blocks,
			nil, nil, nil,
			suite.receipts,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			100,
			nil,
			validENIDs.Strings(), // set the fixed EN Identifiers to the generated execution IDs
			suite.log,
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
			nil, // no default client, hence the receipts storage should be looked up
			nil, nil,
			suite.blocks,
			nil, nil, nil,
			suite.receipts,
			suite.chainID,
			metrics.NewNoopCollector(),
			connFactory, // the connection factory should be used to get the execution node client
			false,
			100,
			nil,
			validENIDs.Strings(),
			suite.log,
		)

		// execute request with an empty block id list and expect an error (not a panic)
		resp, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), []flow.Identifier{})
		require.NoError(suite.T(), err)
		require.Empty(suite.T(), resp)
	})

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetEventsForHeightRange() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

	ctx := context.Background()
	var minHeight uint64 = 5
	var maxHeight uint64 = 10
	var headHeight uint64
	var blockHeaders []*flow.Header

	setupHeadHeight := func(height uint64) {
		header := unittest.BlockHeaderFixture() // create a mock header
		header.Height = height                  // set the header height
		suite.snapshot.On("Head").Return(&header, nil).Once()
	}

	setupStorage := func(min uint64, max uint64) []*flow.Header {
		headers := make([]*flow.Header, 0)

		for i := min; i <= max; i++ {
			b := unittest.BlockFixture()

			suite.blocks.
				On("ByHeight", i).
				Return(&b, nil).Once()

			headers = append(headers, b.Header)
		}

		return headers
	}

	// use the static execution node
	suite.receipts.
		On("ByBlockID", mock.Anything).
		Return(flow.ExecutionReceiptList{}, nil)

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

	suite.Run("invalid request max height < min height", func() {
		backend := New(
			suite.state,
			nil, nil, nil, nil, nil, nil, nil,
			suite.receipts,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
		)

		_, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), maxHeight, minHeight)
		suite.Require().Error(err)

		suite.assertAllExpectations() // assert that request was not sent to execution node
	})

	suite.Run("valid request with min_height < max_height < last_sealed_block_height", func() {

		headHeight = maxHeight + 1

		// setup mocks
		setupHeadHeight(headHeight)
		blockHeaders = setupStorage(minHeight, maxHeight)
		expectedResp := setupExecClient()

		// create handler
		backend := New(
			suite.state,
			suite.execClient,
			nil, nil,
			suite.blocks,
			suite.headers,
			nil, nil,
			suite.receipts,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
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
		blockHeaders = setupStorage(minHeight, headHeight)
		expectedResp := setupExecClient()

		backend := New(
			suite.state,
			suite.execClient,
			nil, nil,
			suite.blocks,
			suite.headers,
			nil, nil,
			suite.receipts,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
		)

		actualResp, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight)
		suite.checkResponse(actualResp, err)

		suite.assertAllExpectations()
		suite.Require().Equal(expectedResp, actualResp)
	})

	suite.Run("invalid request last_sealed_block_height < min height", func() {

		// set sealed height to one less than the request start height
		headHeight = minHeight - 1

		// setup mocks
		setupHeadHeight(headHeight)
		blockHeaders = setupStorage(minHeight, maxHeight)

		// create handler
		backend := New(
			suite.state,
			suite.execClient,
			nil, nil,
			suite.blocks,
			suite.headers,
			nil, nil,
			suite.receipts,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			100,
			nil,
			nil,
			suite.log,
		)

		_, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight)
		suite.Require().Error(err)
	})

}

func (suite *Suite) TestGetAccount() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()

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

	receipts := suite.setupReceipts(&block)
	// create a mock connection factory
	connFactory := new(backendmock.ConnectionFactory)
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// create the handler with the mock
	backend := New(
		suite.state,
		nil,
		nil, nil, nil,
		suite.headers,
		nil, nil,
		suite.receipts,
		suite.chainID,
		metrics.NewNoopCollector(),
		connFactory,
		false,
		100,
		nil,
		nil,
		suite.log,
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

	height := uint64(5)
	address := unittest.AddressFixture()
	account := &entitiesproto.Account{
		Address: address.Bytes(),
	}
	ctx := context.Background()

	// create a mock block header
	h := unittest.BlockHeaderFixture()

	// setup headers storage to return the header when queried by height
	suite.headers.
		On("ByHeight", height).
		Return(&h, nil).
		Once()

	suite.receipts.
		On("ByBlockID", mock.Anything).
		Return(flow.ExecutionReceiptList{}, nil).Once()

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
		suite.execClient,
		nil, nil, nil,
		suite.headers,
		nil, nil,
		suite.receipts,
		flow.Testnet,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
	)

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

	backend := New(
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil,
		flow.Mainnet,
		metrics.NewNoopCollector(),
		nil,
		false,
		100,
		nil,
		nil,
		suite.log,
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
	suite.receipts.
		On("ByBlockID", block.ID()).
		Return(receipts, nil)

	suite.snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter) flow.IdentityList {
			// apply the filter passed in to the list of all the execution nodes
			return allExecutionNodes.Filter(filter)
		},
		func(flow.IdentityFilter) error { return nil })

	testExecutionNodesForBlockID := func(preferredENs, fixedENs, expectedENs flow.IdentityList) {
		if preferredENs != nil {
			preferredENIdentifiers = preferredENs.NodeIDs()
		}
		if fixedENs != nil {
			fixedENIdentifiers = fixedENs.NodeIDs()
		}
		actualList, err := executionNodesForBlockID(block.ID(), suite.receipts, suite.state, suite.log)
		require.NoError(suite.T(), err)
		if expectedENs == nil {
			expectedENs = flow.IdentityList{}
		}
		require.ElementsMatch(suite.T(), actualList, expectedENs)
	}
	// if no preferred or fixed ENs are specified, the ExecutionNodesForBlockID function should
	// return an empty list
	suite.Run("no preferred or fixed ENs", func() {
		testExecutionNodesForBlockID(nil, nil, nil)
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
	suite.Run("four fixed ENs of which two are preferred ENs", func() {
		// mark the first two ENs as fixed
		fixedENs := allExecutionNodes[0:2]
		// specify two ENs not specified in the ERs as preferred
		preferredENs := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
		expectedList := fixedENs
		testExecutionNodesForBlockID(preferredENs, fixedENs, expectedList)
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

func (suite *Suite) setupReceipts(block *flow.Block) []*flow.ExecutionReceipt {
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
	suite.snapshot.On("Identities", mock.Anything).Return(ids, nil)
	return receipts

}

func getEvents(n int) []flow.Event {
	events := make([]flow.Event, n)
	for i := range events {
		events[i] = flow.Event{Type: flow.EventAccountCreated}
	}
	return events
}

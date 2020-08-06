package backend

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	mockaccess "github.com/dapperlabs/flow-go/engine/access/mock"
	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	realstorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks       *storage.Blocks
	headers      *storage.Headers
	collections  *storage.Collections
	transactions *storage.Transactions
	colClient    *mockaccess.AccessAPIClient
	execClient   *mockaccess.ExecutionAPIClient
	chainID      flow.ChainID
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	suite.log = zerolog.Logger{}
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.blocks = new(storage.Blocks)
	suite.headers = new(storage.Headers)
	suite.transactions = new(storage.Transactions)
	suite.collections = new(storage.Collections)
	suite.colClient = new(mockaccess.AccessAPIClient)
	suite.execClient = new(mockaccess.ExecutionAPIClient)
	suite.chainID = flow.Testnet
}

func (suite *Suite) TestPing() {
	suite.colClient.On("Ping", mock.Anything, &access.PingRequest{}).Return(&access.PingResponse{}, nil)
	suite.execClient.On("Ping", mock.Anything, &execution.PingRequest{}).Return(&execution.PingResponse{}, nil)

	backend := New(
		suite.state,
		suite.execClient,
		suite.colClient,
		nil, nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
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
		nil, nil, nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
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

func (suite *Suite) TestGetLatestSealedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(
		suite.state,
		nil, nil, nil,
		suite.headers, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
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
	transaction := unittest.TransactionFixture()
	expected := transaction.TransactionBody

	suite.transactions.On("ByID", transaction.ID()).Return(&expected, nil).Once()

	backend := New(
		suite.state,
		nil, nil, nil, nil, nil,
		suite.transactions,
		suite.chainID,
		metrics.NewNoopCollector(),
	)

	actual, err := backend.GetTransaction(context.Background(), transaction.ID())
	suite.checkResponse(actual, err)

	suite.Require().Equal(expected, *actual)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetCollection() {
	expected := unittest.CollectionFixture(1)

	light := expected.Light()
	suite.collections.On("LightByID", expected.ID()).Return(&light, nil).Once()

	for _, t := range expected.Transactions {
		suite.transactions.On("ByID", t.ID()).Return(t, nil).Once()
	}

	backend := New(
		suite.state,
		nil, nil, nil, nil,
		suite.collections,
		suite.transactions,
		suite.chainID,
		metrics.NewNoopCollector(),
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

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	block.Header.Height = 2
	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = block.Header.Height - 1 // head is behind the current block
	suite.snapshot.On("Head").Return(headBlock.Header, nil)

	light := collection.Light()
	// transaction storage returns the corresponding transaction
	suite.transactions.On("ByID", transactionBody.ID()).Return(transactionBody, nil)
	// collection storage returns the corresponding collection
	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(&light, nil)
	// block storage returns the corresponding block
	suite.blocks.On("ByCollectionID", collection.ID()).Return(&block, nil)

	txID := transactionBody.ID()
	blockID := block.ID()

	exeEventReq := execution.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}

	exeEventResp := execution.GetTransactionResultResponse{
		Events: nil,
	}

	backend := New(
		suite.state,
		suite.execClient,
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.chainID,
		metrics.NewNoopCollector(),
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

// TestTransactionExpiredStatusTransition tests that the status of transaction changes from Unknown to Expired
// when enough blocks pass
func (suite *Suite) TestTransactionExpiredStatusTransition() {

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	block.Header.Height = 2
	transactionBody.SetReferenceBlockID(block.ID())
	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = block.Header.Height - 1 // head is behind the current block
	suite.snapshot.On("Head").Return(headBlock.Header, nil)
	snapshotAtBlock := new(protocol.Snapshot)
	snapshotAtBlock.On("Head").Return(block.Header, nil)
	suite.state.On("AtBlockID", block.ID()).Return(snapshotAtBlock, nil)

	// transaction storage returns the corresponding transaction
	suite.transactions.On("ByID", transactionBody.ID()).Return(transactionBody, nil)
	// collection storage returns a not found error
	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(nil, realstorage.ErrNotFound)

	txID := transactionBody.ID()

	backend := New(
		suite.state,
		suite.execClient,
		nil,
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.chainID,
		metrics.NewNoopCollector(),
	)

	// first call - referenced block isn't known yet, so should return pending status
	result, err := backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	suite.Assert().Equal(flow.TransactionStatusPending, result.Status)

	// now go far into the future
	headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1

	// second call - reference block is now very far behind, and should be considered expired
	result, err = backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	suite.Assert().Equal(flow.TransactionStatusExpired, result.Status)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	// setup the mocks
	expected := unittest.BlockFixture()
	header := expected.Header
	suite.snapshot.On("Head").Return(header, nil).Once()
	suite.blocks.On("ByID", header.ID()).Return(&expected, nil).Once()

	backend := New(
		suite.state,
		nil, nil,
		suite.blocks,
		nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
	)

	// query the handler for the latest finalized header
	actual, err := backend.GetLatestBlock(context.Background(), false)
	suite.checkResponse(actual, err)

	// make sure we got the latest header
	suite.Require().Equal(expected, *actual)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetEventsForBlockIDs() {

	blockIDs := getIDs(5)
	events := getEvents(10)

	// create the expected results from execution node and access node
	exeResults := make([]*execution.GetEventsForBlockIDsResponse_Result, len(blockIDs))

	for i := 0; i < len(blockIDs); i++ {
		exeResults[i] = &execution.GetEventsForBlockIDsResponse_Result{
			BlockId:     convert.IdentifierToMessage(blockIDs[i]),
			BlockHeight: uint64(i),
			Events:      convert.EventsToMessages(events),
		}
	}

	expected := make([]BlockEvents, len(blockIDs))

	for i := 0; i < len(blockIDs); i++ {
		expected[i] = BlockEvents{
			BlockID:     blockIDs[i],
			BlockHeight: uint64(i),
			Events:      events,
		}
	}

	// create the execution node response
	exeResp := execution.GetEventsForBlockIDsResponse{
		Results: exeResults,
	}

	ctx := context.Background()

	exeReq := &execution.GetEventsForBlockIDsRequest{
		BlockIds: convert.IdentifiersToMessages(blockIDs),
		Type:     string(flow.EventAccountCreated),
	}

	// expect one call to the executor api client
	suite.execClient.On("GetEventsForBlockIDs", ctx, exeReq).Return(&exeResp, nil).Once()

	// create the handler
	backend := New(
		suite.state,
		suite.execClient,
		nil, nil, nil, nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
	)

	// execute request
	actual, err := backend.GetEventsForBlockIDs(ctx, string(flow.EventAccountCreated), blockIDs)
	suite.checkResponse(actual, err)

	suite.Require().Equal(expected, actual)
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetEventsForHeightRange() {
	ctx := context.Background()
	var minHeight uint64 = 5
	var maxHeight uint64 = 10
	var headHeight uint64
	var expBlockIDs []flow.Identifier

	setupHeadHeight := func(height uint64) {
		header := unittest.BlockHeaderFixture() // create a mock header
		header.Height = height                  // set the header height
		suite.snapshot.On("Head").Return(&header, nil).Once()
	}

	setupStorage := func(min uint64, max uint64) []flow.Identifier {
		ids := make([]flow.Identifier, 0)

		for i := min; i <= max; i++ {
			b := unittest.BlockFixture()

			suite.blocks.
				On("ByHeight", i).
				Return(&b, nil).Once()

			ids = append(ids, b.ID())
		}

		return ids
	}

	setupExecClient := func() []BlockEvents {
		execReq := &execution.GetEventsForBlockIDsRequest{
			BlockIds: convert.IdentifiersToMessages(expBlockIDs),
			Type:     string(flow.EventAccountCreated),
		}

		results := make([]BlockEvents, len(expBlockIDs))
		exeResults := make([]*execution.GetEventsForBlockIDsResponse_Result, len(expBlockIDs))

		for i, id := range expBlockIDs {
			events := getEvents(1)
			height := uint64(5) // an arbitrary height

			results[i] = BlockEvents{
				BlockID:     id,
				BlockHeight: height,
				Events:      events,
			}

			exeResults[i] = &execution.GetEventsForBlockIDsResponse_Result{
				BlockId:     convert.IdentifierToMessage(id),
				BlockHeight: height,
				Events:      convert.EventsToMessages(events),
			}
		}

		exeResp := &execution.GetEventsForBlockIDsResponse{
			Results: exeResults,
		}

		suite.execClient.On("GetEventsForBlockIDs", ctx, execReq).Return(exeResp, nil).Once()

		return results
	}

	// TODO: validate event requests
	// suite.Run("invalid request max height < min height", func() {
	// 	backend := New(
	// 		suite.state,
	// 		nil, nil, nil, nil, nil, nil,
	// 		suite.chainID,
	// 		metrics.NewNoopCollector(),
	// 	)
	//
	// 	_, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), maxHeight, minHeight)
	// 	suite.Require().Error(err)
	//
	// 	suite.assertAllExpectations() // assert that request was not sent to execution node
	// })

	suite.Run("valid request with min_height < max_height < last_sealed_block_height", func() {

		headHeight = maxHeight + 1

		// setup mocks
		setupHeadHeight(headHeight)
		expBlockIDs = setupStorage(minHeight, maxHeight)
		expectedResp := setupExecClient()

		// create handler
		backend := New(
			suite.state,
			suite.execClient,
			nil,
			suite.blocks,
			suite.headers,
			nil, nil,
			suite.chainID,
			metrics.NewNoopCollector(),
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
		expBlockIDs = setupStorage(minHeight, headHeight)
		expectedResp := setupExecClient()

		backend := New(
			suite.state,
			suite.execClient,
			nil,
			suite.blocks,
			suite.headers,
			nil, nil,
			suite.chainID,
			metrics.NewNoopCollector(),
		)

		actualResp, err := backend.GetEventsForHeightRange(ctx, string(flow.EventAccountCreated), minHeight, maxHeight)
		suite.checkResponse(actualResp, err)

		suite.assertAllExpectations()
		suite.Require().Equal(expectedResp, actualResp)
	})

}

func (suite *Suite) TestGetAccount() {

	address, err := suite.chainID.Chain().NewAddressGenerator().NextAddress()
	suite.Require().NoError(err)

	account := &entities.Account{
		Address: address.Bytes(),
	}
	ctx := context.Background()

	// setup the latest sealed block
	header := unittest.BlockHeaderFixture() // create a mock header
	seal := unittest.BlockSealFixture()     // create a mock seal
	seal.BlockID = header.ID()              // make the seal point to the header
	suite.snapshot.On("Head").Return(&header, nil).Once()

	// create the expected execution API request
	blockID := header.ID()
	exeReq := &execution.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: address.Bytes(),
	}

	// create the expected execution API response
	exeResp := &execution.GetAccountAtBlockIDResponse{
		Account: account,
	}

	// setup the execution client mock
	suite.execClient.On("GetAccountAtBlockID", ctx, exeReq).Return(exeResp, nil).Once()

	// create the handler with the mock
	backend := New(
		suite.state,
		suite.execClient,
		nil, nil,
		suite.headers,
		nil, nil,
		suite.chainID,
		metrics.NewNoopCollector(),
	)

	suite.Run("happy path - valid request and valid response", func() {
		account, err := backend.GetAccountAtLatestBlock(ctx, address)
		suite.checkResponse(account, err)

		suite.Require().Equal(address, account.Address)

		suite.assertAllExpectations()
	})
}

func (suite *Suite) TestGetAccountAtBlockHeight() {
	height := uint64(5)
	address := unittest.AddressFixture()
	account := &entities.Account{
		Address: address.Bytes(),
	}
	ctx := context.Background()

	// create a mock block header
	h := unittest.BlockHeaderFixture()
	// setup headers storage to return the header when queried by height
	suite.headers.On("ByHeight", height).Return(&h, nil).Once()

	// create the expected execution API request
	blockID := h.ID()
	exeReq := &execution.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: address.Bytes(),
	}

	// create the expected execution API response
	exeResp := &execution.GetAccountAtBlockIDResponse{
		Account: account,
	}

	// setup the execution client mock
	suite.execClient.On("GetAccountAtBlockID", ctx, exeReq).Return(exeResp, nil).Once()

	// create the handler with the mock
	backend := New(
		suite.state,
		suite.execClient,
		nil, nil,
		suite.headers,
		nil, nil,
		flow.Testnet,
		metrics.NewNoopCollector(),
	)

	suite.Run("happy path - valid request and valid response", func() {
		account, err := backend.GetAccountAtBlockHeight(ctx, address, height)
		suite.checkResponse(account, err)

		suite.Require().Equal(address, account.Address)

		suite.assertAllExpectations()
	})
}

func (suite *Suite) TestGetNetworkParameters() {
	expectedChainID := flow.Mainnet

	backend := New(
		nil, nil, nil, nil, nil, nil, nil,
		flow.Mainnet,
		metrics.NewNoopCollector(),
	)

	params := backend.GetNetworkParameters(context.Background())

	suite.Require().Equal(expectedChainID, params.ChainID)
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

func getIDs(n int) []flow.Identifier {
	ids := make([]flow.Identifier, n)
	for i := range ids {
		ids[i] = unittest.IdentifierFixture()
	}
	return ids
}

func getEvents(n int) []flow.Event {
	events := make([]flow.Event, 10)
	for i := range events {
		events[i] = flow.Event{Type: flow.EventAccountCreated}
	}
	return events
}

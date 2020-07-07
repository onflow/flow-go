package handler

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	mockaccess "github.com/dapperlabs/flow-go/engine/access/mock"
	"github.com/dapperlabs/flow-go/engine/common/convert"
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
}

func (suite *Suite) TestPing() {
	suite.colClient.On("Ping", mock.Anything, &access.PingRequest{}).Return(&access.PingResponse{}, nil)
	suite.execClient.On("Ping", mock.Anything, &execution.PingRequest{}).Return(&execution.PingResponse{}, nil)
	handler := NewHandler(suite.log, suite.state, suite.execClient, suite.colClient, nil, nil, nil, nil, suite.chainID,
		metrics.NewNoopCollector())
	ping := &access.PingRequest{}
	pong, err := handler.Ping(context.Background(), ping)
	suite.checkResponse(pong, err)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, nil, nil, suite.chainID,
		metrics.NewNoopCollector())

	// query the handler for the latest finalized block
	req := &access.GetLatestBlockHeaderRequest{IsSealed: false}
	resp, err := handler.GetLatestBlockHeader(context.Background(), req)
	suite.checkResponse(resp, err)

	// make sure we got the latest block
	id := block.ID()
	suite.Require().Equal(id[:], resp.GetBlock().GetId())
	suite.Require().Equal(block.Height, resp.GetBlock().GetHeight())
	suite.Require().Equal(block.ParentID[:], resp.GetBlock().GetParentId())
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestSealedBlockHeader() {
	//setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, suite.headers, nil, nil, suite.chainID,
		metrics.NewNoopCollector())

	// query the handler for the latest sealed block
	req := &access.GetLatestBlockHeaderRequest{IsSealed: true}
	resp, err := handler.GetLatestBlockHeader(context.Background(), req)
	suite.checkResponse(resp, err)

	// make sure we got the latest sealed block
	id := block.ID()
	suite.Require().Equal(id[:], resp.GetBlock().GetId())
	suite.Require().Equal(block.Height, resp.GetBlock().GetHeight())
	suite.Require().Equal(block.ParentID[:], resp.GetBlock().GetParentId())
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetTransaction() {
	transaction := unittest.TransactionFixture()
	expected := transaction.TransactionBody
	suite.transactions.On("ByID", transaction.ID()).Return(&expected, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, nil, suite.transactions, suite.chainID,
		metrics.NewNoopCollector())
	id := transaction.ID()
	req := &access.GetTransactionRequest{
		Id: id[:],
	}

	resp, err := handler.GetTransaction(context.Background(), req)
	suite.checkResponse(resp, err)

	actual, err := convert.MessageToTransaction(resp.GetTransaction())
	suite.checkResponse(resp, err)
	suite.Require().Equal(expected, actual)
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetCollection() {
	collection := unittest.CollectionFixture(1)

	expectedIDs := make([]flow.Identifier, len(collection.Transactions))

	for i, t := range collection.Transactions {
		expectedIDs[i] = t.ID()
	}

	light := collection.Light()
	suite.collections.On("LightByID", collection.ID()).Return(&light, nil).Once()
	for _, t := range collection.Transactions {
		suite.transactions.On("ByID", t.ID()).Return(t, nil).Once()
	}
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, suite.collections, suite.transactions,
		suite.chainID, metrics.NewNoopCollector())
	id := collection.ID()
	req := &access.GetCollectionByIDRequest{
		Id: id[:],
	}

	resp, err := handler.GetCollectionByID(context.Background(), req)
	suite.transactions.AssertExpectations(suite.T())
	suite.checkResponse(resp, err)

	actualColl := resp.GetCollection()
	actualIDs := make([]flow.Identifier, len(actualColl.TransactionIds))
	for i, t := range actualColl.TransactionIds {
		actualIDs[i] = flow.HashToID(t)
	}

	suite.ElementsMatch(expectedIDs, actualIDs)
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

	handler := NewHandler(suite.log, suite.state, suite.execClient, nil, suite.blocks, suite.headers, suite.collections,
		suite.transactions, suite.chainID, metrics.NewNoopCollector())
	req := &access.GetTransactionRequest{
		Id: txID[:],
	}

	// Successfully return empty event list
	suite.execClient.On("GetTransactionResult", ctx, &exeEventReq).Return(&exeEventResp, status.Errorf(codes.NotFound,
		"not found")).Once()
	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	resp, err := handler.GetTransactionResult(ctx, req)
	suite.checkResponse(resp, err)

	// status should be finalized since the sealed blocks is smaller in height
	suite.Assert().Equal(entities.TransactionStatus_FINALIZED, resp.GetStatus())

	// Successfully return empty event list from here on
	suite.execClient.On("GetTransactionResult", ctx, &exeEventReq).Return(&exeEventResp, nil)
	// second call - when block under test's height is greater height than the sealed head
	resp, err = handler.GetTransactionResult(ctx, req)
	suite.checkResponse(resp, err)

	// status should be executed since no `NotFound` error in the `GetTransactionResult` call
	suite.Assert().Equal(entities.TransactionStatus_EXECUTED, resp.GetStatus())

	// now let the head block be finalized
	headBlock.Header.Height = block.Header.Height + 1

	// third call - when block under test's height is less than sealed head's height
	resp, err = handler.GetTransactionResult(ctx, req)
	suite.checkResponse(resp, err)

	// status should be sealed since the sealed blocks is greater in height
	suite.Assert().Equal(entities.TransactionStatus_SEALED, resp.GetStatus())

	// now go far into the future
	headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1

	// fourth call - when block under test's height so much less than the head's height that it's considered expired,
	// but since there is a execution result, means it should retain it's sealed status
	resp, err = handler.GetTransactionResult(ctx, req)
	suite.checkResponse(resp, err)

	// status should be expired since
	suite.Assert().Equal(entities.TransactionStatus_SEALED, resp.GetStatus())

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

	handler := NewHandler(suite.log, suite.state, suite.execClient, nil, suite.blocks, suite.headers, suite.collections,
		suite.transactions, suite.chainID, metrics.NewNoopCollector())
	req := &access.GetTransactionRequest{
		Id: txID[:],
	}

	// first call - referenced block isn't known yet, so should return pending status
	resp, err := handler.GetTransactionResult(ctx, req)
	suite.checkResponse(resp, err)

	suite.Assert().Equal(entities.TransactionStatus_PENDING, resp.GetStatus())

	// now go far into the future
	headBlock.Header.Height = block.Header.Height + flow.DefaultTransactionExpiry + 1

	// second call - reference block is now very far behind, and should be considered expired
	resp, err = handler.GetTransactionResult(ctx, req)
	suite.checkResponse(resp, err)

	suite.Assert().Equal(entities.TransactionStatus_EXPIRED, resp.GetStatus())

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header
	suite.snapshot.On("Head").Return(header, nil).Once()
	suite.blocks.On("ByID", header.ID()).Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, suite.blocks, nil, nil, nil, suite.chainID,
		metrics.NewNoopCollector())

	// query the handler for the latest finalized header
	req := &access.GetLatestBlockRequest{IsSealed: false}
	resp, err := handler.GetLatestBlock(context.Background(), req)
	suite.checkResponse(resp, err)

	// make sure we got the latest header
	expected, err := convert.BlockToMessage(&block)
	suite.Require().NoError(err)
	suite.Require().Equal(expected, resp.Block)
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetEventsForBlockIDs() {

	blockIDs := getIDs(5)
	events := getEvents(10)

	// create the expected results from execution node and access node
	exeResults := make([]*execution.GetEventsForBlockIDsResponse_Result, len(blockIDs))
	accResults := make([]*access.EventsResponse_Result, len(blockIDs))

	for i := 0; i < len(blockIDs); i++ {
		exeResults[i] = &execution.GetEventsForBlockIDsResponse_Result{
			BlockId:     blockIDs[i],
			BlockHeight: uint64(i),
			Events:      events,
		}
		accResults[i] = &access.EventsResponse_Result{
			BlockId:     blockIDs[i],
			BlockHeight: uint64(i),
			Events:      events,
		}
	}

	// create the execution node response
	exeResp := execution.GetEventsForBlockIDsResponse{
		Results: exeResults,
	}
	// create the expected access node response
	expectedResp := access.EventsResponse{
		Results: accResults,
	}

	ctx := context.Background()

	exeReq := &execution.GetEventsForBlockIDsRequest{BlockIds: blockIDs, Type: string(flow.EventAccountCreated)}

	// expect one call to the executor api client
	suite.execClient.On("GetEventsForBlockIDs", ctx, exeReq).Return(&exeResp, nil).Once()

	// create the handler
	handler := NewHandler(suite.log, suite.state, suite.execClient, nil, nil, nil, nil, nil, suite.chainID,
		metrics.NewNoopCollector())

	// execute request
	req := &access.GetEventsForBlockIDsRequest{BlockIds: blockIDs, Type: string(flow.EventAccountCreated)}
	acutalResponse, err := handler.GetEventsForBlockIDs(ctx, req)

	// check response
	suite.checkResponse(acutalResponse, err)
	suite.Require().Equal(expectedResp, *acutalResponse)
	suite.assertAllExpectations()
}

func (suite *Suite) TestGetEventsForHeightRange() {
	ctx := context.Background()
	var minHeight uint64 = 5
	var maxHeight uint64 = 10
	var headHeight uint64
	var expBlockIDs [][]byte

	setupHeadHeight := func(height uint64) {
		header := unittest.BlockHeaderFixture() // create a mock header
		header.Height = height                  // set the header height
		suite.snapshot.On("Head").Return(&header, nil).Once()
	}

	setupStorage := func(min uint64, max uint64) [][]byte {
		ids := make([][]byte, 0)
		for i := min; i <= max; i++ {
			b := unittest.BlockFixture()
			suite.blocks.On("ByHeight", i).Return(&b, nil).Once()
			m, err := convert.BlockToMessage(&b)
			suite.Require().NoError(err)
			ids = append(ids, m.Id)
		}
		return ids
	}

	setupExecClient := func() *access.EventsResponse {
		execReq := &execution.GetEventsForBlockIDsRequest{BlockIds: expBlockIDs, Type: string(flow.EventAccountCreated)}
		results := make([]*access.EventsResponse_Result, len(expBlockIDs))
		exeResults := make([]*execution.GetEventsForBlockIDsResponse_Result, len(expBlockIDs))
		for i, id := range expBlockIDs {
			e := getEvents(1)
			h := uint64(5) // an arbitrary height
			results[i] = &access.EventsResponse_Result{
				BlockId:     id,
				BlockHeight: h,
				Events:      e,
			}
			exeResults[i] = &execution.GetEventsForBlockIDsResponse_Result{
				BlockId:     id,
				BlockHeight: h,
				Events:      e,
			}
		}
		expectedResp := &access.EventsResponse{
			Results: results,
		}
		exeResp := &execution.GetEventsForBlockIDsResponse{
			Results: exeResults,
		}
		suite.execClient.On("GetEventsForBlockIDs", ctx, execReq).Return(exeResp, nil).Once()
		return expectedResp
	}

	suite.Run("invalid request max height < min height", func() {
		req := &access.GetEventsForHeightRangeRequest{
			StartHeight: maxHeight,
			EndHeight:   minHeight,
			Type:        string(flow.EventAccountCreated)}

		handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, nil, nil, suite.chainID,
			metrics.NewNoopCollector())
		_, err := handler.GetEventsForHeightRange(ctx, req)
		require.Error(suite.T(), err)
		suite.assertAllExpectations() // assert that request was not sent to execution node
	})

	suite.Run("valid request with min_height < max_height < last_sealed_block_height", func() {

		headHeight = maxHeight + 1

		// setup mocks
		setupHeadHeight(headHeight)
		expBlockIDs = setupStorage(minHeight, maxHeight)
		expectedResp := setupExecClient()

		// create handler
		handler := NewHandler(suite.log, suite.state, suite.execClient, nil, suite.blocks, suite.headers, nil, nil,
			suite.chainID, metrics.NewNoopCollector())

		req := &access.GetEventsForHeightRangeRequest{
			StartHeight: minHeight,
			EndHeight:   maxHeight,
			Type:        string(flow.EventAccountCreated)}

		// execute request
		actualResp, err := handler.GetEventsForHeightRange(ctx, req)

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

		handler := NewHandler(suite.log, suite.state, suite.execClient, nil, suite.blocks, suite.headers, nil, nil,
			suite.chainID, metrics.NewNoopCollector())

		req := &access.GetEventsForHeightRangeRequest{
			StartHeight: minHeight,
			EndHeight:   maxHeight,
			Type:        string(flow.EventAccountCreated)}

		actualResp, err := handler.GetEventsForHeightRange(ctx, req)

		suite.checkResponse(actualResp, err)
		suite.assertAllExpectations()
		suite.Require().Equal(expectedResp, actualResp)
	})

}

func (suite *Suite) TestGetAccount() {

	address := []byte{1}
	account := &entities.Account{
		Address: address,
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
		Address: address,
	}

	// create the expected execution API response
	exeResp := &execution.GetAccountAtBlockIDResponse{
		Account: account,
	}

	// setup the execution client mock
	suite.execClient.On("GetAccountAtBlockID", ctx, exeReq).Return(exeResp, nil).Once()

	// create the handler with the mock
	handler := NewHandler(suite.log, suite.state, suite.execClient, nil, nil, suite.headers, nil, nil, suite.chainID,
		metrics.NewNoopCollector())

	suite.Run("happy path - valid request and valid response", func() {
		expectedResp := &access.AccountResponse{
			Account: account,
		}
		req := &access.GetAccountAtLatestBlockRequest{
			Address: address,
		}
		actualResp, err := handler.GetAccountAtLatestBlock(ctx, req)
		suite.checkResponse(actualResp, err)
		suite.assertAllExpectations()
		suite.Require().Equal(expectedResp, actualResp)
	})

	suite.Run("invalid request with nil address", func() {
		req := &access.GetAccountAtLatestBlockRequest{
			Address: nil,
		}
		_, err := handler.GetAccountAtLatestBlock(ctx, req)
		suite.Require().Error(err)
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
	handler := NewHandler(suite.log, suite.state, suite.execClient, nil, nil, suite.headers, nil, nil, flow.Testnet,
		metrics.NewNoopCollector())

	suite.Run("happy path - valid request and valid response", func() {
		expectedResp := &access.AccountResponse{
			Account: account,
		}
		req := &access.GetAccountAtBlockHeightRequest{
			Address:     address.Bytes(),
			BlockHeight: height,
		}
		actualResp, err := handler.GetAccountAtBlockHeight(ctx, req)
		suite.checkResponse(actualResp, err)
		suite.assertAllExpectations()
		suite.Require().Equal(expectedResp, actualResp)
	})

	suite.Run("invalid request with nil address", func() {
		req := &access.GetAccountAtBlockHeightRequest{
			Address:     nil,
			BlockHeight: height,
		}
		_, err := handler.GetAccountAtBlockHeight(ctx, req)
		suite.Require().Error(err)
	})

	suite.Run("invalid request with empty address", func() {
		req := &access.GetAccountAtBlockHeightRequest{
			Address:     []byte{},
			BlockHeight: height,
		}
		_, err := handler.GetAccountAtBlockHeight(ctx, req)
		suite.Require().Error(err)
	})
}

func (suite *Suite) TestGetNetworkParameters() {
	expectedChainID := string(flow.Mainnet)
	handler := NewHandler(suite.log, nil, nil, nil, nil, nil, nil, nil, flow.Mainnet, metrics.NewNoopCollector())
	npReq := &access.GetNetworkParametersRequest{}
	npResp, err := handler.GetNetworkParameters(context.Background(), npReq)
	suite.checkResponse(npResp, err)
	suite.Require().Equal(expectedChainID, npResp.GetChainId())
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

func getIDs(n int) [][]byte {
	ids := make([][]byte, n)
	for i := range ids {
		id := unittest.IdentifierFixture()
		ids[i] = id[:]
	}
	return ids
}

func getEvents(n int) []*entities.Event {
	events := make([]*entities.Event, 10)
	for i := range events {
		event := entities.Event{Type: string(flow.EventAccountCreated)}
		events[i] = &event
	}
	return events
}

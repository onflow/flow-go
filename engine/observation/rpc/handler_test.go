package rpc

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"
	"github.com/dapperlabs/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	mockaccess "github.com/dapperlabs/flow-go/engine/observation/mock"
	"github.com/dapperlabs/flow-go/model/flow"
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
	execClient   *mockaccess.AccessAPIClient
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.Logger{}
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.blocks = new(storage.Blocks)
	suite.headers = new(storage.Headers)
	suite.transactions = new(storage.Transactions)
	suite.collections = new(storage.Collections)
	suite.execClient = new(mockaccess.AccessAPIClient)
}

func (suite *Suite) TestPing() {
	handler := NewHandler(suite.log, nil, nil, nil, nil, nil, nil, nil)
	ping := &access.PingRequest{}
	pong, err := handler.Ping(context.Background(), ping)
	suite.checkResponse(pong, err)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, nil, nil)

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
	seal := unittest.SealFixture()
	suite.snapshot.On("Seal").Return(seal, nil).Once()

	suite.headers.On("ByBlockID", seal.BlockID).Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, suite.headers, nil, nil)

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
	transaction := unittest.TransactionFixture(func(t *flow.Transaction) {
		t.Nonce = 0
		t.ComputeLimit = 0
	})
	expected := transaction.TransactionBody
	suite.transactions.On("ByID", transaction.ID()).Return(&expected, nil).Once()
	suite.collections.On("LightByTransactionID", transaction.ID()).Return(nil, realstorage.ErrNotFound).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, suite.collections, suite.transactions)
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
		t.Nonce = 0 // obs api doesn't exposes nonce and compute limit as part of a transaction
		t.ComputeLimit = 0
		expectedIDs[i] = t.ID()
	}

	light := collection.Light()
	suite.collections.On("LightByID", collection.ID()).Return(&light, nil).Once()
	for _, t := range collection.Transactions {
		suite.transactions.On("ByID", t.ID()).Return(t, nil).Once()
	}
	handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, suite.collections, suite.transactions)
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

	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	block.Height = 2
	headBlock := unittest.BlockFixture()
	headBlock.Height = block.Height - 1 // head is behind the current block

	seal := unittest.SealFixture()
	seal.BlockID = headBlock.ID()
	suite.snapshot.On("Seal").Return(seal, nil).Twice()
	suite.headers.On("ByBlockID", seal.BlockID).Return(&headBlock.Header, nil).Twice()

	light := collection.Light()
	// transaction storage returns the corresponding transaction
	suite.transactions.On("ByID", transactionBody.ID()).Return(transactionBody, nil).Twice()
	// collection storage returns the corresponding collection
	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(&light, nil).Twice()
	// block storage returns the corresponding block
	suite.blocks.On("ByCollectionID", collection.ID()).Return(&block, nil).Twice()

	handler := NewHandler(suite.log, suite.state, nil, nil, suite.blocks, suite.headers, suite.collections, suite.transactions)
	id := transactionBody.ID()
	req := &access.GetTransactionRequest{
		Id: id[:],
	}

	resp, err := handler.GetTransaction(context.Background(), req)
	suite.checkResponse(resp, err)

	// status should be finalized since the sealed blocks is smaller in height
	suite.Assert().Equal(entities.TransactionStatus_STATUS_FINALIZED, resp.Transaction.Status)

	// now let the head block be finalized
	headBlock.Height = block.Height + 1

	resp, err = handler.GetTransaction(context.Background(), req)
	suite.checkResponse(resp, err)

	// status should be sealed since the sealed blocks is greater in height
	suite.Assert().Equal(entities.TransactionStatus_STATUS_SEALED, resp.Transaction.Status)

	suite.assertAllExpectations()
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	// setup the mocks
	block := unittest.BlockFixture()
	header := block.Header
	suite.snapshot.On("Head").Return(&header, nil).Once()
	suite.blocks.On("ByID", header.ID()).Return(&block, nil).Once()
	handler := NewHandler(suite.log, suite.state, nil, nil, suite.blocks, nil, nil, nil)

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

	req := &access.GetEventsForBlockIDsRequest{BlockIds: blockIDs, Type: string(flow.EventAccountCreated)}
	expectedResp := access.EventsResponse{
		Events: events,
	}
	ctx := context.Background()

	// expect one call to the executor api client
	suite.execClient.On("GetEventsForBlockIDs", ctx, req).Return(&expectedResp, nil).Once()

	// create the handler
	handler := NewHandler(suite.log, suite.state, suite.execClient, nil, nil, nil, nil, nil)

	// execute request
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
		seal := unittest.SealFixture()          // create a mock seal
		seal.BlockID = header.ID()              // make the seal point to the header
		suite.snapshot.On("Seal").Return(seal, nil).Once()
		suite.headers.On("ByBlockID", header.ID()).Return(&header, nil).Once()
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

	setupExecClient := func(events []*entities.Event) {
		execReq := &access.GetEventsForBlockIDsRequest{BlockIds: expBlockIDs, Type: string(flow.EventAccountCreated)}
		execResp := access.EventsResponse{
			Events: events,
		}
		suite.execClient.On("GetEventsForBlockIDs", ctx, execReq).Return(&execResp, nil).Once()
	}

	suite.Run("invalid request max height < min height", func() {
		req := &access.GetEventsForHeightRangeRequest{
			StartHeight: maxHeight,
			EndHeight:   minHeight,
			Type:        string(flow.EventAccountCreated)}

		handler := NewHandler(suite.log, suite.state, nil, nil, nil, nil, nil, nil)
		_, err := handler.GetEventsForHeightRange(ctx, req)
		require.Error(suite.T(), err)
		suite.assertAllExpectations() // assert that request was not sent to execution node
	})

	suite.Run("valid request with min_height < max_height < last_sealed_block_height", func() {

		headHeight = maxHeight + 1

		// setup mocks
		setupHeadHeight(headHeight)
		expBlockIDs = setupStorage(minHeight, maxHeight)
		expEvents := getEvents(10)
		setupExecClient(expEvents)

		// create handler
		handler := NewHandler(suite.log, suite.state, suite.execClient, nil, suite.blocks, suite.headers, nil, nil)

		req := &access.GetEventsForHeightRangeRequest{
			StartHeight: minHeight,
			EndHeight:   maxHeight,
			Type:        string(flow.EventAccountCreated)}

		// execute request
		resp, err := handler.GetEventsForHeightRange(ctx, req)

		// check response
		suite.checkResponse(resp, err)
		suite.assertAllExpectations()
		suite.Require().Equal(expEvents, resp.GetEvents())
	})

	suite.Run("valid request with max_height > last_sealed_block_height", func() {
		headHeight = maxHeight - 1
		setupHeadHeight(headHeight)
		expBlockIDs = setupStorage(minHeight, headHeight)

		expEvents := getEvents(10)
		setupExecClient(expEvents)

		handler := NewHandler(suite.log, suite.state, suite.execClient, nil, suite.blocks, suite.headers, nil, nil)

		req := &access.GetEventsForHeightRangeRequest{
			StartHeight: minHeight,
			EndHeight:   maxHeight,
			Type:        string(flow.EventAccountCreated)}

		resp, err := handler.GetEventsForHeightRange(ctx, req)

		suite.checkResponse(resp, err)
		suite.assertAllExpectations()
		suite.Require().Equal(expEvents, resp.GetEvents())
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

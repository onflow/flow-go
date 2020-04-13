package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow/protobuf/go/flow/entities"
	"github.com/dapperlabs/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	log    zerolog.Logger
	events *storage.Events
	blocks *storage.Blocks
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.Logger{}
	suite.events = new(storage.Events)
	suite.blocks = new(storage.Blocks)
}

// TestGetEventsForBlockIDs tests the GetEventsForBlockIDs API call
func (suite *Suite) TestGetEventsForBlockIDs() {

	totalBlocks := 10
	eventsPerBlock := 10

	blockIDs := make([][]byte, totalBlocks)
	expectedResult := make([]*execution.EventsResponse_Result, totalBlocks)

	// setup the events storage mock
	for i := range blockIDs {
		block := unittest.BlockFixture()
		block.Height = uint64(i)
		id := block.ID()
		blockIDs[i] = id[:]
		eventsForBlock := make([]flow.Event, eventsPerBlock)
		eventMessages := make([]*entities.Event, eventsPerBlock)
		for j := range eventsForBlock {
			e := unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture())
			eventsForBlock[j] = e
			eventMessages[j] = convert.EventToMessage(e)
		}
		// expect one call to lookup events for each block ID
		suite.events.On("ByBlockIDEventType", id, flow.EventAccountCreated).Return(eventsForBlock, nil).Once()

		// expect one call to lookup each block
		suite.blocks.On("ByID", id).Return(&block, nil).Once()

		// create the expected result for this block
		expectedResult[i] = &execution.EventsResponse_Result{
			BlockId:     id[:],
			BlockHeight: block.Height,
			Events:      eventMessages,
		}
	}

	// create the handler
	handler := &handler{
		blocks: suite.blocks,
		events: suite.events,
	}

	concoctReq := func(errType string, blockIDs [][]byte) *execution.GetEventsForBlockIDsRequest {
		return &execution.GetEventsForBlockIDsRequest{
			Type:     errType,
			BlockIds: blockIDs,
		}
	}

	// happy path - valid requests receives a valid response
	suite.Run("happy path", func() {

		// create a valid API request
		req := concoctReq(string(flow.EventAccountCreated), blockIDs)

		// execute the GetEventsForBlockIDs call
		resp, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		actualResult := resp.GetResults()
		suite.Require().ElementsMatch(expectedResult, actualResult)

		// check that appropriate storage calls were made
		suite.events.AssertExpectations(suite.T())
	})

	// failure path - empty even type in the request results in an error
	suite.Run("request with empty event type", func() {

		// create an API request with empty even type
		req := concoctReq("", blockIDs)

		_, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))

		// check that no storage calls was made
		suite.events.AssertExpectations(suite.T())
	})

	// failure path - empty block ids in request results in an error
	suite.Run("request with empty block IDs", func() {

		// create an API request with empty block ids
		req := concoctReq(string(flow.EventAccountCreated), nil)

		_, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))

		// check that no storage calls was made
		suite.events.AssertExpectations(suite.T())
	})

	// failure path - non-existent block id in request results in an error
	suite.Run("request with non-existent block ID", func() {

		id := unittest.IdentifierFixture()

		// expect a storage call for the invalid id but return an error
		suite.events.On("ByBlockIDEventType", id, flow.EventAccountCreated).Return(nil, errors.New("")).Once()

		// create an API request with the invalid block id
		req := concoctReq(string(flow.EventAccountCreated), [][]byte{id[:]})

		_, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.Internal, ""))

		// check that no storage calls was made
		suite.events.AssertExpectations(suite.T())
	})
}

// Test GetAccountAtBlockID tests the GetAccountAtBlockID API call
func (suite *Suite) TestGetAccountAtBlockID() {

	id := unittest.IdentifierFixture()
	rootAddress := flow.RootAddress

	rootAccount := flow.Account{
		Address: rootAddress,
	}

	mockEngine := new(mock.IngestRPC)

	// create the handler
	handler := &handler{
		engine: mockEngine,
	}

	createReq := func(id []byte, address []byte) *execution.GetAccountAtBlockIDRequest {
		return &execution.GetAccountAtBlockIDRequest{
			Address: address,
			BlockId: id,
		}
	}

	suite.Run("happy path with valid request", func() {

		// setup mock expectations
		mockEngine.On("GetAccount", rootAddress, id).Return(&rootAccount, nil).Once()

		req := createReq(id[:], rootAddress.Bytes())

		resp, err := handler.GetAccountAtBlockID(context.Background(), req)

		suite.Require().NoError(err)
		actualAccount := resp.GetAccount()
		expectedAccount, err := convert.AccountToMessage(&rootAccount)
		suite.Require().NoError(err)
		suite.Require().Equal(*expectedAccount, *actualAccount)
		mockEngine.AssertExpectations(suite.T())
	})

	suite.Run("invalid request with nil block id", func() {

		req := createReq(nil, rootAddress.Bytes())

		_, err := handler.GetAccountAtBlockID(context.Background(), req)

		suite.Require().Error(err)
	})

	suite.Run("invalid request with nil root address", func() {

		req := createReq(id[:], nil)

		_, err := handler.GetAccountAtBlockID(context.Background(), req)

		suite.Require().Error(err)
	})
}

// TestGetEventsForBlockIDTransactionID tests the GetEventsForBlockIDTransactionID API call
func (suite *Suite) TestGetEventsForBlockIDTransactionID() {

	totalEvents := 10
	block := unittest.BlockFixture()
	tx := unittest.TransactionFixture()
	bID := block.ID()
	txID := tx.ID()

	// setup the events storage mock
	eventsForTx := make([]flow.Event, totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j := range eventsForTx {
		e := unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture())
		eventsForTx[j] = e
		eventMessages[j] = convert.EventToMessage(e)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", bID, txID).Return(eventsForTx, nil).Once()

	// expect one call to lookup each block
	suite.blocks.On("ByID", block.ID()).Return(&block, nil).Once()

	// create the expected result
	result := &execution.EventsResponse_Result{
		BlockId:     bID[:],
		BlockHeight: block.Height,
		Events:      eventMessages,
	}
	expectedResult := []*execution.EventsResponse_Result{result}

	// create the handler
	handler := &handler{
		blocks: suite.blocks,
		events: suite.events,
	}

	// concoctReq creates a GetEventsForBlockIDTransactionIDRequest
	concoctReq := func(bID []byte, tID []byte) *execution.GetEventsForBlockIDTransactionIDRequest {
		return &execution.GetEventsForBlockIDTransactionIDRequest{
			BlockId:       bID,
			TransactionId: tID,
		}
	}

	// happy path - valid requests receives all events for the given transaction
	suite.Run("happy path", func() {

		// create a valid API request
		req := concoctReq(bID[:], txID[:])

		// execute the GetEventsForBlockIDTransactionID call
		resp, err := handler.GetEventsForBlockIDTransactionID(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		actualResult := resp.GetResults()
		suite.Require().ElementsMatch(expectedResult, actualResult)

		// check that appropriate storage calls were made
		suite.events.AssertExpectations(suite.T())
	})

	// failure path - nil transaction ID in the request results in an error
	suite.Run("request with nil tx ID", func() {

		// create an API request with transaction ID as nil
		req := concoctReq(bID[:], nil)

		_, err := handler.GetEventsForBlockIDTransactionID(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))

		// check that no storage calls was made
		suite.events.AssertExpectations(suite.T())
	})

	// failure path - nil block id in the request results in an error
	suite.Run("request with nil block ID", func() {

		// create an API request with a nil block id
		req := concoctReq(nil, txID[:])

		_, err := handler.GetEventsForBlockIDTransactionID(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))

		// check that no storage calls was made
		suite.events.AssertExpectations(suite.T())
	})

	// failure path - non-existent transaction ID in request results in an error
	suite.Run("request with non-existent transaction ID", func() {

		wrongTxID := unittest.IdentifierFixture()

		// expect a storage call for the invalid bID but return an error
		suite.events.On("ByBlockIDTransactionID", bID, wrongTxID).Return(nil, errors.New("")).Once()

		// create an API request with the invalid transaction ID
		req := concoctReq(bID[:], wrongTxID[:])

		_, err := handler.GetEventsForBlockIDTransactionID(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.Internal, ""))

		// check that one storage call was made
		suite.events.AssertExpectations(suite.T())
	})
}

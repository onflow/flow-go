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
		UnimplementedExecutionAPIServer: execution.UnimplementedExecutionAPIServer{},
		blocks:                          suite.blocks,
		events:                          suite.events,
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

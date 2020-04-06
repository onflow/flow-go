package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	entities "github.com/dapperlabs/flow-go/protobuf/sdk/entities"
	access "github.com/dapperlabs/flow-go/protobuf/services/access"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	log    zerolog.Logger
	events *storage.Events
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.Logger{}
	suite.events = new(storage.Events)

}

// TestGetEventsForBlockIDs tests the GetEventsForBlockIDs API call
func (suite *Suite) TestGetEventsForBlockIDs() {

	totalBlocks := 10
	eventsPerBlock := 10
	totalEvents := totalBlocks * eventsPerBlock

	blockIDs := make([][]byte, totalBlocks)
	events := make([]flow.Event, 0)

	// setup the events storage mock
	for i := range blockIDs {
		id := unittest.IdentifierFixture()
		blockIDs[i] = id[:]
		eventsForBlock := make([]flow.Event, eventsPerBlock)
		for j := range eventsForBlock {
			eventsForBlock[j] = unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture())
		}
		// expect one call for each block ID
		suite.events.On("ByBlockIDEventType", id, flow.EventAccountCreated).Return(eventsForBlock, nil).Once()
		events = append(events, eventsForBlock...)
	}

	// create the handler
	handler := &handler{
		UnimplementedAccessAPIServer: access.UnimplementedAccessAPIServer{},
		events:                       suite.events,
	}

	concoctReq := func(errType string, blockIDs [][]byte) *access.GetEventsForBlockIDsRequest {
		return &access.GetEventsForBlockIDsRequest{
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
		actualEvents := resp.GetEvents()
		expectedEvents := make([]*entities.Event, totalEvents)
		for i, e := range events {
			expectedEvents[i] = convert.EventToMessage(e)
		}
		suite.Require().ElementsMatch(expectedEvents, actualEvents)

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
	suite.Run("request with empty event type", func() {

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
	suite.Run("request with empty event type", func() {

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

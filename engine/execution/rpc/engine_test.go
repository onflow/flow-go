package rpc

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

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

	// create the API request
	req := &access.GetEventsForBlockIDsRequest{
		Type:     string(flow.EventAccountCreated),
		BlockIds: blockIDs,
	}

	// execute the call
	resp, err := handler.GetEventsForBlockIDs(context.Background(), req)

	// check the response
	suite.Require().NoError(err)
	actualEvents := resp.Events
	expectedEvents := make([]*entities.Event, totalEvents)
	for i, e := range events {
		expectedEvents[i] = convert.EventToMessage(e)
	}
	suite.Require().ElementsMatch(expectedEvents, actualEvents)
	suite.events.AssertExpectations(suite.T())
}

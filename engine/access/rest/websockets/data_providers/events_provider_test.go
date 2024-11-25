package data_providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// EventsProviderSuite is a test suite for testing the events providers functionality.
type EventsProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api *ssmock.API

	chain          flow.Chain
	rootBlock      flow.Block
	finalizedBlock *flow.Header
}

func TestEventsProviderSuite(t *testing.T) {
	suite.Run(t, new(EventsProviderSuite))
}

func (s *EventsProviderSuite) SetupTest() {
	s.log = unittest.Logger()
	s.api = ssmock.NewAPI(s.T())

	s.chain = flow.Testnet.Chain()

	s.rootBlock = unittest.BlockFixture()
	s.rootBlock.Header.Height = 0
}

// invalidArgumentsTestCases returns a list of test cases with invalid argument combinations
// for testing the behavior of events data providers. Each test case includes a name,
// a set of input arguments, and the expected error message that should be returned.
//
// The test cases cover scenarios such as:
// 1. Supplying both 'start_block_id' and 'start_block_height' simultaneously, which is not allowed.
// 2. Providing invalid 'start_block_id' value.
// 3. Providing invalid 'start_block_height' value.
func (s *EventsProviderSuite) invalidArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "provide both 'start_block_id' and 'start_block_height' arguments",
			arguments: map[string]string{
				"start_block_id":     s.rootBlock.ID().String(),
				"start_block_height": fmt.Sprintf("%d", s.rootBlock.Header.Height),
			},
			expectedErrorMsg: "can only provide either 'start_block_id' or 'start_block_height'",
		},
		{
			name: "invalid 'start_block_id' argument",
			arguments: map[string]string{
				"start_block_id": "invalid_block_id",
			},
			expectedErrorMsg: "invalid ID format",
		},
		{
			name: "invalid 'start_block_height' argument",
			arguments: map[string]string{
				"start_block_height": "-1",
			},
			expectedErrorMsg: "value must be an unsigned 64 bit integer",
		},
	}
}

// TestEventsDataProvider_InvalidArguments tests the behavior of the event data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Providing both 'start_block_id' and 'start_block_height' simultaneously.
// 2. Invalid 'start_block_id' argument.
// 3. Invalid 'start_block_height' argument.
func (s *EventsProviderSuite) TestEventsDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := EventsTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewEventsDataProvider(
				ctx,
				s.log,
				s.api,
				s.chain,
				state_stream.DefaultEventFilterConfig,
				topic,
				test.arguments,
				send)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// TODO: add tests for responses after the WebsocketController is ready

package data_providers

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
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

	factory *DataProviderFactoryImpl
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

	s.factory = NewDataProviderFactory(s.log, s.api, nil, flow.Testnet.Chain(), state_stream.DefaultEventFilterConfig)
	s.Require().NotNil(s.factory)
}

// subscribeEventsDataProviderTestCases generates test cases for events data providers.
func (s *EventsProviderSuite) subscribeEventsDataProviderTestCases() []testType {
	return []testType{
		{
			name: "SubscribeBlocksFromStartBlockID happy path",
			arguments: models.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeEventsFromStartBlockID",
					mock.Anything,
					s.rootBlock.ID(),
					mock.Anything,
				).Return(sub).Once()
			},
		},
		{
			name: "SubscribeEventsFromStartHeight happy path",
			arguments: models.Arguments{
				"start_block_height": strconv.FormatUint(s.rootBlock.Header.Height, 10),
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeEventsFromStartHeight",
					mock.Anything,
					s.rootBlock.Header.Height,
					mock.Anything,
				).Return(sub).Once()
			},
		},
		{
			name:      "SubscribeEventsFromLatest happy path",
			arguments: models.Arguments{},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeEventsFromLatest",
					mock.Anything,
					mock.Anything,
				).Return(sub).Once()
			},
		},
	}
}

// TestEventsDataProvider_HappyPath tests the behavior of the events data provider
// when it is configured correctly and operating under normal conditions. It
// validates that events are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *EventsProviderSuite) TestEventsDataProvider_HappyPath() {
	s.testHappyPath(
		EventsTopic,
		s.subscribeEventsDataProviderTestCases(),
		s.requireEvents,
	)
}

// testHappyPath tests a variety of scenarios for data providers in
// happy path scenarios. This function runs parameterized test cases that
// simulate various configurations and verifies that the data provider operates
// as expected without encountering errors.
//
// Arguments:
// - topic: The topic associated with the data provider.
// - tests: A slice of test cases to run, each specifying setup and validation logic.
// - requireFn: A function to validate the output received in the send channel.
func (s *EventsProviderSuite) testHappyPath(
	topic string,
	tests []testType,
	requireFn func(interface{}, *flow.Event),
) {
	expectedEvents := []flow.Event{
		unittest.EventFixture(flow.EventAccountCreated, 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture(flow.EventAccountUpdated, 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture(flow.EventAccountCreated, 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture(flow.EventAccountUpdated, 0, 0, unittest.IdentifierFixture(), 0),
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			ctx := context.Background()
			send := make(chan interface{}, 10)

			// Create a channel to simulate the subscription's data channel
			eventChan := make(chan interface{})

			//	// Create a mock subscription and mock the channel
			sub := ssmock.NewSubscription(s.T())
			sub.On("Channel").Return((<-chan interface{})(eventChan))
			sub.On("Err").Return(nil)
			test.setupBackend(sub)

			// Create the data provider instance
			provider, err := s.factory.NewDataProvider(ctx, topic, test.arguments, send)
			s.Require().NotNil(provider)
			s.Require().NoError(err)

			// Run the provider in a separate goroutine
			go func() {
				err = provider.Run()
				s.Require().NoError(err)
			}()

			// Simulate emitting data to the events channel
			go func() {
				defer close(eventChan)

				for i := 0; i < len(expectedEvents); i++ {
					eventChan <- &expectedEvents[i]
				}
			}()

			// Collect responses
			for _, e := range expectedEvents {
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-send
					s.Require().True(ok, "channel closed while waiting for event %v: err: %v", e.ID(), sub.Err())

					requireFn(v, &e)
				}, time.Second, fmt.Sprintf("timed out waiting for event %v ", e.ID()))
			}

			// Ensure the provider is properly closed after the test
			provider.Close()
		})
	}
}

// requireEvents ensures that the received event information matches the expected data.
func (s *EventsProviderSuite) requireEvents(v interface{}, expectedEvent *flow.Event) {
	actualResponse, ok := v.(*models.EventResponse)
	require.True(s.T(), ok, "Expected *models.EventResponse, got %T", v)

	s.Require().Equal(expectedEvent, actualResponse.Event)
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
			arguments: models.Arguments{
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
				topic,
				test.arguments,
				send,
				s.chain,
				state_stream.DefaultEventFilterConfig)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// TestMessageIndexEventProviderResponse_HappyPath tests that MessageIndex values in response are strictly increasing.
func (s *EventsProviderSuite) TestMessageIndexEventProviderResponse_HappyPath() {
	ctx := context.Background()
	send := make(chan interface{}, 10)
	topic := EventsTopic
	eventsCount := 4

	// Create a channel to simulate the subscription's event channel
	eventChan := make(chan interface{})

	// Create a mock subscription and mock the channel
	sub := ssmock.NewSubscription(s.T())
	sub.On("Channel").Return((<-chan interface{})(eventChan))
	sub.On("Err").Return(nil)

	s.api.On("SubscribeEventsFromStartBlockID", mock.Anything, mock.Anything, mock.Anything).Return(sub)

	arguments :=
		map[string]string{
			"start_block_id": s.rootBlock.ID().String(),
		}

	// Create the EventsDataProvider instance
	provider, err := NewEventsDataProvider(
		ctx,
		s.log,
		s.api,
		topic,
		arguments,
		send,
		s.chain,
		state_stream.DefaultEventFilterConfig)
	s.Require().NotNil(provider)
	s.Require().NoError(err)

	// Run the provider in a separate goroutine to simulate subscription processing
	go func() {
		err = provider.Run()
		s.Require().NoError(err)
	}()

	// Simulate emitting events to the event channel
	go func() {
		defer close(eventChan) // Close the channel when done

		for i := 0; i < eventsCount; i++ {
			eventChan <- &flow.Event{
				Type: "flow.AccountCreated",
			}
		}
	}()

	// Collect responses
	var responses []*models.EventResponse
	for i := 0; i < eventsCount; i++ {
		res := <-send
		eventRes, ok := res.(*models.EventResponse)
		s.Require().True(ok, "Expected *models.EventResponse, got %T", res)
		responses = append(responses, eventRes)
	}

	// Verifying that indices are starting from 1
	s.Require().Equal("1", responses[0].MessageIndex, "Expected MessageIndex to start with 1")

	// Verifying that indices are strictly increasing
	for i := 1; i < len(responses); i++ {
		prevIndex, _ := strconv.Atoi(responses[i-1].MessageIndex)
		currentIndex, _ := strconv.Atoi(responses[i].MessageIndex)
		s.Require().Equal(prevIndex+1, currentIndex, "Expected MessageIndex to increment by 1")
	}

	// Ensure the provider is properly closed after the test
	provider.Close()
}

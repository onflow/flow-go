package data_providers

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
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

	s.factory = NewDataProviderFactory(
		s.log,
		s.api,
		nil,
		s.chain,
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
		nil,
	)
	s.Require().NotNil(s.factory)
}

// TestEventsDataProvider_HappyPath tests the behavior of the events data provider
// when it is configured correctly and operating under normal conditions. It
// validates that events are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *EventsProviderSuite) TestEventsDataProvider_HappyPath() {
	events := []flow.Event{
		unittest.EventFixture(flow.EventAccountCreated, 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture(flow.EventAccountUpdated, 0, 0, unittest.IdentifierFixture(), 0),
	}

	backendResponses := s.backendEventsResponses(events)

	testHappyPath(
		s.T(),
		EventsTopic,
		s.factory,
		s.subscribeEventsDataProviderTestCases(backendResponses),
		func(dataChan chan interface{}) {
			for i := 0; i < len(backendResponses); i++ {
				dataChan <- backendResponses[i]
			}
		},
		s.requireEvents,
	)
}

// subscribeEventsDataProviderTestCases generates test cases for events data providers.
func (s *EventsProviderSuite) subscribeEventsDataProviderTestCases(backendResponses []*backend.EventsResponse) []testType {
	expectedResponses := s.expectedEventsResponses(backendResponses)

	return []testType{
		{
			name: "SubscribeBlocksFromStartBlockID happy path",
			arguments: wsmodels.Arguments{
				"start_block_id":     s.rootBlock.ID().String(),
				"event_types":        []string{string(flow.EventAccountCreated)},
				"addresses":          []string{unittest.AddressFixture().String()},
				"contracts":          []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
				"heartbeat_interval": "3",
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeEventsFromStartBlockID",
					mock.Anything,
					s.rootBlock.ID(),
					mock.Anything,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "SubscribeEventsFromStartHeight happy path",
			arguments: wsmodels.Arguments{
				"start_block_height": strconv.FormatUint(s.rootBlock.Header.Height, 10),
				"event_types":        []string{string(flow.EventAccountCreated)},
				"addresses":          []string{unittest.AddressFixture().String()},
				"contracts":          []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
				"heartbeat_interval": "3",
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeEventsFromStartHeight",
					mock.Anything,
					s.rootBlock.Header.Height,
					mock.Anything,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "SubscribeEventsFromLatest happy path",
			arguments: wsmodels.Arguments{
				"event_types":        []string{string(flow.EventAccountCreated)},
				"addresses":          []string{unittest.AddressFixture().String()},
				"contracts":          []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
				"heartbeat_interval": "3",
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeEventsFromLatest",
					mock.Anything,
					mock.Anything,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
	}
}

// requireEvents ensures that the received event information matches the expected data.
func (s *EventsProviderSuite) requireEvents(actual interface{}, expected interface{}) {
	expectedResponse, expectedResponsePayload := extractPayload[*models.EventResponse](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*models.EventResponse](s.T(), actual)

	s.Require().Equal(expectedResponse.Topic, actualResponse.Topic)
	s.Require().Equal(expectedResponsePayload.MessageIndex, actualResponsePayload.MessageIndex)
	s.Require().ElementsMatch(expectedResponsePayload.Events, actualResponsePayload.Events)
}

// backendEventsResponses creates backend events responses based on the provided events.
func (s *EventsProviderSuite) backendEventsResponses(events []flow.Event) []*backend.EventsResponse {
	responses := make([]*backend.EventsResponse, len(events))

	for i := range events {
		responses[i] = &backend.EventsResponse{
			Height:         s.rootBlock.Header.Height,
			BlockID:        s.rootBlock.ID(),
			Events:         events,
			BlockTimestamp: s.rootBlock.Header.Timestamp,
		}
	}

	return responses
}

// expectedEventsResponses creates the expected responses for the provided backend responses.
func (s *EventsProviderSuite) expectedEventsResponses(
	backendResponses []*backend.EventsResponse,
) []interface{} {
	expectedResponses := make([]interface{}, len(backendResponses))

	for i, resp := range backendResponses {
		expectedResponsePayload := models.NewEventResponse(resp, uint64(i))
		expectedResponses[i] = &models.BaseDataProvidersResponse{
			Topic:   EventsTopic,
			Payload: expectedResponsePayload,
		}
	}
	return expectedResponses
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
	sub.On("Err").Return(nil).Once()

	s.api.On("SubscribeEventsFromStartBlockID", mock.Anything, mock.Anything, mock.Anything).Return(sub)

	arguments :=
		map[string]interface{}{
			"start_block_id": s.rootBlock.ID().String(),
			"event_types":    []string{state_stream.CoreEventAccountCreated},
			"addresses":      []string{unittest.AddressFixture().String()},
			"contracts":      []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
		}

	// Create the EventsDataProvider instance
	provider, err := NewEventsDataProvider(
		s.log,
		s.api,
		"dummy-id",
		topic,
		arguments,
		send,
		s.chain,
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
	)

	s.Require().NoError(err)
	s.Require().NotNil(provider)

	// Ensure the provider is properly closed after the test
	defer provider.Close()

	// Run the provider in a separate goroutine to simulate subscription processing
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = provider.Run(ctx)
		s.Require().NoError(err)
	}()

	// Simulate emitting events to the event channel
	go func() {
		defer close(eventChan) // Close the channel when done

		for i := 0; i < eventsCount; i++ {
			eventChan <- &backend.EventsResponse{
				Height: s.rootBlock.Header.Height,
			}
		}
	}()

	// Collect responses
	var responses []*models.EventResponse
	for i := 0; i < eventsCount; i++ {
		res := <-send

		_, eventResData := extractPayload[*models.EventResponse](s.T(), res)

		responses = append(responses, eventResData)
	}

	// Wait for the provider goroutine to finish
	unittest.RequireCloseBefore(s.T(), done, time.Second, "provider failed to stop")

	// Verifying that indices are starting from 0
	s.Require().Equal(uint64(0), responses[0].MessageIndex, "Expected MessageIndex to start with 0")

	// Verifying that indices are strictly increasing
	for i := 1; i < len(responses); i++ {
		prevIndex := responses[i-1].MessageIndex
		currentIndex := responses[i].MessageIndex
		s.Require().Equal(prevIndex+1, currentIndex, "Expected MessageIndex to increment by 1")
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
	send := make(chan interface{})

	topic := EventsTopic

	for _, test := range invalidEventsArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewEventsDataProvider(
				s.log,
				s.api,
				"dummy-id",
				topic,
				test.arguments,
				send,
				s.chain,
				state_stream.DefaultEventFilterConfig,
				subscription.DefaultHeartbeatInterval,
			)
			s.Require().Error(err)
			s.Require().Nil(provider)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

func (s *EventsProviderSuite) TestEventsDataProvider_StateStreamNotConfigured() {
	send := make(chan interface{})

	topic := EventsTopic

	provider, err := NewEventsDataProvider(
		s.log,
		nil,
		"dummy-id",
		topic,
		wsmodels.Arguments{},
		send,
		s.chain,
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
	)
	s.Require().Error(err)
	s.Require().Nil(provider)
	s.Require().Contains(err.Error(), "does not support streaming events")
}

// invalidEventsArgumentsTestCases returns a list of test cases with invalid argument combinations
// for testing the behavior of events data providers. Each test case includes a name,
// a set of input arguments, and the expected error message that should be returned.
func invalidEventsArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "provide both 'start_block_id' and 'start_block_height' arguments",
			arguments: wsmodels.Arguments{
				"start_block_id":     unittest.BlockFixture().ID().String(),
				"start_block_height": fmt.Sprintf("%d", unittest.BlockFixture().Header.Height),
				"event_types":        []string{state_stream.CoreEventAccountCreated},
				"addresses":          []string{unittest.AddressFixture().String()},
				"contracts":          []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
			},
			expectedErrorMsg: "can only provide either 'start_block_id' or 'start_block_height'",
		},
		{
			name: "invalid 'start_block_id' argument",
			arguments: map[string]interface{}{
				"start_block_id": "invalid_block_id",
				"event_types":    []string{state_stream.CoreEventAccountCreated},
				"addresses":      []string{unittest.AddressFixture().String()},
				"contracts":      []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
			},
			expectedErrorMsg: "invalid ID format",
		},
		{
			name: "invalid 'start_block_height' argument",
			arguments: map[string]interface{}{
				"start_block_height": "-1",
				"event_types":        []string{state_stream.CoreEventAccountCreated},
				"addresses":          []string{unittest.AddressFixture().String()},
				"contracts":          []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
			},
			expectedErrorMsg: "'start_block_height' must be convertible to uint64",
		},
		{
			name: "invalid 'heartbeat_interval' argument",
			arguments: map[string]interface{}{
				"start_block_id":     unittest.BlockFixture().ID().String(),
				"event_types":        []string{state_stream.CoreEventAccountCreated},
				"addresses":          []string{unittest.AddressFixture().String()},
				"contracts":          []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
				"heartbeat_interval": "-1",
			},
			expectedErrorMsg: "'heartbeat_interval' must be convertible to uint64",
		},
		{
			name: "unexpected argument",
			arguments: map[string]interface{}{
				"start_block_id":      unittest.BlockFixture().ID().String(),
				"event_types":         []string{state_stream.CoreEventAccountCreated},
				"addresses":           []string{unittest.AddressFixture().String()},
				"contracts":           []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
				"unexpected_argument": "dummy",
			},
			expectedErrorMsg: "unexpected field: 'unexpected_argument'",
		},
	}
}

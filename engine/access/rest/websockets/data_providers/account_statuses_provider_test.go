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
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// AccountStatusesProviderSuite is a test suite for testing the account statuses providers functionality.
type AccountStatusesProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api *ssmock.API

	chain          flow.Chain
	rootBlock      flow.Block
	finalizedBlock *flow.Header

	factory *DataProviderFactoryImpl
}

func TestNewAccountStatusesDataProvider(t *testing.T) {
	suite.Run(t, new(AccountStatusesProviderSuite))
}

func (s *AccountStatusesProviderSuite) SetupTest() {
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

// TestAccountStatusesDataProvider_HappyPath tests the behavior of the account statuses data provider
// when it is configured correctly and operating under normal conditions. It
// validates that events are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *AccountStatusesProviderSuite) TestAccountStatusesDataProvider_HappyPath() {
	events := []flow.Event{
		unittest.EventFixture(state_stream.CoreEventAccountCreated, 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture(state_stream.CoreEventAccountKeyAdded, 0, 0, unittest.IdentifierFixture(), 0),
	}

	backendResponses := s.backendAccountStatusesResponses(events)

	testHappyPath(
		s.T(),
		AccountStatusesTopic,
		s.factory,
		s.subscribeAccountStatusesDataProviderTestCases(backendResponses),
		func(dataChan chan interface{}) {
			for i := 0; i < len(backendResponses); i++ {
				dataChan <- backendResponses[i]
			}
		},
		s.requireAccountStatuses,
	)
}

func (s *AccountStatusesProviderSuite) TestAccountStatusesDataProvider_StateStreamNotConfigured() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := AccountStatusesTopic

	provider, err := NewAccountStatusesDataProvider(
		ctx,
		s.log,
		nil,
		"dummy-id",
		topic,
		models.Arguments{},
		send,
		s.chain,
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
	)
	s.Require().Error(err)
	s.Require().Nil(provider)
	s.Require().Contains(err.Error(), "does not support streaming account statuses")
}

func (s *AccountStatusesProviderSuite) subscribeAccountStatusesDataProviderTestCases(
	backendResponses []*backend.AccountStatusesResponse,
) []testType {
	expectedResponses := s.expectedAccountStatusesResponses(backendResponses)

	return []testType{
		{
			name: "SubscribeAccountStatusesFromStartBlockID happy path",
			arguments: models.Arguments{
				"start_block_id":    s.rootBlock.ID().String(),
				"event_types":       []string{string(flow.EventAccountCreated)},
				"account_addresses": []string{unittest.AddressFixture().String()},
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeAccountStatusesFromStartBlockID",
					mock.Anything,
					s.rootBlock.ID(),
					mock.Anything,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "SubscribeAccountStatusesFromStartHeight happy path",
			arguments: models.Arguments{
				"start_block_height": strconv.FormatUint(s.rootBlock.Header.Height, 10),
				"event_types":        []string{string(flow.EventAccountCreated)},
				"account_addresses":  []string{unittest.AddressFixture().String()},
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeAccountStatusesFromStartHeight",
					mock.Anything,
					s.rootBlock.Header.Height,
					mock.Anything,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "SubscribeAccountStatusesFromLatestBlock happy path",
			arguments: models.Arguments{
				"event_types":       []string{string(flow.EventAccountCreated)},
				"account_addresses": []string{unittest.AddressFixture().String()},
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeAccountStatusesFromLatestBlock",
					mock.Anything,
					mock.Anything,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
	}
}

// requireAccountStatuses ensures that the received account statuses information matches the expected data.
func (s *AccountStatusesProviderSuite) requireAccountStatuses(actual interface{}, expected interface{}) {
	expectedResponse, expectedResponsePayload := extractPayload[*models.AccountStatusesResponse](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*models.AccountStatusesResponse](s.T(), actual)

	require.Equal(s.T(), expectedResponsePayload.BlockID, actualResponsePayload.BlockID)
	require.Equal(s.T(), len(expectedResponsePayload.AccountEvents), len(actualResponsePayload.AccountEvents))
	require.Equal(s.T(), expectedResponsePayload.MessageIndex, actualResponsePayload.MessageIndex)
	require.Equal(s.T(), expectedResponsePayload.Height, actualResponsePayload.Height)
	require.Equal(s.T(), expectedResponse.Topic, actualResponse.Topic)

	for key, expectedEvents := range expectedResponsePayload.AccountEvents {
		actualEvents, ok := actualResponsePayload.AccountEvents[key]
		require.True(s.T(), ok, "Missing key in actual AccountEvents: %s", key)

		s.Require().Equal(expectedEvents, actualEvents, "Mismatch for key: %s", key)
	}
}

// expectedAccountStatusesResponses creates the expected responses for the provided events and backend responses.
func (s *AccountStatusesProviderSuite) expectedAccountStatusesResponses(backendResponses []*backend.AccountStatusesResponse) []interface{} {
	expectedResponses := make([]interface{}, len(backendResponses))

	for i, resp := range backendResponses {
		var expectedResponsePayload models.AccountStatusesResponse
		expectedResponsePayload.Build(resp, uint64(i))

		expectedResponses[i] = &models.BaseDataProvidersResponse{
			Topic:   AccountStatusesTopic,
			Payload: &expectedResponsePayload,
		}
	}

	return expectedResponses
}

// TestAccountStatusesDataProvider_InvalidArguments tests the behavior of the account statuses data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Providing both 'start_block_id' and 'start_block_height' simultaneously.
// 2. Invalid 'start_block_id' argument.
// 3. Invalid 'start_block_height' argument.
func (s *AccountStatusesProviderSuite) TestAccountStatusesDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := AccountStatusesTopic

	for _, test := range invalidAccountStatusesArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewAccountStatusesDataProvider(
				ctx,
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

// TestMessageIndexAccountStatusesProviderResponse_HappyPath tests that MessageIndex values in response are strictly increasing.
func (s *AccountStatusesProviderSuite) TestMessageIndexAccountStatusesProviderResponse_HappyPath() {
	ctx := context.Background()
	send := make(chan interface{}, 10)
	topic := AccountStatusesTopic
	accountStatusesCount := 4

	// Create a channel to simulate the subscription's account statuses channel
	accountStatusesChan := make(chan interface{})

	// Create a mock subscription and mock the channel
	sub := ssmock.NewSubscription(s.T())
	sub.On("Channel").Return((<-chan interface{})(accountStatusesChan))
	sub.On("Err").Return(nil).Once()

	s.api.On("SubscribeAccountStatusesFromStartBlockID", mock.Anything, mock.Anything, mock.Anything).Return(sub)

	arguments :=
		map[string]interface{}{
			"start_block_id":    s.rootBlock.ID().String(),
			"event_types":       []string{string(flow.EventAccountCreated)},
			"account_addresses": []string{unittest.AddressFixture().String()},
		}

	// Create the AccountStatusesDataProvider instance
	provider, err := NewAccountStatusesDataProvider(
		ctx,
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
		err = provider.Run()
		s.Require().NoError(err)
	}()

	// Simulate emitting data to the account statuses channel
	go func() {
		defer close(accountStatusesChan) // Close the channel when done

		for i := 0; i < accountStatusesCount; i++ {
			accountStatusesChan <- &backend.AccountStatusesResponse{}
		}
	}()

	// Collect responses
	var responses []*models.AccountStatusesResponse
	for i := 0; i < accountStatusesCount; i++ {
		res := <-send

		_, accStatusesResponsePayload := extractPayload[*models.AccountStatusesResponse](s.T(), res)

		responses = append(responses, accStatusesResponsePayload)
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

// backendAccountStatusesResponses creates backend account statuses responses based on the provided events.
func (s *AccountStatusesProviderSuite) backendAccountStatusesResponses(events []flow.Event) []*backend.AccountStatusesResponse {
	responses := make([]*backend.AccountStatusesResponse, len(events))

	for i := range events {
		responses[i] = &backend.AccountStatusesResponse{
			Height:  s.rootBlock.Header.Height,
			BlockID: s.rootBlock.ID(),
			AccountEvents: map[string]flow.EventsList{
				unittest.RandomAddressFixture().String(): events,
			},
		}
	}

	return responses
}

func invalidAccountStatusesArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "provide both 'start_block_id' and 'start_block_height' arguments",
			arguments: models.Arguments{
				"start_block_id":     unittest.BlockFixture().ID().String(),
				"start_block_height": fmt.Sprintf("%d", unittest.BlockFixture().Header.Height),
				"event_types":        []string{state_stream.CoreEventAccountCreated},
				"account_addresses":  []string{unittest.AddressFixture().String()},
			},
			expectedErrorMsg: "can only provide either 'start_block_id' or 'start_block_height'",
		},
		{
			name: "invalid 'start_block_id' argument",
			arguments: map[string]interface{}{
				"start_block_id":    "invalid_block_id",
				"event_types":       []string{state_stream.CoreEventAccountCreated},
				"account_addresses": []string{unittest.AddressFixture().String()},
			},
			expectedErrorMsg: "invalid ID format",
		},
		{
			name: "invalid 'start_block_height' argument",
			arguments: map[string]interface{}{
				"start_block_height": "-1",
				"event_types":        []string{state_stream.CoreEventAccountCreated},
				"account_addresses":  []string{unittest.AddressFixture().String()},
			},
			expectedErrorMsg: "'start_block_height' must be convertable to uint64",
		},
		{
			name: "invalid 'heartbeat_interval' argument",
			arguments: map[string]interface{}{
				"start_block_id":     unittest.BlockFixture().ID().String(),
				"event_types":        []string{state_stream.CoreEventAccountCreated},
				"account_addresses":  []string{unittest.AddressFixture().String()},
				"heartbeat_interval": "-1",
			},
			expectedErrorMsg: "'heartbeat_interval' must be convertable to uint64",
		},
		{
			name: "unexpected argument",
			arguments: map[string]interface{}{
				"start_block_id":      unittest.BlockFixture().ID().String(),
				"event_types":         []string{state_stream.CoreEventAccountCreated},
				"account_addresses":   []string{unittest.AddressFixture().String()},
				"unexpected_argument": "dummy",
			},
			expectedErrorMsg: "unexpected field: 'unexpected_argument'",
		},
	}
}

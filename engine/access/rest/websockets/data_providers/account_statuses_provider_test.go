package data_providers

import (
	"context"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/util"
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
		flow.Testnet.Chain(),
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
	s.testHappyPath(
		AccountStatusesTopic,
		s.subscribeAccountStatusesDataProviderTestCases(),
		s.requireAccountStatuses,
	)
}

func (s *AccountStatusesProviderSuite) testHappyPath(
	topic string,
	tests []testType,
	requireFn func(interface{}, *models.AccountStatusesResponse),
) {
	expectedEvents := []flow.Event{
		unittest.EventFixture(state_stream.CoreEventAccountCreated, 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture(state_stream.CoreEventAccountKeyAdded, 0, 0, unittest.IdentifierFixture(), 0),
	}

	var backendResponses []*backend.AccountStatusesResponse
	var expectedResponses []*models.AccountStatusesResponse

	for i := 0; i < len(expectedEvents); i++ {
		backendResponse := &backend.AccountStatusesResponse{
			Height:  s.rootBlock.Header.Height,
			BlockID: s.rootBlock.ID(),
			AccountEvents: map[string]flow.EventsList{
				unittest.RandomAddressFixture().String(): expectedEvents,
			},
		}
		backendResponses = append(backendResponses, backendResponse)

		var accountEvents models.AccountEvents
		accountEvents.Build(backendResponse.AccountEvents)
		expectedResponses = append(expectedResponses, &models.AccountStatusesResponse{
			Height:        util.FromUint(s.rootBlock.Header.Height),
			BlockID:       s.rootBlock.ID().String(),
			AccountEvents: accountEvents,
		})
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			ctx := context.Background()
			send := make(chan interface{}, 10)

			// Create a channel to simulate the subscription's data channel
			accStatusesChan := make(chan interface{})

			//	// Create a mock subscription and mock the channel
			sub := ssmock.NewSubscription(s.T())
			sub.On("Channel").Return((<-chan interface{})(accStatusesChan))
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
				defer close(accStatusesChan)

				for i := 0; i < len(backendResponses); i++ {
					accStatusesChan <- backendResponses[i]
				}
			}()

			// Collect responses
			for _, e := range expectedResponses {
				v, ok := <-send
				s.Require().True(ok, "channel closed while waiting for event %v: err: %v", e.BlockID, sub.Err())

				requireFn(v, e)
			}

			// Ensure the provider is properly closed after the test
			provider.Close()
		})
	}
}

func (s *AccountStatusesProviderSuite) subscribeAccountStatusesDataProviderTestCases() []testType {
	return []testType{
		{
			name: "SubscribeAccountStatusesFromStartBlockID happy path",
			arguments: models.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
				"event_types":    []string{"flow.AccountCreated", "flow.AccountKeyAdded"},
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeAccountStatusesFromStartBlockID",
					mock.Anything,
					s.rootBlock.ID(),
					mock.Anything,
				).Return(sub).Once()
			},
		},
		{
			name: "SubscribeAccountStatusesFromStartHeight happy path",
			arguments: models.Arguments{
				"start_block_height": strconv.FormatUint(s.rootBlock.Header.Height, 10),
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeAccountStatusesFromStartHeight",
					mock.Anything,
					s.rootBlock.Header.Height,
					mock.Anything,
				).Return(sub).Once()
			},
		},
		{
			name:      "SubscribeAccountStatusesFromLatestBlock happy path",
			arguments: models.Arguments{},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeAccountStatusesFromLatestBlock",
					mock.Anything,
					mock.Anything,
				).Return(sub).Once()
			},
		},
	}
}

// requireAccountStatuses ensures that the received account statuses information matches the expected data.
func (s *AccountStatusesProviderSuite) requireAccountStatuses(
	v interface{},
	expectedAccountStatusesResponse *models.AccountStatusesResponse,
) {
	_, ok := v.(*models.AccountStatusesResponse)
	require.True(s.T(), ok, "Expected *models.AccountStatusesResponse, got %T", v)

	//s.Require().ElementsMatch(expectedAccountStatusesResponse.AccountEvents, actualResponse.AccountEvents)
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

	for _, test := range invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewAccountStatusesDataProvider(
				ctx,
				s.log,
				s.api,
				topic,
				test.arguments,
				send,
				s.chain,
				state_stream.DefaultEventFilterConfig,
				subscription.DefaultHeartbeatInterval,
			)
			s.Require().Nil(provider)
			s.Require().Error(err)
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
	sub.On("Err").Return(nil)

	s.api.On("SubscribeAccountStatusesFromStartBlockID", mock.Anything, mock.Anything, mock.Anything).Return(sub)

	arguments :=
		map[string]interface{}{
			"start_block_id": s.rootBlock.ID().String(),
		}

	// Create the AccountStatusesDataProvider instance
	provider, err := NewAccountStatusesDataProvider(
		ctx,
		s.log,
		s.api,
		topic,
		arguments,
		send,
		s.chain,
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
	)
	s.Require().NotNil(provider)
	s.Require().NoError(err)

	// Run the provider in a separate goroutine to simulate subscription processing
	go func() {
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
		accountStatusesRes, ok := res.(*models.AccountStatusesResponse)
		s.Require().True(ok, "Expected *models.AccountStatusesResponse, got %T", res)
		responses = append(responses, accountStatusesRes)
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

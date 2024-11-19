package data_providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	statestreammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// DataProviderFactorySuite is a test suite for testing the DataProviderFactory functionality.
type DataProviderFactorySuite struct {
	suite.Suite

	ctx context.Context
	ch  chan interface{}

	accessApi      *accessmock.API
	stateStreamApi *statestreammock.API

	factory *DataProviderFactory
}

func TestDataProviderFactorySuite(t *testing.T) {
	suite.Run(t, new(DataProviderFactorySuite))
}

// SetupTest sets up the initial context and dependencies for each test case.
// It initializes the factory with mock instances and validates that it is created successfully.
func (s *DataProviderFactorySuite) SetupTest() {
	log := unittest.Logger()
	s.stateStreamApi = statestreammock.NewAPI(s.T())
	s.accessApi = accessmock.NewAPI(s.T())

	s.ctx = context.Background()
	s.ch = make(chan interface{})

	s.factory = NewDataProviderFactory(log, s.stateStreamApi, s.accessApi)
	s.Require().NotNil(s.factory)
}

// setupSubscription creates a mock subscription instance for testing purposes.
// It configures the mock subscription's `ID` method to return a predefined subscription identifier.
// Additionally, it sets the return value of the specified API call to the mock subscription.
func (s *DataProviderFactorySuite) setupSubscription(apiCall *mock.Call) string {
	subscription := statestreammock.NewSubscription(s.T())
	subscriptionID := unittest.IdentifierFixture().String()
	subscription.On("ID").Return(subscriptionID).Once()

	apiCall.Return(subscription).Once()
	return subscriptionID
}

// TODO: add others topic to check when they will be implemented
// TestSupportedTopics verifies that supported topics return a valid provider and no errors.
// Each test case includes a topic and arguments for which a data provider should be created.
func (s *DataProviderFactorySuite) TestSupportedTopics() {
	// Define supported topics and check if each returns the correct provider without errors
	testCases := []struct {
		name               string
		topic              string
		arguments          map[string]string
		setupSubscription  func() string // return subscription id
		assertExpectations func()
	}{
		{
			name:      "block topic",
			topic:     BlocksTopic,
			arguments: map[string]string{"block_status": parser.Finalized},
			setupSubscription: func() string {
				return s.setupSubscription(s.accessApi.On("SubscribeBlocksFromLatest", mock.Anything, flow.BlockStatusFinalized))
			},
			assertExpectations: func() {
				s.accessApi.AssertExpectations(s.T())
			},
		},
		{
			name:      "block headers topic",
			topic:     BlockHeadersTopic,
			arguments: map[string]string{"block_status": parser.Finalized},
			setupSubscription: func() string {
				return s.setupSubscription(s.accessApi.On("SubscribeBlockHeadersFromLatest", mock.Anything, flow.BlockStatusFinalized))
			},
			assertExpectations: func() {
				s.accessApi.AssertExpectations(s.T())
			},
		},
		{
			name:      "block digests topic",
			topic:     BlockDigestsTopic,
			arguments: map[string]string{"block_status": parser.Finalized},
			setupSubscription: func() string {
				return s.setupSubscription(s.accessApi.On("SubscribeBlockDigestsFromLatest", mock.Anything, flow.BlockStatusFinalized))
			},
			assertExpectations: func() {
				s.accessApi.AssertExpectations(s.T())
			},
		},
	}

	for _, test := range testCases {
		s.Run(test.name, func() {
			subscriptionID := test.setupSubscription()

			provider, err := s.factory.NewDataProvider(s.ctx, test.topic, test.arguments, s.ch)
			s.Require().NotNil(provider, "Expected provider for topic %s", test.topic)
			s.Require().NoError(err, "Expected no error for topic %s", test.topic)

			s.Require().Equal(test.topic, provider.Topic())
			s.Require().Equal(subscriptionID, provider.ID())

			test.assertExpectations()
		})
	}
}

// TestUnsupportedTopics verifies that unsupported topics do not return a provider
// and instead return an error indicating the topic is unsupported.
func (s *DataProviderFactorySuite) TestUnsupportedTopics() {
	// Define unsupported topics
	unsupportedTopics := []string{
		"unknown_topic",
		"",
	}

	for _, topic := range unsupportedTopics {
		provider, err := s.factory.NewDataProvider(s.ctx, topic, nil, s.ch)
		s.Require().Nil(provider, "Expected no provider for unsupported topic %s", topic)
		s.Require().Error(err, "Expected error for unsupported topic %s", topic)
		s.Require().EqualError(err, fmt.Sprintf("unsupported topic \"%s\"", topic))
	}
}

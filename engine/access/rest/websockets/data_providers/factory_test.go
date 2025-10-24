package data_providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	submock "github.com/onflow/flow-go/engine/access/subscription/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// DataProviderFactorySuite is a test suite for testing the DataProviderFactory functionality.
type DataProviderFactorySuite struct {
	suite.Suite

	ctx context.Context
	ch  chan interface{}

	accessApi      *accessmock.API
	stateStreamApi *ssmock.API

	factory *DataProviderFactoryImpl
}

func TestDataProviderFactorySuite(t *testing.T) {
	suite.Run(t, new(DataProviderFactorySuite))
}

// SetupTest sets up the initial context and dependencies for each test case.
// It initializes the factory with mock instances and validates that it is created successfully.
func (s *DataProviderFactorySuite) SetupTest() {
	log := unittest.Logger()
	s.stateStreamApi = ssmock.NewAPI(s.T())
	s.accessApi = accessmock.NewAPI(s.T())

	s.ctx = context.Background()
	s.ch = make(chan interface{})

	s.factory = NewDataProviderFactory(
		log,
		s.stateStreamApi,
		s.accessApi,
		flow.Testnet.Chain(),
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
		nil,
	)
	s.Require().NotNil(s.factory)
}

// setupSubscription creates a mock subscription instance for testing purposes.
// It configures the return value of the specified API call to the mock subscription.
func (s *DataProviderFactorySuite) setupSubscription(apiCall *mock.Call) {
	sub := submock.NewSubscription(s.T())
	apiCall.Return(sub).Once()
}

// TestSupportedTopics verifies that supported topics return a valid provider and no errors.
// Each test case includes a topic and arguments for which a data provider should be created.
func (s *DataProviderFactorySuite) TestSupportedTopics() {
	// Define supported topics and check if each returns the correct provider without errors
	tx := unittest.TransactionBodyFixture()
	tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
	tx.Arguments = [][]uint8{}

	testCases := []struct {
		name               string
		topic              string
		arguments          wsmodels.Arguments
		setupSubscription  func()
		assertExpectations func()
	}{
		{
			name:              "block topic",
			topic:             BlocksTopic,
			arguments:         wsmodels.Arguments{"block_status": parser.Finalized},
			setupSubscription: func() {},
			assertExpectations: func() {
				s.accessApi.AssertExpectations(s.T())
			},
		},
		{
			name:              "block headers topic",
			topic:             BlockHeadersTopic,
			arguments:         wsmodels.Arguments{"block_status": parser.Finalized},
			setupSubscription: func() {},
			assertExpectations: func() {
				s.accessApi.AssertExpectations(s.T())
			},
		},
		{
			name:              "block digests topic",
			topic:             BlockDigestsTopic,
			arguments:         wsmodels.Arguments{"block_status": parser.Finalized},
			setupSubscription: func() {},
			assertExpectations: func() {
				s.accessApi.AssertExpectations(s.T())
			},
		},
		{
			name:  "events topic",
			topic: EventsTopic,
			arguments: wsmodels.Arguments{
				"event_types": []string{state_stream.CoreEventAccountCreated},
				"addresses":   []string{unittest.AddressFixture().String()},
				"contracts":   []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
			},
			setupSubscription: func() {},
			assertExpectations: func() {
				s.stateStreamApi.AssertExpectations(s.T())
			},
		},
		{
			name:  "account statuses topic",
			topic: AccountStatusesTopic,
			arguments: wsmodels.Arguments{
				"event_types":       []string{state_stream.CoreEventAccountCreated},
				"account_addresses": []string{unittest.AddressFixture().String()},
			},
			setupSubscription: func() {},
			assertExpectations: func() {
				s.stateStreamApi.AssertExpectations(s.T())
			},
		},
		{
			name:  "transaction statuses topic",
			topic: TransactionStatusesTopic,
			arguments: wsmodels.Arguments{
				"tx_id": unittest.IdentifierFixture().String(),
			},
			setupSubscription: func() {},
			assertExpectations: func() {
				s.stateStreamApi.AssertExpectations(s.T())
			},
		},
		{
			name:              "send transaction statuses topic",
			topic:             SendAndGetTransactionStatusesTopic,
			arguments:         wsmodels.Arguments(unittest.CreateSendTxHttpPayload(tx)),
			setupSubscription: func() {},
			assertExpectations: func() {
				s.stateStreamApi.AssertExpectations(s.T())
			},
		},
	}

	for _, test := range testCases {
		s.Run(test.name, func() {
			s.T().Parallel()
			test.setupSubscription()

			provider, err := s.factory.NewDataProvider(context.Background(), "dummy-id", test.topic, test.arguments, s.ch)
			s.Require().NoError(err, "Expected no error for topic %s", test.topic)
			s.Require().NotNil(provider, "Expected provider for topic %s", test.topic)
			s.Require().Equal(test.topic, provider.Topic())
			s.Require().Equal(test.arguments, provider.Arguments())

			test.assertExpectations()
		})
	}
}

// TestUnsupportedTopics verifies that unsupported topics do not return a provider
// and instead return an error indicating the topic is unsupported.
func (s *DataProviderFactorySuite) TestUnsupportedTopics() {
	s.T().Parallel()

	// Define unsupported topics
	unsupportedTopics := []string{
		"unknown_topic",
		"",
	}

	for _, topic := range unsupportedTopics {
		provider, err := s.factory.NewDataProvider(context.Background(), "dummy-id", topic, nil, s.ch)
		s.Require().Error(err, "Expected error for unsupported topic %s", topic)
		s.Require().Nil(provider, "Expected no provider for unsupported topic %s", topic)
		s.Require().EqualError(err, fmt.Sprintf("unsupported topic \"%s\"", topic))
	}
}

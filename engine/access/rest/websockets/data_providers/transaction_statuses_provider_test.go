package data_providers

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/access"
	accessmock "github.com/onflow/flow-go/access/mock"
	mockcommonmodels "github.com/onflow/flow-go/engine/access/rest/common/models/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

type TransactionStatusesProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api *accessmock.API

	chain          flow.Chain
	rootBlock      flow.Block
	finalizedBlock *flow.Header

	factory       *DataProviderFactoryImpl
	linkGenerator *mockcommonmodels.LinkGenerator
}

func TestNewTransactionStatusesDataProvider(t *testing.T) {
	suite.Run(t, new(TransactionStatusesProviderSuite))
}

func (s *TransactionStatusesProviderSuite) SetupTest() {
	s.log = unittest.Logger()
	s.api = accessmock.NewAPI(s.T())
	s.linkGenerator = mockcommonmodels.NewLinkGenerator(s.T())

	s.chain = flow.Testnet.Chain()

	s.rootBlock = unittest.BlockFixture()
	s.rootBlock.Header.Height = 0

	s.factory = NewDataProviderFactory(
		s.log,
		nil,
		s.api,
		s.chain,
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
		s.linkGenerator,
	)
	s.Require().NotNil(s.factory)
}

// TestTransactionStatusesDataProvider_HappyPath tests the behavior of the transaction statuses data provider
// when it is configured correctly and operating under normal conditions. It
// validates that tx statuses are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *TransactionStatusesProviderSuite) TestTransactionStatusesDataProvider_HappyPath() {
	backendResponse := backendTransactionStatusesResponse(s.rootBlock)

	s.linkGenerator.On("TransactionResultLink", mock.AnythingOfType("flow.Identifier")).Return(
		func(id flow.Identifier) (string, error) {
			return "some_link", nil
		},
	)

	testHappyPath(
		s.T(),
		TransactionStatusesTopic,
		s.factory,
		s.subscribeTransactionStatusesDataProviderTestCases(backendResponse),
		func(dataChan chan interface{}) {
			dataChan <- backendResponse
		},
		s.requireTransactionStatuses,
	)
}

func (s *TransactionStatusesProviderSuite) subscribeTransactionStatusesDataProviderTestCases(backendResponses []*access.TransactionResult) []testType {
	expectedResponses := s.expectedTransactionStatusesResponses(backendResponses, TransactionStatusesTopic)

	return []testType{
		{
			name:      "SubscribeTransactionStatuses happy path",
			arguments: wsmodels.Arguments{},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeTransactionStatuses",
					mock.Anything,
					mock.Anything,
					entities.EventEncodingVersion_JSON_CDC_V0,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
	}
}

// requireTransactionStatuses ensures that the received transaction statuses information matches the expected data.
func (s *TransactionStatusesProviderSuite) requireTransactionStatuses(
	actual interface{},
	expected interface{},
) {
	expectedResponse, expectedResponsePayload := extractPayload[*models.TransactionStatusesResponse](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*models.TransactionStatusesResponse](s.T(), actual)

	require.Equal(s.T(), expectedResponse.Topic, actualResponse.Topic)
	require.Equal(s.T(), expectedResponsePayload.TransactionResult.BlockId, actualResponsePayload.TransactionResult.BlockId)
}

func backendTransactionStatusesResponse(block flow.Block) []*access.TransactionResult {
	id := unittest.IdentifierFixture()
	cid := unittest.IdentifierFixture()
	txr := access.TransactionResult{
		Status:     flow.TransactionStatusSealed,
		StatusCode: 10,
		Events: []flow.Event{
			unittest.EventFixture(flow.EventAccountCreated, 1, 0, id, 200),
		},
		ErrorMessage: "",
		BlockID:      block.ID(),
		CollectionID: cid,
		BlockHeight:  block.Header.Height,
	}

	var expectedTxResultsResponses []*access.TransactionResult

	for i := 0; i < 2; i++ {
		expectedTxResultsResponses = append(expectedTxResultsResponses, &txr)
	}

	return expectedTxResultsResponses
}

// expectedTransactionStatusesResponses creates the expected responses for the provided backend responses.
func (s *TransactionStatusesProviderSuite) expectedTransactionStatusesResponses(
	backendResponses []*access.TransactionResult,
	topic string,
) []interface{} {
	expectedResponses := make([]interface{}, len(backendResponses))

	for i, resp := range backendResponses {
		expectedResponsePayload := models.NewTransactionStatusesResponse(s.linkGenerator, resp, uint64(i))
		expectedResponses[i] = &models.BaseDataProvidersResponse{
			Topic:   topic,
			Payload: &expectedResponsePayload,
		}
	}

	return expectedResponses
}

// TestMessageIndexTransactionStatusesProviderResponse_HappyPath tests that MessageIndex values in response are strictly increasing.
func (s *TransactionStatusesProviderSuite) TestMessageIndexTransactionStatusesProviderResponse_HappyPath() {
	ctx := context.Background()
	send := make(chan interface{}, 10)
	topic := TransactionStatusesTopic
	txStatusesCount := 4

	// Create a channel to simulate the subscription's account statuses channel
	txStatusesChan := make(chan interface{})

	// Create a mock subscription and mock the channel
	sub := ssmock.NewSubscription(s.T())
	sub.On("Channel").Return((<-chan interface{})(txStatusesChan))
	sub.On("Err").Return(nil).Once()

	s.api.On(
		"SubscribeTransactionStatuses",
		mock.Anything,
		mock.Anything,
		entities.EventEncodingVersion_JSON_CDC_V0,
	).Return(sub)

	s.linkGenerator.On("TransactionResultLink", mock.AnythingOfType("flow.Identifier")).Return(
		func(id flow.Identifier) (string, error) {
			return "some_link", nil
		},
	)

	arguments :=
		map[string]interface{}{
			"tx_id": unittest.TransactionFixture().ID().String(),
		}

	// Create the TransactionStatusesDataProvider instance
	provider, err := NewTransactionStatusesDataProvider(
		ctx,
		s.log,
		s.api,
		"dummy-id",
		s.linkGenerator,
		topic,
		arguments,
		send,
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

	// Simulate emitting data to the tx statuses channel
	var txResults []*access.TransactionResult
	for i := 0; i < txStatusesCount; i++ {
		txResults = append(txResults, &access.TransactionResult{
			BlockHeight: s.rootBlock.Header.Height,
		})
	}

	go func() {
		defer close(txStatusesChan) // Close the channel when done

		txStatusesChan <- txResults
	}()

	// Collect responses
	var responses []*models.TransactionStatusesResponse
	for i := 0; i < txStatusesCount; i++ {
		res := <-send
		_, txStatusesResData := extractPayload[*models.TransactionStatusesResponse](s.T(), res)
		responses = append(responses, txStatusesResData)
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

// TestTransactionStatusesDataProvider_InvalidArguments tests the behavior of the transaction statuses data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
func (s *TransactionStatusesProviderSuite) TestTransactionStatusesDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := TransactionStatusesTopic

	for _, test := range invalidTransactionStatusesArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewTransactionStatusesDataProvider(
				ctx,
				s.log,
				s.api,
				"dummy-id",
				s.linkGenerator,
				topic,
				test.arguments,
				send,
			)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// invalidTransactionStatusesArgumentsTestCases returns a list of test cases with invalid argument combinations
// for testing the behavior of transaction statuses data providers. Each test case includes a name,
// a set of input arguments, and the expected error message that should be returned.
//
// The test cases cover scenarios such as:
// 1. Providing both 'start_block_id' and 'start_block_height' simultaneously.
// 2. Providing invalid 'tx_id' value.
// 3. Providing invalid 'start_block_id'  value.
// 4. Invalid 'start_block_id' argument.
// 5. Providing unexpected argument.
func invalidTransactionStatusesArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "invalid 'tx_id' argument",
			arguments: map[string]interface{}{
				"tx_id": "invalid_tx_id",
			},
			expectedErrorMsg: "invalid ID format",
		},
		{
			name: "unexpected argument",
			arguments: map[string]interface{}{
				"unexpected_argument": "dummy",
				"tx_id":               unittest.TransactionFixture().ID().String(),
			},
			expectedErrorMsg: "unexpected field: 'unexpected_argument'",
		},
	}
}

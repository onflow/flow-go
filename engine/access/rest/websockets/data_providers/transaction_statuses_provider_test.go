package data_providers

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/access"
	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
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

	factory *DataProviderFactoryImpl
}

func TestNewTransactionStatusesDataProvider(t *testing.T) {
	suite.Run(t, new(TransactionStatusesProviderSuite))
}

func (s *TransactionStatusesProviderSuite) SetupTest() {
	s.log = unittest.Logger()
	s.api = accessmock.NewAPI(s.T())

	s.chain = flow.Testnet.Chain()

	s.rootBlock = unittest.BlockFixture()
	s.rootBlock.Header.Height = 0

	s.factory = NewDataProviderFactory(
		s.log,
		nil,
		s.api,
		s.chain,
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval)
	s.Require().NotNil(s.factory)
}

// TestTransactionStatusesDataProvider_HappyPath tests the behavior of the transaction statuses data provider
// when it is configured correctly and operating under normal conditions. It
// validates that tx statuses are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *TransactionStatusesProviderSuite) TestTransactionStatusesDataProvider_HappyPath() {
	expectedResponse := expectedTransactionStatusesResponse(s.rootBlock)

	testHappyPath(
		s.T(),
		TransactionStatusesTopic,
		s.factory,
		s.subscribeTransactionStatusesDataProviderTestCases(),
		func(dataChan chan interface{}) {
			dataChan <- expectedResponse
		},
		expectedResponse,
		s.requireTransactionStatuses,
	)
}

func (s *TransactionStatusesProviderSuite) subscribeTransactionStatusesDataProviderTestCases() []testType {
	return []testType{
		{
			name: "SubscribeTransactionStatusesFromStartBlockID happy path",
			arguments: models.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeTransactionStatusesFromStartBlockID",
					mock.Anything,
					mock.Anything,
					s.rootBlock.ID(),
					entities.EventEncodingVersion_JSON_CDC_V0,
				).Return(sub).Once()
			},
		},
		{
			name: "SubscribeTransactionStatusesFromStartHeight happy path",
			arguments: models.Arguments{
				"start_block_height": strconv.FormatUint(s.rootBlock.Header.Height, 10),
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeTransactionStatusesFromStartHeight",
					mock.Anything,
					mock.Anything,
					s.rootBlock.Header.Height,
					entities.EventEncodingVersion_JSON_CDC_V0,
				).Return(sub).Once()
			},
		},
		{
			name:      "SubscribeTransactionStatusesFromLatest happy path",
			arguments: models.Arguments{},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SubscribeTransactionStatusesFromLatest",
					mock.Anything,
					mock.Anything,
					entities.EventEncodingVersion_JSON_CDC_V0,
				).Return(sub).Once()
			},
		},
	}
}

// requireTransactionStatuses ensures that the received transaction statuses information matches the expected data.
func (s *TransactionStatusesProviderSuite) requireTransactionStatuses(
	v interface{},
	expectedResponse interface{},
) {
	expectedTxStatusesResponse, ok := expectedResponse.(*access.TransactionResult)
	require.True(s.T(), ok, "unexpected type: %T", expectedResponse)

	actualResponse, ok := v.(*models.TransactionStatusesResponse)
	require.True(s.T(), ok, "Expected *models.TransactionStatusesResponse, got %T", v)

	require.Equal(s.T(), expectedTxStatusesResponse.BlockID, actualResponse.TransactionResult.BlockID)
	require.Equal(s.T(), expectedTxStatusesResponse.BlockHeight, actualResponse.TransactionResult.BlockHeight)
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
func invalidTransactionStatusesArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "provide both 'start_block_id' and 'start_block_height' arguments",
			arguments: models.Arguments{
				"start_block_id":     unittest.BlockFixture().ID().String(),
				"start_block_height": fmt.Sprintf("%d", unittest.BlockFixture().Header.Height),
			},
			expectedErrorMsg: "can only provide either 'start_block_id' or 'start_block_height'",
		},
		{
			name: "invalid 'tx_id' argument",
			arguments: map[string]interface{}{
				"tx_id": "invalid_tx_id",
			},
			expectedErrorMsg: "invalid ID format",
		},
		{
			name: "invalid 'start_block_id' argument",
			arguments: map[string]interface{}{
				"start_block_id": "invalid_block_id",
			},
			expectedErrorMsg: "invalid ID format",
		},
		{
			name: "invalid 'start_block_height' argument",
			arguments: map[string]interface{}{
				"start_block_height": "-1",
			},
			expectedErrorMsg: "value must be an unsigned 64 bit integer",
		},
	}
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
	sub.On("Err").Return(nil)

	s.api.On(
		"SubscribeTransactionStatusesFromStartBlockID",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		entities.EventEncodingVersion_JSON_CDC_V0,
	).Return(sub)

	arguments :=
		map[string]interface{}{
			"start_block_id": s.rootBlock.ID().String(),
		}

	// Create the TransactionStatusesDataProvider instance
	provider, err := NewTransactionStatusesDataProvider(
		ctx,
		s.log,
		s.api,
		topic,
		arguments,
		send,
	)
	s.Require().NotNil(provider)
	s.Require().NoError(err)

	// Run the provider in a separate goroutine to simulate subscription processing
	go func() {
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
		txStatusesRes, ok := res.(*models.TransactionStatusesResponse)
		s.Require().True(ok, "Expected *models.TransactionStatusesResponse, got %T", res)
		responses = append(responses, txStatusesRes)
	}

	// Verifying that indices are starting from 0
	s.Require().Equal(uint64(0), responses[0].MessageIndex, "Expected MessageIndex to start with 0")

	// Verifying that indices are strictly increasing
	for i := 1; i < len(responses); i++ {
		prevIndex := responses[i-1].MessageIndex
		currentIndex := responses[i].MessageIndex
		s.Require().Equal(prevIndex+1, currentIndex, "Expected MessageIndex to increment by 1")
	}

	// Ensure the provider is properly closed after the test
	provider.Close()
}

func expectedTransactionStatusesResponse(block flow.Block) []*access.TransactionResult {
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
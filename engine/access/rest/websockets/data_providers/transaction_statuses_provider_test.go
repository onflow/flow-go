package data_providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/access"
	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

type TransactionStatusesProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api access.API

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
		flow.Testnet.Chain(),
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval)
	s.Require().NotNil(s.factory)
}

func (s *TransactionStatusesProviderSuite) TestTransactionStatusesDataProvider_HappyPath() {

	//testHappyPath(
	//	s.T(),
	//	AccountStatusesTopic,
	//	s.factory,
	//	s.subscribeTransactionStatusesDataProviderTestCases(),
	//	func(dataChan chan interface{}) {
	//		for i := 0; i < len(expectedAccountStatusesResponses); i++ {
	//			dataChan <- &expectedAccountStatusesResponses[i]
	//		}
	//	},
	//	expectedAccountStatusesResponses,
	//	s.requireAccountStatuses,
	//)

}

func (s *TransactionStatusesProviderSuite) subscribeTransactionStatusesDataProviderTestCases() []testType {
	return []testType{
		//{
		//	name: "SubscribeTransactionStatuses happy path",
		//	arguments: models.Arguments{
		//		"start_block_id": s.rootBlock.ID().String(),
		//		"event_types":    []string{"flow.AccountCreated", "flow.AccountKeyAdded"},
		//	},
		//	setupBackend: func(sub *ssmock.Subscription) {
		//		s.api.On(
		//			"SubscribeTransactionStatuses",
		//			mock.Anything,
		//			mock.Anything,
		//			s.rootBlock.ID(),
		//			entities.EventEncodingVersion_JSON_CDC_V0,
		//		).Return(sub).Once()
		//	},
		//},
	}
}

// requireAccountStatuses ensures that the received account statuses information matches the expected data.
func (s *AccountStatusesProviderSuite) requireTransactionStatuses(
	v interface{},
	expectedResponse interface{},
) {
	_, ok := expectedResponse.([]access.TransactionResult)
	require.True(s.T(), ok, "unexpected type: %T", expectedResponse)

}

// TestAccountStatusesDataProvider_InvalidArguments tests the behavior of the transaction statuses data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Invalid 'tx_id' argument.
// 2. Invalid 'start_block_id' argument.
func (s *TransactionStatusesProviderSuite) TestAccountStatusesDataProvider_InvalidArguments() {
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
// 1. Providing invalid 'tx_id' value.
// 2. Providing invalid 'start_block_id' value.
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

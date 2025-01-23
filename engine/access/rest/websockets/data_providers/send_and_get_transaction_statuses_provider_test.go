package data_providers

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/access/mock"
	mockcommonmodels "github.com/onflow/flow-go/engine/access/rest/common/models/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

type SendTransactionStatusesProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api *accessmock.API

	chain          flow.Chain
	rootBlock      flow.Block
	finalizedBlock *flow.Header

	factory       *DataProviderFactoryImpl
	linkGenerator *mockcommonmodels.LinkGenerator
}

func TestNewSendTransactionStatusesDataProvider(t *testing.T) {
	suite.Run(t, new(SendTransactionStatusesProviderSuite))
}

func (s *SendTransactionStatusesProviderSuite) SetupTest() {
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

// TestSendTransactionStatusesDataProvider_HappyPath tests the behavior of the send transaction statuses data provider
// when it is configured correctly and operating under normal conditions. It
// validates that tx statuses are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *TransactionStatusesProviderSuite) TestSendTransactionStatusesDataProvider_HappyPath() {
	s.linkGenerator.On("TransactionResultLink", mock.AnythingOfType("flow.Identifier")).Return(
		func(id flow.Identifier) (string, error) {
			return "some_link", nil
		},
	)

	backendResponse := backendTransactionStatusesResponse(s.rootBlock)
	expectedResponse := s.expectedTransactionStatusesResponses(backendResponse)

	sendTxStatutesTestCases := []testType{
		{
			name: "SubscribeTransactionStatusesFromStartBlockID happy path",
			arguments: models.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
			},
			setupBackend: func(sub *ssmock.Subscription) {
				s.api.On(
					"SendAndSubscribeTransactionStatuses",
					mock.Anything,
					mock.Anything,
					entities.EventEncodingVersion_JSON_CDC_V0,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponse,
		},
	}

	testHappyPath(
		s.T(),
		SendAndGetTransactionStatusesTopic,
		s.factory,
		sendTxStatutesTestCases,
		func(dataChan chan interface{}) {
			dataChan <- backendResponse
		},
		s.requireTransactionStatuses,
	)

}

// requireTransactionStatuses ensures that the received transaction statuses information matches the expected data.
func (s *SendTransactionStatusesProviderSuite) requireTransactionStatuses(
	v interface{},
	expectedResponse interface{},
) {
	expectedTxStatusesResponse, ok := expectedResponse.(*models.TransactionStatusesResponse)
	require.True(s.T(), ok, "expected *models.TransactionStatusesResponse, got %T", expectedResponse)

	actualResponse, ok := v.(*models.TransactionStatusesResponse)
	require.True(s.T(), ok, "expected *models.TransactionStatusesResponse, got %T", v)

	require.Equal(s.T(), expectedTxStatusesResponse.TransactionResult.BlockId, actualResponse.TransactionResult.BlockId)
}

// TestSendTransactionStatusesDataProvider_InvalidArguments tests the behavior of the send transaction statuses data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
func (s *SendTransactionStatusesProviderSuite) TestSendTransactionStatusesDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := SendAndGetTransactionStatusesTopic

	for _, test := range invalidSendTransactionStatusesArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewSendAndGetTransactionStatusesDataProvider(
				ctx,
				s.log,
				s.api,
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

// invalidSendTransactionStatusesArgumentsTestCases returns a list of test cases with invalid argument combinations
// for testing the behavior of send transaction statuses data providers. Each test case includes a name,
// a set of input arguments, and the expected error message that should be returned.
//
// The test cases cover scenarios such as:
// 1. Providing invalid 'script' type.
// 2. Providing invalid 'script' value.
// 3. Providing invalid 'arguments' type.
// 4. Providing invalid 'arguments' value.
// 5. Providing invalid 'reference_block_id' value.
// 6. Providing invalid 'gas_limit' value.
// 7. Providing invalid 'payer' value.
// 8. Providing invalid 'proposal_key' value.
// 9. Providing invalid 'authorizers' value.
// 10. Providing invalid 'payload_signatures' value.
// 11. Providing invalid 'envelope_signatures' value.
func invalidSendTransactionStatusesArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "invalid 'script' argument type",
			arguments: map[string]interface{}{
				"script": 0,
			},
			expectedErrorMsg: "'script' must be a string",
		},
		{
			name: "invalid 'script' argument",
			arguments: map[string]interface{}{
				"script": "invalid_script",
			},
			expectedErrorMsg: "invalid 'script': illegal base64 data ",
		},
		{
			name: "invalid 'arguments' type",
			arguments: map[string]interface{}{
				"arguments": 0,
			},
			expectedErrorMsg: "'arguments' must be a []string type",
		},
		{
			name: "invalid 'arguments' argument",
			arguments: map[string]interface{}{
				"arguments": []string{"invalid_base64_1", "invalid_base64_2"},
			},
			expectedErrorMsg: "invalid 'arguments'",
		},
		{
			name: "invalid 'reference_block_id' argument",
			arguments: map[string]interface{}{
				"reference_block_id": "invalid_reference_block_id",
			},
			expectedErrorMsg: "invalid ID format",
		},
		{
			name: "invalid 'gas_limit' argument",
			arguments: map[string]interface{}{
				"gas_limit": "-1",
			},
			expectedErrorMsg: "value must be an unsigned 64 bit integer",
		},
		{
			name: "invalid 'payer' argument",
			arguments: map[string]interface{}{
				"payer": "invalid_payer",
			},
			expectedErrorMsg: "invalid 'payer': can not decode hex string",
		},
		{
			name: "invalid 'proposal_key' argument",
			arguments: map[string]interface{}{
				"proposal_key": "invalid ProposalKey object",
			},
			expectedErrorMsg: "'proposal_key' must be a object (ProposalKey)",
		},
		{
			name: "invalid 'authorizers' argument",
			arguments: map[string]interface{}{
				"authorizers": []string{"invalid_base64_1", "invalid_base64_2"},
			},
			expectedErrorMsg: "invalid 'authorizers': can not decode hex string",
		},
		{
			name: "invalid 'payload_signatures' argument",
			arguments: map[string]interface{}{
				"payload_signatures": "invalid TransactionSignature array",
			},
			expectedErrorMsg: "'payload_signatures' must be an array of objects (TransactionSignature)",
		},
		{
			name: "invalid 'envelope_signatures' argument",
			arguments: map[string]interface{}{
				"envelope_signatures": "invalid TransactionSignature array",
			},
			expectedErrorMsg: "'envelope_signatures' must be an array of objects (TransactionSignature)",
		},
	}
}

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
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	submock "github.com/onflow/flow-go/engine/access/subscription/mock"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

type SendTransactionStatusesProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api *accessmock.API

	chain          flow.Chain
	rootBlock      *flow.Block
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

	s.rootBlock = unittest.BlockFixture(
		unittest.Block.WithHeight(0),
	)

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
	tx := unittest.TransactionBodyFixture()
	tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
	tx.Arguments = [][]uint8{}

	s.linkGenerator.On("TransactionResultLink", mock.AnythingOfType("flow.Identifier")).Return(
		func(id flow.Identifier) (string, error) {
			return "some_link", nil
		},
	)

	backendResponse := backendTransactionStatusesResponse(s.rootBlock)
	expectedResponse := s.expectedTransactionStatusesResponses(backendResponse, SendAndGetTransactionStatusesTopic)

	sendTxStatutesTestCases := []testType[[]*accessmodel.TransactionResult, any]{
		{
			name:      "SubscribeTransactionStatusesFromStartBlockID happy path",
			arguments: unittest.CreateSendTxHttpPayload(tx),
			setupBackend: func(sub *submock.Subscription[[]*accessmodel.TransactionResult]) {
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

	testHappyPath[[]*accessmodel.TransactionResult, any](
		s.T(),
		SendAndGetTransactionStatusesTopic,
		s.factory,
		sendTxStatutesTestCases,
		func(dataChan chan []*accessmodel.TransactionResult) {
			dataChan <- backendResponse
		},
		s.requireTransactionStatuses,
	)
}

// requireTransactionStatuses ensures that the received transaction statuses information matches the expected data.
func (s *SendTransactionStatusesProviderSuite) requireTransactionStatuses(
	actual any,
	expected any,
) {
	expectedResponse, expectedResponsePayload := extractPayload[*models.TransactionStatusesResponse](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*models.TransactionStatusesResponse](s.T(), actual)

	require.Equal(s.T(), expectedResponse.Topic, actualResponse.Topic)
	require.Equal(s.T(), expectedResponsePayload.TransactionResult.BlockId, actualResponsePayload.TransactionResult.BlockId)
}

// TestSendTransactionStatusesDataProvider_InvalidArguments tests the behavior of the send transaction statuses data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
func (s *SendTransactionStatusesProviderSuite) TestSendTransactionStatusesDataProvider_InvalidArguments() {
	send := make(chan any)
	topic := SendAndGetTransactionStatusesTopic

	for _, test := range invalidSendTransactionStatusesArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewSendAndGetTransactionStatusesDataProvider(
				context.Background(),
				s.log,
				s.api,
				"dummy-id",
				s.linkGenerator,
				topic,
				test.arguments,
				send,
				s.chain,
			)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
			s.Require().Nil(provider)
		})
	}
}

// invalidSendTransactionStatusesArgumentsTestCases returns a list of test cases with invalid argument combinations
// for testing the behavior of send transaction statuses data providers. Each test case includes a name,
// a set of input arguments, and the expected error message that should be returned.
func invalidSendTransactionStatusesArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "invalid 'script' argument type",
			arguments: map[string]any{
				"script": 0,
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'script' argument",
			arguments: map[string]any{
				"script": "invalid_script",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'arguments' type",
			arguments: map[string]any{
				"arguments": 0,
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'arguments' argument",
			arguments: map[string]any{
				"arguments": []string{"invalid_base64_1", "invalid_base64_2"},
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'reference_block_id' argument",
			arguments: map[string]any{
				"reference_block_id": "invalid_reference_block_id",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'gas_limit' argument",
			arguments: map[string]any{
				"gas_limit": "-1",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'payer' argument",
			arguments: map[string]any{
				"payer": "invalid_payer",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'proposal_key' argument",
			arguments: map[string]any{
				"proposal_key": "invalid ProposalKey object",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'authorizers' argument",
			arguments: map[string]any{
				"authorizers": []string{"invalid_base64_1", "invalid_base64_2"},
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'payload_signatures' argument",
			arguments: map[string]any{
				"payload_signatures": "invalid TransactionSignature array",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'envelope_signatures' argument",
			arguments: map[string]any{
				"envelope_signatures": "invalid TransactionSignature array",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "unexpected argument",
			arguments: map[string]any{
				"unexpected_argument": "dummy",
			},
			expectedErrorMsg: "request body contains unknown field",
		},
	}
}

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
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
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

	// s.rootBlock = unittest.BlockFixture(
	// 	 unittest.Block.WithHeight(0),
	// )
	//  code above was creating a regular block with height 0, which violates Flow's block validation rules. In Flow:
	//  - Genesis blocks (the first block in a chain) are special and can have height 0
	//  - Non-genesis blocks must have height > 0
	//
	//  When the test tried to call block.ID(), it internally calls block.ToHeader(), which validates the block header.
	//  The validation failed with:
	//  panic: could not build header from block: invalid header body: Height must be > 0 for non-root header

	s.rootBlock = unittest.Block.Genesis(s.chain.ChainID())

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
func (s *SendTransactionStatusesProviderSuite) TestSendTransactionStatusesDataProvider_HappyPath() {
	tx := unittest.TransactionFixture()
	tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
	tx.Arguments = [][]uint8{}

	s.linkGenerator.On("TransactionResultLink", mock.AnythingOfType("flow.Identifier")).Return(
		func(id flow.Identifier) (string, error) {
			return "some_link", nil
		},
	)

	backendResponse := backendTransactionStatusesResponse(s.rootBlock)
	expectedResponse := s.expectedTransactionStatusesResponses(backendResponse, SendAndGetTransactionStatusesTopic)

	sendTxStatutesTestCases := []testType{
		{
			name:      "SubscribeTransactionStatusesFromStartBlockID happy path",
			arguments: unittest.CreateSendTxHttpPayload(tx),
			setupBackend: func(sub *submock.Subscription) {
				s.api.On(
					"SendAndSubscribeTransactionStatuses",
					mock.Anything,
					mock.Anything,
					entities.EventEncodingVersion_JSON_CDC_V0,
					mock.Anything, // optimistic_sync.Criteria
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

// TestSendTransactionStatusesDataProvider_WithExecutionStateQuery tests that execution state query
// is properly extracted and passed to the API when provided in the request.
func (s *SendTransactionStatusesProviderSuite) TestSendTransactionStatusesDataProvider_WithExecutionStateQuery() {
	tx := unittest.TransactionFixture()
	tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
	tx.Arguments = [][]uint8{}

	executorID1 := unittest.IdentifierFixture()
	executorID2 := unittest.IdentifierFixture()

	s.linkGenerator.On("TransactionResultLink", mock.AnythingOfType("flow.Identifier")).Return(
		func(id flow.Identifier) (string, error) {
			return "some_link", nil
		},
	)

	backendResponse := backendTransactionStatusesResponse(s.rootBlock)
	expectedResponse := s.expectedTransactionStatusesResponses(backendResponse, SendAndGetTransactionStatusesTopic)

	testCases := []testType{
		{
			name: "with agreeing_executors_count",
			arguments: func() map[string]interface{} {
				args := unittest.CreateSendTxHttpPayload(tx)
				args["execution_state_query"] = map[string]interface{}{
					"agreeing_executors_count": float64(2),
				}
				return args
			}(),
			setupBackend: func(sub *submock.Subscription) {
				s.api.On(
					"SendAndSubscribeTransactionStatuses",
					mock.Anything,
					mock.Anything,
					entities.EventEncodingVersion_JSON_CDC_V0,
					mock.MatchedBy(func(criteria interface{}) bool {
						// Verify criteria has agreeing executors count set to 2
						c, ok := criteria.(optimistic_sync.Criteria)
						if !ok {
							return false
						}
						return c.AgreeingExecutorsCount == 2
					}),
				).Return(sub).Once()
			},
			expectedResponses: expectedResponse,
		},
		{
			name: "with required_executor_ids",
			arguments: func() map[string]interface{} {
				args := unittest.CreateSendTxHttpPayload(tx)
				args["execution_state_query"] = map[string]interface{}{
					"required_executor_ids": []interface{}{
						executorID1.String(),
						executorID2.String(),
					},
				}
				return args
			}(),
			setupBackend: func(sub *submock.Subscription) {
				s.api.On(
					"SendAndSubscribeTransactionStatuses",
					mock.Anything,
					mock.Anything,
					entities.EventEncodingVersion_JSON_CDC_V0,
					mock.MatchedBy(func(criteria interface{}) bool {
						// Verify criteria has required executor IDs
						c, ok := criteria.(optimistic_sync.Criteria)
						if !ok {
							return false
						}
						if len(c.RequiredExecutors) != 2 {
							return false
						}
						return c.RequiredExecutors[0] == executorID1 && c.RequiredExecutors[1] == executorID2
					}),
				).Return(sub).Once()
			},
			expectedResponses: expectedResponse,
		},
		{
			name: "with both agreeing_executors_count and required_executor_ids",
			arguments: func() map[string]interface{} {
				args := unittest.CreateSendTxHttpPayload(tx)
				args["execution_state_query"] = map[string]interface{}{
					"agreeing_executors_count": float64(3),
					"required_executor_ids": []interface{}{
						executorID1.String(),
					},
				}
				return args
			}(),
			setupBackend: func(sub *submock.Subscription) {
				s.api.On(
					"SendAndSubscribeTransactionStatuses",
					mock.Anything,
					mock.Anything,
					entities.EventEncodingVersion_JSON_CDC_V0,
					mock.MatchedBy(func(criteria interface{}) bool {
						// Verify criteria has both agreeing executors count and required executor IDs
						c, ok := criteria.(optimistic_sync.Criteria)
						if !ok {
							return false
						}
						if c.AgreeingExecutorsCount != 3 {
							return false
						}
						if len(c.RequiredExecutors) != 1 {
							return false
						}
						return c.RequiredExecutors[0] == executorID1
					}),
				).Return(sub).Once()
			},
			expectedResponses: expectedResponse,
		},
	}

	testHappyPath(
		s.T(),
		SendAndGetTransactionStatusesTopic,
		s.factory,
		testCases,
		func(dataChan chan interface{}) {
			dataChan <- backendResponse
		},
		s.requireTransactionStatuses,
	)
}

// requireTransactionStatuses ensures that the received transaction statuses information matches the expected data.
func (s *SendTransactionStatusesProviderSuite) requireTransactionStatuses(
	actual interface{},
	expected interface{},
) {
	expectedResponse, expectedResponsePayload := extractPayload[*models.TransactionStatusesResponse](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*models.TransactionStatusesResponse](s.T(), actual)

	require.Equal(s.T(), expectedResponse.Topic, actualResponse.Topic)
	require.Equal(s.T(), expectedResponsePayload.TransactionResult.BlockId, actualResponsePayload.TransactionResult.BlockId)
}

// expectedTransactionStatusesResponses creates the expected responses for the provided backend responses.
func (s *SendTransactionStatusesProviderSuite) expectedTransactionStatusesResponses(
	backendResponses []*accessmodel.TransactionResult,
	topic string,
) []interface{} {
	expectedResponses := make([]interface{}, len(backendResponses))

	for i, resp := range backendResponses {
		expectedResponsePayload := models.NewTransactionStatusesResponse(s.linkGenerator, resp, nil, uint64(i))
		expectedResponses[i] = &models.BaseDataProvidersResponse{
			Topic:   topic,
			Payload: expectedResponsePayload,
		}
	}

	return expectedResponses
}

// TestSendTransactionStatusesDataProvider_InvalidArguments tests the behavior of the send transaction statuses data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
func (s *SendTransactionStatusesProviderSuite) TestSendTransactionStatusesDataProvider_InvalidArguments() {
	send := make(chan interface{})
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
			arguments: map[string]interface{}{
				"script": 0,
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'script' argument",
			arguments: map[string]interface{}{
				"script": "invalid_script",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'arguments' type",
			arguments: map[string]interface{}{
				"arguments": 0,
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'arguments' argument",
			arguments: map[string]interface{}{
				"arguments": []string{"invalid_base64_1", "invalid_base64_2"},
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'reference_block_id' argument",
			arguments: map[string]interface{}{
				"reference_block_id": "invalid_reference_block_id",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'gas_limit' argument",
			arguments: map[string]interface{}{
				"gas_limit": "-1",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'payer' argument",
			arguments: map[string]interface{}{
				"payer": "invalid_payer",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'proposal_key' argument",
			arguments: map[string]interface{}{
				"proposal_key": "invalid ProposalKey object",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'authorizers' argument",
			arguments: map[string]interface{}{
				"authorizers": []string{"invalid_base64_1", "invalid_base64_2"},
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'payload_signatures' argument",
			arguments: map[string]interface{}{
				"payload_signatures": "invalid TransactionSignature array",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "invalid 'envelope_signatures' argument",
			arguments: map[string]interface{}{
				"envelope_signatures": "invalid TransactionSignature array",
			},
			expectedErrorMsg: "failed to parse transaction",
		},
		{
			name: "unexpected argument",
			arguments: map[string]interface{}{
				"unexpected_argument": "dummy",
			},
			expectedErrorMsg: "request body contains unknown field",
		},
		{
			name: "invalid 'execution_state_query' type - not an object",
			arguments: map[string]interface{}{
				"execution_state_query": "not_an_object",
			},
			expectedErrorMsg: "execution_state_query must be an object",
		},
		{
			name: "invalid 'agreeing_executors_count' type",
			arguments: map[string]interface{}{
				"execution_state_query": map[string]interface{}{
					"agreeing_executors_count": "not_a_number",
				},
			},
			expectedErrorMsg: "invalid agreeing_executors_count",
		},
		{
			name: "invalid 'required_executor_ids' type - not an array",
			arguments: map[string]interface{}{
				"execution_state_query": map[string]interface{}{
					"required_executor_ids": "not_an_array",
				},
			},
			expectedErrorMsg: "required_executor_ids must be an array",
		},
		{
			name: "invalid 'required_executor_ids' element type",
			arguments: map[string]interface{}{
				"execution_state_query": map[string]interface{}{
					"required_executor_ids": []interface{}{123, 456},
				},
			},
			expectedErrorMsg: "required_executor_ids[0] must be a string",
		},
		{
			name: "invalid 'required_executor_ids' element value - invalid ID",
			arguments: map[string]interface{}{
				"execution_state_query": map[string]interface{}{
					"required_executor_ids": []interface{}{"invalid_id"},
				},
			},
			expectedErrorMsg: "invalid required_executor_ids[0]",
		},
	}
}

package data_providers

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/access/mock"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	mockcommonmodels "github.com/onflow/flow-go/engine/access/rest/common/models/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	statestreamsmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const unknownBlockStatus = "unknown_block_status"

// BlocksProviderSuite is a test suite for testing the block providers functionality.
type BlocksProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api *accessmock.API

	blocks []*flow.Block

	rootBlock      flow.Block
	finalizedBlock *flow.Header

	factory       *DataProviderFactoryImpl
	linkGenerator *mockcommonmodels.LinkGenerator
}

func TestBlocksProviderSuite(t *testing.T) {
	suite.Run(t, new(BlocksProviderSuite))
}

func (s *BlocksProviderSuite) SetupTest() {
	s.log = unittest.Logger()
	s.api = accessmock.NewAPI(s.T())
	s.linkGenerator = mockcommonmodels.NewLinkGenerator(s.T())

	blockCount := 5
	s.blocks = make([]*flow.Block, 0, blockCount)

	s.rootBlock = unittest.BlockFixture()
	s.rootBlock.Header.Height = 0
	parent := s.rootBlock.Header

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		transaction := unittest.TransactionFixture()
		col := flow.CollectionFromTransactions([]*flow.Transaction{&transaction})
		guarantee := col.Guarantee()
		block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))
		// update for next iteration
		parent = block.Header
		s.blocks = append(s.blocks, block)
	}
	s.finalizedBlock = parent

	s.factory = NewDataProviderFactory(
		s.log,
		nil,
		s.api,
		flow.Testnet.Chain(),
		state_stream.DefaultEventFilterConfig,
		subscription.DefaultHeartbeatInterval,
		s.linkGenerator,
	)
	s.Require().NotNil(s.factory)
}

// TestBlocksDataProvider_HappyPath tests the behavior of the block data provider
// when it is configured correctly and operating under normal conditions. It
// validates that blocks are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *BlocksProviderSuite) TestBlocksDataProvider_HappyPath() {
	s.linkGenerator.On("BlockLink", mock.AnythingOfType("flow.Identifier")).Return(
		func(id flow.Identifier) (string, error) {
			for _, block := range s.blocks {
				if block.ID() == id {
					return fmt.Sprintf("/v1/blocks/%s", id), nil
				}
			}
			return "", assert.AnError
		},
	)

	testHappyPath(
		s.T(),
		BlocksTopic,
		s.factory,
		s.validBlockArgumentsTestCases(),
		func(dataChan chan interface{}) {
			for _, block := range s.blocks {
				dataChan <- block
			}
		},
		s.requireBlock,
	)
}

// validBlockArgumentsTestCases defines test happy cases for block data providers.
// Each test case specifies input arguments, and setup functions for the mock API used in the test.
func (s *BlocksProviderSuite) validBlockArgumentsTestCases() []testType {
	expectedResponses := s.expectedBlockResponses(s.blocks, map[string]bool{commonmodels.ExpandableFieldPayload: true}, flow.BlockStatusFinalized)

	return []testType{
		{
			name: "happy path with start_block_id argument",
			arguments: wsmodels.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
				"block_status":   parser.Finalized,
			},
			setupBackend: func(sub *statestreamsmock.Subscription) {
				s.api.On(
					"SubscribeBlocksFromStartBlockID",
					mock.Anything,
					s.rootBlock.ID(),
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "happy path with start_block_height argument",
			arguments: wsmodels.Arguments{
				"start_block_height": strconv.FormatUint(s.rootBlock.Header.Height, 10),
				"block_status":       parser.Finalized,
			},
			setupBackend: func(sub *statestreamsmock.Subscription) {
				s.api.On(
					"SubscribeBlocksFromStartHeight",
					mock.Anything,
					s.rootBlock.Header.Height,
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "happy path without any start argument",
			arguments: wsmodels.Arguments{
				"block_status": parser.Finalized,
			},
			setupBackend: func(sub *statestreamsmock.Subscription) {
				s.api.On(
					"SubscribeBlocksFromLatest",
					mock.Anything,
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "happy path without any start argument",
			arguments: wsmodels.Arguments{
				"block_status": parser.Finalized,
			},
			setupBackend: func(sub *statestreamsmock.Subscription) {
				s.api.On(
					"SubscribeBlocksFromLatest",
					mock.Anything,
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
	}
}

// requireBlock ensures that the received block information matches the expected data.
func (s *BlocksProviderSuite) requireBlock(actual interface{}, expected interface{}) {
	expectedResponse, expectedResponsePayload := extractPayload[*commonmodels.Block](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*commonmodels.Block](s.T(), actual)

	s.Require().Equal(expectedResponse.Topic, actualResponse.Topic)
	s.Require().Equal(expectedResponsePayload, actualResponsePayload)
}

// expectedBlockResponses generates a list of expected block responses for the given blocks.
func (s *BlocksProviderSuite) expectedBlockResponses(
	blocks []*flow.Block,
	expand map[string]bool,
	status flow.BlockStatus,
) []interface{} {
	responses := make([]interface{}, len(blocks))
	for i, b := range blocks {
		var block commonmodels.Block
		err := block.Build(b, nil, s.linkGenerator, status, expand)
		s.Require().NoError(err)

		responses[i] = &models.BaseDataProvidersResponse{
			Topic:   BlocksTopic,
			Payload: &block,
		}
	}

	return responses
}

// TestBlocksDataProvider_InvalidArguments tests the behavior of the block data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
func (s *BlocksProviderSuite) TestBlocksDataProvider_InvalidArguments() {
	send := make(chan interface{})

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlocksDataProvider(s.log, s.api, "dummy-id", nil, BlocksTopic, test.arguments, send)
			s.Require().Error(err)
			s.Require().Nil(provider)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// invalidArgumentsTestCases returns a list of test cases with invalid argument combinations
// for testing the behavior of block, block headers, block digests data providers. Each test case includes a name,
// a set of input arguments, and the expected error message that should be returned.
func (s *BlocksProviderSuite) invalidArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "missing 'block_status' argument",
			arguments: wsmodels.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
			},
			expectedErrorMsg: "missing 'block_status' field",
		},
		{
			name: "unknown 'block_status' argument",
			arguments: wsmodels.Arguments{
				"block_status": unknownBlockStatus,
			},
			expectedErrorMsg: fmt.Sprintf("invalid 'block_status', must be '%s' or '%s'", parser.Finalized, parser.Sealed),
		},
		{
			name: "provide both 'start_block_id' and 'start_block_height' arguments",
			arguments: wsmodels.Arguments{
				"block_status":       parser.Finalized,
				"start_block_id":     s.rootBlock.ID().String(),
				"start_block_height": fmt.Sprintf("%d", s.rootBlock.Header.Height),
			},
			expectedErrorMsg: "can only provide either 'start_block_id' or 'start_block_height'",
		},
		{
			name: "unexpected argument",
			arguments: map[string]interface{}{
				"block_status":        parser.Finalized,
				"start_block_id":      unittest.BlockFixture().ID().String(),
				"unexpected_argument": "dummy",
			},
			expectedErrorMsg: "unexpected field: 'unexpected_argument'",
		},
	}
}

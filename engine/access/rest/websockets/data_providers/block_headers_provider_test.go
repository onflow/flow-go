package data_providers

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	statestreamsmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
)

type BlockHeadersProviderSuite struct {
	BlocksProviderSuite
}

func TestBlockHeadersProviderSuite(t *testing.T) {
	suite.Run(t, new(BlockHeadersProviderSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *BlockHeadersProviderSuite) SetupTest() {
	s.BlocksProviderSuite.SetupTest()
}

// TestBlockHeadersDataProvider_InvalidArguments tests the behavior of the block headers data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Missing 'block_status' argument.
// 2. Invalid 'block_status' argument.
// 3. Providing both 'start_block_id' and 'start_block_height' simultaneously.
func (s *BlockHeadersProviderSuite) TestBlockHeadersDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := BlockHeadersTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlockHeadersDataProvider(ctx, s.log, s.api, "dummy-id", topic, test.arguments, send)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// validBlockHeadersArgumentsTestCases defines test happy cases for block headers data providers.
// Each test case specifies input arguments, and setup functions for the mock API used in the test.
func (s *BlockHeadersProviderSuite) validBlockHeadersArgumentsTestCases() []testType {
	expectedResponses := make([]interface{}, len(s.blocks))
	for i, b := range s.blocks {
		var header commonmodels.BlockHeader
		header.Build(b.Header)

		expectedResponses[i] = &models.BlockHeaderMessageResponse{Header: &header}
	}

	return []testType{
		{
			name: "happy path with start_block_id argument",
			arguments: models.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
				"block_status":   parser.Finalized,
			},
			setupBackend: func(sub *statestreamsmock.Subscription) {
				s.api.On(
					"SubscribeBlockHeadersFromStartBlockID",
					mock.Anything,
					s.rootBlock.ID(),
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "happy path with start_block_height argument",
			arguments: models.Arguments{
				"start_block_height": strconv.FormatUint(s.rootBlock.Header.Height, 10),
				"block_status":       parser.Finalized,
			},
			setupBackend: func(sub *statestreamsmock.Subscription) {
				s.api.On(
					"SubscribeBlockHeadersFromStartHeight",
					mock.Anything,
					s.rootBlock.Header.Height,
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
		{
			name: "happy path without any start argument",
			arguments: models.Arguments{
				"block_status": parser.Finalized,
			},
			setupBackend: func(sub *statestreamsmock.Subscription) {
				s.api.On(
					"SubscribeBlockHeadersFromLatest",
					mock.Anything,
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
	}
}

// TestBlockHeadersDataProvider_HappyPath tests the behavior of the block headers data provider
// when it is configured correctly and operating under normal conditions. It
// validates that block headers are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *BlockHeadersProviderSuite) TestBlockHeadersDataProvider_HappyPath() {
	testHappyPath(
		s.T(),
		BlockHeadersTopic,
		s.factory,
		s.validBlockHeadersArgumentsTestCases(),
		func(dataChan chan interface{}) {
			for _, block := range s.blocks {
				dataChan <- block.Header
			}
		},
		s.requireBlockHeader,
	)
}

// requireBlockHeaders ensures that the received block header information matches the expected data.
func (s *BlockHeadersProviderSuite) requireBlockHeader(actual interface{}, expected interface{}) {
	actualResponse, ok := actual.(*models.BlockHeaderMessageResponse)
	require.True(s.T(), ok, "unexpected response type: %T", actual)

	expectedResponse, ok := expected.(*models.BlockHeaderMessageResponse)
	require.True(s.T(), ok, "unexpected response type: %T", expected)

	s.Require().Equal(expectedResponse.Header, actualResponse.Header)
}

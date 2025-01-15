package data_providers

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	statestreamsmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
)

type BlockDigestsProviderSuite struct {
	BlocksProviderSuite
}

func TestBlockDigestsProviderSuite(t *testing.T) {
	suite.Run(t, new(BlockDigestsProviderSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *BlockDigestsProviderSuite) SetupTest() {
	s.BlocksProviderSuite.SetupTest()
}

// TestBlockDigestsDataProvider_InvalidArguments tests the behavior of the block digests data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Missing 'block_status' argument.
// 2. Invalid 'block_status' argument.
// 3. Providing both 'start_block_id' and 'start_block_height' simultaneously.
func (s *BlockDigestsProviderSuite) TestBlockDigestsDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := BlockDigestsTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlockDigestsDataProvider(ctx, s.log, s.api, uuid.New(), topic, test.arguments, send)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// validBlockDigestsArgumentsTestCases defines test happy cases for block digests data providers.
// Each test case specifies input arguments, and setup functions for the mock API used in the test.
func (s *BlockDigestsProviderSuite) validBlockDigestsArgumentsTestCases() []testType {
	expectedResponses := make([]interface{}, len(s.blocks))
	for i, b := range s.blocks {
		blockDigest := flow.NewBlockDigest(b.Header.ID(), b.Header.Height, b.Header.Timestamp)

		var block models.BlockDigest
		block.Build(blockDigest)

		expectedResponses[i] = &models.BlockDigestMessageResponse{Block: &block}
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
					"SubscribeBlockDigestsFromStartBlockID",
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
					"SubscribeBlockDigestsFromStartHeight",
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
					"SubscribeBlockDigestsFromLatest",
					mock.Anything,
					flow.BlockStatusFinalized,
				).Return(sub).Once()
			},
			expectedResponses: expectedResponses,
		},
	}
}

// TestBlockDigestsDataProvider_HappyPath tests the behavior of the block digests data provider
// when it is configured correctly and operating under normal conditions. It
// validates that block digests are correctly streamed to the channel and ensures
// no unexpected errors occur.
func (s *BlockDigestsProviderSuite) TestBlockDigestsDataProvider_HappyPath() {
	testHappyPath(
		s.T(),
		BlockDigestsTopic,
		s.factory,
		s.validBlockDigestsArgumentsTestCases(),
		func(dataChan chan interface{}) {
			for _, block := range s.blocks {
				dataChan <- flow.NewBlockDigest(block.Header.ID(), block.Header.Height, block.Header.Timestamp)
			}
		},
		s.requireBlockDigest,
	)
}

// requireBlockHeaders ensures that the received block header information matches the expected data.
func (s *BlocksProviderSuite) requireBlockDigest(actual interface{}, expected interface{}) {
	actualResponse, ok := actual.(*models.BlockDigestMessageResponse)
	require.True(s.T(), ok, "unexpected response type: %T", actual)

	expectedResponse, ok := expected.(*models.BlockDigestMessageResponse)
	require.True(s.T(), ok, "unexpected response type: %T", expected)

	s.Require().Equal(expectedResponse.Block, actualResponse.Block)
}

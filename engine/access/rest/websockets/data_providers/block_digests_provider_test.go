package data_providers

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
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

// validBlockDigestsArgumentsTestCases defines test happy cases for block digests data providers.
// Each test case specifies input arguments, and setup functions for the mock API used in the test.
func (s *BlockDigestsProviderSuite) validBlockDigestsArgumentsTestCases() []testType {
	expectedResponses := make([]interface{}, len(s.blocks))
	for i, b := range s.blocks {
		blockDigest := flow.NewBlockDigest(b.Header.ID(), b.Header.Height, b.Header.Timestamp)
		blockDigestPayload := models.NewBlockDigest(blockDigest)
		expectedResponses[i] = &models.BaseDataProvidersResponse{
			Topic:   BlockDigestsTopic,
			Payload: blockDigestPayload,
		}
	}

	return []testType{
		{
			name: "happy path with start_block_id argument",
			arguments: wsmodels.Arguments{
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
			arguments: wsmodels.Arguments{
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
			arguments: wsmodels.Arguments{
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

// requireBlockDigest ensures that the received block header information matches the expected data.
func (s *BlocksProviderSuite) requireBlockDigest(actual interface{}, expected interface{}) {
	expectedResponse, expectedResponsePayload := extractPayload[*models.BlockDigest](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*models.BlockDigest](s.T(), actual)

	s.Require().Equal(expectedResponse.Topic, actualResponse.Topic)
	s.Require().Equal(expectedResponsePayload, actualResponsePayload)
}

// TestBlockDigestsDataProvider_InvalidArguments tests the behavior of the block digests data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
func (s *BlockDigestsProviderSuite) TestBlockDigestsDataProvider_InvalidArguments() {
	send := make(chan interface{})

	topic := BlockDigestsTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlockDigestsDataProvider(context.Background(), s.log, s.api, "dummy-id", topic, test.arguments, send)
			s.Require().Error(err)
			s.Require().Nil(provider)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

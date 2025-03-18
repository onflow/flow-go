package data_providers

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
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

// validBlockHeadersArgumentsTestCases defines test happy cases for block headers data providers.
// Each test case specifies input arguments, and setup functions for the mock API used in the test.
func (s *BlockHeadersProviderSuite) validBlockHeadersArgumentsTestCases() []testType {
	expectedResponses := make([]interface{}, len(s.blocks))
	for i, b := range s.blocks {
		var header commonmodels.BlockHeader
		header.Build(b.Header)

		expectedResponses[i] = &models.BaseDataProvidersResponse{
			Topic:   BlockHeadersTopic,
			Payload: &header,
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
			arguments: wsmodels.Arguments{
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
			arguments: wsmodels.Arguments{
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

// requireBlockHeaders ensures that the received block header information matches the expected data.
func (s *BlockHeadersProviderSuite) requireBlockHeader(actual interface{}, expected interface{}) {
	expectedResponse, expectedResponsePayload := extractPayload[*commonmodels.BlockHeader](s.T(), expected)
	actualResponse, actualResponsePayload := extractPayload[*commonmodels.BlockHeader](s.T(), actual)

	s.Require().Equal(expectedResponse.Topic, actualResponse.Topic)
	s.Require().Equal(expectedResponsePayload, actualResponsePayload)
}

// TestBlockHeadersDataProvider_InvalidArguments tests the behavior of the block headers data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
func (s *BlockHeadersProviderSuite) TestBlockHeadersDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := BlockHeadersTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlockHeadersDataProvider(ctx, s.log, s.api, "dummy-id", topic, test.arguments, send)
			s.Require().Error(err)
			s.Require().Nil(provider)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

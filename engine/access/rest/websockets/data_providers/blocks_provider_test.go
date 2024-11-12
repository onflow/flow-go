package data_providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	mockstatestream "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const unknownBlockStatus = "unknown_block_status"

type testErrType struct {
	name             string
	arguments        map[string]string
	expectedErrorMsg string
}

// BlocksProviderSuite is a test suite for testing the block providers functionality.
type BlocksProviderSuite struct {
	suite.Suite

	log zerolog.Logger
	api *accessmock.API

	blockMap       map[uint64]*flow.Block
	rootBlock      flow.Block
	finalizedBlock *flow.Header
}

func TestBlocksProviderSuite(t *testing.T) {
	suite.Run(t, new(BlocksProviderSuite))
}

func (s *BlocksProviderSuite) SetupTest() {
	s.log = unittest.Logger()
	s.api = accessmock.NewAPI(s.T())

	blockCount := 5
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.rootBlock = unittest.BlockFixture()
	s.rootBlock.Header.Height = 0
	parent := s.rootBlock.Header

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.Header
		s.blockMap[block.Header.Height] = block
	}
	s.finalizedBlock = parent
}

// TestBlocksDataProvider_InvalidArguments verifies that
func (s *BlocksProviderSuite) TestBlocksDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	testCases := []testErrType{
		{
			name: "missing 'block_status' argument",
			arguments: map[string]string{
				"start_block_id": s.rootBlock.ID().String(),
			},
			expectedErrorMsg: "'block_status' must be provided",
		},
		{
			name: "unknown 'block_status' argument",
			arguments: map[string]string{
				"block_status": unknownBlockStatus,
			},
			expectedErrorMsg: fmt.Sprintf("invalid 'block_status', must be '%s' or '%s'", parser.Finalized, parser.Sealed),
		},
		{
			name: "provide both 'start_block_id' and 'start_block_height' arguments",
			arguments: map[string]string{
				"block_status":       parser.Finalized,
				"start_block_id":     s.rootBlock.ID().String(),
				"start_block_height": fmt.Sprintf("%d", s.rootBlock.Header.Height),
			},
			expectedErrorMsg: "can only provide either 'start_block_id' or 'start_block_height'",
		},
	}

	topic := BlocksTopic

	for _, test := range testCases {
		s.Run(test.name, func() {
			provider, err := NewBlocksDataProvider(ctx, s.log, s.api, topic, test.arguments, send)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// TestBlocksDataProvider_ValidArguments tests
func (s *BlocksProviderSuite) TestBlocksDataProvider_ValidArguments() {
	ctx := context.Background()

	topic := BlocksTopic
	send := make(chan interface{})

	s.Run("subscribe blocks from start block id", func() {
		subscription := mockstatestream.NewSubscription(s.T())
		startBlockId := s.rootBlock.Header.ID()

		s.api.On("SubscribeBlocksFromStartBlockID", mock.Anything, startBlockId, flow.BlockStatusFinalized).Return(subscription).Once()

		arguments := map[string]string{
			"start_block_id": startBlockId.String(),
			"block_status":   parser.Finalized,
		}

		provider, err := NewBlocksDataProvider(ctx, s.log, s.api, topic, arguments, send)
		s.Require().NoError(err)
		s.Require().NotNil(provider)
		s.Require().Equal(flow.BlockStatusFinalized, provider.args.BlockStatus)

		// Create a channel to receive mock Blocks objects
		ch := make(chan interface{})
		var chReadOnly <-chan interface{}
		// Simulate sending a mock Blocks
		go func() {
			for _, block := range s.blockMap {
				// Send the mock Blocks through the channel
				ch <- block
			}
		}()

		chReadOnly = ch
		subscription.Mock.On("Channel").Return(chReadOnly)

		err = provider.Run()
		s.Require().NoError(err)

		provider.Close()
	})
}

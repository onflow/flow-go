package data_providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const unknownBlockStatus = "unknown_block_status"

type testErrType struct {
	name             string
	arguments        models.Arguments
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

// invalidArgumentsTestCases returns a list of test cases with invalid argument combinations
// for testing the behavior of block, block headers, block digests data providers. Each test case includes a name,
// a set of input arguments, and the expected error message that should be returned.
//
// The test cases cover scenarios such as:
// 1. Missing the required 'block_status' argument.
// 2. Providing an unknown or invalid 'block_status' value.
// 3. Supplying both 'start_block_id' and 'start_block_height' simultaneously, which is not allowed.
func (s *BlocksProviderSuite) invalidArgumentsTestCases() []testErrType {
	return []testErrType{
		{
			name: "missing 'block_status' argument",
			arguments: models.Arguments{
				"start_block_id": s.rootBlock.ID().String(),
			},
			expectedErrorMsg: "'block_status' must be provided",
		},
		{
			name: "unknown 'block_status' argument",
			arguments: models.Arguments{
				"block_status": unknownBlockStatus,
			},
			expectedErrorMsg: fmt.Sprintf("invalid 'block_status', must be '%s' or '%s'", parser.Finalized, parser.Sealed),
		},
		{
			name: "provide both 'start_block_id' and 'start_block_height' arguments",
			arguments: models.Arguments{
				"block_status":       parser.Finalized,
				"start_block_id":     s.rootBlock.ID().String(),
				"start_block_height": fmt.Sprintf("%d", s.rootBlock.Header.Height),
			},
			expectedErrorMsg: "can only provide either 'start_block_id' or 'start_block_height'",
		},
	}
}

// TestBlocksDataProvider_InvalidArguments tests the behavior of the block data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Missing 'block_status' argument.
// 2. Invalid 'block_status' argument.
// 3. Providing both 'start_block_id' and 'start_block_height' simultaneously.
func (s *BlocksProviderSuite) TestBlocksDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := BlocksTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlocksDataProvider(ctx, s.log, s.api, topic, test.arguments, send)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// TODO: add tests for responses after the WebsocketController is ready

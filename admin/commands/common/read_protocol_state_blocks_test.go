package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ReadProtocolStateBlocksSuite struct {
	suite.Suite

	command commands.AdminCommand
	state   *protocolmock.State
	blocks  *storagemock.Blocks

	final     *flow.Block
	sealed    *flow.Block
	allBlocks []*flow.Block
}

func TestReadProtocolStateBlocks(t *testing.T) {
	suite.Run(t, new(ReadProtocolStateBlocksSuite))
}

func createSnapshot(head *flow.Header) protocol.Snapshot {
	snapshot := &protocolmock.Snapshot{}
	snapshot.On("Head").Return(
		func() *flow.Header {
			return head
		},
		nil,
	)
	return snapshot
}

func (suite *ReadProtocolStateBlocksSuite) SetupTest() {
	suite.state = new(protocolmock.State)
	suite.blocks = new(storagemock.Blocks)

	var blocks []*flow.Block

	genesis := unittest.GenesisFixture()
	blocks = append(blocks, genesis)
	sealed := unittest.BlockWithParentFixture(genesis.Header)
	blocks = append(blocks, &sealed)
	final := unittest.BlockWithParentFixture(sealed.Header)
	blocks = append(blocks, &final)
	final = unittest.BlockWithParentFixture(final.Header)
	blocks = append(blocks, &final)
	final = unittest.BlockWithParentFixture(final.Header)
	blocks = append(blocks, &final)

	suite.allBlocks = blocks
	suite.sealed = &sealed
	suite.final = &final

	suite.state.On("Final").Return(createSnapshot(final.Header))
	suite.state.On("Sealed").Return(createSnapshot(sealed.Header))
	suite.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protocol.Snapshot {
			for _, block := range blocks {
				if block.ID() == blockID {
					return createSnapshot(block.Header)
				}
			}
			return invalid.NewSnapshot(fmt.Errorf("invalid block ID: %v", blockID))
		},
	)
	suite.state.On("AtHeight", mock.Anything).Return(
		func(height uint64) protocol.Snapshot {
			if int(height) < len(blocks) {
				block := blocks[height]
				return createSnapshot(block.Header)
			}
			return invalid.NewSnapshot(fmt.Errorf("invalid height: %v", height))
		},
	)

	suite.blocks.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Block {
			for _, block := range blocks {
				if block.ID() == blockID {
					return block
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, block := range blocks {
				if block.ID() == blockID {
					return nil
				}
			}
			return errors.New("block not found")
		},
	)

	suite.command = NewReadProtocolStateBlocksCommand(suite.state, suite.blocks)
}

func (suite *ReadProtocolStateBlocksSuite) TestValidateInvalidFormat() {
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: true,
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: 420,
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: "foo",
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"glock": 123,
		},
	}))
}

func (suite *ReadProtocolStateBlocksSuite) TestValidateInvalidBlock() {
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": true,
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": "",
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": "uhznms",
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": "deadbeef",
		},
	}))
}

func (suite *ReadProtocolStateBlocksSuite) TestValidateInvalidBlockHeight() {
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": float64(0),
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": float64(1.1),
		},
	}))
}

func (suite *ReadProtocolStateBlocksSuite) TestValidateInvalidN() {
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": 1,
			"n":     "foo",
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": 1,
			"n":     float64(1.1),
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"block": 1,
			"n":     float64(0),
		},
	}))
}

func (suite *ReadProtocolStateBlocksSuite) getBlocks(reqData map[string]interface{}) []*flow.Block {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &admin.CommandRequest{
		Data: reqData,
	}
	require.NoError(suite.T(), suite.command.Validator(req))
	result, err := suite.command.Handler(ctx, req)
	require.NoError(suite.T(), err)

	var blocks []*flow.Block
	data, err := json.Marshal(result)
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), json.Unmarshal(data, &blocks))

	return blocks
}

func (suite *ReadProtocolStateBlocksSuite) TestHandleFinal() {
	blocks := suite.getBlocks(map[string]interface{}{
		"block": "final",
	})
	require.Len(suite.T(), blocks, 1)
	require.EqualValues(suite.T(), blocks[0], suite.final)
}

func (suite *ReadProtocolStateBlocksSuite) TestHandleSealed() {
	blocks := suite.getBlocks(map[string]interface{}{
		"block": "sealed",
	})
	require.Len(suite.T(), blocks, 1)
	require.EqualValues(suite.T(), blocks[0], suite.sealed)
}

func (suite *ReadProtocolStateBlocksSuite) TestHandleHeight() {
	for i, block := range suite.allBlocks {
		responseBlocks := suite.getBlocks(map[string]interface{}{
			"block": float64(i),
		})
		require.Len(suite.T(), responseBlocks, 1)
		require.EqualValues(suite.T(), responseBlocks[0], block)
	}
}

func (suite *ReadProtocolStateBlocksSuite) TestHandleID() {
	for _, block := range suite.allBlocks {
		responseBlocks := suite.getBlocks(map[string]interface{}{
			"block": block.ID().String(),
		})
		require.Len(suite.T(), responseBlocks, 1)
		require.EqualValues(suite.T(), responseBlocks[0], block)
	}
}

func (suite *ReadProtocolStateBlocksSuite) TestHandleNExceedsRootBlock() {
	responseBlocks := suite.getBlocks(map[string]interface{}{
		"block": "final",
		"n":     float64(len(suite.allBlocks) + 1),
	})
	require.Len(suite.T(), responseBlocks, len(suite.allBlocks))
	require.ElementsMatch(suite.T(), responseBlocks, suite.allBlocks)
}

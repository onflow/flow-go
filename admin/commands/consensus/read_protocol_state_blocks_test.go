package consensus

import (
	"testing"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ReadProtocolStateBlocksSuite struct {
	suite.Suite

	command commands.AdminCommand
}

func TestReadProtocolStateBlocks(t *testing.T) {
	suite.Run(t, new(ReadProtocolStateBlocksSuite))
}

func (suite *ReadProtocolStateBlocksSuite) SetupTest() {
	state := new(protocolmock.State)
	blocks := new(storagemock.Blocks)
	suite.command = NewReadProtocolStateBlocksCommand(state, blocks)
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
			"block": float64(-1),
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
			"n":     float64(-1),
		},
	}))
}

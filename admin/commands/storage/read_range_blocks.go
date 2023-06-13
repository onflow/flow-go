package storage

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/read-light-block"
	"github.com/onflow/flow-go/storage"
)

var _ commands.AdminCommand = (*ReadRangeBlocksCommand)(nil)

type ReadRangeBlocksCommand struct {
	blocks storage.Blocks
}

func NewReadRangeBlocksCommand(blocks storage.Blocks) commands.AdminCommand {
	return &ReadRangeBlocksCommand{
		blocks: blocks,
	}
}

func (c *ReadRangeBlocksCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	reqData, err := parseHeightRangeRequestData(req)
	if err != nil {
		return nil, err
	}

	lights, err := read.ReadLightBlockByHeightRange(c.blocks, reqData.startHeight, reqData.endHeight)
	if err != nil {
		return nil, err
	}
	return commands.ConvertToInterfaceList(lights)
}

func (c *ReadRangeBlocksCommand) Validator(req *admin.CommandRequest) error {
	return nil
}

package storage

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/read-block-light"
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
	data, err := parseHeightRangeRequestData(req)
	if err != nil {
		return nil, err
	}

	lights, err := read.ReadBlockLightByHeightRange(c.blocks, data.startHeight, data.endHeight)
	if err != nil {
		return nil, err
	}
	return lights, nil
}

func (c *ReadRangeBlocksCommand) Validator(req *admin.CommandRequest) error {
	return nil
}

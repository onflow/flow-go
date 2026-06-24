package storage

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/read-light-block"
	"github.com/onflow/flow-go/storage"
)

var _ commands.AdminCommand = (*ReadRangeBlocksCommand)(nil)

// 10001 instead of 10000, because 10000 won't allow a range from 10000 to 20000,
// which is easier to type than [10001, 20000]
const Max_Range_Block_Limit = uint64(10001)

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

	log.Info().Str("module", "admin-tool").Msgf("read range blocks, data: %v", reqData)

	if reqData.Range() > Max_Range_Block_Limit {
		return nil, admin.NewInvalidAdminReqErrorf("getting for more than %v blocks at a time might have an impact to node's performance and is not allowed", Max_Range_Block_Limit)
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

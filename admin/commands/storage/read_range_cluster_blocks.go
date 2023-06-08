package storage

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/read-block-light"
	"github.com/onflow/flow-go/storage"
)

var _ commands.AdminCommand = (*ReadRangeClusterBlocksCommand)(nil)

type ReadRangeClusterBlocksCommand struct {
	clusterBlocks storage.ClusterBlocks
}

func NewReadRangeClusterBlocksCommand(cluster storage.ClusterBlocks) commands.AdminCommand {
	return &ReadRangeClusterBlocksCommand{
		clusterBlocks: cluster,
	}
}

func (c *ReadRangeClusterBlocksCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data, err := parseHeightRangeRequestData(req)
	if err != nil {
		return nil, err
	}

	lights, err := read.ReadClusterBlockLightByHeightRange(c.clusterBlocks, data.startHeight, data.endHeight)
	if err != nil {
		return nil, err
	}
	return lights, nil
}

func (c *ReadRangeClusterBlocksCommand) Validator(req *admin.CommandRequest) error {
	return nil
}

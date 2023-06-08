package storage

import (
	"context"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/read-block-light"
	"github.com/onflow/flow-go/model/flow"
	storage "github.com/onflow/flow-go/storage/badger"
)

var _ commands.AdminCommand = (*ReadRangeClusterBlocksCommand)(nil)

type ReadRangeClusterBlocksCommand struct {
	db       *badger.DB
	headers  *storage.Headers
	payloads *storage.ClusterPayloads
}

func NewReadRangeClusterBlocksCommand(db *badger.DB, headers *storage.Headers, payloads *storage.ClusterPayloads) commands.AdminCommand {
	return &ReadRangeClusterBlocksCommand{
		db:       db,
		headers:  headers,
		payloads: payloads,
	}
}

func (c *ReadRangeClusterBlocksCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return nil, admin.NewInvalidAdminReqFormatError("missing 'data' field")
	}
	chainID, err := findString(input, "chain-id")
	if err != nil {
		return nil, admin.NewInvalidAdminReqErrorf("missing chain-id field")
	}

	data, err := parseHeightRangeRequestData(req)
	if err != nil {
		return nil, err
	}

	clusterBlocks := storage.NewClusterBlocks(
		c.db, flow.ChainID(chainID), c.headers, c.payloads,
	)

	lights, err := read.ReadClusterBlockLightByHeightRange(clusterBlocks, data.startHeight, data.endHeight)
	if err != nil {
		return nil, err
	}
	return commands.ConvertToInterfaceList(lights)
}

func (c *ReadRangeClusterBlocksCommand) Validator(req *admin.CommandRequest) error {
	return nil
}

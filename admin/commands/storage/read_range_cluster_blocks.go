package storage

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/read-light-block"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

var _ commands.AdminCommand = (*ReadRangeClusterBlocksCommand)(nil)

// 10001 instead of 10000, because 10000 won't allow a range from 10000 to 20000,
// which is easier to type than [10001, 20000]
const Max_Range_Cluster_Block_Limit = uint64(10001)

type ReadRangeClusterBlocksCommand struct {
	db       storage.DB
	payloads *store.ClusterPayloads
}

func NewReadRangeClusterBlocksCommand(db storage.DB, payloads *store.ClusterPayloads) commands.AdminCommand {
	return &ReadRangeClusterBlocksCommand{
		db:       db,
		payloads: payloads,
	}
}

func (c *ReadRangeClusterBlocksCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	chainID, err := parseString(req, "chain-id")
	if err != nil {
		return nil, err
	}

	reqData, err := parseHeightRangeRequestData(req)
	if err != nil {
		return nil, err
	}

	log.Info().Str("module", "admin-tool").Msgf("read range cluster blocks, data: %v", reqData)

	if reqData.Range() > Max_Range_Cluster_Block_Limit {
		return nil, admin.NewInvalidAdminReqErrorf("getting for more than %v blocks at a time might have an impact to node's performance and is not allowed", Max_Range_Cluster_Block_Limit)
	}

	clusterHeaders := store.NewClusterHeaders(&metrics.NoopCollector{}, c.db, flow.ChainID(chainID))
	clusterBlocks := store.NewClusterBlocks(
		c.db, flow.ChainID(chainID), clusterHeaders, c.payloads,
	)

	lights, err := read.ReadClusterLightBlockByHeightRange(clusterBlocks, reqData.startHeight, reqData.endHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get with chainID id %v: %w", chainID, err)
	}
	return commands.ConvertToInterfaceList(lights)
}

func (c *ReadRangeClusterBlocksCommand) Validator(req *admin.CommandRequest) error {
	return nil
}

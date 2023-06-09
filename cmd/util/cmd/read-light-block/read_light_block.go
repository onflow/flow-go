package read

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ClusterLightBlock struct {
	ID           flow.Identifier
	Height       uint64
	CollectionID flow.Identifier
	Transactions []flow.Identifier
}

func ClusterBlockToLight(clusterBlock *cluster.Block) *ClusterLightBlock {
	return &ClusterLightBlock{
		ID:           clusterBlock.ID(),
		Height:       clusterBlock.Header.Height,
		CollectionID: clusterBlock.Payload.Collection.ID(),
		Transactions: clusterBlock.Payload.Collection.Light().Transactions,
	}
}

func ReadClusterLightBlockByHeightRange(clusterBlocks storage.ClusterBlocks, startHeight uint64, endHeight uint64) ([]*ClusterLightBlock, error) {
	blocks := make([]*ClusterLightBlock, 0)
	for height := startHeight; height <= endHeight; height++ {
		block, err := clusterBlocks.ByHeight(height)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				break
			}
			return nil, fmt.Errorf("could not get cluster block by height %v: %w", height, err)
		}
		light := ClusterBlockToLight(block)
		blocks = append(blocks, light)
	}
	return blocks, nil
}

type LightBlock struct {
	ID          flow.Identifier
	Height      uint64
	Collections []flow.Identifier
}

func BlockToLight(block *flow.Block) *LightBlock {
	return &LightBlock{
		ID:          block.ID(),
		Height:      block.Header.Height,
		Collections: flow.EntitiesToIDs(block.Payload.Guarantees),
	}
}

func ReadLightBlockByHeightRange(blocks storage.Blocks, startHeight uint64, endHeight uint64) ([]*LightBlock, error) {
	bs := make([]*LightBlock, 0)
	for height := startHeight; height <= endHeight; height++ {
		block, err := blocks.ByHeight(height)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				break
			}

			return nil, fmt.Errorf("could not get cluster block by height %v: %w", height, err)
		}
		light := BlockToLight(block)
		bs = append(bs, light)
	}
	return bs, nil
}

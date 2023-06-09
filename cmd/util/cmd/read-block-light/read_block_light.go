package read

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ClusterBlockLight struct {
	ID           flow.Identifier
	Height       uint64
	CollectionID flow.Identifier
	Transactions []flow.Identifier
}

func ClusterBlockToLight(clusterBlock *cluster.Block) *ClusterBlockLight {
	return &ClusterBlockLight{
		ID:           clusterBlock.ID(),
		Height:       clusterBlock.Header.Height,
		CollectionID: clusterBlock.Payload.Collection.ID(),
		Transactions: clusterBlock.Payload.Collection.Light().Transactions,
	}
}

func ReadClusterBlockLightByHeightRange(clusterBlocks storage.ClusterBlocks, startHeight uint64, endHeight uint64) ([]*ClusterBlockLight, error) {
	blocks := make([]*ClusterBlockLight, 0)
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

type BlockLight struct {
	ID          flow.Identifier
	Height      uint64
	Collections []flow.Identifier
}

func BlockToLight(block *flow.Block) *BlockLight {
	return &BlockLight{
		ID:          block.ID(),
		Height:      block.Header.Height,
		Collections: flow.EntityToIDs(block.Payload.Guarantees),
	}
}

func ReadBlockLightByHeightRange(blocks storage.Blocks, startHeight uint64, endHeight uint64) ([]*BlockLight, error) {
	bs := make([]*BlockLight, 0)
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

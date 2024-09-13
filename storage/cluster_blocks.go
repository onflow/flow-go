package storage

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type ClusterBlocks interface {

	// Store stores the cluster block.
	Store(block *cluster.Block) error

	// ByID returns the block with the given ID.
	ByID(blockID flow.Identifier) (*cluster.Block, error)

	// ByHeight returns the block with the given height. Only available for
	// finalized blocks.
	ByHeight(height uint64) (*cluster.Block, error)
}

type ClusterBlockIndexer interface {
	// InsertClusterBlock inserts a cluster consensus block, updating all associated indexes.
	// When calling by multiple goroutines, it should be thread-safe.
	InsertClusterBlock(block *cluster.Block) func(PebbleReaderBatchWriter) error
}

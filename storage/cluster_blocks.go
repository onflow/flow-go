package storage

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type ClusterBlocks interface {

	// Store stores the cluster block.
	Store(proposal *cluster.BlockProposal) error

	// ByID returns the block with the given ID.
	ByID(blockID flow.Identifier) (*cluster.BlockProposal, error)

	// ByHeight returns the block with the given height. Only available for
	// finalized blocks.
	ByHeight(height uint64) (*cluster.BlockProposal, error)
}

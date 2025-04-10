package storage

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type ClusterBlocks interface {

	// Store stores the cluster block.
	Store(proposal *cluster.BlockProposal) error

	// ProposalByID returns the block with the given ID.
	ProposalByID(blockID flow.Identifier) (*cluster.BlockProposal, error)

	// ProposalByHeight returns the block with the given height. Only available for
	// finalized blocks.
	ProposalByHeight(height uint64) (*cluster.BlockProposal, error)
}

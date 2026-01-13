package storage

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// ClusterBlocks provides persistent storage for collector blocks (aka collections) produced
// by *one specific* collector cluster (identified by the ClusterChainID).
// For consistency, method naming is analogous to the [storage.Blocks] interface. Though,
// at the moment, we only need to store [cluster.Proposal]. Therefore, methods `ByID` and
// `ByHeight` don't exist here (but might be added later).
type ClusterBlocks interface {

	// ProposalByID returns the collection with the given ID, along with the proposer's signature on it.
	// It is available for all incorporated collections (validated blocks that have been appended to any
	// of the known forks) no matter whether the collection has been finalized or not.
	//
	// Error returns:
	//   - [storage.ErrNotFound] if the block ID was not found
	//   - generic error in case of unexpected failure from the database layer, or failure
	//     to decode an existing database value
	ProposalByID(blockID flow.Identifier) (*cluster.Proposal, error)

	// ProposalByHeight returns the collection at the given height, along with the proposer's
	// signature on it. It is only available for finalized collections.
	//
	// Error returns:
	//   - [storage.ErrNotFound] if the block height or block ID was not found
	//   - generic error in case of unexpected failure from the database layer, or failure
	//     to decode an existing database value
	ProposalByHeight(height uint64) (*cluster.Proposal, error)
}

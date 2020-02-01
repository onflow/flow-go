package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/jrick/bitset"
)

type ViewState interface {
	// IsSelf returns if a given identity is myself
	IsSelf(id *flow.Identity) bool

	// IsSelfLeaderForView returns if myself is the leader at a given view
	IsSelfLeaderForView(view uint64) bool

	// GetSelfIdxForBlockID returns my own index in all staked node at a given block.
	GetSelfIdxForBlockID(blockID flow.Identifier) (uint32, error)

	// GetIdentitiesForView returns all the staked nodes in my cluster at a certain block.
	// view specifies the view
	GetIdentitiesForBlockID(blockID flow.Identity) (flow.IdentityList, error)

	// GetIdentitiesForBlockIDAndSignerIndexes returns all the staked nodes in my cluster
	// at a certain block for the given signer indexes
	GetIdentitiesForBlockIDAndSignerIndexes(blockID flow.Identifier, signerIndexes bitset.Bytes) (flow.IdentityList, error)

	// GetQCStakeThresholdForBlockID returns the stack threshold for building QC at a given block
	GetQCStakeThresholdForBlockID(blockID flow.Identifier) (uint64, error)

	// LeaderForView get the leader for a certain view
	LeaderForView(view uint64) *flow.Identity
}

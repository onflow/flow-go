package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type ViewState interface {
	// IsSelf returns if a given identity is myself
	IsSelf(id *flow.Identity) bool

	// IsSelfLeaderForView returns if myself is the leader at a given view
	IsSelfLeaderForView(view uint64) bool

	// GetSelfIndexPubKeyForBlockID returns my own identity in all staked node at a given block.
	GetSelfIndexPubKeyForBlockID(blockID flow.Identifier) (*types.IndexedPubKey, error)

	// GetIdentitiesForView returns all the staked nodes in my cluster at a certain block.
	// view specifies the view
	GetIdentitiesForBlockID(blockID flow.Identifier) (flow.IdentityList, error)

	// GetIdentitiesForBlockIDAndSignerIndexes returns all the staked nodes in my cluster
	// at a certain block for the given signer indexes
	GetIdentitiesForBlockIDAndSignerIndexes(blockID flow.Identifier, signerIndexes []bool) (flow.IdentityList, error)

	// GetIdentityForBlockIDAndSignerIndex returns the identity at a certain block for
	// the given signer index
	GetIdentityForBlockIDAndSignerIndex(blockID flow.Identifier, signerIndex uint32) (flow.Identity, error)

	// Combine a slice of signer indexes in to a compact form that represents multiple signer indexes
	// blockID - the block that determines the range of the signer indexes.
	// signerIndexes - the signer indexes to be combined
	CombineSignerIndexes(blockID flow.Identifier, signerIndexes []uint32) ([]bool, error)

	// GetQCStakeThresholdForBlockID returns the stack threshold for building QC at a given block
	GetQCStakeThresholdForBlockID(blockID flow.Identifier) (uint64, error)

	// LeaderForView get the leader for a certain view
	LeaderForView(view uint64) *flow.Identity
}

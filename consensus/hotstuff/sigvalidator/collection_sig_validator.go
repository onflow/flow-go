package sigvalidator

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// ConsensusSigValidator is used by collection cluster to validate the signature of a vote/block.
type CollectionSigValidator struct {
	identity2SignerIndex map[flow.Identifier]int // lookup signer index by identity
}

func NewCollectionSigValidator() *CollectionSigValidator {
	return &CollectionSigValidator{
		identity2SignerIndex: nil,
	}
}

// ValidateVote validate the vote's signature as a staking signature using voter's staking public key.
// It returns:
// - nil if the vote's signature is valid.
// - model.InvalidVoteError if the vote's signature is invalid
// - error if there is other exception
func (v *CollectionSigValidator) ValidateVote(vote *model.Vote) error {
	panic("TO IMPLEMENT")
}

// ValidateVote validate the block's signature as a staking signature using proposer's staking public key.
// It returns:
// - nil if the block proposal's signature is valid.
// - model.InvalidBlockError if the proposal's signature is invalid
// - error if there is other exception
func (v *CollectionSigValidator) ValidateBlock(block *model.Proposal) error {
	panic("TO IMPLEMENT")
}

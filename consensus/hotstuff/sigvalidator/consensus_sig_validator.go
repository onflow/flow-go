package sigvalidator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// ConsensusSigValidator is used by consensus cluster to validate the signature of a vote/block.
// Since the signature field is encoded as bytes, ConsensusSigValidator knows how to decide it back
// into signature data type.
// The signature of a vote/block can either be a staking signature or a random beacon signature.
// The signature validation will first parse which signature type it is, and then perform the
// coresponding validation.
// If the signature type is a Random Beacon sig, it will use the sigVerifier to verify the signature.
type ConsensusSigValidator struct {
	identity2SignerIndex map[flow.Identifier]int     // lookup signer index by identity
	sigVerifier          hotstuff.RandomBeaconSigner // verifies the signature, but has no concept of vote
}

func NewConsensusSigValidator() *ConsensusSigValidator {
	// TODO: replace with real input
	return &ConsensusSigValidator{
		identity2SignerIndex: nil,
		sigVerifier:          nil,
	}
}

// ValidateVote first checks which signature type the vote has. Depending on the signature
// type it will perform the coresponding checks.
// If the signature type is a staking signature, then validate it as a staking signature
// with signer's public key (stateless).
// If the signature type is a random beacon signature, then use the RandomBeaconSigner object
// to verify the signature.
// It returns:
// - (SigTypeStaking, nil) if the vote's signature is a valid staking signature.
// - (SigTypeRandomBeacon, nil) if the vote's signature is a valid random beacon signature.
// - (_, model.InvalidVoteError) if the vote's signature is invalid
// - (_, error) if there is other exception
func (v *ConsensusSigValidator) ValidateVote(vote *model.Vote) (hotstuff.SigType, error) {
	panic("TO IMPLEMENT")
}

// ValidateBlock first checks which signature type the block has.
// If the signature type is a staking signature, then validate it as a staking signature
// with signer's public key(stateless).
// If the signature type is a random beacon signature, then use the RandomBeaconSigner object
// to verify the signature.
// It returns:
// - (SigTypeStaking, nil) if the block's signature is a valid staking signature.
// - (SigTypeRandomBeacon, nil) if the block's signature is a valid random beacon signature.
// - (_, model.InvalidBlockError) if the block's signature is invalid
// - (_, error) if there is other exception
func (v *ConsensusSigValidator) ValidateBlock(block *model.Proposal) (hotstuff.SigType, error) {
	panic("TO IMPLEMENT")
}

package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// SingleVerifier is a verifier capable of verifying a single signature in the
// signature data for its validity. It is used with an aggregating signature scheme.
type SingleVerifier struct {
	committee      hotstuff.Committee
	verifier       module.AggregatingVerifier
	keysAggregator *stakingKeysAggregator
}

// NewSingleVerifier creates a new single verifier with the given dependencies:
// - the hotstuff committee's state is used to get the public staking key for signers;
// - the verifier is used to verify the signatures against the message;
func NewSingleVerifier(committee hotstuff.Committee, verifier module.AggregatingVerifier) *SingleVerifier {
	s := &SingleVerifier{
		committee:      committee,
		verifier:       verifier,
		keysAggregator: newStakingKeysAggregator(),
	}
	return s
}

// VerifyVote verifies a vote with a single signature as signature data.
func (s *SingleVerifier) VerifyVote(voter *flow.Identity, sigData []byte, block *model.Block) (bool, error) {

	// create the message we verify against and check signature
	msg := MakeVoteMessage(block.View, block.BlockID)
	valid, err := s.verifier.Verify(msg, sigData, voter.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

// VerifyQC verifies a QC with a single aggregated signature as signature data.
func (s *SingleVerifier) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {

	// create the message we verify against and check signature
	msg := MakeVoteMessage(block.View, block.BlockID)

	// compute the aggregated key of signers
	aggregatedKey, err := s.keysAggregator.aggregatedStakingKey(signers)
	if err != nil {
		return false, fmt.Errorf("could not compute BLS key: %w", err)
	}

	valid, err := s.verifier.Verify(msg, sigData, aggregatedKey)
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

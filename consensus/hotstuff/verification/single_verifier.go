package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module"
)

// SingleVerifier is a verifier capable of verifying a single signature in the
// signature data for its validity. It is used with an aggregating signature scheme.
type SingleVerifier struct {
	committee hotstuff.Committee
	verifier  module.AggregatingVerifier
}

// NewSingleVerifier creates a new single verifier with the given dependencies:
// - the hotstuff committee's state is used to get the public staking key for signers;
// - the verifier is used to verify the signatures against the message;
func NewSingleVerifier(committee hotstuff.Committee, verifier module.AggregatingVerifier) *SingleVerifier {
	s := &SingleVerifier{
		committee: committee,
		verifier:  verifier,
	}
	return s
}

// VerifyVote verifies a vote with a single signature as signature data.
func (s *SingleVerifier) VerifyVote(voterID flow.Identifier, sigData []byte, block *model.Block) (bool, error) {

	// get the participants from the selector set
	participants, err := s.committee.Identities(block.BlockID, filter.Any)
	if err != nil {
		return false, fmt.Errorf("error retrieving consensus participants for block %x: %w", block.BlockID, err)
	}

	// get the identity of the voter
	voter, ok := participants.ByNodeID(voterID)
	if !ok {
		return false, fmt.Errorf("voter %x is not a valid consensus participant at block %x: %w", voterID, block.BlockID, model.ErrInvalidSigner)
	}

	// create the message we verify against and check signature
	msg := makeVoteMessage(block.View, block.BlockID)
	valid, err := s.verifier.Verify(msg, sigData, voter.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

// VerifyQC verifies a QC with a single aggregated signature as signature data.
func (s *SingleVerifier) VerifyQC(voterIDs []flow.Identifier, sigData []byte, block *model.Block) (bool, error) {

	// get the full Identities of the signers
	signers, err := s.committee.Identities(block.BlockID, filter.HasNodeID(voterIDs...))
	if err != nil {
		return false, fmt.Errorf("could not get signer identities: %w", err)
	}
	if len(signers) < len(voterIDs) { // check we have valid consensus member Identities for all signers
		return false, fmt.Errorf("some signers are not valid consensus participants at block %x: %w", block.BlockID, model.ErrInvalidSigner)
	}
	signers = signers.Order(order.ByReferenceOrder(voterIDs)) // re-arrange Identities into the same order as in voterIDs

	// create the message we verify against and check signature
	msg := makeVoteMessage(block.View, block.BlockID)
	valid, err := s.verifier.VerifyMany(msg, sigData, signers.StakingKeys())
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

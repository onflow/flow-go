package verification

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// SingleVerifier is a verifier capable of verifying a single signature in the
// signature data for its validity. It is used with an aggregating signature scheme.
type SingleVerifier struct {
	state    protocol.State
	verifier module.AggregatingVerifier
}

// NewSingleVerifier creates a new single verifier with the given dependencies:
// - the protocol state is used to get the public staking key for signers; and
// - the verifier is used to verify the signatures against the message.
func NewSingleVerifier(state protocol.State, verifier module.AggregatingVerifier) *SingleVerifier {
	s := &SingleVerifier{
		state:    state,
		verifier: verifier,
	}
	return s
}

// VerifyProposal verifies a proposal with a single signature as signature data.
func (s *SingleVerifier) VerifyProposal(proposal *model.Proposal) (bool, error) {

	// get the identity of the proposer
	proposer, err := s.state.AtBlockID(proposal.Block.BlockID).Identity(proposal.Block.ProposerID)
	if err != nil {
		return false, fmt.Errorf("could not get proposer identity: %w", err)
	}

	// create the message we verify against and check signature
	msg := messageFromParams(proposal.Block.View, proposal.Block.BlockID)
	valid, err := s.verifier.Verify(msg, proposal.SigData, proposer.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

// VerifyVote verifies a vote with a single signature as signature data.
func (s *SingleVerifier) VerifyVote(vote *model.Vote) (bool, error) {

	// get the identity of the voter
	voter, err := s.state.AtBlockID(vote.BlockID).Identity(vote.SignerID)
	if err != nil {
		return false, fmt.Errorf("could not get voter identity: %w", err)
	}

	// create the message we verify against and check signature
	msg := messageFromParams(vote.View, vote.BlockID)
	valid, err := s.verifier.Verify(msg, vote.SigData, voter.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

// VerifyQC verifies a QC with a single aggregated signature as signature data.
func (s *SingleVerifier) VerifyQC(qc *model.QuorumCertificate) (bool, error) {

	// get the identities of the signers
	signers, err := s.state.AtBlockID(qc.BlockID).Identities(filter.HasNodeID(qc.SignerIDs...))
	if err != nil {
		return false, fmt.Errorf("could not get signer identities: %w", err)
	}

	// create the message we verify against and check signature
	msg := messageFromParams(qc.View, qc.BlockID)
	valid, err := s.verifier.VerifyMany(msg, qc.SigData, signers.StakingKeys())
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

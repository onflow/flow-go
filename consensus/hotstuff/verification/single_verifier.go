package verification

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// SingleVerifier is a verifier capable of verifying a single signature in the
// signature data for its validity. It is used with an aggregating signature scheme.
type SingleVerifier struct {
	state    protocol.State
	verifier module.AggregatingVerifier
	selector flow.IdentityFilter
}

// NewSingleVerifier creates a new single verifier with the given dependencies:
// - the protocol state is used to get the public staking key for signers; and
// - the verifier is used to verify the signatures against the message.
func NewSingleVerifier(state protocol.State, verifier module.AggregatingVerifier, selector flow.IdentityFilter) *SingleVerifier {
	s := &SingleVerifier{
		state:    state,
		verifier: verifier,
		selector: selector,
	}
	return s
}

// VerifyProposal verifies a proposal with a single signature as signature data.
func (s *SingleVerifier) VerifyProposal(proposal *model.Proposal) (bool, error) {

	// get the participants from the selector set
	participants, err := s.state.AtBlockID(proposal.Block.BlockID).Identities(s.selector)
	if err != nil {
		return false, fmt.Errorf("could not get participants selector set: %w", err)
	}

	// get the identity of the proposer
	proposer, ok := participants.ByNodeID(proposal.Block.ProposerID)
	if !ok {
		return false, fmt.Errorf("proposer is not part of selector set (proposer: %x): %w", proposal.Block.ProposerID, ErrInvalidSigner)
	}

	// create the message we verify against and check signature
	msg := makeVoteMessage(proposal.Block.View, proposal.Block.BlockID)
	valid, err := s.verifier.Verify(msg, proposal.SigData, proposer.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

// VerifyVote verifies a vote with a single signature as signature data.
func (s *SingleVerifier) VerifyVote(vote *model.Vote) (bool, error) {

	// get the participants from the selector set
	participants, err := s.state.AtBlockID(vote.BlockID).Identities(s.selector)
	if err != nil {
		return false, fmt.Errorf("could not get participants selector set: %w", err)
	}

	// get the identity of the voter
	voter, ok := participants.ByNodeID(vote.SignerID)
	if !ok {
		return false, fmt.Errorf("voter is not part of selector set (voter: %x): %w", vote.SignerID, ErrInvalidSigner)
	}

	// create the message we verify against and check signature
	msg := makeVoteMessage(vote.View, vote.BlockID)
	valid, err := s.verifier.Verify(msg, vote.SigData, voter.StakingPubKey)
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

// VerifyQC verifies a QC with a single aggregated signature as signature data.
func (s *SingleVerifier) VerifyQC(qc *model.QuorumCertificate) (bool, error) {

	// get the signers of the QC
	selector := filter.And(s.selector, filter.HasNodeID(qc.SignerIDs...))
	signers, err := s.state.AtBlockID(qc.BlockID).Identities(selector)
	if err != nil {
		return false, fmt.Errorf("could not get signer identities: %w", err)
	}

	// check they were all in the selector set
	if len(signers) < len(qc.SignerIDs) {
		return false, fmt.Errorf("not all signers are part of the selector set (signers: %d, selector: %d): %w", len(qc.SignerIDs), len(signers), ErrInvalidSigner)
	}

	// create the message we verify against and check signature
	signers = signers.Order(order.ByReferenceOrder(qc.SignerIDs))
	msg := makeVoteMessage(qc.View, qc.BlockID)
	valid, err := s.verifier.VerifyMany(msg, qc.SigData, signers.StakingKeys())
	if err != nil {
		return false, fmt.Errorf("could not verify signature: %w", err)
	}

	return valid, nil
}

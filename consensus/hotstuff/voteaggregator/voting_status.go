package voteaggregator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// TODO: to be removed and replaced by vote aggregator v2
// VotingStatus keeps track of incorporated votes for the same block
type VotingStatus struct {
	signer           hotstuff.SignerVerifier
	block            *model.Block
	stakeThreshold   uint64
	accumulatedStake uint64
	// assume votes are all valid to build QC
	votes map[flow.Identifier]*model.Vote
}

// NewVotingStatus creates a new Voting Status instance
func NewVotingStatus(block *model.Block, stakeThreshold uint64, signer hotstuff.SignerVerifier) *VotingStatus {
	return &VotingStatus{
		signer:           signer,
		block:            block,
		stakeThreshold:   stakeThreshold,
		accumulatedStake: 0,
		votes:            make(map[flow.Identifier]*model.Vote),
	}
}

// AddVote add votes to the list, and accumulates the stake
// assume votes are valid.
// duplicate votes will not be accumulated again
func (vs *VotingStatus) AddVote(vote *model.Vote, voter *flow.Identity) {
	_, exists := vs.votes[vote.ID()]
	if exists {
		return
	}
	vs.votes[vote.ID()] = vote
	// - We assume different votes with different vote ID must belong to different signers.
	//   This requires the signature-scheme to be deterministic.
	// - Deterministic means that signing on the same data twice with the same private key
	//   will generate the same signature.
	// - The assumption currently holds because we are using BLS signatures, and
	//   BLS signature is deterministic.
	// - If the signature-scheme is non-deterministic, for instance: ECDSA, then we need to
	//   check votes are from unique signers here, only accumulate stakes if the vote is
	//   from a new signer that never seen
	vs.accumulatedStake += voter.Stake
}

// CanBuildQC check whether the
func (vs *VotingStatus) CanBuildQC() bool {
	return vs.hasEnoughStake()
}

// TryBuildQC returns a QC if the existing votes are enought to build a QC, otherwise
// an error will be returned.
func (vs *VotingStatus) TryBuildQC() (*flow.QuorumCertificate, bool, error) {

	// check if there are enough votes to build QC
	if !vs.CanBuildQC() {
		return nil, false, nil
	}

	panic("To be removed")

	// build the aggregated signature
	//votes := getSliceForVotes(vs.votes)
	//qc, err := vs.signer.CreateQC(votes)
	//if errors.Is(err, signature.ErrInsufficientShares) {
	//	return nil, false, nil
	//}
	//if err != nil {
	//	return nil, false, fmt.Errorf("could not create QC from votes: %w", err)
	//}
	//
	//return qc, true, nil
}

func (vs *VotingStatus) hasEnoughStake() bool {
	return vs.accumulatedStake >= vs.stakeThreshold
}

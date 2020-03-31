package voteaggregator

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// VotingStatus keeps track of incorporated votes for the same block
type VotingStatus struct {
	signer           hotstuff.Signer
	block            *model.Block
	stakeThreshold   uint64
	groupSize        uint
	accumulatedStake uint64
	// assume votes are all valid to build QC
	votes map[flow.Identifier]*model.Vote
}

// NewVotingStatus creates a new Voting Status instance
func NewVotingStatus(block *model.Block, stakeThreshold uint64, groupSize uint, signer hotstuff.Signer) *VotingStatus {
	return &VotingStatus{
		signer:           signer,
		block:            block,
		stakeThreshold:   stakeThreshold,
		groupSize:        groupSize,
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

// CanBuildQC check whether it has collected enough signatures from the votes to build a QC
func (vs *VotingStatus) CanBuildQC() bool {
	return vs.hasEnoughStake() && vs.hasEnoughSigShares()
}

// TryBuildQC returns a QC if the existing votes are enought to build a QC, otherwise
// an error will be returned.
func (vs *VotingStatus) TryBuildQC() (*model.QuorumCertificate, bool, error) {
	// check if there are enough votes to build QC
	if !vs.CanBuildQC() {
		return nil, false, nil
	}

	// build the aggregated signature
	votes := getSliceForVotes(vs.votes)
	qc, err := vs.signer.CreateQC(votes)
	if err != nil {
		return nil, false, fmt.Errorf("could not create QC from votes: %w", err)
	}

	return qc, true, nil
}

func (vs *VotingStatus) hasEnoughStake() bool {
	return vs.accumulatedStake >= vs.stakeThreshold
}

func (vs *VotingStatus) hasEnoughSigShares() bool {
	return crypto.EnoughShares(int(vs.groupSize), len(vs.votes))
}

func getSliceForVotes(votes map[flow.Identifier]*model.Vote) []*model.Vote {
	var voteSlice = make([]*model.Vote, 0, len(votes))

	for _, vote := range votes {
		voteSlice = append(voteSlice, vote)
	}

	return voteSlice
}

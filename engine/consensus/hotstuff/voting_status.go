package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// VotingStatus keeps track of incorporated votes for the same block
type VotingStatus struct {
	sigAggregator    SigAggregator
	block            *hotstuff.Block
	stakeThreshold   uint64
	accumulatedStake uint64
	// assume votes are all valid to build QC
	votes map[flow.Identifier]*hotstuff.Vote
}

// NewVotingStatus creates a new Voting Status instance
func NewVotingStatus(block *hotstuff.Block, stakeThreshold uint64, sigAggregator SigAggregator) *VotingStatus {
	return &VotingStatus{
		sigAggregator:    sigAggregator,
		block:            block,
		stakeThreshold:   stakeThreshold,
		accumulatedStake: 0,
		votes:            make(map[flow.Identifier]*hotstuff.Vote),
	}
}

// AddVote add votes to the list, and accumulates the stake
// assume votes are valid.
// duplicate votes will not be accumulated again
func (vs *VotingStatus) AddVote(vote *hotstuff.Vote, voter *flow.Identity) {
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
func (vs *VotingStatus) TryBuildQC() (*hotstuff.QuorumCertificate, bool, error) {
	// check if there are enough votes to build QC
	if !vs.CanBuildQC() {
		return nil, false, nil
	}

	// build the aggregated signature
	sigs := getSigsSliceFromVotes(vs.votes)
	aggregatedSig, err := vs.sigAggregator.Aggregate(vs.block, sigs)
	if err != nil {
		return nil, false, fmt.Errorf("could not build aggregate signatures for building QC: %w", err)
	}

	// build the QC
	qc := &hotstuff.QuorumCertificate{
		View:                vs.block.View,
		BlockID:             vs.block.BlockID,
		AggregatedSignature: aggregatedSig,
	}

	return qc, true, nil
}

func (vs *VotingStatus) hasEnoughStake() bool {
	return vs.accumulatedStake >= vs.stakeThreshold
}

func (vs *VotingStatus) hasEnoughSigShares() bool {
	return vs.sigAggregator.CanReconstruct(len(vs.votes))
}

func getSigsSliceFromVotes(votes map[flow.Identifier]*hotstuff.Vote) []*hotstuff.SingleSignature {
	var signatures = make([]*hotstuff.SingleSignature, 0, len(votes))

	for _, vote := range votes {
		signatures = append(signatures, vote.Signature)
	}

	return signatures
}

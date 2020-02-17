package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// VotingStatus keeps track of incorporated votes for the same block
type VotingStatus struct {
	sigAggregator    SigAggregator
	blockID          flow.Identifier
	signerCount      uint32
	view             uint64
	thresholdStake   uint64
	accumulatedStake uint64
	// assume votes are all valid to build QC
	votes map[flow.Identifier]*hotstuff.Vote
}

func NewVotingStatus(sigAggregator SigAggregator, thresholdStake uint64, view uint64, signerCount uint32, voter *flow.Identity, blockID flow.Identifier) *VotingStatus {
	return &VotingStatus{
		sigAggregator:  sigAggregator,
		thresholdStake: thresholdStake,
		view:           view,
		signerCount:    signerCount,
		blockID:        blockID,
		votes:          make(map[flow.Identifier]*hotstuff.Vote),
	}
}

// assume votes are valid
// duplicate votes will not be accumulated again
func (vs *VotingStatus) AddVote(vote *hotstuff.Vote, voter *flow.Identity) {
	_, exists := vs.votes[vote.ID()]
	if exists {
		return
	}
	vs.votes[vote.ID()] = vote
	vs.accumulatedStake += voter.Stake
}

func (vs *VotingStatus) CanBuildQC() bool {
	return vs.accumulatedStake >= vs.thresholdStake
}

// TryBuildQC returns a QC if the existing votes are enought to build a QC, otherwise
// an error will be returned.
func (vs *VotingStatus) TryBuildQC() (*hotstuff.QuorumCertificate, error) {
	// check if there are enough votes to build QC
	if !vs.CanBuildQC() {
		return nil, fmt.Errorf("could not build QC: %w", hotstuff.ErrInsufficientVotes)
	}

	// build the aggregated signature
	aggregatedSig, err := vs.aggregateSig()
	if err != nil {
		return nil, fmt.Errorf("could not build aggregate signatures for building QC: %w", err)
	}

	// build the QC
	qc := &hotstuff.QuorumCertificate{
		View:                vs.view,
		BlockID:             vs.blockID,
		AggregatedSignature: aggregatedSig,
	}

	return qc, nil
}

func (vs *VotingStatus) aggregateSig() (*hotstuff.AggregatedSignature, error) {
	sigs := getSigsSliceFromVotes(vs.votes)
	return vs.sigAggregator.Aggregate(sigs)
}

func getSigsSliceFromVotes(votes map[flow.Identifier]*hotstuff.Vote) []*hotstuff.SingleSignature {
	var signatures = make([]*hotstuff.SingleSignature, 0, len(votes))

	for _, vote := range votes {
		signatures = append(signatures, vote.Signature)
	}

	return signatures
}

// FromSignatures builds an aggregated signature from a slice of signature and a signerCount
// sigs is the slice of signatures from all the signers
// signers is the flag from the entire identity list for who signed it and who didn't.
func FromSignatures(sigs []*hotstuff.SingleSignature, signerCount uint32) (*hotstuff.AggregatedSignature, error) {
	panic("TODO")
}

package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
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
	votes map[flow.Identifier]*types.Vote
}

func NewVotingStatus(sigAggregator SigAggregator, thresholdStake uint64, view uint64, signerCount uint32, voter *flow.Identity, blockID flow.Identifier) *VotingStatus {
	return &VotingStatus{
		sigAggregator:  sigAggregator,
		thresholdStake: thresholdStake,
		view:           view,
		signerCount:    signerCount,
		blockID:        blockID,
		votes:          make(map[flow.Identifier]*types.Vote),
	}
}

// assume votes are valid
// duplicate votes will not be accumulated again
func (vs *VotingStatus) AddVote(vote *types.Vote, voter *flow.Identity) {
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

// TryBuildQC returns a QC if the existing votes are enough to build a QC
// returns false if the votes are insufficient
func (vs *VotingStatus) TryBuildQC() (*types.QuorumCertificate, bool, error) {
	// check if there are enough votes to build QC
	if !vs.CanBuildQC() {
		return nil, false, nil
	}

	// build the aggregated signature
	aggregatedSig, err := vs.aggregateSig()
	if err != nil {
		return nil, false, fmt.Errorf("could not build aggregate signatures for building QC: %w", err)
	}

	// build the QC
	qc := &types.QuorumCertificate{
		View:                vs.view,
		BlockID:             vs.blockID,
		AggregatedSignature: aggregatedSig,
	}

	return qc, true, nil
}

func (vs *VotingStatus) aggregateSig() (*types.AggregatedSignature, error) {
	sigs := getSigsSliceFromVotes(vs.votes)
	return vs.sigAggregator.Aggregate(sigs)
}

func getSigsSliceFromVotes(votes map[flow.Identifier]*types.Vote) []*types.SingleSignature {
	var signatures = make([]*types.SingleSignature, 0, len(votes))

	for _, vote := range votes {
		signatures = append(signatures, vote.Signature)
	}

	return signatures
}

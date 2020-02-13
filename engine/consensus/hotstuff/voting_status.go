package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// VotingStatus keeps track of incorporated votes for the same block
type VotingStatus struct {
	blockID          flow.Identifier
	signerCount      uint32
	view             uint64
	thresholdStake   uint64
	accumulatedStake uint64
	// assume votes are all valid to build QC
	votes map[flow.Identifier]*types.Vote
}

func NewVotingStatus(thresholdStake uint64, view uint64, signerCount uint32, voter *flow.Identity, blockID flow.Identifier) *VotingStatus {
	return &VotingStatus{
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

func (vs *VotingStatus) TryBuildQC() (*types.QuorumCertificate, bool, error) {
	sigs := vs.getSigsSliceFromVotes()
	aggregatedSig, err := FromSignatures(sigs, vs.signerCount)
	if err != nil {
		return nil, false, fmt.Errorf("could not build QC: %w", err)
	}
	if !vs.CanBuildQC() {
		return nil, false, nil
	}
	qc := &types.QuorumCertificate{
		View:                vs.view,
		BlockID:             vs.blockID,
		AggregatedSignature: aggregatedSig,
	}

	return qc, true, nil
}

func (vs *VotingStatus) getSigsSliceFromVotes() []*types.SingleSignature {
	var signatures = make([]*types.SingleSignature, len(vs.votes))
	i := 0
	for _, vote := range vs.votes {
		signatures[i] = vote.Signature
		i++
	}

	return signatures
}

// FromSignatures builds an aggregated signature from a slice of signature and a signerCount
// sigs is the slice of signatures from all the signers
// signers is the flag from the entire identity list for who signed it and who didn't.
func FromSignatures(sigs []*types.SingleSignature, signerCount uint32) (*types.AggregatedSignature, error) {
	panic("TODO")
}

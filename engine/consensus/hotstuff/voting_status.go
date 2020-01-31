package hotstuff

import (
	"fmt"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type VotingStatus struct {
	blockMRH         flow.Identifier
	signerCount      uint32
	view             uint64
	thresholdStake   uint64
	accumulatedStake uint64
	voteSender       *flow.Identity
	// assume votes are all valid to build QC
	votes map[string]*types.Vote
}

func NewVotingStatus(thresholdStake uint64, view uint64, signerCount uint32, voteSender *flow.Identity, blockMRH flow.Identifier) *VotingStatus {
	return &VotingStatus{
		thresholdStake: thresholdStake,
		view:           view,
		signerCount:    signerCount,
		voteSender:     voteSender,
		blockMRH:       blockMRH,
		votes:          map[string]*types.Vote{},
	}
}

// assume votes are valid
// duplicate votes will not be accumulated again
func (vs *VotingStatus) AddVote(vote *types.Vote) {
	_, exists := vs.votes[string(vote.ID())]
	if exists {
		return
	}
	vs.votes[string(vote.ID())] = vote
	vs.accumulatedStake += vs.voteSender.Stake
}

func (vs *VotingStatus) CanBuildQC() bool {
	return vs.accumulatedStake >= vs.thresholdStake
}

func (vs *VotingStatus) BlockID() flow.Identifier {
	return vs.blockMRH
}

func (vs *VotingStatus) tryBuildQC() (*types.QuorumCertificate, error) {
	sigs := vs.getSigsSliceFromVotes()
	if !vs.CanBuildQC() {
		return nil, fmt.Errorf("could not build QC: %w", types.ErrInsufficientVotes{})
	}
	aggregatedSig, err := types.FromSignatures(sigs, vs.signerCount)
	if err != nil {
		return nil, fmt.Errorf("could not build QC: %w", err)
	}
	qc := &types.QuorumCertificate{
		View:                vs.view,
		BlockID:             vs.BlockID(),
		AggregatedSignature: aggregatedSig,
	}

	return qc, nil
}

func (vs *VotingStatus) getSigsSliceFromVotes() []*types.Signature {
	var signatures = make([]*types.Signature, len(vs.votes))
	i := 0
	for _, vote := range vs.votes {
		signatures[i] = vote.Signature
		i++
	}

	return signatures
}

package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type VotingStatus struct {
	blockMRH         flow.Identifier
	thresholdStake   uint64
	accumulatedStake uint64
	voteSender       *flow.Identity
	// assume votes are all valid to build QC
	votes map[string]*types.Vote
}

func NewVotingStatus(thresholdStake uint64, voteSender *flow.Identity, blockMRH flow.Identifier) *VotingStatus {
	return &VotingStatus{
		thresholdStake: thresholdStake,
		voteSender:     voteSender,
		blockMRH:       blockMRH,
		votes:          map[string]*types.Vote{},
	}
}

// assume vote are valid
func (vs *VotingStatus) AddVote(vote *types.Vote) {
	vs.votes[vote.Hash()] = vote
	vs.accumulatedStake += vs.voteSender.Stake
}

func (vs *VotingStatus) CanBuildQC() bool {
	return vs.accumulatedStake >= vs.thresholdStake
}

func (vs *VotingStatus) BlockID() flow.Identifier {
	return vs.blockMRH
}

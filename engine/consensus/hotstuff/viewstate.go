package hotstuff

import (
	"fmt"
	"math/big"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/protocol"
)

// ViewState is a wrapper of protocolState to provide API for querying view related state
type ViewState struct {
	protocolState protocol.State

	// my own identifier
	myID flow.Identifier
	// my role. Could be flow.Role.Consensus or flow.Role.Collection
	myRole flow.Role
}

// IsSelf returns if a given identity is myself
func (v *ViewState) IsSelf(id *flow.Identity) bool {
	return id.ID() == v.myID
}

// IsSelfLeaderForView returns if myself is the leader at a given view
func (v *ViewState) IsSelfLeaderForView(view uint64) bool {
	leader := v.LeaderForView(view)
	return v.IsSelf(leader)
}

// GetSelfIdxForBlockID returns my own index in all staked node at a given block.
func (v *ViewState) GetSelfIdxForBlockID(blockID flow.Identifier) (uint32, error) {
	identities, err := v.GetIdentitiesForBlockID(blockID)
	if err != nil {
		return 0, err
	}
	// TODO: using the index might be volnerble for attacks that has a big long list of identities
	for idx, id := range identities {
		if v.IsSelf(id) {
			return uint32(idx), nil
		}
	}
	return 0, fmt.Errorf("can not found my index at blockID:%v", blockID)
}

// GetIdentitiesForView returns all the staked nodes for my role at a certain block.
// view specifies the view
func (v *ViewState) GetIdentitiesForBlockID(blockID flow.Identifier) (flow.IdentityList, error) {
	return v.protocolState.AtBlockID(blockID).Identities(identity.HasRole(v.myRole))
}

// GetQCStakeThresholdForBlockID returns the stack threshold for building QC at a given block
func (v *ViewState) GetQCStakeThresholdForBlockID(blockID flow.Identifier) (uint64, error) {
	identities, err := v.GetIdentitiesForBlockID(blockID)
	if err != nil {
		return 0, err
	}
	return ComputeStakeThresholdForBuildingQC(identities), nil
}

// ComputeStakeThresholdForBuildingQC returns the threshold to determine how much stake are needed for building a QC
// identities is the full identity list at a certain block
func ComputeStakeThresholdForBuildingQC(identities flow.IdentityList) uint64 {
	// total * 2 / 3
	total := new(big.Int).SetUint64(identities.TotalStake())
	two := new(big.Int).SetUint64(2)
	three := new(big.Int).SetUint64(3)
	return new(big.Int).Div(
		new(big.Int).Mul(total, two),
		three).Uint64()
}

// LeaderForView get the leader for a certain view
func (v *ViewState) LeaderForView(view uint64) *flow.Identity {
	panic("TODO")
}

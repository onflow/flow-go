package hotstuff

import (
	"fmt"

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

// GetSelfIdxForView returns my own index in all staked node at a given view.
func (v *ViewState) GetSelfIdxForView(view uint64) (uint32, error) {
	identities, err := v.GetIdentitiesForView(view)
	if err != nil {
		return 0, err
	}
	// TODO: using the index from the for loop is not reliable
	for idx, id := range identities {
		if v.IsSelf(id) {
			return uint32(idx), nil
		}
	}
	return 0, fmt.Errorf("can not found my index in view:%v", view)
}

// GetIdentitiesForView returns all the staked nodes for my role at a certain view.
// view specifies the view
func (v *ViewState) GetIdentitiesForView(view uint64) (flow.IdentityList, error) {
	return v.protocolState.AtNumber(view).Identities(identity.HasRole(v.myRole))
}

// LeaderForView get the leader for a certain view
func (v *ViewState) LeaderForView(view uint64) *flow.Identity {
	panic("TODO")
}

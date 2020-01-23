package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/protocol"
)

type ViewState struct {
	protocolState protocol.State
}

func (v *ViewState) IsSelf(id flow.Identity) bool {
	panic("TODO")
}

func (v *ViewState) IsSelfLeaderForView(view uint64) bool {
	panic("TODO")
}

func (v *ViewState) GetSelfIdxForView(view uint64) uint32 {
	panic("TODO")
}

func (v *ViewState) GetIdxOfPubKeyForView(view uint64) uint32 {
	panic("TODO")
}

// GetIdentitiesForView returns all the staked nodes for a specific role at a certain view.
// view specifies the view
// role specifies the role of the nodes. Could be Role.Consensus or Role.Collection
func (v *ViewState) GetIdentitiesForView(view uint64, role flow.Role) (flow.IdentityList, error) {
	return v.protocolState.AtNumber(view).Identities(identity.HasRole(role))
}

func (v *ViewState) LeaderForView(view uint64) flow.Identity {
	panic("TODO")
}

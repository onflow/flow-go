package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/protocol"
)

type ViewState struct {
	protocalState protocol.State
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

func (v *ViewState) GetIdentitiesForView(view uint64) (flow.IdentityList, error) {
	return v.protocolState.AtNumber(view).Identities(identity.HasRole(flow.RoleConsensus))
}

func (v *ViewState) LeaderForView(view uint64) flow.Identity {
	panic("TODO")
}

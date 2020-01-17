package viewstate

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
)

type ViewState struct {
	ps protocol.State
	me module.Local
}

// IsSelf(id types.ID) bool
// IsSelfLeaderForView(view uint64) bool
// GetSelfIdxForView(view uint64) uint32
// GetIdxOfPubKeyForView(view uint64) uint32
// LeaderForView(view uint64) types.ID

func (vs *ViewState) IsSelf(id flow.Identifier) bool {
	return id == vs.me.NodeID()
}

func (vs *ViewState) GetIdentitiesByRole(role flow.IdentityList) bool {
	vs.ps
	func(node *flow.Identity) {
		node.Role == flow.RoleConsensus
	}
}

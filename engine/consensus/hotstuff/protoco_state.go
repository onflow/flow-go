package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type ProtocolState interface {
	IsSelf(id types.ID) bool
	IsSelfLeaderForView(view uint64) bool
	LeaderForView(view uint64) ID
}

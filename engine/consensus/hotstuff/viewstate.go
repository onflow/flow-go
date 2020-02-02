package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type ViewState interface {
	IsSelf(id *flow.Identity) bool
	IsSelfLeaderForView(view uint64) bool
	GetSelfIdxForView(view uint64) uint32
	GetIdxOfPubKeyForView(view uint64) uint32
	LeaderForView(view uint64) *flow.Identity
}

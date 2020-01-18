package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type ViewState interface {
	IsSelf(id types.ID) bool
	IsSelfLeaderForView(view uint64) bool
	LeaderForView(view uint64) types.ID
	GetSelfIdxForBlockID(blockID flow.Identifier) uint32
	GetIdxOfPubKeyForBlockID(blockID flow.Identifier) uint32
}

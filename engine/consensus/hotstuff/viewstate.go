package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type ViewState interface {
	IsSelf(id types.ID) bool
	IsSelfLeaderForView(view uint64) bool
	ThresholdStake(view uint64) float32
	GetSelfIdxForView(view uint64) uint32
	GetIdxOfPubKeyForView(view uint64) uint32
	LeaderForView(view uint64) types.ID
	GetIdentitiesAtBlockID(blockID []byte) (flow.IdentityList, error)
	ComputeQCStakeThresholdAtBlockID(blockID []byte) uint64
}

package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

type FollowerLogic interface {
	FinalizedBlock() *model.Block
	AddBlock(proposal *model.Proposal) error
}

package forks

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Finalizer interface {
	VerifyBlock(*model.Block) error
	IsSafeBlock(*model.Block) bool
	AddBlock(*model.Block) error
	GetBlock(blockID flow.Identifier) (*model.Block, bool)
	GetBlocksForView(view uint64) []*model.Block
	FinalizedBlock() *model.Block
	LockedBlock() *model.Block
}

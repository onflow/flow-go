package forks

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Finalizer interface {
	VerifyBlock(*types.Block) error
	IsSafeBlock(*types.Block) bool
	AddBlock(*types.Block) error
	GetBlock(blockID flow.Identifier) (*types.Block, bool)
	GetBlocksForView(view uint64) []*types.Block
	FinalizedBlock() *types.Block
}

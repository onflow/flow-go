package forks

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

type Finalizer interface {
	VerifyBlock(*hotstuff.Block) error
	IsSafeBlock(*hotstuff.Block) bool
	AddBlock(*hotstuff.Block) error
	GetBlock(blockID flow.Identifier) (*hotstuff.Block, bool)
	GetBlocksForView(view uint64) []*hotstuff.Block
	FinalizedBlock() *hotstuff.Block
	LockedBlock() *hotstuff.Block
}

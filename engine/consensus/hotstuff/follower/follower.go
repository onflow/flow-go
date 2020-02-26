package follower

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	models "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module"
)

// Follower runs in non-consensus nodes. It informs other components within the node
// about finalization of blocks. The consensus Follower consumes all block proposals
// broadcasts by the consensus node, verifies the block header and locally evaluates
// the finalization rules.
//
// CAUTION: Follower is NOT CONCURRENCY safe
type Follower struct {
	finalizationLogic forks.Finalizer
	finalizationCallback module.Finalizer
	notifier finalizer.FinalizationConsumer
	validator hotstuff.Validator
}

func New(trustedRoot *forks.BlockQC, finalizationCallback module.Finalizer, notifier finalizer.FinalizationConsumer) (*Follower, error) {
	finalizationLogic, err := finalizer.New(trustedRoot, finalizationCallback, notifier)
	if err != nil {
		return nil, fmt.Errorf("initialization of consensus follower failed: %w", err)
	}




	return &Follower{
		finalizationLogic:    finalizationLogic,
		finalizationCallback: finalizationCallback,
		notifier:             notifier,
	}, nil
}

func (f *Follower) FinalizedBlock() *models.Block {
	return f.finalizationLogic.FinalizedBlock()
}

func (f *Follower) FinalizedView() uint64 {
	return f.FinalizedBlock().View
}
//
//func (f *Follower) AddBlock(block *hotstuff.Block) error {
//	if err := f.finalizationLogic.VerifyBlock(block); err != nil {
//		return fmt.Errorf("cannot add invalid block to Forks: %w", err)
//	}
//	err := f.finalizationLogic.AddBlock(block)
//	if err != nil {
//		return fmt.Errorf("error storing block in Forks: %w", err)
//	}
//
//	// We only process the block's QC if the block's view is larger than the last finalized block.
//	// By ignoring hte qc's of block's at or below the finalized view, we allow the genesis block
//	// to have a nil QC.
//	if block.View <= f.finalizationLogic.FinalizedBlock().View {
//		return nil
//	}
//	return f.AddQC(block.QC)
//}

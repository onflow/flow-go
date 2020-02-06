package forkchoice

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// NewestForkChoice implements HotStuff's original fork choice rule:
// always use the newest QC (i.e. the QC with highest view number)
type NewestForkChoice struct {
	// preferredParent stores the preferred parent to build a block on top of. It contains the
	// parent block as well as the QC POINTING to the parent, which can be used to build the block.
	// preferredParent.QC called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	preferredParent *types.QCBlock
	finalizer       *finalizer.Finalizer
	notifier        notifications.Distributor
}

func NewNewestForkChoice(finalizer *finalizer.Finalizer, notifier notifications.Distributor) (ForkChoice, error) {
	fc := &NewestForkChoice{
		finalizer: finalizer,
		notifier:  notifier,
	}
	var err error
	fc.preferredParent, err = types.NewQcBlock(finalizer.FinalizedBlockQC(), finalizer.FinalizedBlock())
	if err != nil {
		return nil, fmt.Errorf("cannot create NewestForkChoice: %w", err)
	}
	notifier.OnQcIncorporated(finalizer.FinalizedBlockQC())
	return fc, nil
}

// MakeForkChoice prompts the ForkChoice to generate a fork choice for the
// current view `curView`. NewestForkChoice always returns the qc with the largest
// view number seen.
//
// PREREQUISITE:
// ForkChoice cannot generate ForkChoices retroactively for past views.
// If used correctly, MakeForkChoice should only ever have processed QCs
// whose view is smaller than curView, for the following reason:
// Processing a QC with view v should result in the PaceMaker being in
// view v+1 or larger. Hence, given that the current View is curView,
// all QCs should have view < curView.
// To prevent accidental miss-usage, ForkChoices will error if `curView`
// is smaller than the view of any qc ForkChoice has seen.
// Note that tracking the view of the newest qc is for safety purposes
// and _independent_ of the fork-choice rule.
func (fc *NewestForkChoice) MakeForkChoice(curView uint64) (*types.QCBlock, error) {
	choice := fc.preferredParent
	if choice.View() >= curView {
		// sanity check;
		// processing a QC with view v should result in the PaceMaker being in view v+1 or larger
		// Hence, given that the current View is curView, all QCs should have view < curView
		return nil, fmt.Errorf(
			"ForkChoice slected qc with view %d which is larger than requested view %d",
			choice.View(), curView,
		)
	}
	fc.notifier.OnForkChoiceGenerated(curView, choice.QC())
	return choice, nil
}

// updateQC updates `preferredParent` according to the fork-choice rule.
// Currently, we implement 'Chained HotStuff Protocol' where the fork-choice
// rule is: "build on newest QC"
func (fc *NewestForkChoice) AddQC(qc *types.QuorumCertificate) error {
	if qc.View <= fc.preferredParent.View() {
		// Per construction, preferredParent.View() is always larger than the last finalized block's view.
		// Hence, this check suffices to drop all QCs with qc.View <= last_finalized_block.View().
		return nil
	}

	// Have qc.View > last_finalized_block.View(). Hence, block referenced by qc should be stored:
	block, err := fc.ensureBlockStored(qc)
	if err != nil {
		return fmt.Errorf("cannot add QC: %w", err)
	}
	fc.preferredParent, err = types.NewQcBlock(qc, block)
	if err != nil {
		return fmt.Errorf("cannot add QC: %w", err)
	}
	fc.notifier.OnQcIncorporated(qc)

	return nil
}

func (fc *NewestForkChoice) ensureBlockStored(qc *types.QuorumCertificate) (*types.BlockProposal, error) {
	block, haveBlock := fc.finalizer.GetBlock(qc.BlockID)
	if !haveBlock {
		return nil, &types.ErrorMissingBlock{View: qc.View, BlockID: qc.BlockID}
	}
	if block.View() != qc.View {
		return nil, fmt.Errorf("invalid qc with mismatching view")
	}
	return block, nil
}

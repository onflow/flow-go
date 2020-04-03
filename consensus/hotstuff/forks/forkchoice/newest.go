package forkchoice

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
)

// NewestForkChoice implements HotStuff's original fork choice rule:
// always use the newest QC (i.e. the QC with highest view number)
type NewestForkChoice struct {
	// preferredParent stores the preferred parent to build a block on top of. It contains the
	// parent block as well as the QC POINTING to the parent, which can be used to build the block.
	// preferredParent.QC called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	preferredParent *forks.BlockQC
	finalizer       *finalizer.Finalizer
	notifier        notifications.Consumer
}

func NewNewestForkChoice(finalizer *finalizer.Finalizer, notifier notifications.Consumer) (*NewestForkChoice, error) {

	// build the initial block-QC pair
	// NOTE: I don't like this structure because it stores view and block ID in two separate places; this means
	// we don't have a single field that is the source of truth, and opens the door for bugs that would otherwise
	// be impossible
	block := finalizer.FinalizedBlock()
	qc := finalizer.FinalizedBlockQC()
	if block.BlockID != qc.BlockID || block.View != qc.View {
		return nil, fmt.Errorf("mismatch between finalized block and QC")
	}

	blockQC := forks.BlockQC{Block: block, QC: qc}

	fc := &NewestForkChoice{
		preferredParent: &blockQC,
		finalizer:       finalizer,
		notifier:        notifier,
	}

	notifier.OnQcIncorporated(qc)

	return fc, nil
}

// MakeForkChoice prompts the ForkChoice to generate a fork choice for the
// current view `curView`. NewestForkChoice always returns the qc with the largest
// view number seen.
// It returns a qc and the block that the qc is pointing to.
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
func (fc *NewestForkChoice) MakeForkChoice(curView uint64) (*model.QuorumCertificate, *model.Block, error) {
	choice := fc.preferredParent
	if choice.Block.View >= curView {
		// sanity check;
		// processing a QC with view v should result in the PaceMaker being in view v+1 or larger
		// Hence, given that the current View is curView, all QCs should have view < curView
		return nil, nil, fmt.Errorf(
			"ForkChoice selected qc with view %d which is larger than requested view %d",
			choice.Block.View, curView,
		)
	}
	fc.notifier.OnForkChoiceGenerated(curView, choice.QC)
	return choice.QC, choice.Block, nil
}

// updateQC updates `preferredParent` according to the fork-choice rule.
// Currently, we implement 'Chained HotStuff Protocol' where the fork-choice
// rule is: "build on newest QC"
// It assumes the QC has been validated
func (fc *NewestForkChoice) AddQC(qc *model.QuorumCertificate) error {
	if qc.View <= fc.preferredParent.Block.View {
		// Per construction, preferredParent.View() is always larger than the last finalized block's view.
		// Hence, this check suffices to drop all QCs with qc.View <= last_finalized_block.View().
		return nil
	}

	// Have qc.View > last_finalized_block.View(). Hence, block referenced by qc should be stored:
	block, err := fc.ensureBlockStored(qc)
	if err != nil {
		return fmt.Errorf("cannot add QC: %w", err)
	}

	if block.BlockID != qc.BlockID || block.View != qc.View {
		return fmt.Errorf("mismatch between finalized block and QC")
	}

	blockQC := forks.BlockQC{Block: block, QC: qc}
	fc.preferredParent = &blockQC
	fc.notifier.OnQcIncorporated(qc)

	return nil
}

func (fc *NewestForkChoice) ensureBlockStored(qc *model.QuorumCertificate) (*model.Block, error) {
	block, haveBlock := fc.finalizer.GetBlock(qc.BlockID)
	if !haveBlock {
		// This should never happen and indicates an implementation bug.
		// Finding the block to which the qc points to should always be possible for the folling reason:
		// * Check in function AddQC guarantees: qc.View > fc.preferredParent.Block.View
		// * Forks guarantees that every block's qc is also processed by ForkChoice
		//   => fc.preferredParent.Block.View > fc.finalizer.FinalizedBlock().View()
		//   (as NewestForkChoice tracks the qc with the largest view)
		// => qc.View > fc.finalizer.FinalizedBlock().View()
		//    any block whose view is larger than the latest finalized block should be stored in finalizer
		return nil, &model.ErrorMissingBlock{View: qc.View, BlockID: qc.BlockID}
	}
	if block.View != qc.View {
		return nil, fmt.Errorf("invalid qc with mismatching view")
	}
	return block, nil
}

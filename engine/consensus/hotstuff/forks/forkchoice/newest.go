package forkchoice

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// NewestForkChoice implements HotStuff's original fork choice rule:
// always use the newest QC (i.e. the QC with highest view number)
type NewestForkChoice struct {
	// preferredQc is called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	preferredQc *types.QuorumCertificate
	notifier    notifications.Distributor
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
func (fc *NewestForkChoice) MakeForkChoice(curView uint64) (*types.QuorumCertificate, error) {
	choice := fc.preferredQc
	if choice.View >= curView {
		// sanity check;
		// processing a QC with view v should result in the PaceMaker being in view v+1 or larger
		// Hence, given that the current View is curView, all QCs should have view < curView
		err := fmt.Errorf(
			"ForkChoice slected qc with view %d which is larger than requested view %d",
			choice.View, curView,
		)
		return nil, err
	}
	fc.notifier.OnForkChoiceGenerated(curView, choice)
	return choice, nil
}

// updateQC updates `preferredQc` according to the fork-choice rule.
// Currently, we implement 'Chained HotStuff Protocol' where the fork-choice
// rule is: "build on newest QC"
// Returns true if and only if preferredQc was updated
func (fc *NewestForkChoice) AddQC(qc *types.QuorumCertificate) error {
	if qc.View <= fc.preferredQc.View {
		return nil
	}
	fc.preferredQc = qc
	fc.notifier.OnQcIncorporated(qc)
	return nil
}

func NewNewestForkChoice(rootQC *types.QuorumCertificate, notifier notifications.Distributor) forks.ForkChoice {
	return &NewestForkChoice{
		preferredQc: rootQC,
		notifier:    notifier,
	}
}

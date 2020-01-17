package forkchoice

import (
	"fmt"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/forkchoice/events"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// NewestForkChoice implements HotStuff's original fork choice rule:
// always use the newest QC (i.e. the QC with highest view number)
type NewestForkChoice struct {
	finalizer *finalizer.ReactorCore
	qcCache   QcCache

	preferredQc *types.QuorumCertificate
	// This is called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	// preferredQc is the QC for the preferred block according to the fork-choice rule

	eventprocessor events.Processor
}

func (fc *NewestForkChoice) MakeForkChoice(view uint64) *types.QuorumCertificate {
	choice := fc.preferredQc
	if choice.View > view {
		message := fmt.Sprintf("ForkChoice slected qc with view %d which is larger than requested view %d", choice.View, view)
		ForkChoiceLogger.Warningf(message)
		panic(message)
	}
	fc.eventprocessor.OnForkChoiceGenerated(view, choice)
	return choice
}

func (fc *NewestForkChoice) ProcessBlock(block *types.BlockProposal) {
	fc.finalizer.ProcessBlock(block)
	fc.updateQC(block.QC())
}

func (fc *NewestForkChoice) IsProcessingNeeded(blockMRH []byte, blockView uint64) bool {
	return fc.finalizer.IsProcessingNeeded(blockMRH, blockView)
}

func (fc *NewestForkChoice) IsKnownBlock(blockMRH []byte, blockView uint64) bool {
	return fc.finalizer.IsKnownBlock(blockMRH, blockView)
}

// ProcessQc updates `preferredQc` according to the fork-choice rule.
func (fc *NewestForkChoice) ProcessQc(qc *types.QuorumCertificate) {
	if fc.updateQC(qc) {
		fc.eventprocessor.OnQcFromVotesIncorporated(qc)
	}
}

// updateQC updates `preferredQc` according to the fork-choice rule.
// Currently, the method implements 'Chained HotStuff Protocol' where the fork-choice
// rule is: "build on newest QC"
// Returns true if and only if preferredQc was updated
func (fc *NewestForkChoice) updateQC(qc *types.QuorumCertificate) bool {
	// r.preferredQc is called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	if qc.View <= fc.preferredQc.View {
		return false
	}
	// We are implementing relaxed HotStuff rule which uses the newest QC (see Event-driven HotStuff)
	fc.preferredQc = qc
	return true
}

func NewNewestForkChoice(finalizer *finalizer.ReactorCore, eventprocessor events.Processor) *NewestForkChoice {
	return &NewestForkChoice{
		finalizer:      finalizer,
		qcCache:        NewQcCache(),
		preferredQc:    finalizer.LastFinalizedBlockQC,
		eventprocessor: eventprocessor,
	}
}

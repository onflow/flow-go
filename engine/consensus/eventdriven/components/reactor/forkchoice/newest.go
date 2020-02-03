package forkchoice

import (
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/components/reactor/core"
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/components/reactor/forkchoice/events"
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/def"
)

// NewestForkChoice implements HotStuff's original fork choice rule:
// always use the newest QC (i.e. the QC with highest view number)
type NewestForkChoice struct {
	finalizer *core.ReactorCore
	qcCache   QcCache

	preferredQc *def.QuorumCertificate
	// This is called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	// preferredQc is the QC for the preferred block according to the fork-choice rule

	eventprocessor events.Processor
}

func (fc *NewestForkChoice) OnForkChoiceTrigger(view uint64) {
	choice := fc.preferredQc
	if choice.View <= view {
		fc.eventprocessor.OnForkChoiceGenerated(view, choice)

	} else {
		ForkChoiceLogger.Warningf("Dropped ForkChoiceTrigger for view %d as the result was already stale", view)
	}
}

func (fc *NewestForkChoice) ProcessBlock(block *def.Block) {
	fc.finalizer.ProcessBlock(block)
	fc.updateQC(block.QC)
}

func (fc *NewestForkChoice) IsProcessingNeeded(blockMRH []byte, blockView uint64) bool {
	return fc.finalizer.IsProcessingNeeded(blockMRH, blockView)
}

func (fc *NewestForkChoice) IsKnownBlock(blockMRH []byte, blockView uint64) bool {
	return fc.finalizer.IsKnownBlock(blockMRH, blockView)
}

// ProcessQcFromVotes updates `preferredQc` according to the fork-choice rule.
func (fc *NewestForkChoice) ProcessQcFromVotes(qc *def.QuorumCertificate) {
	if fc.updateQC(qc) {
		fc.eventprocessor.OnQcFromVotesIncorporated(qc)
	}
}

// updateQC updates `preferredQc` according to the fork-choice rule.
// Currently, the method implements 'Chained HotStuff Protocol' where the fork-choice
// rule is: "build on newest QC"
// Returns true if and only if preferredQc was updated
func (fc *NewestForkChoice) updateQC(qc *def.QuorumCertificate) bool {
	// r.preferredQc is called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	if qc.View <= fc.preferredQc.View {
		return false
	}
	// We are implementing relaxed HotStuff rule which uses the newest QC (see Event-driven HotStuff)
	fc.preferredQc = qc
	return true
}

func NewNewestForkChoice(finalizer *core.ReactorCore, eventprocessor events.Processor) *NewestForkChoice {
	return &NewestForkChoice{
		finalizer:      finalizer,
		qcCache:        NewQcCache(),
		preferredQc:    finalizer.LastFinalizedBlockQC,
		eventprocessor: eventprocessor,
	}
}

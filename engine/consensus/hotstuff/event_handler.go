package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type EventHandler struct {
	paceMaker             *PaceMaker
	voteAggregator        *VoteAggregator
	voter                 *Voter
	missingBlockRequester *MissingBlockRequester
	forkChoice            *ForkChoice
	unincorporatedBlocks  *UnincorporatedBlocks
	validator             *Validator
	blockProposalProducer BlockProposalProducer
	viewState             ViewState
	network               NetworkSender
}

func (eh *EventHandler) OnReceiveBlockProposal(block *types.BlockProposal) {
	eh.onReceiveBlockProposal(block)
}

func (eh *EventHandler) OnReceiveVote(vote *types.Vote) {
	eh.onReceiveVote(vote)
}

func (eh *EventHandler) OnLocalTimeout(timeout *types.Timeout) {
	eh.onLocalTimeout(timeout)
}

func (eh *EventHandler) OnBlockRequest(req *types.BlockProposalRequest) {
	eh.onBlockRequest(req)
}

func (eh *EventHandler) onNewViewEntered(newView *types.NewViewEvent) {
	view := newView.View
	proposals := eh.forkChoice.FindProposalsByView(view)

	if len(proposals) == 0 {
		if eh.viewState.IsSelfLeaderForView(view) {
			// no need to check qc from voteaggregator, because it has been checked
			qcForNewBlock := eh.forkChoice.GetQCForNextBlock(view)
			proposal := eh.blockProposalProducer.MakeBlockProposalWithQC(qcForNewBlock)
			eh.onReceiveBlockProposal(proposal)
			go eh.network.BroadcastProposal(proposal)
		} else {
			// keep waiting for the proposal
		}
	} else if len(proposals) == 1 {
		eh.onReceiveBlockProposal(proposals[0])
	} else {
		// double proposals
		// TODO: slash
	}
}

func (eh *EventHandler) onGenericQCUpdated(qc *types.QuorumCertificate) {
	eh.paceMaker.UpdateValidQC(qc)
}

func (eh *EventHandler) processNewQC(qc *types.QuorumCertificate) {
	// an invalid block might have valid QC, so update the QC first regardless the block is valid or not
	genericQCUpdated, finalizedBlocks := eh.forkChoice.UpdateValidQC(qc)

	if genericQCUpdated {
		eh.onGenericQCUpdated(qc)
	}

	eh.onBlocksFinalized(finalizedBlocks)

	newView, updated := eh.paceMaker.UpdateValidQC(qc)
	if updated {
		eh.onNewViewEntered(newView)
	}
}

// QC has been processed
func (eh *EventHandler) processIncorperatedBlock(block *types.BlockProposal) {
	newView, updated := eh.paceMaker.UpdateBlock(block)
	if updated {
		eh.onNewViewEntered(newView)
	}

	// check if there is pending votes to build a QC as a leader
	if eh.paceMaker.CurView() == block.Block.View && eh.viewState.IsSelfLeaderForView(block.Block.View) {
		newQC, built := eh.voteAggregator.BuildQCForBlockProposal(block)
		if built {
			eh.processNewQC(newQC)
		}
	}
}

func (eh *EventHandler) onBlocksFinalized(finalizedBlocks []*types.BlockProposal) {
	if len(finalizedBlocks) == 0 {
		return
	}

	// TODO: notify block is finalized

	// only the last finalized block is needed to prune by view
	lastFinalized := finalizedBlocks[len(finalizedBlocks)-1]
	eh.voteAggregator.PruneByView(lastFinalized.Block.View)
	eh.unincorporatedBlocks.PruneByView(lastFinalized.Block.View)
}

func (eh *EventHandler) onReceiveBlockProposal(blockProposal *types.BlockProposal) {
	if blockProposal.Block.View <= eh.forkChoice.FinalizedView() {
		return // ignore proposals below finalized view
	}

	if eh.forkChoice.CanIncorperate(blockProposal) == false {
		// the proposal is not incorperated (meaning, its QC doesn't exist in the block tree)
		eh.unincorporatedBlocks.Store(blockProposal)
		eh.missingBlockRequester.FetchMissingBlock(blockProposal.Block.View, blockProposal.Block.BlockMRH())
		return
	}

	// the proposal is incorperatable

	if !eh.validator.ValidateQC(blockProposal.Block.QC) {
		return
	}

	eh.processNewQC(blockProposal.Block.QC)

	if eh.validator.ValidateBlock(blockProposal.Block.QC, blockProposal) == false {
		return // ignore invalid block
	}

	incorperatedBlock, added := eh.forkChoice.AddNewProposal(blockProposal)
	if added {
		eh.processIncorperatedBlock(incorperatedBlock)
	}

	if eh.forkChoice.IsSafeNode(blockProposal) == false {
		return
	}

	myVote, voteCollector := eh.voter.ShouldVoteForNewProposal(blockProposal, eh.paceMaker.CurView())
	if myVote == nil {
		return // exit if i should not vote
	}

	if eh.viewState.IsSelf(myVote.View, voteCollector) {
		eh.onReceiveVote(myVote) // if I'm collecting my own vote, then pass it to myself directly
	} else {
		eh.network.SendVote(myVote, voteCollector)
	}

	// To handle: 10, 12, 11
	pendingProposal, extracted := eh.unincorporatedBlocks.ExtractByView(eh.paceMaker.CurView())
	if extracted {
		eh.OnReceiveBlockProposal(pendingProposal)
	}
}

func (eh *EventHandler) onReceiveVote(vote *types.Vote) {
	blockProposal, found := eh.forkChoice.FindProposalByViewAndBlockMRH(vote.View, vote.BlockMRH)
	if found == false {
		eh.voteAggregator.Store(vote)
		eh.missingBlockRequester.FetchMissingBlock(vote.View, vote.BlockMRH)
		return
	}

	newQC, built := eh.voteAggregator.StoreAndMakeQCForIncorporatedVote(vote, blockProposal)
	if built == false {
		// No QC was created. Various cases could lead to this:
		// - not enough valid votes
		// - a QC has been made before
		return
	}

	eh.processNewQC(newQC)
}

func (eh *EventHandler) onLocalTimeout(timeout *types.Timeout) {
	newView, updated := eh.paceMaker.OnLocalTimeout(timeout)
	if updated {
		eh.onNewViewEntered(newView)
	}
}

// Will be replaced by chain compliance layer
func (eh *EventHandler) onBlockRequest(req *types.BlockProposalRequest) {
	blockProposal, found := eh.forkChoice.FindProposalByViewAndBlockMRH(req.View, req.BlockMRH)
	if found {
		eh.network.RespondBlockProposalRequest(req, blockProposal)
	}
}

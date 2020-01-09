package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type EventHandler struct {
	paceMaker             *PaceMaker
	voteAggregator        *VoteAggregator
	voter                 *Voter
	missingBlockRequester *MissingBlockRequester
	forkChoice            *ForkChoice
	incorperatedBlocks    *IncorperatedBlocks
	validator             *Validator
	blockProducer         BlockProducer
	protocolState         ProtocolState
	network               Network
	// Flag to turn on/off consensus acts (voting, block production etc)
	isConActor bool
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
	proposal := eh.forkChoice.FindBlockForView(view)
	if proposal == nil && eh.protocolState.IsSelfLeaderForView(view) {
		// no need to check qc from voteaggregator, because it has been checked
		qcForNewBlock := eh.forkChoice.GetQCForNextBlock(view)
		proposal = eh.blockProducer.MakeBlockWithQC(qcForNewBlock)
		go eh.network.BroadcastProposal(proposal)
	}

	if proposal != nil {
		eh.onReceiveBlockProposal(proposal)
	}
}

func (eh *EventHandler) onGenericQCUpdated(qc *types.QuorumCertificate) {
	eh.paceMaker.UpdateValidQC(qc)
}

func (eh *EventHandler) processNewQC(qc *types.QuorumCertificate) {
	// a invalid block might have valid QC, so update the QC first regardless the block is valid or not
	genericQCUpdated, finalizedBlock := eh.forkChoice.UpdateValidQC(qc)

	if genericQCUpdated {
		eh.onGenericQCUpdated(qc)
	}

	if finalizedBlock != nil {
		eh.onBlockFinalized(finalizedBlock)
	}

	newView := eh.paceMaker.UpdateValidQC(qc)
	if newView != nil {
		eh.onNewViewEntered(newView)
	}
}

// QC has been processed
func (eh *EventHandler) processIncorperatedBlock(block *types.BlockProposal) {
	newView := eh.paceMaker.UpdateBlock(block)
	if newView != nil {
		eh.onNewViewEntered(newView)
	}

	// check if there is pending votes to build a QC as a leader
	if eh.paceMaker.CurView() == block.Block.View && eh.protocolState.IsSelfLeaderForView(block.Block.View) {
		newQC := eh.voteAggregator.BuildQCForBlockProposal(block)
		if newQC != nil {
			eh.processNewQC(newQC)
		}
	}
}

func (eh *EventHandler) onBlockFinalized(finalizedBlock *types.BlockProposal) {
	go eh.voteAggregator.PruneByView(finalizedBlock.Block.View)
	go eh.incorperatedBlocks.PruneByView(finalizedBlock.Block.View)
}

func (eh *EventHandler) onReceiveBlockProposal(blockProposal *types.BlockProposal) {
	if blockProposal.Block.View <= eh.forkChoice.FinalizedView() {
		return // ignore proposals below finalized view
	}

	if eh.forkChoice.CanIncorperate(blockProposal) == false {
		// the proposal is not incorperated (meaning, its QC doesn't exist in the block tree)
		eh.incorperatedBlocks.Store(blockProposal)
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

	incorperatedBlock := eh.forkChoice.AddNewBlock(blockProposal)
	if incorperatedBlock != nil {
		eh.processIncorperatedBlock(incorperatedBlock)
	}

	if eh.isConActor {
		if eh.forkChoice.IsSafeNode(blockProposal) == false || eh.paceMaker.CurView() != blockProposal.Block.View {
			return
		}

		myVote := eh.voter.ProduceVote(blockProposal)
		if myVote == nil {
			return // exit if i should not vote
		}

		if eh.protocolState.IsSelf(myVote.View, myVote.Signature.SignerIdx) {
			eh.onReceiveVote(myVote) // if I'm collecting my own vote, then pass it to myself directly
		} else {
			eh.network.SendVote(myVote, myVote.Signature.SignerIdx)
		}
	}

	// To handle: 10, 12, 11
	pendingProposal := eh.incorperatedBlocks.ExtractByView(eh.paceMaker.CurView())
	if pendingProposal != nil {
		eh.OnReceiveBlockProposal(pendingProposal)
	}
}

func (eh *EventHandler) onReceiveVote(vote *types.Vote) {
	blockProposal := eh.forkChoice.FindBlockProposalByViewAndBlockMRH(vote.View, vote.BlockMRH)
	if blockProposal == nil {
		eh.voteAggregator.Store(vote)
		eh.missingBlockRequester.FetchMissingBlock(vote.View, vote.BlockMRH)
		return
	}

	newQC := eh.voteAggregator.StoreAndMakeQCForIncorporatedVote(vote, blockProposal)
	if newQC == nil {
		return // if a QC has been made before or the vote was invalid
	}

	eh.processNewQC(newQC)
}

func (eh *EventHandler) onLocalTimeout(timeout *types.Timeout) {
	newView := eh.paceMaker.OnLocalTimeout(timeout)
	if newView != nil {
		eh.onNewViewEntered(newView)
	}
}

func (eh *EventHandler) onBlockRequest(req *types.BlockProposalRequest) {
	blockProposal := eh.forkChoice.FindBlockProposalByViewAndBlockMRH(req.View, req.BlockMRH)
	if blockProposal != nil {
		eh.network.RespondBlockProposalRequest(req, blockProposal)
	}
}

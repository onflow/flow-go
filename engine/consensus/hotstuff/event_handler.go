package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type EventHandler struct {
	paceMaker             *PaceMaker
	voteAggregator        *VoteAggregator
	voter                 *Voter
	missingBlockRequester *MissingBlockRequester
	reactor            Reactor
	validator             *Validator
	blockProducer         BlockProducer
	protocolState         ProtocolState
	network               Network
	blockFinalizer 		BlockFinalizer

	internalState stateValue
}

type stateValue int
const (
	WaitingForBlock stateValue = iota
	WaitingForVotes stateValue = iota
)

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

func (eh *EventHandler) startNewView() {
	eh.pruneInMemoryState()
	eh.internalState = WaitingForBlock

	eh.proposeIfLeaderForCurrentView()

	currentBlock := eh.selectBlockProposalForCurrentView()
	if currentBlock == nil {
		return
	}
	eh.processBlockForCurrentView(currentBlock) // CAUTION: this is a state transition;
	// no more processing should be done in this function
}

// proposeIfLeaderForCurrentView generates a block proposal if this replica is the primary for the current view
// (otherwise no-op). The proposal is broadcast it to the entire network and stored in `Reactor`.
// Call MODIFIES internal state of Reactor!
func (eh *EventHandler) proposeIfLeaderForCurrentView() {
	curView := eh.paceMaker.CurView()
	if ! eh.protocolState.IsSelfLeaderForView(curView) {
		return
	}

	// no need to check qc from voteaggregator, because it has been checked
	qcForNewBlock := eh.reactor.MakeForkChoice(curView)
	proposal := eh.blockProducer.MakeBlockWithQC(curView, qcForNewBlock)
	go eh.network.BroadcastProposal(proposal)
	eh.reactor.AddBlock(proposal)
}


// selectBlockProposalForCurrentView selects a single BlockProposal from reactor whose view matches the current view.
// Contracts:
//  * [expected] any known blocks that are attached to the main chain
//    (including a potential proposal for the current view) must be stored in reactor
//  * [guaranteed] internal state is unmodified.
// Note:
// For the view we are primary, it cannot happen that there are multiple proposals, for the following reason.
// This replica is the only one allowed to propose blocks for the respective view. Assuming that this replica is honest,
// only one valid block is proposed (proposals for the same view by other replicas are invalid, as they are not primary).
func (eh *EventHandler) selectBlockProposalForCurrentView() *types.BlockProposal {
	proposalsAtView := eh.reactor.BlocksForView(eh.paceMaker.CurView())
	if len(proposalsAtView) == 0 {
		return nil
	}
	return proposalsAtView[0]
}

func (eh *EventHandler) pruneInMemoryState() {
	finalizedView := eh.reactor.FinalizedView()
	eh.voteAggregator.PruneByView(finalizedView)
}

func (eh *EventHandler) processBlockForCurrentView(block *types.BlockProposal) {
	if block.View() != eh.paceMaker.CurView() { // sanity check
		panic("Internal Error: expecting block for current view")
	}
	nextLeader := eh.protocolState.LeaderForView(eh.paceMaker.CurView() + 1)

	// not leader for next view -> state transitions to EnteringView(curView+1)
	if !eh.protocolState.IsSelf(nextLeader) {
		vote := eh.considerVoting(block) // vote can be nil, if we don't want to vote for block
		eh.sendVote(vote, nextLeader)
		if newView := eh.paceMaker.UpdateBlock(block); newView == nil {
			panic("We are not primary for the next view and completed processing block for current view. But pacemaker did not increment view!")
		}
		eh.startNewView() // CAUTION: this is a state transition; no more processing should be done here
		return
	}

	// I am leader for next view
	eh.commenceWaitingForVotes(block) // CAUTION: this is a state transition;
	// no more processing should be done done here
}


func (eh *EventHandler) considerVoting(block *types.BlockProposal) *types.Vote {
	if eh.reactor.IsSafeNode(block) {
		return eh.voter.ConsiderVoting(block, eh.paceMaker.CurView())
	}
	return nil
}

func (eh *EventHandler) sendVote(vote *types.Vote, nextLeader types.ID) {
	if vote == nil {
		return
	}
	eh.network.SendVote(vote, nextLeader)
}

func (eh *EventHandler) commenceWaitingForVotes(block *types.BlockProposal) {
	eh.internalState = WaitingForVotes
	eh.paceMaker.UpdateBlock(block) // this should solely update the timeout but not the current View
	if block.View() != eh.paceMaker.CurView() { // sanity check
		panic("Internal Error: expecting pacemaker to not change view when commencing vote collection")
	}

	vote := eh.considerVoting(block) // vote can be nil, if we don't want to vote for block
	if vote == nil {
		return // CAUTION: this is a state transition;
		// no more processing should be done done here
	}


}

func (eh *EventHandler) onReceiveVote(vote *types.Vote) {
	if vote == nil {
		return
	}
	blockProposal := eh.reactor.FindBlockProposalByViewAndBlockMRH(vote.View, vote.BlockMRH)
	if blockProposal == nil {
		eh.voteAggregator.Store(vote)
		eh.missingBlockRequester.FetchMissingBlock(vote.View, vote.BlockMRH)
		return
	}

	newQC := eh.voteAggregator.StoreAndMakeQCForIncorporatedVote(vote, blockProposal)
	if newQC == nil {
		return // if a QC has been made before or we don't have enough votes
	}
	eh.processNewQC(newQC)
}



func (eh *EventHandler) processNewQC(qc *types.QuorumCertificate) {
	eh.reactor.ProcessQcFromVotes(qc)
	if newView := eh.paceMaker.UpdateQC(qc); newView != nil {
		eh.startNewView()
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
}



func (eh *EventHandler) onReceiveBlockProposal(receivedProposal *types.BlockProposal) {
	if receivedProposal.Block.View <= eh.reactor.FinalizedView() {
		return // ignore proposals below finalized view
	}
	if !eh.validator.ValidateQC(receivedProposal.QC()) {
		// ToDo Slash
		return
	}

	if eh.paceMaker.UpdateQC(receivedProposal.QC()) != nil {
		eh.onEnteringNewView()
	}


	eh.reactor.AddBlock(receivedProposal)
	currentViewProposal := eh.blockProposalForCurrentView(receivedProposal)
	if currentViewProposal == nil {
		return
	}

	// check if there is pending votes to build a QC as a leader
	if eh.paceMaker.CurView() == block.Block.View && eh.protocolState.IsSelfLeaderForView(block.Block.View) {
		newQC := eh.voteAggregator.BuildQCForBlockProposal(block)
		if newQC != nil {
			eh.processNewQC(newQC)
		}
	}



	incorperatedBlock := eh.reactor.AddNewBlock(receivedProposal)
	if incorperatedBlock != nil {
		eh.processIncorperatedBlock(incorperatedBlock)
	}

	if eh.reactor.IsSafeNode(receivedProposal) == false {
		return
	}

	myVote, voteCollector := eh.voter.ShouldVoteForNewProposal(receivedProposal, eh.paceMaker.CurView())
	if myVote == nil {
		return // exit if i should not vote
	}

	if eh.protocolState.IsSelf(myVote.View, voteCollector) {
		eh.onReceiveVote(myVote) // if I'm collecting my own vote, then pass it to myself directly
	} else {
		eh.network.SendVote(myVote, voteCollector)
	}

	// To handle: 10, 12, 11
	pendingProposal := eh.incorperatedBlocks.ExtractByView(eh.paceMaker.CurView())
	if pendingProposal != nil {
		eh.OnReceiveBlockProposal(pendingProposal)
	}
}



func (eh *EventHandler) onLocalTimeout(timeout *types.Timeout) {
	newView := eh.paceMaker.OnLocalTimeout(timeout)
	if newView != nil {
		eh.onNewViewEntered(newView)
	}
}

func (eh *EventHandler) onBlockRequest(req *types.BlockProposalRequest) {
	blockProposal := eh.reactor.FindBlockProposalByViewAndBlockMRH(req.View, req.BlockMRH)
	if blockProposal != nil {
		eh.network.RespondBlockProposalRequest(req, blockProposal)
	}
}

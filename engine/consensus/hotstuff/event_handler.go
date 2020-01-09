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
	eh.processVote(vote)
}

func (eh *EventHandler) OnLocalTimeout(timeout *types.Timeout) {
	eh.onLocalTimeout(timeout)
}

func (eh *EventHandler) OnBlockRequest(req *types.BlockProposalRequest) {
	eh.onBlockRequest(req)
}

func (eh *EventHandler) startNewView() {
	eh.pruneInMemoryState()
	eh.proposeIfLeaderForCurrentView()

	eh.internalState = WaitingForBlock
	currentBlock := eh.selectBlockProposalForCurrentView()
	if currentBlock == nil {
		return // Remain in state "WaitingForBlock"
	}
	eh.processBlockForCurrentView(currentBlock) // CAUTION: this is a state transition;
	// no more processing should be done in this function
}

func (eh *EventHandler) pruneInMemoryState() {
	finalizedView := eh.reactor.FinalizedView()
	eh.voteAggregator.PruneByView(finalizedView)
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

// processBlockForCurrentView enters state "Process Block for Current View"
func (eh *EventHandler) processBlockForCurrentView(block *types.BlockProposal) {
	if block.View() != eh.paceMaker.CurView() { // sanity check
		panic("Internal Error: expecting block for current view")
	}
	nextLeader := eh.protocolState.LeaderForView(eh.paceMaker.CurView() + 1)

	// not leader for next view -> state transition to EnteringView(curView+1)
	if !eh.protocolState.IsSelf(nextLeader) { // ToDo this should be its own function
		vote := eh.considerVoting(block) // vote can be nil, if we don't want to vote for block
		eh.sendVote(vote, nextLeader) // gracefully handles nil votes

		// informing PaceMaker about block for current view should transition to new view
		if eh.paceMaker.UpdateBlock(block) == nil { // sanity check that state transition took place
			panic("We are not primary for the next view and completed processing block for current view. But pacemaker did not increment view!")
		}
		eh.startNewView() // CAUTION: this is a state transition; no more processing should be done here
		return
	}

	// I am leader for next view -> state transition to WaitingForVotes
	eh.commenceWaitingForVotes(block) // CAUTION: this is a state transition;
	// no more processing should be done here
}


func (eh *EventHandler) considerVoting(block *types.BlockProposal) *types.Vote {
	if eh.reactor.IsSafeNode(block) {
		return eh.voter.ConsiderVoting(block, eh.paceMaker.CurView())
	}
	return nil
}

// sendVote sends vote to nextLeader. Gracefully handles (ignores) nil votes. 
func (eh *EventHandler) sendVote(vote *types.Vote, nextLeader types.ID) {
	if vote == nil {
		return
	}
	eh.network.SendVote(vote, nextLeader)
}

// commenceWaitingForVotes enters state "WaitingForVotes". We provide the block from the current view as input
// so we can incorporate our own vote for the block we just processed.
func (eh *EventHandler) commenceWaitingForVotes(block *types.BlockProposal) {
	eh.internalState = WaitingForVotes
	newView := eh.paceMaker.UpdateBlock(block) // this should solely update the timeout but not the current View
	if (newView != nil) || (block.View() != eh.paceMaker.CurView()) { // sanity check
		panic("Internal Error: expecting pacemaker to not change view when commencing vote collection")
	}

	vote := eh.considerVoting(block) // vote can be nil, if we don't want to vote for block
	if vote == nil {
		return // Remain in state "WaitingForVotes" 
	}
	eh.processVote(vote) // CAUTION: this is a state transition;
	// no more processing should be done here
}

// onReceiveVote enters state "ProcessingVote". Gracefully handles (ignores) nil votes. 
func (eh *EventHandler) processVote(vote *types.Vote) {
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
	eh.processQC(newQC) // CAUTION: this might result in a state transition;
	// no more processing should be done here
}

func (eh *EventHandler) processQC(qc *types.QuorumCertificate) {
	eh.reactor.ProcessQcFromVotes(qc)
	if eh.paceMaker.UpdateQC(qc) == nil {
		return
	}
	eh.startNewView() // CAUTION: this is a state transition;
	// no more processing should be done here
}


func (eh *EventHandler) onReceiveBlockProposal(receivedProposal *types.BlockProposal) {
	if receivedProposal.Block.View <= eh.reactor.FinalizedView() {
		return // ignore proposals below finalized view
	}

	// do the best we can with invalid block
	if !eh.validator.ValidateBlock(receivedProposal) {
		// ToDo Slash
		if !eh.validator.ValidateQC(receivedProposal.QC()) {
			if eh.paceMaker.UpdateQC(receivedProposal.QC()) != nil {
				eh.startNewView()
				return // CAUTION: this might result in a state transition;
				// no more processing should be done here
			}
		}
		return
	}

	eh.reactor.AddBlock(receivedProposal)
	if (receivedProposal.View() != eh.paceMaker.CurView()) || (eh.internalState != WaitingForBlock) {
		return
	}
	eh.processBlockForCurrentView(receivedProposal) // CAUTION: this might result in a state transition;
	// no more processing should be done here
}

func (eh *EventHandler) onLocalTimeout(timeout *types.Timeout) {
	if eh.paceMaker.OnLocalTimeout(timeout) == nil { // sanity check
		panic("Internal Error: expecting pacemaker to not change view on timeout")
	}
	eh.startNewView() // CAUTION: this is a state transition;
	// no more processing should be done here
}

func (eh *EventHandler) onBlockRequest(req *types.BlockProposalRequest) {
	blockProposal := eh.reactor.FindBlockProposalByViewAndBlockMRH(req.View, req.BlockMRH)
	if blockProposal != nil {
		eh.network.RespondBlockProposalRequest(req, blockProposal)
	}
}

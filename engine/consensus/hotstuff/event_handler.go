package hotstuff

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// EventHandler is the main handler for individual events that trigger state transition.
// It exposes API to handle one event at a time synchronously. The caller is
// responsible for running the event loop to ensure that.
type EventHandler struct {
	log zerolog.Logger

	paceMaker      PaceMaker
	voteAggregator *VoteAggregator
	voter          *Voter
	forks          Forks
	validator      *Validator
	blockProducer  *BlockProducer
	viewState      *ViewState
	network        Communicator
}

// NewEventHandler creates an EventHandler instance with initial components.
func NewEventHandler(
	log zerolog.Logger,
	paceMaker PaceMaker,
	voteAggregator *VoteAggregator,
	voter *Voter,
	forks Forks,
	validator *Validator,
	blockProducer *BlockProducer,
	viewState *ViewState,
	network Communicator,
) (*EventHandler, error) {
	e := &EventHandler{
		log:            log.With().Str("hotstuff", "event_handler").Logger(),
		paceMaker:      paceMaker,
		voteAggregator: voteAggregator,
		voter:          voter,
		forks:          forks,
		validator:      validator,
		blockProducer:  blockProducer,
		viewState:      viewState,
		network:        network,
	}
	return e, nil
}

// OnReceiveVote processes the vote when a vote is received.
// It is assumed that the voting block is not a missing block
func (e *EventHandler) OnReceiveVote(vote *hotstuff.Vote) error {

	e.log.Info().
		Hex("vote_block", logging.ID(vote.BlockID)).
		Uint64("vote_view", vote.View).
		Msg("vote received")

	// votes for finalized view or older should be dropped:
	if vote.View <= e.forks.FinalizedView() {
		// TODO: this should at least log something
		return nil
	}

	return e.processVote(vote)
}

// OnReceiveProposal processes the block when a block proposal is received.
// It is assumed that the block proposal is incorporated. (its parent can be found
// in the forks)
func (e *EventHandler) OnReceiveProposal(proposal *hotstuff.Proposal) error {

	e.log.Info().
		Hex("block", logging.ID(proposal.Block.BlockID)).
		Uint64("qc_view", proposal.Block.QC.View).
		Uint64("block_view", proposal.Block.View).
		Msg("proposal received")

	// validate the block. exit if the proposal is invalid
	err := e.validator.ValidateProposal(proposal)
	if errors.Is(err, hotstuff.ErrorInvalidBlock{}) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("cannot validate proposal: %w", err)
	}

	// store the block. the block will also be validated there
	err = e.forks.AddBlock(proposal.Block)
	if err != nil {
		return fmt.Errorf("cannot store block: %w", err)
	}

	// store the proposer's vote in voteAggregator
	_ = e.voteAggregator.StoreProposerVote(proposal.ProposerVote())

	// if the block is for the current view, then process the current block
	curView := e.paceMaker.CurView()
	if proposal.Block.View == curView {
		return e.processBlockForCurrentView(proposal.Block)
	}

	// if the block is not for the current view, try to build QC from votes for this block
	qc, built, err := e.voteAggregator.BuildQCOnReceivedBlock(proposal.Block)
	if err != nil {
		return fmt.Errorf("building qc for block failed: %w", err)
	}
	if !built {
		// if we don't have enough votes to build QC for this block, proceed with block.qc instead
		qc = proposal.Block.QC
	}
	// process the QC
	return e.processQC(qc)
}

// TimeoutChannel returns the channel for subscribing the waiting timeout on receiving
// block or votes for the current view.
func (e *EventHandler) TimeoutChannel() <-chan time.Time {
	return e.paceMaker.TimeoutChannel()
}

// OnLocalTimeout is called when the timeout event created by pacemaker looped through the
// event loop.
func (e *EventHandler) OnLocalTimeout() error {

	e.log.Info().Msg("local timeout triggered")

	curView := e.paceMaker.CurView()
	newview := e.paceMaker.OnTimeout()
	if curView == newview.View {
		return fmt.Errorf("OnLocalTimeout should gurantee that the pacemaker should go to next view, but didn't: (curView: %v, newview: %v)", curView, newview.View)
	}

	// current view has changed, go to new view
	return e.startNewView()
}

// Start will start the pacemaker's timer and start the new view
func (e *EventHandler) Start() error {
	e.paceMaker.Start()
	return e.startNewView()
}

// startNewView will only be called when there is a view change from pacemaker.
// It reads the current view, and check if it needs to proposal or vote for block.
func (e *EventHandler) startNewView() error {
	curView := e.paceMaker.CurView()
	e.log.Info().
		Uint64("curView", curView).
		Msg("start new view")
	e.pruneSubcomponents()

	if e.viewState.IsSelfLeaderForView(curView) {

		// as the leader of the current view,
		// build the block proposal for the current view
		_, qc, err := e.forks.MakeForkChoice(curView)
		if err != nil {
			return fmt.Errorf("can not make for choice for view %v: %w", curView, err)
		}

		proposal, err := e.blockProducer.MakeBlockProposal(qc, curView)
		if err != nil {
			return fmt.Errorf("can not make block proposal for curView %v: %w", curView, err)
		}

		// broadcast the proposal
		header := hotstuff.ProposalToFlow(proposal)
		err = e.network.BroadcastProposal(header)
		if err != nil {
			// when the network failed sending the proposal due to network issues, it will NOT
			// return error.
			// so if there is an error returned here, it must be some exception. It could be, i.e:
			// 1) the network layer double checks the proposal, and the check failed.
			// 2) the network layer had some exception encoding the proposal
			/// ...
			e.log.Warn().
				Hex("proposal_id", logging.ID(proposal.Block.BlockID)).
				Msg("failed to broadcast a block")
			return fmt.Errorf("can not broadcast the new proposal (%x) for view (%v): %w", proposal.Block.BlockID, proposal.Block.View, err)
		}

		// store the proposer's vote in voteAggregator
		// note: duplicate here to account for an edge case
		// where we are the leader of current view as well
		// as the next view
		_ = e.voteAggregator.StoreProposerVote(proposal.ProposerVote())

		err = e.forks.AddBlock(proposal.Block)
		if err != nil {
			return fmt.Errorf("cannot store block for curProposal: %w", err)
		}

		return e.processBlockForCurrentView(proposal.Block)
	}

	// as a replica of the current view,
	// find and process the block for the current view
	blocks := e.forks.GetBlocksForView(curView)
	if len(blocks) == 0 {
		// if there is no block stored before for the current view, then exit and keep
		// waiting
		return nil
	}

	// when there are multiple block proposals, we will just pick the first one.
	// forks is responsible for slashing double proposal behavior, and
	// event handler is aware of double proposals, but picking any should work and
	// won't hurt safeness
	block := blocks[0]

	return e.processBlockForCurrentView(block)
}

// pruneSubcomponents prunes EventHandler's sub-components
// Currently, the implementation follows the simplest design to prune once when we enter a
// new view instead of immediately when a block is finalized.
//
// Technically, EventHandler (and all other components) could consume Forks' OnBlockFinalized
// notifications and prune immediately. However, we have followed the design paradigm that all
// events are only for HotStuff-External components. The interaction of the HotStuff-internal
// components is directly handled by the EventHandler.
func (e *EventHandler) pruneSubcomponents() {
	e.voteAggregator.PruneByView(e.forks.FinalizedView())
}

// processBlockForCurrentView processes the block for the current view.
// It is called AFTER the block has been stored or found in Forks
// It checks whether to vote for this block.
// It might trigger a view change to go to a different view, which might re-enter this function.
func (e *EventHandler) processBlockForCurrentView(block *hotstuff.Block) error {

	// this is a sanity check to see if the block is really for the current view.
	curView := e.paceMaker.CurView()
	if block.View != curView {
		return fmt.Errorf("sanity check fails: block proposal's view does not match with curView, (blockView: %v, curView: %v)",
			block.View, curView)
	}

	// checking if I'm the next leader
	nextLeader := e.viewState.LeaderForView(curView + 1)
	isNextLeader := e.viewState.IsSelf(nextLeader.ID())

	if isNextLeader {
		return e.processBlockForCurrentViewIfIsNextLeader(block)
	}

	// if I'm not the next leader
	return e.processBlockForCurrentViewIfIsNotNextLeader(block, nextLeader)
}

func (e *EventHandler) processBlockForCurrentViewIfIsNextLeader(block *hotstuff.Block) error {

	// voter performs all the checks to decide whether to vote for this block or not.
	// note this call has to make before calling pacemaker, because calling to pace maker first might
	// cause the `curView` here to be stale
	isNextLeader := true
	curView := block.View
	ownVote, shouldVote := e.voter.ProduceVoteIfVotable(block, curView)

	// if i'm the next leader, then stay at the current view to collect votes for the block of the current view.
	nv, viewChanged := e.paceMaker.UpdateCurViewWithBlock(block, isNextLeader)
	if viewChanged {
		// this is a sanity check.
		// when I've processed the block for the current view and I'm the next leader,
		// the pacemaker should not trigger a view change.
		// log fatal error
		return fmt.Errorf("pacemaker should not trigger a view change, but it changed to view. (newview: %v)", nv.View)
	}

	if !shouldVote {
		// even if we are not voting for the block, we will still check if a QC can be built for this block.
		return e.tryBuildQCForBlock(block)
	}

	// since I'm the next leader, instead of sending the vote through the network and receiving it
	// back, it can be processed locally right away.
	return e.processVote(ownVote)
}

func (e *EventHandler) processBlockForCurrentViewIfIsNotNextLeader(block *hotstuff.Block, nextLeader *flow.Identity) error {

	// voter performs all the checks to decide whether to vote for this block or not.
	isNextLeader := false
	curView := block.View
	ownVote, shouldVote := e.voter.ProduceVoteIfVotable(block, curView)

	// send my vote if I should vote and I'm not the leader
	if shouldVote {
		err := e.network.SendVote(ownVote.BlockID, ownVote.View, ownVote.Signature.Raw, nextLeader.NodeID)
		if err != nil {
			// TODO: should we error here? E.g.
			//    return fmt.Errorf("failed to send vote: %w", err)
			//    We probably want to continue in that case ...
			e.log.Warn().Msg(fmt.Sprintf("failed to send vote: %s", err))
		}
	}

	// inform pacemaker that we've done the work for the current view, it should increment the current view
	_, viewChanged := e.paceMaker.UpdateCurViewWithBlock(block, isNextLeader)
	if !viewChanged {
		// this is a sanity check
		// when I've processed the block for the current view and I'm not the next leader,
		// the pacemaker should trigger a view change
		nextView := curView + 1
		return fmt.Errorf("pacemaker should trigger a view change to (view %v), but didn't", nextView)
	}

	// current view has changed, go to new view
	return e.startNewView()
}

// tryBuildQCForBlock checks whether there are enough votes to build a QC for the given block,
// and process the QC if a QC was built.
func (e *EventHandler) tryBuildQCForBlock(block *hotstuff.Block) error {
	qc, built, err := e.voteAggregator.BuildQCOnReceivedBlock(block)
	if err != nil {
		return fmt.Errorf("building qc for block failed: %w", err)
	}
	if !built {
		return nil
	}
	return e.processQC(qc)
}

// processVote stores the vote and check whether a QC can be built.
// If a QC is built, then process the QC.
// It assumes the voting block can be found in forks
func (e *EventHandler) processVote(vote *hotstuff.Vote) error {
	// read the voting block
	block, found := e.forks.GetBlock(vote.BlockID)
	if !found {
		// store the pending vote if voting block is not found.
		// We don't need to proactively fetch the missing voting block, because the chain compliance layer has acknowledged
		// the missing block and requested it already.
		_ = e.voteAggregator.StorePendingVote(vote)

		e.log.Info().
			Uint64("vote_view", vote.View).
			Hex("voting_block", logging.ID(vote.BlockID)).
			Msg("block for vote not found")
	}

	// if the voting block can be found, we should be able to validate the vote
	// and check if we can build a QC with it.
	qc, built, err := e.voteAggregator.StoreVoteAndBuildQC(vote, block)
	if err != nil {
		return fmt.Errorf("building qc for block failed: %w", err)
	}
	// if we don't have enough votes to build QC for this block:
	// nothing more to do for processing vote
	if !built {
		return nil
	}

	return e.processQC(qc)
}

// processQC stores the QC and check whether the QC will trigger view change.
// If triggered, then go to the new view.
func (e *EventHandler) processQC(qc *hotstuff.QuorumCertificate) error {
	err := e.forks.AddQC(qc)

	if err != nil {
		return fmt.Errorf("cannot add QC to forks: %w", err)
	}

	_, viewChanged := e.paceMaker.UpdateCurViewWithQC(qc)
	if !viewChanged {
		// this QC didn't trigger any view change, the processing ends.
		return nil
	}

	// current view has changed, go to new view
	return e.startNewView()
}

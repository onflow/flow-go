package hotstuff

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
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
	viewState      ViewState
	network        NetworkSender
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
	viewState ViewState,
	network NetworkSender,
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
func (e *EventHandler) OnReceiveVote(vote *types.Vote) error {

	e.log.Info().
		Hex("vote_block", logging.ID(vote.BlockID)).
		Uint64("vote_view", vote.View).
		Msg("vote received")

	return e.processVote(vote)
}

// OnReceiveBlockHeader processes the block when a block proposal is received.
// It is assumed that the block proposal is incorporated. (its parent can be found
// in the forks)
func (e *EventHandler) OnReceiveBlockHeader(block *types.BlockHeader) error {

	e.log.Info().
		Hex("block", logging.ID(block.BlockID())).
		Uint64("qc_view", block.QC().View).
		Uint64("block_view", block.View()).
		Msg("block proposal received")

	// find the parent of the block
	parent, found := e.forks.GetBlock(block.QC().BlockID)
	if !found {
		return fmt.Errorf("cannot find block's parent when receiving block header: %v", block.QC().BlockID)
	}

	// validate the block
	_, validBlock, err := e.validator.ValidateBlock(block, parent)
	if err != nil {
		return fmt.Errorf("invalid block header: %w", err)
	}

	// store the block. the block will also be validated there
	err = e.forks.AddBlock(validBlock)
	if err != nil {
		return fmt.Errorf("cannot store block: %w", err)
	}

	curView := e.paceMaker.CurView()

	// if the block is for the current view, then process the current block
	if block.View() == curView {
		return e.processBlockForCurrentView(validBlock)
	}

	// if the block is not for the current view, try to build QC from votes for this block
	qc, built := e.voteAggregator.BuildQCForBlockProposal(validBlock)
	if !built {
		// if cannot build QC for this block, process with block.qc instead
		qc = validBlock.QC()
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

	if e.viewState.IsSelfLeaderForView(curView) {
		// as the leader of the current view,
		// build the block proposal for the current view

		qcForCurProposal, err := e.forks.MakeForkChoice(curView)
		if err != nil {
			return fmt.Errorf("can not make for choice for view %v: %w", curView, err)
		}

		curProposal, err := e.blockProducer.MakeBlockProposal(curView, qcForCurProposal)
		if err != nil {
			return fmt.Errorf("can not make block proposal for curView %v: %w", curView, err)
		}

		err = e.forks.AddBlock(curProposal)
		if err != nil {
			return fmt.Errorf("cannot store block for curProposal: %w", err)
		}

		return e.processBlockForCurrentView(curProposal)
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

// processBlockForCurrentView processes the block for the current view.
// It is called AFTER the block has been stored or found in Forks
// It checks whether to vote for this block.
// It might trigger a view change to go to a different view, which might re-enter this function.
func (e *EventHandler) processBlockForCurrentView(block *types.BlockProposal) error {
	curView := e.paceMaker.CurView()
	// this is a sanity check to see if the block is really for the current view.
	if block.View() != curView {
		return fmt.Errorf("sanity check fails: block proposal's view does not match with curView, (blockView: %v, curView: %v)",
			block.View(), curView)
	}
	// checking if I'm the next leader
	nextLeader := e.viewState.LeaderForView(curView + 1)
	isNextLeader := e.viewState.IsSelf(nextLeader)

	if isNextLeader {
		return e.processBlockForCurrentViewIfIsNextLeader(block)
	}
	// if I'm not the next leader
	return e.processBlockForCurrentViewIfIsNotNextLeader(block, nextLeader)
}

func (e *EventHandler) processBlockForCurrentViewIfIsNextLeader(block *types.BlockProposal) error {
	isNextLeader := true
	curView := block.View()

	// voter performs all the checks to decide whether to vote for this block or not.
	// note this call has to make before calling pacemaker, because calling to pace maker first might
	// cause the `curView` here to be stale
	ownVote, shouldVote := e.voter.ProduceVoteIfVotable(block, curView)

	nv, viewChanged := e.paceMaker.UpdateCurViewWithBlock(block, isNextLeader)
	// if i'm the next leader, then stay at the current view to collect votes for the block of the current view.
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

func (e *EventHandler) processBlockForCurrentViewIfIsNotNextLeader(block *types.BlockProposal, nextLeader *flow.Identity) error {
	isNextLeader := false
	curView := block.View()

	// voter performs all the checks to decide whether to vote for this block or not.
	ownVote, shouldVote := e.voter.ProduceVoteIfVotable(block, curView)

	// send my vote if I should vote and I'm not the leader
	if shouldVote {
		e.network.SendVote(ownVote, nextLeader)
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
func (e *EventHandler) tryBuildQCForBlock(block *types.BlockProposal) error {
	qc, built := e.voteAggregator.BuildQCForBlockProposal(block)
	if !built {
		return nil
	}
	return e.processQC(qc)
}

// processVote stores the vote and check whether a QC can be built.
// If a QC is built, then process the QC.
// It assumes the voting block can be found in forks
func (e *EventHandler) processVote(vote *types.Vote) error {
	// read the voting block
	block, found := e.forks.GetBlock(vote.BlockID)
	if !found {
		// store the pending vote if voting block is not found.
		// We don't need to proactively fetch the missing voting block, because the chain compliance layer has acknowledged
		// the missing block and requested it already.
		e.voteAggregator.StorePendingVote(vote)

		e.log.Info().
			Uint64("vote_view", vote.View).
			Hex("voting_block", logging.ID(vote.BlockID)).
			Msg("block for vote not found")

		return nil
	}

	// if the voting block can be found, we should be able to validate the vote
	// and check if we can build a QC with it.
	qc, built := e.voteAggregator.StoreVoteAndBuildQC(vote, block)
	if !built {
		return nil
	}

	return e.processQC(qc)
}

// processQC stores the QC and check whether the QC will trigger view change.
// If triggered, then go to the new view.
func (e *EventHandler) processQC(qc *types.QuorumCertificate) error {
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

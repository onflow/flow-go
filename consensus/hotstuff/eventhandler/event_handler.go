//nolint
package eventhandler

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// EventHandler is the main handler for individual events that trigger state transition.
// It exposes API to handle one event at a time synchronously. The caller is
// responsible for running the event loop to ensure that.
type EventHandler struct {
	log            zerolog.Logger
	paceMaker      hotstuff.PaceMaker
	blockProducer  hotstuff.BlockProducer
	forks          hotstuff.Forks
	persist        hotstuff.Persister
	communicator   hotstuff.Communicator
	committee      hotstuff.Committee
	voteAggregator hotstuff.VoteAggregator
	voter          hotstuff.Voter
	validator      hotstuff.Validator
	notifier       hotstuff.Consumer
	ownProposal    flow.Identifier
}

// New creates an EventHandler instance with initial components.
func New(
	log zerolog.Logger,
	paceMaker hotstuff.PaceMaker,
	blockProducer hotstuff.BlockProducer,
	forks hotstuff.Forks,
	persist hotstuff.Persister,
	communicator hotstuff.Communicator,
	committee hotstuff.Committee,
	voteAggregator hotstuff.VoteAggregator,
	voter hotstuff.Voter,
	validator hotstuff.Validator,
	notifier hotstuff.Consumer,
) (*EventHandler, error) {
	e := &EventHandler{
		log:            log.With().Str("hotstuff", "participant").Logger(),
		paceMaker:      paceMaker,
		blockProducer:  blockProducer,
		forks:          forks,
		persist:        persist,
		communicator:   communicator,
		voteAggregator: voteAggregator,
		voter:          voter,
		validator:      validator,
		committee:      committee,
		notifier:       notifier,
		ownProposal:    flow.ZeroID,
	}
	return e, nil
}

// OnReceiveVote processes the vote when a vote is received.
// It is assumed that the voting block is not a missing block
func (e *EventHandler) OnReceiveVote(vote *model.Vote) error {
	curView := e.paceMaker.CurView()
	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("signer", vote.SignerID[:]).
		Logger()

	e.notifier.OnReceiveVote(curView, vote)
	defer e.notifier.OnEventProcessed()
	log.Debug().Msg("vote forwarded from compliance engine")

	// votes for finalized view or older should be dropped:
	if vote.View <= e.forks.FinalizedView() {
		log.Debug().Msg("skipping vote view equal or below finalized view")
		return nil
	}

	err := e.processVote(vote)
	if err != nil {
		return fmt.Errorf("failed processing vote: %w", err)
	}
	log.Debug().Msg("block vote processed")

	return nil
}

// OnReceiveProposal processes the block when a block proposal is received.
// It is assumed that the block proposal is incorporated. (its parent can be found
// in the forks)
func (e *EventHandler) OnReceiveProposal(proposal *model.Proposal) error {
	panic("to be deleted")

	block := proposal.Block
	curView := e.paceMaker.CurView()

	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("qc_view", block.QC.View).
		Hex("proposer_id", block.ProposerID[:]).
		Logger()

	e.notifier.OnReceiveProposal(curView, proposal)
	defer e.notifier.OnEventProcessed()
	log.Debug().Msg("proposal forwarded from compliance engine")

	// ignore stale proposals
	if block.View < e.forks.FinalizedView() {
		log.Debug().Msg("stale proposal")
		return nil
	}

	// we skip validation for our last own proposal
	if proposal.Block.BlockID != e.ownProposal {

		// validate the block. exit if the proposal is invalid
		err := e.validator.ValidateProposal(proposal)
		if model.IsInvalidBlockError(err) {
			log.Warn().Err(err).Msg("invalid block proposal")
			return nil
		}
		if errors.Is(err, model.ErrUnverifiableBlock) {
			log.Warn().Err(err).Msg("unverifiable block proposal")

			// even if the block is unverifiable because the QC has been
			// pruned, it still needs to be added to the forks, otherwise,
			// a new block with a QC to this block will fail to be added
			// to forks and crash the event loop.
		} else if err != nil {
			return fmt.Errorf("cannot validate proposal (%x): %w", block.BlockID, err)
		}
	}

	// store the block. the block will also be validated there
	err := e.forks.AddBlock(block)
	if err != nil {
		return fmt.Errorf("cannot add block to fork (%x): %w", block.BlockID, err)
	}

	//// store the proposer's vote in voteAggregator
	//stored := e.voteAggregator.StoreProposerVote(proposal.ProposerVote())

	// if the block is for the current view, then process the current block
	if block.View == curView {
		err = e.processBlockForCurrentView(block)
		if err != nil {
			return fmt.Errorf("failed processing current block: %w", err)
		}

		newView := e.paceMaker.CurView() // in case view has changed

		log.Debug().Uint64("new_view", newView).Msg("block proposal for current view processed")

		return nil
	}

	err = e.tryBuildQCForBlock(block)
	if err != nil {
		return fmt.Errorf("failed processing qc from block: %w", err)
	}

	//newView := e.paceMaker.CurView() // in case we skipped ahead
	//log.Debug().Uint64("new_view", newView).Bool("proposer_vote_stored", stored).Msg("block proposal for non-current view processed")

	return nil
}

// TimeoutChannel returns the channel for subscribing the waiting timeout on receiving
// block or votes for the current view.
func (e *EventHandler) TimeoutChannel() <-chan time.Time {
	return e.paceMaker.TimeoutChannel()
}

// OnLocalTimeout is called when the timeout event created by pacemaker looped through the
// event loop.
func (e *EventHandler) OnLocalTimeout() error {

	curView := e.paceMaker.CurView()
	newView := e.paceMaker.OnTimeout()
	defer e.notifier.OnEventProcessed()

	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("new_view", newView.View).
		Logger()
	// 	notifications about time-outs and view-changes are generated by PaceMaker; no need to send a notification here
	log.Debug().Msg("timeout received from event loop")

	if curView == newView.View {
		return fmt.Errorf("OnLocalTimeout should guarantee that the pacemaker should go to next view, but didn't: (curView: %v, newView: %v)", curView, newView.View)
	}

	// current view has changed, go to new view
	err := e.startNewView()
	if err != nil {
		return fmt.Errorf("could not start new view: %w", err)
	}

	log.Debug().Msg("local timeout processed")

	return nil
}

// Start will start the pacemaker's timer and start the new view
func (e *EventHandler) Start() error {
	e.paceMaker.Start()
	return e.startNewView()
}

// startNewView will only be called when there is a view change from pacemaker.
// It reads the current view, and check if it needs to propose or vote in this view.
func (e *EventHandler) startNewView() error {

	// track the start time
	start := time.Now()

	curView := e.paceMaker.CurView()

	err := e.persist.PutStarted(curView)
	if err != nil {
		return fmt.Errorf("could not persist current view: %w", err)
	}

	currentLeader, err := e.committee.LeaderForView(curView)
	if err != nil {
		return fmt.Errorf("failed to determine primary for new view %d: %w", curView, err)
	}

	log := e.log.With().
		Uint64("cur_view", curView).
		Hex("leader_id", currentLeader[:]).Logger()
	log.Debug().
		Uint64("finalized_view", e.forks.FinalizedView()).
		Msg("entering new view")
	e.notifier.OnEnteringView(curView, currentLeader)

	e.pruneSubcomponents()

	if e.committee.Self() == currentLeader {
		log.Debug().Msg("generating block proposal as leader")

		// as the leader of the current view,
		// build the block proposal for the current view
		qc, _, err := e.forks.MakeForkChoice(curView)
		if err != nil {
			return fmt.Errorf("can not make fork choice for view %v: %w", curView, err)
		}

		proposal, err := e.blockProducer.MakeBlockProposal(qc, curView)
		if err != nil {
			return fmt.Errorf("can not make block proposal for curView %v: %w", curView, err)
		}
		e.notifier.OnProposingBlock(proposal)

		block := proposal.Block
		log.Debug().
			Uint64("block_view", block.View).
			Hex("block_id", block.BlockID[:]).
			Uint64("parent_view", qc.View).
			Hex("parent_id", qc.BlockID[:]).
			Hex("signer", block.ProposerID[:]).
			Msg("forwarding proposal to communicator for broadcasting")

		// broadcast the proposal
		header := model.ProposalToFlow(proposal)
		delay := e.paceMaker.BlockRateDelay()
		elapsed := time.Since(start)
		if elapsed > delay {
			delay = 0
		} else {
			delay = delay - elapsed
		}
		err = e.communicator.BroadcastProposalWithDelay(header, delay)
		if err != nil {
			log.Warn().Err(err).Msg("could not forward proposal")
		}
		// mark our own proposals to avoid double validation
		e.ownProposal = proposal.Block.BlockID

		// We return here to correspond to the HotStuff state machine.
		return nil
		// Algorithmically, this return statement is optional:
		//  * If this replica is the leader for the current view, there can be no valid proposal from any
		//    other node. This replica's proposal is the only valid proposal.
		//  * This replica's proposal got just sent out above. It will enter the HotStuff logic from the
		//    EventLoop. In other words, the own proposal is not yet stored in Forks.
		//    Hence, Forks cannot contain _any_ valid proposal for the current view.
		//  Therefore, if this replica is the leader, the following code is a no-op.
	}

	// as a replica of the current view, find and process the block for the current view
	blocks := e.forks.GetBlocksForView(curView)
	if len(blocks) == 0 {
		// if there is no block stored before for the current view, then exit and keep waiting
		log.Debug().Msg("waiting for proposal from leader")
		return nil
	}

	// when there are multiple block proposals, we will just pick the first one.
	// forks is responsible for slashing double proposal behavior, and
	// event handler is aware of double proposals, but picking any should work and
	// won't hurt safeness
	block := blocks[0]

	log.Debug().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("parent_view", block.QC.View).
		Hex("parent_id", block.QC.BlockID[:]).
		Hex("signer", block.ProposerID[:]).
		Msg("processing cached proposal from leader")

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
	panic("to be deleted")
	//e.voteAggregator.PruneByView(e.forks.FinalizedView())
}

// processBlockForCurrentView processes the block for the current view.
// It is called AFTER the block has been stored or found in Forks
// It checks whether to vote for this block.
// It might trigger a view change to go to a different view, which might re-enter this function.
func (e *EventHandler) processBlockForCurrentView(block *model.Block) error {

	// this is a sanity check to see if the block is really for the current view.
	curView := e.paceMaker.CurView()
	if block.View != curView {
		return fmt.Errorf("sanity check fails: block proposal's view does not match with curView, (blockView: %v, curView: %v)",
			block.View, curView)
	}

	// checking if I'm the next leader
	nextView := curView + 1
	nextLeader, err := e.committee.LeaderForView(nextView)
	if err != nil {
		return fmt.Errorf("failed to determine primary for next view %d: %w", nextView, err)
	}
	if e.committee.Self() == nextLeader {
		return e.processBlockForCurrentViewIfIsNextLeader(block)
	}

	// if I'm not the next leader
	return e.processBlockForCurrentViewIfIsNotNextLeader(block, nextLeader)
}

func (e *EventHandler) processBlockForCurrentViewIfIsNextLeader(block *model.Block) error {

	log := e.log.With().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("parent_view", block.QC.View).
		Hex("parent_id", block.QC.BlockID[:]).
		Hex("signer", block.ProposerID[:]).
		Logger()

	// voter performs all the checks to decide whether to vote for this block or not.
	// note this call has to make before calling pacemaker, because calling to pace maker first might
	// cause the `curView` here to be stale
	isNextLeader := true
	curView := block.View

	shouldVote := true
	ownVote, err := e.voter.ProduceVoteIfVotable(block, curView)
	if err != nil {
		if !model.IsNoVoteError(err) {
			// unknown error, exit the event loop
			return fmt.Errorf("could not produce vote: %w", err)
		}
		log.Debug().Err(err).Msg("should not vote for this block")
		shouldVote = false
	}

	// if I'm the next leader, then stay at the current view to collect votes for the block of the current view.
	nv, viewChanged := e.paceMaker.UpdateCurViewWithBlock(block, isNextLeader)
	if viewChanged {
		// this is a sanity check.
		// when I've processed the block for the current view and I'm the next leader,
		// the pacemaker should not trigger a view change.
		return fmt.Errorf("pacemaker should not trigger a view change, but it changed to view. (newview: %v)", nv.View)
	}

	if !shouldVote {
		// even if we are not voting for the block, we will still check if a QC can be built for this block.
		err := e.tryBuildQCForBlock(block)
		if err != nil {
			return fmt.Errorf("failed to process QC for block when not voting: %w", err)
		}
		return nil
	}

	// since I'm the next leader, instead of sending the vote through the network and receiving it
	// back, it can be processed locally right away.
	e.notifier.OnVoting(ownVote)
	e.notifier.OnEventProcessed()
	e.notifier.OnReceiveVote(curView, ownVote)
	log.Debug().Msg("processing own vote as next leader")
	return e.processVote(ownVote)
}

func (e *EventHandler) processBlockForCurrentViewIfIsNotNextLeader(block *model.Block, nextLeader flow.Identifier) error {

	log := e.log.With().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("parent_view", block.QC.View).
		Hex("parent_id", block.QC.BlockID[:]).
		Hex("signer", block.ProposerID[:]).
		Logger()

	// voter performs all the checks to decide whether to vote for this block or not.
	isNextLeader := false
	curView := block.View

	shouldVote := true
	ownVote, err := e.voter.ProduceVoteIfVotable(block, curView)
	if err != nil {
		if !model.IsNoVoteError(err) {
			// unknown error, exit the event loop
			return fmt.Errorf("could not produce vote: %w", err)
		}
		log.Debug().Err(err).Msg("should not vote for this block")
		shouldVote = false
	}

	// send my vote if I should vote and I'm not the leader
	if shouldVote {
		e.notifier.OnVoting(ownVote)
		log.Debug().Msg("forwarding vote to compliance engine")

		err := e.communicator.SendVote(ownVote.BlockID, ownVote.View, ownVote.SigData, nextLeader)
		if err != nil {
			log.Warn().Err(err).Msg("could not forward vote")
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
func (e *EventHandler) tryBuildQCForBlock(block *model.Block) error {
	panic("to be deleted")
	//// if the block is not for the current view, try to build QC from votes for this block
	//qc, built, err := e.voteAggregator.BuildQCOnReceivedBlock(block)
	//if err != nil {
	//	return fmt.Errorf("building qc for block (%x) failed: %w", block.BlockID, err)
	//}
	//
	//if !built {
	//	// if we don't have enough votes to build QC for this block, proceed with block.qc instead
	//	qc = block.QC
	//}
	//
	//// process the QC
	//return e.processQC(qc)
}

// processVote stores the vote and check whether a QC can be built.
// If a QC is built, then process the QC.
// It assumes the voting block can be found in forks
func (e *EventHandler) processVote(vote *model.Vote) error {

	panic("to be deleted")
	//log := e.log.With().
	//	Uint64("block_view", vote.View).
	//	Hex("block_id", logging.ID(vote.BlockID)).
	//	Hex("signer", vote.SignerID[:]).
	//	Logger()
	//
	//// read the voting block
	//block, found := e.forks.GetBlock(vote.BlockID)
	//if !found {
	//	// store the pending vote if voting block is not found.
	//	// We don't need to proactively fetch the missing voting block, because the chain compliance layer has acknowledged
	//	// the missing block and requested it already.
	//	_, err := e.voteAggregator.StorePendingVote(vote)
	//	if err != nil {
	//		return fmt.Errorf("can not store pending vote: %w", err)
	//	}
	//	log.Debug().Msg("block for vote not found, caching for later")
	//	return nil
	//}
	//
	//// if the voting block can be found, we should be able to validate the vote
	//// and check if we can build a QC with it.
	//qc, built, err := e.voteAggregator.StoreVoteAndBuildQC(vote, block)
	//if err != nil {
	//	return fmt.Errorf("building qc for block failed: %w", err)
	//}
	//// if we don't have enough votes to build QC for this block:
	//// nothing more to do for processing vote
	//if !built {
	//	log.Debug().Msg("insufficient votes for QC, waiting for more")
	//	return nil
	//}
	//log.Debug().Msg("enough votes for QC collected")
	//
	//return e.processQC(qc)
}

// processQC stores the QC and check whether the QC will trigger view change.
// If triggered, then go to the new view.
func (e *EventHandler) processQC(qc *flow.QuorumCertificate) error {

	log := e.log.With().
		Uint64("block_view", qc.View).
		Hex("block_id", qc.BlockID[:]).
		Int("signers", len(qc.SignerIDs)).
		Logger()

	err := e.forks.AddQC(qc)
	if err != nil {
		return fmt.Errorf("cannot add QC to forks: %w", err)
	}

	_, viewChanged := e.paceMaker.UpdateCurViewWithQC(qc)
	if !viewChanged {
		log.Debug().Msg("QC didn't trigger view change, nothing to do")
		return nil
	}
	log.Debug().Msg("QC triggered view change, starting new view now")

	// current view has changed, go to new view
	return e.startNewView()
}

package eventhandler

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// EventHandlerV2 is the main handler for individual events that trigger state transition.
// It exposes API to handle one event at a time synchronously. The caller is
// responsible for running the event loop to ensure that.
type EventHandlerV2 struct {
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

var _ hotstuff.EventHandlerV2 = (*EventHandlerV2)(nil)

// NewEventHandlerV2 creates an EventHandlerV2 instance with initial components.
func NewEventHandlerV2(
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
) (*EventHandlerV2, error) {
	e := &EventHandlerV2{
		log:            log.With().Str("hotstuff", "participant").Logger(),
		paceMaker:      paceMaker,
		blockProducer:  blockProducer,
		forks:          forks,
		persist:        persist,
		communicator:   communicator,
		voter:          voter,
		validator:      validator,
		committee:      committee,
		voteAggregator: voteAggregator,
		notifier:       notifier,
		ownProposal:    flow.ZeroID,
	}
	return e, nil
}

// OnQCConstructed processes constructed QC by our vote aggregator
func (e *EventHandlerV2) OnQCConstructed(qc *flow.QuorumCertificate) error {
	curView := e.paceMaker.CurView()

	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("qc_view", qc.View).
		Hex("qc_block_id", qc.BlockID[:]).
		Logger()

	e.notifier.OnQcConstructedFromVotes(curView, qc)
	defer e.notifier.OnEventProcessed()

	log.Debug().Msg("received constructed QC")

	// ignore stale qc
	if qc.View < e.forks.FinalizedView() {
		log.Debug().Msg("stale qc")
		return nil
	}

	return e.processQC(qc)
}

// OnReceiveProposal processes the block when a block proposal is received.
// It is assumed that the block proposal is incorporated. (its parent can be found
// in the forks)
func (e *EventHandlerV2) OnReceiveProposal(proposal *model.Proposal) error {

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
			perr := e.voteAggregator.InvalidBlock(proposal)
			if mempool.IsDecreasingPruningHeightError(perr) {
				log.Warn().Err(err).Msgf("invalid block proposal, but vote aggregator has pruned this height: %v", perr)
				return nil
			}

			if perr != nil {
				return fmt.Errorf("vote aggregator could not process invalid block proposal %v, err %v: %w",
					block.BlockID, err, perr)
			}

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

	// notify vote aggregator about a new block, so that it can start verifying
	// votes for it.
	err := e.voteAggregator.AddBlock(proposal)
	if err != nil {
		if !mempool.IsDecreasingPruningHeightError(err) {
			return fmt.Errorf("could not add block (%v) to vote aggregator: %w", block.BlockID, err)
		}
	}

	// store the block.
	err = e.forks.AddBlock(block)
	if err != nil {
		return fmt.Errorf("cannot add block to fork (%x): %w", block.BlockID, err)
	}

	// if the block is not for the current view, then process the QC
	if block.View != curView {
		return e.processQC(block.QC)
	}

	// if the block is for the current view, then check whether to vote for this block
	err = e.processBlockForCurrentView(block)
	if err != nil {
		return fmt.Errorf("failed processing current block: %w", err)
	}

	return nil
}

// TimeoutChannel returns the channel for subscribing the waiting timeout on receiving
// block or votes for the current view.
func (e *EventHandlerV2) TimeoutChannel() <-chan time.Time {
	return e.paceMaker.TimeoutChannel()
}

// OnLocalTimeout is called when the timeout event created by pacemaker looped through the
// event loop.
func (e *EventHandlerV2) OnLocalTimeout() error {

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
func (e *EventHandlerV2) Start() error {
	e.paceMaker.Start()
	return e.startNewView()
}

// startNewView will only be called when there is a view change from pacemaker.
// It reads the current view, and check if it needs to propose or vote in this view.
func (e *EventHandlerV2) startNewView() error {

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
	// won't hurt safety
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

// processBlockForCurrentView processes the block for the current view.
// It is called AFTER the block has been stored or found in Forks
// It checks whether to vote for this block.
// It might trigger a view change to go to a different view, which might re-enter this function.
func (e *EventHandlerV2) processBlockForCurrentView(block *model.Block) error {
	// sanity check that block is really for the current view:
	curView := e.paceMaker.CurView()
	if block.View != curView {
		return fmt.Errorf("sanity check fails: block proposal's view does not match with curView, (blockView: %v, curView: %v)",
			block.View, curView)
	}
	// leader (node ID) for next view
	nextLeader, err := e.committee.LeaderForView(curView + 1)
	if err != nil {
		return fmt.Errorf("failed to determine primary for next view %d: %w", curView+1, err)
	}

	// voter performs all the checks to decide whether to vote for this block or not.
	err = e.ownVote(block, curView, nextLeader)
	if err != nil {
		return fmt.Errorf("unexpected error in voting logic: %w", err)
	}

	// Inform PaceMaker that we've processed a block for the current view. We expect
	// a view change if an only if we are _not_ the next leader. We perform a sanity
	// check here, because a wrong view change can have disastrous consequences.
	isSelfNextLeader := e.committee.Self() == nextLeader
	_, viewChanged := e.paceMaker.UpdateCurViewWithBlock(block, isSelfNextLeader)
	if viewChanged == isSelfNextLeader {
		if isSelfNextLeader {
			return fmt.Errorf("I am primary for next view (%v) and should be collecting votes, but pacemaker triggered already view change", curView+1)
		}
		return fmt.Errorf("pacemaker should trigger a view change to net view (%v), but didn't", curView+1)
	}

	if viewChanged {
		return e.startNewView()
	}
	return nil
}

// ownVote generates and forwards the own vote, if we decide to vote.
// Any errors are potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (e *EventHandlerV2) ownVote(block *model.Block, curView uint64, nextLeader flow.Identifier) error {
	log := e.log.With().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("parent_view", block.QC.View).
		Hex("parent_id", block.QC.BlockID[:]).
		Hex("signer", block.ProposerID[:]).
		Logger()

	// voter performs all the checks to decide whether to vote for this block or not.
	ownVote, err := e.voter.ProduceVoteIfVotable(block, curView)
	if err != nil {
		if !model.IsNoVoteError(err) {
			// unknown error, exit the event loop
			return fmt.Errorf("could not produce vote: %w", err)
		}
		log.Debug().Err(err).Msg("should not vote for this block")
		return nil
	}

	// The following code is only reached, if this replica has produced a vote.
	// Send the vote to the next leader (or directly process it, if I am the next leader).
	e.notifier.OnVoting(ownVote)
	log.Debug().Msg("forwarding vote to compliance engine")
	if e.committee.Self() == nextLeader { // I am the next leader
		e.voteAggregator.AddVote(ownVote)
	} else {
		err = e.communicator.SendVote(ownVote.BlockID, ownVote.View, ownVote.SigData, nextLeader)
		if err != nil {
			log.Warn().Err(err).Msg("could not forward vote")
		}
	}
	return nil
}

// processQC stores the QC and check whether the QC will trigger view change.
// If triggered, then go to the new view.
func (e *EventHandlerV2) processQC(qc *flow.QuorumCertificate) error {

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

package eventhandler

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// EventHandler is the main handler for individual events that trigger state transition.
// It exposes API to handle one event at a time synchronously. The caller is
// responsible for running the event loop to ensure that.
type EventHandler struct {
	log               zerolog.Logger
	paceMaker         hotstuff.PaceMaker
	blockProducer     hotstuff.BlockProducer
	forks             hotstuff.Forks
	persist           hotstuff.Persister
	communicator      hotstuff.Communicator
	committee         hotstuff.Replicas
	voteAggregator    hotstuff.VoteAggregator
	timeoutAggregator hotstuff.TimeoutAggregator
	safetyRules       hotstuff.SafetyRules
	notifier          hotstuff.Consumer
}

var _ hotstuff.EventHandler = (*EventHandler)(nil)

// NewEventHandler creates an EventHandler instance with initial components.
func NewEventHandler(
	log zerolog.Logger,
	paceMaker hotstuff.PaceMaker,
	blockProducer hotstuff.BlockProducer,
	forks hotstuff.Forks,
	persist hotstuff.Persister,
	communicator hotstuff.Communicator,
	committee hotstuff.Replicas,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	safetyRules hotstuff.SafetyRules,
	notifier hotstuff.Consumer,
) (*EventHandler, error) {
	e := &EventHandler{
		log:               log.With().Str("hotstuff", "participant").Logger(),
		paceMaker:         paceMaker,
		blockProducer:     blockProducer,
		forks:             forks,
		persist:           persist,
		communicator:      communicator,
		safetyRules:       safetyRules,
		committee:         committee,
		voteAggregator:    voteAggregator,
		timeoutAggregator: timeoutAggregator,
		notifier:          notifier,
	}
	return e, nil
}

// OnQCConstructed processes constructed QC by our vote aggregator
func (e *EventHandler) OnQCConstructed(qc *flow.QuorumCertificate) error {
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

// OnTCConstructed processes a valid tc constructed by internal vote aggregator.
func (e *EventHandler) OnTCConstructed(tc *flow.TimeoutCertificate) error {
	nve, err := e.paceMaker.ProcessTC(tc)
	if err != nil {
		return fmt.Errorf("could not process constructed TC for view %d: %w", tc.View, err)
	}
	if nve == nil {
		return nil
	}
	return e.startNewView()
}

// OnReceiveProposal processes the block when a block proposal is received.
// It is assumed that the block proposal is incorporated. (its parent can be found
// in the forks)
func (e *EventHandler) OnReceiveProposal(proposal *model.Proposal) error {

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

	// store the block.
	err := e.forks.AddProposal(proposal)
	if err != nil {
		return fmt.Errorf("cannot add proposal to forks (%x): %w", block.BlockID, err)
	}

	_, err = e.paceMaker.ProcessQC(proposal.Block.QC)
	if err != nil {
		return fmt.Errorf("could not process QC for block %x: %w", block.BlockID, err)
	}

	_, err = e.paceMaker.ProcessTC(proposal.LastViewTC)
	if err != nil {
		return fmt.Errorf("could not process TC for block %x: %w", block.BlockID, err)
	}

	// notify vote aggregator about a new block, so that it can start verifying
	// votes for it.
	err = e.voteAggregator.AddBlock(proposal)
	if err != nil {
		if !mempool.IsDecreasingPruningHeightError(err) {
			return fmt.Errorf("could not add block (%v) to vote aggregator: %w", block.BlockID, err)
		}
	}

	// if the block is for the current view, then try voting for this block
	err = e.processBlockForCurrentView(proposal)
	if err != nil {
		return fmt.Errorf("failed processing current block: %w", err)
	}

	// we don't call startNewView() which will trigger proposing logic because to make a proposal
	// we need to construct a QC or TC, both QC and TC will be delivered in dedicated callback
	// (see OnQCCreated and OnTCCreated). When obtaining QC or TC for active view EventHandler calls
	// startNewView() which will initiate proposing logic.

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
	newestQC := e.paceMaker.NewestQC()
	lastViewTC := e.paceMaker.LastViewTC()
	defer e.notifier.OnEventProcessed()

	log := e.log.With().
		Uint64("cur_view", curView).
		//Uint64("new_view", newView.View).
		Logger()
	// 	notifications about time-outs and view-changes are generated by PaceMaker; no need to send a notification here
	log.Debug().Msg("timeout received from event loop")

	timeout, err := e.safetyRules.ProduceTimeout(curView, newestQC, lastViewTC)
	if err != nil {
		return fmt.Errorf("could not produce timeout: %w", err)
	}

	// contribute produced timeout to TC aggregation logic
	e.timeoutAggregator.AddTimeout(timeout)

	// broadcast timeout to participants
	err = e.communicator.BroadcastTimeout(timeout)
	if err != nil {
		log.Warn().Err(err).Msg("could not forward vote")
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
		newestQC := e.paceMaker.NewestQC()
		lastViewTC := e.paceMaker.LastViewTC()

		_, found := e.forks.GetProposal(newestQC.BlockID)
		if !found {
			// we don't know anything about block referenced by our newest QC, in this case we can't
			// create a valid proposal since we can't guarantee validity of block payload.
			log.Debug().
				Uint64("qc_view", newestQC.View).
				Hex("block_id", newestQC.BlockID[:]).Msg("no parent found for newest QC, can't propose")
			return nil
		}

		// perform sanity checks to make sure that resulted proposal is valid.
		// to create proposal leader for view N needs to present evidence that has was allowed to.
		// To do that he includes QC or TC for view N-1. Note that PaceMaker advances views only after observing QC or TC,
		// moreover QC and TC are processed always together, keeping in mind that EventHandler is used strictly single-threaded without reentrancy
		// we reach a conclusion that we must have a QC or TC with view equal to (curView-1). Failing one of these sanity checks
		// is a symptom of state corruption or a severe implementation bug.
		if newestQC.View+1 != curView {
			if lastViewTC == nil {
				return fmt.Errorf("possible state corruption, expected lastViewTC to be not nil")
			}
			if lastViewTC.View+1 != curView {
				return fmt.Errorf("possible state corruption, don't have QC(view=%d) and TC(view=%d) for previous view(currentView=%d)",
					newestQC.View, lastViewTC.View, curView)
			}
		} else {
			// in case last view has ended with QC and TC, make sure that only QC is included
			// otherwise such proposal is invalid. This case is possible if TC has included QC with the same
			// view as the TC itself, meaning that newestQC.View == lastViewTC.View
			lastViewTC = nil
		}

		proposal, err := e.blockProducer.MakeBlockProposal(newestQC, curView, lastViewTC)
		if err != nil {
			return fmt.Errorf("can not make block proposal for curView %v: %w", curView, err)
		}
		e.notifier.OnProposingBlock(proposal)

		block := proposal.Block
		log.Debug().
			Uint64("block_view", block.View).
			Hex("block_id", block.BlockID[:]).
			Uint64("parent_view", newestQC.View).
			Hex("parent_id", newestQC.BlockID[:]).
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
	proposals := e.forks.GetProposalsForView(curView)
	if len(proposals) == 0 {
		// if there is no block stored before for the current view, then exit and keep waiting
		log.Debug().Msg("waiting for proposal from leader")
		return nil
	}

	// TODO(active-pacemaker): add processing of cached proposals
	return nil

	// when there are multiple block proposals, we will just pick the first one.
	// forks is responsible for slashing double proposal behavior, and
	// event handler is aware of double proposals, but picking any should work and
	// won't hurt safety
	//block := proposals[0]
	//
	//log.Debug().
	//	Uint64("block_view", block.View).
	//	Hex("block_id", block.BlockID[:]).
	//	Uint64("parent_view", block.QC.View).
	//	Hex("parent_id", block.QC.BlockID[:]).
	//	Hex("signer", block.ProposerID[:]).
	//	Msg("processing cached proposal from leader")
	//
	//
	//return e.processBlockForCurrentView(block)
}

// processBlockForCurrentView processes the block for the current view.
// It is called AFTER the block has been stored or found in Forks
// It checks whether to vote for this block.
func (e *EventHandler) processBlockForCurrentView(proposal *model.Proposal) error {
	// sanity check that block is really for the current view:
	curView := e.paceMaker.CurView()
	block := proposal.Block
	if block.View != curView {
		// ignore outdated proposals in case we have moved forward
		return nil
	}
	// leader (node ID) for next view
	nextLeader, err := e.committee.LeaderForView(curView + 1)
	if err != nil {
		return fmt.Errorf("failed to determine primary for next view %d: %w", curView+1, err)
	}

	// safetyRules performs all the checks to decide whether to vote for this block or not.
	err = e.ownVote(proposal, curView, nextLeader)
	if err != nil {
		return fmt.Errorf("unexpected error in voting logic: %w", err)
	}

	return nil
}

// ownVote generates and forwards the own vote, if we decide to vote.
// Any errors are potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (e *EventHandler) ownVote(proposal *model.Proposal, curView uint64, nextLeader flow.Identifier) error {
	block := proposal.Block
	log := e.log.With().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("parent_view", block.QC.View).
		Hex("parent_id", block.QC.BlockID[:]).
		Hex("signer", block.ProposerID[:]).
		Logger()

	_, found := e.forks.GetProposal(proposal.Block.QC.BlockID)
	if !found {
		// we don't have parent for this proposal, we can't vote since we can't guarantee validity of proposals
		// payload. Strictly speaking this shouldn't ever happen because compliance engine makes sure that we
		// receive proposals with valid parents.
		log.Warn().Msg("won't vote for proposal, no parent block for this proposal")
		return nil
	}

	// safetyRules performs all the checks to decide whether to vote for this block or not.
	ownVote, err := e.safetyRules.ProduceVote(proposal, curView)
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
func (e *EventHandler) processQC(qc *flow.QuorumCertificate) error {

	log := e.log.With().
		Uint64("block_view", qc.View).
		Hex("block_id", qc.BlockID[:]).
		Logger()

	newViewEvent, err := e.paceMaker.ProcessQC(qc)
	if err != nil {
		return fmt.Errorf("could not process QC: %w", err)
	}
	if newViewEvent == nil {
		log.Debug().Msg("QC didn't trigger view change, nothing to do")
		return nil
	}
	log.Debug().Msg("QC triggered view change, starting new view now")

	// current view has changed, go to new view
	return e.startNewView()
}

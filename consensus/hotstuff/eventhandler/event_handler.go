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

// EventHandler is the main handler for individual events that trigger state transition.
// It exposes API to handle one event at a time synchronously. EventHandler is *not concurrency safe*.
// Please use the EventLoop to ensure that only a single go-routine executes the EventHandler's algorithms.
// EventHandler is implemented in event-driven way, it reacts to incoming events and performs certain actions.
// It doesn't perform any actions on its own. There are 3 main responsibilities of EventHandler, vote, propose,
// timeout. There are specific scenarios that lead to each of those actions.
//  - create vote: voting logic is triggered by OnReceiveProposal, after receiving proposal we have all required information
//  to create a valid vote. Compliance engine makes sure that we receive proposals, whose parents are known.
//  Creating a vote can be triggered ONLY by receiving proposal.
//  - create timeout: creating model.TimeoutObject[TO] is triggered by OnLocalTimeout, after reaching deadline for current round.
//  EventHandler gets notified about it and has to create a model.TimeoutObject and broadcast it to other replicas.
//  Creating a TO can be triggered ONLY by reaching round deadline.
//  - create a proposal: proposing logic is more complicated. Creating a proposal is triggered by the EventHandler receiving
//  a QC or TC that induces a view change to a view where the replica is primary. As an edge case, the EventHandler
//  can receive a QC or TC that triggers the view change, but we can't create a proposal in case we are missing parent block the newest QC refers to.
//  In case we already have the QC, but are still missing the respective parent, OnReceiveProposal can trigger the proposing logic
//  as welll, but only when receiving proposal for view lower than active view.
//  To summarize, to make a valid proposal for view N we need to have a QC or TC for N-1 and know the proposal with blockID
//  NewestQC.BlockID.
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

// OnReceiveQc processes a valid qc constructed by internal vote aggregator or discovered in TimeoutObject.
// All inputs should be validated before feeding into this function. Assuming trusted data.
// No errors are expected during normal operation.
func (e *EventHandler) OnReceiveQc(qc *flow.QuorumCertificate) error {
	curView := e.paceMaker.CurView()

	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("qc_view", qc.View).
		Hex("qc_block_id", qc.BlockID[:]).
		Logger()

	e.notifier.OnQcConstructedFromVotes(curView, qc)
	defer e.notifier.OnEventProcessed()

	log.Debug().Msg("received QC")

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
	return e.proposeForNewViewIfPrimary()
}

// OnReceiveTc processes a valid tc constructed by internal timeout aggregator, discovered in TimeoutObject or
// broadcast over the network.
// All inputs should be validated before feeding into this function. Assuming trusted data.
// No errors are expected during normal operation.
func (e *EventHandler) OnReceiveTc(tc *flow.TimeoutCertificate) error {
	curView := e.paceMaker.CurView()

	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("tc_view", tc.View).
		Uint64("tc_newest_qc_view", tc.NewestQC.View).
		Hex("tc_newest_qc_block_id", tc.NewestQC.BlockID[:]).
		Logger()

	defer e.notifier.OnEventProcessed()

	log.Debug().Msg("received TC")

	newViewEvent, err := e.paceMaker.ProcessTC(tc)
	if err != nil {
		return fmt.Errorf("could not process TC for view %d: %w", tc.View, err)
	}
	if newViewEvent == nil {
		log.Debug().Msg("TC didn't trigger view change, nothing to do")
		return nil
	}
	log.Debug().Msg("TC triggered view change, starting new view now")

	// current view has changed, go to new view
	return e.proposeForNewViewIfPrimary()
}

// OnReceiveProposal processes a block proposal received from another HotStuff
// consensus participant.
// All inputs should be validated before feeding into this function. Assuming trusted data.
// No errors are expected during normal operation.
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
		if !mempool.IsBelowPrunedThresholdError(err) {
			return fmt.Errorf("could not add block (%v) to vote aggregator: %w", block.BlockID, err)
		}
	}

	// if the block is for the current view, then try voting for this block
	err = e.processBlockForCurrentView(proposal)
	if err != nil {
		return fmt.Errorf("failed processing current block: %w", err)
	}

	// nothing to do if this proposal is for current view
	if proposal.Block.View == e.paceMaker.CurView() {
		return nil
	}

	return e.proposeForNewViewIfPrimary()
}

// TimeoutChannel returns the channel for subscribing the waiting timeout on receiving
// block or votes for the current view.
func (e *EventHandler) TimeoutChannel() <-chan time.Time {
	return e.paceMaker.TimeoutChannel()
}

// OnLocalTimeout will handle local timeout event by creating TimeoutObject and broadcasting it.
// No errors are expected during normal operation.
func (e *EventHandler) OnLocalTimeout() error {

	curView := e.paceMaker.CurView()
	newestQC := e.paceMaker.NewestQC()
	lastViewTC := e.paceMaker.LastViewTC()
	defer e.notifier.OnEventProcessed()

	log := e.log.With().
		Uint64("cur_view", curView).
		Logger()
	// 	notifications about time-outs and view-changes are generated by PaceMaker; no need to send a notification here
	log.Debug().Msg("timeout received from event loop")

	if newestQC.View+1 == curView {
		// in case last view has ended with QC and TC, make sure that only QC is included
		// otherwise such timeout is invalid. This case is possible if TC has included QC with the same
		// view as the TC itself, meaning that newestQC.View == lastViewTC.View
		lastViewTC = nil
	}

	timeout, err := e.safetyRules.ProduceTimeout(curView, newestQC, lastViewTC)
	if err != nil {
		if model.IsNoTimeoutError(err) {
			log.Warn().Err(err).Msgf("could not produce timeout at view %d, we are ejected", curView)
			return nil
		}
		return fmt.Errorf("could not produce timeout: %w", err)
	}

	// contribute produced timeout to TC aggregation logic
	e.timeoutAggregator.AddTimeout(timeout)

	// broadcast timeout to participants
	err = e.communicator.BroadcastTimeout(timeout)
	if err != nil {
		log.Warn().Err(err).Msg("could not forward timeout")
	}

	log.Debug().Msg("local timeout processed")

	return nil
}

// Start starts the event handler.
// No errors are expected during normal operation.
func (e *EventHandler) Start() error {
	err := e.processPendingBlocks()
	if err != nil {
		return fmt.Errorf("could not process pending blocks: %w", err)
	}
	err = e.proposeForNewViewIfPrimary()
	if err != nil {
		return fmt.Errorf("could not start new view: %w", err)
	}
	e.paceMaker.Start()
	return nil
}

// processPendingBlocks performs processing of pending blocks that were applied to chain state but weren't processed
// by Hotstuff event loop. Due to asynchronous nature of our processing pipelines compliance engine can validate and apply
// blocks to the chain state but fail to deliver them to EventHandler because of shutdown or crash. To recover those QCs and TCs
// recovery logic puts them in Forks and EventHandler can traverse pending blocks by view to obtain them.
func (e *EventHandler) processPendingBlocks() error {
	newestView := e.forks.NewestView()
	currentView := e.paceMaker.CurView()
	for {
		paceMakerActiveView := e.paceMaker.CurView()
		if currentView < paceMakerActiveView {
			currentView = paceMakerActiveView
		}

		if currentView > newestView {
			return nil
		}

		// check if there are pending proposals for active view
		pendingProposals := e.forks.GetProposalsForView(currentView)
		// process all proposals for view, we are dealing only with valid QCs and TCs so no harm in processing
		// double proposals here.
		for _, proposal := range pendingProposals {
			block := proposal.Block
			_, err := e.paceMaker.ProcessQC(block.QC)
			if err != nil {
				return fmt.Errorf("could not process QC for block %x: %w", block.BlockID, err)
			}

			_, err = e.paceMaker.ProcessTC(proposal.LastViewTC)
			if err != nil {
				return fmt.Errorf("could not process TC for block %x: %w", block.BlockID, err)
			}

			// TODO(active-pacemaker): generally speaking we are only interested in QC and TC, but in some cases
			// we might want to vote for blocks as well. Discuss if it's needed.
		}

		currentView++
	}
}

// proposeForNewViewIfPrimary will only be called when we may able to propose a block, after processing a new event.
// * after entering a new view as a result of processing a QC or TC, then we may propose for the newly entered view
// * after receiving a proposal (but not changing view), if that proposal is referenced by our highest known QC,
//   and the proposal was previously unknown, then we can propose a block in the current view
// It reads the current view, and generates a proposal if we are the leader.
// No errors are expected during normal operation.
func (e *EventHandler) proposeForNewViewIfPrimary() error {
	start := time.Now() // track the start time

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

	if e.committee.Self() != currentLeader {
		return nil
	}
	log.Debug().Msg("generating block proposal as leader")

	// as the leader of the current view,
	// build the block proposal for the current view
	newestQC := e.paceMaker.NewestQC()
	lastViewTC := e.paceMaker.LastViewTC()

	_, found := e.forks.GetProposal(newestQC.BlockID)
	if !found {
		// we don't know anything about block referenced by our newest QC, in this case we can't
		// create a valid proposal since we can't guarantee validity of block payload.
		log.Warn().
			Uint64("qc_view", newestQC.View).
			Hex("block_id", newestQC.BlockID[:]).Msg("haven't synced the latest block yet; can't propose")
		return nil
	}

	// Sanity checks to make sure that resulting proposal is valid:
	// * in its proposal, leader for view N needs to present evidence that has legitimately entered view N
	// To do that he includes QC or TC for view N-1. Note that the PaceMaker advances to view N only after observing QC or TC from view N-1.
	// moreover QC and TC are processed always together. As EventHandler is used strictly single-threaded without reentrancy,
	// we must have a QC or TC for the prior view (curView-1). Failing one of these sanity checks
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
		// In case last view has ended with QC and TC, make sure that only QC is included,
		// otherwise such proposal is invalid. This case is possible if TC has included QC with the same
		// view as the TC itself, meaning that newestQC.View == lastViewTC.View
		lastViewTC = nil
	}

	proposal, err := e.blockProducer.MakeBlockProposal(curView, newestQC, lastViewTC)
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

	return nil
}

// processBlockForCurrentView processes the block for the current view.
// It is called AFTER the block has been stored or found in Forks
// It checks whether to vote for this block.
// No errors are expected during normal operation.
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
	if errors.Is(err, model.ErrViewForUnknownEpoch) {
		// We are attempting process a block in an unknown epoch
		// This should never happen, because:
		// * the compliance layer ensures proposals are passed to the event loop strictly after their parent
		// * the protocol state ensures that, before incorporating the first block of an epoch E,
		//    either E is known or we have triggered epoch fallback mode - in either case the epoch for the
		//    current epoch is known
		return fmt.Errorf("attempting to process a block for current view in unknown epoch")
	}
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
// No errors are expected during normal operation.
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
		return fmt.Errorf("won't vote for proposal, no parent block for this proposal")
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

	if e.committee.Self() == nextLeader { // I am the next leader
		log.Debug().Msg("forwarding vote to vote aggregator")
		e.voteAggregator.AddVote(ownVote)
	} else {
		log.Debug().Msg("forwarding vote to compliance engine")
		err = e.communicator.SendVote(ownVote.BlockID, ownVote.View, ownVote.SigData, nextLeader)
		if err != nil {
			log.Warn().Err(err).Msg("could not forward vote")
		}
	}
	return nil
}

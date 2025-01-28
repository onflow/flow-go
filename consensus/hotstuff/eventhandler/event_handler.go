package eventhandler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

// EventHandler is the main handler for individual events that trigger state transition.
// It exposes API to handle one event at a time synchronously. EventHandler is *not concurrency safe*.
// Please use the EventLoop to ensure that only a single go-routine executes the EventHandler's algorithms.
// EventHandler is implemented in event-driven way, it reacts to incoming events and performs certain actions.
// It doesn't perform any actions on its own. There are 3 main responsibilities of EventHandler, vote, propose,
// timeout. There are specific scenarios that lead to each of those actions.
//   - create vote: voting logic is triggered by OnReceiveProposal, after receiving proposal we have all required information
//     to create a valid vote. Compliance engine makes sure that we receive proposals, whose parents are known.
//     Creating a vote can be triggered ONLY by receiving proposal.
//   - create timeout: creating model.TimeoutObject[TO] is triggered by OnLocalTimeout, after reaching deadline for current round.
//     EventHandler gets notified about it and has to create a model.TimeoutObject and broadcast it to other replicas.
//     Creating a TO can be triggered by reaching round deadline or triggered as part of Bracha broadcast when superminority
//     of replicas have contributed to TC creation and created a partial TC.
//   - create a proposal: proposing logic is more complicated. Creating a proposal is triggered by the EventHandler receiving
//     a QC or TC that induces a view change to a view where the replica is primary. As an edge case, the EventHandler
//     can receive a QC or TC that triggers the view change, but we can't create a proposal in case we are missing parent block the newest QC refers to.
//     In case we already have the QC, but are still missing the respective parent, OnReceiveProposal can trigger the proposing logic
//     as well, but only when receiving proposal for view lower than active view.
//     To summarize, to make a valid proposal for view N we need to have a QC or TC for N-1 and know the proposal with blockID
//     NewestQC.BlockID.
//
// Not concurrency safe.
type EventHandler struct {
	log           zerolog.Logger
	paceMaker     hotstuff.PaceMaker
	blockProducer hotstuff.BlockProducer
	forks         hotstuff.Forks
	persist       hotstuff.Persister
	committee     hotstuff.Replicas
	safetyRules   hotstuff.SafetyRules
	notifier      hotstuff.Consumer
}

var _ hotstuff.EventHandler = (*EventHandler)(nil)

// NewEventHandler creates an EventHandler instance with initial components.
func NewEventHandler(
	log zerolog.Logger,
	paceMaker hotstuff.PaceMaker,
	blockProducer hotstuff.BlockProducer,
	forks hotstuff.Forks,
	persist hotstuff.Persister,
	committee hotstuff.Replicas,
	safetyRules hotstuff.SafetyRules,
	notifier hotstuff.Consumer,
) (*EventHandler, error) {
	e := &EventHandler{
		log:           log.With().Str("hotstuff", "participant").Logger(),
		paceMaker:     paceMaker,
		blockProducer: blockProducer,
		forks:         forks,
		persist:       persist,
		safetyRules:   safetyRules,
		committee:     committee,
		notifier:      notifier,
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
	log.Debug().Msg("received QC")
	e.notifier.OnReceiveQc(curView, qc)
	defer e.notifier.OnEventProcessed()

	newViewEvent, err := e.paceMaker.ProcessQC(qc)
	if err != nil {
		return fmt.Errorf("could not process QC: %w", err)
	}
	if newViewEvent == nil {
		log.Debug().Msg("QC didn't trigger view change, nothing to do")
		return nil
	}

	// current view has changed, go to new view
	log.Debug().Msg("QC triggered view change, starting new view now")
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
	log.Debug().Msg("received TC")
	e.notifier.OnReceiveTc(curView, tc)
	defer e.notifier.OnEventProcessed()

	newViewEvent, err := e.paceMaker.ProcessTC(tc)
	if err != nil {
		return fmt.Errorf("could not process TC for view %d: %w", tc.View, err)
	}
	if newViewEvent == nil {
		log.Debug().Msg("TC didn't trigger view change, nothing to do")
		return nil
	}

	// current view has changed, go to new view
	log.Debug().Msg("TC triggered view change, starting new view now")
	return e.proposeForNewViewIfPrimary()
}

// OnReceiveProposal processes a block proposal received from another HotStuff
// consensus participant.
// All inputs should be validated before feeding into this function. Assuming trusted data.
// No errors are expected during normal operation.
func (e *EventHandler) OnReceiveProposal(proposal *model.SignedProposal) error {
	block := proposal.Block
	curView := e.paceMaker.CurView()
	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("qc_view", block.QC.View).
		Hex("proposer_id", block.ProposerID[:]).
		Logger()
	log.Debug().Msg("proposal received from compliance engine")
	e.notifier.OnReceiveProposal(curView, proposal)
	defer e.notifier.OnEventProcessed()

	// ignore stale proposals
	if block.View < e.forks.FinalizedView() {
		log.Debug().Msg("stale proposal")
		return nil
	}

	// store the block.
	err := e.forks.AddValidatedBlock(block)
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

	// if the block is for the current view, then try voting for this block
	err = e.processBlockForCurrentView(proposal)
	if err != nil {
		return fmt.Errorf("failed processing current block: %w", err)
	}
	log.Debug().Msg("proposal processed from compliance engine")

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

// OnLocalTimeout handles a local timeout event by creating a model.TimeoutObject and broadcasting it.
// No errors are expected during normal operation.
func (e *EventHandler) OnLocalTimeout() error {
	curView := e.paceMaker.CurView()
	e.log.Debug().Uint64("cur_view", curView).Msg("timeout received from event loop")
	e.notifier.OnLocalTimeout(curView)
	defer e.notifier.OnEventProcessed()

	err := e.broadcastTimeoutObjectIfAuthorized()
	if err != nil {
		return fmt.Errorf("unexpected exception while processing timeout in view %d: %w", curView, err)
	}
	return nil
}

// OnPartialTcCreated handles notification produces by the internal timeout aggregator.
// If the notification is for the current view, a corresponding model.TimeoutObject is broadcast to the consensus committee.
// No errors are expected during normal operation.
func (e *EventHandler) OnPartialTcCreated(partialTC *hotstuff.PartialTcCreated) error {
	curView := e.paceMaker.CurView()
	lastViewTC := partialTC.LastViewTC
	logger := e.log.With().
		Uint64("cur_view", curView).
		Uint64("qc_view", partialTC.NewestQC.View)
	if lastViewTC != nil {
		logger.Uint64("last_view_tc_view", lastViewTC.View)
	}
	log := logger.Logger()
	log.Debug().Msg("constructed partial TC")

	e.notifier.OnPartialTc(curView, partialTC)
	defer e.notifier.OnEventProcessed()

	// process QC, this might trigger view change
	_, err := e.paceMaker.ProcessQC(partialTC.NewestQC)
	if err != nil {
		return fmt.Errorf("could not process newest QC: %w", err)
	}

	// process TC, this might trigger view change
	_, err = e.paceMaker.ProcessTC(lastViewTC)
	if err != nil {
		return fmt.Errorf("could not process TC for view %d: %w", lastViewTC.View, err)
	}

	// NOTE: in other cases when we have observed a view change we will trigger proposing logic, this is desired logic
	// for handling proposal, QC and TC. However, observing a partial TC means
	// that superminority have timed out and there was at least one honest replica in that set. Honest replicas will never vote
	// after timing out for current view meaning we won't be able to collect supermajority of votes for a proposal made after
	// observing partial TC.

	// by definition, we are allowed to produce timeout object if we have received partial TC for current view
	if e.paceMaker.CurView() != partialTC.View {
		return nil
	}

	log.Debug().Msg("partial TC generated for current view, broadcasting timeout")
	err = e.broadcastTimeoutObjectIfAuthorized()
	if err != nil {
		return fmt.Errorf("unexpected exception while processing partial TC in view %d: %w", partialTC.View, err)
	}
	return nil
}

// Start starts the event handler.
// No errors are expected during normal operation.
// CAUTION: EventHandler is not concurrency safe. The Start method must
// be executed by the same goroutine that also calls the other business logic
// methods, or concurrency safety has to be implemented externally.
func (e *EventHandler) Start(ctx context.Context) error {
	e.notifier.OnStart(e.paceMaker.CurView())
	defer e.notifier.OnEventProcessed()
	e.paceMaker.Start(ctx)
	err := e.proposeForNewViewIfPrimary()
	if err != nil {
		return fmt.Errorf("could not start new view: %w", err)
	}
	return nil
}

// broadcastTimeoutObjectIfAuthorized attempts to generate a model.TimeoutObject, adds it
// to `timeoutAggregator` and broadcasts it to the consensus commettee. We check, whether
// this node, at the current view, is part of the consensus committee. Otherwise, this
// method is functionally a no-op.
// For example, right after an epoch switchover a consensus node might still be online but
// not part of the _active_ consensus committee anymore. Consequently, it should not broadcast
// timeouts anymore.
// No errors are expected during normal operation.
func (e *EventHandler) broadcastTimeoutObjectIfAuthorized() error {
	curView := e.paceMaker.CurView()
	newestQC := e.paceMaker.NewestQC()
	lastViewTC := e.paceMaker.LastViewTC()
	log := e.log.With().Uint64("cur_view", curView).Logger()

	if newestQC.View+1 == curView {
		// in case last view has ended with QC and TC, make sure that only QC is included
		// otherwise such timeout is invalid. This case is possible if TC has included QC with the same
		// view as the TC itself, meaning that newestQC.View == lastViewTC.View
		lastViewTC = nil
	}

	timeout, err := e.safetyRules.ProduceTimeout(curView, newestQC, lastViewTC)
	if err != nil {
		if model.IsNoTimeoutError(err) {
			log.Warn().Err(err).Msgf("not generating timeout as this node is not part of the active committee")
			return nil
		}
		return fmt.Errorf("could not produce timeout: %w", err)
	}

	// raise a notification to broadcast timeout
	e.notifier.OnOwnTimeout(timeout)
	log.Debug().Msg("broadcast TimeoutObject done")

	return nil
}

// proposeForNewViewIfPrimary will only be called when we may be able to propose a block, after processing a new event.
//   - after entering a new view as a result of processing a QC or TC, then we may propose for the newly entered view
//   - after receiving a proposal (but not changing view), if that proposal is referenced by our highest known QC,
//     and the proposal was previously unknown, then we can propose a block in the current view
//
// Enforced INVARIANTS:
//   - There will at most be `OnOwnProposal` notification emitted for views where this node is the leader, and none
//     if another node is the leader. This holds irrespective of restarts. Formally, this prevents proposal equivocation.
//
// It reads the current view, and generates a proposal if we are the leader.
// No errors are expected during normal operation.
func (e *EventHandler) proposeForNewViewIfPrimary() error {
	start := time.Now() // track the start time
	curView := e.paceMaker.CurView()
	currentLeader, err := e.committee.LeaderForView(curView)
	if err != nil {
		return fmt.Errorf("failed to determine primary for new view %d: %w", curView, err)
	}
	finalizedView := e.forks.FinalizedView()
	log := e.log.With().
		Uint64("cur_view", curView).
		Uint64("finalized_view", finalizedView).
		Hex("leader_id", currentLeader[:]).Logger()

	e.notifier.OnCurrentViewDetails(curView, finalizedView, currentLeader)

	// check that I am the primary for this view
	if e.committee.Self() != currentLeader {
		return nil
	}

	// attempt to generate proposal:
	newestQC := e.paceMaker.NewestQC()
	lastViewTC := e.paceMaker.LastViewTC()

	_, found := e.forks.GetBlock(newestQC.BlockID)
	if !found {
		// we don't know anything about block referenced by our newest QC, in this case we can't
		// create a valid proposal since we can't guarantee validity of block payload.
		log.Warn().
			Uint64("qc_view", newestQC.View).
			Hex("block_id", newestQC.BlockID[:]).Msg("haven't synced the latest block yet; can't propose")
		return nil
	}
	log.Debug().Msg("generating proposal as leader")

	// Sanity checks to make sure that resulting proposal is valid:
	// In its proposal, the leader for view N needs to present evidence that it has legitimately entered view N.
	// As evidence, we include a QC or TC for view N-1, which should always be available as the PaceMaker advances
	// to view N only after observing a QC or TC from view N-1. Moreover, QC and TC are always processed together. As
	// EventHandler is strictly single-threaded without reentrancy, we must have a QC or TC for the prior view (curView-1).
	// Failing one of these sanity checks is a symptom of state corruption or a severe implementation bug.
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

	// Construct Own SignedProposal
	// CAUTION, design constraints:
	//    (i) We cannot process our own proposal within the `EventHandler` right away.
	//   (ii) We cannot add our own proposal to Forks here right away.
	//  (iii) Metrics for the PaceMaker/CruiseControl assume that the EventHandler is the only caller of
	//	     `TargetPublicationTime`. Technically, `TargetPublicationTime` records the publication delay
	//	      relative to its _latest_ call.
	//
	// To satisfy all constraints, we construct the proposal here and query (once!) its `TargetPublicationTime`. Though,
	// we do _not_ process our own blocks right away and instead ingest them into the EventHandler the same way as
	// proposals from other consensus participants. Specifically, on the path through the HotStuff state machine leading
	// to block construction, the node's own proposal is largely ephemeral. The proposal is handed to the `MessageHub` (via
	// the `OnOwnProposal` notification including the `TargetPublicationTime`). The `MessageHub` waits until
	// `TargetPublicationTime` and only then broadcast the proposal and puts it into the EventLoop's queue
	// for inbound blocks. This is exactly the same way as proposals from other nodes are ingested by the `EventHandler`,
	// except that we are skipping the ComplianceEngine (assuming that our own proposals are protocol-compliant).
	//
	// Context:
	//  • On constraint (i): We want to support consensus committees only consisting of a *single* node. If the EventHandler
	//    internally processed the block right away via a direct message call, the call-stack would be ever-growing and
	//    the node would crash eventually (we experienced this with a very early HotStuff implementation). Specifically,
	//    if we wanted to process the block directly without taking a detour through the EventLoop's inbound queue,
	//    we would call `OnReceiveProposal` here. The function `OnReceiveProposal` would then end up calling
	//    `proposeForNewViewIfPrimary` (this function) to generate the next proposal, which again
	//    would result in calling `OnReceiveProposal` and so on so forth until the call stack or memory limit is reached
	//    and the node crashes. This is only a problem for consensus committees of size 1.
	//  • On constraint (ii): When adding a proposal to Forks, Forks emits a `BlockIncorporatedEvent` notification, which
	//    is observed by Cruse Control and would change its state. However, note that Cruse Control is trying to estimate
	//    the point in time when _other_ nodes are observing the proposal. The time when we broadcast the proposal (i.e.
	//    `TargetPublicationTime`) is a reasonably good estimator, but *not* the time the proposer constructed the block
	//    (because there is potentially still a significant wait until `TargetPublicationTime`).
	//
	// The current approach is for a node to process its own proposals at the same time and through the same code path as
	// proposals from other nodes. This satisfies constraints (i) and (ii) and generates very strong consistency, from a
	// software design perspective.
	//    Just hypothetically, if we changed Cruise Control to be notified about own block proposals _only_ when they are
	// broadcast (satisfying constraint (ii) without relying on the EventHandler), then we could add a proposal to Forks
	// here right away. Nevertheless, the restriction remains that we cannot process that proposal right away within the
	// EventHandler and instead need to put it into the EventLoop's inbound queue to support consensus committees of size 1.
	flowProposal, err := e.blockProducer.MakeBlockProposal(curView, newestQC, lastViewTC)
	if err != nil {
		if model.IsNoVoteError(err) {
			log.Info().Err(err).Msg("aborting block proposal to prevent equivocation (likely re-entered proposal logic due to crash)")

			return nil
		}
		return fmt.Errorf("can not make block proposal for curView %v: %w", curView, err)
	}
	targetPublicationTime := e.paceMaker.TargetPublicationTime(flowProposal.View, start, flowProposal.ParentID) // determine target publication time
	log.Debug().
		Uint64("block_view", flowProposal.View).
		Time("target_publication", targetPublicationTime).
		Hex("block_id", logging.ID(flowProposal.ID())).
		Uint64("parent_view", newestQC.View).
		Hex("parent_id", newestQC.BlockID[:]).
		Hex("signer", flowProposal.ProposerID[:]).
		Msg("forwarding proposal to communicator for broadcasting")

	// emit notification with own proposal (also triggers broadcast)
	e.notifier.OnOwnProposal(flowProposal, targetPublicationTime)
	return nil
}

// processBlockForCurrentView processes the block for the current view.
// It is called AFTER the block has been stored or found in Forks
// It checks whether to vote for this block.
// No errors are expected during normal operation.
func (e *EventHandler) processBlockForCurrentView(proposal *model.SignedProposal) error {
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
func (e *EventHandler) ownVote(proposal *model.SignedProposal, curView uint64, nextLeader flow.Identifier) error {
	block := proposal.Block
	log := e.log.With().
		Uint64("block_view", block.View).
		Hex("block_id", block.BlockID[:]).
		Uint64("parent_view", block.QC.View).
		Hex("parent_id", block.QC.BlockID[:]).
		Hex("signer", block.ProposerID[:]).
		Logger()

	_, found := e.forks.GetBlock(proposal.Block.QC.BlockID)
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

	log.Debug().Msg("forwarding vote to compliance engine")
	// raise a notification to send vote
	e.notifier.OnOwnVote(ownVote.BlockID, ownVote.View, ownVote.SigData, nextLeader)
	return nil
}

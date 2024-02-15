package hotstuff

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// ProposalViolationConsumer consumes outbound notifications about HotStuff-protocol violations.
// Such notifications are produced by the active consensus participants and consensus follower.
//
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type ProposalViolationConsumer interface {
	// OnInvalidBlockDetected notifications are produced by components that have detected
	// that a block proposal is invalid and need to report it.
	// Most of the time such block can be detected by calling Validator.ValidateProposal.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnInvalidBlockDetected(err flow.Slashable[model.InvalidProposalError])

	// OnDoubleProposeDetected notifications are produced by the Finalization Logic
	// whenever a double block proposal (equivocation) was detected.
	// Equivocation occurs when the same leader proposes two different blocks for the same view.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleProposeDetected(*model.Block, *model.Block)
}

// VoteAggregationViolationConsumer consumes outbound notifications about HotStuff-protocol violations specifically
// invalid votes during processing.
// Such notifications are produced by the Vote Aggregation logic.
//
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type VoteAggregationViolationConsumer interface {
	// OnDoubleVotingDetected notifications are produced by the Vote Aggregation logic
	// whenever a double voting (same voter voting for different blocks at the same view) was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleVotingDetected(*model.Vote, *model.Vote)

	// OnInvalidVoteDetected notifications are produced by the Vote Aggregation logic
	// whenever an invalid vote was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnInvalidVoteDetected(err model.InvalidVoteError)

	// OnVoteForInvalidBlockDetected notifications are produced by the Vote Aggregation logic
	// whenever vote for invalid proposal was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal)
}

// TimeoutAggregationViolationConsumer consumes outbound notifications about Active Pacemaker violations specifically
// invalid timeouts during processing.
// Such notifications are produced by the Timeout Aggregation logic.
//
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type TimeoutAggregationViolationConsumer interface {
	// OnDoubleTimeoutDetected notifications are produced by the Timeout Aggregation logic
	// whenever a double timeout (same replica producing two different timeouts at the same view) was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleTimeoutDetected(*model.TimeoutObject, *model.TimeoutObject)

	// OnInvalidTimeoutDetected notifications are produced by the Timeout Aggregation logic
	// whenever an invalid timeout was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnInvalidTimeoutDetected(err model.InvalidTimeoutError)
}

// FinalizationConsumer consumes outbound notifications produced by the logic tracking
// forks and finalization. Such notifications are produced by the active consensus
// participants, and generally potentially relevant to the larger node. The notifications
// are emitted in the order in which the finalization algorithm makes the respective steps.
//
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type FinalizationConsumer interface {
	// OnBlockIncorporated notifications are produced by the Finalization Logic
	// whenever a block is incorporated into the consensus state.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnBlockIncorporated(*model.Block)

	// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
	// a block has been finalized. They are emitted in the order the blocks are finalized.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnFinalizedBlock(*model.Block)
}

// ParticipantConsumer consumes outbound notifications produced by consensus participants
// actively proposing blocks, voting, collecting & aggregating votes to QCs, and participating in
// the pacemaker (sending timeouts, collecting & aggregating timeouts to TCs).
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type ParticipantConsumer interface {
	// OnEventProcessed notifications are produced by the EventHandler when it is done processing
	// and hands control back to the EventLoop to wait for the next event.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnEventProcessed()

	// OnStart notifications are produced by the EventHandler when it starts blocks recovery and
	// prepares for handling incoming events from EventLoop.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStart(currentView uint64)

	// OnReceiveProposal notifications are produced by the EventHandler when it starts processing a block.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReceiveProposal(currentView uint64, proposal *model.Proposal)

	// OnReceiveQc notifications are produced by the EventHandler when it starts processing a
	// QuorumCertificate [QC] constructed by the node's internal vote aggregator.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReceiveQc(currentView uint64, qc *flow.QuorumCertificate)

	// OnReceiveTc notifications are produced by the EventHandler when it starts processing a
	// TimeoutCertificate [TC]  constructed by the node's internal timeout aggregator.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReceiveTc(currentView uint64, tc *flow.TimeoutCertificate)

	// OnPartialTc notifications are produced by the EventHandler when it starts processing partial TC
	// constructed by local timeout aggregator.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnPartialTc(currentView uint64, partialTc *PartialTcCreated)

	// OnLocalTimeout notifications are produced by the EventHandler when it reacts to expiry of round duration timer.
	// Such a notification indicates that the PaceMaker's timeout was processed by the system.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnLocalTimeout(currentView uint64)

	// OnViewChange notifications are produced by PaceMaker when it transitions to a new view
	// based on processing a QC or TC. The arguments specify the oldView (first argument),
	// and the newView to which the PaceMaker transitioned (second argument).
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnViewChange(oldView, newView uint64)

	// OnQcTriggeredViewChange notifications are produced by PaceMaker when it moves to a new view
	// based on processing a QC. The arguments specify the qc (first argument), which triggered
	// the view change, and the newView to which the PaceMaker transitioned (second argument).
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcTriggeredViewChange(oldView uint64, newView uint64, qc *flow.QuorumCertificate)

	// OnTcTriggeredViewChange notifications are produced by PaceMaker when it moves to a new view
	// based on processing a TC. The arguments specify the tc (first argument), which triggered
	// the view change, and the newView to which the PaceMaker transitioned (second argument).
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnTcTriggeredViewChange(oldView uint64, newView uint64, tc *flow.TimeoutCertificate)

	// OnStartingTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker is now waiting for the system to (receive and) process blocks or votes.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStartingTimeout(model.TimerInfo)

	// OnCurrentViewDetails notifications are produced by the EventHandler during the course of a view with auxiliary information.
	// These notifications are generally not produced for all views (for example skipped views).
	// These notifications are guaranteed to be produced for all views we enter after fully processing a message.
	// Example 1:
	//   - We are in view 8. We process a QC with view 10, causing us to enter view 11.
	//   - Then this notification will be produced for view 11.
	// Example 2:
	//   - We are in view 8. We process a proposal with view 10, which contains a TC for view 9 and TC.NewestQC for view 8.
	//   - The QC would allow us to enter view 9 and the TC would allow us to enter view 10,
	//     so after fully processing the message we are in view 10.
	//   - Then this notification will be produced for view 10, but not view 9
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnCurrentViewDetails(currentView, finalizedView uint64, currentLeader flow.Identifier)
}

// VoteCollectorConsumer consumes outbound notifications produced by HotStuff's vote aggregation
// component. These events are primarily intended for the HotStuff-internal state machine (EventHandler),
// but might also be relevant to the larger node in which HotStuff is running.
//
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type VoteCollectorConsumer interface {
	// OnQcConstructedFromVotes notifications are produced by the VoteAggregator
	// component, whenever it constructs a QC from votes.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcConstructedFromVotes(*flow.QuorumCertificate)

	// OnVoteProcessed notifications are produced by the Vote Aggregation logic, each time
	// we successfully ingest a valid vote.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnVoteProcessed(vote *model.Vote)
}

// TimeoutCollectorConsumer consumes outbound notifications produced by HotStuff's timeout aggregation
// component. These events are primarily intended for the HotStuff-internal state machine (EventHandler),
// but might also be relevant to the larger node in which HotStuff is running.
//
// Caution: the events are not strictly ordered by increasing views!
// The notifications are emitted by concurrent processing logic. Over larger time scales, the
// emitted events are for statistically increasing views. However, on short time scales there
// are _no_ monotonicity guarantees w.r.t. the events' views.
//
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type TimeoutCollectorConsumer interface {
	// OnTcConstructedFromTimeouts notifications are produced by the TimeoutProcessor
	// component, whenever it constructs a TC based on TimeoutObjects from a
	// supermajority of consensus participants.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnTcConstructedFromTimeouts(certificate *flow.TimeoutCertificate)

	// OnPartialTcCreated notifications are produced by the TimeoutProcessor
	// component, whenever it collected TimeoutObjects from a superminority
	// of consensus participants for a specific view. Along with the view, it
	// reports the newest QC and TC (for previous view) discovered in process of
	// timeout collection. Per convention, the newest QC is never nil, while
	// the TC for the previous view might be nil.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnPartialTcCreated(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate)

	// OnNewQcDiscovered notifications are produced by the TimeoutCollector
	// component, whenever it discovers new QC included in timeout object.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnNewQcDiscovered(certificate *flow.QuorumCertificate)

	// OnNewTcDiscovered notifications are produced by the TimeoutCollector
	// component, whenever it discovers new TC included in timeout object.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnNewTcDiscovered(certificate *flow.TimeoutCertificate)

	// OnTimeoutProcessed notifications are produced by the Timeout Aggregation logic,
	// each time we successfully ingest a valid timeout.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnTimeoutProcessed(timeout *model.TimeoutObject)
}

// CommunicatorConsumer consumes outbound notifications produced by HotStuff and it's components.
// Notifications allow the HotStuff core algorithm to communicate with the other actors of the consensus process.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type CommunicatorConsumer interface {
	// OnOwnVote notifies about intent to send a vote for the given parameters to the specified recipient.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnOwnVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier)

	// OnOwnTimeout notifies about intent to broadcast the given timeout object(TO) to all actors of the consensus process.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnOwnTimeout(timeout *model.TimeoutObject)

	// OnOwnProposal notifies about intent to broadcast the given block proposal to all actors of
	// the consensus process.
	// delay is to hold the proposal before broadcasting it. Useful to control the block production rate.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnOwnProposal(proposal *flow.Header, targetPublicationTime time.Time)
}

// FollowerConsumer consumes outbound notifications produced by consensus followers.
// It is a subset of the notifications produced by consensus participants.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type FollowerConsumer interface {
	ProposalViolationConsumer
	FinalizationConsumer
}

// Consumer consumes outbound notifications produced by consensus participants.
// Notifications are consensus-internal state changes which are potentially relevant to
// the larger node in which HotStuff is running. The notifications are emitted
// in the order in which the HotStuff algorithm makes the respective steps.
//
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type Consumer interface {
	FollowerConsumer
	CommunicatorConsumer
	ParticipantConsumer
}

// VoteAggregationConsumer consumes outbound notifications produced by Vote Aggregation logic.
// It is a subset of the notifications produced by consensus participants.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type VoteAggregationConsumer interface {
	VoteAggregationViolationConsumer
	VoteCollectorConsumer
}

// TimeoutAggregationConsumer consumes outbound notifications produced by Vote Aggregation logic.
// It is a subset of the notifications produced by consensus participants.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type TimeoutAggregationConsumer interface {
	TimeoutAggregationViolationConsumer
	TimeoutCollectorConsumer
}

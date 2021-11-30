package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// FinalizationConsumer consumes outbound notifications produced by the finalization logic.
// Notifications represent finalization-specific state changes which are potentially relevant
// to the larger node. The notifications are emitted in the order in which the
// finalization algorithm makes the respective steps.
//
// Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
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

	// OnDoubleProposeDetected notifications are produced by the Finalization Logic
	// whenever a double block proposal (equivocation) was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleProposeDetected(*model.Block, *model.Block)
}

// Consumer consumes outbound notifications produced by HotStuff and its components.
// Notifications are consensus-internal state changes which are potentially relevant to
// the larger node in which HotStuff is running. The notifications are emitted
// in the order in which the HotStuff algorithm makes the respective steps.
//
// Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
type Consumer interface {
	FinalizationConsumer

	// OnEventProcessed notifications are produced by the EventHandler when it is done processing
	// and hands control back to the EventLoop to wait for the next event.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnEventProcessed()

	// OnReceiveVote notifications are produced by the EventHandler when it starts processing a vote.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReceiveVote(currentView uint64, vote *model.Vote)

	// OnReceiveProposal notifications are produced by the EventHandler when it starts processing a block.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReceiveProposal(currentView uint64, proposal *model.Proposal)

	// OnEnteringView notifications are produced by the EventHandler when it enters a new view.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnEnteringView(viewNumber uint64, leader flow.Identifier)

	// OnQcTriggeredViewChange notifications are produced by PaceMaker when it moves to a new view
	// based on processing a QC. The arguments specify the qc (first argument), which triggered
	// the view change, and the newView to which the PaceMaker transitioned (second argument).
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcTriggeredViewChange(qc *flow.QuorumCertificate, newView uint64)

	// OnProposingBlock notifications are produced by the EventHandler when the replica, as
	// leader for the respective view, proposing a block.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnProposingBlock(proposal *model.Proposal)

	// OnVoting notifications are produced by the EventHandler when the replica votes for a block.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnVoting(vote *model.Vote)

	// OnQcConstructedFromVotes notifications are produced by the VoteAggregator
	// component, whenever it constructs a QC from votes.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcConstructedFromVotes(*flow.QuorumCertificate)

	// OnStartingTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker is now waiting for the system to (receive and) process blocks or votes.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStartingTimeout(*model.TimerInfo)

	// OnReachedTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker's timeout was processed by the system. The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReachedTimeout(timeout *model.TimerInfo)

	// OnQcIncorporated notifications are produced by ForkChoice
	// whenever a quorum certificate is incorporated into the consensus state.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcIncorporated(*flow.QuorumCertificate)

	// OnForkChoiceGenerated notifications are produced by ForkChoice whenever a fork choice is generated.
	// The arguments specify the view (first argument) of the block which is to be built and the
	// quorum certificate (second argument) that is supposed to be in the block.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnForkChoiceGenerated(uint64, *flow.QuorumCertificate)

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
	OnInvalidVoteDetected(*model.Vote)

	// OnVoteForInvalidBlockDetected notifications are produced by the Vote Aggregation logic
	// whenever vote for invalid proposal was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal)
}

// QCCreatedConsumer consumes outbound notifications produced by HotStuff and its components.
// Notifications are consensus-internal state changes which are potentially relevant to
// the larger node in which HotStuff is running. The notifications are emitted
// in the order in which the HotStuff algorithm makes the respective steps.
//
// Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
type QCCreatedConsumer interface {
	// OnQcConstructedFromVotes notifications are produced by the VoteAggregator
	// component, whenever it constructs a QC from votes.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcConstructedFromVotes(*flow.QuorumCertificate)
}

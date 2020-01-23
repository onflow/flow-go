package notifications

// Distributor consumes notifications outbound events produced by HotStuff and its components.
// Outbound events are all events that are potentially relevant to the larger node
// in which HotStuff (or Forks) is running.
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Distributor interface {

// OnEnteringView notifications are produced by the EventLoop when it enters a new view.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
OnEnteringView(uint64)

// OnSkippedAhead notifications are produced by PaceMaker when it decides to skip over one or more view numbers.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
OnSkippedAhead(uint64)

// OnStartingBlockTimeout notifications are produced by PaceMaker. Such a notification indicates that the
// PaceMaker is now waiting for the system to (receive and) process the block for the current view.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
OnStartingBlockTimeout(uint64)

// OnReachedBlockTimeout notifications are produced by PaceMaker. Such a notification indicates that the
// PaceMaker's timeout for waiting for a block was processed by the system.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
OnReachedBlockTimeout(uint64)

// OnStartingVotesTimeout notifications are produced by PaceMaker. Such a notification indicates that the
// PaceMaker is now waiting for the system to (receive and) process votes for the block for the current view.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
OnStartingVotesTimeout(uint64)

// OnReachedVotesTimeout notifications are produced by PaceMaker. Such a notification indicates that the
// PaceMaker's timeout for waiting for a votes was processed by the system.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
OnReachedVotesTimeout(uint64)

OnBlockIncorporated(proposal *types.BlockProposal)
OnFinalizedBlock(*types.BlockProposal)
OnDoubleProposeDetected(*types.BlockProposal, *types.BlockProposal)
}

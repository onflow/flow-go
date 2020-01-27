package notifications

// Consumer consumes outbound notification events produced by HotStuff and its
// components. Outbound events are all events that are potentially relevant to
// the larger node in which HotStuff (or Forks) is running.
//
// Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
type Consumer interface {

	// OnEnteringView notifications are produced by the EventLoop when it enters a new view.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnEnteringView(viewNumber uint64)

	// OnSkippedAhead notifications are produced by PaceMaker when it decides to skip over one or more view numbers.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnSkippedAhead(viewNumber uint64)

	// OnStartingBlockTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker is now waiting for the system to (receive and) process the block for the current view.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStartingBlockTimeout(viewNumber uint64)

	// OnReachedBlockTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker's timeout for waiting for a block was processed by the system.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReachedBlockTimeout(viewNumber uint64)

	// OnStartingVotesTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker is now waiting for the system to (receive and) process votes for the block for the current view.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStartingVotesTimeout(viewNumber uint64)

	// OnReachedVotesTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker's timeout for waiting for a votes was processed by the system.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReachedVotesTimeout(viewNumber uint64)
}

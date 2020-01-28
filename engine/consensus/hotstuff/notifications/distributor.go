package notifications

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

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

	// OnStartingTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker is now waiting for the system to (receive and) process blocks or votes.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStartingTimeout(*types.TimerInfo)

	// OnReachedTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker's timeout was processed by the system.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReachedTimeout(timeout *types.Timeout)
}

package ingestion2

import "sync/atomic"

// ResultStatus represents the [ResultsForest]'s *internal* status of processing a particular result.
//
// IMPORTANT: In general, the processing status of a result in the ResultsForest is expected to lag
// behind the consensus follower's notion of the analogous quantities, due to our asynchronous,
// information-driven design.
type ResultStatus uint64

const (
	// ResultForCertifiedBlock indicates that the block the result pertains to is certified. The
	// ResultsForest ingests only certified results. This guarantees that for every view, there is
	// at most one block, which can have results. In other words, all results for a given view are
	// for with the same block. Every result in the ResultsForest must be at least certified.
	ResultForCertifiedBlock ResultStatus = iota + 1

	// ResultForFinalizedBlock states that the block the result pertains to is finalized.
	// CAUTION: the result itself may still be orphaned later, if a conflicting result is sealed.
	ResultForFinalizedBlock

	// ResultSealed states that the result is sealed (specifically, seal for this result
	// has been included in a finalized block).
	ResultSealed

	// ResultOrphaned indicates that a different result for the same block has been sealed or
	// that the block itself has been orphaned. In either case, Access Nodes do not need to index
	// the result's data.
	// CAUTION: results with status `ResultSealed` cannot be orphaned.
	ResultOrphaned
)

// String returns the string representation of the result status
func (bs ResultStatus) String() string {
	switch bs {
	case ResultForCertifiedBlock:
		return "certified"
	case ResultForFinalizedBlock:
		return "finalized"
	case ResultSealed:
		return "sealed"
	case ResultOrphaned:
		return "orphaned"
	default:
		return "unknown"
	}
}

// IsValid returns true if the result status is a valid value.
func (bs ResultStatus) IsValid() bool {
	switch bs {
	case ResultForCertifiedBlock, ResultForFinalizedBlock, ResultSealed, ResultOrphaned:
		return true
	default:
		return false
	}
}

// IsValidTransition returns true if the result status can be transitioned to the given status.
func (bs ResultStatus) IsValidTransition(to ResultStatus) bool {
	if to == bs {
		return true
	}
	switch bs {
	case ResultForCertifiedBlock:
		return to == ResultForFinalizedBlock || to == ResultSealed || to == ResultOrphaned
	case ResultForFinalizedBlock:
		return to == ResultSealed || to == ResultOrphaned
	default:
		return false
	}
}

// ResultStatusTracker is a concurrency-safe tracker for ResultStatus using atomic operations.
// It is more efficient under moderate load, compared to a mutex-based approach.
// Concurrent write-read establishes a 'happens before relation' as detailed in https://go.dev/ref/mem
type ResultStatusTracker struct {
	status uint64
}

// NewResultStatusTracker instantiates a concurrency-safe state machine with valid state transitions
// as specified in function [ResultStatus.IsValidTransition] above. The intended use is to rack the
// status of an execution result from the perspective of the ResultsForest.
// Caution: for simplicity, we do not validate the initial value here.
func NewResultStatusTracker(initialValue ResultStatus) ResultStatusTracker {
	return ResultStatusTracker{
		status: uint64(initialValue),
	}
}

// Set the result status to the new value, if and only if this is a valid state transition as defined
// in function [ResultStatus.IsValidTransition].
func (t *ResultStatusTracker) Set(newValue ResultStatus) bool {
	for {
		oldValue := t.Value()
		if !oldValue.IsValidTransition(newValue) {
			return false
		}
		if atomic.CompareAndSwapUint64(&t.status, uint64(oldValue), uint64(newValue)) {
			return true
		}
	}
}

// Value returns the current status of the execution result from the perspective
// of the ResultsForest.
func (t *ResultStatusTracker) Value() ResultStatus {
	return ResultStatus(atomic.LoadUint64(&t.status))
}

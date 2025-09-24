package ingestion2

import "sync/atomic"

// BlockStatus represents the [ResultsForest]'s *internal* status of processing a particular result.
//
// IMPORTANT: In general, the processing status of a result in the ResultsForest is expected to lag behind the
// consensus follower's notion of the analogous quantities, due to our asynchronous, information-driven design.
type BlockStatus uint64

const (
	// ResultForCertifiedBlock indicates that the block the result pertains to is certified. The
	// ResultsForest ingests only certified results. This guarantees that for every view, there is
	// at most one block, which can have results. In other words, all results for a given view are
	// for with the same block. Every result in the ResultsForest must be at least certified.
	ResultForCertifiedBlock BlockStatus = iota + 1

	// ResultForFinalizedBlock states that the block the result pertains to is finalized.
	// CAUTION: the result itself may still be orphaned later, if a conflicting result is sealed.
	ResultForFinalizedBlock

	// ResultSealed states that the result is sealed (specifically, seal for this result
	// has been included in a finalized block).
	ResultSealed

	// ResultOrphaned indicates that a different result for the same block has been sealed or
	// that the block itself has been orphaned. In either case, Access Nodes do not need to index
	// the result's data. CAUTION: results with status `ResultSealed` cannot be orphaned.
	// TODO: this is not (yet?) used
	ResultOrphaned
)

// String returns the string representation of the block status
func (bs BlockStatus) String() string {
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

// IsValid returns true if the block status is a valid value.
func (bs BlockStatus) IsValid() bool {
	switch bs {
	case ResultForCertifiedBlock, ResultForFinalizedBlock, ResultSealed:
		return true
	default:
		return false
	}
}

// IsValidTransition returns true if the block status can be transitioned to the given status.
func (bs BlockStatus) IsValidTransition(to BlockStatus) bool {
	if to == bs {
		return true
	}
	switch bs {
	case ResultForCertifiedBlock:
		return to == ResultForFinalizedBlock || to == ResultSealed
	case ResultForFinalizedBlock:
		return to == ResultSealed
	default:
		return false
	}
}

// BlockStatusTracker is a concurrency-safe tracker for BlockStatus using atomic operations.
// It is more efficient under moderate load, compared to a mutex-based approach.
// Concurrent write-read establishes a 'happens before relation' as detailed in https://go.dev/ref/mem
type BlockStatusTracker struct {
	status uint64
}

// NewBlockStatusTracker instantiates a concurrency-safe state machine with valid state transitions
// as specified in function [BlockStatus.IsValidTransition] above. The intended use is to rack the
// status of an execution result from the perspective of the ResultsForest.
// Caution: for simplicity, we do not validate the initial value here.
func NewBlockStatusTracker(initialValue BlockStatus) BlockStatusTracker {
	return BlockStatusTracker{
		status: uint64(initialValue),
	}
}

// Set the result status to the new value, if and only if this is a valid state transition as defined
// in function [BlockStatus.IsValidTransition].
func (t *BlockStatusTracker) Set(newValue BlockStatus) bool {
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
func (t *BlockStatusTracker) Value() BlockStatus {
	return BlockStatus(atomic.LoadUint64(&t.status))
}

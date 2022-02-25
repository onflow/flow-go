package synchronization

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// TODO: replace WithinTolerance with something/
// Maybe we can just check that if the active range is too small,
// we prioritize batch requests instead?

type ActiveRangeTracker struct {
	minResponses     uint
	defaultRangeSize uint64

	lowerBound uint64
	upperBound uint64

	pendingStart    uint64
	responseTracker *responseTracker
}

var _ module.ActiveRangeTracker = (*ActiveRangeTracker)(nil)

func NewActiveRangeTracker(
	minResponses uint,
	defaultRangeSize uint64,
	initialHeight uint64,
) *ActiveRangeTracker {
	return &ActiveRangeTracker{
		minResponses:     minResponses,
		defaultRangeSize: defaultRangeSize,
		responseTracker:  newResponseTracker(),
		pendingStart:     initialHeight + 1,
		lowerBound:       initialHeight + 1,
		upperBound:       initialHeight + defaultRangeSize,
	}
}

func (a *ActiveRangeTracker) GetActiveRange() flow.Range {
	return flow.Range{
		From: a.lowerBound,
		To:   a.rangeEnd(),
	}
}

func (a *ActiveRangeTracker) UpdateLowerBound(height uint64) {
	for i := a.lowerBound; i < height; i++ {
		// NOTE: If we decide to implement slashing someday, this is
		// where we would check each of the responses we previously
		// received and slash the incorrect ones

		a.responseTracker.discardResponses(i)
	}

	a.lowerBound = height

	if height > a.pendingStart {
		a.pendingStart = height
	}

	a.updatePendingStart()

	if height > a.upperBound {
		a.upperBound = height
	}
}

func (a *ActiveRangeTracker) updatePendingStart() {
	for uint(a.responseTracker.responseCount(a.pendingStart)) >= a.minResponses {
		a.pendingStart++
	}
}

func (a *ActiveRangeTracker) UpdateUpperBound(height uint64) {
	a.upperBound = height

	if height < a.lowerBound {
		a.upperBound = a.lowerBound
	}

	// TODO: If the new upper bound is lower than the previous one,
	// it might be reasonable here to discard any previously received
	// responses that are now out of range.
}

func (a *ActiveRangeTracker) rangeEnd() uint64 {
	end := a.pendingStart + a.defaultRangeSize - 1

	if a.upperBound < end {
		end = a.upperBound
	}

	return end
}

func (a *ActiveRangeTracker) ProcessRange(startHeight uint64, blockIDs []flow.Identifier, originID flow.Identifier) {
	activeRange := a.GetActiveRange()

	startIdx := int(activeRange.From - startHeight)

	if startIdx >= len(blockIDs) {
		return
	} else if startIdx < 0 {
		startIdx = 0
	}

	endIdx := int(activeRange.To - startHeight)

	if endIdx < 0 {
		return
	} else if endIdx >= len(blockIDs) {
		endIdx = len(blockIDs) - 1
	}

	for i := startIdx; i <= endIdx; i++ {
		height := startHeight + uint64(i)

		a.responseTracker.addResponse(height, originID)
	}

	a.updatePendingStart()
}

type responseTracker struct {
	responses map[uint64]map[flow.Identifier]struct{}
}

func newResponseTracker() *responseTracker {
	return &responseTracker{
		responses: make(map[uint64]map[flow.Identifier]struct{}),
	}
}

func (r *responseTracker) responseCount(height uint64) int {
	return len(r.responses[height])
}

func (r *responseTracker) discardResponses(height uint64) {
	delete(r.responses, height)
}

func (r *responseTracker) addResponse(height uint64, originID flow.Identifier) {
	origins, ok := r.responses[height]

	if !ok {
		origins = make(map[flow.Identifier]struct{})
		r.responses[height] = origins
	}

	origins[originID] = struct{}{}
}

package synchronization

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// TODO: replace WithinTolerance with something

type ActiveRange struct {
	minResponses     uint
	defaultRangeSize uint64

	localFinalizedHeight  uint64
	targetFinalizedHeight uint64

	pendingStart uint64
	numResponses map[uint64]uint
}

var _ module.ActiveRange = (*ActiveRange)(nil)

func NewActiveRange(minResponses uint, defaultRangeSize uint64) *ActiveRange {
	return &ActiveRange{
		minResponses:          minResponses,
		defaultRangeSize:      defaultRangeSize,
		numResponses:          make(map[uint64]uint),
		pendingStart:          0,
		localFinalizedHeight:  0,
		targetFinalizedHeight: 0,
	}
}

func (a *ActiveRange) Get() flow.Range {
	return flow.Range{
		From: a.localFinalizedHeight + 1,
		To:   a.rangeEnd(),
	}
}

func (a *ActiveRange) LocalFinalizedHeight(height uint64) {
	a.localFinalizedHeight = height

	// prune numResponses
	for i := a.pendingStart; i <= height; i++ {
		delete(a.numResponses, i)
	}

	if height >= a.pendingStart {
		a.pendingStart = height + 1
	}
}

func (a *ActiveRange) TargetFinalizedHeight(height uint64) {
	oldRangeEnd := a.rangeEnd()

	a.targetFinalizedHeight = height

	rangeEnd := a.rangeEnd()

	// prune numResponses
	for i := rangeEnd + 1; i <= oldRangeEnd; i++ {
		delete(a.numResponses, i)
	}
}

func (a *ActiveRange) rangeEnd() uint64 {
	end := a.targetFinalizedHeight

	if a.pendingStart+a.defaultRangeSize < end {
		end = a.pendingStart + a.defaultRangeSize
	}

	if end <= a.localFinalizedHeight {
		end = a.localFinalizedHeight + 1
	}

	return end
}

func (a *ActiveRange) Update(headers []flow.Header, originID flow.Identifier) {
	rangeEnd := a.rangeEnd()

	for _, header := range headers {
		if header.Height >= a.pendingStart && header.Height <= rangeEnd {
			a.numResponses[header.Height]++

			if header.Height == a.pendingStart && a.numResponses[header.Height] >= a.minResponses {
				delete(a.numResponses, header.Height)
				a.pendingStart++
			}
		}
	}
}

package synchronization

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// TODO: check payloadhash in sync engine

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

func (a *ActiveRange) Get() flow.Range {
	end := a.targetFinalizedHeight

	if a.pendingStart+a.defaultRangeSize < end {
		end = a.pendingStart + a.defaultRangeSize
	}

	return flow.Range{
		From: a.localFinalizedHeight + 1,
		To:   end,
	}
}

func (a *ActiveRange) LocalFinalizedHeight(height uint64) {
	a.localFinalizedHeight = height
}

func (a *ActiveRange) TargetFinalizedHeight(height uint64) {
	a.targetFinalizedHeight = height
}

func (a *ActiveRange) Update(headers []flow.Header, originID flow.Identifier) {
	for _, header := range headers {
		a.numResponses[header.Height]++

		if header.Height == a.pendingStart && a.numResponses[header.Height] >= a.minResponses {
			delete(a.numResponses, header.Height)
			a.pendingStart++
		}
	}
}

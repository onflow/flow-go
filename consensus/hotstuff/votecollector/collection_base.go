//nolint
package votecollector

import (
	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
)

type CollectionBase struct {
	log                zerolog.Logger
	workerPool         *workerpool.WorkerPool
	doubleVoteDetector *DoubleVoteDetector

	view uint64
}

func NewCollectionBase(view uint64) CollectionBase {
	return CollectionBase{
		view:               view,
		doubleVoteDetector: NewDoubleVoteDetector(view),
	}
}

func (c *CollectionBase) View() uint64 {
	return c.view
}

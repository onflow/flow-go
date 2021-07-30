package votecollector

import (
	"github.com/gammazero/workerpool"
	"github.com/onflow/flow-go/model/flow"
)

type BaseVoteCollector struct {
	workerPool *workerpool.WorkerPool

	blockID flow.Identifier
}

func NewBaseVoteCollector(blockID flow.Identifier) BaseVoteCollector {
	return BaseVoteCollector{
		blockID: blockID,
	}
}

func (c *BaseVoteCollector) BlockID() flow.Identifier {
	return c.blockID
}

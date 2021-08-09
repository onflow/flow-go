package votecollector

import (
	"github.com/gammazero/workerpool"
	"github.com/onflow/flow-go/model/flow"
)

type CollectionBase struct {
	workerPool *workerpool.WorkerPool

	blockID flow.Identifier
}

func NewCollectionBase(blockID flow.Identifier) CollectionBase {
	return CollectionBase{
		blockID: blockID,
	}
}

func (c *CollectionBase) BlockID() flow.Identifier {
	return c.blockID
}

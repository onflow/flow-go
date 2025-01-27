package executor

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type AggregatedExecutor struct {
	executors []IterationExecutor
}

var _ IterationExecutor = (*AggregatedExecutor)(nil)

func NewAggregatedExecutor(executors []IterationExecutor) *AggregatedExecutor {
	return &AggregatedExecutor{
		executors: executors,
	}
}

func (a *AggregatedExecutor) ExecuteByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) (exception error) {
	for _, executor := range a.executors {
		exception = executor.ExecuteByBlockID(blockID, batch)
		if exception != nil {
			return exception
		}
	}
	return nil
}

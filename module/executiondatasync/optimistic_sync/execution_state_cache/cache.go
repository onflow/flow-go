package execution_state_cache

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

type ExecutionStateCache struct {
}

var _ optimistic_sync.ExecutionStateCache = (*ExecutionStateCache)(nil)

func NewExecutionStateCache() *ExecutionStateCache {
	return &ExecutionStateCache{}
}

func (e *ExecutionStateCache) Snapshot(executionResultID flow.Identifier) (optimistic_sync.Snapshot, error) {
	//TODO implement me
	panic("implement me")
}

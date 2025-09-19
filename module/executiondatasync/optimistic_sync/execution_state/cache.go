package execution_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

type CacheMock struct {
	snapshot optimistic_sync.Snapshot
}

var _ optimistic_sync.ExecutionStateCache = (*CacheMock)(nil)

func NewExecutionStateCacheMock(snapshot optimistic_sync.Snapshot) *CacheMock {
	return &CacheMock{
		snapshot: snapshot,
	}
}

// Snapshot returns a static snapshot that always queries the database.
func (e *CacheMock) Snapshot(_ flow.Identifier) (optimistic_sync.Snapshot, error) {
	return e.snapshot, nil
}

package results

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

type PendingResults struct {
	sync.RWMutex
	byID map[flow.Identifier]*flow.PendingResult
}

func NewPendingResults() *PendingResults {
	return &PendingResults{
		byID: make(map[flow.Identifier]*flow.PendingResult),
	}
}

func (b *PendingResults) Add(result *flow.PendingResult) bool {
	b.Lock()
	defer b.Unlock()

	resultID := result.ExecutionResult.ID()
	_, exists := b.byID[resultID]
	if exists {
		return false
	}

	b.byID[resultID] = result
	return true
}

func (b *PendingResults) Rem(resultID flow.Identifier) bool {
	b.Lock()
	defer b.Unlock()

	_, exists := b.byID[resultID]
	if exists {
		return false
	}

	delete(b.byID, resultID)
	return true
}

func (b *PendingResults) Has(resultID flow.Identifier) bool {
	b.RLock()
	defer b.RUnlock()

	_, exists := b.byID[resultID]
	return exists
}

func (b *PendingResults) ByID(resultID flow.Identifier) (*flow.PendingResult, bool) {
	b.RLock()
	defer b.RUnlock()

	result, exists := b.byID[resultID]
	if !exists {
		return nil, false
	}

	return result, true
}

func (b *PendingResults) Size() uint {
	return uint(len(b.byID))
}

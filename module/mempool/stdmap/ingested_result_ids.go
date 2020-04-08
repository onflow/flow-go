package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/model"
)

// IngestedResultIDs represents a concurrency-safe memory pool for ingested execution result IDs.
// By ingested execution result IDs we mean those that have a verifiable chunk for them forwarded from
// Ingest engine to the Verify engine of Verification node
type IngestedResultIDs struct {
	*Backend
}

// NewIngestedChunkIDs creates a new memory pool for chunk states.
func NewIngestedResultIDs(limit uint) (*IngestedResultIDs, error) {
	i := &IngestedResultIDs{
		Backend: NewBackend(WithLimit(limit)),
	}

	return i, nil
}

// Add will add the given result ID to the memory pool or it will error if
// the result ID is already in the memory pool.
func (i *IngestedResultIDs) Add(result *flow.ExecutionResult) error {
	// wraps chunk ID around an ID entity to be stored in the mempool
	id := &model.IdEntity{
		Id: result.ID(),
	}
	return i.Backend.Add(id)
}

// Rem will remove the given result ID from the memory pool; it will
// return true if the result ID was known and removed.
func (i *IngestedResultIDs) Has(resultID flow.Identifier) bool {
	return i.Backend.Has(resultID)
}

// Rem will remove the given result ID from the memory pool; it will
// return true if the result ID was known and removed.
func (i *IngestedResultIDs) Rem(resultID flow.Identifier) bool {
	return i.Backend.Rem(resultID)
}

// All will retrieve all result IDs that are currently in the memory pool
// as an IdentityList
func (i *IngestedResultIDs) All() flow.IdentifierList {
	entities := i.Backend.All()
	resultIDs := make([]flow.Identifier, 0, len(entities))
	for _, entity := range entities {
		idEntity := entity.(model.IdEntity)
		resultIDs = append(resultIDs, idEntity.Id)
	}
	return resultIDs
}

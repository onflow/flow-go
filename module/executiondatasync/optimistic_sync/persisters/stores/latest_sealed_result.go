package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*LatestSealedResultStore)(nil)

// LatestSealedResultStore handles persisting transaction result error messages
type LatestSealedResultStore struct {
	latestPersistedSealedResult storage.LatestPersistedSealedResult
	height                      uint64
	executionResultID           flow.Identifier
}

func NewLatestSealedResultStore(
	latestPersistedSealedResult storage.LatestPersistedSealedResult,
	executionResultID flow.Identifier,
	height uint64,
) *LatestSealedResultStore {
	return &LatestSealedResultStore{
		latestPersistedSealedResult: latestPersistedSealedResult,
		height:                      height,
		executionResultID:           executionResultID,
	}
}

// Persist adds the latest sealed result to the batch.
// No errors are expected during normal operations
func (t *LatestSealedResultStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	if err := t.latestPersistedSealedResult.BatchSet(t.executionResultID, t.height, batch); err != nil {
		return fmt.Errorf("could not persist latest sealed result: %w", err)
	}
	return nil
}

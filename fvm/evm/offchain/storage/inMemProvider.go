package storage

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// InMemoryStorageProvider uses in memory views
// to track deltas of each block.
// Mostly used for testing purposes
type InMemoryStorageProvider struct {
	views []EphemeralStorage
}

var _ types.StorageProvider = &InMemoryStorageProvider{}

// NewInMemoryStorageProvider constructs a new InMemoryStorageProvider
func NewInMemoryStorageProvider() *InMemoryStorageProvider {
	views := make([]EphemeralStorage, 2)
	views[0] = *NewEphemeralStorage(NewReadOnlyStorage(EmptySnapshot))
	views[1] = *NewEphemeralStorage(&views[0])
	return &InMemoryStorageProvider{
		views: views,
	}
}

// GetSnapshotAt returns the snapshot at specific block height
func (sp *InMemoryStorageProvider) GetSnapshotAt(height uint64) (types.BackendStorageSnapshot, error) {
	if int(height) >= len(sp.views) {
		return nil, fmt.Errorf("storage for the given height (%d) is not available", height)
	}
	return &sp.views[height], nil
}

// OnBlockExecuted accepts delta when a block is executed
func (sp *InMemoryStorageProvider) OnBlockExecuted(
	delta map[flow.RegisterID]flow.RegisterValue,
) error {
	lastView := sp.views[len(sp.views)-1]
	for k, v := range delta {
		err := lastView.SetValue(
			[]byte(k.Owner),
			[]byte(k.Key),
			[]byte(v))
		if err != nil {
			return err
		}
	}
	sp.views = append(sp.views, *NewEphemeralStorage(&lastView))
	return nil
}

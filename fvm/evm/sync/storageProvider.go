package sync

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// StorageProvider provides access to storage at
// specific time point in history of the EVM chain
type StorageProvider interface {
	// GetSnapshotAt returns a readonly snapshot of storage
	// at specific block (start state of the block before executing transactions)
	GetSnapshotAt(height uint64) (BackendStorageSnapshot, error)
}

// InMemoryStorageProvider holds on block changes into memory
// mostly provided for testing
type InMemoryStorageProvider struct {
	views []EphemeralStorage
}

var _ StorageProvider = &InMemoryStorageProvider{}

func NewInMemoryStorageProvider() *InMemoryStorageProvider {
	views := make([]EphemeralStorage, 2)
	views[0] = *NewEphemeralStorage(NewReadOnlyStorage(EmptySnapshot))
	views[1] = *NewEphemeralStorage(&views[0])
	return &InMemoryStorageProvider{
		views: views,
	}
}

func (sp *InMemoryStorageProvider) GetSnapshotAt(height uint64) (BackendStorageSnapshot, error) {
	if int(height) >= len(sp.views) {
		return nil, fmt.Errorf("storage for the given height (%d) is not available", height)
	}
	return &sp.views[height], nil
}

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

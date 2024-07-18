package storage

import (
	"context"

	"github.com/ipfs/go-datastore"
)

// ExecutionDataStorage defines the interface for key-value store operations.
type ExecutionDataStorage interface {
	Datastore() datastore.Batching
	Close() error
	CollectGarbage(ctx context.Context) error
}

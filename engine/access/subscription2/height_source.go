package subscription2

import (
	"context"
)

// HeightSource TODO: this interface is only needed for tests
type HeightSource interface {
	// StartHeight first height to stream from
	StartHeight() uint64

	// EndHeight is optional bound; 0 = unbounded.
	EndHeight() uint64

	// ReadyUpToHeight returns the highest height safe to serve for this source
	ReadyUpToHeight(ctx context.Context) (uint64, error)

	// GetItemAtHeight returns an item by height. Return storage.ErrNotFound to mean "not ingested yet".
	GetItemAtHeight(ctx context.Context, height uint64) (any, error)
}

package subscription

import (
	"context"
	"errors"
)

// HeightSource TODO: this interface is only needed for tests.
// As each streamer is anyway coupled to a specific data source.
// So, this interface might be useless. However, I guess it'll be used in tests.
type HeightSource[T any] interface {
	// StartHeight first height to stream from
	StartHeight() uint64

	// EndHeight is optional bound; 0 = unbounded.
	EndHeight() uint64

	// ReadyUpToHeight returns the highest height safe to serve for this source
	ReadyUpToHeight() (uint64, error)

	// GetItemAtHeight returns an item by height.
	// Returns ErrItemNotIngested if data is not available yet.
	GetItemAtHeight(ctx context.Context, height uint64) (T, error)
}

var ErrItemNotIngested = errors.New("data item is not ingested yet")
